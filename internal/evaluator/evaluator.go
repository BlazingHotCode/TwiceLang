package evaluator

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"twice/internal/ast"
	"twice/internal/object"
	"twice/internal/token"
)

// TRUE and FALSE are singletons - we reuse these instead of creating new booleans
var (
	TRUE  = &object.Boolean{Value: true}
	FALSE = &object.Boolean{Value: false}
	NULL  = &object.Null{}
	BREAK = &object.Break{}
	CONT  = &object.Continue{}
)

var builtins = map[string]*object.Builtin{
	"typeof": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("typeof expects 1 argument, got=%d", len(args))
			}
			return &object.TypeValue{Name: runtimeTypeName(args[0])}
		},
	},
	"typeofValue": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("typeofValue expects 1 argument, got=%d", len(args))
			}
			return &object.TypeValue{Name: runtimeTypeName(args[0])}
		},
	},
	"typeofvalue": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("typeofvalue expects 1 argument, got=%d", len(args))
			}
			return &object.TypeValue{Name: runtimeTypeName(args[0])}
		},
	},
	"int": {
		Fn: func(args ...object.Object) object.Object { return castToInt(args) },
	},
	"float": {
		Fn: func(args ...object.Object) object.Object { return castToFloat(args) },
	},
	"string": {
		Fn: func(args ...object.Object) object.Object { return castToString(args) },
	},
	"char": {
		Fn: func(args ...object.Object) object.Object { return castToChar(args) },
	},
	"bool": {
		Fn: func(args ...object.Object) object.Object { return castToBool(args) },
	},
}

