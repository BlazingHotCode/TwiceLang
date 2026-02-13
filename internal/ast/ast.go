package ast

import (
	"bytes"
	"strings"
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

// FloatLiteral represents a floating-point number like 3.14
type FloatLiteral struct {
	Token token.Token
	Value float64
}

func (fl *FloatLiteral) expressionNode()      {}
func (fl *FloatLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FloatLiteral) String() string       { return fl.Token.Literal }

// StringLiteral represents a string like "hello"
type StringLiteral struct {
	Token token.Token
	Value string
}

func (sl *StringLiteral) expressionNode()      {}
func (sl *StringLiteral) TokenLiteral() string { return sl.Token.Literal }
func (sl *StringLiteral) String() string       { return "\"" + sl.Value + "\"" }

// CharLiteral represents a character like 'a'
type CharLiteral struct {
	Token token.Token
	Value rune
}

func (cl *CharLiteral) expressionNode()      {}
func (cl *CharLiteral) TokenLiteral() string { return cl.Token.Literal }
func (cl *CharLiteral) String() string       { return "'" + string(cl.Value) + "'" }

// NullLiteral represents null.
type NullLiteral struct {
	Token token.Token
}

func (nl *NullLiteral) expressionNode()      {}
func (nl *NullLiteral) TokenLiteral() string { return nl.Token.Literal }
func (nl *NullLiteral) String() string       { return "null" }

// ArrayLiteral represents {expr1, expr2, ...}
type ArrayLiteral struct {
	Token    token.Token
	Elements []Expression
}

func (al *ArrayLiteral) expressionNode()      {}
func (al *ArrayLiteral) TokenLiteral() string { return al.Token.Literal }
func (al *ArrayLiteral) String() string {
	var out bytes.Buffer
	parts := make([]string, 0, len(al.Elements))
	for _, el := range al.Elements {
		parts = append(parts, el.String())
	}
	out.WriteString("{")
	out.WriteString(strings.Join(parts, ", "))
	out.WriteString("}")
	return out.String()
}

// LetStatement represents: let <name> = <value>;
type LetStatement struct {
	Token    token.Token // The LET token
	Name     *Identifier // Variable name
	TypeName string      // Optional explicit type annotation
	Value    Expression  // Optional initializer
}

func (ls *LetStatement) statementNode()       {}
func (ls *LetStatement) TokenLiteral() string { return ls.Token.Literal }

func (ls *LetStatement) String() string {
	var out bytes.Buffer
	out.WriteString(ls.TokenLiteral() + " ")
	out.WriteString(ls.Name.String())
	if ls.TypeName != "" {
		out.WriteString(": ")
		out.WriteString(ls.TypeName)
	}
	if ls.Value != nil {
		out.WriteString(" = ")
		out.WriteString(ls.Value.String())
	}
	out.WriteString(";")
	return out.String()
}

// ConstStatement represents: const <name> = <value>;
type ConstStatement struct {
	Token    token.Token // The CONST token
	Name     *Identifier // Variable name
	TypeName string      // Optional explicit type annotation
	Value    Expression  // The expression being assigned
}

func (cs *ConstStatement) statementNode()       {}
func (cs *ConstStatement) TokenLiteral() string { return cs.Token.Literal }

func (cs *ConstStatement) String() string {
	var out bytes.Buffer
	out.WriteString(cs.TokenLiteral() + " ")
	out.WriteString(cs.Name.String())
	if cs.TypeName != "" {
		out.WriteString(": ")
		out.WriteString(cs.TypeName)
	}
	out.WriteString(" = ")
	if cs.Value != nil {
		out.WriteString(cs.Value.String())
	}
	out.WriteString(";")
	return out.String()
}

// AssignStatement represents: <name> = <value>;
type AssignStatement struct {
	Token token.Token // The IDENT token
	Name  *Identifier
	Value Expression
}

func (as *AssignStatement) statementNode()       {}
func (as *AssignStatement) TokenLiteral() string { return as.Token.Literal }

func (as *AssignStatement) String() string {
	var out bytes.Buffer
	out.WriteString(as.Name.String())
	out.WriteString(" = ")
	if as.Value != nil {
		out.WriteString(as.Value.String())
	}
	out.WriteString(";")
	return out.String()
}

// IndexAssignStatement represents: <arrayExpr>[<indexExpr>] = <value>;
type IndexAssignStatement struct {
	Token token.Token // The ASSIGN token
	Left  *IndexExpression
	Value Expression
}

func (ias *IndexAssignStatement) statementNode()       {}
func (ias *IndexAssignStatement) TokenLiteral() string { return ias.Token.Literal }
func (ias *IndexAssignStatement) String() string {
	var out bytes.Buffer
	out.WriteString(ias.Left.String())
	out.WriteString(" = ")
	if ias.Value != nil {
		out.WriteString(ias.Value.String())
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

// WhileStatement represents while (<condition>) { <body> }
type WhileStatement struct {
	Token     token.Token
	Condition Expression
	Body      *BlockStatement
}

func (ws *WhileStatement) statementNode()       {}
func (ws *WhileStatement) TokenLiteral() string { return ws.Token.Literal }
func (ws *WhileStatement) String() string {
	var out bytes.Buffer
	out.WriteString("while (")
	if ws.Condition != nil {
		out.WriteString(ws.Condition.String())
	}
	out.WriteString(") ")
	if ws.Body != nil {
		out.WriteString("{")
		out.WriteString(ws.Body.String())
		out.WriteString("}")
	}
	return out.String()
}

// LoopStatement represents loop { <body> }, equivalent to while (true) { <body> }.
type LoopStatement struct {
	Token token.Token
	Body  *BlockStatement
}

func (ls *LoopStatement) statementNode()       {}
func (ls *LoopStatement) TokenLiteral() string { return ls.Token.Literal }
func (ls *LoopStatement) String() string {
	var out bytes.Buffer
	out.WriteString("loop ")
	if ls.Body != nil {
		out.WriteString("{")
		out.WriteString(ls.Body.String())
		out.WriteString("}")
	}
	return out.String()
}

// ForStatement represents for (<init>; <check>; <periodic>) { <body> }.
type ForStatement struct {
	Token     token.Token
	Init      Statement
	Condition Expression
	Periodic  Statement
	Body      *BlockStatement
}

func (fs *ForStatement) statementNode()       {}
func (fs *ForStatement) TokenLiteral() string { return fs.Token.Literal }
func (fs *ForStatement) String() string {
	var out bytes.Buffer
	out.WriteString("for (")
	if fs.Init != nil {
		out.WriteString(strings.TrimSuffix(fs.Init.String(), ";"))
	}
	out.WriteString("; ")
	if fs.Condition != nil {
		out.WriteString(fs.Condition.String())
	}
	out.WriteString("; ")
	if fs.Periodic != nil {
		out.WriteString(strings.TrimSuffix(fs.Periodic.String(), ";"))
	}
	out.WriteString(") ")
	if fs.Body != nil {
		out.WriteString("{")
		out.WriteString(fs.Body.String())
		out.WriteString("}")
	}
	return out.String()
}

// BreakStatement represents break;
type BreakStatement struct {
	Token token.Token
}

func (bs *BreakStatement) statementNode()       {}
func (bs *BreakStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BreakStatement) String() string       { return "break;" }

// ContinueStatement represents continue;
type ContinueStatement struct {
	Token token.Token
}

func (cs *ContinueStatement) statementNode()       {}
func (cs *ContinueStatement) TokenLiteral() string { return cs.Token.Literal }
func (cs *ContinueStatement) String() string       { return "continue;" }

// Boolean represents true or false
type Boolean struct {
	Token token.Token
	Value bool
}

func (b *Boolean) expressionNode()      {}
func (b *Boolean) TokenLiteral() string { return b.Token.Literal }
func (b *Boolean) String() string       { return b.Token.Literal }

// PrefixExpression represents !<expr> or -<expr>
type PrefixExpression struct {
	Token    token.Token // The prefix token (! or -)
	Operator string      // "!" or "-"
	Right    Expression  // The operand
}

func (pe *PrefixExpression) expressionNode()      {}
func (pe *PrefixExpression) TokenLiteral() string { return pe.Token.Literal }

func (pe *PrefixExpression) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(pe.Operator)
	out.WriteString(pe.Right.String())
	out.WriteString(")")
	return out.String()
}

// InfixExpression represents <left> <op> <right>
type InfixExpression struct {
	Token    token.Token // The operator token
	Left     Expression
	Operator string
	Right    Expression
}

func (ie *InfixExpression) expressionNode()      {}
func (ie *InfixExpression) TokenLiteral() string { return ie.Token.Literal }

func (ie *InfixExpression) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(ie.Left.String())
	out.WriteString(" " + ie.Operator + " ")
	out.WriteString(ie.Right.String())
	out.WriteString(")")
	return out.String()
}

// IndexExpression represents: <left>[<index>]
type IndexExpression struct {
	Token token.Token // The '[' token
	Left  Expression
	Index Expression
}

func (ie *IndexExpression) expressionNode()      {}
func (ie *IndexExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *IndexExpression) String() string {
	var out bytes.Buffer
	out.WriteString(ie.Left.String())
	out.WriteString("[")
	out.WriteString(ie.Index.String())
	out.WriteString("]")
	return out.String()
}

// IfExpression represents if (<condition>) <consequence> else <alternative>
type IfExpression struct {
	Token       token.Token // The IF token
	Condition   Expression
	Consequence *BlockStatement
	Alternative *BlockStatement
}

func (ie *IfExpression) expressionNode()      {}
func (ie *IfExpression) TokenLiteral() string { return ie.Token.Literal }

func (ie *IfExpression) String() string {
	var out bytes.Buffer
	out.WriteString("if")
	out.WriteString(ie.Condition.String())
	out.WriteString(" ")
	out.WriteString(ie.Consequence.String())
	if ie.Alternative != nil {
		out.WriteString("else ")
		out.WriteString(ie.Alternative.String())
	}
	return out.String()
}

// BlockStatement is a sequence of statements inside braces
type BlockStatement struct {
	Token      token.Token // The { token
	Statements []Statement
}

func (bs *BlockStatement) statementNode()       {}
func (bs *BlockStatement) TokenLiteral() string { return bs.Token.Literal }

func (bs *BlockStatement) String() string {
	var out bytes.Buffer
	for _, s := range bs.Statements {
		out.WriteString(s.String())
	}
	return out.String()
}

// FunctionLiteral represents fn(<params>) { <body> }
type FunctionLiteral struct {
	Token      token.Token // The FN token
	Name       *Identifier // Optional function name
	Parameters []*FunctionParameter
	ReturnType string
	Body       *BlockStatement
}

type FunctionParameter struct {
	Name         *Identifier
	TypeName     string
	DefaultValue Expression
}

func (fl *FunctionLiteral) expressionNode()      {}
func (fl *FunctionLiteral) TokenLiteral() string { return fl.Token.Literal }

func (fl *FunctionLiteral) String() string {
	var out bytes.Buffer
	params := []string{}
	for _, p := range fl.Parameters {
		param := p.Name.String()
		if p.TypeName != "" {
			param += ": " + p.TypeName
		}
		if p.DefaultValue != nil {
			param += " = " + p.DefaultValue.String()
		}
		params = append(params, param)
	}
	out.WriteString(fl.TokenLiteral())
	if fl.Name != nil {
		out.WriteString(" ")
		out.WriteString(fl.Name.String())
	}
	out.WriteString("(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") ")
	if fl.ReturnType != "" {
		out.WriteString(fl.ReturnType)
		out.WriteString(" ")
	}
	out.WriteString(fl.Body.String())
	return out.String()
}

type FunctionStatement struct {
	Token    token.Token
	Name     *Identifier
	Function *FunctionLiteral
}

func (fs *FunctionStatement) statementNode()       {}
func (fs *FunctionStatement) TokenLiteral() string { return fs.Token.Literal }
func (fs *FunctionStatement) String() string {
	if fs.Function != nil {
		return fs.Function.String()
	}
	return ""
}

// CallExpression represents <function>(<arguments>)
type CallExpression struct {
	Token     token.Token // The ( token
	Function  Expression  // Identifier or FunctionLiteral
	Arguments []Expression
}

func (ce *CallExpression) expressionNode()      {}
func (ce *CallExpression) TokenLiteral() string { return ce.Token.Literal }

func (ce *CallExpression) String() string {
	var out bytes.Buffer
	args := []string{}
	for _, a := range ce.Arguments {
		args = append(args, a.String())
	}
	out.WriteString(ce.Function.String())
	out.WriteString("(")
	out.WriteString(strings.Join(args, ", "))
	out.WriteString(")")
	return out.String()
}

// MethodCallExpression represents <object>.<method>(<arguments>)
type MethodCallExpression struct {
	Token     token.Token // The . token
	Object    Expression
	Method    *Identifier
	Arguments []Expression
}

func (mce *MethodCallExpression) expressionNode()      {}
func (mce *MethodCallExpression) TokenLiteral() string { return mce.Token.Literal }
func (mce *MethodCallExpression) String() string {
	var out bytes.Buffer
	args := make([]string, 0, len(mce.Arguments))
	for _, a := range mce.Arguments {
		args = append(args, a.String())
	}
	out.WriteString(mce.Object.String())
	out.WriteString(".")
	if mce.Method != nil {
		out.WriteString(mce.Method.String())
	}
	out.WriteString("(")
	out.WriteString(strings.Join(args, ", "))
	out.WriteString(")")
	return out.String()
}

type NamedArgument struct {
	Token token.Token // The argument name token
	Name  string
	Value Expression
}

func (na *NamedArgument) expressionNode()      {}
func (na *NamedArgument) TokenLiteral() string { return na.Token.Literal }
func (na *NamedArgument) String() string {
	return na.Name + " = " + na.Value.String()
}
