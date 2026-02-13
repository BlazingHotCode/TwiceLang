package parser

import (
	"testing"

	"twice/internal/ast"
	"twice/internal/lexer"
)

func TestLetRequiresSemicolon(t *testing.T) {
	p := New(lexer.New("let x = 5"))
	_ = p.ParseProgram()

	if len(p.Errors()) == 0 {
		t.Fatalf("expected parser errors, got none")
	}
}

func TestExpressionWithoutSemicolonBecomesImplicitReturn(t *testing.T) {
	p := New(lexer.New("5"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}

	if _, ok := program.Statements[0].(*ast.ReturnStatement); !ok {
		t.Fatalf("expected implicit return statement, got=%T", program.Statements[0])
	}
}

func TestExpressionWithSemicolonIsExpressionStatement(t *testing.T) {
	p := New(lexer.New("5;"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}

	if _, ok := program.Statements[0].(*ast.ExpressionStatement); !ok {
		t.Fatalf("expected expression statement, got=%T", program.Statements[0])
	}
}

func TestConstStatementParses(t *testing.T) {
	p := New(lexer.New("const x = 5;"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ConstStatement)
	if !ok {
		t.Fatalf("expected const statement, got=%T", program.Statements[0])
	}
	if stmt.Name.Value != "x" {
		t.Fatalf("wrong const name. got=%q", stmt.Name.Value)
	}
}

func TestAssignStatementParses(t *testing.T) {
	p := New(lexer.New("let x = 1; x = 2;"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 2 {
		t.Fatalf("expected 2 statements, got=%d", len(program.Statements))
	}

	if _, ok := program.Statements[1].(*ast.AssignStatement); !ok {
		t.Fatalf("expected assign statement, got=%T", program.Statements[1])
	}
}

func TestCommentsIgnoredByParser(t *testing.T) {
	input := `
let x = 1; // inline comment
/* block
comment */
x = x + 1;
`
	p := New(lexer.New(input))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 2 {
		t.Fatalf("expected 2 statements, got=%d", len(program.Statements))
	}
}

func TestTypedLetDeclarationParses(t *testing.T) {
	p := New(lexer.New("let name: string;"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("expected let statement, got=%T", program.Statements[0])
	}
	if stmt.TypeName != "string" {
		t.Fatalf("expected type annotation string, got=%q", stmt.TypeName)
	}
	if stmt.Value != nil {
		t.Fatalf("expected nil initializer for typed declaration")
	}
}

func TestTypedArrayLetDeclarationParses(t *testing.T) {
	p := New(lexer.New("let arr: int[3];"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("expected let statement, got=%T", program.Statements[0])
	}
	if stmt.TypeName != "int[3]" {
		t.Fatalf("expected type annotation int[3], got=%q", stmt.TypeName)
	}
	if stmt.Value != nil {
		t.Fatalf("expected nil initializer for typed declaration")
	}
}

func TestNestedArrayTypeParses(t *testing.T) {
	p := New(lexer.New("let grid: int[][];"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	stmt, ok := program.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("expected let statement, got=%T", program.Statements[0])
	}
	if stmt.TypeName != "int[][]" {
		t.Fatalf("expected type annotation int[][], got=%q", stmt.TypeName)
	}
}

func TestUnionTypeParses(t *testing.T) {
	p := New(lexer.New("let xs: (int || string)[];"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	stmt, ok := program.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("expected let statement, got=%T", program.Statements[0])
	}
	if stmt.TypeName != "(int||string)[]" {
		t.Fatalf("expected type annotation (int||string)[], got=%q", stmt.TypeName)
	}
}

func TestArrayLiteralParsesInLet(t *testing.T) {
	p := New(lexer.New("let arr = {1, 2, 3};"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("expected let statement, got=%T", program.Statements[0])
	}
	arr, ok := stmt.Value.(*ast.ArrayLiteral)
	if !ok {
		t.Fatalf("expected array literal initializer, got=%T", stmt.Value)
	}
	if len(arr.Elements) != 3 {
		t.Fatalf("expected 3 elements, got=%d", len(arr.Elements))
	}
}

func TestStandaloneBlockStatementParses(t *testing.T) {
	p := New(lexer.New("{ let x = 1; }"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	if _, ok := program.Statements[0].(*ast.BlockStatement); !ok {
		t.Fatalf("expected block statement, got=%T", program.Statements[0])
	}
}

func TestArrayIndexExpressionParses(t *testing.T) {
	p := New(lexer.New("arr[1];"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected expression statement, got=%T", program.Statements[0])
	}
	idx, ok := stmt.Expression.(*ast.IndexExpression)
	if !ok {
		t.Fatalf("expected index expression, got=%T", stmt.Expression)
	}
	if idx.Left.String() != "arr" || idx.Index.String() != "1" {
		t.Fatalf("unexpected index expression: %s", idx.String())
	}
}

func TestArrayIndexAssignmentParses(t *testing.T) {
	p := New(lexer.New("arr[1] = 42;"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.IndexAssignStatement)
	if !ok {
		t.Fatalf("expected index assign statement, got=%T", program.Statements[0])
	}
	if stmt.Left.String() != "arr[1]" {
		t.Fatalf("unexpected left side: %s", stmt.Left.String())
	}
	if stmt.Value.String() != "42" {
		t.Fatalf("unexpected assignment value: %s", stmt.Value.String())
	}
}

func TestArrayLengthMethodCallParses(t *testing.T) {
	p := New(lexer.New("arr.length();"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected expression statement, got=%T", program.Statements[0])
	}
	call, ok := stmt.Expression.(*ast.MethodCallExpression)
	if !ok {
		t.Fatalf("expected method call expression, got=%T", stmt.Expression)
	}
	if call.Object.String() != "arr" {
		t.Fatalf("expected method receiver arr, got=%s", call.Object.String())
	}
	if call.Method == nil || call.Method.Value != "length" {
		t.Fatalf("expected method length, got=%v", call.Method)
	}
	if len(call.Arguments) != 0 {
		t.Fatalf("expected 0 arguments, got=%d", len(call.Arguments))
	}
}

func TestAdditionalLiteralParsing(t *testing.T) {
	p := New(lexer.New(`3.14; "abc"; 'z'; null;`))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)
	if len(program.Statements) != 4 {
		t.Fatalf("expected 4 statements, got=%d", len(program.Statements))
	}
}

func TestBooleanOperatorPrecedenceParsing(t *testing.T) {
	p := New(lexer.New("true || false && true ^^ false;"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected expression statement, got=%T", program.Statements[0])
	}

	top, ok := stmt.Expression.(*ast.InfixExpression)
	if !ok || top.Operator != "||" {
		t.Fatalf("expected top-level || infix expression, got=%T (%v)", stmt.Expression, stmt.Expression)
	}
	right, ok := top.Right.(*ast.InfixExpression)
	if !ok || right.Operator != "^^" {
		t.Fatalf("expected right side to be ^^ infix expression, got=%T (%v)", top.Right, top.Right)
	}
	andNode, ok := right.Left.(*ast.InfixExpression)
	if !ok || andNode.Operator != "&&" {
		t.Fatalf("expected nested && infix expression, got=%T (%v)", right.Left, right.Left)
	}
}

func TestBitwiseOperatorPrecedenceParsing(t *testing.T) {
	p := New(lexer.New("1 | 2 & 3 ^ 4 << 1;"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected expression statement, got=%T", program.Statements[0])
	}

	// 1 | ((2 & 3) ^ (4 << 1))
	top, ok := stmt.Expression.(*ast.InfixExpression)
	if !ok || top.Operator != "|" {
		t.Fatalf("expected top-level | infix expression, got=%T (%v)", stmt.Expression, stmt.Expression)
	}
	xorNode, ok := top.Right.(*ast.InfixExpression)
	if !ok || xorNode.Operator != "^" {
		t.Fatalf("expected right side to be ^ infix expression, got=%T (%v)", top.Right, top.Right)
	}
	andNode, ok := xorNode.Left.(*ast.InfixExpression)
	if !ok || andNode.Operator != "&" {
		t.Fatalf("expected nested & infix expression, got=%T (%v)", xorNode.Left, xorNode.Left)
	}
	shiftNode, ok := xorNode.Right.(*ast.InfixExpression)
	if !ok || shiftNode.Operator != "<<" {
		t.Fatalf("expected nested << infix expression, got=%T (%v)", xorNode.Right, xorNode.Right)
	}
}

func TestNamedFunctionSyntaxParses(t *testing.T) {
	input := `fn add(a: int, b: int = 2) int { return a + b; }`
	p := New(lexer.New(input))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.FunctionStatement)
	if !ok {
		t.Fatalf("expected function statement, got=%T", program.Statements[0])
	}
	if stmt.Name.Value != "add" {
		t.Fatalf("expected function name add, got=%q", stmt.Name.Value)
	}
	if stmt.Function.ReturnType != "int" {
		t.Fatalf("expected return type int, got=%q", stmt.Function.ReturnType)
	}
	if len(stmt.Function.Parameters) != 2 {
		t.Fatalf("expected 2 parameters, got=%d", len(stmt.Function.Parameters))
	}
	if stmt.Function.Parameters[1].DefaultValue == nil {
		t.Fatalf("expected default value for second parameter")
	}
}

func TestWhileStatementParses(t *testing.T) {
	p := New(lexer.New("while (x < 3) { x = x + 1; }"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)
	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	ws, ok := program.Statements[0].(*ast.WhileStatement)
	if !ok {
		t.Fatalf("expected while statement, got=%T", program.Statements[0])
	}
	if ws.Condition == nil || ws.Body == nil {
		t.Fatalf("expected non-nil condition and body")
	}
	if len(ws.Body.Statements) != 1 {
		t.Fatalf("expected 1 while body statement, got=%d", len(ws.Body.Statements))
	}
}

func TestLoopStatementParses(t *testing.T) {
	p := New(lexer.New("loop { x = x + 1; }"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)
	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	ls, ok := program.Statements[0].(*ast.LoopStatement)
	if !ok {
		t.Fatalf("expected loop statement, got=%T", program.Statements[0])
	}
	if ls.Body == nil || len(ls.Body.Statements) != 1 {
		t.Fatalf("expected loop body with 1 statement")
	}
}

func TestForStatementParses(t *testing.T) {
	p := New(lexer.New("for (let i = 0; i < 3; i++) { x = x + i; }"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)
	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	fs, ok := program.Statements[0].(*ast.ForStatement)
	if !ok {
		t.Fatalf("expected for statement, got=%T", program.Statements[0])
	}
	if _, ok := fs.Init.(*ast.LetStatement); !ok {
		t.Fatalf("expected for init let statement, got=%T", fs.Init)
	}
	if fs.Condition == nil {
		t.Fatalf("expected non-nil for condition")
	}
	if _, ok := fs.Periodic.(*ast.AssignStatement); !ok {
		t.Fatalf("expected for periodic assign statement, got=%T", fs.Periodic)
	}
}

func TestBreakAndContinueStatementsParse(t *testing.T) {
	p := New(lexer.New("while (true) { continue; break; }"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	ws, ok := program.Statements[0].(*ast.WhileStatement)
	if !ok {
		t.Fatalf("expected while statement, got=%T", program.Statements[0])
	}
	if len(ws.Body.Statements) != 2 {
		t.Fatalf("expected 2 body statements, got=%d", len(ws.Body.Statements))
	}
	if _, ok := ws.Body.Statements[0].(*ast.ContinueStatement); !ok {
		t.Fatalf("expected continue statement, got=%T", ws.Body.Statements[0])
	}
	if _, ok := ws.Body.Statements[1].(*ast.BreakStatement); !ok {
		t.Fatalf("expected break statement, got=%T", ws.Body.Statements[1])
	}
}

func TestModuloHasProductPrecedence(t *testing.T) {
	p := New(lexer.New("5 + 6 % 4;"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected expression statement, got=%T", program.Statements[0])
	}

	top, ok := stmt.Expression.(*ast.InfixExpression)
	if !ok || top.Operator != "+" {
		t.Fatalf("expected top-level + infix expression, got=%T (%v)", stmt.Expression, stmt.Expression)
	}
	right, ok := top.Right.(*ast.InfixExpression)
	if !ok || right.Operator != "%" {
		t.Fatalf("expected right side to be %% infix expression, got=%T (%v)", top.Right, top.Right)
	}
}

func checkNoParserErrors(t *testing.T, p *Parser) {
	t.Helper()
	if len(p.Errors()) == 0 {
		return
	}
	t.Fatalf("parser errors: %v", p.Errors())
}
