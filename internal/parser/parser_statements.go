package parser

import (
	"twice/internal/ast"
	"twice/internal/token"
)

type parsedVarDecl struct {
	name     *ast.Identifier
	typeName string
	value    ast.Expression
}

func isAssignLikeToken(t token.TokenType) bool {
	switch t {
	case token.ASSIGN,
		token.PLUS_EQ,
		token.MINUS_EQ,
		token.MUL_EQ,
		token.DIV_EQ,
		token.MOD_EQ,
		token.PLUSPLUS,
		token.MINUSMIN:
		return true
	default:
		return false
	}
}

func compoundAssignOperator(t token.TokenType) string {
	switch t {
	case token.PLUS_EQ:
		return "+"
	case token.MINUS_EQ:
		return "-"
	case token.MUL_EQ:
		return "*"
	case token.DIV_EQ:
		return "/"
	case token.MOD_EQ:
		return "%"
	default:
		return ""
	}
}

func (p *Parser) parseVariableDeclCore(requireSemicolon bool, allowEmptyInit bool, emptyInitErr string) (*parsedVarDecl, bool) {
	if !p.expectPeek(token.IDENT) {
		return nil, false
	}

	decl := &parsedVarDecl{
		name: &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal},
	}

	if p.peekTokenIs(token.COLON) {
		p.nextToken()
		typeName, ok := p.parseTypeAnnotation()
		if !ok {
			return nil, false
		}
		decl.typeName = typeName
	}

	if p.peekTokenIs(token.SEMICOLON) {
		if !allowEmptyInit {
			p.addErrorPeek(emptyInitErr, p.peekToken.Literal)
			return nil, false
		}
		p.nextToken()
		if decl.typeName == "" {
			p.addErrorCurrent("let declaration without value requires explicit type annotation", p.curToken.Literal)
			return nil, false
		}
		return decl, true
	}

	if !p.expectPeek(token.ASSIGN) {
		return nil, false
	}
	p.nextToken()
	decl.value = p.parseExpression(LOWEST)
	if requireSemicolon && !p.expectPeek(token.SEMICOLON) {
		return nil, false
	}
	return decl, true
}

func (p *Parser) parseStatement() ast.Statement {
	switch p.curToken.Type {
	case token.LET:
		stmt := p.parseLetStatement()
		if stmt == nil {
			return nil
		}
		return stmt
	case token.CONST:
		stmt := p.parseConstStatement()
		if stmt == nil {
			return nil
		}
		return stmt
	case token.IMPORT:
		stmt := p.parseImportStatement()
		if stmt == nil {
			return nil
		}
		return stmt
	case token.FOR:
		return p.parseForStatement()
	case token.WHILE:
		return p.parseWhileStatement()
	case token.LOOP:
		return p.parseLoopStatement()
	case token.FUNCTION:
		stmt := p.parseFunctionStatement()
		if stmt == nil {
			return nil
		}
		return stmt
	case token.STRUCT:
		stmt := p.parseStructStatement()
		if stmt == nil {
			return nil
		}
		return stmt
	case token.BREAK:
		stmt := p.parseBreakStatement()
		if stmt == nil {
			return nil
		}
		return stmt
	case token.CONTINUE:
		stmt := p.parseContinueStatement()
		if stmt == nil {
			return nil
		}
		return stmt
	case token.RETURN:
		stmt := p.parseReturnStatement()
		if stmt == nil {
			return nil
		}
		return stmt
	case token.IDENT:
		if p.curToken.Literal == "type" && p.peekTokenIs(token.IDENT) {
			return p.parseTypeDeclStatement()
		}
		if isAssignLikeToken(p.peekToken.Type) {
			stmt := p.parseAssignStatement()
			if stmt == nil {
				return nil
			}
			return stmt
		}
		return p.parseExpressionStatement()
	case token.LBRACE:
		return p.parseBlockStatement()
	default:
		return p.parseExpressionStatement()
	}
}

func (p *Parser) parseImportStatement() *ast.ImportStatement {
	stmt := &ast.ImportStatement{Token: p.curToken}
	if !p.expectPeek(token.IDENT) {
		return nil
	}
	stmt.Path = append(stmt.Path, p.curToken.Literal)
	for p.peekTokenIs(token.DOT) {
		p.nextToken()
		if !p.expectPeek(token.IDENT) {
			return nil
		}
		stmt.Path = append(stmt.Path, p.curToken.Literal)
	}
	if p.peekTokenIs(token.IDENT) && p.peekToken.Literal == "as" {
		p.nextToken()
		if !p.expectPeek(token.IDENT) {
			return nil
		}
		stmt.Alias = p.curToken.Literal
	}
	if !p.expectPeek(token.SEMICOLON) {
		return nil
	}
	if stmt.Alias == "" && len(stmt.Path) > 0 {
		stmt.Alias = stmt.Path[len(stmt.Path)-1]
	}
	return stmt
}

