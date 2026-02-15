package codegen

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"twice/internal/ast"
)

func (cg *CodeGen) reset() {
	cg.output = strings.Builder{}
	cg.funcDefs = strings.Builder{}
	cg.labelCount = 0
	cg.exitLabel = ""
	cg.normalExit = ""
	cg.variables = make(map[string]int)
	cg.constVars = make(map[string]bool)
	cg.varTypes = make(map[string]valueType)
	cg.varDeclared = make(map[string]valueType)
	cg.varTypeNames = make(map[string]string)
	cg.varDeclaredNames = make(map[string]string)
	cg.varValueTypeName = make(map[string]string)
	cg.varIsNull = make(map[string]bool)
	cg.varArrayLen = make(map[string]int)
	cg.intVals = make(map[string]int64)
	cg.charVals = make(map[string]rune)
	cg.stringVals = make(map[string]string)
	cg.floatVals = make(map[string]float64)
	cg.stringLits = make(map[string]string)
	cg.functions = make(map[string]*compiledFunction)
	cg.funcByName = make(map[string]string)
	cg.funcStmtKeys = make(map[*ast.FunctionStatement]string)
	cg.funcLitKeys = make(map[*ast.FunctionLiteral]string)
	cg.varFuncs = make(map[string]string)
	cg.typeAliases = make(map[string]string)
	cg.genericTypeAliases = make(map[string]genericTypeAlias)
	cg.stackOffset = 0
	cg.maxStackOffset = 0
	cg.currentFn = ""
	cg.nextAnonFn = 0
	cg.inFunction = false
	cg.funcRetLbl = ""
	cg.funcRetType = typeUnknown
	cg.funcRetTypeName = ""
	cg.loopBreakLabels = nil
	cg.loopContLabels = nil
	cg.arraySlots = make(map[*ast.ArrayLiteral]int)
	cg.tupleSlots = make(map[*ast.TupleLiteral]int)
	cg.scopeDecls = []map[string]struct{}{make(map[string]struct{})}
	cg.typeScopeDecls = []map[string]struct{}{make(map[string]struct{})}
	cg.inferTypeCache = make(map[ast.Expression]valueType)
	cg.inferNameCache = make(map[ast.Expression]string)
	cg.errors = []CodegenError{}
}

func (cg *CodeGen) collectFunctions(program *ast.Program) {
	globalScope := map[string]struct{}{}
	for _, stmt := range program.Statements {
		switch s := stmt.(type) {
		case *ast.FunctionStatement:
			if s != nil && s.Name != nil {
				globalScope[s.Name.Value] = struct{}{}
			}
		case *ast.LetStatement:
			if s != nil && s.Name != nil {
				globalScope[s.Name.Value] = struct{}{}
			}
		case *ast.ConstStatement:
			if s != nil && s.Name != nil {
				globalScope[s.Name.Value] = struct{}{}
			}
		}
	}
	for _, stmt := range program.Statements {
		cg.collectFunctionsInStatement(stmt, globalScope, true)
	}
}

