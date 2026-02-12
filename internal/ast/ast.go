package ast

import (
	"bytes"

	"twice/internal/token"
)

// Node is the base interface for all AST nodes
// Every node must provide a TokenLiteral (for debugging) and String (for printing)
type Node interface {
	TokenLiteral() string
	String() string
}

// Statement nodes don't produce values
// Examples: let x = 5; return 10;
type Statement interface {
	Node
	statementNode() // Dummy method to distinguish statements from expressions
}

// Expression nodes produce values
// Examples: 5, x, add(2, 3), 5 + 3
type Expression interface {
	Node
	expressionNode() // Dummy method to distinguish expressions from statements
}

// Program is the root node of every AST
// It contains a slice of statements (the top-level code)
type Program struct {
	Statements []Statement
}

func (p *Program) TokenLiteral() string {
	if len(p.Statements) > 0 {
		return p.Statements[0].TokenLiteral()
	}
	return ""
}

// String builds the program back into source code (useful for debugging)
func (p *Program) String() string {
	var out bytes.Buffer
	for _, s := range p.Statements {
		out.WriteString(s.String())
	}
	return out.String()
}

// Identifier represents a variable name
// It's an expression because it produces a value (the variable's value)
type Identifier struct {
	Token token.Token // The IDENT token
	Value string      // The actual name: "x", "foo"
}

func (i *Identifier) expressionNode()      {}
func (i *Identifier) TokenLiteral() string { return i.Token.Literal }
func (i *Identifier) String() string       { return i.Value }

// IntegerLiteral represents a number like 5 or 42
type IntegerLiteral struct {
	Token token.Token
	Value int64
}

func (il *IntegerLiteral) expressionNode()      {}
func (il *IntegerLiteral) TokenLiteral() string { return il.Token.Literal }
func (il *IntegerLiteral) String() string       { return il.Token.Literal }

// LetStatement represents: let <name> = <value>;
type LetStatement struct {
	Token token.Token // The LET token
	Name  *Identifier // Variable name
	Value Expression  // The expression being assigned
}

func (ls *LetStatement) statementNode()       {}
func (ls *LetStatement) TokenLiteral() string { return ls.Token.Literal }

func (ls *LetStatement) String() string {
	var out bytes.Buffer
	out.WriteString(ls.TokenLiteral() + " ")
	out.WriteString(ls.Name.String())
	out.WriteString(" = ")
	if ls.Value != nil {
		out.WriteString(ls.Value.String())
	}
	out.WriteString(";")
	return out.String()
}

// ReturnStatement represents: return <expression>;
type ReturnStatement struct {
	Token       token.Token // The RETURN token
	ReturnValue Expression
}

func (rs *ReturnStatement) statementNode()       {}
func (rs *ReturnStatement) TokenLiteral() string { return rs.Token.Literal }

func (rs *ReturnStatement) String() string {
	var out bytes.Buffer
	out.WriteString(rs.TokenLiteral() + " ")
	if rs.ReturnValue != nil {
		out.WriteString(rs.ReturnValue.String())
	}
	out.WriteString(";")
	return out.String()
}

// ExpressionStatement is a wrapper for expressions used as statements
// Example: 5 + 5; or add(2, 3);
// The expression is evaluated, then its value is discarded
type ExpressionStatement struct {
	Token      token.Token // First token of the expression
	Expression Expression
}

func (es *ExpressionStatement) statementNode()       {}
func (es *ExpressionStatement) TokenLiteral() string { return es.Token.Literal }

func (es *ExpressionStatement) String() string {
	if es.Expression != nil {
		return es.Expression.String()
	}
	return ""
}