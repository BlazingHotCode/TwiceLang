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

func TestPrintBooleanSupported(t *testing.T) {
	asm, cg := generateAssembly(t, "print(true);")
	if len(cg.Errors()) != 0 {
		t.Fatalf("expected no codegen errors for print(true), got: %v", cg.Errors())
	}
	if !strings.Contains(asm, "call print_bool") {
		t.Fatalf("expected assembly to call print_bool, got:\n%s", asm)
	}
}

func TestPrintArityValidation(t *testing.T) {
	_, cg := generateAssembly(t, "print();")
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected codegen errors for print(), got none")
	}
}

func TestPrintUnsupportedTypeValidation(t *testing.T) {
	_, cg := generateAssembly(t, "print(fn(x) { x; });")
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected codegen errors for print(function), got none")
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

func TestUndefinedIdentifierCodegenValidation(t *testing.T) {
	_, cg := generateAssembly(t, "print(x);")
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected codegen errors for undefined identifier, got none")
	}
	if !strings.Contains(cg.Errors()[0], "identifier not found: x") {
		t.Fatalf("missing undefined identifier diagnostic, got: %v", cg.Errors())
	}
}

func TestCodegenSupportsAdditionalPrintTypes(t *testing.T) {
	asm, cg := generateAssembly(t, `let s: string = "hi"; let c = 'a'; let f = 3.14; let n: string; print(s); print(c); print(f); print(n);`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "call print_cstr") || !strings.Contains(asm, "call print_char") {
		t.Fatalf("expected string and char print calls in assembly, got:\n%s", asm)
	}
}

func TestCodegenTypeofAndCast(t *testing.T) {
	_, cg := generateAssembly(t, "print(typeof(1)); print(int(true));")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}

	asm, cg := generateAssembly(t, "let n: string; print(typeof(n));")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "string\\n") {
		t.Fatalf("expected typeof(n) to resolve declared type string, got:\n%s", asm)
	}
}

func TestCodegenStringConcatAndFloatAdd(t *testing.T) {
	asm, cg := generateAssembly(t, `let greeting = "Hello, " + "Twice!"; let pi = 1.25 + 2.75; print(greeting); print(pi);`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "Hello, Twice!\\n") {
		t.Fatalf("expected concatenated string literal in assembly, got:\n%s", asm)
	}
	if !strings.Contains(asm, "4\\n") {
		t.Fatalf("expected folded float addition literal in assembly, got:\n%s", asm)
	}
}

func TestCodegenCharPlusCharPrintsAsChar(t *testing.T) {
	asm, cg := generateAssembly(t, "print('A' + 'B');")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "call print_char") {
		t.Fatalf("expected char+char to be printed via print_char, got:\n%s", asm)
	}
}

func TestCodegenMixedNumericOpsAndStringCoercion(t *testing.T) {
	asm, cg := generateAssembly(t, `print(1 + 2.5); print(6 - 2.5); print(3 * 2.0); print(7 / 2.0); print("x:" + 7); print("x:" + 3.5); print("x:" + 'A');`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	for _, want := range []string{"3.5\\n", "6\\n", "x:7\\n", "x:3.5\\n", "x:A\\n"} {
		if !strings.Contains(asm, want) {
			t.Fatalf("expected folded literal %q in assembly, got:\n%s", want, asm)
		}
	}
}

func TestCodegenCharPlusIntPrintsAsChar(t *testing.T) {
	asm, cg := generateAssembly(t, "print('A' + 1);")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "call print_char") {
		t.Fatalf("expected char+int to be printed via print_char, got:\n%s", asm)
	}
}

func TestCodegenBooleanOperators(t *testing.T) {
	asm, cg := generateAssembly(t, "print(true && false); print(true || false); print(true ^^ false);")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if strings.Count(asm, "call print_bool") < 3 {
		t.Fatalf("expected boolean operator results to print via print_bool, got:\n%s", asm)
	}
}

