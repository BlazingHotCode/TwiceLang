package codegen

import (
	"fmt"
	"reflect"
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
	v := reflect.ValueOf(node)
	if !v.IsValid() {
		return token.Token{}, false
	}
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return token.Token{}, false
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return token.Token{}, false
	}
	f := v.FieldByName("Token")
	if !f.IsValid() {
		return token.Token{}, false
	}
	tok, ok := f.Interface().(token.Token)
	if !ok {
		return token.Token{}, false
	}
	return tok, tok.Line > 0 && tok.Column > 0
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
