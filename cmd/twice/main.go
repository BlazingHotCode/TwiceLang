package main

import (
	"fmt"
	"twice/internal/lexer"
	"twice/internal/parser"
)

func main() {
	input := `
let x = 5 + 3 * 2;
let y = 10;
let add = fn(a, b) { a + b; };
let result = add(x, y);
if (result > 10) { return true; } else { return false; }
`

	l := lexer.New(input)
	p := parser.New(l)

	program := p.ParseProgram()

	// Check for errors
	if len(p.Errors()) != 0 {
		fmt.Println("Parser errors:")
		for _, err := range p.Errors() {
			fmt.Printf("\t%s\n", err)
		}
		return
	}

	// Print the parsed program (reconstructed from AST)
	fmt.Println("Parsed program:")
	fmt.Println(program.String())
}
