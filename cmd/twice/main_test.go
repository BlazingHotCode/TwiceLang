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
	source := "println(42);\n"
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
	source := "println(9);\n"
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
	source := "println(-17);\n"
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
	source := "println(true);\nprintln(false);\n"
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

func TestCLICompileAndRunEmptyTypedLiterals(t *testing.T) {
	root := repoRoot(t)
	ensureToolchain(t)

	srcPath := filepath.Join(t.TempDir(), "empty_typed.tw")
	source := `
fn main() {
  let arr: int[3] = {};
  println(arr.length());
  let t: (int, string) = ();
  println(t.0 ?? 7);
  return;
}
`
	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	outputName := "twice_cli_empty_typed_bin"
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
	if !strings.Contains(output, "\n3\n7\n") {
		t.Fatalf("expected runtime print output for empty typed literals. output:\n%s", output)
	}
}

func TestCLICompileAndRunListFeature(t *testing.T) {
	root := repoRoot(t)
	ensureToolchain(t)

	srcPath := filepath.Join(t.TempDir(), "list.tw")
	source := `
fn main() int {
  let xs: List<int> = new List<int>(1, 2);
  xs.append(3);
  xs.insert(1, 7);
  println(xs.length());
  println(xs[1]);
  println(xs.remove(2));
  println(xs.pop());
  println(xs.contains(1));
  xs.clear();
  println(xs.length());
  return 0;
}
`
	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	outputName := "twice_cli_list_bin"
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
	if !strings.Contains(output, "\n4\n7\n2\n3\ntrue\n0\n") {
		t.Fatalf("expected list runtime output. output:\n%s", output)
	}
}

func TestCLIListRuntimeErrorFormatting(t *testing.T) {
	root := repoRoot(t)
	ensureToolchain(t)

	srcPath := filepath.Join(t.TempDir(), "list_oob.tw")
	source := `
fn main() int {
  let xs: List<int> = new List<int>(1, 2);
  println(xs[9]);
  return 0;
}
`
	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	outputName := "twice_cli_list_oob_bin"
	outputPath := filepath.Join(root, outputName)
	_ = os.Remove(outputPath)
	t.Cleanup(func() { _ = os.Remove(outputPath) })

	cmd := exec.Command("go", "run", "./cmd/twice", "-run", "-o", outputName, srcPath)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected runtime failure for out-of-bounds list access. output:\n%s", out)
	}
	output := string(out)
	if !strings.Contains(output, "Runtime error: list index out of bounds") {
		t.Fatalf("missing list runtime error output. output:\n%s", output)
	}
}

func TestCLICompileAndRunMapFeature(t *testing.T) {
	root := repoRoot(t)
	ensureToolchain(t)

	srcPath := filepath.Join(t.TempDir(), "map.tw")
	source := `
fn main() int {
  let m: Map<string,int> = new Map<string,int>(("a", 1), ("b", 2));
  m["c"] = 3;
  println(m["a"]);
  println(m["z"] ?? 0);
  println(m.length());
  println(m.has("a"));
  println(m.removeKey("b"));
  m.clear();
  println(m.length());
  return 0;
}
`
	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	outputName := "twice_cli_map_bin"
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
	if !strings.Contains(output, "\n1\n0\n3\ntrue\n2\n0\n") {
		t.Fatalf("expected map runtime output. output:\n%s", output)
	}
}

func TestCLICompileAndRunStructFeature(t *testing.T) {
	root := repoRoot(t)
	ensureToolchain(t)

	srcPath := filepath.Join(t.TempDir(), "struct.tw")
	source := `
struct Point { x: int, y?: int, z: int = 7 }

fn main() int {
  let p = new Point(x = 2, y = 3);
  println(p.x);
  p.y = 9;
  println(p.y);
  println(p.z);
  println(hasField(p, "x"));
  println(hasField(p, "missing"));
  return 0;
}
`
	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	outputName := "twice_cli_struct_bin"
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
	if !strings.Contains(output, "\n2\n9\n7\ntrue\nfalse\n") {
		t.Fatalf("expected struct runtime output. output:\n%s", output)
	}
}