func (cg *CodeGen) collectFunctionsInStatement(stmt ast.Statement, scope map[string]struct{}, topLevel bool) {
	switch s := stmt.(type) {
	case *ast.FunctionStatement:
		if s == nil || s.Name == nil || s.Function == nil {
			return
		}
		key := s.Name.Value
		if _, exists := cg.functions[key]; exists {
			cg.addNodeError("duplicate function declaration: "+key, s)
			return
		}
		captures := []string{}
		if !topLevel {
			captures = cg.computeCaptures(s.Function, scope)
		}
		cg.functions[key] = &compiledFunction{
			Key:              key,
			Name:             s.Name.Value,
			Label:            "fn_" + s.Name.Value,
			Literal:          s.Function,
			Captures:         captures,
			CaptureTypeNames: make([]string, len(captures)),
		}
		cg.funcStmtKeys[s] = key
		if topLevel {
			cg.funcByName[s.Name.Value] = key
		}

		childScope := copyScope(scope)
		for _, p := range s.Function.Parameters {
			childScope[p.Name.Value] = struct{}{}
		}
		collectDeclaredNames(s.Function.Body, childScope)
		for _, st := range s.Function.Body.Statements {
			cg.collectFunctionsInStatement(st, childScope, false)
		}
	case *ast.BlockStatement:
		childScope := copyScope(scope)
		collectDeclaredNames(s, childScope)
		for _, st := range s.Statements {
			cg.collectFunctionsInStatement(st, childScope, false)
		}
	case *ast.LetStatement:
		if s != nil && s.Value != nil {
			cg.collectFunctionsInExpression(s.Value, scope)
		}
	case *ast.ConstStatement:
		if s != nil && s.Value != nil {
			cg.collectFunctionsInExpression(s.Value, scope)
		}
	case *ast.TypeDeclStatement:
		return
	case *ast.AssignStatement:
		if s != nil && s.Value != nil {
			cg.collectFunctionsInExpression(s.Value, scope)
		}
	case *ast.IndexAssignStatement:
		if s != nil && s.Left != nil {
			cg.collectFunctionsInExpression(s.Left.Left, scope)
			cg.collectFunctionsInExpression(s.Left.Index, scope)
		}
		if s != nil && s.Value != nil {
			cg.collectFunctionsInExpression(s.Value, scope)
		}
	case *ast.WhileStatement:
		if s != nil && s.Condition != nil {
			cg.collectFunctionsInExpression(s.Condition, scope)
		}
		if s != nil && s.Body != nil {
			childScope := copyScope(scope)
			collectDeclaredNames(s.Body, childScope)
			for _, st := range s.Body.Statements {
				cg.collectFunctionsInStatement(st, childScope, false)
			}
		}
	case *ast.LoopStatement:
		if s != nil && s.Body != nil {
			childScope := copyScope(scope)
			collectDeclaredNames(s.Body, childScope)
			for _, st := range s.Body.Statements {
				cg.collectFunctionsInStatement(st, childScope, false)
			}
		}
	case *ast.ForStatement:
		if s != nil && s.Init != nil {
			cg.collectFunctionsInStatement(s.Init, scope, false)
		}
		if s != nil && s.Condition != nil {
			cg.collectFunctionsInExpression(s.Condition, scope)
		}
		if s != nil && s.Periodic != nil {
			cg.collectFunctionsInStatement(s.Periodic, scope, false)
		}
		if s != nil && s.Body != nil {
			childScope := copyScope(scope)
			collectDeclaredNames(s.Body, childScope)
			for _, st := range s.Body.Statements {
				cg.collectFunctionsInStatement(st, childScope, false)
			}
		}
	case *ast.ReturnStatement:
		if s != nil && s.ReturnValue != nil {
			cg.collectFunctionsInExpression(s.ReturnValue, scope)
		}
	case *ast.ExpressionStatement:
		if s != nil && s.Expression != nil {
			cg.collectFunctionsInExpression(s.Expression, scope)
		}
	}
}

func (cg *CodeGen) collectFunctionsInExpression(expr ast.Expression, scope map[string]struct{}) {
	switch e := expr.(type) {
	case *ast.FunctionLiteral:
		key := fmt.Sprintf("lit_%d", cg.nextAnonFn)
		cg.nextAnonFn++
		captures := cg.computeCaptures(e, scope)
		cg.functions[key] = &compiledFunction{
			Key:              key,
			Label:            "fn_" + key,
			Literal:          e,
			Captures:         captures,
			CaptureTypeNames: make([]string, len(captures)),
		}
		cg.funcLitKeys[e] = key
		childScope := copyScope(scope)
		for _, p := range e.Parameters {
			childScope[p.Name.Value] = struct{}{}
		}
		collectDeclaredNames(e.Body, childScope)
		for _, st := range e.Body.Statements {
			cg.collectFunctionsInStatement(st, childScope, false)
		}
	case *ast.CallExpression:
		cg.collectFunctionsInExpression(e.Function, scope)
		for _, arg := range e.Arguments {
			cg.collectFunctionsInExpression(arg, scope)
		}
	case *ast.ArrayLiteral:
		for _, el := range e.Elements {
			cg.collectFunctionsInExpression(el, scope)
		}
	case *ast.TupleLiteral:
		for _, el := range e.Elements {
			cg.collectFunctionsInExpression(el, scope)
		}
	case *ast.IndexExpression:
		cg.collectFunctionsInExpression(e.Left, scope)
		cg.collectFunctionsInExpression(e.Index, scope)
	case *ast.TupleAccessExpression:
		cg.collectFunctionsInExpression(e.Left, scope)
	case *ast.MethodCallExpression:
		cg.collectFunctionsInExpression(e.Object, scope)
		for _, arg := range e.Arguments {
			cg.collectFunctionsInExpression(arg, scope)
		}
	case *ast.MemberAccessExpression:
		cg.collectFunctionsInExpression(e.Object, scope)
	case *ast.NullSafeAccessExpression:
		cg.collectFunctionsInExpression(e.Object, scope)
	case *ast.InfixExpression:
		cg.collectFunctionsInExpression(e.Left, scope)
		cg.collectFunctionsInExpression(e.Right, scope)
	case *ast.PrefixExpression:
		cg.collectFunctionsInExpression(e.Right, scope)
	case *ast.IfExpression:
		cg.collectFunctionsInExpression(e.Condition, scope)
		if e.Consequence != nil {
			consScope := copyScope(scope)
			collectDeclaredNames(e.Consequence, consScope)
			for _, st := range e.Consequence.Statements {
				cg.collectFunctionsInStatement(st, consScope, false)
			}
		}
		if e.Alternative != nil {
			altScope := copyScope(scope)
			collectDeclaredNames(e.Alternative, altScope)
			for _, st := range e.Alternative.Statements {
				cg.collectFunctionsInStatement(st, altScope, false)
			}
		}
	case *ast.NamedArgument:
		cg.collectFunctionsInExpression(e.Value, scope)
	}
}