func TestCodegenBitwiseOperators(t *testing.T) {
	asm, cg := generateAssembly(t, "print(5 & 3); print(5 | 2); print(5 ^ 1); print(5 << 1); print(5 >> 1);")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if strings.Count(asm, "call print_int") < 5 {
		t.Fatalf("expected bitwise operator results to print via print_int, got:\n%s", asm)
	}
	for _, instr := range []string{"and %rcx, %rax", "or %rcx, %rax", "xor %rcx, %rax", "shl %cl, %rax", "sar %cl, %rax"} {
		if !strings.Contains(asm, instr) {
			t.Fatalf("expected instruction %q in assembly, got:\n%s", instr, asm)
		}
	}
}

func TestCodegenIntegerModulo(t *testing.T) {
	asm, cg := generateAssembly(t, "print(7 % 4);")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "idiv %rcx") || !strings.Contains(asm, "mov %rdx, %rax") {
		t.Fatalf("expected integer modulo instruction sequence, got:\n%s", asm)
	}
}

func TestCodegenFloatModuloFolding(t *testing.T) {
	asm, cg := generateAssembly(t, "print(7.5 % 2.0);")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "1.5\\n") {
		t.Fatalf("expected folded float modulo literal in assembly, got:\n%s", asm)
	}
}

func TestCodegenNamedFunctionCall(t *testing.T) {
	asm, cg := generateAssembly(t, "fn add(a: int, b: int = 2) int { return a + b; } print(add(3));")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "fn_add:") {
		t.Fatalf("expected generated function label, got:\n%s", asm)
	}
	if !strings.Contains(asm, "call fn_add") {
		t.Fatalf("expected call to generated function, got:\n%s", asm)
	}
}

func TestCodegenNamedArgumentsCall(t *testing.T) {
	_, cg := generateAssembly(t, "fn sub(a: int, b: int) int { return a - b; } print(sub(b = 2, a = 7));")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
}

func TestCodegenFunctionCallBeforeDeclaration(t *testing.T) {
	asm, cg := generateAssembly(t, "print(add(3)); fn add(a: int, b: int = 2) int { return a + b; }")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "call fn_add") {
		t.Fatalf("expected call to function declared later in file, got:\n%s", asm)
	}
}

func TestCodegenArrayIndexGetAndSet(t *testing.T) {
	asm, cg := generateAssembly(t, "let arr = {1, 2, 3}; print(arr[1]); arr[1] = 9; print(arr[1]);")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "mov (%rcx,%rax), %rax") {
		t.Fatalf("expected indexed load sequence, got:\n%s", asm)
	}
	if !strings.Contains(asm, "mov %rdx, (%rcx,%rax)") {
		t.Fatalf("expected indexed store sequence, got:\n%s", asm)
	}
}

func TestCodegenArrayTypesAndTypeof(t *testing.T) {
	asm, cg := generateAssembly(t, "let arr: int[] = {1,2,3}; print(typeof(arr)); let grid: int[][] = {{1}, {2,3}}; print(typeof(grid));")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "int[]\\n") {
		t.Fatalf("expected typeof(arr) literal int[], got:\n%s", asm)
	}
	if !strings.Contains(asm, "int[][]\\n") {
		t.Fatalf("expected typeof(grid) literal int[][], got:\n%s", asm)
	}
}

func TestCodegenArrayTypeValidation(t *testing.T) {
	_, cg := generateAssembly(t, "let arr: int[2] = {1, 2, 3};")
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected codegen errors for array length mismatch, got none")
	}

	_, cg = generateAssembly(t, "let arr = {1, 2, 3}; arr[1] = true;")
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected codegen errors for indexed assignment type mismatch, got none")
	}
}

func TestCodegenArrayLengthMethod(t *testing.T) {
	asm, cg := generateAssembly(t, "let arr = {1,2,3}; print(arr.length());")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "mov $3, %rax") {
		t.Fatalf("expected constant length load in assembly, got:\n%s", asm)
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
