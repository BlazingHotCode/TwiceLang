package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	if len(cg.DetailedErrors()) > 0 {
		for _, err := range cg.DetailedErrors() {
			printCodegenError(sourceFile, string(source), err)
		}
		os.Exit(1)
	}

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

func printCodegenError(filename string, source string, err codegen.CodegenError) {
	fmt.Printf("Codegen error: %s\n", err.Message)
	line, col, ok := 0, 0, false
	if err.Line > 0 && err.Column > 0 {
		line, col, ok = err.Line, err.Column, true
	} else {
		line, col, ok = findContextLocation(source, err.Context)
	}
	if !ok {
		fmt.Printf("  --> %s\n", filename)
		if err.Context != "" {
			fmt.Printf("   | context: %s\n", err.Context)
		}
		return
	}

	lines := strings.Split(source, "\n")
	fmt.Printf("  --> %s:%d:%d\n", filename, line, col)
	fmt.Println("   |")
	if line-2 >= 0 {
		fmt.Printf("%3d| %s\n", line-1, lines[line-2])
	}
	fmt.Printf("%3d| %s\n", line, lines[line-1])
	fmt.Printf("   | %s^\n", strings.Repeat(" ", col-1))
	if line < len(lines) {
		fmt.Printf("%3d| %s\n", line+1, lines[line])
	}
}

func findContextLocation(source string, context string) (line int, col int, ok bool) {
	ctx := strings.TrimSpace(context)
	if ctx == "" {
		return 0, 0, false
	}
	lines := strings.Split(source, "\n")
	normalize := func(s string) string {
		s = strings.TrimSpace(s)
		s = strings.ReplaceAll(s, " ", "")
		s = strings.ReplaceAll(s, "\t", "")
		return s
	}
	normalizedCtx := normalize(strings.Trim(ctx, "`"))

	// Prefer exact normalized line matches so `return ;` maps to `return;`.
	// If there are multiple identical matches, avoid a misleading location.
	matchLine := -1
	for i, ln := range lines {
		if normalize(ln) == normalizedCtx {
			if matchLine != -1 {
				matchLine = -2 // ambiguous
				break
			}
			matchLine = i
		}
	}
	if matchLine >= 0 {
		ln := lines[matchLine]
		col := strings.Index(ln, strings.TrimSpace(strings.Trim(ctx, "`")))
		if col < 0 {
			col = strings.Index(ln, "return")
		}
		if col < 0 {
			col = 0
		}
		return matchLine + 1, col + 1, true
	}

	candidates := []string{
		ctx,
		strings.Trim(ctx, "`"),
	}
	bestLine := -1
	bestCol := -1
	for i, ln := range lines {
		for _, c := range candidates {
			if c == "" {
				continue
			}
			if idx := strings.Index(ln, c); idx >= 0 {
				if bestLine != -1 {
					return 0, 0, false // ambiguous
				}
				bestLine = i + 1
				bestCol = idx + 1
			}
		}
	}
	if bestLine != -1 {
		return bestLine, bestCol, true
	}
	return 0, 0, false
}