func TestCLICompileAndRunPointerFeature(t *testing.T) {
	root := repoRoot(t)
	ensureToolchain(t)

	srcPath := filepath.Join(t.TempDir(), "pointer.tw")
	source := `
fn main() int {
  let x: int = 1;
  let p: *int = &x;
  *p = 7;
  println(*p);
  println(x);
  return 0;
}
`
	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	outputName := "twice_cli_ptr_bin"
	outputPath := filepath.Join(root, outputName)
	_ = os.Remove(outputPath)
	t.Cleanup(func() { _ = os.Remove(outputPath) })

	cmd := exec.Command("go", "run", "./cmd/twice", "-run", "-o", outputName, srcPath)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compile/run failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "\n7\n7\n") {
		t.Fatalf("expected pointer runtime output. output:\n%s", out)
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

func TestCLICompileAndRunGenericTypeAlias(t *testing.T) {
	root := repoRoot(t)
	ensureToolchain(t)

	srcPath := filepath.Join(t.TempDir(), "generic_alias.tw")
	source := `
type Pair<T, U> = (T, U);
fn main() int {
  let p: Pair<int, string> = (7, "seven");
  println(p.0);
  println(typeof(p));
  return 0;
}
`
	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	outputName := "twice_cli_generic_alias_bin"
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
	if !strings.Contains(output, "\n7\nPair<int,string>\n") {
		t.Fatalf("expected generic alias runtime output. output:\n%s", output)
	}
}

func TestCLICompileAndRunGenericFunctionExplicitCall(t *testing.T) {
	root := repoRoot(t)
	ensureToolchain(t)

	srcPath := filepath.Join(t.TempDir(), "generic_fn_call.tw")
	source := `
fn id<T>(x: T) T { return x; }
fn main() int {
  println(id<int>(12));
  return 0;
}
`
	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	outputName := "twice_cli_generic_fn_call_bin"
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
	if !strings.Contains(output, "\n12\n") {
		t.Fatalf("expected generic explicit call output. output:\n%s", output)
	}
}

func TestCLICompileAndRunGenericFunctionMonomorphization(t *testing.T) {
	root := repoRoot(t)
	ensureToolchain(t)

	srcPath := filepath.Join(t.TempDir(), "generic_monomorph.tw")
	source := `
fn id<T>(x: T) T { return x; }
fn main() int {
  println(id<int>(1));
  println(id<string>("x"));
  return 0;
}
`
	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	outputName := "twice_cli_generic_monomorph_bin"
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
	if !strings.Contains(output, "\n1\nx\n") {
		t.Fatalf("expected generic monomorphization output. output:\n%s", output)
	}
}

func TestCLICompileAndRunGenericFunctionNestedInference(t *testing.T) {
	root := repoRoot(t)
	ensureToolchain(t)

	srcPath := filepath.Join(t.TempDir(), "generic_nested_infer.tw")
	source := `
type Box<T> = T[2];
fn first<T>(b: Box<T>) T { return b[0]; }
fn main() int {
  let b: Box<int>;
  b[0] = 4;
  b[1] = 9;
  println(first(b));
  return 0;
}
`
	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	outputName := "twice_cli_generic_nested_infer_bin"
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
		t.Fatalf("expected nested generic inference output. output:\n%s", output)
	}
}

func TestCLICompileAndRunUnusedGenericFunctionTemplate(t *testing.T) {
	root := repoRoot(t)
	ensureToolchain(t)

	srcPath := filepath.Join(t.TempDir(), "unused_generic_template.tw")
	source := `
fn id<T>(x: T) T { return x; }
fn main() int {
  println(1);
  return 0;
}
`
	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	outputName := "twice_cli_unused_generic_template_bin"
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
	if !strings.Contains(output, "\n1\n") {
		t.Fatalf("expected runtime output with unused generic template present. output:\n%s", output)
	}
}

func TestCLICodegenErrorForGenericAliasArity(t *testing.T) {
	root := repoRoot(t)

	srcPath := filepath.Join(t.TempDir(), "bad_generic_alias_arity.tw")
	source := `
type Pair<T, U> = (T, U);
let p: Pair<int> = (1, "x");
`
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
	if !strings.Contains(output, "Codegen error: wrong number of generic type arguments for Pair: expected 2, got 1") {
		t.Fatalf("missing generic alias arity diagnostic. output:\n%s", output)
	}
}

func TestCLICodegenErrorForNonGenericAliasTypeArgs(t *testing.T) {
	root := repoRoot(t)

	srcPath := filepath.Join(t.TempDir(), "bad_non_generic_alias_type_args.tw")
	source := `
type N = int;
let x: N<string> = 1;
`
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
	if !strings.Contains(output, "Codegen error: wrong number of generic type arguments for N: expected 0, got 1") {
		t.Fatalf("missing non-generic alias type-arg arity diagnostic. output:\n%s", output)
	}
}