func (p *Parser) parseTypeDeclStatement() ast.Statement {
	stmt := &ast.TypeDeclStatement{Token: p.curToken}
	if !p.expectPeek(token.IDENT) {
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	typeParams, ok := p.parseTypeParameterList()
	if !ok {
		return nil
	}
	stmt.TypeParams = typeParams
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

	if p.peekTokenIs(token.LPAREN) {
		if !p.expectPeek(token.LPAREN) {
			return nil
		}
		if p.peekTokenIs(token.RPAREN) {
			p.addErrorPeek("method receiver cannot be empty", p.peekToken.Literal)
			return nil
		}
		p.nextToken()
		recv := p.parseFunctionParameter()
		if recv == nil {
			return nil
		}
		if recv.TypeName == "" {
			p.addErrorCurrent("method receiver must include a type annotation", p.curToken.Literal)
			return nil
		}
		if recv.DefaultValue != nil {
			p.addErrorCurrent("method receiver cannot have a default value", p.curToken.Literal)
			return nil
		}
		if !p.expectPeek(token.RPAREN) {
			return nil
		}
		stmt.Receiver = recv
	}

	if !p.expectPeek(token.IDENT) {
		return nil
	}
	name := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	lit := &ast.FunctionLiteral{
		Token: fnToken,
		Name:  name,
	}
	typeParams, ok := p.parseTypeParameterList()
	if !ok {
		return nil
	}
	lit.TypeParams = typeParams

	if !p.expectPeek(token.LPAREN) {
		return nil
	}
	lit.Parameters = p.parseFunctionParameters()
	if lit.Parameters == nil {
		return nil
	}
	if stmt.Receiver != nil {
		lit.Parameters = append([]*ast.FunctionParameter{stmt.Receiver}, lit.Parameters...)
	}

	if p.peekTokenIs(token.IDENT) || p.peekTokenIs(token.LPAREN) || p.peekTokenIs(token.NULL) {
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

func (p *Parser) parseStructStatement() *ast.StructStatement {
	stmt := &ast.StructStatement{Token: p.curToken, Fields: []*ast.StructField{}}
	if !p.expectPeek(token.IDENT) {
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	if p.peekTokenIs(token.RBRACE) {
		p.nextToken()
		return stmt
	}
	p.nextToken()
	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		if !p.curTokenIs(token.IDENT) {
			p.addErrorCurrent("struct field name must be an identifier", p.curToken.Literal)
			return nil
		}
		field := &ast.StructField{
			Token: p.curToken,
			Name:  &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal},
		}
		if p.peekTokenIs(token.QUESTION) {
			p.nextToken()
			field.Optional = true
		}
		if !p.expectPeek(token.COLON) {
			return nil
		}
		typeName, ok := p.parseTypeAnnotation()
		if !ok {
			return nil
		}
		field.TypeName = typeName
		if p.peekTokenIs(token.ASSIGN) {
			p.nextToken()
			p.nextToken()
			field.DefaultValue = p.parseExpression(LOWEST)
			field.Optional = true
		}
		stmt.Fields = append(stmt.Fields, field)
		if p.peekTokenIs(token.COMMA) {
			p.nextToken()
			if p.peekTokenIs(token.RBRACE) {
				p.nextToken()
				break
			}
			p.nextToken()
			continue
		}
		if !p.expectPeek(token.RBRACE) {
			return nil
		}
		break
	}
	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}
	return stmt
}

// parseLetStatement handles: let <name> = <value>;
func (p *Parser) parseLetStatement() *ast.LetStatement {
	stmt := &ast.LetStatement{Token: p.curToken}
	decl, ok := p.parseVariableDeclCore(true, true, "")
	if !ok {
		return nil
	}
	stmt.Name = decl.name
	stmt.TypeName = decl.typeName
	stmt.Value = decl.value
	return stmt
}

// parseConstStatement handles: const <name> = <value>;
func (p *Parser) parseConstStatement() *ast.ConstStatement {
	stmt := &ast.ConstStatement{Token: p.curToken}
	decl, ok := p.parseVariableDeclCore(true, false, "const declaration requires initializer")
	if !ok {
		return nil
	}
	stmt.Name = decl.name
	stmt.TypeName = decl.typeName
	stmt.Value = decl.value
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

	if !isAssignLikeToken(p.peekToken.Type) {
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
	op := compoundAssignOperator(opTok.Type)
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

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
		stmt.ReturnValue = nil
		return stmt
	}

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
		if isAssignLikeToken(p.peekToken.Type) {
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
	decl, ok := p.parseVariableDeclCore(false, false, "for let declaration requires initializer")
	if !ok {
		return nil
	}
	stmt.Name = decl.name
	stmt.TypeName = decl.typeName
	stmt.Value = decl.value
	return stmt
}

func (p *Parser) parseForConstStatement() ast.Statement {
	stmt := &ast.ConstStatement{Token: p.curToken}
	decl, ok := p.parseVariableDeclCore(false, false, "for const declaration requires initializer")
	if !ok {
		return nil
	}
	stmt.Name = decl.name
	stmt.TypeName = decl.typeName
	stmt.Value = decl.value
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
		if left, ok := stmt.Expression.(*ast.MemberAccessExpression); ok {
			p.nextToken() // '='
			assignTok := p.curToken
			p.nextToken() // value expression start
			value := p.parseExpression(LOWEST)
			if !p.expectPeek(token.SEMICOLON) {
				return nil
			}
			return &ast.MemberAssignStatement{
				Token: assignTok,
				Left:  left,
				Value: value,
			}
		}
		if left, ok := stmt.Expression.(*ast.PrefixExpression); ok && left.Operator == "*" {
			p.nextToken() // '='
			assignTok := p.curToken
			p.nextToken() // value expression start
			value := p.parseExpression(LOWEST)
			if !p.expectPeek(token.SEMICOLON) {
				return nil
			}
			return &ast.DerefAssignStatement{
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