func (cg *CodeGen) generateFunctionDefinitions() {
	generated := make(map[string]struct{}, len(cg.functions))
	for {
		named := make([]string, 0, len(cg.functions))
		literals := make([]string, 0, len(cg.functions))
		for k := range cg.functions {
			if _, done := generated[k]; done {
				continue
			}
			fn := cg.functions[k]
			if fn == nil || fn.Literal == nil {
				continue
			}
			if len(fn.Literal.TypeParams) > 0 && !strings.Contains(k, "<") {
				// Generic base functions are templates; emit only concrete specializations.
				generated[k] = struct{}{}
				continue
			}
			if strings.HasPrefix(k, "lit_") {
				literals = append(literals, k)
			} else {
				named = append(named, k)
			}
		}
		if len(named) == 0 && len(literals) == 0 {
			return
		}
		sort.Strings(named)
		sort.Slice(literals, func(i, j int) bool {
			return literalFnIndex(literals[i]) < literalFnIndex(literals[j])
		})
		for _, key := range named {
			generated[key] = struct{}{}
			cg.generateOneFunction(cg.functions[key])
		}
		for _, key := range literals {
			generated[key] = struct{}{}
			cg.generateOneFunction(cg.functions[key])
		}
	}
}

func literalFnIndex(key string) int {
	if !strings.HasPrefix(key, "lit_") {
		return -1
	}
	n, err := strconv.Atoi(strings.TrimPrefix(key, "lit_"))
	if err != nil {
		return -1
	}
	return n
}

type cgState struct {
	lexical         lexicalScopeState
	stackOffset     int
	maxStackOffset  int
	inFunction      bool
	funcRetLbl      string
	funcRetType     valueType
	funcRetTypeName string
	currentFn       string
	loopBreakLabels []string
	loopContLabels  []string
	arraySlots      map[*ast.ArrayLiteral]int
	tupleSlots      map[*ast.TupleLiteral]int
	scopeDecls      []map[string]struct{}
	typeScopeDecls  []map[string]struct{}
}

func (cg *CodeGen) saveState() cgState {
	return cgState{
		lexical:         cg.snapshotLexicalState(),
		stackOffset:     cg.stackOffset,
		maxStackOffset:  cg.maxStackOffset,
		inFunction:      cg.inFunction,
		funcRetLbl:      cg.funcRetLbl,
		funcRetType:     cg.funcRetType,
		funcRetTypeName: cg.funcRetTypeName,
		currentFn:       cg.currentFn,
		loopBreakLabels: cg.loopBreakLabels,
		loopContLabels:  cg.loopContLabels,
		arraySlots:      cg.arraySlots,
		tupleSlots:      cg.tupleSlots,
		scopeDecls:      cg.scopeDecls,
		typeScopeDecls:  cg.typeScopeDecls,
	}
}

func (cg *CodeGen) restoreState(st cgState) {
	cg.restoreLexicalState(st.lexical)
	cg.stackOffset = st.stackOffset
	cg.maxStackOffset = st.maxStackOffset
	cg.inFunction = st.inFunction
	cg.funcRetLbl = st.funcRetLbl
	cg.funcRetType = st.funcRetType
	cg.funcRetTypeName = st.funcRetTypeName
	cg.currentFn = st.currentFn
	cg.loopBreakLabels = st.loopBreakLabels
	cg.loopContLabels = st.loopContLabels
	cg.arraySlots = st.arraySlots
	cg.tupleSlots = st.tupleSlots
	cg.scopeDecls = st.scopeDecls
	cg.typeScopeDecls = st.typeScopeDecls
}

