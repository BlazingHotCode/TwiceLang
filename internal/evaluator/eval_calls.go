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
func applyFunction(fn object.Object, args []object.Object, namedArgs map[string]object.Object) object.Object {
	if builtin, ok := fn.(*object.Builtin); ok {
		if len(namedArgs) > 0 {
			return newError("named arguments are not supported for builtin functions")
		}
		return builtin.Fn(args...)
	}
	function, ok := fn.(*object.Function)
	if !ok {
		return newError("not a function: %s", fn.Type())
	}
	return applyUserFunction(function, args, namedArgs)
}

func applyUserFunction(function *object.Function, args []object.Object, namedArgs map[string]object.Object) object.Object {
	extendedEnv := object.NewEnclosedEnvironment(function.Env)
	typeArgMap := map[string]string{}
	if len(function.TypeParams) > 0 {
		for _, tp := range function.TypeParams {
			typeArgMap[tp] = ""
		}
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
			if cur, ok := typeArgMap[targetType]; ok {
				valType := runtimeTypeName(val)
				if cur == "" {
					typeArgMap[targetType] = valType
				} else if cur != valType {
					return newError("cannot infer generic type %s from both %s and %s", targetType, cur, valType)
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

func functionHasParam(fn *object.Function, name string) bool {
	for _, p := range fn.Parameters {
		if p.Name.Value == name {
			return true
		}
	}
	return false
}
