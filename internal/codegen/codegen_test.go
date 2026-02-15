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
	if !strings.Contains(asm, "string") {
		t.Fatalf("expected typeof(n) to resolve declared type string, got:\n%s", asm)
	}
}

func TestCodegenTypeofValueComparison(t *testing.T) {
	asm, cg := generateAssembly(t, `let a: int||string = 3; if (typeofValue(a) == int) { print("ran correctly"); };`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "ran correctly") {
		t.Fatalf("expected branch string literal in assembly, got:\n%s", asm)
	}

	_, cg = generateAssembly(t, `let a: int||string = 3; if (typeofvalue(a) == int) { print("ok"); };`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors for lowercase alias: %v", cg.Errors())
	}
}

func TestCodegenWhileLoop(t *testing.T) {
	asm, cg := generateAssembly(t, "let i = 0; while (i < 2) { i++; } print(i);")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if strings.Count(asm, "jmp .L") < 2 {
		t.Fatalf("expected while loop jumps in assembly, got:\n%s", asm)
	}
}

func TestCodegenForLoop(t *testing.T) {
	asm, cg := generateAssembly(t, "let sum = 0; for (let i = 0; i < 4; i++) { sum = sum + i; } print(sum);")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "# let i") {
		t.Fatalf("expected for init declaration in assembly, got:\n%s", asm)
	}
}

func TestCodegenBlockScopeShadowing(t *testing.T) {
	asm, cg := generateAssembly(t, "let x = 1; if (true) { let x = 2; print(x); }; print(x);")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if strings.Count(asm, "call print_int") < 2 {
		t.Fatalf("expected two print calls, got:\n%s", asm)
	}
}

func TestCodegenBlockScopeNoLeak(t *testing.T) {
	_, cg := generateAssembly(t, "if (true) { let y = 1; }; print(y);")
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected scope error for leaked block local")
	}
	if !strings.Contains(cg.Errors()[0], "identifier not found: y") {
		t.Fatalf("expected identifier-not-found for y, got: %v", cg.Errors())
	}
}

func TestCodegenBlockScopeOuterAssignmentPersists(t *testing.T) {
	_, cg := generateAssembly(t, "let x = 1; if (true) { x = 2; }; print(x);")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
}

func TestCodegenForScopeNoLeak(t *testing.T) {
	_, cg := generateAssembly(t, "for (let i = 0; i < 1; i++) { }; print(i);")
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected scope error for leaked for-init local")
	}
	if !strings.Contains(cg.Errors()[0], "identifier not found: i") {
		t.Fatalf("expected identifier-not-found for i, got: %v", cg.Errors())
	}
}

func TestCodegenStandaloneBlockScopeNoLeak(t *testing.T) {
	_, cg := generateAssembly(t, "{ let t = 1; } print(t);")
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected scope error for leaked block local")
	}
	if !strings.Contains(cg.Errors()[0], "identifier not found: t") {
		t.Fatalf("expected identifier-not-found for t, got: %v", cg.Errors())
	}
}

func TestCodegenStandaloneBlockFunctionNoLeak(t *testing.T) {
	_, cg := generateAssembly(t, "{ fn tmp() int { return 1; } print(tmp()); } print(tmp());")
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected scope error for leaked block function")
	}
	last := cg.Errors()[len(cg.Errors())-1]
	if !strings.Contains(last, "unknown function tmp") {
		t.Fatalf("expected unknown function tmp after block, got: %v", cg.Errors())
	}
}

func TestCodegenLoopLocalVarsUseFrameSlots(t *testing.T) {
	asm, cg := generateAssembly(t, "let i = 0; while (i < 3) { let tmp = i; i++; }; print(i);")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if strings.Contains(asm, "push %rax           # let tmp") {
		t.Fatalf("expected frame-slot local storage, found push-based local allocation:\n%s", asm)
	}
	if !strings.Contains(asm, "# let tmp") || !strings.Contains(asm, "mov %rax, -") {
		t.Fatalf("expected stack-slot store for loop local, got:\n%s", asm)
	}
	if !strings.Contains(asm, "sub $") {
		t.Fatalf("expected function/frame stack reservation, got:\n%s", asm)
	}
}

func TestCodegenLoopArrayLiteralsUseFrameSlots(t *testing.T) {
	asm, cg := generateAssembly(t, "let i = 0; while (i < 2) { let a = {1, 2, 3}; i++; print(a[0]); };")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if strings.Contains(asm, "push %rax           # let a") {
		t.Fatalf("expected frame-slot local array binding, found push-based local allocation:\n%s", asm)
	}
	if !strings.Contains(asm, "# let a") {
		t.Fatalf("expected let-binding comment for loop array local, got:\n%s", asm)
	}
}

func TestCodegenBreakAndContinue(t *testing.T) {
	asm, cg := generateAssembly(t, "let i = 0; while (true) { i++; if (i == 2) { continue; }; if (i == 4) { break; }; } print(i);")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if strings.Count(asm, "jmp .L") < 4 {
		t.Fatalf("expected break/continue jumps in assembly, got:\n%s", asm)
	}
}

