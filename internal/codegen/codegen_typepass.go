package codegen

import "twice/internal/ast"

// annotateTypesPass walks the AST and pre-fills inference caches.
func (cg *CodeGen) annotateTypesPass(program *ast.Program) {
	if program == nil {
		return
	}
	for _, st := range program.Statements {
		cg.annotateStatementTypes(st)
	}
}

func (cg *CodeGen) annotateStatementTypes(stmt ast.Statement) {
	switch s := stmt.(type) {
	case *ast.LetStatement:
		if s.Value != nil {
			cg.annotateExpressionTypes(s.Value)
		}
	case *ast.ConstStatement:
		if s.Value != nil {
			cg.annotateExpressionTypes(s.Value)
		}
	case *ast.AssignStatement:
		if s.Value != nil {
			cg.annotateExpressionTypes(s.Value)
		}
	case *ast.IndexAssignStatement:
		if s.Left != nil {
			cg.annotateExpressionTypes(s.Left)
		}
		if s.Value != nil {
			cg.annotateExpressionTypes(s.Value)
		}
	case *ast.MemberAssignStatement:
		if s.Left != nil {
			cg.annotateExpressionTypes(s.Left)
		}
		if s.Value != nil {
			cg.annotateExpressionTypes(s.Value)
		}
	case *ast.StructStatement:
		for _, f := range s.Fields {
			if f != nil && f.DefaultValue != nil {
				cg.annotateExpressionTypes(f.DefaultValue)
			}
		}
	case *ast.ReturnStatement:
		if s.ReturnValue != nil {
			cg.annotateExpressionTypes(s.ReturnValue)
		}
	case *ast.ExpressionStatement:
		if s.Expression != nil {
			cg.annotateExpressionTypes(s.Expression)
		}
	case *ast.BlockStatement:
		for _, st := range s.Statements {
			cg.annotateStatementTypes(st)
		}
	case *ast.WhileStatement:
		cg.annotateExpressionTypes(s.Condition)
		if s.Body != nil {
			cg.annotateStatementTypes(s.Body)
		}
	case *ast.LoopStatement:
		if s.Body != nil {
			cg.annotateStatementTypes(s.Body)
		}
	case *ast.ForStatement:
		if s.Init != nil {
			cg.annotateStatementTypes(s.Init)
		}
		if s.Condition != nil {
			cg.annotateExpressionTypes(s.Condition)
		}
		if s.Periodic != nil {
			cg.annotateStatementTypes(s.Periodic)
		}
		if s.Body != nil {
			cg.annotateStatementTypes(s.Body)
		}
	case *ast.FunctionStatement:
		if s.Function != nil && s.Function.Body != nil {
			cg.annotateStatementTypes(s.Function.Body)
		}
	}
}

func (cg *CodeGen) annotateExpressionTypes(expr ast.Expression) {
	if expr == nil {
		return
	}
	_ = cg.inferExpressionType(expr)
	_ = cg.inferExpressionTypeName(expr)
	switch e := expr.(type) {
	case *ast.ArrayLiteral:
		for _, el := range e.Elements {
			cg.annotateExpressionTypes(el)
		}
	case *ast.TupleLiteral:
		for _, el := range e.Elements {
			cg.annotateExpressionTypes(el)
		}
	case *ast.PrefixExpression:
		cg.annotateExpressionTypes(e.Right)
	case *ast.InfixExpression:
		cg.annotateExpressionTypes(e.Left)
		cg.annotateExpressionTypes(e.Right)
	case *ast.IfExpression:
		cg.annotateExpressionTypes(e.Condition)
		if e.Consequence != nil {
			cg.annotateStatementTypes(e.Consequence)
		}
		if e.Alternative != nil {
			cg.annotateStatementTypes(e.Alternative)
		}
	case *ast.CallExpression:
		cg.annotateExpressionTypes(e.Function)
		for _, a := range e.Arguments {
			cg.annotateExpressionTypes(a)
		}
	case *ast.IndexExpression:
		cg.annotateExpressionTypes(e.Left)
		cg.annotateExpressionTypes(e.Index)
	case *ast.MethodCallExpression:
		cg.annotateExpressionTypes(e.Object)
		for _, a := range e.Arguments {
			cg.annotateExpressionTypes(a)
		}
	case *ast.MemberAccessExpression:
		cg.annotateExpressionTypes(e.Object)
	case *ast.NullSafeAccessExpression:
		cg.annotateExpressionTypes(e.Object)
	case *ast.TupleAccessExpression:
		cg.annotateExpressionTypes(e.Left)
	case *ast.NewExpression:
		for _, a := range e.Arguments {
			cg.annotateExpressionTypes(a)
		}
	case *ast.NamedArgument:
		cg.annotateExpressionTypes(e.Value)
	}
}
