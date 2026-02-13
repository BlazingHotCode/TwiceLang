package codegen

import (
	"fmt"
	"strings"

	"twice/internal/ast"
	"twice/internal/token"
)

func (cg *CodeGen) addError(msg string) {
	cg.errors = append(cg.errors, CodegenError{Message: msg})
}

func (cg *CodeGen) addNodeError(msg string, node ast.Node) {
	ctx := ""
	line := 0
	col := 0
	if node != nil {
		ctx = strings.TrimSpace(node.String())
		if ctx == "" {
			ctx = strings.TrimSpace(node.TokenLiteral())
		}
		if tok, ok := tokenFromNode(node); ok {
			line = tok.Line
			col = tok.Column
		}
	}
	cg.errors = append(cg.errors, CodegenError{
		Message: msg,
		Context: ctx,
		Line:    line,
		Column:  col,
	})
}

func (cg *CodeGen) failNode(msg string, node ast.Node) {
	cg.addNodeError(msg, node)
	cg.emit("    mov $0, %%rax")
}

func (cg *CodeGen) failNodef(node ast.Node, format string, args ...interface{}) {
	cg.failNode(fmt.Sprintf(format, args...), node)
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
	case *ast.TupleAccessExpression:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	case *ast.NamedArgument:
		return n.Token, n.Token.Line > 0 && n.Token.Column > 0
	default:
		return token.Token{}, false
	}
}

func (cg *CodeGen) Errors() []string {
	formatted := make([]string, 0, len(cg.errors))
	for _, err := range cg.errors {
		if err.Context == "" {
			formatted = append(formatted, err.Message)
			continue
		}
		formatted = append(formatted, fmt.Sprintf("%s (at `%s`)", err.Message, err.Context))
	}
	return formatted
}

func (cg *CodeGen) DetailedErrors() []CodegenError {
	out := make([]CodegenError, len(cg.errors))
	copy(out, cg.errors)
	return out
}
