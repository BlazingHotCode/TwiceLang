package evaluator

import (
	"twice/internal/ast"
	"twice/internal/object"
	"twice/internal/typesys"
)

func evalExpressions(exps []ast.Expression, env *object.Environment) []object.Object {
	var result []object.Object

	for _, e := range exps {
		evaluated := Eval(e, env)
		if isError(evaluated) {
			return []object.Object{evaluated}
		}
		result = append(result, evaluated)
	}

	return result
}

func evalCallArguments(exps []ast.Expression, env *object.Environment) ([]object.Object, map[string]object.Object, *object.Error) {
	positional := []object.Object{}
	named := make(map[string]object.Object)

	for _, e := range exps {
		if na, ok := e.(*ast.NamedArgument); ok {
			val := Eval(na.Value, env)
			if isError(val) {
				return nil, nil, val.(*object.Error)
			}
			if _, exists := named[na.Name]; exists {
				return nil, nil, newError("duplicate named argument: %s", na.Name)
			}
			named[na.Name] = val
			continue
		}

		evaluated := Eval(e, env)
		if isError(evaluated) {
			return nil, nil, evaluated.(*object.Error)
		}
		positional = append(positional, evaluated)
	}

	return positional, named, nil
}

// applyFunction calls a function with arguments
func applyFunction(fn object.Object, args []object.Object, namedArgs map[string]object.Object, typeArgs []string) object.Object {
	if builtin, ok := fn.(*object.Builtin); ok {
		if len(typeArgs) > 0 {
			return newError("generic type arguments are not supported for builtin functions")
		}
		if len(namedArgs) > 0 {
			return newError("named arguments are not supported for builtin functions")
		}
		return builtin.Fn(args...)
	}
	function, ok := fn.(*object.Function)
	if !ok {
		return newError("not a function: %s", fn.Type())
	}
	return applyUserFunction(function, args, namedArgs, typeArgs)
}

func applyUserFunction(function *object.Function, args []object.Object, namedArgs map[string]object.Object, typeArgs []string) object.Object {
	extendedEnv := object.NewEnclosedEnvironment(function.Env)
	typeArgMap := map[string]string{}
	if len(function.TypeParams) > 0 {
		if len(typeArgs) > 0 {
			if len(typeArgs) != len(function.TypeParams) {
				return newError("wrong number of generic type arguments: expected %d, got %d", len(function.TypeParams), len(typeArgs))
			}
			for i, tp := range function.TypeParams {
				argType := typeArgs[i]
				if !isKnownTypeName(argType, function.Env) {
					return newError("unknown type: %s", argType)
				}
				typeArgMap[tp] = argType
			}
		} else {
			for _, tp := range function.TypeParams {
				typeArgMap[tp] = ""
			}
		}
	} else if len(typeArgs) > 0 {
		return newError("generic type arguments provided for non-generic function")
	}

	posIdx := 0
	usedNamed := make(map[string]bool)
	for _, param := range function.Parameters {
		var val object.Object
		if posIdx < len(args) {
			val = args[posIdx]
			posIdx++
		} else if namedVal, ok := namedArgs[param.Name.Value]; ok {
			val = namedVal
			usedNamed[param.Name.Value] = true
		} else if param.DefaultValue != nil {
			val = Eval(param.DefaultValue, extendedEnv)
			if isError(val) {
				return val
			}
		} else {
			return newError("missing required argument: %s", param.Name.Value)
		}

		targetType := param.TypeName
		if len(typeArgMap) > 0 && targetType != "" {
			valType := runtimeTypeName(val)
			if valType != "unknown" {
				if msg, ok := inferGenericTypeArgsFromTypes(targetType, valType, typeArgMap, function.Env); !ok {
					return newError("%s", msg)
				}
			}
			targetType = typesys.SubstituteTypeParams(targetType, typeArgMap)
		}
		if targetType == "" {
			targetType = runtimeTypeName(val)
		}
		valType := runtimeTypeName(val)
		if !isAssignableToType(targetType, valType, extendedEnv) {
			return newError("cannot assign %s to %s", valType, targetType)
		}
		extendedEnv.Set(param.Name.Value, val)
		extendedEnv.SetType(param.Name.Value, targetType)
	}

	if posIdx < len(args) {
		return newError("too many positional arguments")
	}
	for name := range namedArgs {
		if usedNamed[name] {
			continue
		}
		if functionHasParam(function, name) {
			return newError("argument provided twice: %s", name)
		}
		return newError("unknown named argument: %s", name)
	}

	if len(function.TypeParams) > 0 && len(typeArgs) == 0 {
		for _, tp := range function.TypeParams {
			if typeArgMap[tp] == "" {
				return newError("could not infer generic type argument for %s; specify it explicitly", tp)
			}
		}
	}

	evaluated := Eval(function.Body, extendedEnv)
	result := unwrapReturnValue(evaluated)
	if result != nil {
		switch result.Type() {
		case object.BREAK_OBJ:
			return newError("break not inside loop")
		case object.CONTINUE_OBJ:
			return newError("continue not inside loop")
		}
	}
	if isError(result) {
		return result
	}

	if function.ReturnType != "" {
		returnType := function.ReturnType
		if len(typeArgMap) > 0 {
			for _, tp := range function.TypeParams {
				if containsTypeToken(returnType, tp) && typeArgMap[tp] == "" {
					return newError("could not infer generic type argument for %s", tp)
				}
			}
			returnType = typesys.SubstituteTypeParams(returnType, typeArgMap)
		}
		got := runtimeTypeName(result)
		if got == "null" {
			if !typeAllowsNull(returnType, function.Env) {
				return newError("cannot return %s from function returning %s", got, returnType)
			}
			return result
		}
		if !isAssignableToType(returnType, got, function.Env) {
			return newError("cannot return %s from function returning %s", got, returnType)
		}
	}

	return result
}

