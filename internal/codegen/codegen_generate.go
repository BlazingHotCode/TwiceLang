package codegen

import (
	"fmt"
	"math"
	"strings"

	"twice/internal/ast"
	"twice/internal/imports"
	"twice/internal/token"
)

func (cg *CodeGen) generateStatement(stmt ast.Statement) {
	cg.inferTypeCache = make(map[ast.Expression]valueType)
	cg.inferNameCache = make(map[ast.Expression]string)
	switch s := stmt.(type) {
	case *ast.ImportStatement:
		cg.generateImportStatement(s)
	case *ast.LetStatement:
		cg.generateLet(s)
	case *ast.ConstStatement:
		cg.generateConst(s)
	case *ast.TypeDeclStatement:
		cg.generateTypeDecl(s)
	case *ast.StructStatement:
		cg.generateStructDecl(s)
	case *ast.AssignStatement:
		cg.generateAssign(s)
	case *ast.MemberAssignStatement:
		cg.generateMemberAssign(s)
	case *ast.DerefAssignStatement:
		cg.generateDerefAssign(s)
	case *ast.IndexAssignStatement:
		cg.generateIndexAssign(s)
	case *ast.ReturnStatement:
		cg.generateReturn(s)
	case *ast.WhileStatement:
		cg.generateWhileStatement(s)
	case *ast.LoopStatement:
		cg.generateLoopStatement(s)
	case *ast.ForStatement:
		cg.generateForStatement(s)
	case *ast.ForeachStatement:
		cg.generateForeachStatement(s)
	case *ast.BreakStatement:
		cg.generateBreakStatement(s)
	case *ast.ContinueStatement:
		cg.generateContinueStatement(s)
	case *ast.ExpressionStatement:
		cg.generateExpression(s.Expression)
	case *ast.FunctionStatement:
		if s == nil || s.Name == nil {
			return
		}
		if s.Receiver != nil {
			return
		}
		key, ok := cg.funcStmtKeys[s]
		if !ok {
			if top, exists := cg.funcByName[s.Name.Value]; exists {
				key = top
			} else {
				cg.addNodeError("unknown function declaration: "+s.Name.Value, s)
				return
			}
		}
		cg.varFuncs[s.Name.Value] = key
		cg.markDeclaredInCurrentScope(s.Name.Value)
	case *ast.BlockStatement:
		cg.generateBlockStatement(s)
	}
}

func (cg *CodeGen) generateImportStatement(is *ast.ImportStatement) {
	if is == nil || len(is.Path) == 0 {
		cg.addNodeError("invalid import statement", is)
		return
	}
	alias := is.Alias
	if alias == "" {
		alias = imports.DefaultAlias(is.Path)
	}
	if alias == "" {
		cg.addNodeError("invalid import alias", is)
		return
	}
	path := imports.JoinPath(is.Path)
	if len(is.Path) == 2 {
		cg.importNamespaces[alias] = path
		return
	}
	if len(is.Path) >= 3 {
		cg.importMembers[alias] = path
		return
	}
}

// generateExpression dispatches to specific expression generators
func (cg *CodeGen) generateExpression(expr ast.Expression) {
	switch e := expr.(type) {
	case *ast.IntegerLiteral:
		cg.generateInteger(e)
	case *ast.FloatLiteral:
		cg.generateFloat(e)
	case *ast.StringLiteral:
		cg.generateString(e)
	case *ast.CharLiteral:
		cg.generateChar(e)
	case *ast.NullLiteral:
		cg.generateNull(e)
	case *ast.ArrayLiteral:
		cg.generateArrayLiteral(e)
	case *ast.TupleLiteral:
		cg.generateTupleLiteral(e)
	case *ast.NewExpression:
		cg.generateNewExpression(e)
	case *ast.Boolean:
		cg.generateBoolean(e)
	case *ast.InfixExpression:
		cg.generateInfix(e)
	case *ast.PrefixExpression:
		cg.generatePrefix(e)
	case *ast.Identifier:
		cg.generateIdentifier(e)
	case *ast.IfExpression:
		cg.generateIfExpression(e)
	case *ast.CallExpression:
		cg.generateCallExpression(e)
	case *ast.IndexExpression:
		cg.generateIndexExpression(e)
	case *ast.MethodCallExpression:
		cg.generateMethodCallExpression(e)
	case *ast.MemberAccessExpression:
		cg.generateMemberAccessExpression(e)
	case *ast.NullSafeAccessExpression:
		cg.generateNullSafeAccessExpression(e)
	case *ast.TupleAccessExpression:
		cg.generateTupleAccessExpression(e)
	case *ast.FunctionLiteral:
		key, ok := cg.funcLitKeys[e]
		if !ok {
			cg.addNodeError("function literal not registered for codegen", e)
			cg.emit("    mov $0, %%rax")
			return
		}
		fn, ok := cg.functions[key]
		if !ok || fn == nil {
			cg.addNodeError("function literal not found in compiled set", e)
			cg.emit("    mov $0, %%rax")
			return
		}
		for i, capName := range fn.Captures {
			if i >= len(fn.CaptureTypeNames) {
				break
			}
			typeName := "unknown"
			if t, ok := cg.varTypeNames[capName]; ok && t != "" {
				typeName = t
			} else if t, ok := cg.varValueTypeName[capName]; ok && t != "" {
				typeName = t
			} else {
				inferred := cg.inferCurrentValueTypeName(&ast.Identifier{Value: capName})
				if inferred != "" {
					typeName = inferred
				}
			}
			fn.CaptureTypeNames[i] = typeName
		}
		cg.emit("    lea %s(%%rip), %%rax", fn.Label)
	case *ast.NamedArgument:
		cg.addNodeError("named arguments are only valid inside function calls", e)
		cg.emit("    mov $0, %%rax")
	default:
		cg.addNodeError("unsupported expression in codegen", e)
		cg.emit("    mov $0, %%rax")
	}
}

// generateInteger loads an integer into rax
func (cg *CodeGen) generateInteger(il *ast.IntegerLiteral) {
	cg.emit("    mov $%d, %%rax", il.Value)
}

func (cg *CodeGen) generateFloat(fl *ast.FloatLiteral) {
	label := cg.stringLabel(fmt.Sprintf("%g", fl.Value))
	cg.emit("    lea %s(%%rip), %%rax", label)
}

func (cg *CodeGen) generateString(sl *ast.StringLiteral) {
	label := cg.stringLabel(sl.Value)
	cg.emit("    lea %s(%%rip), %%rax", label)
}

func (cg *CodeGen) generateChar(cl *ast.CharLiteral) {
	cg.emit("    mov $%d, %%rax", cl.Value)
}

func (cg *CodeGen) generateNull(_ *ast.NullLiteral) {
	cg.emit("    lea null_lit(%%rip), %%rax")
}

func (cg *CodeGen) generateArrayLiteral(al *ast.ArrayLiteral) {
	if al == nil || len(al.Elements) == 0 {
		cg.addNodeError("empty array literals are not supported in codegen", al)
		cg.emit("    mov $0, %%rax")
		return
	}
	elemTypeName, ok := cg.inferArrayLiteralTypeName(al)
	if !ok {
		cg.addNodeError("array literal elements must have the same type", al)
		cg.emit("    mov $0, %%rax")
		return
	}
	if elemTypeName == "null" {
		cg.addNodeError("array literal elements cannot be null", al)
		cg.emit("    mov $0, %%rax")
		return
	}

	baseOffset := cg.ensureArrayLiteralSlot(al)
	for i, el := range al.Elements {
		cg.generateExpression(el)
		cg.emit("    mov %%rax, -%d(%%rbp)", baseOffset-i*8)
	}
	cg.emit("    lea -%d(%%rbp), %%rax", baseOffset)
}

func (cg *CodeGen) generateTupleLiteral(tl *ast.TupleLiteral) {
	if tl == nil || len(tl.Elements) == 0 {
		cg.addNodeError("empty tuple literals are not supported in codegen", tl)
		cg.emit("    mov $0, %%rax")
		return
	}
	baseOffset := cg.ensureTupleLiteralSlot(tl)
	for i, el := range tl.Elements {
		cg.generateExpression(el)
		cg.emit("    mov %%rax, -%d(%%rbp)", baseOffset-i*8)
	}
	cg.emit("    lea -%d(%%rbp), %%rax", baseOffset)
}

func (cg *CodeGen) generateNewExpression(ne *ast.NewExpression) {
	if ne == nil {
		cg.emit("    mov $0, %%rax")
		return
	}
	typeName := ne.TypeName
	if resolved, ok := cg.normalizeTypeName(typeName); ok {
		typeName = resolved
	}
	if inner, ok := peelPointerType(typeName); ok {
		typeName = inner
	}
	if inner, ok := peelPointerType(typeName); ok {
		typeName = inner
	}
	if inner, ok := peelPointerType(typeName); ok {
		typeName = inner
	}
	if st, ok := cg.structDecls[typeName]; ok && st != nil {
		cg.generateStructNewExpression(ne, st, typeName)
		return
	}
	if elemType, ok := peelListType(typeName); ok {
		cg.emit("    mov $%d, %%rdi", len(ne.Arguments))
		cg.emit("    call list_new")
		cg.emit("    mov %%rax, %%rbx")
		for _, arg := range ne.Arguments {
			argType := cg.inferExpressionTypeName(arg)
			if argType != "unknown" && !cg.isAssignableTypeName(elemType, argType) {
				cg.addNodeError(fmt.Sprintf("cannot assign %s to %s", argType, elemType), arg)
				cg.emit("    mov $0, %%rax")
				return
			}
			cg.generateExpression(arg)
			cg.emit("    mov %%rbx, %%rdi")
			cg.emit("    mov %%rax, %%rsi")
			cg.emit("    call list_append")
		}
		cg.emit("    mov %%rbx, %%rax")
		return
	}
	keyType, valueType, ok := peelMapType(typeName)
	if !ok {
		cg.addNodeError("new is only supported for List<T>, Map<K,V>, and structs", ne)
		cg.emit("    mov $0, %%rax")
		return
	}
	keyKind := 0
	if keyType == "string" {
		keyKind = 1
	}
	cg.emit("    mov $%d, %%rdi", len(ne.Arguments))
	cg.emit("    mov $%d, %%rsi", keyKind)
	cg.emit("    call map_new")
	cg.emit("    mov %%rax, %%rbx")
	for _, arg := range ne.Arguments {
		tup, ok := arg.(*ast.TupleLiteral)
		if !ok || len(tup.Elements) != 2 {
			cg.addNodeError("map constructor arguments must be tuple pairs: (key, value)", arg)
			cg.emit("    mov $0, %%rax")
			return
		}
		keyExpr := tup.Elements[0]
		valExpr := tup.Elements[1]
		gotKey := cg.inferExpressionTypeName(keyExpr)
		gotVal := cg.inferExpressionTypeName(valExpr)
		if gotKey != "unknown" && !cg.isAssignableTypeName(keyType, gotKey) {
			cg.addNodeError(fmt.Sprintf("cannot assign key type %s to %s", gotKey, keyType), keyExpr)
			cg.emit("    mov $0, %%rax")
			return
		}
		if gotVal != "unknown" && !cg.isAssignableTypeName(valueType, gotVal) {
			cg.addNodeError(fmt.Sprintf("cannot assign %s to %s", gotVal, valueType), valExpr)
			cg.emit("    mov $0, %%rax")
			return
		}
		cg.generateExpression(keyExpr)
		cg.emit("    push %%rax")
		cg.generateExpression(valExpr)
		cg.emit("    mov %%rax, %%rdx")
		cg.emit("    pop %%rsi")
		cg.emit("    mov %%rbx, %%rdi")
		cg.emit("    call map_set")
	}
	cg.emit("    mov %%rbx, %%rax")
}

func (cg *CodeGen) generateStructNewExpression(ne *ast.NewExpression, st *ast.StructStatement, typeName string) {
	if st == nil {
		cg.addNodeError("unknown struct type: "+typeName, ne)
		cg.emit("    mov $0, %%rax")
		return
	}
	if len(ne.Arguments) > len(st.Fields) {
		cg.addNodeError(fmt.Sprintf("too many constructor arguments for %s", typeName), ne)
		cg.emit("    mov $0, %%rax")
		return
	}
	values := map[string]ast.Expression{}
	used := map[string]struct{}{}
	named := false
	for _, arg := range ne.Arguments {
		if _, ok := arg.(*ast.NamedArgument); ok {
			named = true
			break
		}
	}
	if named {
		for _, arg := range ne.Arguments {
			na, ok := arg.(*ast.NamedArgument)
			if !ok {
				cg.addNodeError("cannot mix named and positional constructor arguments", arg)
				cg.emit("    mov $0, %%rax")
				return
			}
			if _, exists := used[na.Name]; exists {
				cg.addNodeError("duplicate constructor argument: "+na.Name, arg)
				cg.emit("    mov $0, %%rax")
				return
			}
			fieldType, ok := structFieldType(st, na.Name)
			if !ok {
				cg.addNodeError(fmt.Sprintf("unknown field %s in %s", na.Name, typeName), arg)
				cg.emit("    mov $0, %%rax")
				return
			}
			got := cg.inferExpressionTypeName(na.Value)
			if got != "unknown" && !cg.isAssignableTypeName(fieldType, got) {
				cg.addNodeError(fmt.Sprintf("cannot assign %s to %s", got, fieldType), na.Value)
				cg.emit("    mov $0, %%rax")
				return
			}
			values[na.Name] = na.Value
			used[na.Name] = struct{}{}
		}
	} else {
		for i, arg := range ne.Arguments {
			f := st.Fields[i]
			if f == nil || f.Name == nil {
				cg.addNodeError("invalid struct field", ne)
				cg.emit("    mov $0, %%rax")
				return
			}
			got := cg.inferExpressionTypeName(arg)
			if got != "unknown" && !cg.isAssignableTypeName(f.TypeName, got) {
				cg.addNodeError(fmt.Sprintf("cannot assign %s to %s", got, f.TypeName), arg)
				cg.emit("    mov $0, %%rax")
				return
			}
			values[f.Name.Value] = arg
			used[f.Name.Value] = struct{}{}
		}
	}

	cg.emit("    mov $%d, %%rdi", len(st.Fields))
	cg.emit("    mov $1, %%rsi")
	cg.emit("    call map_new")
	cg.emit("    mov %%rax, %%rbx")
	for _, f := range st.Fields {
		if f == nil || f.Name == nil {
			continue
		}
		expr, ok := values[f.Name.Value]
		if !ok {
			if f.DefaultValue != nil {
				expr = f.DefaultValue
			} else if f.Optional {
				expr = &ast.NullLiteral{}
			} else {
				cg.addNodeError(fmt.Sprintf("missing required field %s in %s constructor", f.Name.Value, typeName), ne)
				cg.emit("    mov $0, %%rax")
				return
			}
		}
		got := cg.inferExpressionTypeName(expr)
		if got != "unknown" && !cg.isAssignableTypeName(f.TypeName, got) {
			cg.addNodeError(fmt.Sprintf("cannot assign %s to %s", got, f.TypeName), expr)
			cg.emit("    mov $0, %%rax")
			return
		}
		cg.emit("    lea %s(%%rip), %%rsi", cg.stringLabel(f.Name.Value))
		cg.generateExpression(expr)
		cg.emit("    mov %%rax, %%rdx")
		cg.emit("    mov %%rbx, %%rdi")
		cg.emit("    call map_set")
	}
	cg.emit("    mov %%rbx, %%rax")
}

