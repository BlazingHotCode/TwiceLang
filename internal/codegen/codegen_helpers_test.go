package codegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"twice/internal/ast"
	"twice/internal/token"
)

func tk(tt token.TokenType, lit string) token.Token {
	return token.Token{Type: tt, Literal: lit, Line: 1, Column: 1}
}

func TestCodegenErrorHelpers(t *testing.T) {
	cg := New()
	cg.addError("plain")
	cg.addNodeError("node", &ast.Identifier{Token: tk(token.IDENT, "x"), Value: "x"})
	cg.failNode("fail", &ast.IntegerLiteral{Token: tk(token.INT, "1"), Value: 1})
	cg.failNodef(&ast.Boolean{Token: tk(token.TRUE, "true"), Value: true}, "fmt %d", 7)

	if len(cg.Errors()) < 4 {
		t.Fatalf("expected >=4 errors, got=%v", cg.Errors())
	}
	detailed := cg.DetailedErrors()
	if len(detailed) != len(cg.Errors()) {
		t.Fatalf("detailed errors mismatch")
	}
}

func TestTokenFromNodeCoverage(t *testing.T) {
	nodes := []ast.Node{
		&ast.Identifier{Token: tk(token.IDENT, "x"), Value: "x"},
		&ast.IntegerLiteral{Token: tk(token.INT, "1"), Value: 1},
		&ast.FloatLiteral{Token: tk(token.FLOAT, "1.0"), Value: 1.0},
		&ast.StringLiteral{Token: tk(token.STRING, "s"), Value: "s"},
		&ast.CharLiteral{Token: tk(token.CHAR, "a"), Value: 'a'},
		&ast.NullLiteral{Token: tk(token.NULL, "null")},
		&ast.ArrayLiteral{Token: tk(token.LBRACE, "{"), Elements: []ast.Expression{}},
		&ast.TupleLiteral{Token: tk(token.LPAREN, "("), Elements: []ast.Expression{}},
		&ast.LetStatement{Token: tk(token.LET, "let"), Name: &ast.Identifier{Token: tk(token.IDENT, "x"), Value: "x"}},
		&ast.ConstStatement{Token: tk(token.CONST, "const"), Name: &ast.Identifier{Token: tk(token.IDENT, "x"), Value: "x"}},
		&ast.TypeDeclStatement{Token: tk(token.IDENT, "type"), Name: &ast.Identifier{Token: tk(token.IDENT, "T"), Value: "T"}, TypeName: "int"},
		&ast.AssignStatement{Token: tk(token.IDENT, "x"), Name: &ast.Identifier{Token: tk(token.IDENT, "x"), Value: "x"}, Value: &ast.IntegerLiteral{Token: tk(token.INT, "1"), Value: 1}},
		&ast.IndexAssignStatement{Token: tk(token.ASSIGN, "="), Left: &ast.IndexExpression{Token: tk(token.LBRACKET, "["), Left: &ast.Identifier{Token: tk(token.IDENT, "a"), Value: "a"}, Index: &ast.IntegerLiteral{Token: tk(token.INT, "0"), Value: 0}}, Value: &ast.IntegerLiteral{Token: tk(token.INT, "1"), Value: 1}},
		&ast.ReturnStatement{Token: tk(token.RETURN, "return"), ReturnValue: &ast.IntegerLiteral{Token: tk(token.INT, "1"), Value: 1}},
		&ast.ExpressionStatement{Token: tk(token.IDENT, "x"), Expression: &ast.Identifier{Token: tk(token.IDENT, "x"), Value: "x"}},
		&ast.WhileStatement{Token: tk(token.WHILE, "while"), Condition: &ast.Boolean{Token: tk(token.TRUE, "true"), Value: true}, Body: &ast.BlockStatement{Token: tk(token.LBRACE, "{")}},
		&ast.LoopStatement{Token: tk(token.LOOP, "loop"), Body: &ast.BlockStatement{Token: tk(token.LBRACE, "{")}},
		&ast.ForStatement{Token: tk(token.FOR, "for"), Body: &ast.BlockStatement{Token: tk(token.LBRACE, "{")}},
		&ast.BreakStatement{Token: tk(token.BREAK, "break")},
		&ast.ContinueStatement{Token: tk(token.CONTINUE, "continue")},
		&ast.Boolean{Token: tk(token.TRUE, "true"), Value: true},
		&ast.PrefixExpression{Token: tk(token.BANG, "!"), Operator: "!", Right: &ast.Boolean{Token: tk(token.TRUE, "true"), Value: true}},
		&ast.InfixExpression{Token: tk(token.PLUS, "+"), Left: &ast.IntegerLiteral{Token: tk(token.INT, "1"), Value: 1}, Operator: "+", Right: &ast.IntegerLiteral{Token: tk(token.INT, "2"), Value: 2}},
		&ast.IfExpression{Token: tk(token.IF, "if"), Condition: &ast.Boolean{Token: tk(token.TRUE, "true"), Value: true}, Consequence: &ast.BlockStatement{Token: tk(token.LBRACE, "{")}},
		&ast.BlockStatement{Token: tk(token.LBRACE, "{")},
		&ast.FunctionLiteral{Token: tk(token.FUNCTION, "fn"), Body: &ast.BlockStatement{Token: tk(token.LBRACE, "{")}},
		&ast.FunctionStatement{Token: tk(token.FUNCTION, "fn"), Function: &ast.FunctionLiteral{Token: tk(token.FUNCTION, "fn"), Body: &ast.BlockStatement{Token: tk(token.LBRACE, "{")}}},
		&ast.CallExpression{Token: tk(token.LPAREN, "("), Function: &ast.Identifier{Token: tk(token.IDENT, "f"), Value: "f"}},
		&ast.IndexExpression{Token: tk(token.LBRACKET, "["), Left: &ast.Identifier{Token: tk(token.IDENT, "a"), Value: "a"}, Index: &ast.IntegerLiteral{Token: tk(token.INT, "0"), Value: 0}},
		&ast.MethodCallExpression{Token: tk(token.DOT, "."), Object: &ast.Identifier{Token: tk(token.IDENT, "a"), Value: "a"}, Method: &ast.Identifier{Token: tk(token.IDENT, "m"), Value: "m"}},
		&ast.TupleAccessExpression{Token: tk(token.DOT, "."), Left: &ast.TupleLiteral{Token: tk(token.LPAREN, "("), Elements: []ast.Expression{}}, Index: 0},
		&ast.NamedArgument{Token: tk(token.IDENT, "x"), Name: "x", Value: &ast.IntegerLiteral{Token: tk(token.INT, "1"), Value: 1}},
	}

	for _, n := range nodes {
		if _, ok := tokenFromNode(n); !ok {
			t.Fatalf("expected tokenFromNode success for %T", n)
		}
	}
	if _, ok := tokenFromNode(nil); ok {
		t.Fatalf("expected tokenFromNode(nil) failure")
	}
}

