package parser

import (
	"fmt"

	"twice/internal/ast"
	"twice/internal/lexer"
	"twice/internal/token"
)

// precedence levels (lowest to highest)
// These determine operator binding: 5 + 3 * 2 parses as 5 + (3 * 2) because * has higher precedence
const (
	_ int = iota // Start at 0, ignore this
	LOWEST
	LOGICOR     // ||
	LOGICXOR    // ^^
	LOGICAND    // &&
	BITOR       // |
	BITXOR      // ^
	BITAND      // &
	EQUALS      // ==
	LESSGREATER // > or <
	SHIFT       // << or >>
	SUM         // +
	PRODUCT     // *
	PREFIX      // -X or !X
	CALL        // myFunction(X)
	INDEX       // myArray[X]
	METHOD      // obj.method(...)
)

// precedence table maps token types to their precedence level
var precedences = map[token.TokenType]int{
	token.OR:       LOGICOR,
	token.XOR:      LOGICXOR,
	token.AND:      LOGICAND,
	token.BIT_OR:   BITOR,
	token.BIT_XOR:  BITXOR,
	token.BIT_AND:  BITAND,
	token.EQ:       EQUALS,
	token.NOT_EQ:   EQUALS,
	token.LT:       LESSGREATER,
	token.GT:       LESSGREATER,
	token.SHL:      SHIFT,
	token.SHR:      SHIFT,
	token.PLUS:     SUM,
	token.MINUS:    SUM,
	token.SLASH:    PRODUCT,
	token.ASTERISK: PRODUCT,
	token.PERCENT:  PRODUCT,
	token.LPAREN:   CALL,
	token.LBRACKET: INDEX,
	token.DOT:      METHOD,
}

type Parser struct {
	l *lexer.Lexer // The lexer feeding us tokens

	curToken  token.Token // Current token under examination
	peekToken token.Token // Next Token (for look-ahead)

	errors []string // Accumulated parse errors

	// Pratt parser tables
	prefixParseFns map[token.TokenType]prefixParseFn // Functions for tokens that start expressions
	infixParseFns  map[token.TokenType]infixParseFn  // Functions for tokens that appear in the middle
}

// prefixParseFn parses expressions that start with a specific token
// Example: -5, !true, 42, x
// The int parameter is the precedence level (for recursive calls)
type prefixParseFn func() ast.Expression

// infixParseFn parses expressions where the operator is between operands
// Example: 5 + 3, add(2, 3)
// The ast.Expression is the left side already parsed
type infixParseFn func(ast.Expression) ast.Expression

// New creates a new parser for the given lexer
func New(l *lexer.Lexer) *Parser {
	p := &Parser{
		l:      l,
		errors: []string{},
	}

	// Initialize function tables
	p.prefixParseFns = make(map[token.TokenType]prefixParseFn)
	p.infixParseFns = make(map[token.TokenType]infixParseFn)

	// Register prefix parsers (tokens that can START an expression)
	p.registerPrefix(token.IDENT, p.parseIdentifier)
	p.registerPrefix(token.INT, p.parseIntegerLiteral)
	p.registerPrefix(token.FLOAT, p.parseFloatLiteral)
	p.registerPrefix(token.STRING, p.parseStringLiteral)
	p.registerPrefix(token.CHAR, p.parseCharLiteral)
	p.registerPrefix(token.BANG, p.parsePrefixExpression)
	p.registerPrefix(token.MINUS, p.parsePrefixExpression)
	p.registerPrefix(token.TRUE, p.parseBoolean)
	p.registerPrefix(token.FALSE, p.parseBoolean)
	p.registerPrefix(token.NULL, p.parseNullLiteral)
	p.registerPrefix(token.LPAREN, p.parseGroupedExpression)
	p.registerPrefix(token.LBRACE, p.parseArrayLiteral)
	p.registerPrefix(token.IF, p.parseIfExpression)
	p.registerPrefix(token.FUNCTION, p.parseFunctionLiteral)

	// Register infix parsers (tokens that appear BETWEEN expressions)
	p.registerInfix(token.PLUS, p.parseInfixExpression)
	p.registerInfix(token.MINUS, p.parseInfixExpression)
	p.registerInfix(token.SLASH, p.parseInfixExpression)
	p.registerInfix(token.ASTERISK, p.parseInfixExpression)
	p.registerInfix(token.PERCENT, p.parseInfixExpression)
	p.registerInfix(token.EQ, p.parseInfixExpression)
	p.registerInfix(token.NOT_EQ, p.parseInfixExpression)
	p.registerInfix(token.AND, p.parseInfixExpression)
	p.registerInfix(token.OR, p.parseInfixExpression)
	p.registerInfix(token.XOR, p.parseInfixExpression)
	p.registerInfix(token.BIT_AND, p.parseInfixExpression)
	p.registerInfix(token.BIT_OR, p.parseInfixExpression)
	p.registerInfix(token.BIT_XOR, p.parseInfixExpression)
	p.registerInfix(token.LT, p.parseInfixExpression)
	p.registerInfix(token.GT, p.parseInfixExpression)
	p.registerInfix(token.SHL, p.parseInfixExpression)
	p.registerInfix(token.SHR, p.parseInfixExpression)
	p.registerInfix(token.LPAREN, p.parseCallExpression)
	p.registerInfix(token.LBRACKET, p.parseIndexExpression)
	p.registerInfix(token.DOT, p.parseMethodCallExpression)

	// Read two tokens to set curToken and peekToken
	p.nextToken()
	p.nextToken()

	return p
}

// registerPrefix adds a prefix parser for a token type
func (p *Parser) registerPrefix(tokenType token.TokenType, fn prefixParseFn) {
	p.prefixParseFns[tokenType] = fn
}

// registerInfix adds an infix parser for a token type
func (p *Parser) registerInfix(tokenType token.TokenType, fn infixParseFn) {
	p.infixParseFns[tokenType] = fn
}

// nextToken advances to the next token
func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

// Errors returns accumulated parse errors
func (p *Parser) Errors() []string {
	return p.errors
}

// peekError adds an error when we expected a different token
func (p *Parser) peekError(t token.TokenType) {
	msg := fmt.Sprintf("expected next token to be %s, got %s instead", t, p.peekToken.Type)
	p.errors = append(p.errors, msg)
}

// curTokenIs checks if current token matches
func (p *Parser) curTokenIs(t token.TokenType) bool {
	return p.curToken.Type == t
}

// peekTokenIs checks if next token matches
func (p *Parser) peekTokenIs(t token.TokenType) bool {
	return p.peekToken.Type == t
}

// expectPeek checks next token and advances if correct, else errors
// Used for mandatory syntax like "let <ident> ="
func (p *Parser) expectPeek(t token.TokenType) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	}
	p.peekError(t)
	return false
}

// peekPrecedence returns precedence of next token
func (p *Parser) peekPrecedence() int {
	if p, ok := precedences[p.peekToken.Type]; ok {
		return p
	}
	return LOWEST
}

// curPrecedence returns precedence of current token
func (p *Parser) curPrecedence() int {
	if p, ok := precedences[p.curToken.Type]; ok {
		return p
	}
	return LOWEST
}

func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{}
	program.Statements = []ast.Statement{}

	// Keep parsing statements until EOF
	for !p.curTokenIs(token.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		} else {
			p.synchronize()
		}
		p.nextToken()
	}

	return program
}

func (p *Parser) synchronize() {
	for !p.curTokenIs(token.EOF) && !p.curTokenIs(token.SEMICOLON) && !p.curTokenIs(token.RBRACE) {
		p.nextToken()
	}
}

// parseStatement dispatches to specific statement parsers based on token type
