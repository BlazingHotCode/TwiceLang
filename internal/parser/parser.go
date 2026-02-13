package parser

import (
	"fmt"
	"strconv"
	"strings"

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
		}
		p.nextToken()
	}

	return program
}

// parseStatement dispatches to specific statement parsers based on token type
func (p *Parser) parseStatement() ast.Statement {
	switch p.curToken.Type {
	case token.LET:
		return p.parseLetStatement()
	case token.CONST:
		return p.parseConstStatement()
	case token.FOR:
		return p.parseForStatement()
	case token.WHILE:
		return p.parseWhileStatement()
	case token.LOOP:
		return p.parseLoopStatement()
	case token.FUNCTION:
		return p.parseFunctionStatement()
	case token.BREAK:
		return p.parseBreakStatement()
	case token.CONTINUE:
		return p.parseContinueStatement()
	case token.RETURN:
		return p.parseReturnStatement()
	case token.IDENT:
		if p.curToken.Literal == "type" && p.peekTokenIs(token.IDENT) {
			return p.parseTypeDeclStatement()
		}
		if p.peekTokenIs(token.ASSIGN) ||
			p.peekTokenIs(token.PLUS_EQ) ||
			p.peekTokenIs(token.MINUS_EQ) ||
			p.peekTokenIs(token.MUL_EQ) ||
			p.peekTokenIs(token.DIV_EQ) ||
			p.peekTokenIs(token.MOD_EQ) ||
			p.peekTokenIs(token.PLUSPLUS) ||
			p.peekTokenIs(token.MINUSMIN) {
			return p.parseAssignStatement()
		}
		return p.parseExpressionStatement()
	case token.LBRACE:
		return p.parseBlockStatement()
	default:
		return p.parseExpressionStatement()
	}
}

func (p *Parser) parseTypeDeclStatement() ast.Statement {
	stmt := &ast.TypeDeclStatement{Token: p.curToken}
	if !p.expectPeek(token.IDENT) {
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	if !p.expectPeek(token.ASSIGN) {
		return nil
	}
	p.nextToken()
	typeName, ok := p.parseTypeExpressionFromCurrent()
	if !ok {
		return nil
	}
	stmt.TypeName = typeName
	if !p.expectPeek(token.SEMICOLON) {
		return nil
	}
	return stmt
}

func (p *Parser) parseFunctionStatement() *ast.FunctionStatement {
	fnToken := p.curToken
	stmt := &ast.FunctionStatement{Token: fnToken}

	if !p.expectPeek(token.IDENT) {
		return nil
	}
	name := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	lit := &ast.FunctionLiteral{
		Token: fnToken,
		Name:  name,
	}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}
	lit.Parameters = p.parseFunctionParameters()
	if lit.Parameters == nil {
		return nil
	}

	if p.peekTokenIs(token.IDENT) || p.peekTokenIs(token.LPAREN) {
		p.nextToken()
		returnType, ok := p.parseTypeExpressionFromCurrent()
		if !ok {
			return nil
		}
		lit.ReturnType = returnType
	}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	lit.Body = p.parseBlockStatement()

	stmt.Name = name
	stmt.Function = lit
	return stmt
}

// parseLetStatement handles: let <name> = <value>;
func (p *Parser) parseLetStatement() *ast.LetStatement {
	stmt := &ast.LetStatement{Token: p.curToken}

	// Expect identifier after let
	if !p.expectPeek(token.IDENT) {
		return nil
	}

	// Create identifier node
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	// Optional type annotation: let name: type or let name: type[len]
	if p.peekTokenIs(token.COLON) {
		p.nextToken()
		typeName, ok := p.parseTypeAnnotation()
		if !ok {
			return nil
		}
		stmt.TypeName = typeName
	}

	// Optional initializer:
	// - let name: type;
	// - let name = value;
	// - let name: type = value;
	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
		if stmt.TypeName == "" {
			p.errors = append(p.errors, "let declaration without value requires explicit type annotation")
			return nil
		}
		stmt.Value = nil
		return stmt
	}

	if !p.expectPeek(token.ASSIGN) {
		return nil
	}

	// Advance past =
	p.nextToken()

	// Parse the expression (value being assigned)
	stmt.Value = p.parseExpression(LOWEST)

	// Semicolons are mandatory statement terminators.
	if !p.expectPeek(token.SEMICOLON) {
		return nil
	}

	return stmt
}