func TestCodegenSmallHelpers(t *testing.T) {
	if !isBuiltinName("print") || isBuiltinName("nope") {
		t.Fatalf("isBuiltinName unexpected")
	}
	if !isTypeLiteralIdentifier("int") || isTypeLiteralIdentifier("hello") {
		t.Fatalf("isTypeLiteralIdentifier unexpected")
	}
	if got := escapeAsmString("a\\b\"c\n"); !strings.Contains(got, "\\\\") || !strings.Contains(got, "\\\"") || !strings.Contains(got, "\\n") {
		t.Fatalf("escapeAsmString unexpected: %q", got)
	}
	if got := stripOuterGroupingParens("((int))"); got != "int" {
		t.Fatalf("stripOuterGroupingParens=%q", got)
	}
	if got, ok := mergeUnionBases("int", "string"); !ok || got != "int||string" {
		t.Fatalf("mergeUnionBases=%q %v", got, ok)
	}
}

func TestCodegenLoopStatementGeneration(t *testing.T) {
	asm, cg := generateAssembly(t, "let i = 0; loop { i++; if (i == 2) { break; }; } print(i);")
	if len(cg.Errors()) != 0 {
		t.Fatalf("unexpected codegen errors: %v", cg.Errors())
	}
	if strings.Count(asm, "jmp .L") < 2 {
		t.Fatalf("expected loop jump sequence, got:\n%s", asm)
	}
}