func (cg *CodeGen) pickTypedEmptyLiteralTarget(typeName string, wantArray bool) (string, bool) {
	target := typeName
	if resolved, ok := cg.normalizeTypeName(typeName); ok {
		target = resolved
	}
	if target == "" || target == "unknown" {
		return "", false
	}
	if wantArray {
		if _, dims, ok := parseTypeDescriptor(target); ok && len(dims) > 0 {
			return target, true
		}
	} else if _, ok := splitTopLevelTuple(target); ok {
		return target, true
	}

	parts, isUnion := splitTopLevelUnion(target)
	if !isUnion {
		return "", false
	}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "null" {
			continue
		}
		if wantArray {
			if _, dims, ok := parseTypeDescriptor(part); ok && len(dims) > 0 {
				return part, true
			}
			continue
		}
		if _, ok := splitTopLevelTuple(part); ok {
			return part, true
		}
	}
	return "", false
}

func (cg *CodeGen) generateTypedDefaultValue(typeName string) bool {
	target := typeName
	if resolved, ok := cg.normalizeTypeName(typeName); ok {
		target = resolved
	}
	base, dims, ok := parseTypeDescriptor(target)
	if ok && len(dims) > 0 {
		cg.generateTypedArrayDefault(base, dims)
		return true
	}
	if members, ok := splitTopLevelTuple(target); ok {
		cg.generateTypedTupleDefault(members)
		return true
	}
	return false
}

func (cg *CodeGen) generateTypedArrayDefault(base string, dims []int) {
	if len(dims) == 0 {
		cg.emit("    mov $0, %%rax")
		return
	}
	count := dims[0]
	if count < 0 {
		count = 0
	}
	slots := count
	if slots == 0 {
		slots = 1
	}
	baseOffset := cg.allocateSlots(slots)
	if len(dims) > 1 {
		for i := 0; i < count; i++ {
			cg.generateTypedArrayDefault(base, dims[1:])
			cg.emit("    mov %%rax, -%d(%%rbp)", baseOffset-i*8)
		}
	} else {
		cg.emit("    lea null_lit(%%rip), %%rcx")
		for i := 0; i < count; i++ {
			cg.emit("    mov %%rcx, -%d(%%rbp)", baseOffset-i*8)
		}
	}
	cg.emit("    lea -%d(%%rbp), %%rax", baseOffset)
}

func (cg *CodeGen) generateTypedTupleDefault(members []string) {
	count := len(members)
	slots := count
	if slots == 0 {
		slots = 1
	}
	baseOffset := cg.allocateSlots(slots)
	cg.emit("    lea null_lit(%%rip), %%rcx")
	for i := 0; i < count; i++ {
		cg.emit("    mov %%rcx, -%d(%%rbp)", baseOffset-i*8)
	}
	cg.emit("    lea -%d(%%rbp), %%rax", baseOffset)
}

func (cg *CodeGen) generateIndexExpression(ie *ast.IndexExpression) {
	if ie == nil {
		cg.emit("    mov $0, %%rax")
		return
	}
	leftTypeName := cg.inferExpressionTypeName(ie.Left)
	if id, ok := ie.Left.(*ast.Identifier); ok && cg.varIsNull[id.Value] && leftTypeName != "string" {
		cg.addNodeError("cannot index null array/list", ie)
		cg.emit("    mov $0, %%rax")
		return
	}

	resolvedLeftTypeName := leftTypeName
	if normalized, ok := cg.normalizeTypeName(leftTypeName); ok {
		resolvedLeftTypeName = normalized
	}
	if inner, ok := peelPointerType(resolvedLeftTypeName); ok {
		resolvedLeftTypeName = inner
	}

	if resolvedLeftTypeName == "string" {
		if cg.inferExpressionType(ie.Index) != typeInt {
			cg.addNodeError("index must be int", ie.Index)
			cg.emit("    mov $0, %%rax")
			return
		}
		if idx, ok := cg.constIntValue(ie.Index); ok {
			if s, ok := cg.constStringValue(ie.Left); ok {
				if idx < 0 || int(idx) >= len(s) {
					cg.addNodeError(fmt.Sprintf("string index out of bounds: %d", idx), ie)
					cg.emit("    mov $0, %%rax")
					return
				}
			}
		}
		cg.emitObjectValueForAccess(ie.Left) // string pointer
		cg.emit("    push %%rax")
		cg.generateExpression(ie.Index)
		errLabel := cg.newLabel()
		lenLoop := cg.newLabel()
		lenDone := cg.newLabel()
		okLabel := cg.newLabel()
		cg.emit("    pop %%rcx")
		cg.emit("    cmp $0, %%rax")
		cg.emit("    jl %s", errLabel)
		cg.emit("    xor %%r8, %%r8")
		cg.emit("%s:", lenLoop)
		cg.emit("    cmpb $0, (%%rcx,%%r8,1)")
		cg.emit("    je %s", lenDone)
		cg.emit("    inc %%r8")
		cg.emit("    jmp %s", lenLoop)
		cg.emit("%s:", lenDone)
		cg.emit("    cmp %%r8, %%rax")
		cg.emit("    jl %s", okLabel)
		cg.emit("%s:", errLabel)
		cg.emitRuntimeFail(ie, "string index out of bounds")
		cg.emit("%s:", okLabel)
		cg.emit("    movzbq (%%rcx,%%rax), %%rax")
		return
	}

	if keyType, _, ok := peelMapType(resolvedLeftTypeName); ok {
		gotKeyType := cg.inferExpressionTypeName(ie.Index)
		if gotKeyType != "unknown" && !cg.isAssignableTypeName(keyType, gotKeyType) {
			cg.addNodeError(fmt.Sprintf("cannot use %s as map key type %s", gotKeyType, keyType), ie.Index)
			cg.emit("    mov $0, %%rax")
			return
		}
		cg.emitObjectValueForAccess(ie.Left)
		cg.emit("    push %%rax")
		cg.generateExpression(ie.Index)
		cg.emit("    pop %%rdi")
		cg.emit("    mov %%rax, %%rsi")
		cg.emit("    call map_get")
		return
	}
	if _, ok := peelListType(resolvedLeftTypeName); ok {
		if cg.inferExpressionType(ie.Index) != typeInt {
			cg.addNodeError("index must be int", ie.Index)
			cg.emit("    mov $0, %%rax")
			return
		}
		cg.emitObjectValueForAccess(ie.Left) // list pointer
		cg.emit("    push %%rax")
		cg.generateExpression(ie.Index)
		cg.emit("    pop %%rdi")
		cg.emit("    mov %%rax, %%rsi")
		cg.emit("    call list_get")
		return
	}
	elemTypeName, arrLen, ok := peelArrayType(resolvedLeftTypeName)
	if !ok {
		cg.addNodeError("index operator not supported for non-array/string/list/map type", ie)
		cg.emit("    mov $0, %%rax")
		return
	}
	if cg.inferExpressionType(ie.Index) != typeInt {
		cg.addNodeError("index must be int", ie.Index)
		cg.emit("    mov $0, %%rax")
		return
	}
	if idx, ok := cg.constIntValue(ie.Index); ok && arrLen >= 0 {
		if idx < 0 || int(idx) >= arrLen {
			cg.addNodeError(fmt.Sprintf("array index out of bounds: %d", idx), ie)
			cg.emit("    mov $0, %%rax")
			return
		}
	}

	cg.emitObjectValueForAccess(ie.Left) // array pointer
	cg.emit("    push %%rax")
	cg.generateExpression(ie.Index)
	if arrLen >= 0 {
		errLabel := cg.newLabel()
		okLabel := cg.newLabel()
		cg.emit("    cmp $0, %%rax")
		cg.emit("    jl %s", errLabel)
		cg.emit("    cmp $%d, %%rax", arrLen)
		cg.emit("    jl %s", okLabel)
		cg.emit("%s:", errLabel)
		cg.emitRuntimeFail(ie, "array index out of bounds")
		cg.emit("%s:", okLabel)
	}
	cg.emit("    imul $8, %%rax, %%rax")
	cg.emit("    pop %%rcx")
	cg.emit("    mov (%%rcx,%%rax), %%rax")

	_ = elemTypeName
}

func (cg *CodeGen) generateMethodCallExpression(mce *ast.MethodCallExpression) {
	if mce == nil || mce.Method == nil {
		cg.addNodeError("invalid method call", mce)
		cg.emit("    mov $0, %%rax")
		return
	}
	if cg.generateImportedNamespaceCall(mce) {
		return
	}
	if mce.NullSafe {
		doneLabel := cg.emitNullSafeObjectGuard(mce.Object)
		cg.generateMethodByName(mce, true)
		cg.emit("%s:", doneLabel)
		return
	}
	cg.generateMethodByName(mce, false)
}

func (cg *CodeGen) generateImportedNamespaceCall(mce *ast.MethodCallExpression) bool {
	objID, ok := mce.Object.(*ast.Identifier)
	if !ok || objID == nil {
		return false
	}
	ns, ok := cg.importNamespaces[objID.Value]
	if !ok {
		return false
	}
	target := ns + "." + mce.Method.Value
	if builtinTarget, ok := imports.BuiltinMemberTarget(target); ok {
		cg.generateImportedBuiltinCall(builtinTarget, mce.Arguments, mce)
		return true
	}
	if imports.BuiltinNamespace(ns) {
		if mce.NullSafe {
			cg.emit("    lea null_lit(%%rip), %%rax")
		} else {
			cg.addNodeError("unknown imported member: "+target, mce)
			cg.emit("    mov $0, %%rax")
		}
		return true
	}
	if key, ok := cg.varFuncs[mce.Method.Value]; ok {
		cg.generateUserFunctionCall(cg.functions[key], &ast.CallExpression{
			Token:     token.Token{Type: token.LPAREN, Literal: "("},
			Arguments: mce.Arguments,
		})
		return true
	}
	if key, ok := cg.funcByName[mce.Method.Value]; ok {
		cg.generateUserFunctionCall(cg.functions[key], &ast.CallExpression{
			Token:     token.Token{Type: token.LPAREN, Literal: "("},
			Arguments: mce.Arguments,
		})
		return true
	}
	if mce.NullSafe {
		cg.emit("    lea null_lit(%%rip), %%rax")
		return true
	}
	cg.addNodeError("unknown local import member: "+target, mce)
	cg.emit("    mov $0, %%rax")
	return true
}

func (cg *CodeGen) generateMethodByName(mce *ast.MethodCallExpression, nullSafe bool) {
	if cg.generateStructMethodCall(mce) {
		return
	}
	switch mce.Method.Value {
	case "length":
		cg.generateArrayLengthMethod(mce, nullSafe)
	case "append":
		cg.generateListAppendMethod(mce, nullSafe)
	case "remove":
		cg.generateListRemoveMethod(mce, nullSafe)
	case "insert":
		cg.generateListInsertMethod(mce, nullSafe)
	case "pop":
		cg.generateListPopMethod(mce, nullSafe)
	case "contains":
		cg.generateListContainsMethod(mce, nullSafe)
	case "clear":
		cg.generateListClearMethod(mce, nullSafe)
	case "has":
		cg.generateMapHasMethod(mce, nullSafe)
	case "removeKey":
		cg.generateMapRemoveKeyMethod(mce, nullSafe)
	default:
		cg.generateUnknownMethodError(mce, nullSafe)
	}
}

