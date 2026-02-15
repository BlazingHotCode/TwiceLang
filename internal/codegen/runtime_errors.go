package codegen

import (
	"fmt"
	"strings"

	"twice/internal/ast"
)

func (cg *CodeGen) runtimeErrorText(node ast.Node, message string) string {
	text := fmt.Sprintf("Runtime error: %s", message)
	if tok, ok := tokenFromNode(node); ok {
		text += fmt.Sprintf(" (at %d:%d)", tok.Line, tok.Column)
	}
	ctx := ""
	if node != nil {
		ctx = strings.TrimSpace(node.String())
		if ctx == "" {
			ctx = strings.TrimSpace(node.TokenLiteral())
		}
	}
	if ctx != "" {
		if len(ctx) > 140 {
			ctx = ctx[:140] + "..."
		}
		text += fmt.Sprintf(" | context: %s", ctx)
	}
	return text
}

func (cg *CodeGen) emitRuntimeFail(node ast.Node, message string) {
	label := cg.stringLabel(cg.runtimeErrorText(node, message))
	cg.emit("    lea %s(%%rip), %%rax", label)
	cg.emit("    call runtime_fail")
}