// parseConstStatement handles: const <name> = <value>;
func (p *Parser) parseConstStatement() *ast.ConstStatement {
	stmt := &ast.ConstStatement{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil
	}

	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if p.peekTokenIs(token.COLON) {
		p.nextToken()
		typeName, ok := p.parseTypeAnnotation()
		if !ok {
			return nil
		}
		stmt.TypeName = typeName
	}

	if !p.expectPeek(token.ASSIGN) {
		return nil
	}

	p.nextToken()
	stmt.Value = p.parseExpression(LOWEST)

	if !p.expectPeek(token.SEMICOLON) {
		return nil
	}

	return stmt
}

// parseAssignStatement handles: <name> = <value>;
func (p *Parser) parseAssignStatement() *ast.AssignStatement {
	return p.parseAssignStatementCore(true)
}

func (p *Parser) parseAssignStatementCore(requireSemicolon bool) *ast.AssignStatement {
	stmt := &ast.AssignStatement{
		Token: p.curToken,
		Name:  &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal},
	}

	if !p.peekTokenIs(token.ASSIGN) &&
		!p.peekTokenIs(token.PLUS_EQ) &&
		!p.peekTokenIs(token.MINUS_EQ) &&
		!p.peekTokenIs(token.MUL_EQ) &&
		!p.peekTokenIs(token.DIV_EQ) &&
		!p.peekTokenIs(token.MOD_EQ) &&
		!p.peekTokenIs(token.PLUSPLUS) &&
		!p.peekTokenIs(token.MINUSMIN) {
		return nil
	}
	p.nextToken()
	opTok := p.curToken

	if opTok.Type == token.PLUSPLUS || opTok.Type == token.MINUSMIN {
		if requireSemicolon && !p.expectPeek(token.SEMICOLON) {
			return nil
		}
		op := "+"
		if opTok.Type == token.MINUSMIN {
			op = "-"
		}
		stmt.Value = &ast.InfixExpression{
			Token:    opTok,
			Left:     &ast.Identifier{Token: stmt.Name.Token, Value: stmt.Name.Value},
			Operator: op,
			Right:    &ast.IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "1"}, Value: 1},
		}
		return stmt
	}

	if opTok.Type == token.ASSIGN {
		p.nextToken()
		stmt.Value = p.parseExpression(LOWEST)
		if requireSemicolon && !p.expectPeek(token.SEMICOLON) {
			return nil
		}
		return stmt
	}

	p.nextToken()
	rhs := p.parseExpression(LOWEST)
	if requireSemicolon && !p.expectPeek(token.SEMICOLON) {
		return nil
	}
	op := ""
	switch opTok.Type {
	case token.PLUS_EQ:
		op = "+"
	case token.MINUS_EQ:
		op = "-"
	case token.MUL_EQ:
		op = "*"
	case token.DIV_EQ:
		op = "/"
	case token.MOD_EQ:
		op = "%"
	}
	stmt.Value = &ast.InfixExpression{
		Token:    opTok,
		Left:     &ast.Identifier{Token: stmt.Name.Token, Value: stmt.Name.Value},
		Operator: op,
		Right:    rhs,
	}

	return stmt
}

// parseReturnStatement handles: return <expression>;
func (p *Parser) parseReturnStatement() *ast.ReturnStatement {
	stmt := &ast.ReturnStatement{Token: p.curToken}

	p.nextToken()

	stmt.ReturnValue = p.parseExpression(LOWEST)

	if !p.expectPeek(token.SEMICOLON) {
		return nil
	}

	return stmt
}

func (p *Parser) parseBreakStatement() ast.Statement {
	stmt := &ast.BreakStatement{Token: p.curToken}
	if !p.expectPeek(token.SEMICOLON) {
		return nil
	}
	return stmt
}

func (p *Parser) parseContinueStatement() ast.Statement {
	stmt := &ast.ContinueStatement{Token: p.curToken}
	if !p.expectPeek(token.SEMICOLON) {
		return nil
	}
	return stmt
}

