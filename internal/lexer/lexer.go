package lexer

import "twice/internal/token"

// Lexer holds the state while tokenizing input
// It reads character by character, like a tape reader
type Lexer struct {
	input        string // The source code
	position     int    // Current position in input (points to current char)
	readPosition int    // Current reading position (after current char)
	ch           byte   // Current character under examination
	line         int
	column       int
}

// New creates a new Lexer for the given input
func New(input string) *Lexer {
	l := &Lexer{input: input, line: 1}
	l.readChar() // Initialize with first character
	return l
}

// readChar advances to the next character
// Think of it like moving the tape forward one position
func (l *Lexer) readChar() {
	if l.ch == '\n' {
		l.line++
		l.column = 0
	}

	// If we've reached the end, set ch to 0 (NUL byte, signifies EOF)
	if l.readPosition >= len(l.input) {
		l.ch = 0
		l.position = l.readPosition
		l.readPosition += 1
		return
	} else {
		// Otherwise read the next character
		l.ch = l.input[l.readPosition]
	}
	// Move position forward
	l.position = l.readPosition
	l.readPosition += 1
	l.column++
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

	l.skipIgnored() // Ignore spaces/newlines/comments
	startLine, startCol := l.line, l.column

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
		if l.peekChar() == '+' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.PLUSPLUS, Literal: string(ch) + string(l.ch)}
		} else if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.PLUS_EQ, Literal: string(ch) + string(l.ch)}
		} else {
			tok = newToken(token.PLUS, l.ch)
		}
	case '-':
		if l.peekChar() == '-' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.MINUSMIN, Literal: string(ch) + string(l.ch)}
		} else if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.MINUS_EQ, Literal: string(ch) + string(l.ch)}
		} else {
			tok = newToken(token.MINUS, l.ch)
		}
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
	case '&':
		if l.peekChar() == '&' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.AND, Literal: literal}
		} else {
			tok = newToken(token.BIT_AND, l.ch)
		}
	case '|':
		if l.peekChar() == '|' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.OR, Literal: literal}
		} else {
			tok = newToken(token.BIT_OR, l.ch)
		}
	case '^':
		if l.peekChar() == '^' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.XOR, Literal: literal}
		} else {
			tok = newToken(token.BIT_XOR, l.ch)
		}
	case '/':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.DIV_EQ, Literal: string(ch) + string(l.ch)}
		} else {
			tok = newToken(token.SLASH, l.ch)
		}
	case '*':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.MUL_EQ, Literal: string(ch) + string(l.ch)}
		} else {
			tok = newToken(token.ASTERISK, l.ch)
		}
	case '%':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.MOD_EQ, Literal: string(ch) + string(l.ch)}
		} else {
			tok = newToken(token.PERCENT, l.ch)
		}
	case '<':
		if l.peekChar() == '<' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.SHL, Literal: literal}
		} else {
			tok = newToken(token.LT, l.ch)
		}
	case '>':
		if l.peekChar() == '>' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.SHR, Literal: literal}
		} else {
			tok = newToken(token.GT, l.ch)
		}
	case '?':
		if l.peekChar() == '.' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.QDOT, Literal: string(ch) + string(l.ch)}
		} else if l.peekChar() == '?' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.QCOALESCE, Literal: string(ch) + string(l.ch)}
		} else {
			tok = newToken(token.ILLEGAL, l.ch)
		}
	case ';':
		tok = newToken(token.SEMICOLON, l.ch)
	case ':':
		tok = newToken(token.COLON, l.ch)
	case ',':
		tok = newToken(token.COMMA, l.ch)
	case '.':
		tok = newToken(token.DOT, l.ch)
	case '(':
		tok = newToken(token.LPAREN, l.ch)
	case ')':
		tok = newToken(token.RPAREN, l.ch)
	case '{':
		tok = newToken(token.LBRACE, l.ch)
	case '}':
		tok = newToken(token.RBRACE, l.ch)
	case '[':
		tok = newToken(token.LBRACKET, l.ch)
	case ']':
		tok = newToken(token.RBRACKET, l.ch)
	case '"':
		tok.Type = token.STRING
		tok.Literal = l.readString()
		tok.Line = startLine
		tok.Column = startCol
		return tok
	case '\'':
		tok.Type = token.CHAR
		tok.Literal = l.readCharLiteral()
		tok.Line = startLine
		tok.Column = startCol
		return tok
	case 0:
		// NUL byte means end of input
		tok.Literal = ""
		tok.Type = token.EOF
		tok.Line = startLine
		tok.Column = startCol
	default:
		// Not a single-char token, check if letter or digit
		if isLetter(l.ch) {
			// Read the full identifier/keyword
			tok.Literal = l.readIdentifier()
			// Check if it's a keyword (let, fn, if) or user-defined (x, foo)
			tok.Type = token.LookupIdent(tok.Literal)
			tok.Line = startLine
			tok.Column = startCol
			return tok // Already advanced past identifier
		} else if isDigit(l.ch) {
			tok.Type, tok.Literal = l.readNumber()
			tok.Line = startLine
			tok.Column = startCol
			return tok // Already advanced past number
		} else {
			// Unknown character
			tok = newToken(token.ILLEGAL, l.ch)
		}
	}

	if tok.Line == 0 {
		tok.Line = startLine
		tok.Column = startCol
	}
	l.readChar() // Advance to next character for next call
	return tok
}

// skipIgnored skips whitespace and comments.