func TestCodegenBreakContinueOutsideLoop(t *testing.T) {
	_, cg := generateAssembly(t, "break;")
	if len(cg.Errors()) == 0 || !strings.Contains(cg.Errors()[0], "break not inside loop") {
		t.Fatalf("expected break outside loop error, got: %v", cg.Errors())
	}

	_, cg = generateAssembly(t, "continue;")
	if len(cg.Errors()) == 0 || !strings.Contains(cg.Errors()[0], "continue not inside loop") {
		t.Fatalf("expected continue outside loop error, got: %v", cg.Errors())
	}
}

func TestCodegenForeachArrayAndList(t *testing.T) {
	asm, cg := generateAssembly(t, "let sum = 0; foreach (let v : {1,2,3}) { sum = sum + v; }; print(sum);")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "call print_int") {
		t.Fatalf("expected foreach array sum to print int, got:\n%s", asm)
	}

	asm, cg = generateAssembly(t, "let xs: List<int> = new List<int>(2,3,4); let sum = 0; foreach (let x : xs) { sum = sum + x; }; print(sum);")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "call list_get") {
		t.Fatalf("expected foreach list path to use list_get, got:\n%s", asm)
	}
}

func TestCodegenForeachTypeError(t *testing.T) {
	_, cg := generateAssembly(t, "foreach (let v : 1) { print(v); };")
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected codegen error for non-iterable foreach target")
	}
	found := false
	for _, err := range cg.Errors() {
		if strings.Contains(err, "foreach expects array or list iterable") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected foreach iterable type error, got: %v", cg.Errors())
	}
}

func TestCodegenStringConcatAndFloatAdd(t *testing.T) {
	asm, cg := generateAssembly(t, `let greeting = "Hello, " + "Twice!"; let pi = 1.25 + 2.75; print(greeting); print(pi);`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "Hello, Twice!") {
		t.Fatalf("expected concatenated string literal in assembly, got:\n%s", asm)
	}
	if !strings.Contains(asm, "4") {
		t.Fatalf("expected folded float addition literal in assembly, got:\n%s", asm)
	}
}