func (cg *CodeGen) generateStructMethodCall(mce *ast.MethodCallExpression) bool {
	if mce == nil || mce.Method == nil {
		return false
	}
	objType := cg.inferExpressionTypeName(mce.Object)
	if resolved, ok := cg.normalizeTypeName(objType); ok {
		objType = resolved
	}
	if noNull, changed := removeNullMember(objType); changed {
		objType = noNull
	}
	if objType == "" || objType == "unknown" || objType == "null" {
		return false
	}

	findMethod := func(receiverType string) (*compiledFunction, bool) {
		key, ok := cg.structMethods[methodFunctionKey(receiverType, mce.Method.Value)]
		if ok {
			fn, ok := cg.functions[key]
			return fn, ok && fn != nil
		}
		want := receiverType
		if resolved, ok := cg.normalizeTypeName(want); ok {
			want = resolved
		}
		for methodKey, fnKey := range cg.structMethods {
			recvType, methodName, ok := splitMethodFunctionKey(methodKey)
			if !ok || methodName != mce.Method.Value {
				continue
			}
			got := recvType
			if resolved, ok := cg.normalizeTypeName(got); ok {
				got = resolved
			}
			if got != want {
				continue
			}
			fn, ok := cg.functions[fnKey]
			return fn, ok && fn != nil
		}
		return nil, false
	}

	callWith := func(fn *compiledFunction, recv ast.Expression) bool {
		if fn == nil {
			return false
		}
		args := make([]ast.Expression, 0, 1+len(mce.Arguments))
		args = append(args, recv)
		args = append(args, mce.Arguments...)
		cg.generateUserFunctionCall(fn, &ast.CallExpression{
			Token:     token.Token{Type: token.LPAREN, Literal: "("},
			Arguments: args,
		})
		return true
	}

	if pointee, ok := peelPointerType(objType); ok {
		if fn, ok := findMethod("*" + pointee); ok {
			return callWith(fn, mce.Object)
		}
		if fn, ok := findMethod(pointee); ok {
			return callWith(fn, &ast.PrefixExpression{
				Token:    token.Token{Type: token.ASTERISK, Literal: "*"},
				Operator: "*",
				Right:    mce.Object,
			})
		}
		return false
	}

	if fn, ok := findMethod(objType); ok {
		return callWith(fn, mce.Object)
	}
	return false
}

func (cg *CodeGen) generateMemberAccessExpression(mae *ast.MemberAccessExpression) {
	if mae == nil || mae.Property == nil {
		cg.addNodeError("invalid member access", mae)
		cg.emit("    mov $0, %%rax")
		return
	}
	cg.generateMemberByName(mae.Object, mae.Property, mae, false)
}

func (cg *CodeGen) generateMemberByName(object ast.Expression, property *ast.Identifier, node ast.Node, nullSafe bool) {
	if property == nil {
		if nullSafe {
			cg.emit("    lea null_lit(%%rip), %%rax")
			return
		}
		cg.addNodeError("invalid member access", node)
		cg.emit("    mov $0, %%rax")
		return
	}
	switch property.Value {
	case "length":
		cg.generateArrayLengthProperty(object, node, nullSafe)
	default:
		if id, ok := object.(*ast.Identifier); ok && cg.varIsNull[id.Value] && !nullSafe {
			cg.addNodeError("cannot access member on null value", node)
			cg.emit("    mov $0, %%rax")
			return
		}
		if _, ok := cg.structFieldTypeForExpression(object, property); ok {
			cg.emitObjectValueForAccess(object)
			cg.emit("    push %%rax")
			cg.emit("    lea %s(%%rip), %%rsi", cg.stringLabel(property.Value))
			cg.emit("    pop %%rdi")
			cg.emit("    call map_get")
			return
		}
		if nullSafe {
			cg.emit("    lea null_lit(%%rip), %%rax")
			return
		}
		cg.addNodeError("unknown member: "+property.Value, node)
		cg.emit("    mov $0, %%rax")
	}
}

func (cg *CodeGen) generateArrayLengthMethod(mce *ast.MethodCallExpression, nullSafe bool) {
	if len(mce.Arguments) != 0 {
		if nullSafe {
			cg.emit("    lea null_lit(%%rip), %%rax")
			return
		}
		cg.addNodeError(fmt.Sprintf("length expects 0 arguments, got=%d", len(mce.Arguments)), mce)
		cg.emit("    mov $0, %%rax")
		return
	}
	if _, ok := cg.resolveListElementTypeForAccess(mce.Object, mce.NullSafe); ok {
		cg.emitObjectValueForAccess(mce.Object)
		cg.emit("    mov (%%rax), %%rax")
		return
	}
	if _, _, ok := cg.resolveMapTypesForAccess(mce.Object, mce.NullSafe); ok {
		cg.emitObjectValueForAccess(mce.Object)
		cg.emit("    mov (%%rax), %%rax")
		return
	}
	n, ok := cg.resolveLengthForAccess(mce.Object, mce.NullSafe)
	if !ok {
		if nullSafe {
			cg.emit("    lea null_lit(%%rip), %%rax")
			return
		}
		cg.addNodeError("length is only supported on arrays/lists/maps", mce)
		cg.emit("    mov $0, %%rax")
		return
	}
	if n < 0 {
		cg.addNodeError("array length is unknown at compile time", mce)
		cg.emit("    mov $0, %%rax")
		return
	}
	cg.emit("    mov $%d, %%rax", n)
}

func (cg *CodeGen) generateArrayLengthProperty(object ast.Expression, node ast.Node, nullSafe bool) {
	_, allowNullable := node.(*ast.NullSafeAccessExpression)
	if _, ok := cg.resolveListElementTypeForAccess(object, allowNullable); ok {
		cg.emitObjectValueForAccess(object)
		cg.emit("    mov (%%rax), %%rax")
		return
	}
	if _, _, ok := cg.resolveMapTypesForAccess(object, allowNullable); ok {
		cg.emitObjectValueForAccess(object)
		cg.emit("    mov (%%rax), %%rax")
		return
	}
	n, ok := cg.resolveLengthForAccess(object, allowNullable)
	if !ok {
		if nullSafe {
			cg.emit("    lea null_lit(%%rip), %%rax")
			return
		}
		cg.addNodeError("length is only supported on arrays/lists/maps", node)
		cg.emit("    mov $0, %%rax")
		return
	}
	if n < 0 {
		cg.addNodeError("array length is unknown at compile time", node)
		cg.emit("    mov $0, %%rax")
		return
	}
	cg.emit("    mov $%d, %%rax", n)
}

func (cg *CodeGen) resolveListElementTypeForAccess(object ast.Expression, allowNullable bool) (string, bool) {
	typeName := cg.inferExpressionTypeName(object)
	if resolved, ok := cg.normalizeTypeName(typeName); ok {
		typeName = resolved
	}
	if elem, ok := peelListType(typeName); ok {
		return elem, true
	}
	if !allowNullable {
		return "", false
	}
	parts, isUnion := splitTopLevelUnion(typeName)
	if !isUnion {
		return "", false
	}
	found := false
	elemType := ""
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "null" {
			continue
		}
		if inner, ok := peelPointerType(part); ok {
			part = inner
		}
		elem, ok := peelListType(part)
		if !ok {
			return "", false
		}
		if !found {
			found = true
			elemType = elem
			continue
		}
		if elemType != elem {
			return "", false
		}
	}
	if !found {
		return "", false
	}
	return elemType, true
}

func (cg *CodeGen) resolveMapTypesForAccess(object ast.Expression, allowNullable bool) (string, string, bool) {
	typeName := cg.inferExpressionTypeName(object)
	if resolved, ok := cg.normalizeTypeName(typeName); ok {
		typeName = resolved
	}
	if inner, ok := peelPointerType(typeName); ok {
		typeName = inner
	}
	if k, v, ok := peelMapType(typeName); ok {
		return k, v, true
	}
	if !allowNullable {
		return "", "", false
	}
	parts, isUnion := splitTopLevelUnion(typeName)
	if !isUnion {
		return "", "", false
	}
	found := false
	keyType := ""
	valType := ""
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "null" {
			continue
		}
		if inner, ok := peelPointerType(part); ok {
			part = inner
		}
		k, v, ok := peelMapType(part)
		if !ok {
			return "", "", false
		}
		if !found {
			found = true
			keyType = k
			valType = v
			continue
		}
		if keyType != k || valType != v {
			return "", "", false
		}
	}
	if !found {
		return "", "", false
	}
	return keyType, valType, true
}

func (cg *CodeGen) resolveLengthForAccess(object ast.Expression, allowNullable bool) (int, bool) {
	typeName := cg.inferExpressionTypeName(object)
	if resolved, ok := cg.normalizeTypeName(typeName); ok {
		typeName = resolved
	}
	if inner, ok := peelPointerType(typeName); ok {
		typeName = inner
	}
	_, n, ok := peelArrayType(typeName)
	if ok {
		return n, true
	}
	if !allowNullable {
		return 0, false
	}

	parts, isUnion := splitTopLevelUnion(typeName)
	if !isUnion {
		return 0, false
	}
	found := false
	targetLen := -1
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "null" {
			continue
		}
		if inner, ok := peelPointerType(part); ok {
			part = inner
		}
		_, n, ok := peelArrayType(part)
		if !ok {
			return 0, false
		}
		if !found {
			found = true
			targetLen = n
			continue
		}
		if targetLen != n {
			return 0, false
		}
	}
	if !found {
		return 0, false
	}
	return targetLen, true
}

func (cg *CodeGen) typeSupportsLengthField(typeName string) (supports bool, nullable bool) {
	if resolved, ok := cg.normalizeTypeName(typeName); ok {
		typeName = resolved
	}

	if _, _, ok := peelArrayType(typeName); ok {
		return true, false
	}
	if _, ok := peelListType(typeName); ok {
		return true, false
	}
	if _, _, ok := peelMapType(typeName); ok {
		return true, false
	}
	if typeName == "string" {
		return true, false
	}

	parts, isUnion := splitTopLevelUnion(typeName)
	if !isUnion {
		return false, false
	}
	sawSupported := false
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "null" {
			nullable = true
			continue
		}
		if inner, ok := peelPointerType(part); ok {
			part = inner
		}
		if _, _, ok := peelArrayType(part); ok {
			sawSupported = true
			continue
		}
		if _, ok := peelListType(part); ok {
			sawSupported = true
			continue
		}
		if _, _, ok := peelMapType(part); ok {
			sawSupported = true
			continue
		}
		if part == "string" {
			sawSupported = true
			continue
		}
		return false, nullable
	}
	return sawSupported, nullable
}

func (cg *CodeGen) typeSupportsStructFieldLookup(typeName string) (supports bool, nullable bool) {
	if resolved, ok := cg.normalizeTypeName(typeName); ok {
		typeName = resolved
	}
	if _, ok := cg.structDecls[typeName]; ok {
		return true, false
	}
	parts, isUnion := splitTopLevelUnion(typeName)
	if !isUnion {
		return false, false
	}
	sawStruct := false
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "null" {
			nullable = true
			continue
		}
		if inner, ok := peelPointerType(part); ok {
			part = inner
		}
		if _, ok := cg.structDecls[part]; ok {
			sawStruct = true
			continue
		}
		return false, nullable
	}
	return sawStruct, nullable
}

func (cg *CodeGen) generateUnknownMethodError(mce *ast.MethodCallExpression, nullSafe bool) {
	if nullSafe {
		cg.emit("    lea null_lit(%%rip), %%rax")
		return
	}
	cg.addNodeError("unknown method: "+mce.Method.Value, mce)
	cg.emit("    mov $0, %%rax")
}

func (cg *CodeGen) generateListAppendMethod(mce *ast.MethodCallExpression, nullSafe bool) {
	if len(mce.Arguments) != 1 {
		if nullSafe {
			cg.emit("    lea null_lit(%%rip), %%rax")
			return
		}
		cg.addNodeError(fmt.Sprintf("append expects 1 argument, got=%d", len(mce.Arguments)), mce)
		cg.emit("    mov $0, %%rax")
		return
	}
	elemType, ok := cg.resolveListElementTypeForAccess(mce.Object, mce.NullSafe)
	if !ok {
		if nullSafe {
			cg.emit("    lea null_lit(%%rip), %%rax")
			return
		}
		cg.addNodeError("append is only supported on lists", mce)
		cg.emit("    mov $0, %%rax")
		return
	}
	argType := cg.inferExpressionTypeName(mce.Arguments[0])
	if argType != "unknown" && !cg.isAssignableTypeName(elemType, argType) {
		cg.addNodeError(fmt.Sprintf("cannot append %s to %s", argType, withListElement(elemType)), mce.Arguments[0])
		cg.emit("    mov $0, %%rax")
		return
	}
	cg.emitObjectValueForAccess(mce.Object)
	cg.emit("    push %%rax")
	cg.generateExpression(mce.Arguments[0])
	cg.emit("    mov %%rax, %%rsi")
	cg.emit("    pop %%rdi")
	cg.emit("    call list_append")
	cg.emit("    lea null_lit(%%rip), %%rax")
}

func (cg *CodeGen) generateListPopMethod(mce *ast.MethodCallExpression, nullSafe bool) {
	if len(mce.Arguments) != 0 {
		if nullSafe {
			cg.emit("    lea null_lit(%%rip), %%rax")
			return
		}
		cg.addNodeError(fmt.Sprintf("pop expects 0 arguments, got=%d", len(mce.Arguments)), mce)
		cg.emit("    mov $0, %%rax")
		return
	}
	if _, ok := cg.resolveListElementTypeForAccess(mce.Object, mce.NullSafe); !ok {
		if nullSafe {
			cg.emit("    lea null_lit(%%rip), %%rax")
			return
		}
		cg.addNodeError("pop is only supported on lists", mce)
		cg.emit("    mov $0, %%rax")
		return
	}
	cg.emitObjectValueForAccess(mce.Object)
	cg.emit("    mov %%rax, %%rdi")
	cg.emit("    call list_pop")
}

func (cg *CodeGen) generateListClearMethod(mce *ast.MethodCallExpression, nullSafe bool) {
	if len(mce.Arguments) != 0 {
		if nullSafe {
			cg.emit("    lea null_lit(%%rip), %%rax")
			return
		}
		cg.addNodeError(fmt.Sprintf("clear expects 0 arguments, got=%d", len(mce.Arguments)), mce)
		cg.emit("    mov $0, %%rax")
		return
	}
	if _, _, ok := cg.resolveMapTypesForAccess(mce.Object, mce.NullSafe); ok {
		cg.emitObjectValueForAccess(mce.Object)
		cg.emit("    mov %%rax, %%rdi")
		cg.emit("    call map_clear")
		cg.emit("    lea null_lit(%%rip), %%rax")
		return
	}
	if _, ok := cg.resolveListElementTypeForAccess(mce.Object, mce.NullSafe); !ok {
		if nullSafe {
			cg.emit("    lea null_lit(%%rip), %%rax")
			return
		}
		cg.addNodeError("clear is only supported on lists", mce)
		cg.emit("    mov $0, %%rax")
		return
	}
	cg.emitObjectValueForAccess(mce.Object)
	cg.emit("    mov %%rax, %%rdi")
	cg.emit("    call list_clear")
	cg.emit("    lea null_lit(%%rip), %%rax")
}

