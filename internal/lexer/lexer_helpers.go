package lexer

import (
	"strconv"
	"strings"

	"twice/internal/token"
)

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
	var out strings.Builder
	for l.ch != '"' && l.ch != 0 {
		if l.ch == '\\' {
			r, ok := l.readEscapedRune()
			if ok {
				out.WriteRune(r)
				continue
			}
		}
		out.WriteByte(l.ch)
		l.readChar()
	}
	lit := out.String()
	if l.ch == '"' {
		l.readChar()
	}
	return lit
}

func (l *Lexer) readTemplateString() string {
	// current ch is opening backtick
	l.readChar()
	var out strings.Builder
	for l.ch != '`' && l.ch != 0 {
		if l.ch == '\\' {
			r, ok := l.readEscapedRune()
			if ok {
				out.WriteRune(r)
				continue
			}
		}
		out.WriteByte(l.ch)
		l.readChar()
	}
	lit := out.String()
	if l.ch == '`' {
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
	ch := rune(l.ch)
	if l.ch == '\\' {
		if r, ok := l.readEscapedRune(); ok {
			ch = r
		}
	} else {
		l.readChar()
	}
	if l.ch == '\'' {
		l.readChar()
	}
	return string(ch)
}

func (l *Lexer) readEscapedRune() (rune, bool) {
	// current ch must be '\'
	l.readChar()
	if l.ch == 0 {
		return 0, false
	}
	switch l.ch {
	case 'n':
		l.readChar()
		return '\n', true
	case 't':
		l.readChar()
		return '\t', true
	case 'r':
		l.readChar()
		return '\r', true
	case 'b':
		l.readChar()
		return '\b', true
	case 'f':
		l.readChar()
		return '\f', true
	case 'v':
		l.readChar()
		return '\v', true
	case 'a':
		l.readChar()
		return '\a', true
	case '\\':
		l.readChar()
		return '\\', true
	case '"':
		l.readChar()
		return '"', true
	case '\'':
		l.readChar()
		return '\'', true
	case '`':
		l.readChar()
		return '`', true
	case '$':
		l.readChar()
		return '$', true
	case '0':
		l.readChar()
		return '\000', true
	case 'x':
		return l.readHexEscape(2)
	case 'u':
		return l.readHexEscape(4)
	default:
		r := rune(l.ch)
		l.readChar()
		return r, true
	}
}

func (l *Lexer) readHexEscape(n int) (rune, bool) {
	var b strings.Builder
	for i := 0; i < n; i++ {
		l.readChar()
		if !isHexDigit(l.ch) {
			return 0, false
		}
		b.WriteByte(l.ch)
	}
	v, err := strconv.ParseInt(b.String(), 16, 32)
	if err != nil {
		return 0, false
	}
	l.readChar()
	return rune(v), true
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

func isHexDigit(ch byte) bool {
	return ('0' <= ch && ch <= '9') || ('a' <= ch && ch <= 'f') || ('A' <= ch && ch <= 'F')
}

// newToken is a helper to create single-character tokens
func newToken(tokenType token.TokenType, ch byte) token.Token {
	return token.Token{Type: tokenType, Literal: string(ch)}
}