func TestCodegenRuntimeIntStringConcat(t *testing.T) {
	asm, cg := generateAssembly(t, `let n = 2; print(n + " loop count");`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "call concat_int_cstr") {
		t.Fatalf("expected int+string runtime concat helper call, got:\n%s", asm)
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
	for _, want := range []string{"3.5", "6", "x:7", "x:3.5", "x:A"} {
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
	if !strings.Contains(asm, "1.5") {
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

func TestCodegenFunctionEmptyReturn(t *testing.T) {
	asm, cg := generateAssembly(t, `fn noop() { return; } print(noop());`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors for empty return: %v", cg.Errors())
	}
	if !strings.Contains(asm, "lea null_lit(%rip), %rax") {
		t.Fatalf("expected empty return to load null literal, got:\n%s", asm)
	}
}

func TestCodegenFunctionEmptyReturnTypeValidation(t *testing.T) {
	_, cg := generateAssembly(t, `fn bad() int { return; }`)
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected codegen error for empty return in int function")
	}
	found := false
	for _, err := range cg.Errors() {
		if strings.Contains(err, "cannot return null from function returning int") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected null-return type validation error, got: %v", cg.Errors())
	}

	_, cg = generateAssembly(t, `fn ok() int||null { return; }`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors for int||null empty return: %v", cg.Errors())
	}

	_, cg = generateAssembly(t, `fn ok2() string||null { return; }`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors for string||null empty return: %v", cg.Errors())
	}
}

func TestCodegenUnknownReturnTypeDoesNotCascadeReturnErrors(t *testing.T) {
	const invalidReturnType = "int||__invalid_type__"
	_, cg := generateAssembly(t, `fn main() int||__invalid_type__ { return; }`)
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected codegen errors for unknown return type")
	}
	unknownCount := 0
	returnMismatchCount := 0
	for _, err := range cg.Errors() {
		if strings.Contains(err, "unknown type: "+invalidReturnType) {
			unknownCount++
		}
		if strings.Contains(err, "cannot return null from function returning "+invalidReturnType) {
			returnMismatchCount++
		}
	}
	if unknownCount == 0 {
		t.Fatalf("expected unknown type error, got: %v", cg.Errors())
	}
	if returnMismatchCount != 0 {
		t.Fatalf("did not expect return mismatch error for unknown return type, got: %v", cg.Errors())
	}
}

func TestCodegenAutoRunsMainWithIntExitCode(t *testing.T) {
	asm, cg := generateAssembly(t, `fn main() int { return 7; }`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "call fn_main") {
		t.Fatalf("expected auto entrypoint call to fn_main, got:\n%s", asm)
	}
	if !strings.Contains(asm, "mov %rax, %rdi") {
		t.Fatalf("expected int-return main to map return value to exit code, got:\n%s", asm)
	}
}

func TestCodegenAutoRunsMainNoReturnAsZeroExitCode(t *testing.T) {
	asm, cg := generateAssembly(t, `fn main() { return; }`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "call fn_main") {
		t.Fatalf("expected auto entrypoint call to fn_main, got:\n%s", asm)
	}
	if !strings.Contains(asm, "xor %rdi, %rdi") {
		t.Fatalf("expected no-return main to exit with 0, got:\n%s", asm)
	}
}

func TestCodegenNestedFunctionReturnInIntMain(t *testing.T) {
	_, cg := generateAssembly(t, `
fn main() int {
  {
    fn tempFn(x: int) int {
      return x + 1;
    }
    print(tempFn(10));
  }
  return 0;
}
`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors for nested int-return function in int main: %v", cg.Errors())
	}
}

func TestCodegenDoesNotAutoRunMainWhenExplicitlyCalled(t *testing.T) {
	asm, cg := generateAssembly(t, `fn main() { return; } main();`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if strings.Count(asm, "call fn_main") != 1 {
		t.Fatalf("expected exactly one call to fn_main (explicit call only), got:\n%s", asm)
	}
}

func TestCodegenTopLevelFunctionsDoNotCaptureEachOther(t *testing.T) {
	_, cg := generateAssembly(t, `
fn header(name: string) { print(name); return; }
fn helper() { header("ok"); return; }
fn main() { helper(); return; }
`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors for top-level function calls: %v", cg.Errors())
	}
}

func TestCodegenRuntimeStringStringConcat(t *testing.T) {
	asm, cg := generateAssembly(t, `
fn header(name: string) { print("--- " + name + " ---"); return; }
fn main() { header("ok"); return; }
`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors for runtime string+string concat: %v", cg.Errors())
	}
	if !strings.Contains(asm, "call concat_cstr_cstr") {
		t.Fatalf("expected runtime cstr+cstr helper call, got:\n%s", asm)
	}
}

func TestCodegenTemplateStringConcat(t *testing.T) {
	asm, cg := generateAssembly(t, `
fn main() {
  let name = "Twice";
  print(`+"`"+`Hello ${name}\n`+"`"+`);
  return;
}
`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors for template concat: %v", cg.Errors())
	}
	if !strings.Contains(asm, "call concat_cstr_cstr") && !strings.Contains(asm, "Hello Twice\\n") {
		t.Fatalf("expected template string lowering via concat helper or folded literal, got:\n%s", asm)
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

func TestCodegenFunctionCallSupportsMoreThanSixParameters(t *testing.T) {
	asm, cg := generateAssembly(t, `
fn sum7(a: int, b: int, c: int, d: int, e: int, f: int, g: int) int {
  return a + b + c + d + e + f + g;
}
print(sum7(1, 2, 3, 4, 5, 6, 7));
`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "call fn_sum7") {
		t.Fatalf("expected call to compiled sum7 function, got:\n%s", asm)
	}
	if !strings.Contains(asm, "add $56, %rsp") {
		t.Fatalf("expected stack cleanup for 7 call arguments, got:\n%s", asm)
	}
}

func TestCodegenFunctionCallSupportsMoreThanSixParamsAndCaptures(t *testing.T) {
	asm, cg := generateAssembly(t, `
let a = 1;
let b = 2;
let c = 3;
let d = 4;
let e = 5;
let f = fn(x: int, y: int) int {
  return a + b + c + d + e + x + y;
};
print(f(6, 7));
`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "call fn_lit_") {
		t.Fatalf("expected call to compiled function literal, got:\n%s", asm)
	}
	if !strings.Contains(asm, "add $56, %rsp") {
		t.Fatalf("expected stack cleanup for 7 total params/captures, got:\n%s", asm)
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

func TestCodegenRuntimeArrayBoundsCheckForDynamicIndex(t *testing.T) {
	asm, cg := generateAssembly(t, "fn pick(i: int) int { let arr: int[3] = {1,2,3}; return arr[i]; } print(pick(5));")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "call runtime_fail") {
		t.Fatalf("expected runtime bounds-check trap for dynamic array index, got:\n%s", asm)
	}
	if !strings.Contains(asm, "Runtime error: array index out of bounds") {
		t.Fatalf("expected runtime bounds-check error text in assembly, got:\n%s", asm)
	}
}

func TestCodegenRuntimeStringBoundsCheckForDynamicIndex(t *testing.T) {
	asm, cg := generateAssembly(t, "fn pick(i: int) char { let s = \"abc\"; return s[i]; } print(pick(9));")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "call runtime_fail") {
		t.Fatalf("expected runtime bounds-check trap for dynamic string index, got:\n%s", asm)
	}
	if !strings.Contains(asm, "Runtime error: string index out of bounds") {
		t.Fatalf("expected runtime bounds-check error text in assembly, got:\n%s", asm)
	}
}

func TestCodegenArrayTypesAndTypeof(t *testing.T) {
	asm, cg := generateAssembly(t, "let arr: int[3] = {1,2,3}; print(typeof(arr)); let grid: int[2][2] = {{1,2}, {2,3}}; print(typeof(grid));")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "int[3]") {
		t.Fatalf("expected typeof(arr) literal int[3], got:\n%s", asm)
	}
	if !strings.Contains(asm, "int[2][2]") {
		t.Fatalf("expected typeof(grid) literal int[2][2], got:\n%s", asm)
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

func TestCodegenNullishCoalesce(t *testing.T) {
	asm, cg := generateAssembly(t, "let x: int; x ?? 7; 3 ?? 7;")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "lea null_lit(%rip), %rcx") || !strings.Contains(asm, "je .L") {
		t.Fatalf("expected null-coalescing compare/jump sequence, got:\n%s", asm)
	}
}

func TestCodegenNullishCoalesceWorksInPrint(t *testing.T) {
	asm, cg := generateAssembly(t, "let x: int; print(x ?? 7); print(3 ?? 7);")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if strings.Count(asm, "call print_int") < 2 {
		t.Fatalf("expected coalesced values to print as ints, got:\n%s", asm)
	}
}

func TestCodegenNullSafeMethodCall(t *testing.T) {
	asm, cg := generateAssembly(t, "let arr: int[3]; print(arr?.length()); let arr2 = {1,2,3}; print(arr2?.length());")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "mov $3, %rax") {
		t.Fatalf("expected non-null length path to materialize array length, got:\n%s", asm)
	}
}

func TestCodegenMemberAccess(t *testing.T) {
	asm, cg := generateAssembly(t, "let arr = {1,2,3}; print(arr.length);")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "mov $3, %rax") {
		t.Fatalf("expected constant length load in assembly, got:\n%s", asm)
	}
}

func TestCodegenNullSafeAccess(t *testing.T) {
	asm, cg := generateAssembly(t, "let arr: int[3]; print(arr?.length); let arr2 = {1,2,3}; print(arr2?.length);")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "mov $3, %rax") {
		t.Fatalf("expected non-null length path to materialize array length, got:\n%s", asm)
	}
}

func TestCodegenEmptyTypedLiterals(t *testing.T) {
	asm, cg := generateAssembly(t, "let arr: int[3] = {}; print(arr.length()); let t: (int,string) = (); print(t.0 ?? 7);")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "mov $3, %rax") {
		t.Fatalf("expected typed empty array to preserve declared length, got:\n%s", asm)
	}
}

func TestCodegenNullSafeMissingMemberReturnsNull(t *testing.T) {
	asm, cg := generateAssembly(t, "let arr = {1,2,3}; print(arr?.missing); let x = 1; print(x?.missing);")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if strings.Count(asm, "lea null_lit(%rip), %rax") < 2 {
		t.Fatalf("expected null-safe missing members to materialize null, got:\n%s", asm)
	}
}

func TestCodegenNullSafeMissingMethodReturnsNull(t *testing.T) {
	asm, cg := generateAssembly(t, "let arr = {1,2,3}; print(arr?.missing()); let x = 1; print(x?.missing());")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if strings.Count(asm, "lea null_lit(%rip), %rax") < 2 {
		t.Fatalf("expected null-safe missing methods to materialize null, got:\n%s", asm)
	}
}

func TestCodegenNullSafeAccessNullableArrayUnion(t *testing.T) {
	asm, cg := generateAssembly(t, "let x: int[3]||null = {1,2,3}; print(x?.length); x = null; print(x?.length ?? 0);")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if strings.Count(asm, "mov $3, %rax") < 1 {
		t.Fatalf("expected nullable array union null-safe length to resolve array length, got:\n%s", asm)
	}
}

func TestCodegenNullSafeMethodCallNullableArrayUnion(t *testing.T) {
	asm, cg := generateAssembly(t, "let x: int[3]||null = {1,2,3}; print(x?.length()); x = null; print(x?.length() ?? 0);")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if strings.Count(asm, "mov $3, %rax") < 1 {
		t.Fatalf("expected nullable array union null-safe length() to resolve array length, got:\n%s", asm)
	}
}

func TestCodegenMethodCallErrors(t *testing.T) {
	_, cg := generateAssembly(t, "let s = \"abc\"; s.length();")
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected codegen error for length() on non-array")
	}
	if !strings.Contains(cg.Errors()[0], "length is only supported on arrays/lists/maps") {
		t.Fatalf("expected non-array length error, got: %v", cg.Errors())
	}

	_, cg = generateAssembly(t, "let arr = {1,2,3}; arr.length(1);")
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected codegen error for length() arity")
	}
	if !strings.Contains(cg.Errors()[0], "length expects 0 arguments, got=1") {
		t.Fatalf("expected length arity error, got: %v", cg.Errors())
	}

	_, cg = generateAssembly(t, "let arr = {1,2,3}; arr.missing();")
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected codegen error for unknown method")
	}
	if !strings.Contains(cg.Errors()[0], "unknown method: missing") {
		t.Fatalf("expected unknown method error, got: %v", cg.Errors())
	}
}

