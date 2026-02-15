package token

import "testing"

func TestLookupIdent(t *testing.T) {
	tests := map[string]TokenType{
		"fn": FUNCTION,
		"let": LET,
		"const": CONST,
		"true": TRUE,
		"false": FALSE,
		"if": IF,
		"elif": ELIF,
		"else": ELSE,
		"for": FOR,
		"while": WHILE,
		"loop": LOOP,
		"break": BREAK,
		"continue": CONTINUE,
		"return": RETURN,
		"null": NULL,
		"x": IDENT,
	}

	for in, want := range tests {
		if got := LookupIdent(in); got != want {
			t.Fatalf("LookupIdent(%q)=%q want=%q", in, got, want)
		}
	}
}