func (cg *CodeGen) generateOneFunction(fn *compiledFunction) {
	prevOutput := cg.output
	cg.output = strings.Builder{}
	state := cg.saveState()

	cg.variables = make(map[string]int)
	cg.constVars = make(map[string]bool)
	cg.varTypes = make(map[string]valueType)
	cg.varDeclared = make(map[string]valueType)
	cg.varTypeNames = make(map[string]string)
	cg.varDeclaredNames = make(map[string]string)
	cg.varValueTypeName = make(map[string]string)
	cg.varIsNull = make(map[string]bool)
	cg.varArrayLen = make(map[string]int)
	cg.intVals = make(map[string]int64)
	cg.charVals = make(map[string]rune)
	cg.stringVals = make(map[string]string)
	cg.floatVals = make(map[string]float64)
	cg.varFuncs = make(map[string]string)
	cg.stackOffset = 0
	cg.maxStackOffset = 0
	cg.inFunction = true
	cg.funcRetLbl = cg.newLabel()
	retTypeName := fn.Literal.ReturnType
	if len(fn.TypeArgMap) > 0 && retTypeName != "" {
		retTypeName = substituteTypeParams(retTypeName, fn.TypeArgMap)
	}
	cg.funcRetType = cg.parseTypeName(retTypeName)
	cg.funcRetTypeName = retTypeName
	cg.currentFn = fn.Key
	cg.arraySlots = make(map[*ast.ArrayLiteral]int)
	cg.tupleSlots = make(map[*ast.TupleLiteral]int)
	cg.scopeDecls = []map[string]struct{}{make(map[string]struct{})}
	cg.typeScopeDecls = []map[string]struct{}{make(map[string]struct{})}

	label := fn.Label
	cg.emit("%s:", label)
	cg.emit("    push %%rbp")
	cg.emit("    mov %%rsp, %%rbp")
	cg.emit("__STACK_ALLOC_FUNC__")

	for idx, p := range fn.Literal.Parameters {
		offset := cg.allocateSlots(1)
		cg.variables[p.Name.Value] = offset
		cg.markDeclaredInCurrentScope(p.Name.Value)
		cg.emit("    mov %d(%%rbp), %%rax  # param %s", 16+idx*8, p.Name.Value)
		cg.emit("    mov %%rax, -%d(%%rbp)", offset)
		paramTypeName := p.TypeName
		if len(fn.TypeArgMap) > 0 && paramTypeName != "" {
			paramTypeName = substituteTypeParams(paramTypeName, fn.TypeArgMap)
		}
		pt := cg.parseTypeName(paramTypeName)
		cg.varTypes[p.Name.Value] = pt
		cg.varDeclared[p.Name.Value] = pt
		if paramTypeName != "" {
			cg.varTypeNames[p.Name.Value] = paramTypeName
			cg.varDeclaredNames[p.Name.Value] = paramTypeName
			cg.varValueTypeName[p.Name.Value] = paramTypeName
			if _, n, ok := peelArrayType(paramTypeName); ok {
				cg.varArrayLen[p.Name.Value] = n
			}
		}
		cg.varIsNull[p.Name.Value] = false
	}

	for idx, name := range fn.Captures {
		argIdx := len(fn.Literal.Parameters) + idx
		offset := cg.allocateSlots(1)
		cg.variables[name] = offset
		cg.markDeclaredInCurrentScope(name)
		cg.emit("    mov %d(%%rbp), %%rax  # capture %s", 16+argIdx*8, name)
		cg.emit("    mov %%rax, -%d(%%rbp)", offset)
		cg.varTypeNames[name] = "unknown"
		if idx < len(fn.CaptureTypeNames) && fn.CaptureTypeNames[idx] != "" {
			cg.varTypeNames[name] = fn.CaptureTypeNames[idx]
		} else if t, ok := state.lexical.varTypeNames[name]; ok && t != "" {
			cg.varTypeNames[name] = t
		}
		cg.varTypes[name] = cg.parseTypeName(cg.varTypeNames[name])
		cg.varDeclared[name] = cg.varTypes[name]
		cg.varDeclaredNames[name] = cg.varTypeNames[name]
		if t, ok := state.lexical.varDeclaredNames[name]; ok && t != "" {
			cg.varDeclaredNames[name] = t
		}
		cg.varValueTypeName[name] = cg.varTypeNames[name]
		if t, ok := state.lexical.varValueTypeName[name]; ok && t != "" {
			cg.varValueTypeName[name] = t
		}
		cg.varIsNull[name] = false
		if n, ok := state.lexical.varIsNull[name]; ok {
			cg.varIsNull[name] = n
		}
		if n, ok := state.lexical.varArrayLen[name]; ok {
			cg.varArrayLen[name] = n
		}
		if v, ok := state.lexical.intVals[name]; ok {
			cg.intVals[name] = v
		}
		if v, ok := state.lexical.floatVals[name]; ok {
			cg.floatVals[name] = v
		}
		if v, ok := state.lexical.charVals[name]; ok {
			cg.charVals[name] = v
		}
		if v, ok := state.lexical.stringVals[name]; ok {
			cg.stringVals[name] = v
		}
	}

	for _, st := range fn.Literal.Body.Statements {
		if fs, ok := st.(*ast.FunctionStatement); ok && fs != nil && fs.Name != nil {
			if key, ok := cg.funcStmtKeys[fs]; ok {
				cg.varFuncs[fs.Name.Value] = key
			}
		}
	}

	cg.generateBlockStatement(fn.Literal.Body)
	cg.emit("    mov $0, %%rax")
	cg.emit("%s:", cg.funcRetLbl)
	cg.emit("    mov %%rbp, %%rsp")
	cg.emit("    pop %%rbp")
	cg.emit("    ret")
	fnAsm := cg.output.String()
	fnAsm = strings.ReplaceAll(fnAsm, "__STACK_ALLOC_FUNC__", cg.stackAllocLine())
	cg.funcDefs.WriteString(fnAsm)
	cg.output = prevOutput
	cg.restoreState(state)
}