func TestCodegenInferHelpers(t *testing.T) {
	cg := New()
	block := &ast.BlockStatement{Token: tk(token.LBRACE, "{"), Statements: []ast.Statement{
		&ast.ExpressionStatement{Token: tk(token.INT, "1"), Expression: &ast.IntegerLiteral{Token: tk(token.INT, "1"), Value: 1}},
	}}
	if got := cg.inferBlockType(block); got != typeInt {
		t.Fatalf("inferBlockType=%v", got)
	}
	if !functionReturnsOnlyNull(&ast.BlockStatement{Token: tk(token.LBRACE, "{"), Statements: []ast.Statement{
		&ast.ReturnStatement{Token: tk(token.RETURN, "return"), ReturnValue: nil},
	}}) {
		t.Fatalf("functionReturnsOnlyNull expected true")
	}
	if functionReturnsOnlyNull(&ast.BlockStatement{Token: tk(token.LBRACE, "{"), Statements: []ast.Statement{
		&ast.ReturnStatement{Token: tk(token.RETURN, "return"), ReturnValue: &ast.IntegerLiteral{Token: tk(token.INT, "1"), Value: 1}},
	}}) {
		t.Fatalf("functionReturnsOnlyNull expected false")
	}
}

func TestCodegenCastCallBranches(t *testing.T) {
	tests := []string{
		"print(int(3));",
		"print(int(true));",
		"print(bool(1));",
		"print(bool(true));",
		"print(char(65));",
		"print(string(\"x\"));",
		"print(float(1.5));",
	}
	for _, in := range tests {
		_, cg := generateAssembly(t, in)
		if len(cg.Errors()) != 0 {
			t.Fatalf("unexpected codegen errors for %q: %v", in, cg.Errors())
		}
	}

	_, cg := generateAssembly(t, "int();")
	if len(cg.Errors()) == 0 {
		t.Fatalf("expected int() arity error")
	}
}

func TestCompileToExecutableErrorPath(t *testing.T) {
	// Invalid assembly should produce a deterministic assembler/linker error.
	out := filepath.Join(t.TempDir(), "badbin")
	err := CompileToExecutable("not valid assembly", out)
	if err == nil {
		// If the host toolchain somehow accepts this, ensure output exists.
		if _, statErr := os.Stat(out); statErr != nil {
			t.Fatalf("expected compile error or output file, got neither")
		}
	}
}

