package codegen

import (
	"testing"

	"twice/internal/lexer"
	"twice/internal/parser"
)

// FuzzCodegenNoPanic ensures codegen never panics for arbitrary parser outputs.
func FuzzCodegenNoPanic(f *testing.F) {
	seeds := []string{
		"",
		"print(1);",
		"let x = 1; x = x + 1; print(x);",
		"let arr = {1,2,3}; arr[1] = 9; print(arr[1]);",
		"let t: (int,string) = (1, \"x\"); print(t.0);",
		"let x: int; print(x ?? 7);",
		"let arr: int[3]; print(arr?.length());",
		"fn add(a: int, b: int = 2) int { return a + b; } print(add(3));",
		"for (let i = 0; i < 3; i++) { if (i == 1) { continue; }; };",
		"while (true) { break; };",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("codegen panicked for input %q: %v", input, r)
			}
		}()

		p := parser.New(lexer.New(input))
		program := p.ParseProgram()
		if program == nil {
			return
		}

		cg := New()
		_ = cg.Generate(program)
		_ = cg.Errors()
		_ = cg.DetailedErrors()
	})
}