func (cg *CodeGen) generateListRemoveMethod(mce *ast.MethodCallExpression, nullSafe bool) {
	if len(mce.Arguments) != 1 {
		if nullSafe {
			cg.emit("    lea null_lit(%%rip), %%rax")
			return
		}
		cg.addNodeError(fmt.Sprintf("remove expects 1 argument, got=%d", len(mce.Arguments)), mce)
		cg.emit("    mov $0, %%rax")
		return
	}
	if _, ok := cg.resolveListElementTypeForAccess(mce.Object, mce.NullSafe); !ok {
		if nullSafe {
			cg.emit("    lea null_lit(%%rip), %%rax")
			return
		}
		cg.addNodeError("remove is only supported on lists", mce)
		cg.emit("    mov $0, %%rax")
		return
	}
	if cg.inferExpressionType(mce.Arguments[0]) != typeInt {
		cg.addNodeError("remove index must be int", mce.Arguments[0])
		cg.emit("    mov $0, %%rax")
		return
	}
	cg.emitObjectValueForAccess(mce.Object)
	cg.emit("    push %%rax")
	cg.generateExpression(mce.Arguments[0])
	cg.emit("    mov %%rax, %%rsi")
	cg.emit("    pop %%rdi")
	cg.emit("    call list_remove")
}

func (cg *CodeGen) generateListInsertMethod(mce *ast.MethodCallExpression, nullSafe bool) {
	if len(mce.Arguments) != 2 {
		if nullSafe {
			cg.emit("    lea null_lit(%%rip), %%rax")
			return
		}
		cg.addNodeError(fmt.Sprintf("insert expects 2 arguments, got=%d", len(mce.Arguments)), mce)
		cg.emit("    mov $0, %%rax")
		return
	}
	elemType, ok := cg.resolveListElementTypeForAccess(mce.Object, mce.NullSafe)
	if !ok {
		if nullSafe {
			cg.emit("    lea null_lit(%%rip), %%rax")
			return
		}
		cg.addNodeError("insert is only supported on lists", mce)
		cg.emit("    mov $0, %%rax")
		return
	}
	if cg.inferExpressionType(mce.Arguments[0]) != typeInt {
		cg.addNodeError("insert index must be int", mce.Arguments[0])
		cg.emit("    mov $0, %%rax")
		return
	}
	valueType := cg.inferExpressionTypeName(mce.Arguments[1])
	if valueType != "unknown" && !cg.isAssignableTypeName(elemType, valueType) {
		cg.addNodeError(fmt.Sprintf("cannot insert %s into %s", valueType, withListElement(elemType)), mce.Arguments[1])
		cg.emit("    mov $0, %%rax")
		return
	}
	cg.emitObjectValueForAccess(mce.Object)
	cg.emit("    push %%rax")
	cg.generateExpression(mce.Arguments[0])
	cg.emit("    push %%rax")
	cg.generateExpression(mce.Arguments[1])
	cg.emit("    mov %%rax, %%rdx")
	cg.emit("    pop %%rsi")
	cg.emit("    pop %%rdi")
	cg.emit("    call list_insert")
	cg.emit("    lea null_lit(%%rip), %%rax")
}

func (cg *CodeGen) generateListContainsMethod(mce *ast.MethodCallExpression, nullSafe bool) {
	if len(mce.Arguments) != 1 {
		if nullSafe {
			cg.emit("    lea null_lit(%%rip), %%rax")
			return
		}
		cg.addNodeError(fmt.Sprintf("contains expects 1 argument, got=%d", len(mce.Arguments)), mce)
		cg.emit("    mov $0, %%rax")
		return
	}
	elemType, ok := cg.resolveListElementTypeForAccess(mce.Object, mce.NullSafe)
	if !ok {
		if nullSafe {
			cg.emit("    lea null_lit(%%rip), %%rax")
			return
		}
		cg.addNodeError("contains is only supported on lists", mce)
		cg.emit("    mov $0, %%rax")
		return
	}
	argType := cg.inferExpressionTypeName(mce.Arguments[0])
	comparable := argType == "unknown" || cg.isAssignableTypeName(elemType, argType) || cg.isAssignableTypeName(argType, elemType)
	if !comparable {
		cg.emit("    lea null_lit(%%rip), %%rax")
		return
	}
	cg.emitObjectValueForAccess(mce.Object)
	cg.emit("    push %%rax")
	cg.generateExpression(mce.Arguments[0])
	cg.emit("    mov %%rax, %%rsi")
	cg.emit("    pop %%rdi")
	if elemType == "string" {
		cg.emit("    mov $1, %%rdx")
	} else {
		cg.emit("    xor %%rdx, %%rdx")
	}
	cg.emit("    call list_contains")
}

func (cg *CodeGen) generateMapHasMethod(mce *ast.MethodCallExpression, nullSafe bool) {
	if len(mce.Arguments) != 1 {
		if nullSafe {
			cg.emit("    lea null_lit(%%rip), %%rax")
			return
		}
		cg.addNodeError(fmt.Sprintf("has expects 1 argument, got=%d", len(mce.Arguments)), mce)
		cg.emit("    mov $0, %%rax")
		return
	}
	keyType, _, ok := cg.resolveMapTypesForAccess(mce.Object, mce.NullSafe)
	if !ok {
		if nullSafe {
			cg.emit("    lea null_lit(%%rip), %%rax")
			return
		}
		cg.addNodeError("has is only supported on maps", mce)
		cg.emit("    mov $0, %%rax")
		return
	}
	gotKeyType := cg.inferExpressionTypeName(mce.Arguments[0])
	if gotKeyType != "unknown" && !cg.isAssignableTypeName(keyType, gotKeyType) {
		cg.emit("    mov $0, %%rax")
		return
	}
	cg.emitObjectValueForAccess(mce.Object)
	cg.emit("    push %%rax")
	cg.generateExpression(mce.Arguments[0])
	cg.emit("    mov %%rax, %%rsi")
	cg.emit("    pop %%rdi")
	cg.emit("    call map_has")
}

func (cg *CodeGen) generateMapRemoveKeyMethod(mce *ast.MethodCallExpression, nullSafe bool) {
	if len(mce.Arguments) != 1 {
		if nullSafe {
			cg.emit("    lea null_lit(%%rip), %%rax")
			return
		}
		cg.addNodeError(fmt.Sprintf("removeKey expects 1 argument, got=%d", len(mce.Arguments)), mce)
		cg.emit("    mov $0, %%rax")
		return
	}
	keyType, _, ok := cg.resolveMapTypesForAccess(mce.Object, mce.NullSafe)
	if !ok {
		if nullSafe {
			cg.emit("    lea null_lit(%%rip), %%rax")
			return
		}
		cg.addNodeError("removeKey is only supported on maps", mce)
		cg.emit("    mov $0, %%rax")
		return
	}
	gotKeyType := cg.inferExpressionTypeName(mce.Arguments[0])
	if gotKeyType != "unknown" && !cg.isAssignableTypeName(keyType, gotKeyType) {
		cg.emit("    lea null_lit(%%rip), %%rax")
		return
	}
	cg.emitObjectValueForAccess(mce.Object)
	cg.emit("    push %%rax")
	cg.generateExpression(mce.Arguments[0])
	cg.emit("    mov %%rax, %%rsi")
	cg.emit("    pop %%rdi")
	cg.emit("    call map_remove")
}

func (cg *CodeGen) emitObjectValueForAccess(object ast.Expression) {
	cg.generateExpression(object)
	typeName := cg.inferCurrentValueTypeName(object)
	if resolved, ok := cg.normalizeTypeName(typeName); ok {
		typeName = resolved
	}
	if _, ok := peelPointerType(typeName); ok {
		cg.emit("    lea null_lit(%%rip), %%rcx")
		cg.emit("    cmp %%rcx, %%rax")
		okLabel := cg.newLabel()
		cg.emit("    jne %s", okLabel)
		cg.emit("    lea runtime_err_null_deref(%%rip), %%rax")
		cg.emit("    call runtime_fail")
		cg.emit("%s:", okLabel)
		cg.emit("    mov (%%rax), %%rax")
	}
}

func (cg *CodeGen) emitNullSafeObjectGuard(object ast.Expression) string {
	nonNullLabel := cg.newLabel()
	doneLabel := cg.newLabel()

	cg.generateExpression(object)
	cg.emit("    lea null_lit(%%rip), %%rcx")
	cg.emit("    cmp %%rcx, %%rax")
	cg.emit("    jne %s", nonNullLabel)
	cg.emit("    lea null_lit(%%rip), %%rax")
	cg.emit("    jmp %s", doneLabel)
	cg.emit("%s:", nonNullLabel)

	return doneLabel
}

func (cg *CodeGen) generateNullSafeAccessExpression(e *ast.NullSafeAccessExpression) {
	doneLabel := cg.emitNullSafeObjectGuard(e.Object)
	cg.generateMemberByName(e.Object, e.Property, e, true)
	cg.emit("%s:", doneLabel)
}

func (cg *CodeGen) generateTupleAccessExpression(tae *ast.TupleAccessExpression) {
	if tae == nil {
		cg.emit("    mov $0, %%rax")
		return
	}
	leftType := cg.inferCurrentValueTypeName(tae.Left)
	if resolved, ok := cg.normalizeTypeName(leftType); ok {
		leftType = resolved
	}
	members, ok := splitTopLevelTuple(leftType)
	if !ok {
		cg.addNodeError("tuple access is only supported on tuples", tae)
		cg.emit("    mov $0, %%rax")
		return
	}
	if tae.Index < 0 || tae.Index >= len(members) {
		cg.addNodeError(fmt.Sprintf("tuple index out of bounds: %d", tae.Index), tae)
		cg.emit("    mov $0, %%rax")
		return
	}
	cg.generateExpression(tae.Left)
	cg.emit("    mov %d(%%rax), %%rax", tae.Index*8)
}

func (cg *CodeGen) generateBoolean(b *ast.Boolean) {
	if b.Value {
		cg.emit("    mov $1, %%rax") // true = 1
	} else {
		cg.emit("    mov $0, %%rax") // false = 0
	}
}