func TestCodegenMapConstructorIndexAndMethods(t *testing.T) {
	asm, cg := generateAssembly(t, `let m: Map<string,int> = new Map<string,int>(("a",1),("b",2)); m["c"] = 3; print(m["a"]); print(m["z"] ?? 0); print(m.length()); print(m.has("a")); print(m.removeKey("b")); m.clear(); print(m.length);`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	for _, want := range []string{
		"call map_new",
		"call map_set",
		"call map_get",
		"call map_has",
		"call map_remove",
		"call map_clear",
	} {
		if !strings.Contains(asm, want) {
			t.Fatalf("expected %q in assembly, got:\n%s", want, asm)
		}
	}
}

func TestCodegenMapTypeValidation(t *testing.T) {
	_, cg := generateAssembly(t, `let m: Map<string,int> = new Map<string,int>(("a","x"));`)
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected codegen errors for map value type mismatch")
	}

	_, cg = generateAssembly(t, `let m: Map<string,int> = new Map<string,int>(("a",1)); m[1] = 2;`)
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected codegen errors for map key type mismatch")
	}
}

func TestCodegenStructConstructorAndMemberAccess(t *testing.T) {
	asm, cg := generateAssembly(t, `
struct Point { x: int, y?: int, z: int = 7 }
let p: Point = new Point(x = 1, y = 2);
print(p.x);
p.y = 9;
print(p.y);
print(hasField(p, "x"));
`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	for _, want := range []string{"call map_new", "call map_set", "call map_get", "call map_has"} {
		if !strings.Contains(asm, want) {
			t.Fatalf("expected %q in assembly, got:\n%s", want, asm)
		}
	}
}

func TestCodegenStructValidation(t *testing.T) {
	_, cg := generateAssembly(t, `struct Point { x: int }; let p: Point = new Point();`)
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected codegen errors for missing required struct field")
	}
	_, cg = generateAssembly(t, `struct Point { x: int }; let p: Point = new Point(1); p.x = "s";`)
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected codegen errors for member assignment type mismatch")
	}
}