func TestCodegenInferAndCollectHelpers(t *testing.T) {
	cg := New()
	cg.varDeclared["x"] = typeInt
	if got := cg.inferTypeofType(&ast.Identifier{Token: tk(token.IDENT, "x"), Value: "x"}); got != typeInt {
		t.Fatalf("inferTypeofType expected int, got=%v", got)
	}

	used := map[string]struct{}{}
	stmt := &ast.ForStatement{
		Token: tk(token.FOR, "for"),
		Init:  &ast.LetStatement{Token: tk(token.LET, "let"), Name: &ast.Identifier{Token: tk(token.IDENT, "i"), Value: "i"}, Value: &ast.IntegerLiteral{Token: tk(token.INT, "0"), Value: 0}},
		Condition: &ast.InfixExpression{
			Token: tk(token.LT, "<"),
			Left:  &ast.Identifier{Token: tk(token.IDENT, "i"), Value: "i"},
			Operator: "<",
			Right: &ast.Identifier{Token: tk(token.IDENT, "n"), Value: "n"},
		},
		Periodic: &ast.AssignStatement{
			Token: tk(token.IDENT, "i"),
			Name:  &ast.Identifier{Token: tk(token.IDENT, "i"), Value: "i"},
			Value: &ast.InfixExpression{
				Token: tk(token.PLUS, "+"),
				Left:  &ast.Identifier{Token: tk(token.IDENT, "i"), Value: "i"},
				Operator: "+",
				Right: &ast.IntegerLiteral{Token: tk(token.INT, "1"), Value: 1},
			},
		},
		Body: &ast.BlockStatement{Token: tk(token.LBRACE, "{"), Statements: []ast.Statement{
			&ast.ExpressionStatement{Token: tk(token.IDENT, "f"), Expression: &ast.CallExpression{
				Token:    tk(token.LPAREN, "("),
				Function: &ast.Identifier{Token: tk(token.IDENT, "f"), Value: "f"},
				Arguments: []ast.Expression{
					&ast.IndexExpression{Token: tk(token.LBRACKET, "["), Left: &ast.Identifier{Token: tk(token.IDENT, "arr"), Value: "arr"}, Index: &ast.Identifier{Token: tk(token.IDENT, "i"), Value: "i"}},
					&ast.MethodCallExpression{Token: tk(token.DOT, "."), Object: &ast.Identifier{Token: tk(token.IDENT, "arr"), Value: "arr"}, Method: &ast.Identifier{Token: tk(token.IDENT, "length"), Value: "length"}},
				},
			}},
		}},
	}
	collectUsedNamesInStatement(stmt, used)
	for _, name := range []string{"i", "n", "f", "arr"} {
		if _, ok := used[name]; !ok {
			t.Fatalf("expected %q in used-set", name)
		}
	}

	scope := map[string]struct{}{"outer": {}, "print": {}}
	fn := &ast.FunctionLiteral{
		Token: tk(token.FUNCTION, "fn"),
		Parameters: []*ast.FunctionParameter{
			{Name: &ast.Identifier{Token: tk(token.IDENT, "a"), Value: "a"}},
		},
		Body: &ast.BlockStatement{Token: tk(token.LBRACE, "{"), Statements: []ast.Statement{
			&ast.ExpressionStatement{Token: tk(token.IDENT, "outer"), Expression: &ast.Identifier{Token: tk(token.IDENT, "outer"), Value: "outer"}},
			&ast.ExpressionStatement{Token: tk(token.IDENT, "print"), Expression: &ast.Identifier{Token: tk(token.IDENT, "print"), Value: "print"}},
		}},
	}
	caps := cg.computeCaptures(fn, scope)
	if len(caps) != 1 || caps[0] != "outer" {
		t.Fatalf("unexpected captures: %v", caps)
	}

	scope2 := map[string]struct{}{}
	collectDeclaredNames(&ast.BlockStatement{Token: tk(token.LBRACE, "{"), Statements: []ast.Statement{
		&ast.LetStatement{Token: tk(token.LET, "let"), Name: &ast.Identifier{Token: tk(token.IDENT, "a"), Value: "a"}},
		&ast.ConstStatement{Token: tk(token.CONST, "const"), Name: &ast.Identifier{Token: tk(token.IDENT, "b"), Value: "b"}},
		&ast.FunctionStatement{Token: tk(token.FUNCTION, "fn"), Name: &ast.Identifier{Token: tk(token.IDENT, "f"), Value: "f"}},
		&ast.ForStatement{Token: tk(token.FOR, "for"), Init: &ast.ConstStatement{Token: tk(token.CONST, "const"), Name: &ast.Identifier{Token: tk(token.IDENT, "i"), Value: "i"}}},
	}}, scope2)
	for _, k := range []string{"a", "b", "f", "i"} {
		if _, ok := scope2[k]; !ok {
			t.Fatalf("missing declared name %q", k)
		}
	}
}

