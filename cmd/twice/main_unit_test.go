package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"twice/internal/codegen"
	"twice/internal/parser"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String()
}

func TestRunPath(t *testing.T) {
	if got := runPath("bin"); got != "."+string(os.PathSeparator)+"bin" {
		t.Fatalf("runPath relative=%q", got)
	}
	abs := filepath.Join(string(os.PathSeparator), "tmp", "bin")
	if got := runPath(abs); got != abs {
		t.Fatalf("runPath abs=%q", got)
	}
	nested := filepath.Join("out", "bin")
	if got := runPath(nested); got != nested {
		t.Fatalf("runPath nested=%q", got)
	}
}

func TestRunCLINoArgs(t *testing.T) {
	var out bytes.Buffer
	code := runCLI(nil, strings.NewReader(""), &out, &out)
	if code != 1 {
		t.Fatalf("runCLI() code=%d want=1", code)
	}
	if !strings.Contains(out.String(), "Usage: twice") {
		t.Fatalf("expected usage output, got:\n%s", out.String())
	}
}

func TestMainUsesExitFn(t *testing.T) {
	oldArgs := os.Args
	oldExit := exitFn
	oldCompile := compileFn
	defer func() {
		os.Args = oldArgs
		exitFn = oldExit
		compileFn = oldCompile
	}()

	os.Args = []string{"twice"}
	var got int
	exitFn = func(code int) { got = code }
	compileFn = func(_, _ string) error { return errors.New("unused") }
	main()
	if got != 1 {
		t.Fatalf("main exit code=%d want=1", got)
	}
}

func TestRunCLIFromStdinParseAndCodegenErrors(t *testing.T) {
	var out bytes.Buffer
	printed := captureStdout(t, func() {
		code := runCLI([]string{"-"}, strings.NewReader("let x = 1\n"), &out, &out)
		if code != 1 {
			t.Fatalf("expected parse error code=1, got=%d", code)
		}
	})
	if !strings.Contains(printed, "Parse error:") {
		t.Fatalf("expected parse error output, got=%s", printed)
	}

	out.Reset()
	printed = captureStdout(t, func() {
		code := runCLI([]string{"-"}, strings.NewReader("print();\n"), &out, &out)
		if code != 1 {
			t.Fatalf("expected codegen error code=1, got=%d", code)
		}
	})
	if !strings.Contains(printed, "Codegen error:") {
		t.Fatalf("expected codegen error output, got=%s", printed)
	}
}

func TestRunCLICompileAndRunBranches(t *testing.T) {
	src := filepath.Join(t.TempDir(), "ok.tw")
	if err := os.WriteFile(src, []byte("print(1);\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	oldCompile := compileFn
	oldExec := execCmdFn
	defer func() {
		compileFn = oldCompile
		execCmdFn = oldExec
	}()

	var out bytes.Buffer
	compileFn = func(_, _ string) error { return errors.New("compile boom") }
	code := runCLI([]string{"-o", "xbin", src}, strings.NewReader(""), &out, &out)
	if code != 1 || !strings.Contains(out.String(), "Compilation failed:") {
		t.Fatalf("expected compile failure branch, code=%d out=%s", code, out.String())
	}

	out.Reset()
	compileFn = func(_, _ string) error { return nil }
	execCmdFn = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("sh", "-c", "exit 1")
	}
	code = runCLI([]string{"-run", "-o", "xbin", src}, strings.NewReader(""), &out, &out)
	if code != 1 || !strings.Contains(out.String(), "Execution failed:") {
		t.Fatalf("expected execution failure branch, code=%d out=%s", code, out.String())
	}

	out.Reset()
	execCmdFn = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("sh", "-c", "exit 0")
	}
	code = runCLI([]string{"-run", "-o", "xbin", src}, strings.NewReader(""), &out, &out)
	if code != 0 || !strings.Contains(out.String(), "Compiled to:") {
		t.Fatalf("expected successful run branch, code=%d out=%s", code, out.String())
	}
}

func TestPrintParseErrorVariants(t *testing.T) {
	source := "let x = 1;\nprint(x);\n"
	out := captureStdout(t, func() {
		printParseError("f.tw", source, parser.ParseError{Message: "bad", Line: 2, Column: 1})
	})
	if !strings.Contains(out, "Parse error: bad") || !strings.Contains(out, "--> f.tw:2:1") {
		t.Fatalf("unexpected parse error output:\n%s", out)
	}

	out = captureStdout(t, func() {
		printParseError("f.tw", source, parser.ParseError{Message: "bad2", Context: "print(x);"})
	})
	if !strings.Contains(out, "Parse error: bad2") || !strings.Contains(out, "--> f.tw:2:1") {
		t.Fatalf("unexpected parse context output:\n%s", out)
	}

	out = captureStdout(t, func() {
		printParseError("f.tw", source, parser.ParseError{Message: "bad3", Context: "nope"})
	})
	if !strings.Contains(out, "Parse error: bad3") || !strings.Contains(out, "context: nope") {
		t.Fatalf("unexpected parse fallback output:\n%s", out)
	}
}

func TestPrintCodegenErrorVariants(t *testing.T) {
	source := "let x = 1;\nprint(x);\n"
	out := captureStdout(t, func() {
		printCodegenError("f.tw", source, codegen.CodegenError{Message: "bad", Line: 2, Column: 1})
	})
	if !strings.Contains(out, "Codegen error: bad") || !strings.Contains(out, "--> f.tw:2:1") {
		t.Fatalf("unexpected codegen error output:\n%s", out)
	}

	out = captureStdout(t, func() {
		printCodegenError("f.tw", source, codegen.CodegenError{Message: "bad2", Context: "print(x)", Line: 0, Column: 0})
	})
	if !strings.Contains(out, "Codegen error: bad2") {
		t.Fatalf("unexpected codegen context output:\n%s", out)
	}

	out = captureStdout(t, func() {
		printCodegenError("f.tw", source, codegen.CodegenError{Message: "bad3", Context: "nope"})
	})
	if !strings.Contains(out, "Codegen error: bad3") || !strings.Contains(out, "context: nope") {
		t.Fatalf("unexpected codegen fallback output:\n%s", out)
	}
}