func (p *Parser) parseWhileStatement() ast.Statement {
	stmt := &ast.WhileStatement{Token: p.curToken}
	if !p.expectPeek(token.LPAREN) {
		return nil
	}
	p.nextToken()
	stmt.Condition = p.parseExpression(LOWEST)
	if !p.expectPeek(token.RPAREN) {
		return nil
	}
	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	stmt.Body = p.parseBlockStatement()
	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseLoopStatement() ast.Statement {
	stmt := &ast.LoopStatement{Token: p.curToken}
	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	stmt.Body = p.parseBlockStatement()
	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseForStatement() ast.Statement {
	stmt := &ast.ForStatement{Token: p.curToken}
	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	// init clause
	p.nextToken()
	if !p.curTokenIs(token.SEMICOLON) {
		stmt.Init = p.parseForClauseStatement()
		if stmt.Init == nil {
			return nil
		}
	}
	if !p.curTokenIs(token.SEMICOLON) {
		if !p.expectPeek(token.SEMICOLON) {
			return nil
		}
	}

	// condition clause
	p.nextToken()
	if p.curTokenIs(token.SEMICOLON) {
		stmt.Condition = &ast.Boolean{Token: token.Token{Type: token.TRUE, Literal: "true"}, Value: true}
	} else {
		stmt.Condition = p.parseExpression(LOWEST)
		if stmt.Condition == nil {
			return nil
		}
		if !p.curTokenIs(token.SEMICOLON) {
			if !p.expectPeek(token.SEMICOLON) {
				return nil
			}
		}
	}

	// periodic clause
	p.nextToken()
	if !p.curTokenIs(token.RPAREN) {
		stmt.Periodic = p.parseForClauseStatement()
		if stmt.Periodic == nil {
			return nil
		}
	}
	if !p.curTokenIs(token.RPAREN) {
		if !p.expectPeek(token.RPAREN) {
			return nil
		}
	}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	stmt.Body = p.parseBlockStatement()
	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseForClauseStatement() ast.Statement {
	switch p.curToken.Type {
	case token.LET:
		return p.parseForLetStatement()
	case token.CONST:
		return p.parseForConstStatement()
	case token.IDENT:
		if p.peekTokenIs(token.ASSIGN) ||
			p.peekTokenIs(token.PLUS_EQ) ||
			p.peekTokenIs(token.MINUS_EQ) ||
			p.peekTokenIs(token.MUL_EQ) ||
			p.peekTokenIs(token.DIV_EQ) ||
			p.peekTokenIs(token.MOD_EQ) ||
			p.peekTokenIs(token.PLUSPLUS) ||
			p.peekTokenIs(token.MINUSMIN) {
			return p.parseAssignStatementCore(false)
		}
	}
	return &ast.ExpressionStatement{
		Token:      p.curToken,
		Expression: p.parseExpression(LOWEST),
	}
}

func (p *Parser) parseForLetStatement() ast.Statement {
	stmt := &ast.LetStatement{Token: p.curToken}
	if !p.expectPeek(token.IDENT) {
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	if p.peekTokenIs(token.COLON) {
		p.nextToken()
		typeName, ok := p.parseTypeAnnotation()
		if !ok {
			return nil
		}
		stmt.TypeName = typeName
	}
	if !p.expectPeek(token.ASSIGN) {
		return nil
	}
	p.nextToken()
	stmt.Value = p.parseExpression(LOWEST)
	return stmt
}

func (p *Parser) parseForConstStatement() ast.Statement {
	stmt := &ast.ConstStatement{Token: p.curToken}
	if !p.expectPeek(token.IDENT) {
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	if p.peekTokenIs(token.COLON) {
		p.nextToken()
		typeName, ok := p.parseTypeAnnotation()
		if !ok {
			return nil
		}
		stmt.TypeName = typeName
	}
	if !p.expectPeek(token.ASSIGN) {
		return nil
	}
	p.nextToken()
	stmt.Value = p.parseExpression(LOWEST)
	return stmt
}

// parseExpressionStatement handles expressions used as statements.
// If an expression is not terminated by ';', we treat it as an implicit return.
func (p *Parser) parseExpressionStatement() ast.Statement {
	stmt := &ast.ExpressionStatement{Token: p.curToken}

	stmt.Expression = p.parseExpression(LOWEST)

	// Support indexed assignment: arr[1] = value;
	if p.peekTokenIs(token.ASSIGN) {
		if left, ok := stmt.Expression.(*ast.IndexExpression); ok {
			p.nextToken() // '='
			assignTok := p.curToken
			p.nextToken() // value expression start
			value := p.parseExpression(LOWEST)
			if !p.expectPeek(token.SEMICOLON) {
				return nil
			}
			return &ast.IndexAssignStatement{
				Token: assignTok,
				Left:  left,
				Value: value,
			}
		}
	}

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
		return stmt
	}

	return &ast.ReturnStatement{
		Token:       token.Token{Type: token.RETURN, Literal: "return"},
		ReturnValue: stmt.Expression,
	}
}

// parseExpression is the heart of Pratt parsing
// It handles prefix operators first, then loops to handle infix operators
// based on precedence
func (p *Parser) parseExpression(precedence int) ast.Expression {
	// First, find a prefix parser for current token
	// This handles: literals, identifiers, prefix operators (!, -), grouped expressions
	prefix := p.prefixParseFns[p.curToken.Type]
	if prefix == nil {
		p.noPrefixParseFnError(p.curToken.Type)
		return nil
	}
	leftExp := prefix()

	// While next token is an infix operator with higher precedence than ours,
	// consume it and build the expression tree
	for !p.peekTokenIs(token.SEMICOLON) && precedence < p.peekPrecedence() {
		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil {
			return leftExp
		}

		p.nextToken()            // Advance to the operator
		leftExp = infix(leftExp) // Parse with left side already known
	}

	return leftExp
}

func (p *Parser) noPrefixParseFnError(t token.TokenType) {
	msg := fmt.Sprintf("no prefix parse function for %s found", t)
	p.errors = append(p.errors, msg)
}

// parseIdentifier parses a variable name
func (p *Parser) parseIdentifier() ast.Expression {
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

// parseIntegerLiteral parses a number
func (p *Parser) parseIntegerLiteral() ast.Expression {
	lit := &ast.IntegerLiteral{Token: p.curToken}

	// Convert string to int64
	value, err := strconv.ParseInt(p.curToken.Literal, 0, 64)
	if err != nil {
		msg := fmt.Sprintf("could not parse %q as integer", p.curToken.Literal)
		p.errors = append(p.errors, msg)
		return nil
	}

	lit.Value = value
	return lit
}

func (p *Parser) parseFloatLiteral() ast.Expression {
	lit := &ast.FloatLiteral{Token: p.curToken}
	value, err := strconv.ParseFloat(p.curToken.Literal, 64)
	if err != nil {
		msg := fmt.Sprintf("could not parse %q as float", p.curToken.Literal)
		p.errors = append(p.errors, msg)
		return nil
	}
	lit.Value = value
	return lit
}

func (p *Parser) parseStringLiteral() ast.Expression {
	return &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseCharLiteral() ast.Expression {
	if len(p.curToken.Literal) != 1 {
		p.errors = append(p.errors, "invalid char literal")
		return nil
	}
	return &ast.CharLiteral{Token: p.curToken, Value: rune(p.curToken.Literal[0])}
}

func (p *Parser) parseNullLiteral() ast.Expression {
	return &ast.NullLiteral{Token: p.curToken}
}

func (p *Parser) parseArrayLiteral() ast.Expression {
	lit := &ast.ArrayLiteral{Token: p.curToken}
	lit.Elements = []ast.Expression{}

	if p.peekTokenIs(token.RBRACE) {
		p.nextToken()
		return lit
	}

	p.nextToken()
	lit.Elements = append(lit.Elements, p.parseExpression(LOWEST))

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		lit.Elements = append(lit.Elements, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(token.RBRACE) {
		return nil
	}

	return lit
}

// parsePrefixExpression handles !<expr> and -<expr>
func (p *Parser) parsePrefixExpression() ast.Expression {
	expression := &ast.PrefixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
	}

	p.nextToken() // Advance past ! or -

	// Parse the operand with PREFIX precedence (high)
	expression.Right = p.parseExpression(PREFIX)

	return expression
}

// parseInfixExpression handles <left> <op> <right>
// Called with left side already parsed
func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	expression := &ast.InfixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
		Left:     left,
	}

	precedence := p.curPrecedence()
	p.nextToken()
	expression.Right = p.parseExpression(precedence)

	return expression
}

// parseBoolean handles true/false
func (p *Parser) parseBoolean() ast.Expression {
	return &ast.Boolean{
		Token: p.curToken,
		Value: p.curTokenIs(token.TRUE),
	}
}

// parseGroupedExpression handles ( <expr> )
// Parentheses let us override precedence: (5 + 3) * 2
func (p *Parser) parseGroupedExpression() ast.Expression {
	p.nextToken() // Advance past (

	exp := p.parseExpression(LOWEST)

	// Expect closing )
	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return exp
}

// parseIfExpression handles: if (<condition>) <consequence> else <alternative>
func (p *Parser) parseIfExpression() ast.Expression {
	expression := &ast.IfExpression{Token: p.curToken}

	// Expect (
	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	p.nextToken()
	expression.Condition = p.parseExpression(LOWEST)

	// Expect )
	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	// Expect {
	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	expression.Consequence = p.parseBlockStatement()

	// Optional else
	if p.peekTokenIs(token.ELIF) {
		p.nextToken()
		elifExpression := p.parseIfExpression()
		if elifExpression == nil {
			return nil
		}

		expression.Alternative = &ast.BlockStatement{
			Token: p.curToken,
			Statements: []ast.Statement{
				&ast.ExpressionStatement{
					Token:      p.curToken,
					Expression: elifExpression,
				},
			},
		}
	} else if p.peekTokenIs(token.ELSE) {
		p.nextToken()

		if !p.expectPeek(token.LBRACE) {
			return nil
		}

		expression.Alternative = p.parseBlockStatement()
	}

	return expression
}

// parseBlockStatement parses a sequence of statements inside { }
func (p *Parser) parseBlockStatement() *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.curToken}
	block.Statements = []ast.Statement{}

	p.nextToken()

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken()
	}

	return block
}

// parseFunctionLiteral handles: fn(<params>) { <body> }
func (p *Parser) parseFunctionLiteral() ast.Expression {
	lit := &ast.FunctionLiteral{Token: p.curToken}

	// Expect (
	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	lit.Parameters = p.parseFunctionParameters()
	if lit.Parameters == nil {
		return nil
	}

	if p.peekTokenIs(token.IDENT) || p.peekTokenIs(token.LPAREN) {
		p.nextToken()
		returnType, ok := p.parseTypeExpressionFromCurrent()
		if !ok {
			return nil
		}
		lit.ReturnType = returnType
	}

	// Expect {
	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	lit.Body = p.parseBlockStatement()

	return lit
}

// parseFunctionParameters parses the parameter list: x, y, z
func (p *Parser) parseFunctionParameters() []*ast.FunctionParameter {
	params := []*ast.FunctionParameter{}

	// Empty params: fn()
	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
		return params
	}

	p.nextToken() // Advance to first param

	param := p.parseFunctionParameter()
	if param == nil {
		return nil
	}
	params = append(params, param)

	// More params separated by commas
	for p.peekTokenIs(token.COMMA) {
		p.nextToken() // skip comma
		p.nextToken() // advance to next param
		param := p.parseFunctionParameter()
		if param == nil {
			return nil
		}
		params = append(params, param)
	}

	// Expect closing )
	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return params
}