func TestCodegenPointerOps(t *testing.T) {
	asm, cg := generateAssembly(t, `let x: int = 1; let p: *int = &x; *p = 7; print(*p);`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "lea -") || !strings.Contains(asm, "mov (%rax), %rax") {
		t.Fatalf("expected pointer address/deref instructions, got:\n%s", asm)
	}
}

func TestCodegenPointerCoalesceDeref(t *testing.T) {
	asm, cg := generateAssembly(t, `let x: int = 7; let maybe: *int||null = null; print(*(maybe ?? &x));`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "call print_int") {
		t.Fatalf("expected coalesced pointer deref to print as int, got:\n%s", asm)
	}
}

func TestCodegenPointerValidation(t *testing.T) {
	_, cg := generateAssembly(t, `let p: *int;`)
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected codegen error for non-null pointer without initializer")
	}
}

func TestCodegenStructMethodWithPointerIntegration(t *testing.T) {
	asm, cg := generateAssembly(t, `
struct Point { x: int, y: int }
fn (self: Point) sum() int { return self.x + self.y; }
fn (self: *Point) setX(v: int) { self.x = v; return; }
let p: Point = new Point(1, 2);
let pp: *Point = &p;
pp.setX(9);
print(pp.sum());
`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "call fn_method") {
		t.Fatalf("expected generated method function calls, got:\n%s", asm)
	}
}

func TestCodegenListConstructorAndMethods(t *testing.T) {
	asm, cg := generateAssembly(t, "let xs: List<int> = new List<int>(1,2); xs.append(3); xs.insert(1,7); print(xs.length()); print(xs[1]); print(xs.remove(2)); print(xs.pop()); print(xs.contains(1)); xs.clear(); print(xs.length());")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	for _, want := range []string{
		"call list_new",
		"call list_append",
		"call list_insert",
		"call list_remove",
		"call list_pop",
		"call list_contains",
		"call list_clear",
		"call list_get",
	} {
		if !strings.Contains(asm, want) {
			t.Fatalf("expected %q in assembly, got:\n%s", want, asm)
		}
	}
}

func TestCodegenListTypeValidation(t *testing.T) {
	_, cg := generateAssembly(t, `let xs: List<int> = new List<int>(1, "x");`)
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected codegen errors for list constructor type mismatch")
	}

	_, cg = generateAssembly(t, `let xs: List<int> = new List<int>(1,2); xs.append("x");`)
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected codegen errors for append type mismatch")
	}

	_, cg = generateAssembly(t, `let xs: List<int> = new List<int>(1,2); xs[0] = "x";`)
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected codegen errors for list index assign mismatch")
	}
}

func TestCodegenBuiltinCallErrors(t *testing.T) {
	_, cg := generateAssembly(t, "print();")
	if len(cg.Errors()) == 0 || !strings.Contains(cg.Errors()[0], "print expects exactly 1 argument") {
		t.Fatalf("expected print arity error, got: %v", cg.Errors())
	}

	_, cg = generateAssembly(t, "print(x = 1);")
	if len(cg.Errors()) == 0 || !strings.Contains(cg.Errors()[0], "named arguments are not supported for print") {
		t.Fatalf("expected print named-arg error, got: %v", cg.Errors())
	}

	_, cg = generateAssembly(t, "typeof();")
	if len(cg.Errors()) == 0 || !strings.Contains(cg.Errors()[0], "typeof expects exactly 1 argument") {
		t.Fatalf("expected typeof arity error, got: %v", cg.Errors())
	}

	_, cg = generateAssembly(t, "typeof(x = 1);")
	if len(cg.Errors()) == 0 || !strings.Contains(cg.Errors()[0], "named arguments are not supported for typeof") {
		t.Fatalf("expected typeof named-arg error, got: %v", cg.Errors())
	}

	_, cg = generateAssembly(t, "typeofValue();")
	if len(cg.Errors()) == 0 || !strings.Contains(cg.Errors()[0], "typeofValue expects exactly 1 argument") {
		t.Fatalf("expected typeofValue arity error, got: %v", cg.Errors())
	}

	_, cg = generateAssembly(t, "typeofvalue(x = 1);")
	if len(cg.Errors()) == 0 || !strings.Contains(cg.Errors()[0], "named arguments are not supported for typeofvalue") {
		t.Fatalf("expected typeofvalue named-arg error, got: %v", cg.Errors())
	}

	_, cg = generateAssembly(t, `hasField({1,2,3});`)
	if len(cg.Errors()) == 0 || !strings.Contains(cg.Errors()[0], "hasField expects exactly 2 arguments") {
		t.Fatalf("expected hasField arity error, got: %v", cg.Errors())
	}

	_, cg = generateAssembly(t, `hasField({1,2,3}, 1);`)
	if len(cg.Errors()) == 0 || !strings.Contains(cg.Errors()[0], "hasField field must be string") {
		t.Fatalf("expected hasField field-type error, got: %v", cg.Errors())
	}
}

