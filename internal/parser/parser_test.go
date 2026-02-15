package parser

import (
	"strings"
	"testing"

	"twice/internal/ast"
	"twice/internal/lexer"
	"twice/internal/token"
)

func TestLetRequiresSemicolon(t *testing.T) {
	p := New(lexer.New("let x = 5"))
	_ = p.ParseProgram()

	if len(p.Errors()) == 0 {
		t.Fatalf("expected parser errors, got none")
	}
}

func TestExpressionWithoutSemicolonBecomesImplicitReturn(t *testing.T) {
	p := New(lexer.New("5"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}

	if _, ok := program.Statements[0].(*ast.ReturnStatement); !ok {
		t.Fatalf("expected implicit return statement, got=%T", program.Statements[0])
	}
}

func TestExpressionWithSemicolonIsExpressionStatement(t *testing.T) {
	p := New(lexer.New("5;"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}

	if _, ok := program.Statements[0].(*ast.ExpressionStatement); !ok {
		t.Fatalf("expected expression statement, got=%T", program.Statements[0])
	}
}

func TestReturnWithoutValueParses(t *testing.T) {
	p := New(lexer.New("fn noop() { return; }"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	fnStmt, ok := program.Statements[0].(*ast.FunctionStatement)
	if !ok {
		t.Fatalf("expected function statement, got=%T", program.Statements[0])
	}
	ret, ok := fnStmt.Function.Body.Statements[0].(*ast.ReturnStatement)
	if !ok {
		t.Fatalf("expected return statement, got=%T", fnStmt.Function.Body.Statements[0])
	}
	if ret.ReturnValue != nil {
		t.Fatalf("expected nil return value for `return;`, got=%T", ret.ReturnValue)
	}
}

func TestConstStatementParses(t *testing.T) {
	p := New(lexer.New("const x = 5;"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ConstStatement)
	if !ok {
		t.Fatalf("expected const statement, got=%T", program.Statements[0])
	}
	if stmt.Name.Value != "x" {
		t.Fatalf("wrong const name. got=%q", stmt.Name.Value)
	}
}

func TestAssignStatementParses(t *testing.T) {
	p := New(lexer.New("let x = 1; x = 2;"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 2 {
		t.Fatalf("expected 2 statements, got=%d", len(program.Statements))
	}

	if _, ok := program.Statements[1].(*ast.AssignStatement); !ok {
		t.Fatalf("expected assign statement, got=%T", program.Statements[1])
	}
}

func TestCommentsIgnoredByParser(t *testing.T) {
	input := `
let x = 1; // inline comment
/* block
comment */
x = x + 1;
`
	p := New(lexer.New(input))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 2 {
		t.Fatalf("expected 2 statements, got=%d", len(program.Statements))
	}
}

func TestTypedLetDeclarationParses(t *testing.T) {
	p := New(lexer.New("let name: string;"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("expected let statement, got=%T", program.Statements[0])
	}
	if stmt.TypeName != "string" {
		t.Fatalf("expected type annotation string, got=%q", stmt.TypeName)
	}
	if stmt.Value != nil {
		t.Fatalf("expected nil initializer for typed declaration")
	}
}

func TestTypeDeclarationParses(t *testing.T) {
	p := New(lexer.New("type NumOrText = int||string; let v: NumOrText = 1;"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 2 {
		t.Fatalf("expected 2 statements, got=%d", len(program.Statements))
	}
	td, ok := program.Statements[0].(*ast.TypeDeclStatement)
	if !ok {
		t.Fatalf("expected type declaration, got=%T", program.Statements[0])
	}
	if td.Name.Value != "NumOrText" {
		t.Fatalf("wrong alias name: %q", td.Name.Value)
	}
	if td.TypeName != "int||string" {
		t.Fatalf("wrong aliased type: %q", td.TypeName)
	}
}

func TestImportStatementParses(t *testing.T) {
	p := New(lexer.New("import twice.math as math; import twice.math.sqrt as sqrt; import local.lib;"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 3 {
		t.Fatalf("expected 3 statements, got=%d", len(program.Statements))
	}
	im0, ok := program.Statements[0].(*ast.ImportStatement)
	if !ok {
		t.Fatalf("expected import statement, got=%T", program.Statements[0])
	}
	if got := strings.Join(im0.Path, "."); got != "twice.math" {
		t.Fatalf("unexpected import path: %s", got)
	}
	if im0.Alias != "math" {
		t.Fatalf("unexpected import alias: %s", im0.Alias)
	}
	im1, ok := program.Statements[1].(*ast.ImportStatement)
	if !ok {
		t.Fatalf("expected import statement, got=%T", program.Statements[1])
	}
	if got := strings.Join(im1.Path, "."); got != "twice.math.sqrt" {
		t.Fatalf("unexpected import path: %s", got)
	}
	if im1.Alias != "sqrt" {
		t.Fatalf("unexpected import alias: %s", im1.Alias)
	}
	im2, ok := program.Statements[2].(*ast.ImportStatement)
	if !ok {
		t.Fatalf("expected import statement, got=%T", program.Statements[2])
	}
	if got := strings.Join(im2.Path, "."); got != "local.lib" {
		t.Fatalf("unexpected import path: %s", got)
	}
	if im2.Alias != "lib" {
		t.Fatalf("unexpected default alias: %s", im2.Alias)
	}
}

func TestGenericTypeDeclarationParses(t *testing.T) {
	p := New(lexer.New("type Pair<T, U> = (T, U); let p: Pair<int, string>;"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 2 {
		t.Fatalf("expected 2 statements, got=%d", len(program.Statements))
	}
	td, ok := program.Statements[0].(*ast.TypeDeclStatement)
	if !ok {
		t.Fatalf("expected type declaration, got=%T", program.Statements[0])
	}
	if len(td.TypeParams) != 2 || td.TypeParams[0] != "T" || td.TypeParams[1] != "U" {
		t.Fatalf("unexpected generic params: %#v", td.TypeParams)
	}
	if td.TypeName != "(T,U)" {
		t.Fatalf("unexpected generic alias body: %q", td.TypeName)
	}
	ls, ok := program.Statements[1].(*ast.LetStatement)
	if !ok {
		t.Fatalf("expected let statement, got=%T", program.Statements[1])
	}
	if ls.TypeName != "Pair<int,string>" {
		t.Fatalf("expected Pair<int,string>, got=%q", ls.TypeName)
	}
}

func TestNestedGenericTypeParsesWithShiftCloser(t *testing.T) {
	p := New(lexer.New("let xs: Box<Box<int>>;"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	stmt, ok := program.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("expected let statement, got=%T", program.Statements[0])
	}
	if stmt.TypeName != "Box<Box<int>>" {
		t.Fatalf("expected nested generic type, got=%q", stmt.TypeName)
	}
}

func TestTypedArrayLetDeclarationParses(t *testing.T) {
	p := New(lexer.New("let arr: int[3];"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("expected let statement, got=%T", program.Statements[0])
	}
	if stmt.TypeName != "int[3]" {
		t.Fatalf("expected type annotation int[3], got=%q", stmt.TypeName)
	}
	if stmt.Value != nil {
		t.Fatalf("expected nil initializer for typed declaration")
	}
}

func TestNestedArrayTypeParses(t *testing.T) {
	p := New(lexer.New("let grid: int[2][3];"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	stmt, ok := program.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("expected let statement, got=%T", program.Statements[0])
	}
	if stmt.TypeName != "int[2][3]" {
		t.Fatalf("expected type annotation int[2][3], got=%q", stmt.TypeName)
	}
}

func TestUnionTypeParses(t *testing.T) {
	p := New(lexer.New("let xs: (int || string)[3];"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	stmt, ok := program.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("expected let statement, got=%T", program.Statements[0])
	}
	if stmt.TypeName != "(int||string)[3]" {
		t.Fatalf("expected type annotation (int||string)[3], got=%q", stmt.TypeName)
	}
}

func TestUnionArrayOrScalarTypeParses(t *testing.T) {
	p := New(lexer.New("let v: int[2]||string;"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	stmt, ok := program.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("expected let statement, got=%T", program.Statements[0])
	}
	if stmt.TypeName != "int[2]||string" {
		t.Fatalf("expected type annotation int[2]||string, got=%q", stmt.TypeName)
	}
}

func TestUnsizedArrayTypeParseError(t *testing.T) {
	p := New(lexer.New("let xs: int[];"))
	_ = p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatalf("expected parser errors for unsized array type")
	}
	found := false
	for _, err := range p.Errors() {
		if strings.Contains(err, "array type dimensions require explicit size") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected explicit-size parse error, got=%v", p.Errors())
	}
}

func TestTupleTypeAndLiteralParses(t *testing.T) {
	p := New(lexer.New("let a: (int, string, bool) = (1, \"x\", true);"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	stmt, ok := program.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("expected let statement, got=%T", program.Statements[0])
	}
	if stmt.TypeName != "(int,string,bool)" {
		t.Fatalf("expected tuple type annotation, got=%q", stmt.TypeName)
	}
	if _, ok := stmt.Value.(*ast.TupleLiteral); !ok {
		t.Fatalf("expected tuple literal initializer, got=%T", stmt.Value)
	}
}

func TestTupleUnionTypeParses(t *testing.T) {
	p := New(lexer.New(`let v: (int, string)||string = (1, "x");`))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	stmt, ok := program.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("expected let statement, got=%T", program.Statements[0])
	}
	if stmt.TypeName != "(int,string)||string" {
		t.Fatalf("expected tuple-union annotation, got=%q", stmt.TypeName)
	}
}

func TestTupleAccessParses(t *testing.T) {
	p := New(lexer.New("a.2;"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected expression statement, got=%T", program.Statements[0])
	}
	acc, ok := stmt.Expression.(*ast.TupleAccessExpression)
	if !ok {
		t.Fatalf("expected tuple access expression, got=%T", stmt.Expression)
	}
	if acc.Index != 2 {
		t.Fatalf("expected tuple access index 2, got=%d", acc.Index)
	}
}

func TestEmptyTupleLiteralParses(t *testing.T) {
	p := New(lexer.New("let t: (int, string) = ();"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	stmt, ok := program.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("expected let statement, got=%T", program.Statements[0])
	}
	tup, ok := stmt.Value.(*ast.TupleLiteral)
	if !ok {
		t.Fatalf("expected tuple literal initializer, got=%T", stmt.Value)
	}
	if len(tup.Elements) != 0 {
		t.Fatalf("expected empty tuple literal, got=%d elements", len(tup.Elements))
	}
}

func TestArrayLiteralParsesInLet(t *testing.T) {
	p := New(lexer.New("let arr = {1, 2, 3};"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("expected let statement, got=%T", program.Statements[0])
	}
	arr, ok := stmt.Value.(*ast.ArrayLiteral)
	if !ok {
		t.Fatalf("expected array literal initializer, got=%T", stmt.Value)
	}
	if len(arr.Elements) != 3 {
		t.Fatalf("expected 3 elements, got=%d", len(arr.Elements))
	}
}

func TestStandaloneBlockStatementParses(t *testing.T) {
	p := New(lexer.New("{ let x = 1; }"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	if _, ok := program.Statements[0].(*ast.BlockStatement); !ok {
		t.Fatalf("expected block statement, got=%T", program.Statements[0])
	}
}

func TestArrayIndexExpressionParses(t *testing.T) {
	p := New(lexer.New("arr[1];"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected expression statement, got=%T", program.Statements[0])
	}
	idx, ok := stmt.Expression.(*ast.IndexExpression)
	if !ok {
		t.Fatalf("expected index expression, got=%T", stmt.Expression)
	}
	if idx.Left.String() != "arr" || idx.Index.String() != "1" {
		t.Fatalf("unexpected index expression: %s", idx.String())
	}
}

func TestArrayIndexAssignmentParses(t *testing.T) {
	p := New(lexer.New("arr[1] = 42;"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.IndexAssignStatement)
	if !ok {
		t.Fatalf("expected index assign statement, got=%T", program.Statements[0])
	}
	if stmt.Left.String() != "arr[1]" {
		t.Fatalf("unexpected left side: %s", stmt.Left.String())
	}
	if stmt.Value.String() != "42" {
		t.Fatalf("unexpected assignment value: %s", stmt.Value.String())
	}
}

func TestStructDeclarationParses(t *testing.T) {
	src := `struct Point { x: int, y?: int, z: int = 7 }`
	p := New(lexer.New(src))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	st, ok := program.Statements[0].(*ast.StructStatement)
	if !ok {
		t.Fatalf("expected struct statement, got=%T", program.Statements[0])
	}
	if st.Name == nil || st.Name.Value != "Point" {
		t.Fatalf("wrong struct name: %+v", st.Name)
	}
	if len(st.Fields) != 3 {
		t.Fatalf("expected 3 fields, got=%d", len(st.Fields))
	}
	if st.Fields[1] == nil || !st.Fields[1].Optional || st.Fields[1].Name.Value != "y" {
		t.Fatalf("expected y field to be optional")
	}
	if st.Fields[2] == nil || st.Fields[2].DefaultValue == nil {
		t.Fatalf("expected z field default value")
	}
}

func TestMemberAssignmentParses(t *testing.T) {
	p := New(lexer.New("p.x = 3;"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)
	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	st, ok := program.Statements[0].(*ast.MemberAssignStatement)
	if !ok {
		t.Fatalf("expected member assignment, got=%T", program.Statements[0])
	}
	if st.Left == nil || st.Left.Property == nil || st.Left.Property.Value != "x" {
		t.Fatalf("expected property x, got=%+v", st.Left)
	}
	if st.Value == nil || st.Value.String() != "3" {
		t.Fatalf("expected value 3, got=%v", st.Value)
	}
}

func TestPointerDeclarationAndOpsParse(t *testing.T) {
	p := New(lexer.New("let x: int = 1; let p: *int = &x; let y = *p; *p = 2;"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)
	if len(program.Statements) != 4 {
		t.Fatalf("expected 4 statements, got=%d", len(program.Statements))
	}
	ls, ok := program.Statements[1].(*ast.LetStatement)
	if !ok {
		t.Fatalf("expected second statement let, got=%T", program.Statements[1])
	}
	if ls.TypeName != "*int" {
		t.Fatalf("expected pointer type *int, got=%q", ls.TypeName)
	}
	if _, ok := ls.Value.(*ast.PrefixExpression); !ok {
		t.Fatalf("expected address-of prefix expression")
	}
	if _, ok := program.Statements[3].(*ast.DerefAssignStatement); !ok {
		t.Fatalf("expected deref assignment statement, got=%T", program.Statements[3])
	}
}

func TestArrayLengthMethodCallParses(t *testing.T) {
	p := New(lexer.New("arr.length();"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected expression statement, got=%T", program.Statements[0])
	}
	call, ok := stmt.Expression.(*ast.MethodCallExpression)
	if !ok {
		t.Fatalf("expected method call expression, got=%T", stmt.Expression)
	}
	if call.Object.String() != "arr" {
		t.Fatalf("expected method receiver arr, got=%s", call.Object.String())
	}
	if call.Method == nil || call.Method.Value != "length" {
		t.Fatalf("expected method length, got=%v", call.Method)
	}
	if len(call.Arguments) != 0 {
		t.Fatalf("expected 0 arguments, got=%d", len(call.Arguments))
	}
}

func TestNullSafeMethodCallParses(t *testing.T) {
	p := New(lexer.New("arr?.length();"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected expression statement, got=%T", program.Statements[0])
	}
	call, ok := stmt.Expression.(*ast.MethodCallExpression)
	if !ok {
		t.Fatalf("expected method call expression, got=%T", stmt.Expression)
	}
	if !call.NullSafe {
		t.Fatalf("expected null-safe method call")
	}
	if call.Method == nil || call.Method.Value != "length" {
		t.Fatalf("expected method length, got=%v", call.Method)
	}
}

func TestNullSafeAccessParses(t *testing.T) {
	p := New(lexer.New("a?.b;"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected expression statement, got=%T", program.Statements[0])
	}
	access, ok := stmt.Expression.(*ast.NullSafeAccessExpression)
	if !ok {
		t.Fatalf("expected null-safe access expression, got=%T", stmt.Expression)
	}
	if access.Object.String() != "a" {
		t.Fatalf("expected object a, got=%s", access.Object.String())
	}
	if access.Property == nil || access.Property.Value != "b" {
		t.Fatalf("expected property b, got=%v", access.Property)
	}
}

func TestNullishCoalesceAssociativityParses(t *testing.T) {
	p := New(lexer.New("a ?? b ?? c;"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected expression statement, got=%T", program.Statements[0])
	}
	top, ok := stmt.Expression.(*ast.InfixExpression)
	if !ok || top.Operator != "??" {
		t.Fatalf("expected top-level ?? infix expression, got=%T (%v)", stmt.Expression, stmt.Expression)
	}
	left, ok := top.Left.(*ast.InfixExpression)
	if !ok || left.Operator != "??" {
		t.Fatalf("expected left side to be ?? infix expression, got=%T (%v)", top.Left, top.Left)
	}
}

func TestAdditionalLiteralParsing(t *testing.T) {
	p := New(lexer.New(`3.14; "abc"; 'z'; null;`))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)
	if len(program.Statements) != 4 {
		t.Fatalf("expected 4 statements, got=%d", len(program.Statements))
	}
}

func TestTemplateLiteralParsesToConcat(t *testing.T) {
	p := New(lexer.New("`hi ${name}`;"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected expression statement, got=%T", program.Statements[0])
	}
	infix, ok := stmt.Expression.(*ast.InfixExpression)
	if !ok || infix.Operator != "+" {
		t.Fatalf("expected + infix expression, got=%T (%v)", stmt.Expression, stmt.Expression)
	}
	left, ok := infix.Left.(*ast.StringLiteral)
	if !ok || left.Value != "hi " {
		t.Fatalf("expected left string literal \"hi \", got=%T (%v)", infix.Left, infix.Left)
	}
	if _, ok := infix.Right.(*ast.Identifier); !ok {
		t.Fatalf("expected right interpolation expression identifier, got=%T", infix.Right)
	}
}

func TestBooleanOperatorPrecedenceParsing(t *testing.T) {
	p := New(lexer.New("true || false && true ^^ false;"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected expression statement, got=%T", program.Statements[0])
	}

	top, ok := stmt.Expression.(*ast.InfixExpression)
	if !ok || top.Operator != "||" {
		t.Fatalf("expected top-level || infix expression, got=%T (%v)", stmt.Expression, stmt.Expression)
	}
	right, ok := top.Right.(*ast.InfixExpression)
	if !ok || right.Operator != "^^" {
		t.Fatalf("expected right side to be ^^ infix expression, got=%T (%v)", top.Right, top.Right)
	}
	andNode, ok := right.Left.(*ast.InfixExpression)
	if !ok || andNode.Operator != "&&" {
		t.Fatalf("expected nested && infix expression, got=%T (%v)", right.Left, right.Left)
	}
}

func TestBitwiseOperatorPrecedenceParsing(t *testing.T) {
	p := New(lexer.New("1 | 2 & 3 ^ 4 << 1;"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected expression statement, got=%T", program.Statements[0])
	}

	// 1 | ((2 & 3) ^ (4 << 1))
	top, ok := stmt.Expression.(*ast.InfixExpression)
	if !ok || top.Operator != "|" {
		t.Fatalf("expected top-level | infix expression, got=%T (%v)", stmt.Expression, stmt.Expression)
	}
	xorNode, ok := top.Right.(*ast.InfixExpression)
	if !ok || xorNode.Operator != "^" {
		t.Fatalf("expected right side to be ^ infix expression, got=%T (%v)", top.Right, top.Right)
	}
	andNode, ok := xorNode.Left.(*ast.InfixExpression)
	if !ok || andNode.Operator != "&" {
		t.Fatalf("expected nested & infix expression, got=%T (%v)", xorNode.Left, xorNode.Left)
	}
	shiftNode, ok := xorNode.Right.(*ast.InfixExpression)
	if !ok || shiftNode.Operator != "<<" {
		t.Fatalf("expected nested << infix expression, got=%T (%v)", xorNode.Right, xorNode.Right)
	}
}

func TestNamedFunctionSyntaxParses(t *testing.T) {
	input := `fn add(a: int, b: int = 2) int { return a + b; }`
	p := New(lexer.New(input))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.FunctionStatement)
	if !ok {
		t.Fatalf("expected function statement, got=%T", program.Statements[0])
	}
	if stmt.Name.Value != "add" {
		t.Fatalf("expected function name add, got=%q", stmt.Name.Value)
	}
	if stmt.Function.ReturnType != "int" {
		t.Fatalf("expected return type int, got=%q", stmt.Function.ReturnType)
	}
	if len(stmt.Function.Parameters) != 2 {
		t.Fatalf("expected 2 parameters, got=%d", len(stmt.Function.Parameters))
	}
	if stmt.Function.Parameters[1].DefaultValue == nil {
		t.Fatalf("expected default value for second parameter")
	}
}

func TestMethodFunctionSyntaxParses(t *testing.T) {
	input := `fn (self: *Point) move(dx: int, dy: int) { return; }`
	p := New(lexer.New(input))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.FunctionStatement)
	if !ok {
		t.Fatalf("expected function statement, got=%T", program.Statements[0])
	}
	if stmt.Receiver == nil {
		t.Fatalf("expected method receiver")
	}
	if stmt.Receiver.Name == nil || stmt.Receiver.Name.Value != "self" {
		t.Fatalf("expected receiver name self, got=%#v", stmt.Receiver)
	}
	if stmt.Receiver.TypeName != "*Point" {
		t.Fatalf("expected receiver type *Point, got=%q", stmt.Receiver.TypeName)
	}
	if len(stmt.Function.Parameters) != 3 {
		t.Fatalf("expected receiver + 2 params, got=%d", len(stmt.Function.Parameters))
	}
	if stmt.Function.Parameters[0].Name.Value != "self" {
		t.Fatalf("expected first function parameter to be receiver self, got=%q", stmt.Function.Parameters[0].Name.Value)
	}
}

func TestGenericFunctionSyntaxParses(t *testing.T) {
	input := `fn id<T>(x: T) T { return x; }`
	p := New(lexer.New(input))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.FunctionStatement)
	if !ok {
		t.Fatalf("expected function statement, got=%T", program.Statements[0])
	}
	if len(stmt.Function.TypeParams) != 1 || stmt.Function.TypeParams[0] != "T" {
		t.Fatalf("unexpected function type params: %#v", stmt.Function.TypeParams)
	}
	if stmt.Function.Parameters[0].TypeName != "T" {
		t.Fatalf("expected parameter type T, got=%q", stmt.Function.Parameters[0].TypeName)
	}
	if stmt.Function.ReturnType != "T" {
		t.Fatalf("expected return type T, got=%q", stmt.Function.ReturnType)
	}
}

func TestGenericCallSyntaxParses(t *testing.T) {
	p := New(lexer.New(`fn id<T>(x: T) T { return x; } let a = id<int>(7);`))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)
	if len(program.Statements) != 2 {
		t.Fatalf("expected 2 statements, got=%d", len(program.Statements))
	}
	ls, ok := program.Statements[1].(*ast.LetStatement)
	if !ok {
		t.Fatalf("expected let statement, got=%T", program.Statements[1])
	}
	call, ok := ls.Value.(*ast.CallExpression)
	if !ok {
		t.Fatalf("expected call expression, got=%T", ls.Value)
	}
	if len(call.TypeArguments) != 1 || call.TypeArguments[0] != "int" {
		t.Fatalf("expected generic type args [int], got=%v", call.TypeArguments)
	}
}

func TestNewListExpressionParses(t *testing.T) {
	p := New(lexer.New(`let xs: List<int> = new List<int>(1, 2, 3);`))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)
	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	ls, ok := program.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("expected let statement, got=%T", program.Statements[0])
	}
	if ls.TypeName != "List<int>" {
		t.Fatalf("expected List<int> type annotation, got=%q", ls.TypeName)
	}
	nx, ok := ls.Value.(*ast.NewExpression)
	if !ok {
		t.Fatalf("expected new expression, got=%T", ls.Value)
	}
	if nx.TypeName != "List<int>" {
		t.Fatalf("expected new List<int>, got=%q", nx.TypeName)
	}
	if len(nx.Arguments) != 3 {
		t.Fatalf("expected 3 constructor args, got=%d", len(nx.Arguments))
	}
}

func TestNewMapExpressionParses(t *testing.T) {
	p := New(lexer.New(`let m: Map<string,int> = new Map<string,int>(("a", 1), ("b", 2));`))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)
	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	ls, ok := program.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("expected let statement, got=%T", program.Statements[0])
	}
	if ls.TypeName != "Map<string,int>" {
		t.Fatalf("expected Map<string,int> type annotation, got=%q", ls.TypeName)
	}
	nx, ok := ls.Value.(*ast.NewExpression)
	if !ok {
		t.Fatalf("expected new expression, got=%T", ls.Value)
	}
	if nx.TypeName != "Map<string,int>" {
		t.Fatalf("expected new Map<string,int>, got=%q", nx.TypeName)
	}
	if len(nx.Arguments) != 2 {
		t.Fatalf("expected 2 constructor args, got=%d", len(nx.Arguments))
	}
	for i, a := range nx.Arguments {
		tup, ok := a.(*ast.TupleLiteral)
		if !ok || len(tup.Elements) != 2 {
			t.Fatalf("expected constructor arg %d to be tuple pair, got=%T", i, a)
		}
	}
}

func TestLessThanStillParsesAsInfix(t *testing.T) {
	p := New(lexer.New(`let x = a < b;`))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)
	ls, ok := program.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("expected let statement, got=%T", program.Statements[0])
	}
	inf, ok := ls.Value.(*ast.InfixExpression)
	if !ok {
		t.Fatalf("expected infix expression, got=%T", ls.Value)
	}
	if inf.Operator != "<" {
		t.Fatalf("expected operator <, got=%q", inf.Operator)
	}
}

func TestWhileStatementParses(t *testing.T) {
	p := New(lexer.New("while (x < 3) { x = x + 1; }"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)
	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	ws, ok := program.Statements[0].(*ast.WhileStatement)
	if !ok {
		t.Fatalf("expected while statement, got=%T", program.Statements[0])
	}
	if ws.Condition == nil || ws.Body == nil {
		t.Fatalf("expected non-nil condition and body")
	}
	if len(ws.Body.Statements) != 1 {
		t.Fatalf("expected 1 while body statement, got=%d", len(ws.Body.Statements))
	}
}

func TestLoopStatementParses(t *testing.T) {
	p := New(lexer.New("loop { x = x + 1; }"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)
	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	ls, ok := program.Statements[0].(*ast.LoopStatement)
	if !ok {
		t.Fatalf("expected loop statement, got=%T", program.Statements[0])
	}
	if ls.Body == nil || len(ls.Body.Statements) != 1 {
		t.Fatalf("expected loop body with 1 statement")
	}
}

func TestForStatementParses(t *testing.T) {
	p := New(lexer.New("for (let i = 0; i < 3; i++) { x = x + i; }"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)
	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	fs, ok := program.Statements[0].(*ast.ForStatement)
	if !ok {
		t.Fatalf("expected for statement, got=%T", program.Statements[0])
	}
	if _, ok := fs.Init.(*ast.LetStatement); !ok {
		t.Fatalf("expected for init let statement, got=%T", fs.Init)
	}
	if fs.Condition == nil {
		t.Fatalf("expected non-nil for condition")
	}
	if _, ok := fs.Periodic.(*ast.AssignStatement); !ok {
		t.Fatalf("expected for periodic assign statement, got=%T", fs.Periodic)
	}
}

func TestForeachStatementParses(t *testing.T) {
	p := New(lexer.New("foreach (let item : arr) { println(item); }"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)
	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	fs, ok := program.Statements[0].(*ast.ForeachStatement)
	if !ok {
		t.Fatalf("expected foreach statement, got=%T", program.Statements[0])
	}
	if fs.Name == nil || fs.Name.Value != "item" {
		t.Fatalf("unexpected foreach variable: %#v", fs.Name)
	}
	if id, ok := fs.Iterable.(*ast.Identifier); !ok || id.Value != "arr" {
		t.Fatalf("expected iterable identifier arr, got=%T (%v)", fs.Iterable, fs.Iterable)
	}
	if fs.Body == nil || len(fs.Body.Statements) != 1 {
		t.Fatalf("expected foreach body with 1 statement")
	}
}

func TestBreakAndContinueStatementsParse(t *testing.T) {
	p := New(lexer.New("while (true) { continue; break; }"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	ws, ok := program.Statements[0].(*ast.WhileStatement)
	if !ok {
		t.Fatalf("expected while statement, got=%T", program.Statements[0])
	}
	if len(ws.Body.Statements) != 2 {
		t.Fatalf("expected 2 body statements, got=%d", len(ws.Body.Statements))
	}
	if _, ok := ws.Body.Statements[0].(*ast.ContinueStatement); !ok {
		t.Fatalf("expected continue statement, got=%T", ws.Body.Statements[0])
	}
	if _, ok := ws.Body.Statements[1].(*ast.BreakStatement); !ok {
		t.Fatalf("expected break statement, got=%T", ws.Body.Statements[1])
	}
}

func TestModuloHasProductPrecedence(t *testing.T) {
	p := New(lexer.New("5 + 6 % 4;"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected expression statement, got=%T", program.Statements[0])
	}

	top, ok := stmt.Expression.(*ast.InfixExpression)
	if !ok || top.Operator != "+" {
		t.Fatalf("expected top-level + infix expression, got=%T (%v)", stmt.Expression, stmt.Expression)
	}
	right, ok := top.Right.(*ast.InfixExpression)
	if !ok || right.Operator != "%" {
		t.Fatalf("expected right side to be %% infix expression, got=%T (%v)", top.Right, top.Right)
	}
}

func checkNoParserErrors(t *testing.T, p *Parser) {
	t.Helper()
	if len(p.Errors()) == 0 {
		return
	}
	t.Fatalf("parser errors: %v", p.Errors())
}

func TestNullishMixingRequiresParens(t *testing.T) {
	tests := []struct {
		input       string
		shouldError bool
	}{
		{`let x = a ?? b;`, false},
		{`let x = (a || b) ?? c;`, false},
		{`let x = a ?? (b && c);`, false},
		{`let x = a ?? b || c;`, true},
		{`let x = a && b ?? c;`, true},
	}

	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := New(l)
		_ = p.ParseProgram()

		has := false
		for _, err := range p.Errors() {
			if strings.Contains(err, "cannot mix ?? with &&/|| without parentheses") {
				has = true
				break
			}
		}

		if tt.shouldError && !has {
			t.Fatalf("expected nullish mixing error for input: %s, got: %v", tt.input, p.Errors())
		}
		if !tt.shouldError && has {
			t.Fatalf("unexpected nullish mixing error for input: %s, got: %v", tt.input, p.Errors())
		}
	}
}

func TestPrefixExpressionParses(t *testing.T) {
	p := New(lexer.New("!true; -5;"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)
	if len(program.Statements) != 2 {
		t.Fatalf("expected 2 statements, got=%d", len(program.Statements))
	}
	for i := 0; i < 2; i++ {
		stmt, ok := program.Statements[i].(*ast.ExpressionStatement)
		if !ok {
			t.Fatalf("expected expression statement, got=%T", program.Statements[i])
		}
		if _, ok := stmt.Expression.(*ast.PrefixExpression); !ok {
			t.Fatalf("expected prefix expression, got=%T", stmt.Expression)
		}
	}
}

func TestFunctionLiteralExpressionParses(t *testing.T) {
	p := New(lexer.New("let f = fn(x: int) int { return x; };"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	stmt, ok := program.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("expected let statement, got=%T", program.Statements[0])
	}
	if _, ok := stmt.Value.(*ast.FunctionLiteral); !ok {
		t.Fatalf("expected function literal value, got=%T", stmt.Value)
	}
}

func TestForConstStatementParses(t *testing.T) {
	p := New(lexer.New("for (const i = 0; i < 1; i++) { }"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)
	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	fs, ok := program.Statements[0].(*ast.ForStatement)
	if !ok {
		t.Fatalf("expected for statement, got=%T", program.Statements[0])
	}
	if _, ok := fs.Init.(*ast.ConstStatement); !ok {
		t.Fatalf("expected const init in for, got=%T", fs.Init)
	}
}

func TestParserDetailedErrorsAndHelpers(t *testing.T) {
	p := New(lexer.New("let x = ;"))
	_ = p.ParseProgram()
	derr := p.DetailedErrors()
	if len(derr) == 0 {
		t.Fatalf("expected detailed errors")
	}

	if got := compoundAssignOperator(token.PLUS_EQ); got != "+" {
		t.Fatalf("compoundAssignOperator += got=%q", got)
	}
	if got := compoundAssignOperator(token.MINUS_EQ); got != "-" {
		t.Fatalf("compoundAssignOperator -= got=%q", got)
	}
	if got := compoundAssignOperator(token.MUL_EQ); got != "*" {
		t.Fatalf("compoundAssignOperator *= got=%q", got)
	}
	if got := compoundAssignOperator(token.DIV_EQ); got != "/" {
		t.Fatalf("compoundAssignOperator /= got=%q", got)
	}
	if got := compoundAssignOperator(token.MOD_EQ); got != "%" {
		t.Fatalf("compoundAssignOperator %%= got=%q", got)
	}
	if got := compoundAssignOperator(token.ASSIGN); got != "" {
		t.Fatalf("compoundAssignOperator = got=%q", got)
	}

	p2 := New(lexer.New(""))
	if got, ok := p2.parseOptionalArrayTypeSuffix("int[2][3]"); !ok || got != "int[2][3]" {
		t.Fatalf("parseOptionalArrayTypeSuffix got=%q ok=%v", got, ok)
	}
}

func TestParserNoPrefixErrorPath(t *testing.T) {
	p := New(lexer.New(");"))
	_ = p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatalf("expected parser error for missing prefix parse fn")
	}
	found := false
	for _, err := range p.Errors() {
		if strings.Contains(err, "no prefix parse function") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected no-prefix parser error, got=%v", p.Errors())
	}
}

func TestIfElifElseParses(t *testing.T) {
	p := New(lexer.New("if (x) { y; } elif (z) { w; } else { v; }"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)
	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	var ifx *ast.IfExpression
	switch st := program.Statements[0].(type) {
	case *ast.ExpressionStatement:
		parsed, ok := st.Expression.(*ast.IfExpression)
		if !ok {
			t.Fatalf("expected if expression, got=%T", st.Expression)
		}
		ifx = parsed
	case *ast.ReturnStatement:
		parsed, ok := st.ReturnValue.(*ast.IfExpression)
		if !ok {
			t.Fatalf("expected if expression return value, got=%T", st.ReturnValue)
		}
		ifx = parsed
	default:
		t.Fatalf("expected expression/return statement, got=%T", program.Statements[0])
	}
	if ifx.Alternative == nil {
		t.Fatalf("expected alternative block")
	}
}

func TestParserInvalidLiteralsAndCallArgs(t *testing.T) {
	p := New(lexer.New("let c = ''; let n = 12.3.4; fn f(a: int) int { return a; } f(a=1, b);"))
	_ = p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatalf("expected parser errors")
	}
}

func TestParserForClauseForms(t *testing.T) {
	p := New(lexer.New("for (; i < 2; i++) { } for (i = 0; i < 2; i++) { }"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)
	if len(program.Statements) != 2 {
		t.Fatalf("expected 2 for statements, got=%d", len(program.Statements))
	}
}

func TestMemberAccessWithoutCallParses(t *testing.T) {
	p := New(lexer.New("a.b;"))
	program := p.ParseProgram()
	checkNoParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got=%d", len(program.Statements))
	}
	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected expression statement, got=%T", program.Statements[0])
	}
	access, ok := stmt.Expression.(*ast.MemberAccessExpression)
	if !ok {
		t.Fatalf("expected member access expression, got=%T", stmt.Expression)
	}
	if access.Object.String() != "a" {
		t.Fatalf("expected object a, got=%s", access.Object.String())
	}
	if access.Property == nil || access.Property.Value != "b" {
		t.Fatalf("expected property b, got=%v", access.Property)
	}
}

func TestNullSafeAccessRequiresIdentifierParseError(t *testing.T) {
	p := New(lexer.New("a?.1;"))
	_ = p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatalf("expected parser errors for invalid null-safe access")
	}
	found := false
	for _, err := range p.Errors() {
		if strings.Contains(err, "expected next token to be IDENT") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected IDENT-after-?. parse error, got=%v", p.Errors())
	}
}

func TestConstRequiresInitializerParseError(t *testing.T) {
	p := New(lexer.New("const x;"))
	_ = p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatalf("expected parser errors for const without initializer")
	}
	found := false
	for _, err := range p.Errors() {
		if strings.Contains(err, "const declaration requires initializer") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected const-initializer parse error, got=%v", p.Errors())
	}
}

func TestLetWithoutValueRequiresTypeParseError(t *testing.T) {
	p := New(lexer.New("let x;"))
	_ = p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatalf("expected parser errors for let without value/type")
	}
	found := false
	for _, err := range p.Errors() {
		if strings.Contains(err, "let declaration without value requires explicit type annotation") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected let-without-type parse error, got=%v", p.Errors())
	}
}