func (p *Parser) parseFunctionParameter() *ast.FunctionParameter {
	if !p.curTokenIs(token.IDENT) {
		p.errors = append(p.errors, "function parameter name must be an identifier")
		return nil
	}
	param := &ast.FunctionParameter{
		Name: &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal},
	}

	if p.peekTokenIs(token.COLON) {
		p.nextToken()
		typeName, ok := p.parseTypeAnnotation()
		if !ok {
			return nil
		}
		param.TypeName = typeName
	}

	if p.peekTokenIs(token.ASSIGN) {
		p.nextToken()
		p.nextToken()
		param.DefaultValue = p.parseExpression(LOWEST)
	}

	return param
}

// parseCallExpression handles: <function>(<arguments>)
// Called when we see ( after parsing a function expression
func (p *Parser) parseCallExpression(function ast.Expression) ast.Expression {
	exp := &ast.CallExpression{Token: p.curToken, Function: function}
	exp.Arguments = p.parseCallArguments()
	return exp
}

func (p *Parser) parseIndexExpression(left ast.Expression) ast.Expression {
	exp := &ast.IndexExpression{Token: p.curToken, Left: left}
	p.nextToken()
	exp.Index = p.parseExpression(LOWEST)
	if !p.expectPeek(token.RBRACKET) {
		return nil
	}
	return exp
}

