package token

// TokenType is a string alias for token types
// Using string makes debugging easier (we can print "PLUS" instead of a number)
type TokenType string

// Token struct holds the type and literal value
// For example: Token{Type: INT, Literal: "5"} or Token{Type: PLUS, Literal: "+"}
type Token struct {
	Type    TokenType
	Literal string
}

// Token constants - these are the vocabulary of our language
const (
	// Special
	ILLEGAL TokenType = "ILLEGAL" // Unknown/invalid character
	EOF     TokenType = "EOF"     // End of file, tells parser we're done

	// Identifiers and literals
	IDENT  TokenType = "IDENT"  // Variable names: x, y, foo
	INT    TokenType = "INT"    // Integers: 1, 42, 999
	FLOAT  TokenType = "FLOAT"  // Floating-point: 3.14
	STRING TokenType = "STRING" // Strings: "hello"
	CHAR   TokenType = "CHAR"   // Char literals: 'a'

	// Operators
	ASSIGN   TokenType = "="
	PLUS     TokenType = "+"
	MINUS    TokenType = "-"
	BANG     TokenType = "!"
	ASTERISK TokenType = "*"
	SLASH    TokenType = "/"
	LT       TokenType = "<"
	GT       TokenType = ">"
	EQ       TokenType = "=="
	NOT_EQ   TokenType = "!="
	AND      TokenType = "&&"
	OR       TokenType = "||"
	XOR      TokenType = "^^"
	BIT_AND  TokenType = "&"
	BIT_OR   TokenType = "|"
	BIT_XOR  TokenType = "^"
	SHL      TokenType = "<<"
	SHR      TokenType = ">>"

	// Delimiters
	COMMA     TokenType = ","
	SEMICOLON TokenType = ";"
	COLON     TokenType = ":"
	LPAREN    TokenType = "("
	RPAREN    TokenType = ")"
	LBRACE    TokenType = "{"
	RBRACE    TokenType = "}"

	// Keywords
	FUNCTION TokenType = "FUNCTION"
	LET      TokenType = "LET"
	CONST    TokenType = "CONST"
	TRUE     TokenType = "TRUE"
	FALSE    TokenType = "FALSE"
	IF       TokenType = "IF"
	ELIF     TokenType = "ELIF"
	ELSE     TokenType = "ELSE"
	RETURN   TokenType = "RETURN"
	NULL     TokenType = "NULL"
)

// keywords maps string identifiers to their token type
// This lets us distinguish between "let" (keyword) and "x" (identifier)
var keywords = map[string]TokenType{
	"fn":     FUNCTION,
	"let":    LET,
	"const":  CONST,
	"true":   TRUE,
	"false":  FALSE,
	"if":     IF,
	"elif":   ELIF,
	"else":   ELSE,
	"return": RETURN,
	"null":   NULL,
}

// LookupIdent checks if an identifier is a keyword
// If "let" is in keywords map, returns LET token type
// Otherwise returns IDENT (it's a variable name)
func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}