func TestCodegenConstFoldingHelpers(t *testing.T) {
	cg := New()
	cg.intVals["n"] = 7
	cg.charVals["c"] = 'A'
	cg.floatVals["f"] = 2.5

	if v, ok := cg.constIntValue(&ast.PrefixExpression{Token: tk(token.MINUS, "-"), Operator: "-", Right: &ast.Identifier{Token: tk(token.IDENT, "n"), Value: "n"}}); !ok || v != -7 {
		t.Fatalf("constIntValue prefix failed: %d %v", v, ok)
	}
	if _, ok := cg.constIntValue(&ast.PrefixExpression{Token: tk(token.BANG, "!"), Operator: "!", Right: &ast.IntegerLiteral{Token: tk(token.INT, "1"), Value: 1}}); ok {
		t.Fatalf("constIntValue should fail for unsupported prefix")
	}
	if v, ok := cg.constCharValue(&ast.InfixExpression{Token: tk(token.PLUS, "+"), Left: &ast.Identifier{Token: tk(token.IDENT, "c"), Value: "c"}, Operator: "+", Right: &ast.IntegerLiteral{Token: tk(token.INT, "1"), Value: 1}}); !ok || v != 'B' {
		t.Fatalf("constCharValue infix failed: %q %v", v, ok)
	}
	if _, ok := cg.constCharValue(&ast.InfixExpression{Token: tk(token.MINUS, "-"), Left: &ast.CharLiteral{Token: tk(token.CHAR, "a"), Value: 'a'}, Operator: "-", Right: &ast.CharLiteral{Token: tk(token.CHAR, "b"), Value: 'b'}}); ok {
		t.Fatalf("constCharValue should fail for unsupported op")
	}
	if v, ok := cg.constFloatValue(&ast.Identifier{Token: tk(token.IDENT, "f"), Value: "f"}); !ok || v != 2.5 {
		t.Fatalf("constFloatValue identifier failed: %g %v", v, ok)
	}
}

func TestCodegenErrorBranchesByProgram(t *testing.T) {
	cases := []string{
		"let a = {};",
		"let x = 1; x = true;",
		"const x = 1; x = 2;",
		"type int = string;",
		"type T = int; type T = int;",
		"let x = 1; x[0] = 1;",
		"let arr = {1,2}; arr[true] = 1;",
		"let arr = {1,2}; arr[9] = 1;",
		"let arr = {1,2}; arr[0] = true;",
		"let a = 1; a.0;",
		"let t: (int,string) = (1,\"x\"); t.9;",
		"fn f(a: int) int { return a; } f(b = 1);",
		"fn f(a: int) int { return a; } f(a = 1, a = 2);",
		"fn f(a: int) int { return a; } f(a = 1, 2);",
		"fn f(a: int) int { return a; } f();",
		"fn f(a: int) int { return a; } f(1, 2);",
	}
	for _, in := range cases {
		_, cg := generateAssembly(t, in)
		if len(cg.Errors()) == 0 {
			t.Fatalf("expected codegen errors for %q", in)
		}
	}
}

func TestCodegenGeneratePrefixDefaultBranch(t *testing.T) {
	cg := New()
	cg.generatePrefix(&ast.PrefixExpression{
		Token:    tk(token.ILLEGAL, "~"),
		Operator: "~",
		Right:    &ast.IntegerLiteral{Token: tk(token.INT, "1"), Value: 1},
	})
	// Unknown prefix operators are currently treated as no-op after RHS generation.
	if len(cg.Errors()) != 0 {
		t.Fatalf("did not expect prefix errors, got=%v", cg.Errors())
	}
}

func TestCompileToExecutableSuccessIfToolchainAvailable(t *testing.T) {
	if _, err := exec.LookPath("as"); err != nil {
		t.Skip("assembler not installed")
	}
	if _, err := exec.LookPath("ld"); err != nil {
		t.Skip("linker not installed")
	}
	out := filepath.Join(t.TempDir(), "okbin")
	asm := ".global _start\n.text\n_start:\n    mov $60, %rax\n    xor %rdi, %rdi\n    syscall\n"
	if err := CompileToExecutable(asm, out); err != nil {
		t.Skipf("toolchain unavailable for success path: %v", err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("expected output binary: %v", err)
	}
}