func (p *Parser) parseMethodCallExpression(left ast.Expression) ast.Expression {
	exp := &ast.MethodCallExpression{Token: p.curToken, Object: left}
	if !p.expectPeek(token.IDENT) {
		return nil
	}
	exp.Method = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	if !p.expectPeek(token.LPAREN) {
		return nil
	}
	exp.Arguments = p.parseCallArguments()
	return exp
}

// parseCallArguments parses argument list: 1, 2, 3
func (p *Parser) parseCallArguments() []ast.Expression {
	args := []ast.Expression{}

	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
		return args
	}

	p.nextToken()
	args = append(args, p.parseCallArgument())

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		args = append(args, p.parseCallArgument())
	}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return args
}

func (p *Parser) parseCallArgument() ast.Expression {
	if p.curTokenIs(token.IDENT) && p.peekTokenIs(token.ASSIGN) {
		nameTok := p.curToken
		name := p.curToken.Literal
		p.nextToken()
		p.nextToken()
		return &ast.NamedArgument{
			Token: nameTok,
			Name:  name,
			Value: p.parseExpression(LOWEST),
		}
	}
	return p.parseExpression(LOWEST)
}

// parseTypeAnnotation parses type and optional array suffixes:
// type, type[], type[len], type[][], type[][len], ...
// Caller must have current token on ':'.
func (p *Parser) parseTypeAnnotation() (string, bool) {
	p.nextToken()
	return p.parseTypeExpressionFromCurrent()
}