func TestCodegenHasFieldBuiltin(t *testing.T) {
	asm, cg := generateAssembly(t, `let arr = {1,2,3}; print(hasField(arr, "length")); print(hasField("abc", "length")); print(hasField(1, "length"));`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if strings.Count(asm, "call print_bool") < 3 {
		t.Fatalf("expected hasField results to print as bool, got:\n%s", asm)
	}

	_, cg = generateAssembly(t, `let arr = {1,2,3}; let f = "length"; print(hasField(arr, f)); f = "x"; print(hasField(arr, f));`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors with runtime string identifier field: %v", cg.Errors())
	}
}

func TestCodegenUnsupportedCallTarget(t *testing.T) {
	_, cg := generateAssembly(t, "(1)(2);")
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected unsupported call target codegen error")
	}
	found := false
	for _, err := range cg.Errors() {
		if strings.Contains(err, "unsupported call target") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected unsupported call target error, got: %v", cg.Errors())
	}
}

func TestCodegenFunctionLiteralDirectCall(t *testing.T) {
	asm, cg := generateAssembly(t, "print((fn(x: int) int { return x + 1; })(2));")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "call fn_lit_") {
		t.Fatalf("expected direct call to generated function-literal label, got:\n%s", asm)
	}
	if !strings.Contains(asm, "call print_int") {
		t.Fatalf("expected function-literal result to print as int, got:\n%s", asm)
	}
}

func TestCodegenFunctionLiteralAssignedCallWithCapture(t *testing.T) {
	asm, cg := generateAssembly(t, "let y = 3; let getY = fn() int { return y; }; print(getY());")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "call fn_lit_") {
		t.Fatalf("expected call to generated function-literal label, got:\n%s", asm)
	}
}

func TestCodegenLambdaLiteralDirectCall(t *testing.T) {
	asm, cg := generateAssembly(t, "print(((a: int) int => { return a + 1; })(2));")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "call fn_lit_") {
		t.Fatalf("expected direct call to generated lambda label, got:\n%s", asm)
	}
	if !strings.Contains(asm, "call print_int") {
		t.Fatalf("expected lambda result to print as int, got:\n%s", asm)
	}
}

func TestCodegenLambdaAssignedCall(t *testing.T) {
	asm, cg := generateAssembly(t, "const square = (a: int) int => { return a * a; }; print(square(2));")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "call fn_lit_") {
		t.Fatalf("expected call to generated lambda label, got:\n%s", asm)
	}
}

func TestCodegenUnknownFunctionError(t *testing.T) {
	_, cg := generateAssembly(t, "doesNotExist();")
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected unknown function codegen error")
	}
	if !strings.Contains(cg.Errors()[0], "unknown function doesNotExist") {
		t.Fatalf("expected unknown-function error, got: %v", cg.Errors())
	}
}

func TestCodegenParserErrorPropagation(t *testing.T) {
	defer func() {
		if recover() != nil {
			t.Fatalf("generateAssembly should fail test on parser errors, not panic")
		}
	}()
	p := parser.New(lexer.New("let x = ;"))
	_ = p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatalf("expected parser errors for invalid input")
	}
}

