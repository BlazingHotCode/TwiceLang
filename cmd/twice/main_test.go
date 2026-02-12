package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCLIParseError(t *testing.T) {
	root := repoRoot(t)

	srcPath := filepath.Join(t.TempDir(), "bad.tw")
	source := "let x = 1\nprint(x);\n"
	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	cmd := exec.Command("go", "run", "./cmd/twice", srcPath)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected parse failure, got success. output:\n%s", out)
	}

	output := string(out)
	if !strings.Contains(output, "Parse error:") {
		t.Fatalf("missing parse error prefix. output:\n%s", output)
	}
	if !strings.Contains(output, "expected next token to be ;") {
		t.Fatalf("missing semicolon diagnostic. output:\n%s", output)
	}
}

func TestCLICompileAndRunPrint(t *testing.T) {
	root := repoRoot(t)
	ensureToolchain(t)

	srcPath := filepath.Join(t.TempDir(), "ok.tw")
	source := "print(42);\n"
	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	outputName := "twice_cli_smoke_bin"
	outputPath := filepath.Join(root, outputName)
	_ = os.Remove(outputPath)
	t.Cleanup(func() { _ = os.Remove(outputPath) })

	cmd := exec.Command("go", "run", "./cmd/twice", "-run", "-o", outputName, srcPath)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compile/run failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "Compiled to: "+outputName) {
		t.Fatalf("missing compile success message. output:\n%s", output)
	}
	if !strings.Contains(output, "\n42\n") {
		t.Fatalf("expected runtime print output. output:\n%s", output)
	}
}

func TestCLICompileAndRunPrintWithAbsoluteOutputPath(t *testing.T) {
	root := repoRoot(t)
	ensureToolchain(t)

	srcPath := filepath.Join(t.TempDir(), "ok_abs.tw")
	source := "print(9);\n"
	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	outputPath := filepath.Join(t.TempDir(), "twice_cli_abs_bin")
	_ = os.Remove(outputPath)
	t.Cleanup(func() { _ = os.Remove(outputPath) })

	cmd := exec.Command("go", "run", "./cmd/twice", "-run", "-o", outputPath, srcPath)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compile/run failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "Compiled to: "+outputPath) {
		t.Fatalf("missing compile success message. output:\n%s", output)
	}
	if !strings.Contains(output, "\n9\n") {
		t.Fatalf("expected runtime print output. output:\n%s", output)
	}
}

func TestCLICompileAndRunPrintNegativeNumber(t *testing.T) {
	root := repoRoot(t)
	ensureToolchain(t)

	srcPath := filepath.Join(t.TempDir(), "neg.tw")
	source := "print(-17);\n"
	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	outputName := "twice_cli_negative_bin"
	outputPath := filepath.Join(root, outputName)
	_ = os.Remove(outputPath)
	t.Cleanup(func() { _ = os.Remove(outputPath) })

	cmd := exec.Command("go", "run", "./cmd/twice", "-run", "-o", outputName, srcPath)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compile/run failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "\n-17\n") {
		t.Fatalf("expected runtime print output. output:\n%s", output)
	}
}

func TestCLICodegenErrorForPrintBoolean(t *testing.T) {
	root := repoRoot(t)

	srcPath := filepath.Join(t.TempDir(), "bad_print_bool.tw")
	source := "print(true);\n"
	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	cmd := exec.Command("go", "run", "./cmd/twice", srcPath)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected codegen failure, got success. output:\n%s", out)
	}

	output := string(out)
	if !strings.Contains(output, "Codegen error: print expects integer argument, got boolean") {
		t.Fatalf("missing codegen type diagnostic. output:\n%s", output)
	}
}

func TestCLICodegenErrorForPrintArity(t *testing.T) {
	root := repoRoot(t)

	srcPath := filepath.Join(t.TempDir(), "bad_print_arity.tw")
	source := "print();\n"
	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	cmd := exec.Command("go", "run", "./cmd/twice", srcPath)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected codegen failure, got success. output:\n%s", out)
	}

	output := string(out)
	if !strings.Contains(output, "Codegen error: print expects exactly 1 argument") {
		t.Fatalf("missing codegen arity diagnostic. output:\n%s", output)
	}
}

func TestCLICompileAndRunConstValue(t *testing.T) {
	root := repoRoot(t)
	ensureToolchain(t)

	srcPath := filepath.Join(t.TempDir(), "const_ok.tw")
	source := "const x = 8;\nprint(x);\n"
	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	outputName := "twice_cli_const_bin"
	outputPath := filepath.Join(root, outputName)
	_ = os.Remove(outputPath)
	t.Cleanup(func() { _ = os.Remove(outputPath) })

	cmd := exec.Command("go", "run", "./cmd/twice", "-run", "-o", outputName, srcPath)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compile/run failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "\n8\n") {
		t.Fatalf("expected runtime print output. output:\n%s", output)
	}
}

func TestCLICodegenErrorForConstRedeclare(t *testing.T) {
	root := repoRoot(t)

	srcPath := filepath.Join(t.TempDir(), "const_redeclare.tw")
	source := "const x = 1;\nlet x = 2;\n"
	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	cmd := exec.Command("go", "run", "./cmd/twice", srcPath)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected codegen failure, got success. output:\n%s", out)
	}

	output := string(out)
	if !strings.Contains(output, "Codegen error: cannot reassign const: x") {
		t.Fatalf("missing const redeclare diagnostic. output:\n%s", output)
	}
}

func TestCLICompileAndRunVariableReassignment(t *testing.T) {
	root := repoRoot(t)
	ensureToolchain(t)

	srcPath := filepath.Join(t.TempDir(), "assign_ok.tw")
	source := "let x = 1;\nx = 4;\nprint(x);\n"
	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	outputName := "twice_cli_assign_bin"
	outputPath := filepath.Join(root, outputName)
	_ = os.Remove(outputPath)
	t.Cleanup(func() { _ = os.Remove(outputPath) })

	cmd := exec.Command("go", "run", "./cmd/twice", "-run", "-o", outputName, srcPath)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compile/run failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "\n4\n") {
		t.Fatalf("expected runtime print output. output:\n%s", output)
	}
}

func TestCLICodegenErrorForConstReassignment(t *testing.T) {
	root := repoRoot(t)

	srcPath := filepath.Join(t.TempDir(), "const_reassign.tw")
	source := "const x = 1;\nx = 2;\n"
	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	cmd := exec.Command("go", "run", "./cmd/twice", srcPath)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected codegen failure, got success. output:\n%s", out)
	}

	output := string(out)
	if !strings.Contains(output, "Codegen error: cannot reassign const: x") {
		t.Fatalf("missing const reassignment diagnostic. output:\n%s", output)
	}
}

func ensureToolchain(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("as"); err != nil {
		t.Skip("skipping: assembler 'as' not found")
	}
	if _, err := exec.LookPath("gcc"); err != nil {
		if _, err2 := exec.LookPath("ld"); err2 != nil {
			t.Skip("skipping: neither 'gcc' nor 'ld' found")
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to locate test file path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