func containsTypeToken(typeName, tokenName string) bool {
	if typeName == "" || tokenName == "" {
		return false
	}
	for i := 0; i < len(typeName); {
		ch := typeName[i]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_' {
			j := i + 1
			for j < len(typeName) {
				c := typeName[j]
				if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || (c >= '0' && c <= '9') {
					j++
					continue
				}
				break
			}
			if typeName[i:j] == tokenName {
				return true
			}
			i = j
			continue
		}
		i++
	}
	return false
}

func inferGenericTypeArgsFromTypes(paramType, argType string, typeArgMap map[string]string, env *object.Environment) (string, bool) {
	paramType = stripOuterGroupingParens(paramType)
	argType = stripOuterGroupingParens(argType)

	if cur, ok := typeArgMap[paramType]; ok {
		argNorm := canonicalGenericTypeArg(argType, env)
		if cur == "" {
			typeArgMap[paramType] = argNorm
			return "", true
		}
		if cur == argNorm {
			return "", true
		}
		return "cannot infer generic type " + paramType + " from both " + cur + " and " + argNorm, false
	}

	pbase, pdims, pok := parseTypeName(paramType)
	abase, adims, aok := parseTypeName(argType)
	if pok && aok && len(pdims) > 0 && len(adims) > 0 && len(pdims) == len(adims) {
		for i := range pdims {
			if pdims[i] >= 0 && adims[i] >= 0 && pdims[i] != adims[i] {
				return "", true
			}
		}
		return inferGenericTypeArgsFromTypes(pbase, abase, typeArgMap, env)
	}

	if pmembers, isTuple := splitTopLevelTuple(paramType); isTuple {
		amembers, argTuple := splitTopLevelTuple(argType)
		if !argTuple || len(pmembers) != len(amembers) {
			return "", true
		}
		for i := range pmembers {
			if msg, ok := inferGenericTypeArgsFromTypes(pmembers[i], amembers[i], typeArgMap, env); !ok {
				return msg, false
			}
		}
		return "", true
	}

	if pbase, pargs, isGen := typesys.SplitGenericType(paramType); isGen {
		abase, aargs, argGen := typesys.SplitGenericType(argType)
		if argGen && pbase == abase && len(pargs) == len(aargs) {
			for i := range pargs {
				if msg, ok := inferGenericTypeArgsFromTypes(pargs[i], aargs[i], typeArgMap, env); !ok {
					return msg, false
				}
			}
			return "", true
		}
		if alias, ok := env.GenericTypeAlias(pbase); ok && len(alias.TypeParams) == len(pargs) {
			mapping := make(map[string]string, len(alias.TypeParams))
			for i, tp := range alias.TypeParams {
				mapping[tp] = pargs[i]
			}
			inst := typesys.SubstituteTypeParams(alias.TypeName, mapping)
			if msg, ok := inferGenericTypeArgsFromTypes(inst, argType, typeArgMap, env); !ok {
				return msg, false
			}
			return "", true
		}
	}

	if members, isUnion := splitTopLevelUnion(paramType); isUnion {
		unique := 0
		var last map[string]string
		for _, m := range members {
			if !isAssignableToType(m, argType, env) {
				continue
			}
			candidate := make(map[string]string, len(typeArgMap))
			for k, v := range typeArgMap {
				candidate[k] = v
			}
			if _, ok := inferGenericTypeArgsFromTypes(m, argType, candidate, env); !ok {
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

func canonicalGenericTypeArg(t string, env *object.Environment) string {
	if resolved, ok := typesys.NormalizeTypeName(t, typeAliasResolver(env)); ok && resolved != "" {
		return resolved
	}
	return t
}

func stripOuterGroupingParens(s string) string {
	out := s
	for len(out) >= 2 && out[0] == '(' && out[len(out)-1] == ')' {
		inner := out[1 : len(out)-1]
		if inner == "" {
			break
		}
		parts, isTuple := splitTopLevelTuple(out)
		if isTuple && len(parts) > 0 {
			break
		}
		out = inner
	}
	return out
}

func functionHasParam(fn *object.Function, name string) bool {
	for _, p := range fn.Parameters {
		if p.Name.Value == name {
			return true
		}
	}
	return false
}
