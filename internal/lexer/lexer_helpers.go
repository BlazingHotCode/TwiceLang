package lexer

import "twice/internal/token"

func (l *Lexer) skipIgnored() {
	for {
		l.skipWhitespace()

		// Line comment: // ...
		if l.ch == '/' && l.peekChar() == '/' {
			l.skipLineComment()
			continue
		}

		// Block comment: /* ... */
		if l.ch == '/' && l.peekChar() == '*' {
			l.skipBlockComment()
			continue
		}

		return
	}
}

// skipWhitespace ignores spaces, tabs, newlines, carriage returns
// These have no meaning in our language except to separate tokens
func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		l.readChar()
	}
}

func (l *Lexer) skipLineComment() {
	// Skip leading "//"
	l.readChar()
	l.readChar()
	for l.ch != '\n' && l.ch != '\r' && l.ch != 0 {
		l.readChar()
	}
}

func (l *Lexer) skipBlockComment() {
	// Skip leading "/*"
	l.readChar()
	l.readChar()
	for {
		if l.ch == 0 {
			return
		}
		if l.ch == '*' && l.peekChar() == '/' {
			l.readChar()
			l.readChar()
			return
		}
		l.readChar()
	}
}

// readIdentifier reads an identifier.
// First char is guaranteed to be a letter/underscore by caller.
// Subsequent chars may include digits.
func (l *Lexer) readIdentifier() string {
	position := l.position
	for isLetter(l.ch) || isDigit(l.ch) {
		l.readChar()
	}
	return l.input[position:l.position]
}

// readNumber reads a sequence of digits
// For now, only integers. No decimals or scientific notation.
func (l *Lexer) readNumber() (token.TokenType, string) {
	position := l.position
	hasDot := false
	for isDigit(l.ch) {
		l.readChar()
	}
	if l.ch == '.' && isDigit(l.peekChar()) {
		hasDot = true
		l.readChar()
		for isDigit(l.ch) {
			l.readChar()
		}
	}
	if hasDot {
		return token.FLOAT, l.input[position:l.position]
	}
	return token.INT, l.input[position:l.position]
}

func (l *Lexer) readString() string {
	// current ch is opening quote
	l.readChar()
	position := l.position
	for l.ch != '"' && l.ch != 0 {
		l.readChar()
	}
	lit := l.input[position:l.position]
	if l.ch == '"' {
		l.readChar()
	}
	return lit
}

func (l *Lexer) readCharLiteral() string {
	// current ch is opening single quote
	l.readChar()
	if l.ch == 0 || l.ch == '\'' {
		// invalid/empty char literal, keep parser/evaluator responsible for erroring later
		lit := ""
		if l.ch == '\'' {
			l.readChar()
		}
		return lit
	}
	ch := l.ch
	l.readChar()
	if l.ch == '\'' {
		l.readChar()
	}
	return string(ch)
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