// Eval is the heart of the interpreter
// It takes an AST node and returns an Object
// This is recursive - expressions contain expressions
func Eval(node ast.Node, env *object.Environment) object.Object {
	switch node := node.(type) {
	case *ast.Program:
		return evalProgram(node, env)

	case *ast.BlockStatement:
		return evalBlockStatement(node, env)

	case *ast.ExpressionStatement:
		return Eval(node.Expression, env)

	case *ast.ReturnStatement:
		val := Eval(node.ReturnValue, env)
		if isError(val) {
			return val
		}
		return &object.ReturnValue{Value: val}
	case *ast.BreakStatement:
		return BREAK
	case *ast.ContinueStatement:
		return CONT
	case *ast.WhileStatement:
		return evalWhileStatement(node, env)
	case *ast.LoopStatement:
		return evalLoopStatement(node, env)
	case *ast.ForStatement:
		return evalForStatement(node, env)
	case *ast.LetStatement:
		if env.HasInCurrentScope(node.Name.Value) {
			if env.IsConstInCurrentScope(node.Name.Value) {
				return newError("cannot reassign const: %s", node.Name.Value)
			}
			return newError("identifier already declared: %s", node.Name.Value)
		}
		var val object.Object = NULL
		if node.Value != nil {
			val = Eval(node.Value, env)
			if isError(val) {
				return val
			}
		}
		varType := node.TypeName
		if varType != "" && !isKnownTypeName(varType, env) {
			return newError("unknown type: %s", varType)
		}
		valType := runtimeTypeName(val)
		if varType == "" {
			varType = valType
		}
		if !isAssignableToType(varType, valType, env) {
			return newError("cannot assign %s to %s", valType, varType)
		}
		env.Set(node.Name.Value, val)
		env.SetType(node.Name.Value, varType)

	case *ast.ConstStatement:
		if env.HasInCurrentScope(node.Name.Value) {
			return newError("identifier already declared: %s", node.Name.Value)
		}
		val := Eval(node.Value, env)
		if isError(val) {
			return val
		}
		varType := node.TypeName
		if varType != "" && !isKnownTypeName(varType, env) {
			return newError("unknown type: %s", varType)
		}
		valType := runtimeTypeName(val)
		if varType == "" {
			varType = valType
		}
		if !isAssignableToType(varType, valType, env) {
			return newError("cannot assign %s to %s", valType, varType)
		}
		env.SetConst(node.Name.Value, val)
		env.SetType(node.Name.Value, varType)
	case *ast.TypeDeclStatement:
		if node.Name == nil {
			return newError("invalid type declaration")
		}
		if node.Name.Value == "type" || isBuiltinTypeName(node.Name.Value) {
			return newError("cannot redefine builtin type: %s", node.Name.Value)
		}
		if env.HasTypeAliasInCurrentScope(node.Name.Value) {
			return newError("type already declared: %s", node.Name.Value)
		}
		resolved, ok := resolveTypeName(node.TypeName, env, map[string]struct{}{})
		if !ok || !isKnownTypeName(resolved, env) {
			return newError("unknown type: %s", node.TypeName)
		}
		env.SetTypeAlias(node.Name.Value, resolved)
	case *ast.AssignStatement:
		if !env.Has(node.Name.Value) {
			return newError("identifier not found: " + node.Name.Value)
		}
		if env.IsConst(node.Name.Value) {
			return newError("cannot reassign const: %s", node.Name.Value)
		}
		val := Eval(node.Value, env)
		if isError(val) {
			return val
		}
		targetType, ok := env.TypeOf(node.Name.Value)
		if !ok {
			targetType = runtimeTypeName(val)
			env.SetType(node.Name.Value, targetType)
		}
		if inf, ok := node.Value.(*ast.InfixExpression); ok {
			if inf.Token.Type == token.PLUSPLUS || inf.Token.Type == token.MINUSMIN {
				if targetType != "int" {
					return newError("++/-- only supported for int variables")
				}
			}
		}
		valType := runtimeTypeName(val)
		if targetType == "null" && valType != "null" {
			targetType = valType
			env.SetType(node.Name.Value, targetType)
		}
		if !isAssignableToType(targetType, valType, env) {
			return newError("cannot assign %s to %s", valType, targetType)
		}
		env.Assign(node.Name.Value, val)
	case *ast.IndexAssignStatement:
		return evalIndexAssignStatement(node, env)
	case *ast.FunctionStatement:
		fnObj := Eval(node.Function, env)
		if isError(fnObj) {
			return fnObj
		}
		env.Set(node.Name.Value, fnObj)
		env.SetType(node.Name.Value, "unknown")

	// Expressions
	case *ast.IntegerLiteral:
		return &object.Integer{Value: node.Value}

	case *ast.FloatLiteral:
		return &object.Float{Value: node.Value}

	case *ast.StringLiteral:
		return &object.String{Value: node.Value}

	case *ast.CharLiteral:
		return &object.Char{Value: node.Value}

	case *ast.NullLiteral:
		return NULL

	case *ast.ArrayLiteral:
		return evalArrayLiteral(node, env)

	case *ast.Boolean:
		return nativeBoolToBooleanObject(node.Value)

	case *ast.Identifier:
		return evalIdentifier(node, env)

	case *ast.PrefixExpression:
		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return evalPrefixExpression(node.Operator, right)

	case *ast.InfixExpression:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return evalInfixExpression(node.Operator, left, right)
	case *ast.IndexExpression:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		index := Eval(node.Index, env)
		if isError(index) {
			return index
		}
		return evalIndexExpression(left, index)

	case *ast.IfExpression:
		return evalIfExpression(node, env)

	case *ast.FunctionLiteral:
		for _, param := range node.Parameters {
			if param.TypeName != "" && !isKnownTypeName(param.TypeName, env) {
				return newError("unknown type: %s", param.TypeName)
			}
		}
		if node.ReturnType != "" && !isKnownTypeName(node.ReturnType, env) {
			return newError("unknown type: %s", node.ReturnType)
		}
		name := ""
		if node.Name != nil {
			name = node.Name.Value
		}
		return &object.Function{
			Name:       name,
			Parameters: node.Parameters,
			ReturnType: node.ReturnType,
			Body:       node.Body,
			Env:        env,
		}

	case *ast.CallExpression:
		if ident, ok := node.Function.(*ast.Identifier); ok && ident.Value == "typeof" && len(node.Arguments) == 1 {
			if argIdent, ok := node.Arguments[0].(*ast.Identifier); ok {
				if t, ok := env.TypeOf(argIdent.Value); ok {
					return &object.TypeValue{Name: t}
				}
			}
		}
		if ident, ok := node.Function.(*ast.Identifier); ok {
			if _, exists := env.Get(ident.Value); !exists {
				if builtin, ok := builtins[ident.Value]; ok {
					args, namedArgs, argErr := evalCallArguments(node.Arguments, env)
					if argErr != nil {
						return argErr
					}
					return applyFunction(builtin, args, namedArgs)
				}
			}
		}
		function := Eval(node.Function, env)
		if isError(function) {
			return function
		}
		args, namedArgs, argErr := evalCallArguments(node.Arguments, env)
		if argErr != nil {
			return argErr
		}
		return applyFunction(function, args, namedArgs)
	case *ast.MethodCallExpression:
		return evalMethodCallExpression(node, env)
	}

	return nil
}

// evalProgram evaluates a list of statements
// Stops early if it hits a return or error
func evalProgram(program *ast.Program, env *object.Environment) object.Object {
	var result object.Object

	for _, statement := range program.Statements {
		result = Eval(statement, env)

		switch result := result.(type) {
		case *object.ReturnValue:
			return result.Value // Unwrap the return value
		case *object.Error:
			return result // Propagate errors
		case *object.Break:
			return newError("break not inside loop")
		case *object.Continue:
			return newError("continue not inside loop")
		}
	}

	return result
}

