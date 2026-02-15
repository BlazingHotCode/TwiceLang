package parser

import (
	"testing"

	"twice/internal/lexer"
)

// FuzzParserNoPanic ensures parsing never panics for arbitrary input.
func FuzzParserNoPanic(f *testing.F) {
	seeds := []string{
		"",
		"let x = 1;",
		"const y = 2;",
		"fn add(a: int, b: int = 2) int { return a + b; }",
		"let arr = {1,2,3}; arr[1] = 9;",
		"let a: int||string = 3; if (a == 3) { print(\"ok\"); };",
		"let x = null ?? 7;",
		"let a: int[3]; a?.length();",
		"let x = a ?? b || c;",
		"let x = (a || b) ?? c;",
		"let t: (int,string) = (1, \"x\"); t.0;",
		"for (let i = 0; i < 3; i++) { if (i == 1) { continue; }; };",
		"while (true) { break; };",
		"{ let scoped = 1; }",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("parser panicked for input %q: %v", input, r)
			}
		}()

		l := lexer.New(input)
		p := New(l)
		program := p.ParseProgram()
		if program != nil {
			_ = program.String()
		}
		_ = p.Errors()
	})
}

