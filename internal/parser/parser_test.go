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

func checkNoParserErrors(t *testing.T, p *Parser) {
	t.Helper()
	if len(p.Errors()) == 0 {
		return
	}
	t.Fatalf("parser errors: %v", p.Errors())
}
