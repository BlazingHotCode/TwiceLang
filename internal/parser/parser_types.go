package parser

import (
	"fmt"
	"strings"

	"twice/internal/token"
)

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
	case token.NULL:
		base = p.curToken.Literal
	case token.LPAREN:
		p.nextToken()
		first, ok := p.parseTypeExpressionFromCurrent()
		if !ok {
			return "", false
		}
		if p.peekTokenIs(token.COMMA) {
			parts := []string{first}
			for p.peekTokenIs(token.COMMA) {
				p.nextToken()
				p.nextToken()
				next, ok := p.parseTypeExpressionFromCurrent()
				if !ok {
					return "", false
				}
				parts = append(parts, next)
			}
			if !p.expectPeek(token.RPAREN) {
				return "", false
			}
			base = "(" + strings.Join(parts, ",") + ")"
			return p.parseArrayTypeSuffixes(base)
		}
		if !p.expectPeek(token.RPAREN) {
			return "", false
		}
		base = "(" + first + ")"
	default:
		p.addErrorCurrent(fmt.Sprintf("expected type, got %s", p.curToken.Type), p.curToken.Literal)
		return "", false
	}
	return p.parseArrayTypeSuffixes(base)
}

func (p *Parser) parseArrayTypeSuffixes(base string) (string, bool) {
	out := base
	for p.peekTokenIs(token.LBRACKET) {
		p.nextToken() // '['
		if !p.expectPeek(token.INT) {
			p.addErrorPeek("array type dimensions require explicit size, use [N]", p.peekToken.Literal)
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
