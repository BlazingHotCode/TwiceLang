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
