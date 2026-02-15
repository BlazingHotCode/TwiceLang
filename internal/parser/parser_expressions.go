package parser

import (
	"fmt"
	"strconv"
	"strings"

	"twice/internal/ast"
	"twice/internal/lexer"
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
	p.addErrorCurrent(msg, p.curToken.Literal)
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
		p.addErrorCurrent(msg, p.curToken.Literal)
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
		p.addErrorCurrent(msg, p.curToken.Literal)
		return nil
	}
	lit.Value = value
	return lit
}

func (p *Parser) parseStringLiteral() ast.Expression {
	return &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseTemplateStringLiteral() ast.Expression {
	parts, err := splitTemplateParts(p.curToken.Literal)
	if err != nil {
		p.addErrorCurrent("invalid template literal", err.Error())
		return nil
	}
	if len(parts) == 0 {
		return &ast.StringLiteral{Token: p.curToken, Value: ""}
	}

	var expr ast.Expression
	appendPart := func(part ast.Expression) {
		if expr == nil {
			expr = part
			return
		}
		expr = &ast.InfixExpression{
			Token:    token.Token{Type: token.PLUS, Literal: "+"},
			Operator: "+",
			Left:     expr,
			Right:    part,
		}
	}

	for _, part := range parts {
		if !part.isExpr {
			appendPart(&ast.StringLiteral{Token: p.curToken, Value: part.value})
			continue
		}
		interpolated, ok := parseTemplateInterpolationExpression(part.value)
		if !ok {
			p.addErrorCurrent("invalid template interpolation", part.value)
			return nil
		}
		appendPart(interpolated)
	}

	if expr == nil {
		return &ast.StringLiteral{Token: p.curToken, Value: ""}
	}
	return expr
}

type templatePart struct {
	value  string
	isExpr bool
}

func splitTemplateParts(input string) ([]templatePart, error) {
	parts := []templatePart{}
	var text strings.Builder

	for i := 0; i < len(input); i++ {
		ch := input[i]
		if ch == '$' && i+1 < len(input) && input[i+1] == '{' {
			if text.Len() > 0 {
				parts = append(parts, templatePart{value: text.String()})
				text.Reset()
			}
			i += 2
			start := i
			depth := 1
			for i < len(input) {
				switch input[i] {
				case '{':
					depth++
				case '}':
					depth--
					if depth == 0 {
						expr := strings.TrimSpace(input[start:i])
						if expr == "" {
							return nil, fmt.Errorf("empty ${} expression")
						}
						parts = append(parts, templatePart{value: expr, isExpr: true})
						goto next
					}
				}
				i++
			}
			return nil, fmt.Errorf("unterminated ${...} expression")
		}
		text.WriteByte(ch)
	next:
	}

	if text.Len() > 0 {
		parts = append(parts, templatePart{value: text.String()})
	}
	return parts, nil
}

func parseTemplateInterpolationExpression(src string) (ast.Expression, bool) {
	lp := lexer.New(src + ";")
	pp := New(lp)
	program := pp.ParseProgram()
	if len(pp.Errors()) != 0 {
		return nil, false
	}
	if program == nil || len(program.Statements) != 1 {
		return nil, false
	}
	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok || stmt.Expression == nil {
		return nil, false
	}
	return stmt.Expression, true
}

func (p *Parser) parseCharLiteral() ast.Expression {
	if len(p.curToken.Literal) != 1 {
		p.addErrorCurrent("invalid char literal", p.curToken.Literal)
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

	p.validateNullishMixing(expression)

	return expression
}

func directInfixOp(expr ast.Expression) (string, bool) {
	infix, ok := expr.(*ast.InfixExpression)
	if !ok || infix == nil {
		return "", false
	}
	return infix.Operator, true
}

func (p *Parser) validateNullishMixing(infix *ast.InfixExpression) {
	if infix == nil {
		return
	}

	leftOp, leftIsInfix := directInfixOp(infix.Left)
	rightOp, rightIsInfix := directInfixOp(infix.Right)
	leftGrouped := p.isGroupedExpr(infix.Left)
	rightGrouped := p.isGroupedExpr(infix.Right)

	switch infix.Operator {
	case "??":
		if (leftIsInfix && !leftGrouped && (leftOp == "&&" || leftOp == "||")) ||
			(rightIsInfix && !rightGrouped && (rightOp == "&&" || rightOp == "||")) {
			p.addErrorCurrent("cannot mix ?? with &&/|| without parentheses", infix.Operator)
		}
	case "&&", "||":
		if (leftIsInfix && !leftGrouped && leftOp == "??") ||
			(rightIsInfix && !rightGrouped && rightOp == "??") {
			p.addErrorCurrent("cannot mix ?? with &&/|| without parentheses", infix.Operator)
		}
	}
}

func (p *Parser) isGroupedExpr(expr ast.Expression) bool {
	if expr == nil {
		return false
	}
	_, ok := p.groupedExpr[expr]
	return ok
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
	if p.peekTokenIs(token.RPAREN) {
		p.nextToken() // )
		return &ast.TupleLiteral{
			Token:    token.Token{Type: token.LPAREN, Literal: "("},
			Elements: []ast.Expression{},
		}
	}

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
	p.groupedExpr[first] = struct{}{}
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
		} else {
			p.synchronize()
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
		p.addErrorCurrent("function parameter name must be an identifier", p.curToken.Literal)
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
	isNullSafe := p.curTokenIs(token.QDOT)
	if !isNullSafe && p.peekTokenIs(token.INT) {
		p.nextToken()
		idx, err := strconv.Atoi(p.curToken.Literal)
		if err != nil || idx < 0 {
			p.addErrorCurrent("tuple access index must be a non-negative int literal", p.curToken.Literal)
			return nil
		}
		return &ast.TupleAccessExpression{
			Token: token.Token{Type: token.DOT, Literal: "."},
			Left:  left,
			Index: idx,
		}
	}

	if !p.expectPeek(token.IDENT) {
		return nil
	}
	prop := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.peekTokenIs(token.LPAREN) {
		if isNullSafe {
			return &ast.NullSafeAccessExpression{
				Token:    token.Token{Type: token.QDOT, Literal: "?."},
				Object:   left,
				Property: prop,
			}
		}
		return &ast.MemberAccessExpression{
			Token:    token.Token{Type: token.DOT, Literal: "."},
			Object:   left,
			Property: prop,
		}
	}

	// a.b(...) or a?.b(...)
	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	exp := &ast.MethodCallExpression{
		Token:    token.Token{Type: token.DOT, Literal: "."},
		Object:   left,
		Method:   prop,
		NullSafe: isNullSafe,
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
