package codegen

import (
	"fmt"
	"sort"
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
	cg.varFuncs = make(map[string]string)
	cg.typeAliases = make(map[string]string)
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
			Key:      key,
			Name:     s.Name.Value,
			Label:    "fn_" + s.Name.Value,
			Literal:  s.Function,
			Captures: captures,
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
			Key:      key,
			Label:    "fn_" + key,
			Literal:  e,
			Captures: captures,
		}
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
	keys := make([]string, 0, len(cg.functions))
	for k := range cg.functions {
		keys = append(keys, k)
	}
	sort.Strings(keys) // deterministic output for tests
	for _, key := range keys {
		cg.generateOneFunction(cg.functions[key])
	}
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
	cg.funcRetType = cg.parseTypeName(fn.Literal.ReturnType)
	cg.funcRetTypeName = fn.Literal.ReturnType
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

	paramRegs := []string{"%rdi", "%rsi", "%rdx", "%rcx", "%r8", "%r9"}
	totalParams := len(fn.Literal.Parameters) + len(fn.Captures)
	if totalParams > len(paramRegs) {
		cg.addNodeError("functions with more than 6 total parameters/captures are not supported in codegen", fn.Literal)
	}

	for idx, p := range fn.Literal.Parameters {
		if idx >= len(paramRegs) {
			break
		}
		offset := cg.allocateSlots(1)
		cg.variables[p.Name.Value] = offset
		cg.markDeclaredInCurrentScope(p.Name.Value)
		cg.emit("    mov %s, -%d(%%rbp)  # param %s", paramRegs[idx], offset, p.Name.Value)
		pt := cg.parseTypeName(p.TypeName)
		cg.varTypes[p.Name.Value] = pt
		cg.varDeclared[p.Name.Value] = pt
		if p.TypeName != "" {
			cg.varTypeNames[p.Name.Value] = p.TypeName
			cg.varDeclaredNames[p.Name.Value] = p.TypeName
			cg.varValueTypeName[p.Name.Value] = p.TypeName
			if _, n, ok := peelArrayType(p.TypeName); ok {
				cg.varArrayLen[p.Name.Value] = n
			}
		}
		cg.varIsNull[p.Name.Value] = false
	}

	for idx, name := range fn.Captures {
		regIdx := len(fn.Literal.Parameters) + idx
		if regIdx >= len(paramRegs) {
			break
		}
		offset := cg.allocateSlots(1)
		cg.variables[name] = offset
		cg.markDeclaredInCurrentScope(name)
		cg.emit("    mov %s, -%d(%%rbp)  # capture %s", paramRegs[regIdx], offset, name)
		cg.varTypes[name] = typeUnknown
		cg.varDeclared[name] = typeUnknown
		cg.varTypeNames[name] = "unknown"
		cg.varValueTypeName[name] = "unknown"
		cg.varIsNull[name] = false
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
	paramRegs := []string{"%rdi", "%rsi", "%rdx", "%rcx", "%r8", "%r9"}
	totalParams := len(fn.Literal.Parameters) + len(fn.Captures)
	if totalParams > len(paramRegs) {
		cg.addNodeError("functions with more than 6 total parameters/captures are not supported in codegen", ce)
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

	for i, arg := range finalArgs {
		want := cg.parseTypeName(fn.Literal.Parameters[i].TypeName)
		got := cg.inferExpressionType(arg)
		wantName := fn.Literal.Parameters[i].TypeName
		if wantName == "" {
			wantName = typeName(want)
		}
		gotName := cg.inferExpressionTypeName(arg)
		if wantName != "unknown" && gotName != "unknown" && !cg.isAssignableTypeName(wantName, gotName) {
			cg.addNodeError(fmt.Sprintf("cannot assign %s to %s", gotName, wantName), ce)
			cg.emit("    mov $0, %%rax")
			return
		}
		if want != typeUnknown && got != typeUnknown && got != typeNull && got != want && cg.parseTypeName(wantName) != typeArray {
			cg.addNodeError(fmt.Sprintf("cannot assign %s to %s", typeName(got), typeName(want)), ce)
			cg.emit("    mov $0, %%rax")
			return
		}
		cg.generateExpression(arg)
		cg.emit("    mov %%rax, %s", paramRegs[i])
	}

	for idx, capName := range fn.Captures {
		regIdx := len(fn.Literal.Parameters) + idx
		if regIdx >= len(paramRegs) {
			break
		}
		cg.generateExpression(&ast.Identifier{Value: capName})
		cg.emit("    mov %%rax, %s", paramRegs[regIdx])
	}

	cg.emit("    call %s", fn.Label)
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
	case "print", "typeof", "typeofValue", "typeofvalue", "int", "float", "string", "char", "bool":
		return true
	default:
		return false
	}
}

func isTypeLiteralIdentifier(name string) bool {
	switch name {
	case "int", "bool", "float", "string", "char", "null", "type":
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
