package main

import (
	"fmt"
	"twice/internal/evaluator"
	"twice/internal/lexer"
	"twice/internal/object"
	"twice/internal/parser"
)

func main() {
	// Test various programs
	tests := []string{
		"5 + 5",
		"let x = 5; x",
		"let a = 5; let b = 10; a + b",
		"fn(x) { x + 5 }(10)",
		"let add = fn(a, b) { a + b; }; add(5, 10)",
		"if (5 < 10) { 10 } else { 5 }",
		"if (5 > 10) { 10 } else { 5 }",
		"!!true",
		"5 + true",
	}

	for _, input := range tests {
		fmt.Printf("Input: %s\n", input)
		
		l := lexer.New(input)
		p := parser.New(l)
		program := p.ParseProgram()

		if len(p.Errors()) != 0 {
			for _, err := range p.Errors() {
				fmt.Printf("  Parse error: %s\n", err)
			}
			continue
		}

		env := object.NewEnvironment()
		result := evaluator.Eval(program, env)
		
		fmt.Printf("  Result: %s\n\n", result.Inspect())
	}

}
