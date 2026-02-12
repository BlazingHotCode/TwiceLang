package codegen

import (
	"strings"
	"testing"

	"twice/internal/lexer"
	"twice/internal/parser"
)

func TestGenerateIncludesPrintCall(t *testing.T) {
	asm := generateAssembly(t, "print(123);")
	if !strings.Contains(asm, "call print_int") {
		t.Fatalf("expected assembly to call print_int, got:\n%s", asm)
	}
}

func TestGenerateReturnSetsExitCodeAndJumpsToExit(t *testing.T) {
	asm := generateAssembly(t, "return 7;")
	if !strings.Contains(asm, "mov %rax, %rdi") {
		t.Fatalf("expected return to move result into exit register, got:\n%s", asm)
	}
	if !strings.Contains(asm, "jmp .L") {
		t.Fatalf("expected return to jump to exit label, got:\n%s", asm)
	}
}

func generateAssembly(t *testing.T, input string) string {
	t.Helper()
	p := parser.New(lexer.New(input))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	cg := New()
	return cg.Generate(program)
}