func TestCLICodegenErrorForUnresolvedGenericInference(t *testing.T) {
	root := repoRoot(t)

	srcPath := filepath.Join(t.TempDir(), "bad_generic_inference.tw")
	source := `
fn passthrough<T>(x: int) int { return x; }
print(passthrough(5));
`
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
	if !strings.Contains(output, "Codegen error: could not infer generic type argument for T; specify it explicitly") {
		t.Fatalf("missing unresolved generic inference diagnostic. output:\n%s", output)
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

func TestCLIRuntimeErrorForDynamicArrayIndex(t *testing.T) {
	root := repoRoot(t)
	ensureToolchain(t)

	srcPath := filepath.Join(t.TempDir(), "runtime_array_oob.tw")
	source := `
fn pick(i: int) int {
  let arr: int[3] = {1,2,3};
  return arr[i];
}
fn main() {
  println(pick(9));
  return;
}
`
	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	outputName := "twice_cli_runtime_array_oob_bin"
	outputPath := filepath.Join(root, outputName)
	_ = os.Remove(outputPath)
	t.Cleanup(func() { _ = os.Remove(outputPath) })

	cmd := exec.Command("go", "run", "./cmd/twice", "-run", "-o", outputName, srcPath)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected runtime failure for out-of-bounds array index. output:\n%s", out)
	}
	output := string(out)
	if !strings.Contains(output, "Runtime error: array index out of bounds") {
		t.Fatalf("missing runtime error prefix/message. output:\n%s", output)
	}
	if !strings.Contains(output, "--> "+srcPath+":4:13") {
		t.Fatalf("missing runtime error location with file/line/col. output:\n%s", output)
	}
	if !strings.Contains(output, "4|   return arr[i];") {
		t.Fatalf("missing runtime error source context line. output:\n%s", output)
	}
	if !strings.Contains(output, "context: arr[i]") {
		t.Fatalf("missing runtime error context snippet. output:\n%s", output)
	}
	if !strings.Contains(output, "Execution failed: exit status 1") {
		t.Fatalf("expected non-zero runtime exit propagation. output:\n%s", output)
	}
}

func TestCLIStringConcatPrintsSingleLine(t *testing.T) {
	root := repoRoot(t)
	ensureToolchain(t)

	srcPath := filepath.Join(t.TempDir(), "concat_header.tw")
	source := `
fn header(name: string) {
  println("--- " + name + " ---");
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

func TestCLIEscapedAndTemplateStrings(t *testing.T) {
	root := repoRoot(t)
	ensureToolchain(t)

	srcPath := filepath.Join(t.TempDir(), "template_escapes.tw")
	source := `
fn main() {
  let name = "Twice";
  println("line1\nline2\tend");
  println(` + "`" + `Hello ${name}\n` + "`" + `);
  return;
}
`
	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	outputName := "twice_cli_template_escapes_bin"
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
	if !strings.Contains(output, "line1\nline2\tend\n") {
		t.Fatalf("missing escaped string output. output:\n%s", output)
	}
	if !strings.Contains(output, "Hello Twice\n") {
		t.Fatalf("missing template string interpolation output. output:\n%s", output)
	}
}

func TestCLIHasFieldRuntimeFieldName(t *testing.T) {
	root := repoRoot(t)
	ensureToolchain(t)

	srcPath := filepath.Join(t.TempDir(), "hasfield_runtime.tw")
	source := `let arr = {1,2,3};
let f: string = "length";
println(hasField(arr, f));
f = "missing";
println(hasField(arr, f));
println(hasField("abc", "length"));
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
println((fn(a: int) int { return a + 1; })(2));
println(getY());
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

func TestCLIAnyTypeRuntimeFlow(t *testing.T) {
	root := repoRoot(t)
	ensureToolchain(t)

	srcPath := filepath.Join(t.TempDir(), "any_runtime.tw")
	source := `let x: any = 3;
println(typeof(x));
println(typeofValue(x));
println(x + 4);
x = "ok";
println(typeof(x));
println(typeofValue(x));
println(x + "!");
`
	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	outputName := "twice_cli_any_runtime_bin"
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
	for _, want := range []string{"\nany\n", "\nint\n", "\n7\n", "\nany\n", "\nstring\n", "\nok!\n"} {
		if !strings.Contains(output, want) {
			t.Fatalf("missing expected any-runtime output %q. full output:\n%s", want, output)
		}
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
println(s);
println(c);
println(f);
println(n);
println(typeof(c));
println(int(true));
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
	source := "const x = 8;\nprintln(x);\n"
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
	source := "let x = 1;\nx = 4;\nprintln(x);\n"
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
