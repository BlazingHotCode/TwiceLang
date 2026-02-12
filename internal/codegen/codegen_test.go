package codegen

import (
	"strings"
	"testing"

	"twice/internal/lexer"
	"twice/internal/parser"
)

func TestGenerateIncludesPrintCall(t *testing.T) {
	asm, _ := generateAssembly(t, "print(123);")
	if !strings.Contains(asm, "call print_int") {
		t.Fatalf("expected assembly to call print_int, got:\n%s", asm)
	}
}

func TestGenerateReturnSetsExitCodeAndJumpsToExit(t *testing.T) {
	asm, _ := generateAssembly(t, "return 7;")
	if !strings.Contains(asm, "mov %rax, %rdi") {
		t.Fatalf("expected return to move result into exit register, got:\n%s", asm)
	}
	if !strings.Contains(asm, "jmp .L") {
		t.Fatalf("expected return to jump to exit label, got:\n%s", asm)
	}
}

func TestPrintIntSupportsSignedNumbers(t *testing.T) {
	asm, _ := generateAssembly(t, "print(-42);")
	if !strings.Contains(asm, "neg %rcx") {
		t.Fatalf("expected sign normalization in print_int, got:\n%s", asm)
	}
	if !strings.Contains(asm, "movb $45, (%rsi)") {
		t.Fatalf("expected '-' emission in print_int, got:\n%s", asm)
	}
}

func TestPrintArgumentValidation(t *testing.T) {
	_, cg := generateAssembly(t, "print(true);")
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected codegen errors for print(true), got none")
	}
}

func TestPrintArityValidation(t *testing.T) {
	_, cg := generateAssembly(t, "print();")
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected codegen errors for print(), got none")
	}
}

func TestConstCodegen(t *testing.T) {
	asm, _ := generateAssembly(t, "const x = 3; print(x);")
	if !strings.Contains(asm, "# const x") {
		t.Fatalf("expected const declaration emission, got:\n%s", asm)
	}
}

func TestConstRedeclarationValidation(t *testing.T) {
	_, cg := generateAssembly(t, "const x = 1; let x = 2;")
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected codegen errors for const redeclaration, got none")
	}
}

func TestAssignmentCodegen(t *testing.T) {
	asm, _ := generateAssembly(t, "let x = 1; x = 2;")
	if !strings.Contains(asm, "# assign x") {
		t.Fatalf("expected assignment emission, got:\n%s", asm)
	}
}

func TestAssignmentConstValidation(t *testing.T) {
	_, cg := generateAssembly(t, "const x = 1; x = 2;")
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected codegen errors for const reassignment, got none")
	}
}

func generateAssembly(t *testing.T, input string) (string, *CodeGen) {
	t.Helper()
	p := parser.New(lexer.New(input))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	cg := New()
	return cg.Generate(program), cg
}
