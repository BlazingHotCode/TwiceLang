package evaluator

import (
	"testing"

	"twice/internal/lexer"
	"twice/internal/object"
	"twice/internal/parser"
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

func TestArrayLiteralAndTypedArrayEval(t *testing.T) {
	evaluated := testEval("let arr = {1, 2, 3}; typeof(arr)")
	if evaluated.Type() != object.TYPE_OBJ || evaluated.Inspect() != "int[3]" {
		t.Fatalf("expected type(int[3]), got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}

	evaluated = testEval("let arr: int[3] = {1, 2, 3}; typeof(arr)")
	if evaluated.Type() != object.TYPE_OBJ || evaluated.Inspect() != "int[3]" {
		t.Fatalf("expected type(int[3]), got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}

	evaluated = testEval("let arr: int[] = {1, 2, 3}; typeof(arr)")
	if evaluated.Type() != object.TYPE_OBJ || evaluated.Inspect() != "int[]" {
		t.Fatalf("expected type(int[]), got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}

	evaluated = testEval("let grid: int[][] = {{1}, {2, 3}}; typeof(grid)")
	if evaluated.Type() != object.TYPE_OBJ || evaluated.Inspect() != "int[][]" {
		t.Fatalf("expected type(int[][]), got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}

	evaluated = testEval("let grid = {{1}, {2, 3}}; typeof(grid)")
	if evaluated.Type() != object.TYPE_OBJ || evaluated.Inspect() != "int[][2]" {
		t.Fatalf("expected type(int[][2]), got=%s (%s)", evaluated.Type(), evaluated.Inspect())
	}
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

	evaluated = testEval(`let xs: (int||string)[] = {1, "two", 3}; typeof(xs)`)
	if evaluated.Type() != object.TYPE_OBJ || evaluated.Inspect() != "(int||string)[]" {
		t.Fatalf("expected type((int||string)[]), got=%s (%s)", evaluated.Type(), evaluated.Inspect())
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
