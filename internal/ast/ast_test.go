package ast

import (
	"testing"
	"twice/internal/token"
)

func tok(tt token.TokenType, lit string) token.Token { return token.Token{Type: tt, Literal: lit} }

func TestProgramAndNodeStrings(t *testing.T) {
	idX := &Identifier{Token: tok(token.IDENT, "x"), Value: "x"}
	idY := &Identifier{Token: tok(token.IDENT, "y"), Value: "y"}
	int1 := &IntegerLiteral{Token: tok(token.INT, "1"), Value: 1}
	flt := &FloatLiteral{Token: tok(token.FLOAT, "3.14"), Value: 3.14}
	str := &StringLiteral{Token: tok(token.STRING, "hello"), Value: "hello"}
	chr := &CharLiteral{Token: tok(token.CHAR, "a"), Value: 'a'}
	null := &NullLiteral{Token: tok(token.NULL, "null")}
	arr := &ArrayLiteral{Token: tok(token.LBRACE, "{"), Elements: []Expression{int1, idX}}
	tuple := &TupleLiteral{Token: tok(token.LPAREN, "("), Elements: []Expression{idX, str}}
	pref := &PrefixExpression{Token: tok(token.BANG, "!"), Operator: "!", Right: idX}
	infx := &InfixExpression{Token: tok(token.PLUS, "+"), Left: idX, Operator: "+", Right: int1}
	idx := &IndexExpression{Token: tok(token.LBRACKET, "["), Left: arr, Index: int1}

	blk := &BlockStatement{Token: tok(token.LBRACE, "{"), Statements: []Statement{
		&ExpressionStatement{Token: tok(token.IDENT, "x"), Expression: infx},
	}}
	ifx := &IfExpression{Token: tok(token.IF, "if"), Condition: idX, Consequence: blk, Alternative: blk}
	fnParam := &FunctionParameter{Name: idX, TypeName: "int", DefaultValue: int1}
	fnLit := &FunctionLiteral{Token: tok(token.FUNCTION, "fn"), Name: &Identifier{Token: tok(token.IDENT, "add"), Value: "add"}, Parameters: []*FunctionParameter{fnParam}, ReturnType: "int", Body: blk}
	fnStmt := &FunctionStatement{Token: tok(token.FUNCTION, "fn"), Name: fnLit.Name, Function: fnLit}
	call := &CallExpression{Token: tok(token.LPAREN, "("), Function: fnLit.Name, Arguments: []Expression{int1, idY}}
	newExpr := &NewExpression{Token: tok(token.NEW, "new"), TypeName: "List<int>", Arguments: []Expression{int1}}
	meth := &MethodCallExpression{Token: tok(token.DOT, "."), Object: idX, Method: &Identifier{Token: tok(token.IDENT, "length"), Value: "length"}, Arguments: []Expression{}, NullSafe: true}
	macc := &MemberAccessExpression{Token: tok(token.DOT, "."), Object: idX, Property: idY}
	tacc := &TupleAccessExpression{Token: tok(token.DOT, "."), Left: tuple, Index: 1}
	narg := &NamedArgument{Token: tok(token.IDENT, "a"), Name: "a", Value: int1}
	nsafe := &NullSafeAccessExpression{Token: tok(token.QDOT, "?."), Object: idX, Property: idY}

	stmts := []Statement{
		&LetStatement{Token: tok(token.LET, "let"), Name: idX, TypeName: "int", Value: int1},
		&ConstStatement{Token: tok(token.CONST, "const"), Name: idY, TypeName: "float", Value: flt},
		&TypeDeclStatement{Token: tok(token.IDENT, "type"), Name: &Identifier{Token: tok(token.IDENT, "N"), Value: "N"}, TypeName: "int||string"},
		&AssignStatement{Token: tok(token.IDENT, "x"), Name: idX, Value: infx},
		&IndexAssignStatement{Token: tok(token.ASSIGN, "="), Left: idx, Value: int1},
		&ReturnStatement{Token: tok(token.RETURN, "return"), ReturnValue: idX},
		&ExpressionStatement{Token: tok(token.IDENT, "x"), Expression: ifx},
		&WhileStatement{Token: tok(token.WHILE, "while"), Condition: idX, Body: blk},
		&LoopStatement{Token: tok(token.LOOP, "loop"), Body: blk},
		&ForStatement{Token: tok(token.FOR, "for"), Init: &LetStatement{Token: tok(token.LET, "let"), Name: idX, Value: int1}, Condition: idX, Periodic: &AssignStatement{Token: tok(token.IDENT, "x"), Name: idX, Value: infx}, Body: blk},
		&BreakStatement{Token: tok(token.BREAK, "break")},
		&ContinueStatement{Token: tok(token.CONTINUE, "continue")},
		fnStmt,
		blk,
	}
	prog := &Program{Statements: stmts}

	if prog.TokenLiteral() == "" || prog.String() == "" {
		t.Fatalf("program stringify/token literal empty")
	}

	exprs := []Expression{idX, int1, flt, str, chr, null, arr, tuple, &Boolean{Token: tok(token.TRUE, "true"), Value: true}, pref, infx, idx, ifx, fnLit, call, newExpr, meth, macc, tacc, narg, nsafe}
	for _, e := range exprs {
		if e.TokenLiteral() == "" {
			t.Fatalf("empty token literal for %T", e)
		}
		_ = e.String()
	}
	for _, s := range stmts {
		if s.TokenLiteral() == "" {
			t.Fatalf("empty token literal for %T", s)
		}
		_ = s.String()
	}
}

func TestEmptyProgramAndNilBranches(t *testing.T) {
	if got := (&Program{}).TokenLiteral(); got != "" {
		t.Fatalf("empty program token literal=%q", got)
	}
	if got := (&ExpressionStatement{}).String(); got != "" {
		t.Fatalf("nil expression stmt string=%q", got)
	}
	if got := (&FunctionStatement{}).String(); got != "" {
		t.Fatalf("nil function stmt string=%q", got)
	}
	_ = (&ReturnStatement{Token: tok(token.RETURN, "return")}).String()
	_ = (&LetStatement{Token: tok(token.LET, "let"), Name: &Identifier{Token: tok(token.IDENT, "a"), Value: "a"}}).String()
	_ = (&ConstStatement{Token: tok(token.CONST, "const"), Name: &Identifier{Token: tok(token.IDENT, "a"), Value: "a"}}).String()
	_ = (&TypeDeclStatement{Token: tok(token.IDENT, "type")}).String()
	_ = (&MethodCallExpression{Token: tok(token.DOT, ".")}).String()
	_ = (&MemberAccessExpression{Token: tok(token.DOT, ".")}).String()
	_ = (&NullSafeAccessExpression{Token: tok(token.QDOT, "?.")}).String()
}
