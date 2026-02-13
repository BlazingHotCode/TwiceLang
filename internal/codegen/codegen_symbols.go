package codegen

import "twice/internal/ast"

type symbolScope struct {
	values map[string]bool
	types  map[string]struct{}
}

func (cg *CodeGen) symbolCheck(program *ast.Program) {
	if program == nil {
		return
	}
	scopes := []symbolScope{{values: map[string]bool{}, types: map[string]struct{}{}}}
	for _, st := range program.Statements {
		cg.symbolCheckStatement(st, &scopes)
	}
}

func (cg *CodeGen) pushSymbolScope(scopes *[]symbolScope) {
	*scopes = append(*scopes, symbolScope{values: map[string]bool{}, types: map[string]struct{}{}})
}

func (cg *CodeGen) popSymbolScope(scopes *[]symbolScope) {
	if len(*scopes) > 1 {
		*scopes = (*scopes)[:len(*scopes)-1]
	}
}

func (cg *CodeGen) declareValue(name string, isConst bool, node ast.Node, scopes *[]symbolScope) {
	if name == "" {
		return
	}
	curr := &(*scopes)[len(*scopes)-1]
	if wasConst, ok := curr.values[name]; ok {
		if wasConst {
			cg.addNodeError("cannot reassign const: "+name, node)
		} else {
			cg.addNodeError("identifier already declared: "+name, node)
		}
		return
	}
	curr.values[name] = isConst
}

func (cg *CodeGen) declareType(name string, node ast.Node, scopes *[]symbolScope) {
	if name == "" {
		return
	}
	curr := &(*scopes)[len(*scopes)-1]
	if _, ok := curr.types[name]; ok {
		cg.addNodeError("type already declared: "+name, node)
		return
	}
	curr.types[name] = struct{}{}
}

func (cg *CodeGen) symbolCheckStatement(stmt ast.Statement, scopes *[]symbolScope) {
	switch s := stmt.(type) {
	case *ast.LetStatement:
		if s != nil && s.Name != nil {
			cg.declareValue(s.Name.Value, false, s, scopes)
		}
		if s != nil && s.Value != nil {
			cg.symbolCheckExpression(s.Value, scopes)
		}
	case *ast.ConstStatement:
		if s != nil && s.Name != nil {
			cg.declareValue(s.Name.Value, true, s, scopes)
		}
		if s != nil && s.Value != nil {
			cg.symbolCheckExpression(s.Value, scopes)
		}
	case *ast.FunctionStatement:
		if s != nil && s.Name != nil {
			cg.declareValue(s.Name.Value, false, s, scopes)
		}
		if s != nil && s.Function != nil && s.Function.Body != nil {
			cg.pushSymbolScope(scopes)
			for _, p := range s.Function.Parameters {
				if p != nil && p.Name != nil {
					cg.declareValue(p.Name.Value, false, p.Name, scopes)
				}
			}
			for _, st := range s.Function.Body.Statements {
				cg.symbolCheckStatement(st, scopes)
			}
			cg.popSymbolScope(scopes)
		}
	case *ast.TypeDeclStatement:
		if s != nil && s.Name != nil {
			cg.declareType(s.Name.Value, s, scopes)
		}
	case *ast.AssignStatement:
		if s != nil && s.Value != nil {
			cg.symbolCheckExpression(s.Value, scopes)
		}
	case *ast.IndexAssignStatement:
		if s != nil && s.Left != nil {
			cg.symbolCheckExpression(s.Left, scopes)
		}
		if s != nil && s.Value != nil {
			cg.symbolCheckExpression(s.Value, scopes)
		}
	case *ast.ExpressionStatement:
		if s != nil && s.Expression != nil {
			cg.symbolCheckExpression(s.Expression, scopes)
		}
	case *ast.ReturnStatement:
		if s != nil && s.ReturnValue != nil {
			cg.symbolCheckExpression(s.ReturnValue, scopes)
		}
	case *ast.BlockStatement:
		cg.pushSymbolScope(scopes)
		for _, st := range s.Statements {
			cg.symbolCheckStatement(st, scopes)
		}
		cg.popSymbolScope(scopes)
	case *ast.WhileStatement:
		cg.symbolCheckExpression(s.Condition, scopes)
		if s.Body != nil {
			cg.pushSymbolScope(scopes)
			for _, st := range s.Body.Statements {
				cg.symbolCheckStatement(st, scopes)
			}
			cg.popSymbolScope(scopes)
		}
	case *ast.LoopStatement:
		if s.Body != nil {
			cg.pushSymbolScope(scopes)
			for _, st := range s.Body.Statements {
				cg.symbolCheckStatement(st, scopes)
			}
			cg.popSymbolScope(scopes)
		}
	case *ast.ForStatement:
		cg.pushSymbolScope(scopes)
		if s.Init != nil {
			cg.symbolCheckStatement(s.Init, scopes)
		}
		if s.Condition != nil {
			cg.symbolCheckExpression(s.Condition, scopes)
		}
		if s.Periodic != nil {
			cg.symbolCheckStatement(s.Periodic, scopes)
		}
		if s.Body != nil {
			for _, st := range s.Body.Statements {
				cg.symbolCheckStatement(st, scopes)
			}
		}
		cg.popSymbolScope(scopes)
	}
}

func (cg *CodeGen) symbolCheckExpression(expr ast.Expression, scopes *[]symbolScope) {
	switch e := expr.(type) {
	case *ast.ArrayLiteral:
		for _, el := range e.Elements {
			cg.symbolCheckExpression(el, scopes)
		}
	case *ast.TupleLiteral:
		for _, el := range e.Elements {
			cg.symbolCheckExpression(el, scopes)
		}
	case *ast.PrefixExpression:
		cg.symbolCheckExpression(e.Right, scopes)
	case *ast.InfixExpression:
		cg.symbolCheckExpression(e.Left, scopes)
		cg.symbolCheckExpression(e.Right, scopes)
	case *ast.IfExpression:
		cg.symbolCheckExpression(e.Condition, scopes)
		if e.Consequence != nil {
			cg.symbolCheckStatement(e.Consequence, scopes)
		}
		if e.Alternative != nil {
			cg.symbolCheckStatement(e.Alternative, scopes)
		}
	case *ast.CallExpression:
		cg.symbolCheckExpression(e.Function, scopes)
		for _, a := range e.Arguments {
			cg.symbolCheckExpression(a, scopes)
		}
	case *ast.IndexExpression:
		cg.symbolCheckExpression(e.Left, scopes)
		cg.symbolCheckExpression(e.Index, scopes)
	case *ast.MethodCallExpression:
		cg.symbolCheckExpression(e.Object, scopes)
		for _, a := range e.Arguments {
			cg.symbolCheckExpression(a, scopes)
		}
	case *ast.TupleAccessExpression:
		cg.symbolCheckExpression(e.Left, scopes)
	case *ast.NamedArgument:
		cg.symbolCheckExpression(e.Value, scopes)
	}
}