func TestCodegenStringIndexing(t *testing.T) {
	asm, cg := generateAssembly(t, `print("abc"[1]); let s = "xyz"; print(s[2]);`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if strings.Count(asm, "call print_char") < 2 {
		t.Fatalf("expected string indexing results to print via print_char, got:\n%s", asm)
	}
	if !strings.Contains(asm, "movzbq (%rcx,%rax), %rax") {
		t.Fatalf("expected byte-load index sequence for string indexing, got:\n%s", asm)
	}
}

func TestCodegenUnionTypes(t *testing.T) {
	asm, cg := generateAssembly(t, `let v: int||string = 1; v = "ok"; print(typeof(v)); let xs: (int||string)[3] = {1, "two", 3}; print(typeof(xs));`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "int||string") {
		t.Fatalf("expected typeof(v) literal int||string, got:\n%s", asm)
	}
	if !strings.Contains(asm, "(int||string)[3]") {
		t.Fatalf("expected typeof(xs) literal (int||string)[3], got:\n%s", asm)
	}
}

func TestCodegenTypeAliases(t *testing.T) {
	asm, cg := generateAssembly(t, `type NumOrText = int||string; let v: NumOrText = "ok"; print(typeof(v));`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "NumOrText") {
		t.Fatalf("expected typeof(v) to preserve alias name, got:\n%s", asm)
	}

	_, cg = generateAssembly(t, `type N = int; fn add1(x: N) N { return x + 1; } print(add1(2));`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors for alias in function signature: %v", cg.Errors())
	}

	asm, cg = generateAssembly(t, `let v: int[2]||string = {1,2}; v = "ok"; print(typeof(v));`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors for int[2]||string: %v", cg.Errors())
	}
	if !strings.Contains(asm, "int[2]||string") {
		t.Fatalf("expected typeof(v) to preserve int[2]||string, got:\n%s", asm)
	}
}

func TestCodegenGenericTypeAliases(t *testing.T) {
	asm, cg := generateAssembly(t, `type Pair<T, U> = (T, U); let p: Pair<int, string> = (1, "x"); print(p.0); print(typeof(p));`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors for generic alias tuple: %v", cg.Errors())
	}
	if !strings.Contains(asm, "Pair<int,string>") {
		t.Fatalf("expected generic alias type name in typeof output, got:\n%s", asm)
	}

	asm, cg = generateAssembly(t, `type Box<T> = T[2]; let b: Box<int>; b[0] = 3; b[1] = 4; print(typeof(b)); print(b[0]); print(b[1]); print(b[0] + b[1]);`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors for generic alias array: %v", cg.Errors())
	}
	if !strings.Contains(asm, "Box<int>") {
		t.Fatalf("expected generic alias type name in typeof output, got:\n%s", asm)
	}
	if strings.Count(asm, "call print_int") < 3 {
		t.Fatalf("expected index/plus values to print as int, got:\n%s", asm)
	}
}

func TestCodegenGenericFunctionExplicitCall(t *testing.T) {
	asm, cg := generateAssembly(t, `fn id<T>(x: T) T { return x; } print(id<int>(9));`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors for explicit generic call: %v", cg.Errors())
	}
	if !strings.Contains(asm, "call print_int") {
		t.Fatalf("expected print_int for id<int>(9), got:\n%s", asm)
	}
	if !strings.Contains(asm, "fn_id__int:") {
		t.Fatalf("expected specialized label fn_id__int, got:\n%s", asm)
	}
	if !strings.Contains(asm, "call fn_id__int") {
		t.Fatalf("expected call to specialized function fn_id__int, got:\n%s", asm)
	}
}

func TestCodegenGenericFunctionMonomorphizesPerTypeArg(t *testing.T) {
	asm, cg := generateAssembly(t, `fn id<T>(x: T) T { return x; } print(id<int>(1)); print(id<string>("x"));`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors for generic monomorphization: %v", cg.Errors())
	}
	if !strings.Contains(asm, "fn_id__int:") || !strings.Contains(asm, "fn_id__string:") {
		t.Fatalf("expected per-instantiation specialized labels, got:\n%s", asm)
	}
	if !strings.Contains(asm, "call fn_id__int") || !strings.Contains(asm, "call fn_id__string") {
		t.Fatalf("expected calls to per-instantiation specialized labels, got:\n%s", asm)
	}
	if strings.Contains(asm, "fn_id:") {
		t.Fatalf("did not expect unspecialized generic base body emission, got:\n%s", asm)
	}
}

func TestCodegenGenericFunctionSpecializationDedupByAliasNormalization(t *testing.T) {
	asm, cg := generateAssembly(t, `
type N = int;
fn id<T>(x: T) T { return x; }
print(id<int>(1));
print(id<N>(2));
`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors for alias-normalized specialization: %v", cg.Errors())
	}
	if strings.Count(asm, "fn_id__int:") != 1 {
		t.Fatalf("expected exactly one int specialization, got:\n%s", asm)
	}
	if strings.Contains(asm, "fn_id__N:") {
		t.Fatalf("did not expect alias-based duplicate specialization label, got:\n%s", asm)
	}
}

func TestCodegenUnusedGenericFunctionTemplateNotEmitted(t *testing.T) {
	asm, cg := generateAssembly(t, `fn id<T>(x: T) T { return x; } print(1);`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors for unused generic template: %v", cg.Errors())
	}
	if strings.Contains(asm, "fn_id:") || strings.Contains(asm, "fn_id__") {
		t.Fatalf("did not expect generic function template/specialization when unused, got:\n%s", asm)
	}
}

func TestCodegenGenericFunctionTypeArgArityErrors(t *testing.T) {
	_, cg := generateAssembly(t, `fn id<T>(x: T) T { return x; } print(id<int, string>(9));`)
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected codegen errors for generic arity mismatch")
	}
	found := false
	for _, err := range cg.Errors() {
		if strings.Contains(err, "wrong number of generic type arguments") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected generic arity error, got: %v", cg.Errors())
	}
}

func TestCodegenGenericFunctionInferenceRequiresUnambiguousUse(t *testing.T) {
	var asm string
	_, cg := generateAssembly(t, `fn passthrough<T>(x: int) int { return x; } print(passthrough(5));`)
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected codegen errors for unresolved generic inference")
	}
	found := false
	for _, err := range cg.Errors() {
		if strings.Contains(err, "could not infer generic type argument for T; specify it explicitly") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected unresolved-generic inference error, got: %v", cg.Errors())
	}

	_, cg = generateAssembly(t, `fn passthrough<T>(x: int) int { return x; } print(passthrough<int>(5));`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors for explicit generic passthrough: %v", cg.Errors())
	}

	asm, cg = generateAssembly(t, `
type Box<T> = T[2];
fn first<T>(b: Box<T>) T { return b[0]; }
let b: Box<int>;
b[0] = 4;
b[1] = 9;
print(first(b));
`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors for nested generic inference: %v", cg.Errors())
	}
	if !strings.Contains(asm, "call fn_first__int") {
		t.Fatalf("expected inferred specialization call fn_first__int, got:\n%s", asm)
	}
}