// evalBlockStatement evaluates statements inside braces
// Unlike evalProgram, it does NOT unwrap ReturnValue
// This lets returns bubble up through nested blocks
func evalBlockStatement(block *ast.BlockStatement, env *object.Environment) object.Object {
	blockEnv := object.NewEnclosedEnvironment(env)
	var result object.Object

	for _, statement := range block.Statements {
		result = Eval(statement, blockEnv)

		if result != nil {
			rt := result.Type()
			if rt == object.RETURN_VALUE_OBJ || rt == object.ERROR_OBJ || rt == object.BREAK_OBJ || rt == object.CONTINUE_OBJ {
				return result // Return as-is, don't unwrap
			}
		}
	}

	return result
}

// nativeBoolToBooleanObject converts Go bool to our singleton true/false
func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return TRUE
	}
	return FALSE
}

// evalIdentifier looks up a variable in the environment
func evalIdentifier(node *ast.Identifier, env *object.Environment) object.Object {
	val, ok := env.Get(node.Value)
	if !ok {
		if isKnownTypeName(node.Value, env) {
			return &object.TypeValue{Name: node.Value}
		}
		if builtin, ok := builtins[node.Value]; ok {
			return builtin
		}
		return newError("identifier not found: " + node.Value)
	}
	return val
}

// evalPrefixExpression handles ! and -
func evalPrefixExpression(operator string, right object.Object) object.Object {
	switch operator {
	case "!":
		return evalBangOperatorExpression(right)
	case "-":
		return evalMinusPrefixOperatorExpression(right)
	default:
		return newError("unknown operator: %s%s", operator, right.Type())
	}
}

// evalBangOperatorExpression handles !true, !false, !5, !null
func evalBangOperatorExpression(right object.Object) object.Object {
	switch right {
	case TRUE:
		return FALSE
	case FALSE:
		return TRUE
	case NULL:
		return TRUE // !null is true
	default:
		return FALSE // !anything_else is false (truthy)
	}
}

// evalMinusPrefixOperatorExpression handles -5, -10
func evalMinusPrefixOperatorExpression(right object.Object) object.Object {
	switch right := right.(type) {
	case *object.Integer:
		return &object.Integer{Value: -right.Value}
	case *object.Float:
		return &object.Float{Value: -right.Value}
	default:
		return newError("unknown operator: -%s", right.Type())
	}
}