// generateInfix handles binary operations: left op right
// We use the stack to hold intermediate results
func (cg *CodeGen) generateInfix(ie *ast.InfixExpression) {
	if ie.Operator == "??" {
		// Evaluate left first.
		cg.generateExpression(ie.Left)
		cg.emit("    push %%rax")
		cg.emit("    lea null_lit(%%rip), %%rcx")
		cg.emit("    cmp %%rcx, %%rax")
		useRight := cg.newLabel()
		done := cg.newLabel()
		cg.emit("    je %s", useRight)

		// left is non-null -> result is left
		cg.emit("    pop %%rax")
		cg.emit("    jmp %s", done)

		// left is null -> evaluate/use right
		cg.emit("%s:", useRight)
		cg.emit("    add $8, %%rsp") // discard saved left
		cg.generateExpression(ie.Right)

		cg.emit("%s:", done)
		return
	}

	leftType := cg.inferExpressionType(ie.Left)
	rightType := cg.inferExpressionType(ie.Right)

	if leftType == typeBool && rightType == typeBool {
		cg.generateExpression(ie.Right)
		cg.emit("    push %%rax")
		cg.generateExpression(ie.Left)
		cg.emit("    pop %%rcx")
		switch ie.Operator {
		case "&&":
			cg.emit("    and %%rcx, %%rax")
			return
		case "||":
			cg.emit("    or %%rcx, %%rax")
			cg.emit("    test %%rax, %%rax")
			cg.emit("    setne %%al")
			cg.emit("    movzbq %%al, %%rax")
			return
		case "^^":
			cg.emit("    xor %%rcx, %%rax")
			return
		default:
			cg.addNodeError("unsupported boolean operator in codegen", ie)
			cg.emit("    mov $0, %%rax")
			return
		}
	}

	if leftType == typeString && ie.Operator == "+" {
		combined, ok := cg.constStringValue(ie)
		if !ok {
			if rightType == typeString {
				// runtime cstr + cstr
				cg.generateExpression(ie.Right) // right c-string ptr
				cg.emit("    push %%rax")
				cg.generateExpression(ie.Left) // left c-string ptr
				cg.emit("    pop %%rdx")
				cg.emit("    call concat_cstr_cstr")
				return
			}
			if rightType == typeInt {
				// runtime cstr + int
				cg.generateExpression(ie.Right) // int
				cg.emit("    push %%rax")
				cg.generateExpression(ie.Left) // c-string ptr
				cg.emit("    pop %%rdx")
				cg.emit("    call concat_cstr_int")
				return
			}
			cg.addNodeError("string concatenation in codegen requires compile-time known values", ie)
			cg.emit("    mov $0, %%rax")
			return
		}
		label := cg.stringLabel(combined)
		cg.emit("    lea %s(%%rip), %%rax", label)
		return
	}

	if rightType == typeString && leftType == typeInt && ie.Operator == "+" {
		// runtime int + cstr
		cg.generateExpression(ie.Right) // c-string ptr
		cg.emit("    push %%rax")
		cg.generateExpression(ie.Left) // int
		cg.emit("    pop %%rdx")
		cg.emit("    call concat_int_cstr")
		return
	}

	if isNumericType(leftType) && isNumericType(rightType) && (leftType == typeFloat || rightType == typeFloat) {
		v, ok := cg.constFloatValue(ie)
		if !ok {
			cg.addNodeError("numeric infix with float result requires compile-time known values in codegen", ie)
			cg.emit("    mov $0, %%rax")
			return
		}
		label := cg.stringLabel(fmt.Sprintf("%g", v))
		cg.emit("    lea %s(%%rip), %%rax", label)
		return
	}

	if leftType == typeChar && rightType == typeChar {
		if ie.Operator != "+" {
			cg.addNodeError("char infix supports only + in codegen", ie)
			cg.emit("    mov $0, %%rax")
			return
		}
		// char + char -> char
		cg.generateExpression(ie.Right)
		cg.emit("    push %%rax")
		cg.generateExpression(ie.Left)
		cg.emit("    pop %%rcx")
		cg.emit("    add %%rcx, %%rax")
		return
	}

	if leftType == typeChar && rightType == typeInt && ie.Operator == "+" {
		// char + int -> char
		cg.generateExpression(ie.Right)
		cg.emit("    push %%rax")
		cg.generateExpression(ie.Left)
		cg.emit("    pop %%rcx")
		cg.emit("    add %%rcx, %%rax")
		return
	}

	if leftType == typeType && rightType == typeType {
		if ie.Operator != "==" && ie.Operator != "!=" {
			cg.addNodeError("type comparisons support only == and !=", ie)
			cg.emit("    mov $0, %%rax")
			return
		}
		cg.generateExpression(ie.Right)
		cg.emit("    push %%rax")
		cg.generateExpression(ie.Left)
		cg.emit("    pop %%rcx")
		cg.emit("    cmp %%rcx, %%rax")
		if ie.Operator == "==" {
			cg.emit("    sete %%al")
		} else {
			cg.emit("    setne %%al")
		}
		cg.emit("    movzbq %%al, %%rax")
		return
	}

	if leftType != typeInt || rightType != typeInt {
		cg.addNodeError("unsupported infix operand types in codegen", ie)
		cg.emit("    mov $0, %%rax")
		return
	}

	// Generate right side first (will be in rax)
	cg.generateExpression(ie.Right)
	// Push right side to stack
	cg.emit("    push %%rax")

	// Generate left side (will be in rax)
	cg.generateExpression(ie.Left)
	// Pop right side into rcx
	cg.emit("    pop %%rcx")

	// Now rax = left, rcx = right
	switch ie.Operator {
	case "+":
		cg.emit("    add %%rcx, %%rax")
	case "-":
		cg.emit("    sub %%rcx, %%rax")
	case "*":
		cg.emit("    imul %%rcx, %%rax")
	case "/":
		cg.emit("    cqo                 # sign extend rax to rdx:rax")
		cg.emit("    idiv %%rcx          # rax = rdx:rax / rcx")
	case "%":
		cg.emit("    cqo                 # sign extend rax to rdx:rax")
		cg.emit("    idiv %%rcx          # rdx = rdx:rax %% rcx")
		cg.emit("    mov %%rdx, %%rax")
	case "&":
		cg.emit("    and %%rcx, %%rax")
	case "|":
		cg.emit("    or %%rcx, %%rax")
	case "^":
		cg.emit("    xor %%rcx, %%rax")
	case "<<":
		cg.emit("    mov %%ecx, %%ecx")
		cg.emit("    shl %%cl, %%rax")
	case ">>":
		cg.emit("    mov %%ecx, %%ecx")
		cg.emit("    sar %%cl, %%rax")
	case "<":
		cg.emit("    cmp %%rcx, %%rax")
		cg.emit("    setl %%al           # set al to 1 if less, 0 otherwise")
		cg.emit("    movzbq %%al, %%rax  # zero extend to 64 bits")
	case ">":
		cg.emit("    cmp %%rcx, %%rax")
		cg.emit("    setg %%al")
		cg.emit("    movzbq %%al, %%rax")
	case "==":
		cg.emit("    cmp %%rcx, %%rax")
		cg.emit("    sete %%al")
		cg.emit("    movzbq %%al, %%rax")
	case "!=":
		cg.emit("    cmp %%rcx, %%rax")
		cg.emit("    setne %%al")
		cg.emit("    movzbq %%al, %%rax")
	}
}

// generatePrefix handles unary operators
func (cg *CodeGen) generatePrefix(pe *ast.PrefixExpression) {
	switch pe.Operator {
	case "-":
		cg.generateExpression(pe.Right)
		cg.emit("    neg %%rax")
	case "!":
		cg.generateExpression(pe.Right)
		// !x is equivalent to x == 0
		cg.emit("    test %%rax, %%rax")
		cg.emit("    sete %%al")
		cg.emit("    movzbq %%al, %%rax")
	case "&":
		id, ok := pe.Right.(*ast.Identifier)
		if !ok {
			cg.addNodeError("address-of operator requires identifier", pe)
			cg.emit("    mov $0, %%rax")
			return
		}
		offset, exists := cg.variables[id.Value]
		if !exists {
			cg.addNodeError("identifier not found: "+id.Value, pe)
			cg.emit("    mov $0, %%rax")
			return
		}
		cg.emit("    lea -%d(%%rbp), %%rax", offset)
	case "*":
		cg.generateExpression(pe.Right)
		cg.emit("    lea null_lit(%%rip), %%rcx")
		cg.emit("    cmp %%rcx, %%rax")
		okLabel := cg.newLabel()
		cg.emit("    jne %s", okLabel)
		cg.emit("    lea runtime_err_null_deref(%%rip), %%rax")
		cg.emit("    call runtime_fail")
		cg.emit("%s:", okLabel)
		cg.emit("    mov (%%rax), %%rax")
	default:
		cg.generateExpression(pe.Right)
	}
}

func (cg *CodeGen) generateIdentifier(i *ast.Identifier) {
	offset, ok := cg.variables[i.Value]
	if !ok {
		if isTypeLiteralIdentifier(i.Value) {
			label := cg.stringLabel(i.Value)
			cg.emit("    lea %s(%%rip), %%rax", label)
			return
		}
		cg.addNodeError("identifier not found: "+i.Value, i)
		cg.emit("    mov $0, %%rax")
		return
	}
	if cg.varIsNull[i.Value] {
		if cg.varTypes[i.Value] == typeArray {
			cg.emit("    mov $0, %%rax")
			return
		}
		cg.emit("    lea null_lit(%%rip), %%rax")
		return
	}
	// Load from stack: rbp - offset
	cg.emit("    mov -%d(%%rbp), %%rax  # load %s", offset, i.Value)
}

// generateLet handles variable declarations (simplified - no stack frame yet)
func (cg *CodeGen) generateLet(ls *ast.LetStatement) {
	if cg.isDeclaredInCurrentScope(ls.Name.Value) {
		if cg.constVars[ls.Name.Value] {
			cg.addNodeError("cannot reassign const: "+ls.Name.Value, ls)
		} else {
			cg.addNodeError("identifier already declared: "+ls.Name.Value, ls)
		}
		cg.emit("    mov $0, %%rax")
		return
	}

	declared := cg.parseTypeName(ls.TypeName)
	if declared != typeUnknown {
		cg.varDeclared[ls.Name.Value] = declared
		cg.varDeclaredNames[ls.Name.Value] = ls.TypeName
	}

	inferred := typeNull
	inferredName := "null"
	if ls.Value == nil {
		targetName := ls.TypeName
		if targetName != "" {
			normalized := targetName
			if resolved, ok := cg.normalizeTypeName(targetName); ok {
				normalized = resolved
			}
			if _, ok := peelPointerType(normalized); ok && !cg.typeAllowsNullTypeName(normalized) {
				cg.addNodeError("pointer variable "+ls.Name.Value+" requires initializer unless nullable", ls)
				cg.emit("    mov $0, %%rax")
				return
			}
		}
		if targetName != "" {
			if cg.generateTypedDefaultValue(targetName) {
				if resolved, ok := cg.normalizeTypeName(targetName); ok {
					targetName = resolved
				}
				inferredName = targetName
				inferred = cg.parseTypeName(targetName)
			} else {
				cg.generateNull(&ast.NullLiteral{})
			}
		} else {
			cg.generateNull(&ast.NullLiteral{})
		}
	} else {
		generatedTypedEmpty := false
		switch lit := ls.Value.(type) {
		case *ast.ArrayLiteral:
			if len(lit.Elements) == 0 {
				target, ok := cg.pickTypedEmptyLiteralTarget(ls.TypeName, true)
				if !ok {
					cg.addNodeError("empty array literal requires array type context", ls.Value)
					cg.emit("    mov $0, %%rax")
					return
				}
				cg.generateTypedDefaultValue(target)
				inferredName = target
				inferred = cg.parseTypeName(target)
				generatedTypedEmpty = true
			}
		case *ast.TupleLiteral:
			if len(lit.Elements) == 0 {
				target, ok := cg.pickTypedEmptyLiteralTarget(ls.TypeName, false)
				if !ok {
					cg.addNodeError("empty tuple literal requires tuple type context", ls.Value)
					cg.emit("    mov $0, %%rax")
					return
				}
				cg.generateTypedDefaultValue(target)
				inferredName = target
				inferred = cg.parseTypeName(target)
				generatedTypedEmpty = true
			}
		}
		if !generatedTypedEmpty {
			cg.generateExpression(ls.Value)
			inferred = cg.inferExpressionType(ls.Value)
			inferredName = cg.inferExpressionTypeName(ls.Value)
		}
	}

	name := ls.Name.Value
	offset := cg.allocateSlots(1)
	cg.variables[name] = offset
	cg.markDeclaredInCurrentScope(name)
	targetName := ls.TypeName
	if targetName == "" {
		targetName = inferredName
	}
	if targetName != "unknown" && inferredName != "unknown" && !cg.isAssignableTypeName(targetName, inferredName) {
		cg.addNodeError(fmt.Sprintf("cannot assign %s to %s", inferredName, targetName), ls)
		cg.emit("    mov $0, %%rax")
		return
	}
	if declared != typeUnknown && declared != typeAny {
		cg.varTypes[name] = declared
	} else {
		cg.varTypes[name] = inferred
	}
	cg.varTypeNames[name] = targetName
	cg.varValueTypeName[name] = inferredName
	normalizedTargetName := targetName
	if resolved, ok := cg.normalizeTypeName(targetName); ok {
		normalizedTargetName = resolved
	}
	if _, n, ok := peelArrayType(normalizedTargetName); ok {
		cg.varArrayLen[name] = n
	} else {
		delete(cg.varArrayLen, name)
	}
	cg.varIsNull[name] = inferred == typeNull
	cg.trackKnownValue(name, cg.varTypes[name], ls.Value)
	delete(cg.varFuncs, name)
	switch v := ls.Value.(type) {
	case *ast.FunctionLiteral:
		if key, ok := cg.funcLitKeys[v]; ok {
			cg.varFuncs[name] = key
		}
	case *ast.Identifier:
		if key, ok := cg.varFuncs[v.Value]; ok {
			cg.varFuncs[name] = key
		}
	}

	cg.emit("    mov %%rax, -%d(%%rbp)  # let %s", offset, name)
}

// generateConst handles immutable variable declarations.
func (cg *CodeGen) generateConst(cs *ast.ConstStatement) {
	if cg.isDeclaredInCurrentScope(cs.Name.Value) {
		cg.addNodeError("identifier already declared: "+cs.Name.Value, cs)
		cg.emit("    mov $0, %%rax")
		return
	}

	declared := cg.parseTypeName(cs.TypeName)
	if declared != typeUnknown {
		cg.varDeclared[cs.Name.Value] = declared
		cg.varDeclaredNames[cs.Name.Value] = cs.TypeName
	}

	inferred := typeUnknown
	inferredName := "unknown"
	generatedTypedEmpty := false
	switch lit := cs.Value.(type) {
	case *ast.ArrayLiteral:
		if len(lit.Elements) == 0 {
			target, ok := cg.pickTypedEmptyLiteralTarget(cs.TypeName, true)
			if !ok {
				cg.addNodeError("empty array literal requires array type context", cs.Value)
				cg.emit("    mov $0, %%rax")
				return
			}
			cg.generateTypedDefaultValue(target)
			inferredName = target
			inferred = cg.parseTypeName(target)
			generatedTypedEmpty = true
		}
	case *ast.TupleLiteral:
		if len(lit.Elements) == 0 {
			target, ok := cg.pickTypedEmptyLiteralTarget(cs.TypeName, false)
			if !ok {
				cg.addNodeError("empty tuple literal requires tuple type context", cs.Value)
				cg.emit("    mov $0, %%rax")
				return
			}
			cg.generateTypedDefaultValue(target)
			inferredName = target
			inferred = cg.parseTypeName(target)
			generatedTypedEmpty = true
		}
	}
	if !generatedTypedEmpty {
		cg.generateExpression(cs.Value)
		inferred = cg.inferExpressionType(cs.Value)
		inferredName = cg.inferExpressionTypeName(cs.Value)
	}

	name := cs.Name.Value
	offset := cg.allocateSlots(1)
	cg.variables[name] = offset
	cg.markDeclaredInCurrentScope(name)
	cg.constVars[name] = true
	targetName := cs.TypeName
	if targetName == "" {
		targetName = inferredName
	}
	if targetName != "unknown" && inferredName != "unknown" && !cg.isAssignableTypeName(targetName, inferredName) {
		cg.addNodeError(fmt.Sprintf("cannot assign %s to %s", inferredName, targetName), cs)
		cg.emit("    mov $0, %%rax")
		return
	}
	if declared != typeUnknown && declared != typeAny {
		cg.varTypes[name] = declared
	} else {
		cg.varTypes[name] = inferred
	}
	cg.varTypeNames[name] = targetName
	cg.varValueTypeName[name] = inferredName
	normalizedTargetName := targetName
	if resolved, ok := cg.normalizeTypeName(targetName); ok {
		normalizedTargetName = resolved
	}
	if _, n, ok := peelArrayType(normalizedTargetName); ok {
		cg.varArrayLen[name] = n
	} else {
		delete(cg.varArrayLen, name)
	}
	cg.varIsNull[name] = inferred == typeNull
	cg.trackKnownValue(name, cg.varTypes[name], cs.Value)
	delete(cg.varFuncs, name)
	switch v := cs.Value.(type) {
	case *ast.FunctionLiteral:
		if key, ok := cg.funcLitKeys[v]; ok {
			cg.varFuncs[name] = key
		}
	case *ast.Identifier:
		if key, ok := cg.varFuncs[v.Value]; ok {
			cg.varFuncs[name] = key
		}
	}

	cg.emit("    mov %%rax, -%d(%%rbp)  # const %s", offset, name)
}

