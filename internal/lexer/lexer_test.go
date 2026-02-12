package lexer

import (
	"testing"

	"twice/internal/token"
)

func TestNextToken(t *testing.T) {
	input := `
let five = 5;
const c = 1;
let ten = 10;
if (five < ten) { return true; } else { return false; }
5 == 5;
5 != 10;
true && false;
true || false;
true ^^ false;
5 & 3;
5 | 2;
5 ^ 1;
5 << 1;
5 >> 1;
`

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.LET, "let"},
		{token.IDENT, "five"},
		{token.ASSIGN, "="},
		{token.INT, "5"},
		{token.SEMICOLON, ";"},
		{token.CONST, "const"},
		{token.IDENT, "c"},
		{token.ASSIGN, "="},
		{token.INT, "1"},
		{token.SEMICOLON, ";"},
		{token.LET, "let"},
		{token.IDENT, "ten"},
		{token.ASSIGN, "="},
		{token.INT, "10"},
		{token.SEMICOLON, ";"},
		{token.IF, "if"},
		{token.LPAREN, "("},
		{token.IDENT, "five"},
		{token.LT, "<"},
		{token.IDENT, "ten"},
		{token.RPAREN, ")"},
		{token.LBRACE, "{"},
		{token.RETURN, "return"},
		{token.TRUE, "true"},
		{token.SEMICOLON, ";"},
		{token.RBRACE, "}"},
		{token.ELSE, "else"},
		{token.LBRACE, "{"},
		{token.RETURN, "return"},
		{token.FALSE, "false"},
		{token.SEMICOLON, ";"},
		{token.RBRACE, "}"},
		{token.INT, "5"},
		{token.EQ, "=="},
		{token.INT, "5"},
		{token.SEMICOLON, ";"},
		{token.INT, "5"},
		{token.NOT_EQ, "!="},
		{token.INT, "10"},
		{token.SEMICOLON, ";"},
		{token.TRUE, "true"},
		{token.AND, "&&"},
		{token.FALSE, "false"},
		{token.SEMICOLON, ";"},
		{token.TRUE, "true"},
		{token.OR, "||"},
		{token.FALSE, "false"},
		{token.SEMICOLON, ";"},
		{token.TRUE, "true"},
		{token.XOR, "^^"},
		{token.FALSE, "false"},
		{token.SEMICOLON, ";"},
		{token.INT, "5"},
		{token.BIT_AND, "&"},
		{token.INT, "3"},
		{token.SEMICOLON, ";"},
		{token.INT, "5"},
		{token.BIT_OR, "|"},
		{token.INT, "2"},
		{token.SEMICOLON, ";"},
		{token.INT, "5"},
		{token.BIT_XOR, "^"},
		{token.INT, "1"},
		{token.SEMICOLON, ";"},
		{token.INT, "5"},
		{token.SHL, "<<"},
		{token.INT, "1"},
		{token.SEMICOLON, ";"},
		{token.INT, "5"},
		{token.SHR, ">>"},
		{token.INT, "1"},
		{token.SEMICOLON, ";"},
		{token.EOF, ""},
	}

	l := New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - token type wrong. expected=%q, got=%q", i, tt.expectedType, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q", i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestCommentsAreSkipped(t *testing.T) {
	input := `
// top-level comment
let x = 1; // end-line comment
/* multi-line
   block comment */
x = x + 2; /* inline block comment */ print(x);
`

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.LET, "let"},
		{token.IDENT, "x"},
		{token.ASSIGN, "="},
		{token.INT, "1"},
		{token.SEMICOLON, ";"},
		{token.IDENT, "x"},
		{token.ASSIGN, "="},
		{token.IDENT, "x"},
		{token.PLUS, "+"},
		{token.INT, "2"},
		{token.SEMICOLON, ";"},
		{token.IDENT, "print"},
		{token.LPAREN, "("},
		{token.IDENT, "x"},
		{token.RPAREN, ")"},
		{token.SEMICOLON, ";"},
		{token.EOF, ""},
	}

	l := New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - token type wrong. expected=%q, got=%q", i, tt.expectedType, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q", i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestAdditionalLiteralTokens(t *testing.T) {
	input := `let s: string = "hi"; let c = 'a'; let f = 3.14; let n = null;`
	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.LET, "let"},
		{token.IDENT, "s"},
		{token.COLON, ":"},
		{token.IDENT, "string"},
		{token.ASSIGN, "="},
		{token.STRING, "hi"},
		{token.SEMICOLON, ";"},
		{token.LET, "let"},
		{token.IDENT, "c"},
		{token.ASSIGN, "="},
		{token.CHAR, "a"},
		{token.SEMICOLON, ";"},
		{token.LET, "let"},
		{token.IDENT, "f"},
		{token.ASSIGN, "="},
		{token.FLOAT, "3.14"},
		{token.SEMICOLON, ";"},
		{token.LET, "let"},
		{token.IDENT, "n"},
		{token.ASSIGN, "="},
		{token.NULL, "null"},
		{token.SEMICOLON, ";"},
		{token.EOF, ""},
	}
	l := New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.expectedType || tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - got=(%q,%q) want=(%q,%q)", i, tok.Type, tok.Literal, tt.expectedType, tt.expectedLiteral)
		}
	}
}

func TestIdentifierWithDigits(t *testing.T) {
	input := `let n1 = 1; let value2x = n1 + 2;`
	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.LET, "let"},
		{token.IDENT, "n1"},
		{token.ASSIGN, "="},
		{token.INT, "1"},
		{token.SEMICOLON, ";"},
		{token.LET, "let"},
		{token.IDENT, "value2x"},
		{token.ASSIGN, "="},
		{token.IDENT, "n1"},
		{token.PLUS, "+"},
		{token.INT, "2"},
		{token.SEMICOLON, ";"},
		{token.EOF, ""},
	}

	l := New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.expectedType || tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - got=(%q,%q) want=(%q,%q)", i, tok.Type, tok.Literal, tt.expectedType, tt.expectedLiteral)
		}
	}
}

func TestModuloAndCompoundOperatorTokens(t *testing.T) {
	input := `x %= 2; y = 5 % 3;`
	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.IDENT, "x"},
		{token.MOD_EQ, "%="},
		{token.INT, "2"},
		{token.SEMICOLON, ";"},
		{token.IDENT, "y"},
		{token.ASSIGN, "="},
		{token.INT, "5"},
		{token.PERCENT, "%"},
		{token.INT, "3"},
		{token.SEMICOLON, ";"},
		{token.EOF, ""},
	}

	l := New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.expectedType || tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - got=(%q,%q) want=(%q,%q)", i, tok.Type, tok.Literal, tt.expectedType, tt.expectedLiteral)
		}
	}
}
