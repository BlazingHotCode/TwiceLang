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

func checkNoParserErrors(t *testing.T, p *Parser) {
	t.Helper()
	if len(p.Errors()) == 0 {
		return
	}
	t.Fatalf("parser errors: %v", p.Errors())
}
