package evaluator

import (
	"strings"
	"testing"

	"twice/internal/ast"
	"twice/internal/lexer"
	"twice/internal/object"
	"twice/internal/parser"
	"twice/internal/token"
)

func TestIntegerEval(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"5", 5},
		{"10", 10},
		{"5 + 5 + 5 + 5 - 10", 10},
		{"2 * 3 * 4", 24},
		{"(2 + 3) * 4", 20},
		{"5 & 3", 1},
		{"5 | 2", 7},
		{"5 ^ 1", 4},
		{"5 << 1", 10},
		{"5 >> 1", 2},
		{"5 % 2", 1},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		testIntegerObject(t, evaluated, tt.expected)
	}
}

func TestBooleanEval(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"true", true},
		{"false", false},
		{"1 < 2", true},
		{"1 > 2", false},
		{"1 == 1", true},
		{"1 != 1", false},
		{"!true", false},
		{"!false", true},
		{"true && true", true},
		{"true && false", false},
		{"false || true", true},
		{"false || false", false},
		{"true ^^ false", true},
		{"true ^^ true", false},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		testBooleanObject(t, evaluated, tt.expected)
	}
}

func TestBooleanOperatorTypeMismatch(t *testing.T) {
	evaluated := testEval("1 && true")
	errObj, ok := evaluated.(*object.Error)
	if !ok {
		t.Fatalf("expected error object, got=%T", evaluated)
	}
	if errObj.Message != "type mismatch: INTEGER && BOOLEAN" {
		t.Fatalf("wrong error message: %q", errObj.Message)
	}
}

func TestIfElseEval(t *testing.T) {
	evaluated := testEval("if (1 < 2) { 10 } else { 20 }")
	testIntegerObject(t, evaluated, 10)
}

func TestWhileEval(t *testing.T) {
	evaluated := testEval("let i = 0; while (i < 3) { i = i + 1; } i")
	testIntegerObject(t, evaluated, 3)
}

func TestForEval(t *testing.T) {
	evaluated := testEval("let sum = 0; for (let i = 0; i < 4; i++) { sum = sum + i; } sum")
	testIntegerObject(t, evaluated, 6)
}

func TestBlockScopeShadowingEval(t *testing.T) {
	evaluated := testEval("let x = 1; if (true) { let x = 2; }; x")
	testIntegerObject(t, evaluated, 1)
}

func TestBlockScopeAssignmentToOuterEval(t *testing.T) {
	evaluated := testEval("let x = 1; if (true) { x = 2; }; x")
	testIntegerObject(t, evaluated, 2)
}

func TestBlockScopeNoLeakEval(t *testing.T) {
	evaluated := testEval("if (true) { let y = 1; }; y")
	errObj, ok := evaluated.(*object.Error)
	if !ok {
		t.Fatalf("expected error object, got=%T", evaluated)
	}
	if errObj.Message != "identifier not found: y" {
		t.Fatalf("wrong error message: %q", errObj.Message)
	}
}

func TestForScopeNoLeakEval(t *testing.T) {
	evaluated := testEval("for (let i = 0; i < 1; i++) { }; i")
	errObj, ok := evaluated.(*object.Error)
	if !ok {
		t.Fatalf("expected error object, got=%T", evaluated)
	}
	if errObj.Message != "identifier not found: i" {
		t.Fatalf("wrong error message: %q", errObj.Message)
	}
}

func TestStandaloneBlockScopeNoLeakEval(t *testing.T) {
	evaluated := testEval("{ let t = 123; } t")
	errObj, ok := evaluated.(*object.Error)
	if !ok {
		t.Fatalf("expected error object, got=%T", evaluated)
	}
	if errObj.Message != "identifier not found: t" {
		t.Fatalf("wrong error message: %q", errObj.Message)
	}
}

func TestStandaloneBlockFunctionNoLeakEval(t *testing.T) {
	evaluated := testEval("{ fn tmp() int { return 1; } tmp(); } tmp();")
	errObj, ok := evaluated.(*object.Error)
	if !ok {
		t.Fatalf("expected error object, got=%T", evaluated)
	}
	if errObj.Message != "identifier not found: tmp" {
		t.Fatalf("wrong error message: %q", errObj.Message)
	}
}

func TestBreakInWhileEval(t *testing.T) {
	evaluated := testEval("let i = 0; while (true) { i++; if (i == 3) { break; }; }; i")
	testIntegerObject(t, evaluated, 3)
}

func TestContinueInForEval(t *testing.T) {
	evaluated := testEval("let sum = 0; for (let i = 0; i < 5; i++) { if (i == 2) { continue; }; sum = sum + i; }; sum")
	testIntegerObject(t, evaluated, 8)
}

func TestBreakContinueOutsideLoopEval(t *testing.T) {
	evaluated := testEval("break;")
	errObj, ok := evaluated.(*object.Error)
	if !ok {
		t.Fatalf("expected error object, got=%T", evaluated)
	}
	if errObj.Message != "break not inside loop" {
		t.Fatalf("wrong error message: %q", errObj.Message)
	}

	evaluated = testEval("continue;")
	errObj, ok = evaluated.(*object.Error)
	if !ok {
		t.Fatalf("expected error object, got=%T", evaluated)
	}
	if errObj.Message != "continue not inside loop" {
		t.Fatalf("wrong error message: %q", errObj.Message)
	}
}

func TestLoopEvalPropagatesError(t *testing.T) {
	evaluated := testEval("loop { missing; }")
	errObj, ok := evaluated.(*object.Error)
	if !ok {
		t.Fatalf("expected error object, got=%T", evaluated)
	}
	if errObj.Message != "identifier not found: missing" {
		t.Fatalf("wrong error message: %q", errObj.Message)
	}
}

func TestFunctionCallEval(t *testing.T) {
	evaluated := testEval("let add = fn(x, y) { x + y; }; add(2, 3)")
	testIntegerObject(t, evaluated, 5)
}

