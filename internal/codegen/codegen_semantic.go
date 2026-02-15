package codegen

import (
	"twice/internal/ast"
	"twice/internal/typesys"
)

// semanticCheck performs a fast validation pass before codegen so we can
// surface type-declaration issues early and avoid noisy downstream errors.
func (cg *CodeGen) semanticCheck(program *ast.Program) {
	if program == nil {
		return
	}
	aliases := map[string]string{}
	collectTypeAliases(program, aliases)
	for _, st := range program.Statements {
		cg.semanticCheckStatement(st, aliases)
	}
}

func collectTypeAliases(node ast.Node, aliases map[string]string) {
	switch n := node.(type) {
	case *ast.Program:
		for _, st := range n.Statements {
			collectTypeAliases(st, aliases)
		}
	case *ast.TypeDeclStatement:
		if n.Name != nil && n.Name.Value != "" {
			if _, exists := aliases[n.Name.Value]; !exists {
				aliases[n.Name.Value] = n.TypeName
			}
		}
	case *ast.BlockStatement:
		for _, st := range n.Statements {
			collectTypeAliases(st, aliases)
		}
	case *ast.FunctionStatement:
		if n.Function != nil && n.Function.Body != nil {
			collectTypeAliases(n.Function.Body, aliases)
		}
	case *ast.WhileStatement:
		if n.Body != nil {
			collectTypeAliases(n.Body, aliases)
		}
	case *ast.LoopStatement:
		if n.Body != nil {
			collectTypeAliases(n.Body, aliases)
		}
	case *ast.ForStatement:
		if n.Init != nil {
			collectTypeAliases(n.Init, aliases)
		}
		if n.Periodic != nil {
			collectTypeAliases(n.Periodic, aliases)
		}
		if n.Body != nil {
			collectTypeAliases(n.Body, aliases)
		}
	}
}

func semanticAliasResolver(aliases map[string]string) typesys.AliasResolver {
	return func(name string) (string, bool) {
		v, ok := aliases[name]
		return v, ok
	}
}

func (cg *CodeGen) semanticKnownType(typeName string, aliases map[string]string) bool {
	return cg.semanticKnownTypeWithParams(typeName, aliases, nil)
}

func (cg *CodeGen) semanticKnownTypeWithParams(typeName string, aliases map[string]string, typeParams map[string]struct{}) bool {
	if resolved, ok := typesys.NormalizeTypeName(typeName, semanticAliasResolver(aliases)); ok {
		typeName = resolved
	}
	base, _, ok := typesys.ParseTypeDescriptor(typeName)
	if !ok {
		return false
	}
	if parts, isUnion := typesys.SplitTopLevelUnion(base); isUnion {
		for _, p := range parts {
			if !cg.semanticKnownTypeWithParams(p, aliases, typeParams) {
				return false
			}
		}
		return true
	}
	if parts, isTuple := typesys.SplitTopLevelTuple(base); isTuple {
		for _, p := range parts {
			if !cg.semanticKnownTypeWithParams(p, aliases, typeParams) {
				return false
			}
		}
		return true
	}
	if typeParams != nil {
		if _, ok := typeParams[base]; ok {
			return true
		}
	}
	if gbase, gargs, ok := typesys.SplitGenericType(base); ok {
		if _, exists := aliases[gbase]; !exists {
			return false
		}
		for _, a := range gargs {
			if !cg.semanticKnownTypeWithParams(a, aliases, typeParams) {
				return false
			}
		}
		return true
	}
	if _, exists := aliases[base]; exists {
		return true
	}
	return typesys.IsBuiltinTypeName(base)
}

