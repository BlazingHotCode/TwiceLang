package object

import (
	"bytes"
	"fmt"
	"strings"
	"twice/internal/ast"
)

// ObjectType identifies what kind of value we have
type ObjectType string

const (
	INTEGER_OBJ      ObjectType = "INTEGER"
	FLOAT_OBJ        ObjectType = "FLOAT"
	STRING_OBJ       ObjectType = "STRING"
	CHAR_OBJ         ObjectType = "CHAR"
	BOOLEAN_OBJ      ObjectType = "BOOLEAN"
	ARRAY_OBJ        ObjectType = "ARRAY"
	TYPE_OBJ         ObjectType = "TYPE"
	NULL_OBJ         ObjectType = "NULL"
	RETURN_VALUE_OBJ ObjectType = "RETURN_VALUE"
	BREAK_OBJ        ObjectType = "BREAK"
	CONTINUE_OBJ     ObjectType = "CONTINUE"
	ERROR_OBJ        ObjectType = "ERROR"
	FUNCTION_OBJ     ObjectType = "FUNCTION"
	BUILTIN_OBJ      ObjectType = "BUILTIN"
)

// Object is the interface for all runtime values
// Every value in our language implements this
type Object interface {
	Type() ObjectType
	Inspect() string // String representation for printing
}

// Integer represents integer values like 5, 42
type Integer struct {
	Value int64
}

func (i *Integer) Type() ObjectType { return INTEGER_OBJ }
func (i *Integer) Inspect() string  { return fmt.Sprintf("%d", i.Value) }

// Float represents floating-point values like 3.14
type Float struct {
	Value float64
}

func (f *Float) Type() ObjectType { return FLOAT_OBJ }
func (f *Float) Inspect() string  { return fmt.Sprintf("%g", f.Value) }

// String represents text values.
type String struct {
	Value string
}

func (s *String) Type() ObjectType { return STRING_OBJ }
func (s *String) Inspect() string  { return s.Value }

// Char represents a single Unicode code point.
type Char struct {
	Value rune
}

func (c *Char) Type() ObjectType { return CHAR_OBJ }
func (c *Char) Inspect() string  { return string(c.Value) }

// Boolean represents true or false
type Boolean struct {
	Value bool
}

func (b *Boolean) Type() ObjectType { return BOOLEAN_OBJ }
func (b *Boolean) Inspect() string  { return fmt.Sprintf("%t", b.Value) }

// Array is a fixed-size homogeneous array value.
type Array struct {
	ElementType string
	Elements    []Object
}

func (a *Array) Type() ObjectType { return ARRAY_OBJ }
func (a *Array) Inspect() string {
	var out bytes.Buffer
	parts := make([]string, 0, len(a.Elements))
	for _, el := range a.Elements {
		parts = append(parts, el.Inspect())
	}
	out.WriteString("{")
	out.WriteString(strings.Join(parts, ", "))
	out.WriteString("}")
	return out.String()
}

// TypeValue represents a runtime type descriptor returned by typeof.
type TypeValue struct {
	Name string
}

func (t *TypeValue) Type() ObjectType { return TYPE_OBJ }
func (t *TypeValue) Inspect() string  { return t.Name }

// Null represents the absence of value
// There's only one null value, but we use a struct for the interface
type Null struct{}

func (n *Null) Type() ObjectType { return NULL_OBJ }
func (n *Null) Inspect() string  { return "null" }

// ReturnValue wraps the value being returned
// We need this to "bubble up" return statements through nested evaluations
type ReturnValue struct {
	Value Object
}

func (rv *ReturnValue) Type() ObjectType { return RETURN_VALUE_OBJ }
func (rv *ReturnValue) Inspect() string  { return rv.Value.Inspect() }

type Break struct{}

func (b *Break) Type() ObjectType { return BREAK_OBJ }
func (b *Break) Inspect() string  { return "break" }

type Continue struct{}

func (c *Continue) Type() ObjectType { return CONTINUE_OBJ }
func (c *Continue) Inspect() string  { return "continue" }

// Error represents runtime errors (type mismatches, unknown identifiers)
type Error struct {
	Message string
}

func (e *Error) Type() ObjectType { return ERROR_OBJ }
func (e *Error) Inspect() string  { return "ERROR: " + e.Message }

// Function represents a user-defined function
// It has parameters, a body (AST block), and its own environment (closure)
type Function struct {
	Name       string
	Parameters []*ast.FunctionParameter
	ReturnType string
	Body       *ast.BlockStatement
	Env        *Environment
}

func (f *Function) Type() ObjectType { return FUNCTION_OBJ }
func (f *Function) Inspect() string {
	var out bytes.Buffer

	params := []string{}
	for _, p := range f.Parameters {
		param := p.Name.String()
		if p.TypeName != "" {
			param += ": " + p.TypeName
		}
		if p.DefaultValue != nil {
			param += " = " + p.DefaultValue.String()
		}
		params = append(params, param)
	}

	out.WriteString("fn")
	if f.Name != "" {
		out.WriteString(" ")
		out.WriteString(f.Name)
	}
	out.WriteString("(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") {\n")
	out.WriteString(f.Body.String())
	out.WriteString("\n}")

	return out.String()
}

type BuiltinFunction func(args ...Object) Object

type Builtin struct {
	Fn BuiltinFunction
}

func (b *Builtin) Type() ObjectType { return BUILTIN_OBJ }
func (b *Builtin) Inspect() string  { return "builtin function" }
