package lexer

import "twice/internal/token"

// Lexer holds the state while tokenizing input
// It reads character by character, like a tape reader
type Lexer struct {
	input        string // The source code
	position     int    // Current position in input (points to current char)
	readPosition int    // Current reading position (after current char)
	ch           byte   // Current character under examination
}

// New creates a new Lexer for the given input
func New(input string) *Lexer {
	l := &Lexer{input: input}
	l.readChar() // Initialize with first character
	return l
}

// readChar advances to the next character
// Think of it like moving the tape forward one position
func (l *Lexer) readChar() {
	// If we've reached the end, set ch to 0 (NUL byte, signifies EOF)
	if l.readPosition >= len(l.input) {
		l.ch = 0
	} else {
		// Otherwise read the next character
		l.ch = l.input[l.readPosition]
	}
	// Move position forward
	l.position = l.readPosition
	l.readPosition += 1
}

// peekChar looks at the next character without consuming it
// Used for two-character tokens like == and !=
func (l *Lexer) peekChar() byte {
	if l.readPosition >= len(l.input) {
		return 0
	}
	return l.input[l.readPosition]
}

// NextToken returns the next token from input
// This is the heart of the lexer - it recognizes patterns
func (l *Lexer) NextToken() token.Token {
	var tok token.Token

	l.skipWhitespace() // Ignore spaces, tabs, newlines

	// Check current character and decide what token to make
	switch l.ch {
	case '=':
		// Could be = or ==
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.EQ, Literal: literal}
		} else {
			tok = newToken(token.ASSIGN, l.ch)
		}
	case '+':
		tok = newToken(token.PLUS, l.ch)
	case '-':
		tok = newToken(token.MINUS, l.ch)
	case '!':
		// Could be ! or !=
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.NOT_EQ, Literal: literal}
		} else {
			tok = newToken(token.BANG, l.ch)
		}
	case '/':
		tok = newToken(token.SLASH, l.ch)
	case '*':
		tok = newToken(token.ASTERISK, l.ch)
	case '<':
		tok = newToken(token.LT, l.ch)
	case '>':
		tok = newToken(token.GT, l.ch)
	case ';':
		tok = newToken(token.SEMICOLON, l.ch)
	case ',':
		tok = newToken(token.COMMA, l.ch)
	case '(':
		tok = newToken(token.LPAREN, l.ch)
	case ')':
		tok = newToken(token.RPAREN, l.ch)
	case '{':
		tok = newToken(token.LBRACE, l.ch)
	case '}':
		tok = newToken(token.RBRACE, l.ch)
	case 0:
		// NUL byte means end of input
		tok.Literal = ""
		tok.Type = token.EOF
	default:
		// Not a single-char token, check if letter or digit
		if isLetter(l.ch) {
			// Read the full identifier/keyword
			tok.Literal = l.readIdentifier()
			// Check if it's a keyword (let, fn, if) or user-defined (x, foo)
			tok.Type = token.LookupIdent(tok.Literal)
			return tok // Already advanced past identifier
		} else if isDigit(l.ch) {
			tok.Type = token.INT
			tok.Literal = l.readNumber()
			return tok // Already advanced past number
		} else {
			// Unknown character
			tok = newToken(token.ILLEGAL, l.ch)
		}
	}

	l.readChar() // Advance to next character for next call
	return tok
}

// skipWhitespace ignores spaces, tabs, newlines, carriage returns
// These have no meaning in our language except to separate tokens
func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		l.readChar()
	}
}

// readIdentifier reads a sequence of letters (and underscores)
// Stops when it hits a non-letter character
func (l *Lexer) readIdentifier() string {
	position := l.position
	for isLetter(l.ch) {
		l.readChar()
	}
	return l.input[position:l.position]
}

// readNumber reads a sequence of digits
// For now, only integers. No decimals or scientific notation.
func (l *Lexer) readNumber() string {
	position := l.position
	for isDigit(l.ch) {
		l.readChar()
	}
	return l.input[position:l.position]
}

// isLetter checks if ch is a letter or underscore
// We allow underscores in identifiers: foo_bar
func isLetter(ch byte) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_'
}

// isDigit checks if ch is 0-9
func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}

// newToken is a helper to create single-character tokens
func newToken(tokenType token.TokenType, ch byte) token.Token {
	return token.Token{Type: tokenType, Literal: string(ch)}
}