func TestFunctionEmptyReturnEval(t *testing.T) {
	evaluated := testEval("fn noop() { return; } noop()")
	if evaluated.Type() != object.NULL_OBJ {
		t.Fatalf("expected null from empty return, got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}
}

func TestNamedFunctionWithDefaultsAndTypedReturnEval(t *testing.T) {
	evaluated := testEval("fn add(a: int, b: int = 2) int { return a + b; } add(3)")
	testIntegerObject(t, evaluated, 5)
}

func TestNamedArgumentsCallEval(t *testing.T) {
	evaluated := testEval("fn sub(a: int, b: int) int { return a - b; } sub(b = 2, a = 7)")
	testIntegerObject(t, evaluated, 5)
}

func TestMixedPositionalAndNamedCallEval(t *testing.T) {
	evaluated := testEval("fn sum(a: int, b: int = 4, c: int = 1) int { return a + b + c; } sum(2, c = 3)")
	testIntegerObject(t, evaluated, 9)
}

func TestFunctionReturnTypeValidationEval(t *testing.T) {
	evaluated := testEval("fn bad() int { return true; } bad()")
	errObj, ok := evaluated.(*object.Error)
	if !ok {
		t.Fatalf("expected error object, got=%T", evaluated)
	}
	if errObj.Message != "cannot return bool from function returning int" {
		t.Fatalf("wrong error message: %q", errObj.Message)
	}
}

func TestFunctionEmptyReturnTypeValidationEval(t *testing.T) {
	evaluated := testEval("fn bad() int { return; } bad()")
	errObj, ok := evaluated.(*object.Error)
	if !ok {
		t.Fatalf("expected error object, got=%T", evaluated)
	}
	if errObj.Message != "cannot return null from function returning int" {
		t.Fatalf("wrong error message: %q", errObj.Message)
	}

	evaluated = testEval("fn ok() int||null { return; } ok()")
	if evaluated.Type() != object.NULL_OBJ {
		t.Fatalf("expected null object for int||null return, got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}

	evaluated = testEval(`fn ok2() string||null { return; } ok2()`)
	if evaluated.Type() != object.NULL_OBJ {
		t.Fatalf("expected null object for string||null return, got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}
}

func TestUnknownIdentifierError(t *testing.T) {
	evaluated := testEval("foobar")
	errObj, ok := evaluated.(*object.Error)
	if !ok {
		t.Fatalf("expected error object, got=%T", evaluated)
	}
	if errObj.Message != "identifier not found: foobar" {
		t.Fatalf("wrong error message: %q", errObj.Message)
	}
}

func TestConstEval(t *testing.T) {
	evaluated := testEval("const x = 7; x")
	testIntegerObject(t, evaluated, 7)
}

func TestConstCannotBeRedeclaredByLet(t *testing.T) {
	evaluated := testEval("const x = 1; let x = 2;")
	errObj, ok := evaluated.(*object.Error)
	if !ok {
		t.Fatalf("expected error object, got=%T", evaluated)
	}
	if errObj.Message != "cannot reassign const: x" {
		t.Fatalf("wrong error message: %q", errObj.Message)
	}
}

func TestVariableReassignmentEval(t *testing.T) {
	evaluated := testEval("let x = 1; x = 3; x")
	testIntegerObject(t, evaluated, 3)
}

func TestReassignmentUndeclaredError(t *testing.T) {
	evaluated := testEval("x = 3;")
	errObj, ok := evaluated.(*object.Error)
	if !ok {
		t.Fatalf("expected error object, got=%T", evaluated)
	}
	if errObj.Message != "identifier not found: x" {
		t.Fatalf("wrong error message: %q", errObj.Message)
	}
}

func TestAdditionalLiteralTypesEval(t *testing.T) {
	tests := []struct {
		input        string
		expectedType object.ObjectType
	}{
		{`3.14`, object.FLOAT_OBJ},
		{`"hello"`, object.STRING_OBJ},
		{`'a'`, object.CHAR_OBJ},
		{`null`, object.NULL_OBJ},
	}
	for _, tt := range tests {
		evaluated := testEval(tt.input)
		if evaluated.Type() != tt.expectedType {
			t.Fatalf("wrong type for %q. got=%s want=%s", tt.input, evaluated.Type(), tt.expectedType)
		}
	}
}

func TestTypedDeclarationAndNullInit(t *testing.T) {
	evaluated := testEval("let s: string; s")
	if evaluated.Type() != object.NULL_OBJ {
		t.Fatalf("expected null object, got=%s", evaluated.Type())
	}
}

func TestTypedAssignmentValidation(t *testing.T) {
	evaluated := testEval("let x: int = 1; x = true;")
	errObj, ok := evaluated.(*object.Error)
	if !ok {
		t.Fatalf("expected error object, got=%T", evaluated)
	}
	if errObj.Message != "cannot assign bool to int" {
		t.Fatalf("wrong error message: %q", errObj.Message)
	}
}

func TestTypeofAndCasts(t *testing.T) {
	evaluated := testEval("typeof(1)")
	if evaluated.Type() != object.TYPE_OBJ || evaluated.Inspect() != "int" {
		t.Fatalf("expected type(int), got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}

	evaluated = testEval("int(true)")
	testIntegerObject(t, evaluated, 1)

	evaluated = testEval("bool(0)")
	testBooleanObject(t, evaluated, false)

	evaluated = testEval("let n: string; typeof(n)")
	if evaluated.Type() != object.TYPE_OBJ || evaluated.Inspect() != "string" {
		t.Fatalf("expected type(string), got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}

	evaluated = testEval("let a: int||string = 3; typeofValue(a)")
	if evaluated.Type() != object.TYPE_OBJ || evaluated.Inspect() != "int" {
		t.Fatalf("expected value type(int), got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}

	evaluated = testEval("let a: int||string = 3; typeofvalue(a)")
	if evaluated.Type() != object.TYPE_OBJ || evaluated.Inspect() != "int" {
		t.Fatalf("expected value type(int), got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}

	evaluated = testEval("let a: int||string = 3; if (typeofValue(a) == int) { 1 } else { 0 }")
	testIntegerObject(t, evaluated, 1)
}

func TestStringConcatenationEval(t *testing.T) {
	evaluated := testEval(`"Hello, " + "Twice!"`)
	got, ok := unwrapReturn(evaluated).(*object.String)
	if !ok {
		t.Fatalf("expected string object, got=%T", evaluated)
	}
	if got.Value != "Hello, Twice!" {
		t.Fatalf("wrong string concat value: got=%q", got.Value)
	}
}

func TestFloatAdditionEval(t *testing.T) {
	evaluated := testEval("1.25 + 2.75")
	got, ok := unwrapReturn(evaluated).(*object.Float)
	if !ok {
		t.Fatalf("expected float object, got=%T", evaluated)
	}
	if got.Value != 4.0 {
		t.Fatalf("wrong float addition value: got=%g", got.Value)
	}
}

func TestCharPlusCharReturnsCharEval(t *testing.T) {
	evaluated := testEval("'A' + 'B'")
	got, ok := unwrapReturn(evaluated).(*object.Char)
	if !ok {
		t.Fatalf("expected char object, got=%T", evaluated)
	}
	if got.Value != rune('A'+'B') {
		t.Fatalf("wrong char addition value: got=%q want=%q", got.Value, rune('A'+'B'))
	}
}

func TestMixedNumericOpsReturnFloatEval(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"1 + 2.5", 3.5},
		{"5 - 1.5", 3.5},
		{"3 * 1.5", 4.5},
		{"7 / 2.0", 3.5},
		{"1.5 + 2.0", 3.5},
		{"5.0 - 1.5", 3.5},
		{"1.5 * 2.0", 3.0},
		{"7.0 / 2.0", 3.5},
		{"7.5 % 2.0", 1.5},
		{"7 % 2.0", 1.0},
	}
	for _, tt := range tests {
		evaluated := testEval(tt.input)
		got, ok := unwrapReturn(evaluated).(*object.Float)
		if !ok {
			t.Fatalf("expected float object for %q, got=%T", tt.input, evaluated)
		}
		if got.Value != tt.want {
			t.Fatalf("wrong value for %q: got=%g want=%g", tt.input, got.Value, tt.want)
		}
	}
}

func TestModuloAssignmentEval(t *testing.T) {
	evaluated := testEval("let x = 7; x %= 4; x")
	testIntegerObject(t, evaluated, 3)
}

func TestCharPlusIntReturnsCharEval(t *testing.T) {
	evaluated := testEval("'A' + 1")
	got, ok := unwrapReturn(evaluated).(*object.Char)
	if !ok {
		t.Fatalf("expected char object, got=%T", evaluated)
	}
	if got.Value != 'B' {
		t.Fatalf("wrong char+int value: got=%q want=%q", got.Value, 'B')
	}
}

func TestStringConcatWithNumericAndCharEval(t *testing.T) {
	evaluated := testEval(`"x:" + 7`)
	if got := unwrapReturn(evaluated).Inspect(); got != "x:7" {
		t.Fatalf("wrong string+int value: got=%q", got)
	}
	evaluated = testEval(`"x:" + 3.5`)
	if got := unwrapReturn(evaluated).Inspect(); got != "x:3.5" {
		t.Fatalf("wrong string+float value: got=%q", got)
	}
	evaluated = testEval(`"x:" + 'A'`)
	if got := unwrapReturn(evaluated).Inspect(); got != "x:A" {
		t.Fatalf("wrong string+char value: got=%q", got)
	}

	evaluated = testEval(`7 + " apples"`)
	if got := unwrapReturn(evaluated).Inspect(); got != "7 apples" {
		t.Fatalf("wrong int+string value: got=%q", got)
	}
}

func TestEscapedAndTemplateStringsEval(t *testing.T) {
	evaluated := testEval(`"line1\nline2\tend"`)
	if got := unwrapReturn(evaluated).Inspect(); got != "line1\nline2\tend" {
		t.Fatalf("wrong escaped string value: got=%q", got)
	}

	evaluated = testEval("let name = \"Twice\"; `Hello ${name}\\n`;")
	if got := unwrapReturn(evaluated).Inspect(); got != "Hello Twice\n" {
		t.Fatalf("wrong template string value: got=%q", got)
	}
}

func TestArrayLiteralAndTypedArrayEval(t *testing.T) {
	evaluated := testEval("let arr = {1, 2, 3}; typeof(arr)")
	if evaluated.Type() != object.TYPE_OBJ || evaluated.Inspect() != "int[3]" {
		t.Fatalf("expected type(int[3]), got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}

	evaluated = testEval("let arr: int[3] = {1, 2, 3}; typeof(arr)")
	if evaluated.Type() != object.TYPE_OBJ || evaluated.Inspect() != "int[3]" {
		t.Fatalf("expected type(int[3]), got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}

	evaluated = testEval("let arr: int[3] = {1, 2, 3}; typeof(arr)")
	if evaluated.Type() != object.TYPE_OBJ || evaluated.Inspect() != "int[3]" {
		t.Fatalf("expected type(int[3]), got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}

	evaluated = testEval("let grid: int[2][2] = {{1, 2}, {2, 3}}; typeof(grid)")
	if evaluated.Type() != object.TYPE_OBJ || evaluated.Inspect() != "int[2][2]" {
		t.Fatalf("expected type(int[2][2]), got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}

	evaluated = testEval("let grid = {{1, 2}, {2, 3}}; typeof(grid)")
	if evaluated.Type() != object.TYPE_OBJ || evaluated.Inspect() != "int[2][2]" {
		t.Fatalf("expected type(int[2][2]), got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}

	evaluated = testEval("let arr: int[3] = {}; typeof(arr)")
	if evaluated.Type() != object.TYPE_OBJ || evaluated.Inspect() != "int[3]" {
		t.Fatalf("expected type(int[3]) from empty literal, got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}

	evaluated = testEval("let arr: int[3]; arr.length()")
	testIntegerObject(t, evaluated, 3)
}

func TestArrayTypeValidationEval(t *testing.T) {
	evaluated := testEval("let arr: int[2] = {1, 2, 3};")
	errObj, ok := evaluated.(*object.Error)
	if !ok {
		t.Fatalf("expected error object, got=%T", evaluated)
	}
	if errObj.Message != "cannot assign int[3] to int[2]" {
		t.Fatalf("wrong error message: %q", errObj.Message)
	}

	evaluated = testEval("let arr = {1, true};")
	if evaluated != nil {
		t.Fatalf("expected nil result for declaration statement, got=%T", evaluated)
	}

	evaluated = testEval("let arr = {1, true}; typeof(arr)")
	if evaluated.Type() != object.TYPE_OBJ || evaluated.Inspect() != "(int||bool)[2]" {
		t.Fatalf("expected type((int||bool)[2]), got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}
}

func TestArrayIndexGetAndSetEval(t *testing.T) {
	evaluated := testEval("let arr = {1, 2, 3}; arr[1]")
	testIntegerObject(t, evaluated, 2)

	evaluated = testEval("let arr = {1, 2, 3}; arr[1] = 99; arr[1]")
	testIntegerObject(t, evaluated, 99)
}

func TestArrayIndexErrorsEval(t *testing.T) {
	evaluated := testEval("let arr = {1, 2, 3}; arr[9]")
	errObj, ok := evaluated.(*object.Error)
	if !ok {
		t.Fatalf("expected error object, got=%T", evaluated)
	}
	if errObj.Message != "array index out of bounds: 9" {
		t.Fatalf("wrong error message: %q", errObj.Message)
	}

	evaluated = testEval("let arr = {1, 2, 3}; arr[1] = true;")
	errObj, ok = evaluated.(*object.Error)
	if !ok {
		t.Fatalf("expected error object, got=%T", evaluated)
	}
	if errObj.Message != "cannot assign bool to int" {
		t.Fatalf("wrong error message: %q", errObj.Message)
	}
}

func TestArrayLengthMethodEval(t *testing.T) {
	evaluated := testEval("let arr = {1, 2, 3}; arr.length()")
	testIntegerObject(t, evaluated, 3)

	evaluated = testEval("let grid = {{1}, {2, 3}}; grid.length()")
	testIntegerObject(t, evaluated, 2)
}

func TestNullishCoalescingEval(t *testing.T) {
	evaluated := testEval(`null ?? 7`)
	testIntegerObject(t, evaluated, 7)

	evaluated = testEval(`5 ?? 7`)
	testIntegerObject(t, evaluated, 5)

	evaluated = testEval(`let x: int; x ?? 9`)
	testIntegerObject(t, evaluated, 9)
}

func TestNullSafeMethodCallEval(t *testing.T) {
	evaluated := testEval(`let arr: int[3]; arr?.length()`)
	testIntegerObject(t, evaluated, 3)

	evaluated = testEval(`let arr = {1,2,3}; arr?.length()`)
	testIntegerObject(t, evaluated, 3)

	evaluated = testEval(`let arr: int[3]||null = {1,2,3}; arr?.length()`)
	testIntegerObject(t, evaluated, 3)

	evaluated = testEval(`let arr: int[3]||null = null; arr?.length()`)
	if evaluated != NULL {
		t.Fatalf("expected null result for null-safe method call on null union receiver, got=%T (%s)", evaluated, evaluated.Inspect())
	}
}

func TestEmptyTupleLiteralEval(t *testing.T) {
	evaluated := testEval(`let t: (int, string) = (); t.0`)
	if evaluated != NULL {
		t.Fatalf("expected null tuple slot from empty literal, got=%T (%s)", evaluated, evaluated.Inspect())
	}
}

func TestNullSafeAccessEval(t *testing.T) {
	evaluated := testEval(`let x: int; x?.missing`)
	if evaluated != NULL {
		t.Fatalf("expected null for null-safe access on null receiver, got=%T (%s)", evaluated, evaluated.Inspect())
	}

	evaluated = testEval(`let x = 1; x?.missing`)
	if evaluated != NULL {
		t.Fatalf("expected null for unknown null-safe member on non-null receiver, got=%T (%s)", evaluated, evaluated.Inspect())
	}

	evaluated = testEval(`let x: int[3]||null = {1,2,3}; x?.length`)
	testIntegerObject(t, evaluated, 3)

	evaluated = testEval(`let x: int[3]||null = null; x?.length`)
	if evaluated != NULL {
		t.Fatalf("expected null for null-safe access on null union receiver, got=%T (%s)", evaluated, evaluated.Inspect())
	}
}

func TestMemberAccessEval(t *testing.T) {
	evaluated := testEval(`let arr = {1,2,3}; arr.length`)
	testIntegerObject(t, evaluated, 3)

	evaluated = testEval(`let x = 1; x.missing`)
	errObj, ok := evaluated.(*object.Error)
	if !ok {
		t.Fatalf("expected error object, got=%T", evaluated)
	}
	if errObj.Message != "unknown member: missing" {
		t.Fatalf("wrong error message: %q", errObj.Message)
	}
}

func TestNullSafeUnknownMethodEval(t *testing.T) {
	evaluated := testEval(`let arr = {1,2,3}; arr?.missing()`)
	if evaluated != NULL {
		t.Fatalf("expected null for unknown null-safe method, got=%T (%s)", evaluated, evaluated.Inspect())
	}

	evaluated = testEval(`let x = 1; x?.missing()`)
	if evaluated != NULL {
		t.Fatalf("expected null for null-safe method on unsupported receiver, got=%T (%s)", evaluated, evaluated.Inspect())
	}
}

func TestHasFieldBuiltinEval(t *testing.T) {
	evaluated := testEval(`hasField({1,2,3}, "length")`)
	testBooleanObject(t, evaluated, true)

	evaluated = testEval(`hasField("abc", "length")`)
	testBooleanObject(t, evaluated, true)

	evaluated = testEval(`hasField(1, "x")`)
	testBooleanObject(t, evaluated, false)

	evaluated = testEval(`hasField(null, "x")`)
	testBooleanObject(t, evaluated, false)
}

func TestHasFieldBuiltinErrorsEval(t *testing.T) {
	evaluated := testEval(`hasField({1,2,3}, 1)`)
	errObj, ok := evaluated.(*object.Error)
	if !ok {
		t.Fatalf("expected error object, got=%T", evaluated)
	}
	if errObj.Message != "hasField field must be string, got int" {
		t.Fatalf("wrong error message: %q", errObj.Message)
	}

	evaluated = testEval(`hasField({1,2,3})`)
	errObj, ok = evaluated.(*object.Error)
	if !ok {
		t.Fatalf("expected error object, got=%T", evaluated)
	}
	if errObj.Message != "hasField expects 2 arguments, got=1" {
		t.Fatalf("wrong error message: %q", errObj.Message)
	}
}

func TestBuiltinArgumentErrorsEval(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`typeof()`, "typeof expects 1 argument, got=0"},
		{`typeof(1, 2)`, "typeof expects 1 argument, got=2"},
		{`typeofValue()`, "typeofValue expects 1 argument, got=0"},
		{`typeofvalue(1, 2)`, "typeofvalue expects 1 argument, got=2"},
		{`int()`, "int expects 1 argument, got=0"},
		{`float()`, "float expects 1 argument, got=0"},
		{`string()`, "string expects 1 argument, got=0"},
		{`char()`, "char expects 1 argument, got=0"},
		{`bool()`, "bool expects 1 argument, got=0"},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		errObj, ok := evaluated.(*object.Error)
		if !ok {
			t.Fatalf("expected error object for %q, got=%T", tt.input, evaluated)
		}
		if errObj.Message != tt.want {
			t.Fatalf("wrong error message for %q: got=%q want=%q", tt.input, errObj.Message, tt.want)
		}
	}
}

func TestNamedArgumentsBuiltinCallErrorEval(t *testing.T) {
	evaluated := testEval(`typeof(x = 1)`)
	errObj, ok := evaluated.(*object.Error)
	if !ok {
		t.Fatalf("expected error object, got=%T", evaluated)
	}
	if errObj.Message != "named arguments are not supported for builtin functions" {
		t.Fatalf("wrong error message: %q", errObj.Message)
	}
}

func TestFunctionCallArgumentErrorsEval(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`fn f(a: int) int { return a; } f()`, "missing required argument: a"},
		{`fn f(a: int) int { return a; } f(1, 2)`, "too many positional arguments"},
		{`fn f(a: int = 1) int { return a; } f(b = 1)`, "unknown named argument: b"},
		{`fn f(a: int) int { return a; } f(a = 1, a = 2)`, "duplicate named argument: a"},
		{`fn f(a: int) int { return a; } f(1, a = 2)`, "argument provided twice: a"},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		errObj, ok := evaluated.(*object.Error)
		if !ok {
			t.Fatalf("expected error object for %q, got=%T", tt.input, evaluated)
		}
		if errObj.Message != tt.want {
			t.Fatalf("wrong error message for %q: got=%q want=%q", tt.input, errObj.Message, tt.want)
		}
	}
}

func TestMethodCallErrorsEval(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`let s = "abc"; s.length()`, "length is only supported on arrays/lists/maps"},
		{`let arr = {1,2,3}; arr.length(1)`, "length expects 0 arguments, got=1"},
		{`let arr = {1,2,3}; arr.missing()`, "unknown method: missing"},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		errObj, ok := evaluated.(*object.Error)
		if !ok {
			t.Fatalf("expected error object for %q, got=%T", tt.input, evaluated)
		}
		if errObj.Message != tt.want {
			t.Fatalf("wrong error message for %q: got=%q want=%q", tt.input, errObj.Message, tt.want)
		}
	}
}

func TestListEvalBasics(t *testing.T) {
	evaluated := testEval(`let xs: List<int> = new List<int>(1, 2); xs.append(3); xs[1] = 9; xs.length()`)
	testIntegerObject(t, evaluated, 3)

	evaluated = testEval(`let xs: List<int> = new List<int>(1, 2, 3); xs.remove(1)`)
	testIntegerObject(t, evaluated, 2)

	evaluated = testEval(`let xs: List<int> = new List<int>(1, 2, 3); xs.insert(1, 7); xs[1]`)
	testIntegerObject(t, evaluated, 7)

	evaluated = testEval(`let xs: List<int> = new List<int>(1, 2, 3); xs.pop()`)
	testIntegerObject(t, evaluated, 3)

	evaluated = testEval(`let xs: List<int> = new List<int>(1, 2, 3); xs.clear(); xs.length`)
	testIntegerObject(t, evaluated, 0)

	evaluated = testEval(`let xs: List<int> = new List<int>(1, 2, 3); xs.contains(2)`)
	testBooleanObject(t, evaluated, true)
}

func TestListEvalErrors(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`let xs: List<int> = new List<int>(1, 2); xs[9]`, "list index out of bounds: 9"},
		{`let xs: List<int> = new List<int>(1, 2); xs[0] = "x"; xs[0]`, "cannot assign string to int"},
		{`let xs: List<int> = new List<int>(1, 2); xs.append("x"); xs.length()`, "cannot assign string to int"},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		errObj, ok := evaluated.(*object.Error)
		if !ok {
			t.Fatalf("expected error object for %q, got=%T", tt.input, evaluated)
		}
		if errObj.Message != tt.want {
			t.Fatalf("wrong error message for %q: got=%q want=%q", tt.input, errObj.Message, tt.want)
		}
	}
}

func TestMapEvalBasics(t *testing.T) {
	evaluated := testEval(`let m: Map<string,int> = new Map<string,int>(("a", 1), ("b", 2)); m["c"] = 3; m["a"]`)
	testIntegerObject(t, evaluated, 1)

	evaluated = testEval(`let m: Map<string,int> = new Map<string,int>(("a", 1), ("b", 2)); m["b"] = 9; m["b"]`)
	testIntegerObject(t, evaluated, 9)

	evaluated = testEval(`let m: Map<string,int> = new Map<string,int>(("a", 1), ("b", 2)); m.length()`)
	testIntegerObject(t, evaluated, 2)

	evaluated = testEval(`let m: Map<string,int> = new Map<string,int>(("a", 1), ("b", 2)); m.has("a")`)
	testBooleanObject(t, evaluated, true)

	evaluated = testEval(`let m: Map<string,int> = new Map<string,int>(("a", 1), ("b", 2)); m.removeKey("a")`)
	testIntegerObject(t, evaluated, 1)

	evaluated = testEval(`let m: Map<string,int> = new Map<string,int>(("a", 1)); m.clear(); m.length`)
	testIntegerObject(t, evaluated, 0)
}

func TestMapEvalErrors(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`let m: Map<string,int> = new Map<string,int>(("a", 1)); m[1]`, "cannot use int as map key type string"},
		{`let m: Map<string,int> = new Map<string,int>(("a", 1)); m[1] = 2; m["a"]`, "cannot assign key type int to string"},
		{`let m: Map<string,int> = new Map<string,int>(("a", "x")); m["a"]`, "cannot assign string to int"},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		errObj, ok := evaluated.(*object.Error)
		if !ok {
			t.Fatalf("expected error object for %q, got=%T", tt.input, evaluated)
		}
		if errObj.Message != tt.want {
			t.Fatalf("wrong error message for %q: got=%q want=%q", tt.input, errObj.Message, tt.want)
		}
	}
}

func TestStructEvalBasics(t *testing.T) {
	src := `
struct Point { x: int, y?: int, z: int = 7 }
let p = new Point(1);
p.y = 4;
p.x + p.y + p.z
`
	evaluated := testEval(src)
	testIntegerObject(t, evaluated, 12)

	evaluated = testEval(`
struct Point { x: int, y?: int, z: int = 7 }
let p: Point = new Point(x = 2, y = 3);
p.z
`)
	testIntegerObject(t, evaluated, 7)
}

func TestStructEvalErrors(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`struct Point { x: int }; let p = new Point();`, "missing required field x in Point constructor"},
		{`struct Point { x: int }; let p = new Point(1); p.y`, "unknown member: y"},
		{`struct Point { x: int }; let p = new Point(1); p.x = "s";`, "cannot assign string to int"},
	}
	for _, tt := range tests {
		evaluated := testEval(tt.input)
		errObj, ok := evaluated.(*object.Error)
		if !ok {
			t.Fatalf("expected error object for %q, got=%T", tt.input, evaluated)
		}
		if errObj.Message != tt.want {
			t.Fatalf("wrong error message for %q: got=%q want=%q", tt.input, errObj.Message, tt.want)
		}
	}
}

func TestIndexOperatorTypeErrorsEval(t *testing.T) {
	evaluated := testEval(`1[0]`)
	errObj, ok := evaluated.(*object.Error)
	if !ok {
		t.Fatalf("expected error object, got=%T", evaluated)
	}
	if errObj.Message != "index operator not supported: INTEGER" {
		t.Fatalf("wrong error message: %q", errObj.Message)
	}
}

func TestTypeDeclarationErrorsEval(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`type int = string;`, "cannot redefine builtin type: int"},
		{`type A = int; type A = string;`, "type already declared: A"},
		{`type A = unknown_type;`, "unknown type: unknown_type"},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		errObj, ok := evaluated.(*object.Error)
		if !ok {
			t.Fatalf("expected error object for %q, got=%T", tt.input, evaluated)
		}
		if errObj.Message != tt.want {
			t.Fatalf("wrong error message for %q: got=%q want=%q", tt.input, errObj.Message, tt.want)
		}
	}
}

func TestEvaluatorAdditionalCoverageHelpers(t *testing.T) {
	env := object.NewEnvironment()
	results := evalExpressions([]ast.Expression{
		&ast.IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "1"}, Value: 1},
		&ast.IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "2"}, Value: 2},
	}, env)
	if len(results) != 2 {
		t.Fatalf("evalExpressions expected 2 results, got=%d", len(results))
	}

	if got := evalPrefixExpression("-", &object.Integer{Value: 5}); got.(*object.Integer).Value != -5 {
		t.Fatalf("prefix minus on int failed")
	}
	if got := evalPrefixExpression("-", &object.Float{Value: 2.5}); got.(*object.Float).Value != -2.5 {
		t.Fatalf("prefix minus on float failed")
	}
	if got := evalPrefixExpression("!", TRUE); got != FALSE {
		t.Fatalf("bang true failed")
	}
	if got := evalPrefixExpression("!", FALSE); got != TRUE {
		t.Fatalf("bang false failed")
	}
	if got := evalPrefixExpression("!", NULL); got != TRUE {
		t.Fatalf("bang null failed")
	}
	if got := evalPrefixExpression("?", TRUE); got.(*object.Error).Message == "" {
		t.Fatalf("unknown prefix should error")
	}
}

func TestEvaluatorFloatAndCastCoverage(t *testing.T) {
	tests := []struct {
		input string
	}{
		{"1.0 + 2.0"},
		{"4.0 - 1.0"},
		{"3.0 * 2.0"},
		{"9.0 / 3.0"},
		{"5.5 % 2.0"},
		{"1.0 < 2.0"},
		{"2.0 > 1.0"},
		{"2.0 == 2.0"},
		{"2.0 != 3.0"},
		{"int(3.8)"},
		{"int('A')"},
		{"int(\"12\")"},
		{"float(3)"},
		{"float('A')"},
		{"float(\"3.5\")"},
		{"string(3)"},
		{"string(3.5)"},
		{"char(65)"},
		{"char(\"A\")"},
		{"bool(1)"},
		{"bool(0.0)"},
		{"bool(\"x\")"},
	}
	for _, tt := range tests {
		got := testEval(tt.input)
		if errObj, ok := got.(*object.Error); ok {
			t.Fatalf("unexpected error for %q: %s", tt.input, errObj.Message)
		}
	}

	errCases := []struct {
		input string
	}{
		{"float(\"bad\")"},
		{"char(\"ab\")"},
		{"char(true)"},
		{"int(\"bad\")"},
	}
	for _, tt := range errCases {
		if _, ok := testEval(tt.input).(*object.Error); !ok {
			t.Fatalf("expected error for %q", tt.input)
		}
	}
}

func TestEvaluatorTypeHelperWrappers(t *testing.T) {
	env := object.NewEnvironment()
	env.SetTypeAlias("Num", "int")
	if base, dims, ok := parseTypeName("Num[2]"); !ok || base != "Num" || len(dims) != 1 || dims[0] != 2 {
		t.Fatalf("parseTypeName unexpected: %q %v %v", base, dims, ok)
	}
	if got := formatTypeName("int||string", []int{2}); got != "(int||string)[2]" {
		t.Fatalf("formatTypeName unexpected: %q", got)
	}
	if got := normalizeTypeName("Num[2]", env); got != "int[2]" {
		t.Fatalf("normalizeTypeName unexpected: %q", got)
	}
	if got, ok := resolveTypeName("Num", env, nil); !ok || got != "int" {
		t.Fatalf("resolveTypeName unexpected: %q %v", got, ok)
	}
	if !isBuiltinTypeName("int") || isBuiltinTypeName("nope") {
		t.Fatalf("isBuiltinTypeName unexpected")
	}
	if !typeAllowsNull("int||null", env) || typeAllowsNull("int", env) {
		t.Fatalf("typeAllowsNull unexpected")
	}
	if parts, ok := splitTopLevelUnion("int||string||bool"); !ok || len(parts) != 3 {
		t.Fatalf("splitTopLevelUnion unexpected: %v %v", parts, ok)
	}
	if parts, ok := splitTopLevelTuple("(int,string,bool)"); !ok || len(parts) != 3 {
		t.Fatalf("splitTopLevelTuple unexpected: %v %v", parts, ok)
	}
}

func TestEvaluatorDirectInfixHelpers(t *testing.T) {
	lf := &object.Float{Value: 7.5}
	rf := &object.Float{Value: 2.5}
	for _, op := range []string{"+", "-", "*", "/", "%", "<", ">", "==", "!="} {
		got := evalFloatInfixExpression(op, lf, rf)
		if got == nil {
			t.Fatalf("nil result for float op %q", op)
		}
	}
	if errObj, ok := evalFloatInfixExpression("?", lf, rf).(*object.Error); !ok || errObj.Message == "" {
		t.Fatalf("expected float unknown-op error")
	}

	ls := &object.String{Value: "a"}
	rs := &object.String{Value: "b"}
	for _, op := range []string{"+", "==", "!="} {
		if got := evalStringInfixExpression(op, ls, rs); got == nil {
			t.Fatalf("nil result for string op %q", op)
		}
	}
	if _, ok := evalStringInfixExpression("?", ls, rs).(*object.Error); !ok {
		t.Fatalf("expected string unknown-op error")
	}

	lc := &object.Char{Value: 'a'}
	rc := &object.Char{Value: 'b'}
	for _, op := range []string{"+", "==", "!="} {
		if got := evalCharInfixExpression(op, lc, rc); got == nil {
			t.Fatalf("nil result for char op %q", op)
		}
	}
	if _, ok := evalCharInfixExpression("?", lc, rc).(*object.Error); !ok {
		t.Fatalf("expected char unknown-op error")
	}

	lt := &object.TypeValue{Name: "int"}
	rt := &object.TypeValue{Name: "string"}
	for _, op := range []string{"==", "!="} {
		if got := evalTypeValueInfixExpression(op, lt, rt); got == nil {
			t.Fatalf("nil result for type op %q", op)
		}
	}
	if _, ok := evalTypeValueInfixExpression("?", lt, rt).(*object.Error); !ok {
		t.Fatalf("expected type unknown-op error")
	}
}

func TestEvaluatorDirectConcatAndTruthinessHelpers(t *testing.T) {
	left := &object.String{Value: "x:"}
	rights := []object.Object{
		&object.String{Value: "s"},
		&object.Integer{Value: 7},
		&object.Float{Value: 3.5},
		&object.Char{Value: 'A'},
	}
	for _, r := range rights {
		if got := evalStringConcatWithCoercion(left, r); got == nil {
			t.Fatalf("nil concat result for %T", r)
		}
	}
	if _, ok := evalStringConcatWithCoercion(left, &object.Boolean{Value: true}).(*object.Error); !ok {
		t.Fatalf("expected concat type mismatch error")
	}

	for _, l := range []object.Object{
		&object.String{Value: "s"},
		&object.Integer{Value: 7},
		&object.Float{Value: 3.5},
		&object.Char{Value: 'A'},
	} {
		if got := evalStringConcatWithCoercionRight(l, &object.String{Value: ":x"}); got == nil {
			t.Fatalf("nil right-concat result for %T", l)
		}
	}
	if _, ok := evalStringConcatWithCoercionRight(&object.Boolean{Value: true}, &object.String{Value: ":x"}).(*object.Error); !ok {
		t.Fatalf("expected right-concat type mismatch error")
	}

	if isTruthy(NULL) {
		t.Fatalf("null should be falsy")
	}
	if !isTruthy(TRUE) {
		t.Fatalf("true should be truthy")
	}
	if isTruthy(FALSE) {
		t.Fatalf("false should be falsy")
	}
	if !isTruthy(&object.Integer{Value: 0}) {
		t.Fatalf("non-null non-bool should be truthy by current rules")
	}
}

func TestStringIndexingEval(t *testing.T) {
	evaluated := testEval(`"abc"[1]`)
	got, ok := unwrapReturn(evaluated).(*object.Char)
	if !ok {
		t.Fatalf("expected char object, got=%T", evaluated)
	}
	if got.Value != 'b' {
		t.Fatalf("wrong indexed char: got=%q want=%q", got.Value, 'b')
	}

	evaluated = testEval(`let s = "xyz"; s[2]`)
	got, ok = unwrapReturn(evaluated).(*object.Char)
	if !ok {
		t.Fatalf("expected char object, got=%T", evaluated)
	}
	if got.Value != 'z' {
		t.Fatalf("wrong indexed char: got=%q want=%q", got.Value, 'z')
	}
}

func TestStringIndexingErrorsEval(t *testing.T) {
	evaluated := testEval(`"abc"[9]`)
	errObj, ok := evaluated.(*object.Error)
	if !ok {
		t.Fatalf("expected error object, got=%T", evaluated)
	}
	if errObj.Message != "string index out of bounds: 9" {
		t.Fatalf("wrong error message: %q", errObj.Message)
	}

	evaluated = testEval(`"abc"[true]`)
	errObj, ok = evaluated.(*object.Error)
	if !ok {
		t.Fatalf("expected error object, got=%T", evaluated)
	}
	if errObj.Message != "index must be int, got bool" {
		t.Fatalf("wrong error message: %q", errObj.Message)
	}
}

func TestUnionTypesEval(t *testing.T) {
	evaluated := testEval(`let v: int||string = 1; v = "ok"; v`)
	if evaluated.Type() != object.STRING_OBJ || evaluated.Inspect() != "ok" {
		t.Fatalf("expected string value ok, got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}

	evaluated = testEval(`let v: int||string = 1; typeof(v)`)
	if evaluated.Type() != object.TYPE_OBJ || evaluated.Inspect() != "int||string" {
		t.Fatalf("expected type(int||string), got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}

	evaluated = testEval(`let xs: (int||string)[3] = {1, "two", 3}; typeof(xs)`)
	if evaluated.Type() != object.TYPE_OBJ || evaluated.Inspect() != "(int||string)[3]" {
		t.Fatalf("expected type((int||string)[3]), got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}

	evaluated = testEval(`let v: int||string||bool = 1; v = "ok"; v = true; v`)
	testBooleanObject(t, evaluated, true)

	evaluated = testEval(`let v: int||string||bool = 1; typeof(v)`)
	if evaluated.Type() != object.TYPE_OBJ || evaluated.Inspect() != "int||string||bool" {
		t.Fatalf("expected type(int||string||bool), got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}
}

func TestGenericTypeAliasesEval(t *testing.T) {
	evaluated := testEval(`type Pair<T, U> = (T, U); let p: Pair<int, string> = (7, "seven"); p.0`)
	testIntegerObject(t, evaluated, 7)

	evaluated = testEval(`type Pair<T, U> = (T, U); let p: Pair<int, string> = (7, "seven"); typeof(p)`)
	if evaluated.Type() != object.TYPE_OBJ || evaluated.Inspect() != "Pair<int,string>" {
		t.Fatalf("expected normalized tuple type, got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}

	evaluated = testEval(`type Box<T> = T[2]; let xs: Box<int>; xs[0] = 4; xs[1] = 9; xs[1]`)
	testIntegerObject(t, evaluated, 9)
}

func TestGenericTypeAliasArityErrorsEval(t *testing.T) {
	evaluated := testEval(`type Pair<T, U> = (T, U); let p: Pair<int> = (1, "x");`)
	errObj, ok := evaluated.(*object.Error)
	if !ok {
		t.Fatalf("expected error object, got=%T", evaluated)
	}
	if errObj.Message != "wrong number of generic type arguments for Pair: expected 2, got 1" {
		t.Fatalf("wrong error message: %q", errObj.Message)
	}

	evaluated = testEval(`type N = int; let x: N<string> = 1;`)
	errObj, ok = evaluated.(*object.Error)
	if !ok {
		t.Fatalf("expected error object, got=%T", evaluated)
	}
	if errObj.Message != "wrong number of generic type arguments for N: expected 0, got 1" {
		t.Fatalf("wrong error message: %q", errObj.Message)
	}
}

func TestGenericFunctionEval(t *testing.T) {
	evaluated := testEval(`fn id<T>(x: T) T { return x; } id(11);`)
	testIntegerObject(t, evaluated, 11)

	evaluated = testEval(`fn first<T>(a: T, b: T) T { return a; } first(3, 5);`)
	testIntegerObject(t, evaluated, 3)

	evaluated = testEval(`fn id<T>(x: T) T { return x; } id<int>(11);`)
	testIntegerObject(t, evaluated, 11)
}

func TestGenericFunctionExplicitTypeArgErrorsEval(t *testing.T) {
	evaluated := testEval(`fn id<T>(x: T) T { return x; } id<int, string>(11);`)
	errObj, ok := evaluated.(*object.Error)
	if !ok {
		t.Fatalf("expected error object, got=%T", evaluated)
	}
	if errObj.Message != "wrong number of generic type arguments: expected 1, got 2" {
		t.Fatalf("wrong error message: %q", errObj.Message)
	}

	evaluated = testEval(`fn id<T>(x: T) T { return x; } id<unknown>(11);`)
	errObj, ok = evaluated.(*object.Error)
	if !ok {
		t.Fatalf("expected error object, got=%T", evaluated)
	}
	if errObj.Message != "unknown type: unknown" {
		t.Fatalf("wrong error message: %q", errObj.Message)
	}

	evaluated = testEval(`fn passthrough<T>(x: int) int { return x; } passthrough(5);`)
	errObj, ok = evaluated.(*object.Error)
	if !ok {
		t.Fatalf("expected error object, got=%T", evaluated)
	}
	if errObj.Message != "could not infer generic type argument for T; specify it explicitly" {
		t.Fatalf("wrong error message: %q", errObj.Message)
	}

	evaluated = testEval(`fn passthrough<T>(x: int) int { return x; } passthrough<int>(5);`)
	testIntegerObject(t, evaluated, 5)

	evaluated = testEval(`
type Box<T> = T[2];
fn first<T>(b: Box<T>) T { return b[0]; }
let b: Box<int>;
b[0] = 4;
b[1] = 9;
first(b);
`)
	testIntegerObject(t, evaluated, 4)
}

func TestTypeAliasesEval(t *testing.T) {
	evaluated := testEval(`type NumOrText = int||string; let v: NumOrText = "ok"; v`)
	if evaluated.Type() != object.STRING_OBJ || evaluated.Inspect() != "ok" {
		t.Fatalf("expected aliased union assignment to work, got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}

	evaluated = testEval(`type Row = int[2]; type Grid = Row[2]; let g: Grid = {{1,2}, {3,4}}; typeof(g)`)
	if evaluated.Type() != object.TYPE_OBJ || evaluated.Inspect() != "Grid" {
		t.Fatalf("expected declared type name Grid, got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}

	evaluated = testEval(`type N = int; fn add1(x: N) N { return x + 1; } add1(2)`)
	testIntegerObject(t, evaluated, 3)

	evaluated = testEval(`let v: int[2]||string = {1,2}; v = "ok"; v`)
	if evaluated.Type() != object.STRING_OBJ || evaluated.Inspect() != "ok" {
		t.Fatalf("expected int[2]||string to accept string assignment, got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}

	evaluated = testEval(`let v: int[2]||string = {1,2}; typeof(v)`)
	if evaluated.Type() != object.TYPE_OBJ || evaluated.Inspect() != "int[2]||string" {
		t.Fatalf("expected declared type int[2]||string, got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}
}

func TestTupleTypesAndAccessEval(t *testing.T) {
	evaluated := testEval(`let a: (int,string,bool) = (1, "x", true); a.0`)
	testIntegerObject(t, evaluated, 1)

	evaluated = testEval(`let a: (int,string,bool) = (1, "x", true); a.1`)
	if evaluated.Type() != object.STRING_OBJ || evaluated.Inspect() != "x" {
		t.Fatalf("expected string tuple element, got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}

	evaluated = testEval(`let a: (int,string,bool) = (1, "x", true); a.2`)
	testBooleanObject(t, evaluated, true)

	evaluated = testEval(`let a: (int,string,bool) = (1, "x", true); typeof(a)`)
	if evaluated.Type() != object.TYPE_OBJ || evaluated.Inspect() != "(int,string,bool)" {
		t.Fatalf("expected tuple type name, got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}
}

func TestTupleUnionAndAliasEval(t *testing.T) {
	evaluated := testEval(`let v: (int,string)||string = (1, "x"); v = "ok"; v`)
	if evaluated.Type() != object.STRING_OBJ || evaluated.Inspect() != "ok" {
		t.Fatalf("expected tuple-union to accept string reassignment, got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}

	evaluated = testEval(`type Pair = (int,string); let p: Pair = (1, "x"); p.1`)
	if evaluated.Type() != object.STRING_OBJ || evaluated.Inspect() != "x" {
		t.Fatalf("expected tuple alias access to work, got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}

	evaluated = testEval(`type MaybePair = (int,string)||string; let v: MaybePair = (1, "x"); typeof(v)`)
	if evaluated.Type() != object.TYPE_OBJ || evaluated.Inspect() != "MaybePair" {
		t.Fatalf("expected typeof to preserve tuple-union alias, got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}
}

func TestAnyTypeEval(t *testing.T) {
	evaluated := testEval(`let x: any = 3; x = "ok"; typeof(x)`)
	if evaluated.Type() != object.TYPE_OBJ || evaluated.Inspect() != "any" {
		t.Fatalf("expected typeof(x) to be any, got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}

	evaluated = testEval(`let x: any = 3; x = "ok"; typeofValue(x)`)
	if evaluated.Type() != object.TYPE_OBJ || evaluated.Inspect() != "string" {
		t.Fatalf("expected typeofValue(x) to be string, got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}

	evaluated = testEval(`let x: any = 3; x + 4`)
	testIntegerObject(t, evaluated, 7)

	evaluated = testEval(`let x: any; x`)
	if evaluated.Type() != object.NULL_OBJ {
		t.Fatalf("expected default any value to be null, got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}
}

func TestRuntimeErrorIncludesPrefixLocationAndContext(t *testing.T) {
	evaluated := testEval("let arr: int[2] = {1, 2}; let i = 9; arr[i]")
	errObj, ok := evaluated.(*object.Error)
	if !ok {
		t.Fatalf("expected error object, got=%T", evaluated)
	}
	if errObj.Message != "array index out of bounds: 9" {
		t.Fatalf("wrong error message: %q", errObj.Message)
	}
	if errObj.Line <= 0 || errObj.Column <= 0 {
		t.Fatalf("expected runtime error location, got line=%d col=%d", errObj.Line, errObj.Column)
	}
	if errObj.Context == "" {
		t.Fatalf("expected runtime error context to be populated")
	}
	inspected := errObj.Inspect()
	if !strings.Contains(inspected, "Runtime error: array index out of bounds: 9") {
		t.Fatalf("expected standardized runtime error prefix, got: %q", inspected)
	}
	if !strings.Contains(inspected, "context: arr[i]") {
		t.Fatalf("expected runtime error context in inspect output, got: %q", inspected)
	}
}

func testEval(input string) object.Object {
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	env := object.NewEnvironment()
	return Eval(program, env)
}

func testIntegerObject(t *testing.T, obj object.Object, expected int64) {
	t.Helper()
	obj = unwrapReturn(obj)
	result, ok := obj.(*object.Integer)
	if !ok {
		t.Fatalf("object is not Integer. got=%T (%+v)", obj, obj)
	}
	if result.Value != expected {
		t.Fatalf("object has wrong value. got=%d, want=%d", result.Value, expected)
	}
}

func testBooleanObject(t *testing.T, obj object.Object, expected bool) {
	t.Helper()
	obj = unwrapReturn(obj)
	result, ok := obj.(*object.Boolean)
	if !ok {
		t.Fatalf("object is not Boolean. got=%T (%+v)", obj, obj)
	}
	if result.Value != expected {
		t.Fatalf("object has wrong value. got=%t, want=%t", result.Value, expected)
	}
}

func unwrapReturn(obj object.Object) object.Object {
	for {
		rv, ok := obj.(*object.ReturnValue)
		if !ok {
			return obj
		}
		obj = rv.Value
	}
}
