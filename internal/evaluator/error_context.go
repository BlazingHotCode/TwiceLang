package evaluator

import (
	"strings"

	"twice/internal/ast"
	"twice/internal/object"
	"twice/internal/token"
)

func annotateErrorWithNode(obj object.Object, node ast.Node) object.Object {
	err, ok := obj.(*object.Error)
	if !ok || node == nil {
		return obj
	}
	if err.Line > 0 && err.Column > 0 && strings.TrimSpace(err.Context) != "" {
		return obj
	}
	ctx := strings.TrimSpace(node.String())
	if ctx == "" {
		ctx = strings.TrimSpace(node.TokenLiteral())
	}
	if err.Context == "" {
		err.Context = ctx
	}
	if tok, ok := tokenFromNode(node); ok {
		if err.Line <= 0 {
			err.Line = tok.Line
		}
		if err.Column <= 0 {
			err.Column = tok.Column
		}
	}
	return err
}

func tokenFromNode(node ast.Node) (token.Token, bool) {
	switch n := node.(type) {
	case *ast.Identifier:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.IntegerLiteral:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.FloatLiteral:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.StringLiteral:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.CharLiteral:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.NullLiteral:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.ArrayLiteral:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.TupleLiteral:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.LetStatement:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.ConstStatement:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.TypeDeclStatement:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.AssignStatement:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.IndexAssignStatement:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.ReturnStatement:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.ExpressionStatement:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.WhileStatement:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.LoopStatement:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.ForStatement:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.ForeachStatement:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.BreakStatement:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.ContinueStatement:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.Boolean:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.PrefixExpression:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.InfixExpression:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.IfExpression:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.BlockStatement:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.FunctionLiteral:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.FunctionStatement:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.CallExpression:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.IndexExpression:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.MethodCallExpression:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.MemberAccessExpression:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.NullSafeAccessExpression:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.TupleAccessExpression:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.NamedArgument:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	default:
		return token.Token{}, false
	}
}