func (cg *CodeGen) semanticCheckStatement(stmt ast.Statement, aliases map[string]string) {
	switch s := stmt.(type) {
	case *ast.LetStatement:
		if s.TypeName != "" && !cg.semanticKnownType(s.TypeName, aliases) {
			cg.addNodeError("unknown type: "+s.TypeName, s)
		}
		if s.Value != nil {
			cg.semanticCheckExpression(s.Value, aliases)
		}
	case *ast.ConstStatement:
		if s.TypeName != "" && !cg.semanticKnownType(s.TypeName, aliases) {
			cg.addNodeError("unknown type: "+s.TypeName, s)
		}
		if s.Value != nil {
			cg.semanticCheckExpression(s.Value, aliases)
		}
	case *ast.TypeDeclStatement:
		if s.TypeName != "" {
			typeParamSet := toTypeParamSet(s.TypeParams)
			if !cg.semanticKnownTypeWithParams(s.TypeName, aliases, typeParamSet) {
				cg.addNodeError("unknown type: "+s.TypeName, s)
			}
		}
	case *ast.AssignStatement:
		if s.Value != nil {
			cg.semanticCheckExpression(s.Value, aliases)
		}
	case *ast.IndexAssignStatement:
		if s.Left != nil {
			cg.semanticCheckExpression(s.Left, aliases)
		}
		if s.Value != nil {
			cg.semanticCheckExpression(s.Value, aliases)
		}
	case *ast.ReturnStatement:
		if s.ReturnValue != nil {
			cg.semanticCheckExpression(s.ReturnValue, aliases)
		}
	case *ast.ExpressionStatement:
		if s.Expression != nil {
			cg.semanticCheckExpression(s.Expression, aliases)
		}
	case *ast.FunctionStatement:
		if s.Function != nil {
			typeParamSet := toTypeParamSet(s.Function.TypeParams)
			if s.Function.ReturnType != "" && !cg.semanticKnownTypeWithParams(s.Function.ReturnType, aliases, typeParamSet) {
				cg.addNodeError("unknown type: "+s.Function.ReturnType, s.Function)
			}
			for _, p := range s.Function.Parameters {
				if p != nil && p.TypeName != "" && !cg.semanticKnownTypeWithParams(p.TypeName, aliases, typeParamSet) {
					cg.addNodeError("unknown type: "+p.TypeName, p.Name)
				}
			}
			if s.Function.Body != nil {
				for _, nested := range s.Function.Body.Statements {
					cg.semanticCheckStatement(nested, aliases)
				}
			}
		}
	case *ast.BlockStatement:
		for _, nested := range s.Statements {
			cg.semanticCheckStatement(nested, aliases)
		}
	case *ast.WhileStatement:
		cg.semanticCheckExpression(s.Condition, aliases)
		if s.Body != nil {
			for _, nested := range s.Body.Statements {
				cg.semanticCheckStatement(nested, aliases)
			}
		}
	case *ast.LoopStatement:
		if s.Body != nil {
			for _, nested := range s.Body.Statements {
				cg.semanticCheckStatement(nested, aliases)
			}
		}
	case *ast.ForStatement:
		if s.Init != nil {
			cg.semanticCheckStatement(s.Init, aliases)
		}
		if s.Condition != nil {
			cg.semanticCheckExpression(s.Condition, aliases)
		}
		if s.Periodic != nil {
			cg.semanticCheckStatement(s.Periodic, aliases)
		}
		if s.Body != nil {
			for _, nested := range s.Body.Statements {
				cg.semanticCheckStatement(nested, aliases)
			}
		}
	}
}

func (cg *CodeGen) semanticCheckExpression(expr ast.Expression, aliases map[string]string) {
	switch e := expr.(type) {
	case *ast.ArrayLiteral:
		for _, el := range e.Elements {
			cg.semanticCheckExpression(el, aliases)
		}
	case *ast.TupleLiteral:
		for _, el := range e.Elements {
			cg.semanticCheckExpression(el, aliases)
		}
	case *ast.PrefixExpression:
		cg.semanticCheckExpression(e.Right, aliases)
	case *ast.InfixExpression:
		cg.semanticCheckExpression(e.Left, aliases)
		cg.semanticCheckExpression(e.Right, aliases)
	case *ast.IfExpression:
		cg.semanticCheckExpression(e.Condition, aliases)
		if e.Consequence != nil {
			for _, st := range e.Consequence.Statements {
				cg.semanticCheckStatement(st, aliases)
			}
		}
		if e.Alternative != nil {
			for _, st := range e.Alternative.Statements {
				cg.semanticCheckStatement(st, aliases)
			}
		}
	case *ast.FunctionLiteral:
		typeParamSet := toTypeParamSet(e.TypeParams)
		if e.ReturnType != "" && !cg.semanticKnownTypeWithParams(e.ReturnType, aliases, typeParamSet) {
			cg.addNodeError("unknown type: "+e.ReturnType, e)
		}
		for _, p := range e.Parameters {
			if p != nil && p.TypeName != "" && !cg.semanticKnownTypeWithParams(p.TypeName, aliases, typeParamSet) {
				cg.addNodeError("unknown type: "+p.TypeName, p.Name)
			}
		}
		if e.Body != nil {
			for _, st := range e.Body.Statements {
				cg.semanticCheckStatement(st, aliases)
			}
		}
	case *ast.CallExpression:
		cg.semanticCheckExpression(e.Function, aliases)
		for _, a := range e.Arguments {
			cg.semanticCheckExpression(a, aliases)
		}
	case *ast.IndexExpression:
		cg.semanticCheckExpression(e.Left, aliases)
		cg.semanticCheckExpression(e.Index, aliases)
	case *ast.MethodCallExpression:
		cg.semanticCheckExpression(e.Object, aliases)
		for _, a := range e.Arguments {
			cg.semanticCheckExpression(a, aliases)
		}
	case *ast.MemberAccessExpression:
		cg.semanticCheckExpression(e.Object, aliases)
	case *ast.NullSafeAccessExpression:
		cg.semanticCheckExpression(e.Object, aliases)
	case *ast.TupleAccessExpression:
		cg.semanticCheckExpression(e.Left, aliases)
	case *ast.NamedArgument:
		cg.semanticCheckExpression(e.Value, aliases)
	}
}
