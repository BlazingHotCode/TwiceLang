package parser

import (
	"fmt"
	"strconv"

	"twice/internal/ast"
	"twice/internal/token"
)

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

	first := p.parseExpression(LOWEST)
	if first == nil {
		return nil
	}
	if p.peekTokenIs(token.COMMA) {
		tuple := &ast.TupleLiteral{
			Token:    token.Token{Type: token.LPAREN, Literal: "("},
			Elements: []ast.Expression{first},
		}
		for p.peekTokenIs(token.COMMA) {
			p.nextToken() // ,
			p.nextToken() // next element
			el := p.parseExpression(LOWEST)
			if el == nil {
				return nil
			}
			tuple.Elements = append(tuple.Elements, el)
		}
		if !p.expectPeek(token.RPAREN) {
			return nil
		}
		return tuple
	}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}
	return first
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

	if p.peekTokenIs(token.IDENT) || p.peekTokenIs(token.LPAREN) || p.peekTokenIs(token.NULL) {
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
	if p.peekTokenIs(token.INT) {
		p.nextToken()
		idx, err := strconv.Atoi(p.curToken.Literal)
		if err != nil || idx < 0 {
			p.errors = append(p.errors, "tuple access index must be a non-negative int literal")
			return nil
		}
		return &ast.TupleAccessExpression{
			Token: token.Token{Type: token.DOT, Literal: "."},
			Left:  left,
			Index: idx,
		}
	}

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
// type, type[len], type[len][len], ...
// Caller must have current token on ':'.