func (cg *CodeGen) generateUserFunctionCall(fn *compiledFunction, ce *ast.CallExpression) {
	typeArgMap := map[string]string{}
	if len(fn.Literal.TypeParams) > 0 {
		if len(ce.TypeArguments) > 0 {
			if len(ce.TypeArguments) != len(fn.Literal.TypeParams) {
				cg.addNodeError(fmt.Sprintf("wrong number of generic type arguments: expected %d, got %d", len(fn.Literal.TypeParams), len(ce.TypeArguments)), ce)
				cg.emit("    mov $0, %%rax")
				return
			}
			for i, tp := range fn.Literal.TypeParams {
				ta := ce.TypeArguments[i]
				if !cg.isKnownTypeName(ta) {
					cg.addNodeError("unknown type: "+ta, ce)
					cg.emit("    mov $0, %%rax")
					return
				}
				typeArgMap[tp] = cg.canonicalGenericTypeArg(ta)
			}
		} else {
			for _, tp := range fn.Literal.TypeParams {
				typeArgMap[tp] = ""
			}
		}
	} else if len(ce.TypeArguments) > 0 {
		cg.addNodeError("generic type arguments provided for non-generic function", ce)
		cg.emit("    mov $0, %%rax")
		return
	}

	finalArgs := make([]ast.Expression, len(fn.Literal.Parameters))
	namedMode := false
	posIdx := 0
	for _, arg := range ce.Arguments {
		if na, ok := arg.(*ast.NamedArgument); ok {
			namedMode = true
			found := -1
			for i, p := range fn.Literal.Parameters {
				if p.Name.Value == na.Name {
					found = i
					break
				}
			}
			if found == -1 {
				cg.addNodeError("unknown named argument: "+na.Name, na)
				cg.emit("    mov $0, %%rax")
				return
			}
			if finalArgs[found] != nil {
				cg.addNodeError("argument provided twice: "+na.Name, na)
				cg.emit("    mov $0, %%rax")
				return
			}
			finalArgs[found] = na.Value
			continue
		}

		if namedMode {
			cg.addNodeError("positional arguments cannot appear after named arguments", ce)
			cg.emit("    mov $0, %%rax")
			return
		}
		if posIdx >= len(fn.Literal.Parameters) {
			cg.addNodeError("too many positional arguments", ce)
			cg.emit("    mov $0, %%rax")
			return
		}
		finalArgs[posIdx] = arg
		posIdx++
	}

	for i, p := range fn.Literal.Parameters {
		if finalArgs[i] != nil {
			continue
		}
		if p.DefaultValue == nil {
			cg.addNodeError("missing required argument: "+p.Name.Value, ce)
			cg.emit("    mov $0, %%rax")
			return
		}
		finalArgs[i] = p.DefaultValue
	}

	passedArgs := make([]ast.Expression, 0, len(finalArgs)+len(fn.Captures))
	for i, arg := range finalArgs {
		wantName := fn.Literal.Parameters[i].TypeName
		if len(typeArgMap) > 0 && wantName != "" {
			gotName := cg.inferExpressionTypeName(arg)
			if gotName != "unknown" {
				if errMsg, ok := cg.inferGenericTypeArgsFromTypes(wantName, gotName, typeArgMap); !ok {
					cg.addNodeError(errMsg, ce)
					cg.emit("    mov $0, %%rax")
					return
				}
			}
			wantName = substituteTypeParams(wantName, typeArgMap)
		}
		want := cg.parseTypeName(wantName)
		got := cg.inferExpressionType(arg)
		if wantName == "" {
			wantName = typeName(want)
		}
		gotName := cg.inferExpressionTypeName(arg)
		if wantName != "unknown" && gotName != "unknown" && !cg.isAssignableTypeName(wantName, gotName) {
			cg.addNodeError(fmt.Sprintf("cannot assign %s to %s", gotName, wantName), ce)
			cg.emit("    mov $0, %%rax")
			return
		}
		if want != typeUnknown && want != typeAny && got != typeUnknown && got != typeNull && got != want && cg.parseTypeName(wantName) != typeArray {
			cg.addNodeError(fmt.Sprintf("cannot assign %s to %s", typeName(got), typeName(want)), ce)
			cg.emit("    mov $0, %%rax")
			return
		}
		passedArgs = append(passedArgs, arg)
	}

	if len(fn.Literal.TypeParams) > 0 && len(ce.TypeArguments) == 0 {
		for _, tp := range fn.Literal.TypeParams {
			if typeArgMap[tp] == "" {
				cg.addNodeError("could not infer generic type argument for "+tp+"; specify it explicitly", ce)
				cg.emit("    mov $0, %%rax")
				return
			}
		}
	}

	for _, capName := range fn.Captures {
		passedArgs = append(passedArgs, &ast.Identifier{Value: capName})
	}

	for i := len(passedArgs) - 1; i >= 0; i-- {
		cg.generateExpression(passedArgs[i])
		cg.emit("    push %%rax")
	}

	callTarget := fn
	if len(fn.Literal.TypeParams) > 0 {
		callTarget = cg.ensureFunctionSpecialization(fn, typeArgMap)
	}
	cg.emit("    call %s", callTarget.Label)
	if len(passedArgs) > 0 {
		cg.emit("    add $%d, %%rsp", len(passedArgs)*8)
	}
}

