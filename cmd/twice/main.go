package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"twice/internal/codegen"
	"twice/internal/diag"
	"twice/internal/lexer"
	"twice/internal/parser"
)

var exitFn = os.Exit
var compileFn = codegen.CompileToExecutable
var execCmdFn = exec.Command

func main() {
	exitFn(runCLI(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func runCLI(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	// Command line flags
	fs := flag.NewFlagSet("twice", flag.ContinueOnError)
	fs.SetOutput(stderr)
	run := fs.Bool("run", false, "Run the generated executable")
	output := fs.String("o", "output", "Output executable name")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	// Check for source file
	rest := fs.Args()
	if len(rest) < 1 {
		fmt.Fprintln(stdout, "Usage: twice [-run] [-o output] <source.tw>")
		return 1
	}

	sourceFile := rest[0]

	// If source is "-", read from stdin
	var source []byte
	var err error

	if sourceFile == "-" {
		source, err = io.ReadAll(stdin)
	} else {
		source, err = os.ReadFile(sourceFile)
	}

	if err != nil {
		fmt.Fprintf(stdout, "Error reading input: %v\n", err)
		return 1
	}

	// Parse
	l := lexer.New(string(source))
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		for _, err := range p.DetailedErrors() {
			printParseError(sourceFile, string(source), err)
		}
		return 1
	}

	// Generate code
	cg := codegen.New()
	assembly := cg.Generate(program)
	if len(cg.DetailedErrors()) > 0 {
		for _, err := range cg.DetailedErrors() {
			printCodegenError(sourceFile, string(source), err)
		}
		return 1
	}

	// Print assembly for debugging
	fmt.Fprintln(stdout, "=== Generated Assembly ===")
	fmt.Fprintln(stdout, assembly)
	fmt.Fprintln(stdout, "==========================")

	// Compile to executable
	if err := compileFn(assembly, *output); err != nil {
		fmt.Fprintf(stdout, "Compilation failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "Compiled to: %s\n", *output)

	// Optionally run
	if *run {
		cmd := execCmdFn(runPath(*output))
		var runStdout bytes.Buffer
		var runStderr bytes.Buffer
		cmd.Stdout = &runStdout
		cmd.Stderr = &runStderr
		if err := cmd.Run(); err != nil {
			if runStdout.Len() > 0 {
				_, _ = io.Copy(stdout, &runStdout)
			}
			if runStderr.Len() > 0 {
				formatRuntimeStderr(stderr, sourceFile, string(source), runStderr.String())
			}
			fmt.Fprintf(stdout, "Execution failed: %v\n", err)
			return 1
		}
		if runStdout.Len() > 0 {
			_, _ = io.Copy(stdout, &runStdout)
		}
		if runStderr.Len() > 0 {
			_, _ = io.Copy(stderr, &runStderr)
		}
	}

	return 0
}

type runtimeExecError struct {
	Message string
	Context string
	Line    int
	Column  int
}

func parseRuntimeErrorLine(line string) (runtimeExecError, bool) {
	const prefix = "Runtime error: "
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, prefix) {
		return runtimeExecError{}, false
	}

	payload := strings.TrimPrefix(line, prefix)
	msg := payload
	ctx := ""
	ln := 0
	col := 0

	if idx := strings.Index(payload, " | context: "); idx >= 0 {
		msg = strings.TrimSpace(payload[:idx])
		ctx = strings.TrimSpace(payload[idx+len(" | context: "):])
	}

	if idx := strings.LastIndex(msg, " (at "); idx >= 0 && strings.HasSuffix(msg, ")") {
		loc := strings.TrimSuffix(strings.TrimPrefix(msg[idx:], " (at "), ")")
		parts := strings.Split(loc, ":")
		if len(parts) == 2 {
			if l, err := strconv.Atoi(parts[0]); err == nil {
				if c, err := strconv.Atoi(parts[1]); err == nil {
					ln, col = l, c
					msg = strings.TrimSpace(msg[:idx])
				}
			}
		}
	}

	return runtimeExecError{
		Message: msg,
		Context: ctx,
		Line:    ln,
		Column:  col,
	}, true
}

func formatRuntimeStderr(stderr io.Writer, filename string, source string, stderrText string) {
	lines := strings.Split(stderrText, "\n")
	for _, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		if trimmed == "" {
			continue
		}
		if rerr, ok := parseRuntimeErrorLine(trimmed); ok {
			printRuntimeError(filename, source, rerr)
			continue
		}
		fmt.Fprintln(stderr, ln)
	}
}

func printRuntimeError(filename string, source string, err runtimeExecError) {
	fmt.Printf("Runtime error: %s\n", err.Message)
	line, col, ok := 0, 0, false
	if err.Line > 0 && err.Column > 0 {
		line, col, ok = err.Line, err.Column, true
	} else if err.Context != "" {
		line, col, ok = diag.LocateContext(source, err.Context)
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
		line, col, ok = diag.LocateContext(source, err.Context)
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

func printParseError(filename string, source string, err parser.ParseError) {
	fmt.Printf("Parse error: %s\n", err.Message)
	line, col, ok := 0, 0, false
	if err.Line > 0 && err.Column > 0 {
		line, col, ok = err.Line, err.Column, true
	} else {
		line, col, ok = diag.LocateContext(source, err.Context)
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