func (cg *CodeGen) generateTypeDecl(ts *ast.TypeDeclStatement) {
	if ts == nil || ts.Name == nil {
		cg.addNodeError("invalid type declaration", ts)
		return
	}
	name := ts.Name.Value
	if name == "type" || isBuiltinTypeName(name) {
		cg.addNodeError("cannot redefine builtin type: "+name, ts)
		return
	}
	if cg.isTypeAliasDeclaredInCurrentScope(name) {
		cg.addNodeError("type already declared: "+name, ts)
		return
	}
	if len(ts.TypeParams) == 0 {
		resolved, ok := cg.normalizeTypeName(ts.TypeName)
		if !ok || !cg.isKnownTypeName(resolved) {
			return
		}
		cg.typeAliases[name] = resolved
		cg.markTypeAliasDeclaredInCurrentScope(name)
		return
	}
	resolved, ok := cg.normalizeTypeNameWithParams(ts.TypeName, make(map[string]struct{}), toTypeParamSet(ts.TypeParams))
	if !ok || !cg.isKnownTypeNameWithParams(resolved, toTypeParamSet(ts.TypeParams)) {
		return
	}
	cg.genericTypeAliases[name] = genericTypeAlias{
		TypeParams: append([]string{}, ts.TypeParams...),
		TypeName:   resolved,
	}
	cg.markTypeAliasDeclaredInCurrentScope(name)
}

func (cg *CodeGen) generateStructDecl(ss *ast.StructStatement) {
	if ss == nil || ss.Name == nil {
		cg.addNodeError("invalid struct declaration", ss)
		return
	}
	name := ss.Name.Value
	if _, exists := cg.structDecls[name]; !exists {
		cg.structDecls[name] = ss
	}
}

// generateAssign handles variable reassignment.
func (cg *CodeGen) generateAssign(as *ast.AssignStatement) {
	offset, exists := cg.variables[as.Name.Value]
	if !exists {
		cg.addNodeError("identifier not found: "+as.Name.Value, as)
		cg.emit("    mov $0, %%rax")
		return
	}
	if cg.constVars[as.Name.Value] {
		cg.addNodeError("cannot reassign const: "+as.Name.Value, as)
		cg.emit("    mov $0, %%rax")
		return
	}

	target := cg.varTypes[as.Name.Value]
	targetName := cg.varTypeNames[as.Name.Value]
	if declared, ok := cg.varDeclared[as.Name.Value]; ok {
		if declared != typeAny {
			target = declared
		}
	}
	if declaredName, ok := cg.varDeclaredNames[as.Name.Value]; ok {
		targetName = declaredName
	}
	if targetName == "" {
		targetName = typeName(target)
	}
	inferred := typeUnknown
	inferredName := "unknown"
	generatedTypedEmpty := false
	switch lit := as.Value.(type) {
	case *ast.ArrayLiteral:
		if len(lit.Elements) == 0 {
			targetTypeName, ok := cg.pickTypedEmptyLiteralTarget(targetName, true)
			if !ok {
				cg.addNodeError("empty array literal requires array type context", as.Value)
				cg.emit("    mov $0, %%rax")
				return
			}
			cg.generateTypedDefaultValue(targetTypeName)
			inferredName = targetTypeName
			inferred = cg.parseTypeName(targetTypeName)
			generatedTypedEmpty = true
		}
	case *ast.TupleLiteral:
		if len(lit.Elements) == 0 {
			targetTypeName, ok := cg.pickTypedEmptyLiteralTarget(targetName, false)
			if !ok {
				cg.addNodeError("empty tuple literal requires tuple type context", as.Value)
				cg.emit("    mov $0, %%rax")
				return
			}
			cg.generateTypedDefaultValue(targetTypeName)
			inferredName = targetTypeName
			inferred = cg.parseTypeName(targetTypeName)
			generatedTypedEmpty = true
		}
	}
	if !generatedTypedEmpty {
		cg.generateExpression(as.Value)
		inferred = cg.inferExpressionType(as.Value)
		inferredName = cg.inferExpressionTypeName(as.Value)
	}
	cg.emit("    mov %%rax, -%d(%%rbp)  # assign %s", offset, as.Name.Value)
	if target == typeNull && inferred != typeNull {
		target = inferred
	}
	if inf, ok := as.Value.(*ast.InfixExpression); ok {
		if inf.Token.Type == token.PLUSPLUS || inf.Token.Type == token.MINUSMIN {
			if target != typeInt {
				cg.addNodeError("++/-- only supported for int variables", as)
				cg.emit("    mov $0, %%rax")
				return
			}
		}
	}
	if inferredName != "unknown" && targetName != "unknown" && !cg.isAssignableTypeName(targetName, inferredName) {
		cg.addNodeError(fmt.Sprintf("cannot assign %s to %s", inferredName, targetName), as)
		cg.emit("    mov $0, %%rax")
		return
	}
	isTargetAny := targetName == "any"
	if !isTargetAny {
		if resolved, ok := cg.normalizeTypeName(targetName); ok && resolved == "any" {
			isTargetAny = true
		}
	}
	if isTargetAny || target == typeAny {
		cg.varTypes[as.Name.Value] = inferred
	} else if _, isUnion := splitTopLevelUnion(targetName); isUnion && inferred != typeUnknown && inferred != typeNull {
		cg.varTypes[as.Name.Value] = inferred
	} else if target == typeUnknown {
		cg.varTypes[as.Name.Value] = inferred
	} else {
		cg.varTypes[as.Name.Value] = target
	}
	cg.varTypeNames[as.Name.Value] = targetName
	cg.varValueTypeName[as.Name.Value] = inferredName
	normalizedTargetName := targetName
	if resolved, ok := cg.normalizeTypeName(targetName); ok {
		normalizedTargetName = resolved
	}
	if _, n, ok := peelArrayType(normalizedTargetName); ok {
		cg.varArrayLen[as.Name.Value] = n
	} else {
		delete(cg.varArrayLen, as.Name.Value)
	}
	cg.varIsNull[as.Name.Value] = inferred == typeNull
	cg.trackKnownValue(as.Name.Value, cg.varTypes[as.Name.Value], as.Value)
	delete(cg.varFuncs, as.Name.Value)
	switch v := as.Value.(type) {
	case *ast.FunctionLiteral:
		if key, ok := cg.funcLitKeys[v]; ok {
			cg.varFuncs[as.Name.Value] = key
		}
	case *ast.Identifier:
		if key, ok := cg.varFuncs[v.Value]; ok {
			cg.varFuncs[as.Name.Value] = key
		}
	}
}

func (cg *CodeGen) generateMemberAssign(mas *ast.MemberAssignStatement) {
	if mas == nil || mas.Left == nil || mas.Left.Object == nil || mas.Left.Property == nil {
		cg.addNodeError("invalid member assignment", mas)
		cg.emit("    mov $0, %%rax")
		return
	}
	id, ok := mas.Left.Object.(*ast.Identifier)
	if !ok {
		cg.addNodeError("member assignment target must be an identifier", mas)
		cg.emit("    mov $0, %%rax")
		return
	}
	offset, exists := cg.variables[id.Value]
	if !exists {
		cg.addNodeError("identifier not found: "+id.Value, mas)
		cg.emit("    mov $0, %%rax")
		return
	}
	if cg.constVars[id.Value] {
		cg.addNodeError("cannot reassign const: "+id.Value, mas)
		cg.emit("    mov $0, %%rax")
		return
	}
	if cg.varIsNull[id.Value] {
		cg.addNodeError("cannot assign member on null value", mas)
		cg.emit("    mov $0, %%rax")
		return
	}
	fieldType, ok := cg.structFieldTypeForExpression(mas.Left.Object, mas.Left.Property)
	if !ok {
		cg.addNodeError("unknown member: "+mas.Left.Property.Value, mas)
		cg.emit("    mov $0, %%rax")
		return
	}
	gotType := cg.inferExpressionTypeName(mas.Value)
	if gotType != "unknown" && !cg.isAssignableTypeName(fieldType, gotType) {
		cg.addNodeError(fmt.Sprintf("cannot assign %s to %s", gotType, fieldType), mas.Value)
		cg.emit("    mov $0, %%rax")
		return
	}
	cg.generateExpression(mas.Value)
	cg.emit("    mov %%rax, %%rdx")
	cg.emit("    lea %s(%%rip), %%rsi", cg.stringLabel(mas.Left.Property.Value))
	cg.emit("    mov -%d(%%rbp), %%rdi", offset)
	typeName := cg.inferCurrentValueTypeName(mas.Left.Object)
	if resolved, ok := cg.normalizeTypeName(typeName); ok {
		typeName = resolved
	}
	if _, ok := peelPointerType(typeName); ok {
		okLabel := cg.newLabel()
		cg.emit("    lea null_lit(%%rip), %%rcx")
		cg.emit("    cmp %%rcx, %%rdi")
		cg.emit("    jne %s", okLabel)
		cg.emit("    lea runtime_err_null_deref(%%rip), %%rax")
		cg.emit("    call runtime_fail")
		cg.emit("%s:", okLabel)
		cg.emit("    mov (%%rdi), %%rdi")
	}
	cg.emit("    call map_set")
	cg.emit("    mov %%rdx, %%rax")
}

func (cg *CodeGen) generateDerefAssign(das *ast.DerefAssignStatement) {
	if das == nil || das.Left == nil || das.Left.Operator != "*" {
		cg.addNodeError("invalid dereference assignment", das)
		cg.emit("    mov $0, %%rax")
		return
	}
	cg.generateExpression(das.Left.Right)
	cg.emit("    lea null_lit(%%rip), %%rcx")
	cg.emit("    cmp %%rcx, %%rax")
	okLabel := cg.newLabel()
	cg.emit("    jne %s", okLabel)
	cg.emit("    lea runtime_err_null_deref(%%rip), %%rax")
	cg.emit("    call runtime_fail")
	cg.emit("%s:", okLabel)
	cg.emit("    push %%rax")
	cg.generateExpression(das.Value)
	cg.emit("    pop %%rcx")
	cg.emit("    mov %%rax, (%%rcx)")
}

func (cg *CodeGen) generateIndexAssign(ias *ast.IndexAssignStatement) {
	if ias == nil || ias.Left == nil {
		cg.addNodeError("invalid indexed assignment", ias)
		cg.emit("    mov $0, %%rax")
		return
	}
	id, ok := ias.Left.Left.(*ast.Identifier)
	if !ok {
		cg.addNodeError("indexed assignment target must be an identifier", ias.Left.Left)
		cg.emit("    mov $0, %%rax")
		return
	}
	name := id.Value
	offset, exists := cg.variables[name]
	if !exists {
		cg.addNodeError("identifier not found: "+name, ias)
		cg.emit("    mov $0, %%rax")
		return
	}
	if cg.constVars[name] {
		cg.addNodeError("cannot reassign const: "+name, ias)
		cg.emit("    mov $0, %%rax")
		return
	}
	if cg.varIsNull[name] {
		cg.addNodeError("cannot index null array/list", ias)
		cg.emit("    mov $0, %%rax")
		return
	}

	arrTypeName := cg.varTypeNames[name]
	if normalized, ok := cg.normalizeTypeName(arrTypeName); ok {
		arrTypeName = normalized
	}
	isPointerTarget := false
	if inner, ok := peelPointerType(arrTypeName); ok {
		isPointerTarget = true
		arrTypeName = inner
	}
	emitLoadTarget := func(reg string) {
		cg.emit("    mov -%d(%%rbp), %s  # load %s", offset, reg, name)
		if isPointerTarget {
			okLabel := cg.newLabel()
			cg.emit("    lea null_lit(%%rip), %%r8")
			cg.emit("    cmp %%r8, %s", reg)
			cg.emit("    jne %s", okLabel)
			cg.emit("    lea runtime_err_null_deref(%%rip), %%rax")
			cg.emit("    call runtime_fail")
			cg.emit("%s:", okLabel)
			cg.emit("    mov (%s), %s", reg, reg)
		}
	}
	if keyTypeName, valueTypeName, ok := peelMapType(arrTypeName); ok {
		gotKeyType := cg.inferExpressionTypeName(ias.Left.Index)
		if gotKeyType != "unknown" && !cg.isAssignableTypeName(keyTypeName, gotKeyType) {
			cg.addNodeError(fmt.Sprintf("cannot assign key type %s to %s", gotKeyType, keyTypeName), ias.Left.Index)
			cg.emit("    mov $0, %%rax")
			return
		}
		gotValueType := cg.inferExpressionTypeName(ias.Value)
		if gotValueType != "unknown" && !cg.isAssignableTypeName(valueTypeName, gotValueType) {
			cg.addNodeError(fmt.Sprintf("cannot assign %s to %s", gotValueType, valueTypeName), ias)
			cg.emit("    mov $0, %%rax")
			return
		}
		cg.generateExpression(ias.Left.Index)
		cg.emit("    push %%rax")
		cg.generateExpression(ias.Value)
		cg.emit("    mov %%rax, %%rdx")
		cg.emit("    pop %%rsi")
		emitLoadTarget("%rdi")
		cg.emit("    call map_set")
		cg.emit("    mov %%rdx, %%rax")
		cg.varValueTypeName[name] = arrTypeName
		cg.varIsNull[name] = false
		delete(cg.varFuncs, name)
		return
	}
	if elemTypeName, ok := peelListType(arrTypeName); ok {
		if cg.inferExpressionType(ias.Left.Index) != typeInt {
			cg.addNodeError("list index must be int", ias.Left.Index)
			cg.emit("    mov $0, %%rax")
			return
		}
		valueTypeName := cg.inferExpressionTypeName(ias.Value)
		if valueTypeName != "unknown" && !cg.isAssignableTypeName(elemTypeName, valueTypeName) {
			cg.addNodeError(fmt.Sprintf("cannot assign %s to %s", valueTypeName, elemTypeName), ias)
			cg.emit("    mov $0, %%rax")
			return
		}
		cg.generateExpression(ias.Value)
		cg.emit("    push %%rax")
		cg.generateExpression(ias.Left.Index)
		cg.emit("    mov %%rax, %%rsi")
		cg.emit("    pop %%rdx")
		emitLoadTarget("%rdi")
		cg.emit("    call list_set")
		cg.emit("    mov %%rdx, %%rax")
		cg.varValueTypeName[name] = arrTypeName
		cg.varIsNull[name] = false
		delete(cg.varFuncs, name)
		return
	}
	elemTypeName, arrLen, ok := peelArrayType(arrTypeName)
	if !ok {
		cg.addNodeError("indexed assignment target is not an array/list/map", ias)
		cg.emit("    mov $0, %%rax")
		return
	}
	if cg.inferExpressionType(ias.Left.Index) != typeInt {
		cg.addNodeError("array index must be int", ias.Left.Index)
		cg.emit("    mov $0, %%rax")
		return
	}
	if idx, ok := cg.constIntValue(ias.Left.Index); ok && arrLen >= 0 {
		if idx < 0 || int(idx) >= arrLen {
			cg.addNodeError(fmt.Sprintf("array index out of bounds: %d", idx), ias)
			cg.emit("    mov $0, %%rax")
			return
		}
	}

	valueTypeName := cg.inferExpressionTypeName(ias.Value)
	if valueTypeName != "unknown" && !cg.isAssignableTypeName(elemTypeName, valueTypeName) {
		cg.addNodeError(fmt.Sprintf("cannot assign %s to %s", valueTypeName, elemTypeName), ias)
		cg.emit("    mov $0, %%rax")
		return
	}

	cg.generateExpression(ias.Value)
	cg.emit("    push %%rax")
	cg.generateExpression(ias.Left.Index)
	if arrLen >= 0 {
		errLabel := cg.newLabel()
		okLabel := cg.newLabel()
		cg.emit("    cmp $0, %%rax")
		cg.emit("    jl %s", errLabel)
		cg.emit("    cmp $%d, %%rax", arrLen)
		cg.emit("    jl %s", okLabel)
		cg.emit("%s:", errLabel)
		cg.emitRuntimeFail(ias, "array index out of bounds")
		cg.emit("%s:", okLabel)
	}
	cg.emit("    imul $8, %%rax, %%rax")
	emitLoadTarget("%rcx")
	cg.emit("    pop %%rdx")
	cg.emit("    mov %%rdx, (%%rcx,%%rax)")
	cg.emit("    mov %%rdx, %%rax")
}

