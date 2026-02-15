package codegen

type lexicalScopeState struct {
	variables          map[string]int
	constVars          map[string]bool
	varTypes           map[string]valueType
	varDeclared        map[string]valueType
	varTypeNames       map[string]string
	varDeclaredNames   map[string]string
	varValueTypeName   map[string]string
	varIsNull          map[string]bool
	varArrayLen        map[string]int
	intVals            map[string]int64
	charVals           map[string]rune
	stringVals         map[string]string
	floatVals          map[string]float64
	varFuncs           map[string]string
	typeAliases        map[string]string
	genericTypeAliases map[string]genericTypeAlias
}

func (cg *CodeGen) snapshotLexicalState() lexicalScopeState {
	return lexicalScopeState{
		variables:          cloneMap(cg.variables),
		constVars:          cloneMap(cg.constVars),
		varTypes:           cloneMap(cg.varTypes),
		varDeclared:        cloneMap(cg.varDeclared),
		varTypeNames:       cloneMap(cg.varTypeNames),
		varDeclaredNames:   cloneMap(cg.varDeclaredNames),
		varValueTypeName:   cloneMap(cg.varValueTypeName),
		varIsNull:          cloneMap(cg.varIsNull),
		varArrayLen:        cloneMap(cg.varArrayLen),
		intVals:            cloneMap(cg.intVals),
		charVals:           cloneMap(cg.charVals),
		stringVals:         cloneMap(cg.stringVals),
		floatVals:          cloneMap(cg.floatVals),
		varFuncs:           cloneMap(cg.varFuncs),
		typeAliases:        cloneMap(cg.typeAliases),
		genericTypeAliases: cloneMap(cg.genericTypeAliases),
	}
}

func (cg *CodeGen) restoreLexicalState(st lexicalScopeState) {
	cg.variables = st.variables
	cg.constVars = st.constVars
	cg.varTypes = st.varTypes
	cg.varDeclared = st.varDeclared
	cg.varTypeNames = st.varTypeNames
	cg.varDeclaredNames = st.varDeclaredNames
	cg.varValueTypeName = st.varValueTypeName
	cg.varIsNull = st.varIsNull
	cg.varArrayLen = st.varArrayLen
	cg.intVals = st.intVals
	cg.charVals = st.charVals
	cg.stringVals = st.stringVals
	cg.floatVals = st.floatVals
	cg.varFuncs = st.varFuncs
	cg.typeAliases = st.typeAliases
	cg.genericTypeAliases = st.genericTypeAliases
}

func cloneMap[K comparable, V any](in map[K]V) map[K]V {
	out := make(map[K]V, len(in))
	for k := range in {
		out[k] = in[k]
	}
	return out
}

func (cg *CodeGen) currentScopeDecls() map[string]struct{} {
	if len(cg.scopeDecls) == 0 {
		cg.scopeDecls = append(cg.scopeDecls, make(map[string]struct{}))
	}
	return cg.scopeDecls[len(cg.scopeDecls)-1]
}

func (cg *CodeGen) currentTypeScopeDecls() map[string]struct{} {
	if len(cg.typeScopeDecls) == 0 {
		cg.typeScopeDecls = append(cg.typeScopeDecls, make(map[string]struct{}))
	}
	return cg.typeScopeDecls[len(cg.typeScopeDecls)-1]
}

func (cg *CodeGen) isDeclaredInCurrentScope(name string) bool {
	_, ok := cg.currentScopeDecls()[name]
	return ok
}

func (cg *CodeGen) markDeclaredInCurrentScope(name string) {
	cg.currentScopeDecls()[name] = struct{}{}
}

func (cg *CodeGen) isTypeAliasDeclaredInCurrentScope(name string) bool {
	_, ok := cg.currentTypeScopeDecls()[name]
	return ok
}

func (cg *CodeGen) markTypeAliasDeclaredInCurrentScope(name string) {
	cg.currentTypeScopeDecls()[name] = struct{}{}
}

func (cg *CodeGen) enterScope() lexicalScopeState {
	cg.scopeDecls = append(cg.scopeDecls, make(map[string]struct{}))
	cg.typeScopeDecls = append(cg.typeScopeDecls, make(map[string]struct{}))
	return cg.snapshotLexicalState()
}

func (cg *CodeGen) exitScope(st lexicalScopeState) {
	if len(cg.scopeDecls) == 0 || len(cg.typeScopeDecls) == 0 {
		return
	}
	declared := cg.scopeDecls[len(cg.scopeDecls)-1]
	cg.scopeDecls = cg.scopeDecls[:len(cg.scopeDecls)-1]
	cg.typeScopeDecls = cg.typeScopeDecls[:len(cg.typeScopeDecls)-1]

	currVariables := cg.variables
	currConstVars := cg.constVars
	currVarTypes := cg.varTypes
	currVarDeclared := cg.varDeclared
	currVarTypeNames := cg.varTypeNames
	currVarDeclaredNames := cg.varDeclaredNames
	currVarValueTypeName := cg.varValueTypeName
	currVarIsNull := cg.varIsNull
	currVarArrayLen := cg.varArrayLen
	currIntVals := cg.intVals
	currCharVals := cg.charVals
	currStringVals := cg.stringVals
	currFloatVals := cg.floatVals
	currVarFuncs := cg.varFuncs

	cg.restoreLexicalState(st)

	for name := range st.variables {
		if _, shadowed := declared[name]; shadowed {
			continue
		}
		if _, ok := currVariables[name]; !ok {
			continue
		}

		cg.variables[name] = currVariables[name]
		if v, ok := currConstVars[name]; ok {
			cg.constVars[name] = v
		} else {
			delete(cg.constVars, name)
		}
		if v, ok := currVarTypes[name]; ok {
			cg.varTypes[name] = v
		} else {
			delete(cg.varTypes, name)
		}
		if v, ok := currVarDeclared[name]; ok {
			cg.varDeclared[name] = v
		} else {
			delete(cg.varDeclared, name)
		}
		if v, ok := currVarTypeNames[name]; ok {
			cg.varTypeNames[name] = v
		} else {
			delete(cg.varTypeNames, name)
		}
		if v, ok := currVarDeclaredNames[name]; ok {
			cg.varDeclaredNames[name] = v
		} else {
			delete(cg.varDeclaredNames, name)
		}
		if v, ok := currVarValueTypeName[name]; ok {
			cg.varValueTypeName[name] = v
		} else {
			delete(cg.varValueTypeName, name)
		}
		if v, ok := currVarIsNull[name]; ok {
			cg.varIsNull[name] = v
		} else {
			delete(cg.varIsNull, name)
		}
		if v, ok := currVarArrayLen[name]; ok {
			cg.varArrayLen[name] = v
		} else {
			delete(cg.varArrayLen, name)
		}
		if v, ok := currIntVals[name]; ok {
			cg.intVals[name] = v
		} else {
			delete(cg.intVals, name)
		}
		if v, ok := currCharVals[name]; ok {
			cg.charVals[name] = v
		} else {
			delete(cg.charVals, name)
		}
		if v, ok := currStringVals[name]; ok {
			cg.stringVals[name] = v
		} else {
			delete(cg.stringVals, name)
		}
		if v, ok := currFloatVals[name]; ok {
			cg.floatVals[name] = v
		} else {
			delete(cg.floatVals, name)
		}
		if v, ok := currVarFuncs[name]; ok {
			cg.varFuncs[name] = v
		} else {
			delete(cg.varFuncs, name)
		}
	}
}
