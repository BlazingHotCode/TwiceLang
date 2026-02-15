package codegen

import (
	"fmt"
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
	genericArities := map[string]int{}
	nonGenericAliases := map[string]struct{}{}
	collectTypeAliases(program, aliases)
	collectGenericAliasArities(program, genericArities)
	collectNonGenericAliasNames(program, nonGenericAliases)
	for _, st := range program.Statements {
		cg.semanticCheckStatement(st, aliases, genericArities, nonGenericAliases)
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

func collectGenericAliasArities(node ast.Node, arities map[string]int) {
	switch n := node.(type) {
	case *ast.Program:
		for _, st := range n.Statements {
			collectGenericAliasArities(st, arities)
		}
	case *ast.TypeDeclStatement:
		if n.Name != nil && n.Name.Value != "" && len(n.TypeParams) > 0 {
			if _, exists := arities[n.Name.Value]; !exists {
				arities[n.Name.Value] = len(n.TypeParams)
			}
		}
	case *ast.BlockStatement:
		for _, st := range n.Statements {
			collectGenericAliasArities(st, arities)
		}
	case *ast.FunctionStatement:
		if n.Function != nil && n.Function.Body != nil {
			collectGenericAliasArities(n.Function.Body, arities)
		}
	case *ast.WhileStatement:
		if n.Body != nil {
			collectGenericAliasArities(n.Body, arities)
		}
	case *ast.LoopStatement:
		if n.Body != nil {
			collectGenericAliasArities(n.Body, arities)
		}
	case *ast.ForStatement:
		if n.Init != nil {
			collectGenericAliasArities(n.Init, arities)
		}
		if n.Periodic != nil {
			collectGenericAliasArities(n.Periodic, arities)
		}
		if n.Body != nil {
			collectGenericAliasArities(n.Body, arities)
		}
	}
}

func collectNonGenericAliasNames(node ast.Node, names map[string]struct{}) {
	switch n := node.(type) {
	case *ast.Program:
		for _, st := range n.Statements {
			collectNonGenericAliasNames(st, names)
		}
	case *ast.TypeDeclStatement:
		if n.Name != nil && n.Name.Value != "" && len(n.TypeParams) == 0 {
			names[n.Name.Value] = struct{}{}
		}
	case *ast.BlockStatement:
		for _, st := range n.Statements {
			collectNonGenericAliasNames(st, names)
		}
	case *ast.FunctionStatement:
		if n.Function != nil && n.Function.Body != nil {
			collectNonGenericAliasNames(n.Function.Body, names)
		}
	case *ast.WhileStatement:
		if n.Body != nil {
			collectNonGenericAliasNames(n.Body, names)
		}
	case *ast.LoopStatement:
		if n.Body != nil {
			collectNonGenericAliasNames(n.Body, names)
		}
	case *ast.ForStatement:
		if n.Init != nil {
			collectNonGenericAliasNames(n.Init, names)
		}
		if n.Periodic != nil {
			collectNonGenericAliasNames(n.Periodic, names)
		}
		if n.Body != nil {
			collectNonGenericAliasNames(n.Body, names)
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

func (cg *CodeGen) semanticCheckStatement(stmt ast.Statement, aliases map[string]string, genericArities map[string]int, nonGenericAliases map[string]struct{}) {
	switch s := stmt.(type) {
	case *ast.LetStatement:
		if s.TypeName != "" {
			if msg, ok := semanticGenericTypeArityError(s.TypeName, genericArities, nonGenericAliases, nil); ok {
				cg.addNodeError(msg, s)
			} else if !cg.semanticKnownType(s.TypeName, aliases) {
				cg.addNodeError("unknown type: "+s.TypeName, s)
			}
		}
		if s.Value != nil {
			cg.semanticCheckExpression(s.Value, aliases, genericArities, nonGenericAliases)
		}
	case *ast.ConstStatement:
		if s.TypeName != "" {
			if msg, ok := semanticGenericTypeArityError(s.TypeName, genericArities, nonGenericAliases, nil); ok {
				cg.addNodeError(msg, s)
			} else if !cg.semanticKnownType(s.TypeName, aliases) {
				cg.addNodeError("unknown type: "+s.TypeName, s)
			}
		}
		if s.Value != nil {
			cg.semanticCheckExpression(s.Value, aliases, genericArities, nonGenericAliases)
		}
	case *ast.TypeDeclStatement:
		if s.TypeName != "" {
			typeParamSet := toTypeParamSet(s.TypeParams)
			if msg, ok := semanticGenericTypeArityError(s.TypeName, genericArities, nonGenericAliases, typeParamSet); ok {
				cg.addNodeError(msg, s)
			} else if !cg.semanticKnownTypeWithParams(s.TypeName, aliases, typeParamSet) {
				cg.addNodeError("unknown type: "+s.TypeName, s)
			}
		}
	case *ast.AssignStatement:
		if s.Value != nil {
			cg.semanticCheckExpression(s.Value, aliases, genericArities, nonGenericAliases)
		}
	case *ast.IndexAssignStatement:
		if s.Left != nil {
			cg.semanticCheckExpression(s.Left, aliases, genericArities, nonGenericAliases)
		}
		if s.Value != nil {
			cg.semanticCheckExpression(s.Value, aliases, genericArities, nonGenericAliases)
		}
	case *ast.ReturnStatement:
		if s.ReturnValue != nil {
			cg.semanticCheckExpression(s.ReturnValue, aliases, genericArities, nonGenericAliases)
		}
	case *ast.ExpressionStatement:
		if s.Expression != nil {
			cg.semanticCheckExpression(s.Expression, aliases, genericArities, nonGenericAliases)
		}
	case *ast.FunctionStatement:
		if s.Function != nil {
			typeParamSet := toTypeParamSet(s.Function.TypeParams)
			if s.Function.ReturnType != "" {
				if msg, ok := semanticGenericTypeArityError(s.Function.ReturnType, genericArities, nonGenericAliases, typeParamSet); ok {
					cg.addNodeError(msg, s.Function)
				} else if !cg.semanticKnownTypeWithParams(s.Function.ReturnType, aliases, typeParamSet) {
					cg.addNodeError("unknown type: "+s.Function.ReturnType, s.Function)
				}
			}
			for _, p := range s.Function.Parameters {
				if p != nil && p.TypeName != "" {
					if msg, ok := semanticGenericTypeArityError(p.TypeName, genericArities, nonGenericAliases, typeParamSet); ok {
						cg.addNodeError(msg, p.Name)
					} else if !cg.semanticKnownTypeWithParams(p.TypeName, aliases, typeParamSet) {
						cg.addNodeError("unknown type: "+p.TypeName, p.Name)
					}
				}
			}
			if s.Function.Body != nil {
				for _, nested := range s.Function.Body.Statements {
					cg.semanticCheckStatement(nested, aliases, genericArities, nonGenericAliases)
				}
			}
		}
	case *ast.BlockStatement:
		for _, nested := range s.Statements {
			cg.semanticCheckStatement(nested, aliases, genericArities, nonGenericAliases)
		}
	case *ast.WhileStatement:
		cg.semanticCheckExpression(s.Condition, aliases, genericArities, nonGenericAliases)
		if s.Body != nil {
			for _, nested := range s.Body.Statements {
				cg.semanticCheckStatement(nested, aliases, genericArities, nonGenericAliases)
			}
		}
	case *ast.LoopStatement:
		if s.Body != nil {
			for _, nested := range s.Body.Statements {
				cg.semanticCheckStatement(nested, aliases, genericArities, nonGenericAliases)
			}
		}
	case *ast.ForStatement:
		if s.Init != nil {
			cg.semanticCheckStatement(s.Init, aliases, genericArities, nonGenericAliases)
		}
		if s.Condition != nil {
			cg.semanticCheckExpression(s.Condition, aliases, genericArities, nonGenericAliases)
		}
		if s.Periodic != nil {
			cg.semanticCheckStatement(s.Periodic, aliases, genericArities, nonGenericAliases)
		}
		if s.Body != nil {
			for _, nested := range s.Body.Statements {
				cg.semanticCheckStatement(nested, aliases, genericArities, nonGenericAliases)
			}
		}
	}
}

func (cg *CodeGen) semanticCheckExpression(expr ast.Expression, aliases map[string]string, genericArities map[string]int, nonGenericAliases map[string]struct{}) {
	switch e := expr.(type) {
	case *ast.ArrayLiteral:
		for _, el := range e.Elements {
			cg.semanticCheckExpression(el, aliases, genericArities, nonGenericAliases)
		}
	case *ast.TupleLiteral:
		for _, el := range e.Elements {
			cg.semanticCheckExpression(el, aliases, genericArities, nonGenericAliases)
		}
	case *ast.PrefixExpression:
		cg.semanticCheckExpression(e.Right, aliases, genericArities, nonGenericAliases)
	case *ast.InfixExpression:
		cg.semanticCheckExpression(e.Left, aliases, genericArities, nonGenericAliases)
		cg.semanticCheckExpression(e.Right, aliases, genericArities, nonGenericAliases)
	case *ast.IfExpression:
		cg.semanticCheckExpression(e.Condition, aliases, genericArities, nonGenericAliases)
		if e.Consequence != nil {
			for _, st := range e.Consequence.Statements {
				cg.semanticCheckStatement(st, aliases, genericArities, nonGenericAliases)
			}
		}
		if e.Alternative != nil {
			for _, st := range e.Alternative.Statements {
				cg.semanticCheckStatement(st, aliases, genericArities, nonGenericAliases)
			}
		}
	case *ast.FunctionLiteral:
		typeParamSet := toTypeParamSet(e.TypeParams)
		if e.ReturnType != "" {
			if msg, ok := semanticGenericTypeArityError(e.ReturnType, genericArities, nonGenericAliases, typeParamSet); ok {
				cg.addNodeError(msg, e)
			} else if !cg.semanticKnownTypeWithParams(e.ReturnType, aliases, typeParamSet) {
				cg.addNodeError("unknown type: "+e.ReturnType, e)
			}
		}
		for _, p := range e.Parameters {
			if p != nil && p.TypeName != "" {
				if msg, ok := semanticGenericTypeArityError(p.TypeName, genericArities, nonGenericAliases, typeParamSet); ok {
					cg.addNodeError(msg, p.Name)
				} else if !cg.semanticKnownTypeWithParams(p.TypeName, aliases, typeParamSet) {
					cg.addNodeError("unknown type: "+p.TypeName, p.Name)
				}
			}
		}
		if e.Body != nil {
			for _, st := range e.Body.Statements {
				cg.semanticCheckStatement(st, aliases, genericArities, nonGenericAliases)
			}
		}
	case *ast.CallExpression:
		cg.semanticCheckExpression(e.Function, aliases, genericArities, nonGenericAliases)
		for _, a := range e.Arguments {
			cg.semanticCheckExpression(a, aliases, genericArities, nonGenericAliases)
		}
	case *ast.IndexExpression:
		cg.semanticCheckExpression(e.Left, aliases, genericArities, nonGenericAliases)
		cg.semanticCheckExpression(e.Index, aliases, genericArities, nonGenericAliases)
	case *ast.MethodCallExpression:
		cg.semanticCheckExpression(e.Object, aliases, genericArities, nonGenericAliases)
		for _, a := range e.Arguments {
			cg.semanticCheckExpression(a, aliases, genericArities, nonGenericAliases)
		}
	case *ast.MemberAccessExpression:
		cg.semanticCheckExpression(e.Object, aliases, genericArities, nonGenericAliases)
	case *ast.NullSafeAccessExpression:
		cg.semanticCheckExpression(e.Object, aliases, genericArities, nonGenericAliases)
	case *ast.TupleAccessExpression:
		cg.semanticCheckExpression(e.Left, aliases, genericArities, nonGenericAliases)
	case *ast.NamedArgument:
		cg.semanticCheckExpression(e.Value, aliases, genericArities, nonGenericAliases)
	}
}

func semanticGenericTypeArityError(typeName string, genericArities map[string]int, nonGenericAliases map[string]struct{}, typeParams map[string]struct{}) (string, bool) {
	base, _, ok := typesys.ParseTypeDescriptor(typeName)
	if !ok {
		return "", false
	}
	if members, isUnion := typesys.SplitTopLevelUnion(base); isUnion {
		for _, m := range members {
			if msg, ok := semanticGenericTypeArityError(m, genericArities, nonGenericAliases, typeParams); ok {
				return msg, true
			}
		}
		return "", false
	}
	if members, isTuple := typesys.SplitTopLevelTuple(base); isTuple {
		for _, m := range members {
			if msg, ok := semanticGenericTypeArityError(m, genericArities, nonGenericAliases, typeParams); ok {
				return msg, true
			}
		}
		return "", false
	}
	if gb, args, ok := typesys.SplitGenericType(base); ok {
		if _, isNonGeneric := nonGenericAliases[gb]; isNonGeneric {
			return fmt.Sprintf("wrong number of generic type arguments for %s: expected %d, got %d", gb, 0, len(args)), true
		}
		if expected, exists := genericArities[gb]; exists && expected != len(args) {
			return fmt.Sprintf("wrong number of generic type arguments for %s: expected %d, got %d", gb, expected, len(args)), true
		}
		for _, a := range args {
			if msg, ok := semanticGenericTypeArityError(a, genericArities, nonGenericAliases, typeParams); ok {
				return msg, true
			}
		}
		return "", false
	}
	if typeParams != nil {
		if _, ok := typeParams[base]; ok {
			return "", false
		}
	}
	if _, ok := nonGenericAliases[base]; ok {
		return "", false
	}
	if expected, exists := genericArities[base]; exists && expected > 0 {
		return fmt.Sprintf("wrong number of generic type arguments for %s: expected %d, got %d", base, expected, 0), true
	}
	return "", false
}