func TestCodegenGenericAliasTypeArgArityErrors(t *testing.T) {
	_, cg := generateAssembly(t, `type Pair<T, U> = (T, U); let p: Pair<int> = (1, "x");`)
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected codegen errors for generic alias arity mismatch")
	}
	found := false
	for _, err := range cg.Errors() {
		if strings.Contains(err, "wrong number of generic type arguments for Pair: expected 2, got 1") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected generic alias arity error, got: %v", cg.Errors())
	}

	_, cg = generateAssembly(t, `type N = int; let x: N<string> = 1;`)
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected codegen errors for non-generic alias type arguments")
	}
	found = false
	for _, err := range cg.Errors() {
		if strings.Contains(err, "wrong number of generic type arguments for N: expected 0, got 1") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected non-generic alias type-arg arity error, got: %v", cg.Errors())
	}
}

func TestCodegenTupleTypesAndAccess(t *testing.T) {
	asm, cg := generateAssembly(t, `let a: (int,string,bool) = (1, "x", true); print(a.0); print(a.1); print(a.2);`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors for tuple access: %v", cg.Errors())
	}
	if strings.Count(asm, "call print_int") < 1 || strings.Count(asm, "call print_cstr") < 1 || strings.Count(asm, "call print_bool") < 1 {
		t.Fatalf("expected tuple access prints for int/string/bool, got:\n%s", asm)
	}
}

func TestCodegenTupleUnionAndAlias(t *testing.T) {
	asm, cg := generateAssembly(t, `let v: (int,string)||string = (1, "x"); print(v.0); v = "ok"; print(typeof(v));`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors for tuple union: %v", cg.Errors())
	}
	if !strings.Contains(asm, "(int,string)||string") {
		t.Fatalf("expected typeof(v) tuple-union literal, got:\n%s", asm)
	}

	asm, cg = generateAssembly(t, `type Pair = (int,string); let p: Pair = (1, "x"); print(p.1); print(typeof(p));`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors for tuple alias: %v", cg.Errors())
	}
	if !strings.Contains(asm, "Pair") {
		t.Fatalf("expected typeof(p) alias name Pair, got:\n%s", asm)
	}
}

func TestCodegenUnionTypedIfComparison(t *testing.T) {
	asm, cg := generateAssembly(t, `let a: string||int = 3; if (a == 3) { print("entered"); };`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "entered") {
		t.Fatalf("expected branch string literal in assembly, got:\n%s", asm)
	}
}

func TestCodegenUnionTypedIfAfterReassignment(t *testing.T) {
	_, cg := generateAssembly(t, `let a: string||int = "x"; a = 5; if (a == 5) { print("ok"); };`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
}

func TestCodegenUnionWithThreeOrMoreTypes(t *testing.T) {
	asm, cg := generateAssembly(t, `let v: int||string||bool = 1; v = "ok"; v = true; print(typeof(v));`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors for 3-member union: %v", cg.Errors())
	}
	if !strings.Contains(asm, "int||string||bool") {
		t.Fatalf("expected typeof(v) to preserve 3-member union, got:\n%s", asm)
	}
}

func TestCodegenAnyType(t *testing.T) {
	asm, cg := generateAssembly(t, `let x: any = 3; x = "ok"; print(typeof(x)); print(typeofValue(x));`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "any") {
		t.Fatalf("expected typeof(x) literal any, got:\n%s", asm)
	}
	if !strings.Contains(asm, "string") {
		t.Fatalf("expected typeofValue(x) to resolve to runtime string after assignment, got:\n%s", asm)
	}
}

func TestCodegenAnyTypeInfixWithKnownValue(t *testing.T) {
	asm, cg := generateAssembly(t, `let x: any = 3; print(x + 4);`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "add %rcx, %rax") {
		t.Fatalf("expected int add path for known any value, got:\n%s", asm)
	}
}

func TestCodegenImportBuiltinMathNamespace(t *testing.T) {
	asm, cg := generateAssembly(t, `import twice.math as math; println(math.abs(-7)); println(math.min(9, 2)); println(math.sqrt(49));`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if strings.Count(asm, "call println_int") < 3 {
		t.Fatalf("expected imported math calls to print ints, got:\n%s", asm)
	}
	if !strings.Contains(asm, "imul %rdx, %rdx") {
		t.Fatalf("expected sqrt integer loop path, got:\n%s", asm)
	}
}

func TestCodegenImportBuiltinMathFloatInputs(t *testing.T) {
	asm, cg := generateAssembly(t, `import twice.math as math; println(math.abs(-3.5)); println(math.min(1.5, 2)); println(math.sqrt(2.25));`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if strings.Count(asm, "call println_cstr") < 3 {
		t.Fatalf("expected float math calls to print as cstr floats, got:\n%s", asm)
	}
	if !strings.Contains(asm, ".asciz \"3.5\"") {
		t.Fatalf("expected folded float abs result, got:\n%s", asm)
	}
	if !strings.Contains(asm, ".asciz \"1.5\"") {
		t.Fatalf("expected folded float sqrt result, got:\n%s", asm)
	}
}

func TestCodegenImportBuiltinMathMemberAlias(t *testing.T) {
	asm, cg := generateAssembly(t, `import twice.math.max as max; println(max(1, 8));`)
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if !strings.Contains(asm, "call println_int") {
		t.Fatalf("expected imported alias call to print int, got:\n%s", asm)
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
