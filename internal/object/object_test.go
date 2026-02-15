package object

import (
	"testing"
	"twice/internal/ast"
	"twice/internal/token"
)

func TestObjectInspectAndType(t *testing.T) {
	objs := []Object{
		&Integer{Value: 7},
		&Float{Value: 3.5},
		&String{Value: "hi"},
		&Char{Value: 'A'},
		&Boolean{Value: true},
		&Array{ElementType: "int", Elements: []Object{&Integer{Value: 1}, &Integer{Value: 2}}},
		&Tuple{ElementTypes: []string{"int", "string"}, Elements: []Object{&Integer{Value: 1}, &String{Value: "x"}}},
		&TypeValue{Name: "int"},
		&Null{},
		&ReturnValue{Value: &Integer{Value: 1}},
		&Break{},
		&Continue{},
		&Error{Message: "boom"},
		&Builtin{Fn: func(args ...Object) Object { return &Null{} }},
	}

	for _, o := range objs {
		if o.Type() == "" {
			t.Fatalf("empty type for %T", o)
		}
		if o.Inspect() == "" && o.Type() != NULL_OBJ {
			t.Fatalf("empty inspect for %T", o)
		}
	}
}

func TestFunctionInspect(t *testing.T) {
	fn := &Function{
		Name:       "add",
		ReturnType: "int",
		Parameters: []*ast.FunctionParameter{{
			Name:     &ast.Identifier{Token: token.Token{Type: token.IDENT, Literal: "a"}, Value: "a"},
			TypeName: "int",
		}, {
			Name:         &ast.Identifier{Token: token.Token{Type: token.IDENT, Literal: "b"}, Value: "b"},
			TypeName:     "int",
			DefaultValue: &ast.IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "2"}, Value: 2},
		}},
		Body: &ast.BlockStatement{Token: token.Token{Type: token.LBRACE, Literal: "{"}, Statements: []ast.Statement{
			&ast.ReturnStatement{Token: token.Token{Type: token.RETURN, Literal: "return"}, ReturnValue: &ast.Identifier{Token: token.Token{Type: token.IDENT, Literal: "a"}, Value: "a"}},
		}},
	}

	if got := fn.Type(); got != FUNCTION_OBJ {
		t.Fatalf("Type=%s want=%s", got, FUNCTION_OBJ)
	}
	if got := fn.Inspect(); got == "" {
		t.Fatalf("Inspect empty")
	}
}

func TestEnvironmentOperations(t *testing.T) {
	global := NewEnvironment()
	if global.Has("x") {
		t.Fatalf("expected missing x")
	}

	global.Set("x", &Integer{Value: 1})
	global.SetConst("c", &Integer{Value: 9})
	global.SetType("x", "int")
	global.SetTypeAlias("Num", "int")

	if !global.Has("x") || !global.HasInCurrentScope("x") {
		t.Fatalf("expected x in scope")
	}
	if !global.IsConst("c") || !global.IsConstInCurrentScope("c") {
		t.Fatalf("expected c const")
	}
	if typ, ok := global.TypeOf("x"); !ok || typ != "int" {
		t.Fatalf("TypeOf(x)=%q,%v", typ, ok)
	}
	if ali, ok := global.TypeAlias("Num"); !ok || ali != "int" {
		t.Fatalf("TypeAlias(Num)=%q,%v", ali, ok)
	}
	if !global.HasTypeAliasInCurrentScope("Num") {
		t.Fatalf("expected alias in current scope")
	}

	child := NewEnclosedEnvironment(global)
	if _, ok := child.Get("x"); !ok {
		t.Fatalf("child should see outer x")
	}
	if !child.Assign("x", &Integer{Value: 2}) {
		t.Fatalf("assign should update outer x")
	}
	if v, _ := global.Get("x"); v.(*Integer).Value != 2 {
		t.Fatalf("outer x not updated")
	}
	if child.Assign("missing", &Integer{Value: 1}) {
		t.Fatalf("assign should fail for missing")
	}
}