// generateReturn handles return statements
func (cg *CodeGen) generateReturn(rs *ast.ReturnStatement) {
	if rs.ReturnValue == nil {
		cg.generateNull(&ast.NullLiteral{})
	} else {
		cg.generateExpression(rs.ReturnValue)
	}
	if cg.inFunction {
		addedTypeError := false
		if cg.funcRetType != typeUnknown && cg.funcRetType != typeAny {
			got := typeNull
			if rs.ReturnValue != nil {
				got = cg.inferExpressionType(rs.ReturnValue)
			}
			if got != typeUnknown && got != cg.funcRetType && cg.funcRetType != typeArray {
				cg.addNodeError(fmt.Sprintf("cannot return %s from function returning %s", typeName(got), typeName(cg.funcRetType)), rs)
				addedTypeError = true
			}
		}
		if cg.funcRetTypeName != "" && cg.isKnownTypeName(cg.funcRetTypeName) {
			gotName := "null"
			if rs.ReturnValue != nil {
				gotName = cg.inferExpressionTypeName(rs.ReturnValue)
				// Some complex expressions can transiently infer as null due to
				// conservative value tracking. Treat those as unknown instead of
				// hard-failing null-return validation.
				if gotName == "null" {
					switch rs.ReturnValue.(type) {
					case *ast.NullLiteral, *ast.Identifier, *ast.CallExpression:
						// keep null: identifiers/calls may legitimately resolve to null
					default:
						gotName = "unknown"
					}
				}
			}
			if gotName == "null" {
				if !cg.typeAllowsNullTypeName(cg.funcRetTypeName) {
					if !addedTypeError {
						cg.addNodeError(fmt.Sprintf("cannot return %s from function returning %s", gotName, cg.funcRetTypeName), rs)
					}
				}
			} else if gotName != "unknown" && !cg.isAssignableTypeName(cg.funcRetTypeName, gotName) {
				if !addedTypeError {
					cg.addNodeError(fmt.Sprintf("cannot return %s from function returning %s", gotName, cg.funcRetTypeName), rs)
				}
			}
		}
		cg.emit("    jmp %s", cg.funcRetLbl)
		return
	}
	cg.emit("    mov %%rax, %%rdi       # return value as process exit code")
	cg.emit("    jmp %s", cg.exitLabel)
}

func (cg *CodeGen) generateIfExpression(ie *ast.IfExpression) {
	elseLabel := cg.newLabel()
	endLabel := cg.newLabel()

	// Generate condition
	cg.generateExpression(ie.Condition)

	// Test if false (0)
	cg.emit("    test %%rax, %%rax")
	cg.emit("    jz %s              # jump if condition is false", elseLabel)

	// Generate consequence (if block)
	cg.generateBlockStatement(ie.Consequence)
	cg.emit("    jmp %s             # jump to end", endLabel)

	// Else block
	cg.emit("%s:", elseLabel)
	if ie.Alternative != nil {
		cg.generateBlockStatement(ie.Alternative)
	}

	// End
	cg.emit("%s:", endLabel)
}

func (cg *CodeGen) generateWhileStatement(ws *ast.WhileStatement) {
	startLabel := cg.newLabel()
	endLabel := cg.newLabel()
	cg.pushLoopLabels(endLabel, startLabel)
	defer cg.popLoopLabels()
	cg.emit("%s:", startLabel)
	cg.generateExpression(ws.Condition)
	cg.emit("    test %%rax, %%rax")
	cg.emit("    jz %s", endLabel)
	cg.generateBlockStatement(ws.Body)
	cg.emit("    jmp %s", startLabel)
	cg.emit("%s:", endLabel)
}

func (cg *CodeGen) generateLoopStatement(ls *ast.LoopStatement) {
	startLabel := cg.newLabel()
	cg.pushLoopLabels(cg.newLabel(), startLabel)
	breakLabel := cg.currentBreakLabel()
	defer cg.popLoopLabels()
	cg.emit("%s:", startLabel)
	cg.generateBlockStatement(ls.Body)
	cg.emit("    jmp %s", startLabel)
	cg.emit("%s:", breakLabel)
}

func (cg *CodeGen) generateForStatement(fs *ast.ForStatement) {
	scope := cg.enterScope()
	defer cg.exitScope(scope)

	if fs.Init != nil {
		cg.generateStatement(fs.Init)
	}
	startLabel := cg.newLabel()
	periodicLabel := cg.newLabel()
	endLabel := cg.newLabel()
	cg.pushLoopLabels(endLabel, periodicLabel)
	defer cg.popLoopLabels()
	cg.emit("%s:", startLabel)
	if fs.Condition != nil {
		cg.generateExpression(fs.Condition)
		cg.emit("    test %%rax, %%rax")
		cg.emit("    jz %s", endLabel)
	}
	cg.generateBlockStatement(fs.Body)
	cg.emit("%s:", periodicLabel)
	if fs.Periodic != nil {
		cg.generateStatement(fs.Periodic)
	}
	cg.emit("    jmp %s", startLabel)
	cg.emit("%s:", endLabel)
}

func (cg *CodeGen) generateForeachStatement(fs *ast.ForeachStatement) {
	if fs == nil || fs.Name == nil || fs.Iterable == nil {
		cg.addNodeError("invalid foreach statement", fs)
		cg.emit("    mov $0, %%rax")
		return
	}

	scope := cg.enterScope()
	defer cg.exitScope(scope)

	iterType := cg.inferExpressionTypeName(fs.Iterable)
	if resolved, ok := cg.normalizeTypeName(iterType); ok {
		iterType = resolved
	}
	if inner, ok := peelPointerType(iterType); ok {
		iterType = inner
	}

	elemTypeName := ""
	arrayLen := -1
	isList := false
	if elem, n, ok := peelArrayType(iterType); ok {
		elemTypeName = elem
		arrayLen = n
	} else if elem, ok := peelListType(iterType); ok {
		elemTypeName = elem
		isList = true
	} else {
		cg.addNodeError("foreach expects array or list iterable", fs.Iterable)
		cg.emit("    mov $0, %%rax")
		return
	}

	name := fs.Name.Value
	if cg.isDeclaredInCurrentScope(name) {
		cg.addNodeError("identifier already declared: "+name, fs)
		cg.emit("    mov $0, %%rax")
		return
	}

	iterOffset := cg.allocateSlots(1)
	idxOffset := cg.allocateSlots(1)
	valueOffset := cg.allocateSlots(1)

	cg.variables[name] = valueOffset
	cg.markDeclaredInCurrentScope(name)
	cg.varTypes[name] = cg.parseTypeName(elemTypeName)
	cg.varDeclared[name] = cg.varTypes[name]
	cg.varTypeNames[name] = elemTypeName
	cg.varDeclaredNames[name] = elemTypeName
	cg.varValueTypeName[name] = elemTypeName
	cg.varIsNull[name] = cg.typeAllowsNullTypeName(elemTypeName)
	delete(cg.varFuncs, name)
	cg.trackKnownValue(name, cg.varTypes[name], nil)

	cg.generateExpression(fs.Iterable)
	cg.emit("    mov %%rax, -%d(%%rbp)", iterOffset)
	cg.emit("    mov $0, %%rax")
	cg.emit("    mov %%rax, -%d(%%rbp)", idxOffset)

	startLabel := cg.newLabel()
	stepLabel := cg.newLabel()
	endLabel := cg.newLabel()
	cg.pushLoopLabels(endLabel, stepLabel)
	defer cg.popLoopLabels()

	cg.emit("%s:", startLabel)
	cg.emit("    mov -%d(%%rbp), %%rax", idxOffset)
	if isList {
		cg.emit("    mov -%d(%%rbp), %%rdi", iterOffset)
		okLabel := cg.newLabel()
		cg.emit("    lea null_lit(%%rip), %%rcx")
		cg.emit("    cmp %%rcx, %%rdi")
		cg.emit("    jne %s", okLabel)
		cg.emit("    lea runtime_err_null_deref(%%rip), %%rax")
		cg.emit("    call runtime_fail")
		cg.emit("%s:", okLabel)
		cg.emit("    mov (%%rdi), %%rcx")
		cg.emit("    cmp %%rcx, %%rax")
		cg.emit("    jge %s", endLabel)
		cg.emit("    mov %%rax, %%rsi")
		cg.emit("    call list_get")
	} else {
		cg.emit("    cmp $%d, %%rax", arrayLen)
		cg.emit("    jge %s", endLabel)
		cg.emit("    imul $8, %%rax, %%rax")
		cg.emit("    mov -%d(%%rbp), %%rcx", iterOffset)
		cg.emit("    mov (%%rcx,%%rax), %%rax")
	}
	cg.emit("    mov %%rax, -%d(%%rbp)", valueOffset)

	cg.generateBlockStatement(fs.Body)

	cg.emit("%s:", stepLabel)
	cg.emit("    mov -%d(%%rbp), %%rax", idxOffset)
	cg.emit("    add $1, %%rax")
	cg.emit("    mov %%rax, -%d(%%rbp)", idxOffset)
	cg.emit("    jmp %s", startLabel)
	cg.emit("%s:", endLabel)
}

func (cg *CodeGen) generateBreakStatement(bs *ast.BreakStatement) {
	label := cg.currentBreakLabel()
	if label == "" {
		cg.addNodeError("break not inside loop", bs)
		cg.emit("    mov $0, %%rax")
		return
	}
	cg.emit("    jmp %s", label)
}

func (cg *CodeGen) generateContinueStatement(cs *ast.ContinueStatement) {
	label := cg.currentContinueLabel()
	if label == "" {
		cg.addNodeError("continue not inside loop", cs)
		cg.emit("    mov $0, %%rax")
		return
	}
	cg.emit("    jmp %s", label)
}

func (cg *CodeGen) pushLoopLabels(breakLabel, contLabel string) {
	cg.loopBreakLabels = append(cg.loopBreakLabels, breakLabel)
	cg.loopContLabels = append(cg.loopContLabels, contLabel)
}

func (cg *CodeGen) popLoopLabels() {
	if len(cg.loopBreakLabels) > 0 {
		cg.loopBreakLabels = cg.loopBreakLabels[:len(cg.loopBreakLabels)-1]
	}
	if len(cg.loopContLabels) > 0 {
		cg.loopContLabels = cg.loopContLabels[:len(cg.loopContLabels)-1]
	}
}

func (cg *CodeGen) currentBreakLabel() string {
	if len(cg.loopBreakLabels) == 0 {
		return ""
	}
	return cg.loopBreakLabels[len(cg.loopBreakLabels)-1]
}

func (cg *CodeGen) currentContinueLabel() string {
	if len(cg.loopContLabels) == 0 {
		return ""
	}
	return cg.loopContLabels[len(cg.loopContLabels)-1]
}

