package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"twice/internal/codegen"
	"twice/internal/lexer"
	"twice/internal/parser"
)

func main() {
	// Command line flags
	run := flag.Bool("run", false, "Run the generated executable")
	output := flag.String("o", "output", "Output executable name")
	flag.Parse()

	// Check for source file
	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("Usage: twice [-run] [-o output] <source.tw>")
		os.Exit(1)
	}

	sourceFile := args[0]

	// If source is "-", read from stdin
	var source []byte
	var err error

	if sourceFile == "-" {
		source, err = io.ReadAll(os.Stdin)
	} else {
		source, err = os.ReadFile(sourceFile)
	}

	if err != nil {
		fmt.Printf("Error reading input: %v\n", err)
		os.Exit(1)
	}

	// Parse
	l := lexer.New(string(source))
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		for _, err := range p.Errors() {
			fmt.Printf("Parse error: %s\n", err)
		}
		os.Exit(1)
	}

	// Generate code
	cg := codegen.New()
	assembly := cg.Generate(program)

	// Print assembly for debugging
	fmt.Println("=== Generated Assembly ===")
	fmt.Println(assembly)
	fmt.Println("==========================")

	// Compile to executable
	if err := codegen.CompileToExecutable(assembly, *output); err != nil {
		fmt.Printf("Compilation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Compiled to: %s\n", *output)

	// Optionally run
	if *run {
		cmd := exec.Command(runPath(*output))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("Execution failed: %v\n", err)
			os.Exit(1)
		}
	}
}

func runPath(output string) string {
	if filepath.IsAbs(output) || filepath.Dir(output) != "." {
		return output
	}
	return "." + string(os.PathSeparator) + output
}