func (p *Parser) parseOptionalArrayTypeSuffix(base string) (string, bool) {
	return p.parseArrayTypeSuffixes(base)
}

func (p *Parser) parseTypeExpressionFromCurrent() (string, bool) {
	left, ok := p.parseTypeTermFromCurrent()
	if !ok {
		return "", false
	}
	parts := []string{left}
	for p.peekTokenIs(token.OR) {
		p.nextToken() // ||
		p.nextToken() // start of next term
		right, ok := p.parseTypeTermFromCurrent()
		if !ok {
			return "", false
		}
		parts = append(parts, right)
	}
	if len(parts) == 1 {
		return parts[0], true
	}
	return strings.Join(parts, "||"), true
}

func (p *Parser) parseTypeTermFromCurrent() (string, bool) {
	base := ""
	switch p.curToken.Type {
	case token.IDENT:
		base = p.curToken.Literal
	case token.LPAREN:
		p.nextToken()
		inner, ok := p.parseTypeExpressionFromCurrent()
		if !ok {
			return "", false
		}
		if !p.expectPeek(token.RPAREN) {
			return "", false
		}
		base = "(" + inner + ")"
	default:
		p.errors = append(p.errors, fmt.Sprintf("expected type, got %s", p.curToken.Type))
		return "", false
	}
	return p.parseArrayTypeSuffixes(base)
}

func (p *Parser) parseArrayTypeSuffixes(base string) (string, bool) {
	out := base
	for p.peekTokenIs(token.LBRACKET) {
		p.nextToken() // '['
		if p.peekTokenIs(token.RBRACKET) {
			p.nextToken() // ']'
			out += "[]"
			continue
		}
		if !p.expectPeek(token.INT) {
			return "", false
		}
		size := p.curToken.Literal
		if !p.expectPeek(token.RBRACKET) {
			return "", false
		}
		out += fmt.Sprintf("[%s]", size)
	}
	return out, true
}
