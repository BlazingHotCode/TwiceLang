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

func TestCLICompileAndRunPrintBoolean(t *testing.T) {
	root := repoRoot(t)
	ensureToolchain(t)

	srcPath := filepath.Join(t.TempDir(), "print_bool.tw")
	source := "print(true);\nprint(false);\n"
	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	outputName := "twice_cli_print_bool_bin"
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
	if !strings.Contains(output, "\ntrue\nfalse\n") {
		t.Fatalf("expected boolean print output. output:\n%s", output)
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

func TestCLICodegenErrorForPrintUnsupportedType(t *testing.T) {
	root := repoRoot(t)

	srcPath := filepath.Join(t.TempDir(), "bad_print_type.tw")
	source := "print(fn(x) { x; });\n"
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
	if !strings.Contains(output, "Codegen error: print supports only int, bool, float, string, char, null, and type arguments") &&
		!strings.Contains(output, "Codegen error: function literals are not supported in codegen yet") {
		t.Fatalf("missing print unsupported-type diagnostic. output:\n%s", output)
	}
}

func TestCLICodegenErrorForUndefinedIdentifier(t *testing.T) {
	root := repoRoot(t)

	srcPath := filepath.Join(t.TempDir(), "bad_undefined_ident.tw")
	source := "print(y);\n"
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
	if !strings.Contains(output, "Codegen error: identifier not found: y") {
		t.Fatalf("missing undefined identifier diagnostic. output:\n%s", output)
	}
}

func TestCLIStringConcatPrintsSingleLine(t *testing.T) {
	root := repoRoot(t)
	ensureToolchain(t)

	srcPath := filepath.Join(t.TempDir(), "concat_header.tw")
	source := `
fn header(name: string) {
  print("--- " + name + " ---");
  return;
}
fn main() {
  header("arrays unions tuples");
  return;
}
`
	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	outputName := "twice_cli_concat_header_bin"
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
	if !strings.Contains(output, "\n--- arrays unions tuples ---\n") {
		t.Fatalf("expected single-line concatenated header output. output:\n%s", output)
	}
	if strings.Contains(output, "--- \narrays unions tuples\n ---") {
		t.Fatalf("unexpected multi-line concat output. output:\n%s", output)
	}
}

func TestCLIHasFieldRuntimeFieldName(t *testing.T) {
	root := repoRoot(t)
	ensureToolchain(t)

	srcPath := filepath.Join(t.TempDir(), "hasfield_runtime.tw")
	source := `let arr = {1,2,3};
let f: string = "length";
print(hasField(arr, f));
f = "missing";
print(hasField(arr, f));
print(hasField("abc", "length"));
`
	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	outputName := "twice_cli_hasfield_runtime_bin"
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
	if !strings.Contains(output, "\ntrue\nfalse\ntrue\n") {
		t.Fatalf("unexpected hasField runtime output. output:\n%s", output)
	}
}

func TestCLIFunctionLiteralDirectCallAndCapture(t *testing.T) {
	root := repoRoot(t)
	ensureToolchain(t)

	srcPath := filepath.Join(t.TempDir(), "fn_lit_call.tw")
	source := `let y = 3;
let getY = fn() int { return y; };
print((fn(a: int) int { return a + 1; })(2));
print(getY());
`
	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	outputName := "twice_cli_fn_lit_call_bin"
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
	if !strings.Contains(output, "\n3\n3\n") {
		t.Fatalf("unexpected function-literal runtime output. output:\n%s", output)
	}
}

func TestCLICompileAndRunNewTypesTypeofAndCasts(t *testing.T) {
	root := repoRoot(t)
	ensureToolchain(t)

	srcPath := filepath.Join(t.TempDir(), "new_types.tw")
	source := `let s: string = "hi";
let c = 'A';
let f = 3.14;
let n: string;
print(s);
print(c);
print(f);
print(n);
print(typeof(c));
print(int(true));
`
	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	outputName := "twice_cli_new_types_bin"
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
	for _, want := range []string{"\nhi\n", "\nA\n", "\n3.14\n", "\nnull\n", "\nchar\n", "\n1\n"} {
		if !strings.Contains(output, want) {
			t.Fatalf("missing expected output %q. full output:\n%s", want, output)
		}
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