// evalInfixExpression handles 5 + 3, true == false, etc.
func evalInfixExpression(operator string, left, right object.Object) object.Object {
	if operator == "&&" || operator == "||" || operator == "^^" {
		if left.Type() != object.BOOLEAN_OBJ || right.Type() != object.BOOLEAN_OBJ {
			return newError("type mismatch: %s %s %s", left.Type(), operator, right.Type())
		}
		lb := left.(*object.Boolean).Value
		rb := right.(*object.Boolean).Value
		switch operator {
		case "&&":
			return nativeBoolToBooleanObject(lb && rb)
		case "||":
			return nativeBoolToBooleanObject(lb || rb)
		case "^^":
			return nativeBoolToBooleanObject(lb != rb)
		}
	}

	switch {
	case left.Type() == object.INTEGER_OBJ && right.Type() == object.INTEGER_OBJ:
		if operator == "&" || operator == "|" || operator == "^" || operator == "<<" || operator == ">>" {
			return evalIntegerBitwiseInfixExpression(operator, left, right)
		}
		return evalIntegerInfixExpression(operator, left, right)
	case isNumeric(left) && isNumeric(right):
		return evalMixedNumericInfixExpression(operator, left, right)
	case left.Type() == object.FLOAT_OBJ && right.Type() == object.FLOAT_OBJ:
		return evalFloatInfixExpression(operator, left, right)
	case left.Type() == object.STRING_OBJ && right.Type() == object.STRING_OBJ:
		return evalStringInfixExpression(operator, left, right)
	case left.Type() == object.STRING_OBJ && operator == "+":
		return evalStringConcatWithCoercion(left, right)
	case right.Type() == object.STRING_OBJ && operator == "+":
		return evalStringConcatWithCoercionRight(left, right)
	case left.Type() == object.CHAR_OBJ && right.Type() == object.CHAR_OBJ:
		return evalCharInfixExpression(operator, left, right)
	case left.Type() == object.TYPE_OBJ && right.Type() == object.TYPE_OBJ:
		return evalTypeValueInfixExpression(operator, left, right)
	case left.Type() == object.CHAR_OBJ && right.Type() == object.INTEGER_OBJ && operator == "+":
		return &object.Char{Value: left.(*object.Char).Value + rune(right.(*object.Integer).Value)}
	case operator == "==":
		return nativeBoolToBooleanObject(left == right) // Pointer comparison works for singletons
	case operator == "!=":
		return nativeBoolToBooleanObject(left != right)
	case left.Type() != right.Type():
		return newError("type mismatch: %s %s %s", left.Type(), operator, right.Type())
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func evalIntegerBitwiseInfixExpression(operator string, left, right object.Object) object.Object {
	leftVal := left.(*object.Integer).Value
	rightVal := right.(*object.Integer).Value

	switch operator {
	case "&":
		return &object.Integer{Value: leftVal & rightVal}
	case "|":
		return &object.Integer{Value: leftVal | rightVal}
	case "^":
		return &object.Integer{Value: leftVal ^ rightVal}
	case "<<":
		return &object.Integer{Value: leftVal << uint(rightVal)}
	case ">>":
		return &object.Integer{Value: leftVal >> uint(rightVal)}
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func isNumeric(obj object.Object) bool {
	return obj.Type() == object.INTEGER_OBJ || obj.Type() == object.FLOAT_OBJ
}

func toFloat64(obj object.Object) float64 {
	switch v := obj.(type) {
	case *object.Integer:
		return float64(v.Value)
	case *object.Float:
		return v.Value
	default:
		return 0
	}
}

func evalMixedNumericInfixExpression(operator string, left, right object.Object) object.Object {
	leftVal := toFloat64(left)
	rightVal := toFloat64(right)

	switch operator {
	case "+":
		return &object.Float{Value: leftVal + rightVal}
	case "-":
		return &object.Float{Value: leftVal - rightVal}
	case "*":
		return &object.Float{Value: leftVal * rightVal}
	case "/":
		return &object.Float{Value: leftVal / rightVal}
	case "%":
		return &object.Float{Value: math.Mod(leftVal, rightVal)}
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func evalStringConcatWithCoercion(left, right object.Object) object.Object {
	leftVal := left.(*object.String).Value
	switch v := right.(type) {
	case *object.String:
		return &object.String{Value: leftVal + v.Value}
	case *object.Integer:
		return &object.String{Value: leftVal + strconv.FormatInt(v.Value, 10)}
	case *object.Float:
		return &object.String{Value: leftVal + strconv.FormatFloat(v.Value, 'g', -1, 64)}
	case *object.Char:
		return &object.String{Value: leftVal + string(v.Value)}
	default:
		return newError("type mismatch: %s + %s", left.Type(), right.Type())
	}
}

func evalStringConcatWithCoercionRight(left, right object.Object) object.Object {
	rightVal := right.(*object.String).Value
	switch v := left.(type) {
	case *object.String:
		return &object.String{Value: v.Value + rightVal}
	case *object.Integer:
		return &object.String{Value: strconv.FormatInt(v.Value, 10) + rightVal}
	case *object.Float:
		return &object.String{Value: strconv.FormatFloat(v.Value, 'g', -1, 64) + rightVal}
	case *object.Char:
		return &object.String{Value: string(v.Value) + rightVal}
	default:
		return newError("type mismatch: %s + %s", left.Type(), right.Type())
	}
}

func evalCharInfixExpression(operator string, left, right object.Object) object.Object {
	leftVal := left.(*object.Char).Value
	rightVal := right.(*object.Char).Value

	switch operator {
	case "+":
		return &object.Char{Value: leftVal + rightVal}
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func evalStringInfixExpression(operator string, left, right object.Object) object.Object {
	leftVal := left.(*object.String).Value
	rightVal := right.(*object.String).Value

	switch operator {
	case "+":
		return &object.String{Value: leftVal + rightVal}
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func evalTypeValueInfixExpression(operator string, left, right object.Object) object.Object {
	leftVal := left.(*object.TypeValue).Name
	rightVal := right.(*object.TypeValue).Name

	switch operator {
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func evalFloatInfixExpression(operator string, left, right object.Object) object.Object {
	leftVal := left.(*object.Float).Value
	rightVal := right.(*object.Float).Value

	switch operator {
	case "+":
		return &object.Float{Value: leftVal + rightVal}
	case "-":
		return &object.Float{Value: leftVal - rightVal}
	case "*":
		return &object.Float{Value: leftVal * rightVal}
	case "/":
		return &object.Float{Value: leftVal / rightVal}
	case "%":
		return &object.Float{Value: math.Mod(leftVal, rightVal)}
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

// evalIntegerInfixExpression handles integer arithmetic and comparison
func evalIntegerInfixExpression(operator string, left, right object.Object) object.Object {
	leftVal := left.(*object.Integer).Value
	rightVal := right.(*object.Integer).Value

	switch operator {
	case "+":
		return &object.Integer{Value: leftVal + rightVal}
	case "-":
		return &object.Integer{Value: leftVal - rightVal}
	case "*":
		return &object.Integer{Value: leftVal * rightVal}
	case "/":
		return &object.Integer{Value: leftVal / rightVal}
	case "%":
		return &object.Integer{Value: leftVal % rightVal}
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

// evalIfExpression handles if-else
func evalIfExpression(ie *ast.IfExpression, env *object.Environment) object.Object {
	condition := Eval(ie.Condition, env)
	if isError(condition) {
		return condition
	}

	if isTruthy(condition) {
		return Eval(ie.Consequence, env)
	} else if ie.Alternative != nil {
		return Eval(ie.Alternative, env)
	} else {
		return NULL
	}
}

func evalWhileStatement(ws *ast.WhileStatement, env *object.Environment) object.Object {
	var result object.Object = NULL
	for {
		condition := Eval(ws.Condition, env)
		if isError(condition) {
			return condition
		}
		if !isTruthy(condition) {
			return result
		}
		result = Eval(ws.Body, env)
		if result != nil {
			rt := result.Type()
			if rt == object.RETURN_VALUE_OBJ || rt == object.ERROR_OBJ {
				return result
			}
			if rt == object.BREAK_OBJ {
				return NULL
			}
			if rt == object.CONTINUE_OBJ {
				continue
			}
		}
	}
}

func evalLoopStatement(ls *ast.LoopStatement, env *object.Environment) object.Object {
	var result object.Object = NULL
	for {
		result = Eval(ls.Body, env)
		if result != nil {
			rt := result.Type()
			if rt == object.RETURN_VALUE_OBJ || rt == object.ERROR_OBJ {
				return result
			}
			if rt == object.BREAK_OBJ {
				return NULL
			}
			if rt == object.CONTINUE_OBJ {
				continue
			}
		}
	}
}

func evalForStatement(fs *ast.ForStatement, env *object.Environment) object.Object {
	loopEnv := object.NewEnclosedEnvironment(env)
	if fs.Init != nil {
		initRes := Eval(fs.Init, loopEnv)
		if isError(initRes) {
			return initRes
		}
	}

	var result object.Object = NULL
	for {
		condition := Eval(fs.Condition, loopEnv)
		if isError(condition) {
			return condition
		}
		if !isTruthy(condition) {
			return result
		}

		result = Eval(fs.Body, loopEnv)
		if result != nil {
			rt := result.Type()
			if rt == object.RETURN_VALUE_OBJ || rt == object.ERROR_OBJ {
				return result
			}
			if rt == object.BREAK_OBJ {
				return NULL
			}
			if rt == object.CONTINUE_OBJ {
				if fs.Periodic != nil {
					stepRes := Eval(fs.Periodic, loopEnv)
					if isError(stepRes) {
						return stepRes
					}
				}
				continue
			}
		}

		if fs.Periodic != nil {
			stepRes := Eval(fs.Periodic, loopEnv)
			if isError(stepRes) {
				return stepRes
			}
		}
	}
}

// isTruthy determines what counts as "true" in conditionals
// Only false and null are falsy, everything else is truthy
func isTruthy(obj object.Object) bool {
	switch obj {
	case NULL:
		return false
	case TRUE:
		return true
	case FALSE:
		return false
	default:
		return true
	}
}

// evalExpressions evaluates a list of expressions (function arguments)
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
		got := runtimeTypeName(result)
		if !isAssignableToType(function.ReturnType, got, function.Env) {
			return newError("cannot return %s from function returning %s", got, function.ReturnType)
		}
	}

	return result
}

func functionHasParam(fn *object.Function, name string) bool {
	for _, p := range fn.Parameters {
		if p.Name.Value == name {
			return true
		}
	}
	return false
}

func evalIndexExpression(left object.Object, index object.Object) object.Object {
	idxObj, ok := index.(*object.Integer)
	if !ok {
		return newError("index must be int, got %s", runtimeTypeName(index))
	}
	idx := int(idxObj.Value)

	if arr, ok := left.(*object.Array); ok {
		if idx < 0 || idx >= len(arr.Elements) {
			return newError("array index out of bounds: %d", idx)
		}
		return arr.Elements[idx]
	}

	if str, ok := left.(*object.String); ok {
		if idx < 0 || idx >= len(str.Value) {
			return newError("string index out of bounds: %d", idx)
		}
		return &object.Char{Value: rune(str.Value[idx])}
	}

	return newError("index operator not supported: %s", left.Type())
}

func evalIndexAssignStatement(node *ast.IndexAssignStatement, env *object.Environment) object.Object {
	if node.Left == nil {
		return newError("invalid indexed assignment")
	}

	ident, ok := node.Left.Left.(*ast.Identifier)
	if !ok {
		return newError("indexed assignment target must be an identifier")
	}
	if env.IsConst(ident.Value) {
		return newError("cannot reassign const: %s", ident.Value)
	}

	targetObj, exists := env.Get(ident.Value)
	if !exists {
		return newError("identifier not found: %s", ident.Value)
	}
	arr, ok := targetObj.(*object.Array)
	if !ok {
		return newError("indexed assignment target is not an array")
	}

	indexObj := Eval(node.Left.Index, env)
	if isError(indexObj) {
		return indexObj
	}
	idxInt, ok := indexObj.(*object.Integer)
	if !ok {
		return newError("array index must be int, got %s", runtimeTypeName(indexObj))
	}
	idx := int(idxInt.Value)
	if idx < 0 || idx >= len(arr.Elements) {
		return newError("array index out of bounds: %d", idx)
	}

	val := Eval(node.Value, env)
	if isError(val) {
		return val
	}
	valType := runtimeTypeName(val)
	if !isAssignableToType(arr.ElementType, valType, env) {
		return newError("cannot assign %s to %s", valType, arr.ElementType)
	}
	arr.Elements[idx] = val
	return val
}

func evalMethodCallExpression(node *ast.MethodCallExpression, env *object.Environment) object.Object {
	if node == nil || node.Method == nil {
		return newError("invalid method call")
	}
	obj := Eval(node.Object, env)
	if isError(obj) {
		return obj
	}
	switch node.Method.Value {
	case "length":
		if len(node.Arguments) != 0 {
			return newError("length expects 0 arguments, got=%d", len(node.Arguments))
		}
		arr, ok := obj.(*object.Array)
		if !ok {
			return newError("length is only supported on arrays")
		}
		return &object.Integer{Value: int64(len(arr.Elements))}
	default:
		return newError("unknown method: %s", node.Method.Value)
	}
}

func evalArrayLiteral(lit *ast.ArrayLiteral, env *object.Environment) object.Object {
	if len(lit.Elements) == 0 {
		return newError("empty array literals are not supported")
	}

	elements := make([]object.Object, 0, len(lit.Elements))
	elemType := ""
	for _, el := range lit.Elements {
		val := Eval(el, env)
		if isError(val) {
			return val
		}
		t := runtimeTypeName(val)
		if t == "null" {
			return newError("array literal elements cannot be null")
		}
		if elemType == "" {
			elemType = t
		} else if merged, ok := mergeTypeNames(elemType, t, env); ok {
			elemType = merged
		} else {
			return newError("array literal elements must have the same type")
		}
		elements = append(elements, val)
	}

	return &object.Array{
		ElementType: elemType,
		Elements:    elements,
	}
}

// unwrapReturnValue extracts the actual value from a ReturnValue
func unwrapReturnValue(obj object.Object) object.Object {
	if returnValue, ok := obj.(*object.ReturnValue); ok {
		return returnValue.Value
	}
	return obj
}

// newError creates an error object
func newError(format string, a ...interface{}) *object.Error {
	return &object.Error{Message: fmt.Sprintf(format, a...)}
}

// isError checks if an object is an error (to stop propagation)
func isError(obj object.Object) bool {
	if obj != nil {
		return obj.Type() == object.ERROR_OBJ
	}
	return false
}

func runtimeTypeName(obj object.Object) string {
	switch v := obj.(type) {
	case *object.Integer:
		return "int"
	case *object.Float:
		return "float"
	case *object.String:
		return "string"
	case *object.Char:
		return "char"
	case *object.Boolean:
		return "bool"
	case *object.Array:
		return formatTypeName(v.ElementType, []int{len(v.Elements)})
	case *object.Null:
		return "null"
	case *object.TypeValue:
		return "type"
	default:
		return "unknown"
	}
}

func isAssignableToType(targetType string, valueType string, env *object.Environment) bool {
	targetType = normalizeTypeName(targetType, env)
	valueType = normalizeTypeName(valueType, env)
	if targetType == "" {
		return true
	}
	if valueType == "null" {
		return true
	}

	if targetType == valueType {
		return true
	}

	targetMembers, targetIsUnion := splitTopLevelUnion(targetType)
	if targetIsUnion {
		for _, m := range targetMembers {
			if isAssignableToType(m, valueType, env) {
				return true
			}
		}
		return false
	}
	if valueMembers, valueIsUnion := splitTopLevelUnion(valueType); valueIsUnion {
		for _, m := range valueMembers {
			if !isAssignableToType(targetType, m, env) {
				return false
			}
		}
		return true
	}

	targetBase, targetDims, okTarget := parseTypeName(targetType)
	valueBase, valueDims, okValue := parseTypeName(valueType)
	if !okTarget || !okValue {
		return false
	}
	if len(targetDims) != len(valueDims) {
		return false
	}
	for i := range targetDims {
		if targetDims[i] == -1 {
			continue
		}
		if targetDims[i] != valueDims[i] {
			return false
		}
	}
	if len(targetDims) == 0 {
		return targetBase == valueBase
	}
	return isAssignableToType(targetBase, valueBase, env)
}

func isKnownTypeName(t string, env *object.Environment) bool {
	t = normalizeTypeName(t, env)
	base, _, ok := parseTypeName(t)
	if !ok {
		return false
	}
	if members, isUnion := splitTopLevelUnion(base); isUnion {
		for _, m := range members {
			if !isKnownTypeName(m, env) {
				return false
			}
		}
		return true
	}
	switch base {
	case "int", "float", "string", "char", "bool", "null", "type":
		return true
	default:
		return false
	}
}

func mergeTypeNames(a, b string, env *object.Environment) (string, bool) {
	a = normalizeTypeName(a, env)
	b = normalizeTypeName(b, env)
	if a == b {
		return a, true
	}
	baseA, dimsA, okA := parseTypeName(a)
	baseB, dimsB, okB := parseTypeName(b)
	if !okA || !okB {
		return "", false
	}
	if len(dimsA) != len(dimsB) {
		return "", false
	}
	merged := make([]int, len(dimsA))
	for i := range dimsA {
		if dimsA[i] == dimsB[i] {
			merged[i] = dimsA[i]
			continue
		}
		merged[i] = -1
	}
	mergedBase, ok := mergeUnionBases(baseA, baseB)
	if !ok {
		return "", false
	}
	return formatTypeName(mergedBase, merged), true
}

// parseTypeName parses scalar and array types such as:
// int, int[], int[3], int[][], int[][2]
// Returns base type name and per-dimension sizes where -1 means unsized ([]).
func parseTypeName(t string) (string, []int, bool) {
	if t == "" {
		return "", nil, false
	}
	base := t
	dimsRev := make([]int, 0, 2)
	for strings.HasSuffix(base, "]") {
		open := strings.LastIndexByte(base, '[')
		if open <= 0 {
			return "", nil, false
		}
		sizeLit := base[open+1 : len(base)-1]
		if sizeLit == "" {
			dimsRev = append(dimsRev, -1)
		} else {
			size, err := strconv.Atoi(sizeLit)
			if err != nil || size < 0 {
				return "", nil, false
			}
			dimsRev = append(dimsRev, size)
		}
		base = base[:open]
	}
	base = stripOuterParens(base)
	if base == "" {
		return "", nil, false
	}
	dims := make([]int, len(dimsRev))
	for i := range dimsRev {
		dims[len(dimsRev)-1-i] = dimsRev[i]
	}
	return base, dims, true
}

func formatTypeName(base string, dims []int) string {
	if len(dims) > 0 {
		if _, isUnion := splitTopLevelUnion(base); isUnion && !isWrappedInParens(base) {
			base = "(" + base + ")"
		}
	}
	out := base
	for _, d := range dims {
		if d == -1 {
			out += "[]"
			continue
		}
		out += fmt.Sprintf("[%d]", d)
	}
	return out
}

func normalizeTypeName(t string, env *object.Environment) string {
	resolved, ok := resolveTypeName(t, env, map[string]struct{}{})
	if !ok {
		return t
	}
	return resolved
}

func resolveTypeName(t string, env *object.Environment, visiting map[string]struct{}) (string, bool) {
	base, dims, ok := parseTypeName(t)
	if !ok {
		return "", false
	}
	resolvedBase, ok := resolveTypeBase(base, env, visiting)
	if !ok {
		return "", false
	}
	resolvedType := resolvedBase
	if len(dims) > 0 {
		rb, rd, ok := parseTypeName(resolvedBase)
		if !ok {
			return "", false
		}
		allDims := append(append([]int{}, rd...), dims...)
		resolvedType = formatTypeName(rb, allDims)
	}
	return resolvedType, true
}

func resolveTypeBase(base string, env *object.Environment, visiting map[string]struct{}) (string, bool) {
	base = stripOuterParens(base)
	if parts, isUnion := splitTopLevelUnion(base); isUnion {
		resolved := make([]string, 0, len(parts))
		for _, p := range parts {
			r, ok := resolveTypeBase(p, env, visiting)
			if !ok {
				return "", false
			}
			resolved = append(resolved, r)
		}
		return strings.Join(resolved, "||"), true
	}
	if alias, ok := env.TypeAlias(base); ok {
		if _, seen := visiting[base]; seen {
			return "", false
		}
		visiting[base] = struct{}{}
		defer delete(visiting, base)
		return resolveTypeName(alias, env, visiting)
	}
	return base, true
}

func isBuiltinTypeName(name string) bool {
	switch name {
	case "int", "float", "string", "char", "bool", "null", "type":
		return true
	default:
		return false
	}
}

func splitTopLevelUnion(t string) ([]string, bool) {
	s := stripOuterParens(t)
	parts := []string{}
	depth := 0
	start := 0
	found := false
	for i := 0; i < len(s)-1; i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case '|':
			if depth == 0 && s[i+1] == '|' {
				part := strings.TrimSpace(s[start:i])
				if part == "" {
					return nil, false
				}
				parts = append(parts, stripOuterParens(part))
				start = i + 2
				found = true
				i++
			}
		}
	}
	if !found {
		return nil, false
	}
	last := strings.TrimSpace(s[start:])
	if last == "" {
		return nil, false
	}
	parts = append(parts, stripOuterParens(last))
	return parts, true
}

func stripOuterParens(s string) string {
	out := strings.TrimSpace(s)
	for isWrappedInParens(out) {
		out = strings.TrimSpace(out[1 : len(out)-1])
	}
	return out
}

func isWrappedInParens(s string) bool {
	if len(s) < 2 || s[0] != '(' || s[len(s)-1] != ')' {
		return false
	}
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 && i != len(s)-1 {
				return false
			}
		}
		if depth < 0 {
			return false
		}
	}
	return depth == 0
}

func mergeUnionBases(a, b string) (string, bool) {
	listA := []string{stripOuterParens(a)}
	if parts, ok := splitTopLevelUnion(a); ok {
		listA = parts
	}
	listB := []string{stripOuterParens(b)}
	if parts, ok := splitTopLevelUnion(b); ok {
		listB = parts
	}
	merged := make([]string, 0, len(listA)+len(listB))
	seen := map[string]struct{}{}
	for _, x := range append(listA, listB...) {
		if x == "" {
			return "", false
		}
		if _, ok := seen[x]; ok {
			continue
		}
		seen[x] = struct{}{}
		merged = append(merged, x)
	}
	if len(merged) == 0 {
		return "", false
	}
	if len(merged) == 1 {
		return merged[0], true
	}
	return strings.Join(merged, "||"), true
}

func castToInt(args []object.Object) object.Object {
	if len(args) != 1 {
		return newError("int expects 1 argument, got=%d", len(args))
	}
	switch v := args[0].(type) {
	case *object.Integer:
		return &object.Integer{Value: v.Value}
	case *object.Float:
		return &object.Integer{Value: int64(v.Value)}
	case *object.Boolean:
		if v.Value {
			return &object.Integer{Value: 1}
		}
		return &object.Integer{Value: 0}
	case *object.Char:
		return &object.Integer{Value: int64(v.Value)}
	case *object.String:
		n, err := strconv.ParseInt(v.Value, 10, 64)
		if err != nil {
			return newError("cannot cast string to int: %q", v.Value)
		}
		return &object.Integer{Value: n}
	default:
		return newError("cannot cast %s to int", runtimeTypeName(args[0]))
	}
}

func castToFloat(args []object.Object) object.Object {
	if len(args) != 1 {
		return newError("float expects 1 argument, got=%d", len(args))
	}
	switch v := args[0].(type) {
	case *object.Float:
		return &object.Float{Value: v.Value}
	case *object.Integer:
		return &object.Float{Value: float64(v.Value)}
	case *object.Boolean:
		if v.Value {
			return &object.Float{Value: 1}
		}
		return &object.Float{Value: 0}
	case *object.Char:
		return &object.Float{Value: float64(v.Value)}
	case *object.String:
		n, err := strconv.ParseFloat(v.Value, 64)
		if err != nil {
			return newError("cannot cast string to float: %q", v.Value)
		}
		return &object.Float{Value: n}
	default:
		return newError("cannot cast %s to float", runtimeTypeName(args[0]))
	}
}

func castToString(args []object.Object) object.Object {
	if len(args) != 1 {
		return newError("string expects 1 argument, got=%d", len(args))
	}
	return &object.String{Value: args[0].Inspect()}
}

func castToChar(args []object.Object) object.Object {
	if len(args) != 1 {
		return newError("char expects 1 argument, got=%d", len(args))
	}
	switch v := args[0].(type) {
	case *object.Char:
		return &object.Char{Value: v.Value}
	case *object.Integer:
		return &object.Char{Value: rune(v.Value)}
	case *object.String:
		if len(v.Value) != 1 {
			return newError("cannot cast string to char: %q", v.Value)
		}
		return &object.Char{Value: rune(v.Value[0])}
	default:
		return newError("cannot cast %s to char", runtimeTypeName(args[0]))
	}
}

func castToBool(args []object.Object) object.Object {
	if len(args) != 1 {
		return newError("bool expects 1 argument, got=%d", len(args))
	}
	switch v := args[0].(type) {
	case *object.Boolean:
		return nativeBoolToBooleanObject(v.Value)
	case *object.Integer:
		return nativeBoolToBooleanObject(v.Value != 0)
	case *object.Float:
		return nativeBoolToBooleanObject(v.Value != 0)
	case *object.Char:
		return nativeBoolToBooleanObject(v.Value != 0)
	case *object.String:
		return nativeBoolToBooleanObject(v.Value != "")
	case *object.Null:
		return FALSE
	default:
		return newError("cannot cast %s to bool", runtimeTypeName(args[0]))
	}
}