func (cg *CodeGen) generateImportedBuiltinCall(target string, args []ast.Expression, node ast.Node) {
	switch target {
	case "twice.math.abs":
		if len(args) != 1 {
			cg.failNodef(node, "abs expects 1 argument, got=%d", len(args))
			return
		}
		argType := cg.inferExpressionType(args[0])
		if argType != typeInt && argType != typeFloat {
			cg.failNodef(args[0], "abs expects int or float argument")
			return
		}
		if argType == typeFloat {
			v, ok := cg.constFloatValue(args[0])
			if !ok {
				cg.failNodef(args[0], "float abs in codegen requires compile-time known numeric value")
				return
			}
			cg.emitConstFloatResult(math.Abs(v))
			return
		}
		cg.generateExpression(args[0])
		done := cg.newLabel()
		cg.emit("    cmp $0, %%rax")
		cg.emit("    jge %s", done)
		cg.emit("    neg %%rax")
		cg.emit("%s:", done)
	case "twice.math.min", "twice.math.max":
		if len(args) != 2 {
			cg.failNodef(node, "%s expects 2 arguments, got=%d", mangleImportedName(target), len(args))
			return
		}
		leftType := cg.inferExpressionType(args[0])
		rightType := cg.inferExpressionType(args[1])
		if !isNumericType(leftType) || !isNumericType(rightType) {
			cg.failNodef(node, "%s expects int/float arguments", mangleImportedName(target))
			return
		}
		if leftType == typeFloat || rightType == typeFloat {
			left, lok := cg.constFloatValue(args[0])
			right, rok := cg.constFloatValue(args[1])
			if !lok || !rok {
				cg.failNodef(node, "float %s in codegen requires compile-time known numeric values", mangleImportedName(target))
				return
			}
			if target == "twice.math.min" {
				cg.emitConstFloatResult(math.Min(left, right))
			} else {
				cg.emitConstFloatResult(math.Max(left, right))
			}
			return
		}
		cg.generateExpression(args[0])
		cg.emit("    push %%rax")
		cg.generateExpression(args[1])
		cg.emit("    mov %%rax, %%rcx")
		cg.emit("    pop %%rax")
		done := cg.newLabel()
		if target == "twice.math.min" {
			cg.emit("    cmp %%rcx, %%rax")
			cg.emit("    jle %s", done)
			cg.emit("    mov %%rcx, %%rax")
		} else {
			cg.emit("    cmp %%rcx, %%rax")
			cg.emit("    jge %s", done)
			cg.emit("    mov %%rcx, %%rax")
		}
		cg.emit("%s:", done)
	case "twice.math.sqrt":
		if len(args) != 1 {
			cg.failNodef(node, "sqrt expects 1 argument, got=%d", len(args))
			return
		}
		argType := cg.inferExpressionType(args[0])
		if argType != typeInt && argType != typeFloat {
			cg.failNodef(args[0], "sqrt expects int or float argument")
			return
		}
		if argType == typeFloat {
			v, ok := cg.constFloatValue(args[0])
			if !ok {
				cg.failNodef(args[0], "float sqrt in codegen requires compile-time known numeric value")
				return
			}
			if v < 0 {
				cg.failNodef(args[0], "sqrt expects non-negative number")
				return
			}
			cg.emitConstFloatResult(math.Sqrt(v))
			return
		}
		cg.generateExpression(args[0])
		nonNeg := cg.newLabel()
		done := cg.newLabel()
		loop := cg.newLabel()
		cg.emit("    cmp $0, %%rax")
		cg.emit("    jge %s", nonNeg)
		cg.emit("    mov $0, %%rax")
		cg.emit("    jmp %s", done)
		cg.emit("%s:", nonNeg)
		cg.emit("    mov %%rax, %%r8")
		cg.emit("    xor %%rax, %%rax")
		cg.emit("%s:", loop)
		cg.emit("    mov %%rax, %%rcx")
		cg.emit("    inc %%rcx")
		cg.emit("    mov %%rcx, %%rdx")
		cg.emit("    imul %%rdx, %%rdx")
		cg.emit("    cmp %%r8, %%rdx")
		cg.emit("    jg %s", done)
		cg.emit("    mov %%rcx, %%rax")
		cg.emit("    jmp %s", loop)
		cg.emit("%s:", done)
	default:
		cg.failNodef(node, "unsupported imported function: %s", target)
	}
}

func (cg *CodeGen) emitConstFloatResult(v float64) {
	label := cg.stringLabel(fmt.Sprintf("%g", v))
	cg.emit("    lea %s(%%rip), %%rax", label)
}

func mangleImportedName(target string) string {
	if strings.HasPrefix(target, "twice.math.") {
		return strings.TrimPrefix(target, "twice.math.")
	}
	return target
}

func (cg *CodeGen) generateCallExpression(ce *ast.CallExpression) {
	if fl, ok := ce.Function.(*ast.FunctionLiteral); ok {
		key, ok := cg.funcLitKeys[fl]
		if !ok {
			cg.failNode("function literal not registered for call", ce)
			return
		}
		fn, ok := cg.functions[key]
		if !ok || fn == nil {
			cg.failNode("function literal call target missing", ce)
			return
		}
		cg.generateUserFunctionCall(fn, ce)
		return
	}
	fn, ok := ce.Function.(*ast.Identifier)
	if !ok {
		cg.failNode("unsupported call target", ce)
		return
	}
	if target, ok := cg.importMembers[fn.Value]; ok {
		if len(ce.TypeArguments) > 0 {
			cg.failNode("generic type arguments are not supported for imported builtin functions", ce)
			return
		}
		for _, arg := range ce.Arguments {
			if _, ok := arg.(*ast.NamedArgument); ok {
				cg.failNode("named arguments are not supported for imported builtin functions", arg)
				return
			}
		}
		if builtinTarget, ok := imports.BuiltinMemberTarget(target); ok {
			cg.generateImportedBuiltinCall(builtinTarget, ce.Arguments, ce)
			return
		}
		localName := target
		if dot := strings.LastIndex(target, "."); dot >= 0 {
			localName = target[dot+1:]
		}
		if key, ok := cg.varFuncs[localName]; ok {
			cg.generateUserFunctionCall(cg.functions[key], ce)
			return
		}
		if key, ok := cg.funcByName[localName]; ok {
			cg.generateUserFunctionCall(cg.functions[key], ce)
			return
		}
		cg.failNode("unknown imported function "+target, ce)
		return
	}

	switch fn.Value {
	case "print", "println":
		if len(ce.TypeArguments) > 0 {
			cg.failNodef(ce, "generic type arguments are not supported for %s", fn.Value)
			return
		}
		if len(ce.Arguments) != 1 {
			cg.failNodef(ce, "%s expects exactly 1 argument", fn.Value)
			return
		}
		if _, ok := ce.Arguments[0].(*ast.NamedArgument); ok {
			cg.failNodef(ce.Arguments[0], "named arguments are not supported for %s", fn.Value)
			return
		}
		argType := cg.inferExpressionType(ce.Arguments[0])
		prevErrCount := len(cg.errors)
		cg.generateExpression(ce.Arguments[0])
		if len(cg.errors) > prevErrCount {
			cg.emit("    mov $0, %%rax")
			return
		}
		switch argType {
		case typeBool:
			if fn.Value == "println" {
				cg.emit("    call println_bool")
			} else {
				cg.emit("    call print_bool")
			}
		case typeInt:
			if fn.Value == "println" {
				cg.emit("    call println_int")
			} else {
				cg.emit("    call print_int")
			}
		case typeChar:
			if fn.Value == "println" {
				cg.emit("    call println_char")
			} else {
				cg.emit("    call print_char")
			}
		case typeString, typeFloat, typeType, typeNull:
			if fn.Value == "println" {
				cg.emit("    call println_cstr")
			} else {
				cg.emit("    call print_cstr")
			}
		default:
			cg.failNodef(ce.Arguments[0], "%s supports only int, bool, float, string, char, null, and type arguments", fn.Value)
		}
	case "typeof":
		if len(ce.TypeArguments) > 0 {
			cg.failNode("generic type arguments are not supported for typeof", ce)
			return
		}
		if len(ce.Arguments) != 1 {
			cg.failNode("typeof expects exactly 1 argument", ce)
			return
		}
		if _, ok := ce.Arguments[0].(*ast.NamedArgument); ok {
			cg.failNode("named arguments are not supported for typeof", ce.Arguments[0])
			return
		}
		typeNameStr := cg.inferExpressionTypeName(ce.Arguments[0])
		if id, ok := ce.Arguments[0].(*ast.Identifier); ok {
			if declared, ok := cg.varDeclaredNames[id.Value]; ok && declared != "" {
				typeNameStr = declared
			}
		}
		label := cg.stringLabel(typeNameStr)
		cg.emit("    lea %s(%%rip), %%rax", label)
	case "typeofValue", "typeofvalue":
		if len(ce.TypeArguments) > 0 {
			cg.failNodef(ce, "generic type arguments are not supported for %s", fn.Value)
			return
		}
		if len(ce.Arguments) != 1 {
			cg.failNodef(ce, "%s expects exactly 1 argument", fn.Value)
			return
		}
		if _, ok := ce.Arguments[0].(*ast.NamedArgument); ok {
			cg.failNodef(ce.Arguments[0], "named arguments are not supported for %s", fn.Value)
			return
		}
		typeNameStr := typeName(cg.inferExpressionType(ce.Arguments[0]))
		if typeNameStr == "array" {
			typeNameStr = cg.inferExpressionTypeName(ce.Arguments[0])
		}
		label := cg.stringLabel(typeNameStr)
		cg.emit("    lea %s(%%rip), %%rax", label)
	case "hasField":
		if len(ce.TypeArguments) > 0 {
			cg.failNode("generic type arguments are not supported for hasField", ce)
			return
		}
		if len(ce.Arguments) != 2 {
			cg.failNodef(ce, "hasField expects exactly 2 arguments, got=%d", len(ce.Arguments))
			return
		}
		for _, arg := range ce.Arguments {
			if _, ok := arg.(*ast.NamedArgument); ok {
				cg.failNode("named arguments are not supported for hasField", arg)
				return
			}
		}
		// Evaluate object first to preserve argument side effects.
		cg.generateExpression(ce.Arguments[0])
		cg.emit("    push %%rax")

		fieldType := cg.inferExpressionType(ce.Arguments[1])
		cg.generateExpression(ce.Arguments[1])
		if fieldType != typeString {
			cg.emit("    add $8, %%rsp")
			cg.failNodef(ce.Arguments[1], "hasField field must be string, got %s", typeName(fieldType))
			return
		}

		objTypeName := cg.inferExpressionTypeName(ce.Arguments[0])
		if supportsStruct, nullableStruct := cg.typeSupportsStructFieldLookup(objTypeName); supportsStruct {
			cg.emit("    mov %%rax, %%rsi")
			cg.emit("    pop %%rdi")
			normalizedObj := objTypeName
			if resolved, ok := cg.normalizeTypeName(objTypeName); ok {
				normalizedObj = resolved
			}
			if _, ok := peelPointerType(normalizedObj); ok {
				okLabel := cg.newLabel()
				cg.emit("    lea null_lit(%%rip), %%rdx")
				cg.emit("    cmp %%rdx, %%rdi")
				cg.emit("    jne %s", okLabel)
				cg.emit("    lea runtime_err_null_deref(%%rip), %%rax")
				cg.emit("    call runtime_fail")
				cg.emit("%s:", okLabel)
				cg.emit("    mov (%%rdi), %%rdi")
			}
			if nullableStruct {
				nonNullLabel := cg.newLabel()
				doneLabel := cg.newLabel()
				cg.emit("    lea null_lit(%%rip), %%rdx")
				cg.emit("    cmp %%rdx, %%rdi")
				cg.emit("    jne %s", nonNullLabel)
				cg.emit("    mov $0, %%rax")
				cg.emit("    jmp %s", doneLabel)
				cg.emit("%s:", nonNullLabel)
				cg.emit("    call map_has")
				cg.emit("%s:", doneLabel)
				return
			}
			cg.emit("    call map_has")
			return
		}
		supportsLength, nullable := cg.typeSupportsLengthField(objTypeName)
		if !supportsLength {
			cg.emit("    add $8, %%rsp")
			cg.emit("    mov $0, %%rax")
			return
		}

		cg.emit("    mov %%rax, %%rdi")
		cg.emit("    lea hasfield_length(%%rip), %%rsi")
		cg.emit("    call cstr_eq")

		if nullable {
			nonNullLabel := cg.newLabel()
			doneLabel := cg.newLabel()
			cg.emit("    pop %%rcx")
			cg.emit("    lea null_lit(%%rip), %%rdx")
			cg.emit("    cmp %%rdx, %%rcx")
			cg.emit("    jne %s", nonNullLabel)
			cg.emit("    mov $0, %%rax")
			cg.emit("    jmp %s", doneLabel)
			cg.emit("%s:", nonNullLabel)
			cg.emit("%s:", doneLabel)
			return
		}

		cg.emit("    add $8, %%rsp")
	case "int", "float", "string", "char", "bool":
		if len(ce.TypeArguments) > 0 {
			cg.failNodef(ce, "generic type arguments are not supported for %s", fn.Value)
			return
		}
		cg.generateCastCall(fn.Value, ce)
	default:
		if key, ok := cg.varFuncs[fn.Value]; ok {
			cg.generateUserFunctionCall(cg.functions[key], ce)
			return
		}
		if key, ok := cg.funcByName[fn.Value]; ok {
			cg.generateUserFunctionCall(cg.functions[key], ce)
			return
		}
		cg.failNode("unknown function "+fn.Value, ce)
	}
}

func (cg *CodeGen) generateBlockStatement(block *ast.BlockStatement) {
	if block == nil {
		return
	}
	scope := cg.enterScope()
	defer cg.exitScope(scope)

	for _, stmt := range block.Statements {
		cg.generateStatement(stmt)
	}
}
