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
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		testBooleanObject(t, evaluated, tt.expected)
	}
}

func TestIfElseEval(t *testing.T) {
	evaluated := testEval("if (1 < 2) { 10 } else { 20 }")
	testIntegerObject(t, evaluated, 10)
}

func TestFunctionCallEval(t *testing.T) {
	evaluated := testEval("let add = fn(x, y) { x + y; }; add(2, 3)")
	testIntegerObject(t, evaluated, 5)
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
