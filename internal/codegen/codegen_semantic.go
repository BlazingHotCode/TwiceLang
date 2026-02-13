package codegen

import "twice/internal/ast"

// semanticCheck performs a fast validation pass before codegen so we can
// surface type-declaration issues early and avoid noisy downstream errors.
func (cg *CodeGen) semanticCheck(program *ast.Program) {
	if program == nil {
		return
	}
	for _, st := range program.Statements {
		cg.semanticCheckStatement(st)
	}
}

func (cg *CodeGen) semanticCheckStatement(stmt ast.Statement) {
	switch s := stmt.(type) {
	case *ast.LetStatement:
		if s.Value != nil {
			cg.semanticCheckExpression(s.Value)
		}
	case *ast.ConstStatement:
		if s.Value != nil {
			cg.semanticCheckExpression(s.Value)
		}
	case *ast.TypeDeclStatement:
		if s.TypeName != "" {
			if resolved, ok := cg.normalizeTypeName(s.TypeName); !ok || !cg.isKnownTypeName(resolved) {
				cg.addNodeError("unknown type: "+s.TypeName, s)
			}
		}
	case *ast.AssignStatement:
		if s.Value != nil {
			cg.semanticCheckExpression(s.Value)
		}
	case *ast.IndexAssignStatement:
		if s.Left != nil {
			cg.semanticCheckExpression(s.Left)
		}
		if s.Value != nil {
			cg.semanticCheckExpression(s.Value)
		}
	case *ast.ReturnStatement:
		if s.ReturnValue != nil {
			cg.semanticCheckExpression(s.ReturnValue)
		}
	case *ast.ExpressionStatement:
		if s.Expression != nil {
			cg.semanticCheckExpression(s.Expression)
		}
	case *ast.FunctionStatement:
		if s.Function != nil {
			if s.Function.Body != nil {
				for _, nested := range s.Function.Body.Statements {
					cg.semanticCheckStatement(nested)
				}
			}
		}
	case *ast.BlockStatement:
		for _, nested := range s.Statements {
			cg.semanticCheckStatement(nested)
		}
	case *ast.WhileStatement:
		cg.semanticCheckExpression(s.Condition)
		if s.Body != nil {
			for _, nested := range s.Body.Statements {
				cg.semanticCheckStatement(nested)
			}
		}
	case *ast.LoopStatement:
		if s.Body != nil {
			for _, nested := range s.Body.Statements {
				cg.semanticCheckStatement(nested)
			}
		}
	case *ast.ForStatement:
		if s.Init != nil {
			cg.semanticCheckStatement(s.Init)
		}
		if s.Condition != nil {
			cg.semanticCheckExpression(s.Condition)
		}
		if s.Periodic != nil {
			cg.semanticCheckStatement(s.Periodic)
		}
		if s.Body != nil {
			for _, nested := range s.Body.Statements {
				cg.semanticCheckStatement(nested)
			}
		}
	}
}

func (cg *CodeGen) semanticCheckExpression(expr ast.Expression) {
	switch e := expr.(type) {
	case *ast.ArrayLiteral:
		for _, el := range e.Elements {
			cg.semanticCheckExpression(el)
		}
	case *ast.TupleLiteral:
		for _, el := range e.Elements {
			cg.semanticCheckExpression(el)
		}
	case *ast.PrefixExpression:
		cg.semanticCheckExpression(e.Right)
	case *ast.InfixExpression:
		cg.semanticCheckExpression(e.Left)
		cg.semanticCheckExpression(e.Right)
	case *ast.IfExpression:
		cg.semanticCheckExpression(e.Condition)
		if e.Consequence != nil {
			for _, st := range e.Consequence.Statements {
				cg.semanticCheckStatement(st)
			}
		}
		if e.Alternative != nil {
			for _, st := range e.Alternative.Statements {
				cg.semanticCheckStatement(st)
			}
		}
	case *ast.FunctionLiteral:
		if e.Body != nil {
			for _, st := range e.Body.Statements {
				cg.semanticCheckStatement(st)
			}
		}
	case *ast.CallExpression:
		cg.semanticCheckExpression(e.Function)
		for _, a := range e.Arguments {
			cg.semanticCheckExpression(a)
		}
	case *ast.IndexExpression:
		cg.semanticCheckExpression(e.Left)
		cg.semanticCheckExpression(e.Index)
	case *ast.MethodCallExpression:
		cg.semanticCheckExpression(e.Object)
		for _, a := range e.Arguments {
			cg.semanticCheckExpression(a)
		}
	case *ast.TupleAccessExpression:
		cg.semanticCheckExpression(e.Left)
	case *ast.NamedArgument:
		cg.semanticCheckExpression(e.Value)
	}
}
