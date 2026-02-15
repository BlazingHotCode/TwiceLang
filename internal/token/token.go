package token

// TokenType is a string alias for token types
// Using string makes debugging easier (we can print "PLUS" instead of a number)
type TokenType string

// Token struct holds the type and literal value
// For example: Token{Type: INT, Literal: "5"} or Token{Type: PLUS, Literal: "+"}
type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Column  int
}

// Token constants - these are the vocabulary of our language
const (
	// Special
	ILLEGAL TokenType = "ILLEGAL" // Unknown/invalid character
	EOF     TokenType = "EOF"     // End of file, tells parser we're done

	// Identifiers and literals
	IDENT    TokenType = "IDENT"    // Variable names: x, y, foo
	INT      TokenType = "INT"      // Integers: 1, 42, 999
	FLOAT    TokenType = "FLOAT"    // Floating-point: 3.14
	STRING   TokenType = "STRING"   // Strings: "hello"
	TEMPLATE TokenType = "TEMPLATE" // Template strings: `hello ${x}`
	CHAR     TokenType = "CHAR"     // Char literals: 'a'

	// Operators
	ASSIGN   TokenType = "="
	PLUS     TokenType = "+"
	MINUS    TokenType = "-"
	PLUSPLUS TokenType = "++"
	MINUSMIN TokenType = "--"
	PLUS_EQ  TokenType = "+="
	MINUS_EQ TokenType = "-="
	BANG     TokenType = "!"
	ASTERISK TokenType = "*"
	MUL_EQ   TokenType = "*="
	SLASH    TokenType = "/"
	DIV_EQ   TokenType = "/="
	PERCENT  TokenType = "%"
	MOD_EQ   TokenType = "%="
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
	DOT       TokenType = "."
	SEMICOLON TokenType = ";"
	COLON     TokenType = ":"
	LPAREN    TokenType = "("
	RPAREN    TokenType = ")"
	LBRACE    TokenType = "{"
	RBRACE    TokenType = "}"
	LBRACKET  TokenType = "["
	RBRACKET  TokenType = "]"
	QDOT      TokenType = "?."
	QCOALESCE TokenType = "??"
	QUESTION  TokenType = "?"

	// Keywords
	FUNCTION TokenType = "FUNCTION"
	LET      TokenType = "LET"
	CONST    TokenType = "CONST"
	TRUE     TokenType = "TRUE"
	FALSE    TokenType = "FALSE"
	IF       TokenType = "IF"
	ELIF     TokenType = "ELIF"
	ELSE     TokenType = "ELSE"
	FOR      TokenType = "FOR"
	WHILE    TokenType = "WHILE"
	LOOP     TokenType = "LOOP"
	BREAK    TokenType = "BREAK"
	CONTINUE TokenType = "CONTINUE"
	RETURN   TokenType = "RETURN"
	NULL     TokenType = "NULL"
	NEW      TokenType = "NEW"
	STRUCT   TokenType = "STRUCT"
	IMPORT   TokenType = "IMPORT"
)

// keywords maps string identifiers to their token type
// This lets us distinguish between "let" (keyword) and "x" (identifier)
var keywords = map[string]TokenType{
	"fn":       FUNCTION,
	"let":      LET,
	"const":    CONST,
	"true":     TRUE,
	"false":    FALSE,
	"if":       IF,
	"elif":     ELIF,
	"else":     ELSE,
	"for":      FOR,
	"while":    WHILE,
	"loop":     LOOP,
	"break":    BREAK,
	"continue": CONTINUE,
	"return":   RETURN,
	"null":     NULL,
	"new":      NEW,
	"struct":   STRUCT,
	"import":   IMPORT,
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