func (cg *CodeGen) ensureFunctionSpecialization(fn *compiledFunction, typeArgMap map[string]string) *compiledFunction {
	if fn == nil || fn.Literal == nil || len(fn.Literal.TypeParams) == 0 {
		return fn
	}
	args := make([]string, len(fn.Literal.TypeParams))
	for i, tp := range fn.Literal.TypeParams {
		args[i] = typeArgMap[tp]
	}
	specKey := fn.Key + "<" + strings.Join(args, ",") + ">"
	if existing, ok := cg.functions[specKey]; ok && existing != nil {
		return existing
	}
	specTypeMap := make(map[string]string, len(typeArgMap))
	for k, v := range typeArgMap {
		specTypeMap[k] = v
	}
	spec := &compiledFunction{
		Key:              specKey,
		Name:             fn.Name,
		Label:            fn.Label + "__" + mangleGenericArgs(args),
		Literal:          fn.Literal,
		Captures:         append([]string{}, fn.Captures...),
		CaptureTypeNames: append([]string{}, fn.CaptureTypeNames...),
		TypeArgMap:       specTypeMap,
	}
	cg.functions[specKey] = spec
	return spec
}

func mangleGenericArgs(args []string) string {
	if len(args) == 0 {
		return "generic"
	}
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = mangleTypeForLabel(a)
	}
	return strings.Join(parts, "__")
}

func mangleTypeForLabel(t string) string {
	var b strings.Builder
	for _, r := range t {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		switch r {
		case '_':
			b.WriteString("_u")
		case '<':
			b.WriteString("_lt")
		case '>':
			b.WriteString("_gt")
		case '[':
			b.WriteString("_lb")
		case ']':
			b.WriteString("_rb")
		case '(':
			b.WriteString("_lp")
		case ')':
			b.WriteString("_rp")
		case ',':
			b.WriteString("_c")
		case '|':
			b.WriteString("_p")
		default:
			b.WriteString("_x")
		}
	}
	if b.Len() == 0 {
		return "type"
	}
	return b.String()
}

func (cg *CodeGen) canonicalGenericTypeArg(t string) string {
	if resolved, ok := cg.normalizeTypeName(t); ok && resolved != "" {
		return resolved
	}
	return t
}

