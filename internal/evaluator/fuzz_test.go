package evaluator

import (
	"testing"

	"twice/internal/lexer"
	"twice/internal/object"
	"twice/internal/parser"
)

// FuzzEvaluatorNoPanic ensures evaluation never panics for arbitrary input.
func FuzzEvaluatorNoPanic(f *testing.F) {
	seeds := []string{
		"",
		"1 + 2;",
		"let x = 1; x = x + 1; x;",
		"if (true) { 1 } else { 2 }",
		"fn add(a: int, b: int = 2) int { return a + b; } add(3);",
		"let arr = {1,2,3}; arr[1];",
		"let arr = {1,2,3}; arr[1] = 9; arr[1];",
		"let s = \"abc\"; s[1];",
		"null ?? 7;",
		"let a: int[3]; a?.length();",
		"let a: int||string = 3; typeofValue(a);",
		"let t: (int,string,bool) = (1,\"x\",true); t.1;",
		"for (let i = 0; i < 3; i++) { if (i == 1) { continue; }; };",
		"loop { break; };",
		"hasField({1,2,3}, \"length\");",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("evaluator panicked for input %q: %v", input, r)
			}
		}()

		l := lexer.New(input)
		p := parser.New(l)
		program := p.ParseProgram()
		if program == nil {
			return
		}

		env := object.NewEnvironment()
		_ = Eval(program, env)
	})
}