func (cg *CodeGen) inferGenericTypeArgsFromTypes(paramType, argType string, typeArgMap map[string]string) (string, bool) {
	paramType = stripOuterGroupingParens(paramType)
	argType = stripOuterGroupingParens(cg.canonicalGenericTypeArg(argType))

	if cur, ok := typeArgMap[paramType]; ok {
		if cur == "" {
			typeArgMap[paramType] = argType
			return "", true
		}
		if cur == argType {
			return "", true
		}
		return fmt.Sprintf("cannot infer generic type %s from both %s and %s", paramType, cur, argType), false
	}

	pbase, pdims, pok := parseTypeDescriptor(paramType)
	abase, adims, aok := parseTypeDescriptor(argType)
	if pok && aok && len(pdims) > 0 && len(adims) > 0 && len(pdims) == len(adims) {
		for i := range pdims {
			if pdims[i] >= 0 && adims[i] >= 0 && pdims[i] != adims[i] {
				return "", true
			}
		}
		return cg.inferGenericTypeArgsFromTypes(pbase, abase, typeArgMap)
	}

	if pmembers, isTuple := splitTopLevelTuple(paramType); isTuple {
		amembers, argTuple := splitTopLevelTuple(argType)
		if !argTuple || len(pmembers) != len(amembers) {
			return "", true
		}
		for i := range pmembers {
			if msg, ok := cg.inferGenericTypeArgsFromTypes(pmembers[i], amembers[i], typeArgMap); !ok {
				return msg, false
			}
		}
		return "", true
	}

	if pbase, pargs, isGen := splitGenericType(paramType); isGen {
		abase, aargs, argGen := splitGenericType(argType)
		if argGen && pbase == abase && len(pargs) == len(aargs) {
			for i := range pargs {
				if msg, ok := cg.inferGenericTypeArgsFromTypes(pargs[i], aargs[i], typeArgMap); !ok {
					return msg, false
				}
			}
			return "", true
		}
		if alias, ok := cg.genericTypeAliases[pbase]; ok && len(alias.TypeParams) == len(pargs) {
			mapping := make(map[string]string, len(alias.TypeParams))
			for i, tp := range alias.TypeParams {
				mapping[tp] = pargs[i]
			}
			inst := substituteTypeParams(alias.TypeName, mapping)
			if msg, ok := cg.inferGenericTypeArgsFromTypes(inst, argType, typeArgMap); !ok {
				return msg, false
			}
			return "", true
		}
	}

	if members, isUnion := splitTopLevelUnion(paramType); isUnion {
		unique := 0
		var last map[string]string
		for _, m := range members {
			if !cg.isAssignableTypeName(m, argType) {
				continue
			}
			candidate := make(map[string]string, len(typeArgMap))
			for k, v := range typeArgMap {
				candidate[k] = v
			}
			if _, ok := cg.inferGenericTypeArgsFromTypes(m, argType, candidate); !ok {
				continue
			}
			unique++
			last = candidate
			if unique > 1 {
				return "", true
			}
		}
		if unique == 1 && last != nil {
			for k, v := range last {
				typeArgMap[k] = v
			}
		}
		return "", true
	}

	return "", true
}

func copyScope(scope map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{}, len(scope))
	for k := range scope {
		out[k] = struct{}{}
	}
	return out
}

func collectDeclaredNames(block *ast.BlockStatement, scope map[string]struct{}) {
	if block == nil {
		return
	}
	for _, st := range block.Statements {
		switch s := st.(type) {
		case *ast.LetStatement:
			if s != nil && s.Name != nil {
				scope[s.Name.Value] = struct{}{}
			}
		case *ast.ConstStatement:
			if s != nil && s.Name != nil {
				scope[s.Name.Value] = struct{}{}
			}
		case *ast.FunctionStatement:
			if s != nil && s.Name != nil {
				scope[s.Name.Value] = struct{}{}
			}
		case *ast.ForStatement:
			switch init := s.Init.(type) {
			case *ast.LetStatement:
				if init != nil && init.Name != nil {
					scope[init.Name.Value] = struct{}{}
				}
			case *ast.ConstStatement:
				if init != nil && init.Name != nil {
					scope[init.Name.Value] = struct{}{}
				}
			}
		}
	}
}

func (cg *CodeGen) computeCaptures(fn *ast.FunctionLiteral, outerScope map[string]struct{}) []string {
	local := make(map[string]struct{})
	for _, p := range fn.Parameters {
		local[p.Name.Value] = struct{}{}
	}
	collectDeclaredNames(fn.Body, local)

	used := map[string]struct{}{}
	collectUsedNamesInBlock(fn.Body, used)

	captures := []string{}
	for name := range used {
		if _, isLocal := local[name]; isLocal {
			continue
		}
		if _, existsOuter := outerScope[name]; !existsOuter {
			continue
		}
		if isBuiltinName(name) {
			continue
		}
		captures = append(captures, name)
	}

	for i := 0; i < len(captures)-1; i++ {
		for j := i + 1; j < len(captures); j++ {
			if captures[j] < captures[i] {
				captures[i], captures[j] = captures[j], captures[i]
			}
		}
	}
	return captures
}

func isBuiltinName(name string) bool {
	switch name {
	case "print", "println", "typeof", "typeofValue", "typeofvalue", "int", "float", "string", "char", "bool":
		return true
	default:
		return false
	}
}

func isTypeLiteralIdentifier(name string) bool {
	switch name {
	case "int", "bool", "float", "string", "char", "null", "type", "any":
		return true
	default:
		return false
	}
}

func collectUsedNamesInBlock(block *ast.BlockStatement, used map[string]struct{}) {
	if block == nil {
		return
	}
	for _, st := range block.Statements {
		collectUsedNamesInStatement(st, used)
	}
}

func collectUsedNamesInStatement(stmt ast.Statement, used map[string]struct{}) {
	switch s := stmt.(type) {
	case *ast.LetStatement:
		if s != nil && s.Value != nil {
			collectUsedNamesInExpression(s.Value, used)
		}
	case *ast.ConstStatement:
		if s != nil && s.Value != nil {
			collectUsedNamesInExpression(s.Value, used)
		}
	case *ast.AssignStatement:
		if s != nil && s.Name != nil {
			used[s.Name.Value] = struct{}{}
		}
		if s != nil && s.Value != nil {
			collectUsedNamesInExpression(s.Value, used)
		}
	case *ast.IndexAssignStatement:
		if s != nil && s.Left != nil {
			collectUsedNamesInExpression(s.Left.Left, used)
			collectUsedNamesInExpression(s.Left.Index, used)
		}
		if s != nil && s.Value != nil {
			collectUsedNamesInExpression(s.Value, used)
		}
	case *ast.WhileStatement:
		if s != nil && s.Condition != nil {
			collectUsedNamesInExpression(s.Condition, used)
		}
		if s != nil && s.Body != nil {
			collectUsedNamesInBlock(s.Body, used)
		}
	case *ast.LoopStatement:
		if s != nil && s.Body != nil {
			collectUsedNamesInBlock(s.Body, used)
		}
	case *ast.ForStatement:
		if s != nil && s.Init != nil {
			collectUsedNamesInStatement(s.Init, used)
		}
		if s != nil && s.Condition != nil {
			collectUsedNamesInExpression(s.Condition, used)
		}
		if s != nil && s.Periodic != nil {
			collectUsedNamesInStatement(s.Periodic, used)
		}
		if s != nil && s.Body != nil {
			collectUsedNamesInBlock(s.Body, used)
		}
	case *ast.ReturnStatement:
		if s != nil && s.ReturnValue != nil {
			collectUsedNamesInExpression(s.ReturnValue, used)
		}
	case *ast.ExpressionStatement:
		if s != nil && s.Expression != nil {
			collectUsedNamesInExpression(s.Expression, used)
		}
	case *ast.FunctionStatement:
		// Nested function bodies are handled independently; skip here.
		return
	}
}

func collectUsedNamesInExpression(expr ast.Expression, used map[string]struct{}) {
	switch e := expr.(type) {
	case *ast.Identifier:
		used[e.Value] = struct{}{}
	case *ast.PrefixExpression:
		collectUsedNamesInExpression(e.Right, used)
	case *ast.InfixExpression:
		collectUsedNamesInExpression(e.Left, used)
		collectUsedNamesInExpression(e.Right, used)
	case *ast.IfExpression:
		collectUsedNamesInExpression(e.Condition, used)
		collectUsedNamesInBlock(e.Consequence, used)
		collectUsedNamesInBlock(e.Alternative, used)
	case *ast.CallExpression:
		collectUsedNamesInExpression(e.Function, used)
		for _, arg := range e.Arguments {
			collectUsedNamesInExpression(arg, used)
		}
	case *ast.ArrayLiteral:
		for _, el := range e.Elements {
			collectUsedNamesInExpression(el, used)
		}
	case *ast.TupleLiteral:
		for _, el := range e.Elements {
			collectUsedNamesInExpression(el, used)
		}
	case *ast.IndexExpression:
		collectUsedNamesInExpression(e.Left, used)
		collectUsedNamesInExpression(e.Index, used)
	case *ast.TupleAccessExpression:
		collectUsedNamesInExpression(e.Left, used)
	case *ast.MethodCallExpression:
		collectUsedNamesInExpression(e.Object, used)
		for _, arg := range e.Arguments {
			collectUsedNamesInExpression(arg, used)
		}
	case *ast.MemberAccessExpression:
		collectUsedNamesInExpression(e.Object, used)
	case *ast.NullSafeAccessExpression:
		collectUsedNamesInExpression(e.Object, used)
	case *ast.NamedArgument:
		collectUsedNamesInExpression(e.Value, used)
	case *ast.FunctionLiteral:
		// Nested function body capture set should be computed independently.
		return
	}
}

func (cg *CodeGen) generateCastCall(castName string, ce *ast.CallExpression) {
	if len(ce.Arguments) != 1 {
		cg.failNodef(ce, "%s expects exactly 1 argument", castName)
		return
	}
	argType := cg.inferExpressionType(ce.Arguments[0])
	cg.generateExpression(ce.Arguments[0])

	switch castName {
	case "int":
		if argType == typeInt || argType == typeBool || argType == typeChar {
			return
		}
	case "bool":
		if argType == typeBool {
			return
		}
		if argType == typeInt || argType == typeChar {
			cg.emit("    test %%rax, %%rax")
			cg.emit("    setne %%al")
			cg.emit("    movzbq %%al, %%rax")
			return
		}
	case "char":
		if argType == typeChar || argType == typeInt || argType == typeBool {
			return
		}
	case "string":
		if argType == typeString || argType == typeFloat || argType == typeType || argType == typeNull {
			return
		}
	case "float":
		if argType == typeFloat {
			return
		}
	}

	cg.failNodef(ce, "cannot cast %s to %s", typeName(argType), castName)
}
