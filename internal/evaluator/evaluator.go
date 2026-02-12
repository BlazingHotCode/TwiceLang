package evaluator

import (
	"fmt"
	"strconv"

	"twice/internal/ast"
	"twice/internal/object"
)

// TRUE and FALSE are singletons - we reuse these instead of creating new booleans
var (
	TRUE  = &object.Boolean{Value: true}
	FALSE = &object.Boolean{Value: false}
	NULL  = &object.Null{}
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
		if varType != "" && !isKnownTypeName(varType) {
			return newError("unknown type: %s", varType)
		}
		valType := runtimeTypeName(val)
		if varType == "" {
			varType = valType
		}
		if !isAssignableToType(varType, valType) {
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
		if varType != "" && !isKnownTypeName(varType) {
			return newError("unknown type: %s", varType)
		}
		valType := runtimeTypeName(val)
		if varType == "" {
			varType = valType
		}
		if !isAssignableToType(varType, valType) {
			return newError("cannot assign %s to %s", valType, varType)
		}
		env.SetConst(node.Name.Value, val)
		env.SetType(node.Name.Value, varType)
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
		valType := runtimeTypeName(val)
		if targetType == "null" && valType != "null" {
			targetType = valType
			env.SetType(node.Name.Value, targetType)
		}
		if !isAssignableToType(targetType, valType) {
			return newError("cannot assign %s to %s", valType, targetType)
		}
		env.Assign(node.Name.Value, val)

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

	case *ast.IfExpression:
		return evalIfExpression(node, env)

	case *ast.FunctionLiteral:
		params := node.Parameters
		body := node.Body
		return &object.Function{Parameters: params, Body: body, Env: env}

	case *ast.CallExpression:
		if ident, ok := node.Function.(*ast.Identifier); ok && ident.Value == "typeof" && len(node.Arguments) == 1 {
			if argIdent, ok := node.Arguments[0].(*ast.Identifier); ok {
				if t, ok := env.TypeOf(argIdent.Value); ok {
					return &object.TypeValue{Name: t}
				}
			}
		}
		function := Eval(node.Function, env)
		if isError(function) {
			return function
		}
		args := evalExpressions(node.Arguments, env)
		if len(args) == 1 && isError(args[0]) {
			return args[0]
		}
		return applyFunction(function, args)
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
		}
	}

	return result
}

// evalBlockStatement evaluates statements inside braces
// Unlike evalProgram, it does NOT unwrap ReturnValue
// This lets returns bubble up through nested blocks
func evalBlockStatement(block *ast.BlockStatement, env *object.Environment) object.Object {
	var result object.Object

	for _, statement := range block.Statements {
		result = Eval(statement, env)

		if result != nil {
			rt := result.Type()
			if rt == object.RETURN_VALUE_OBJ || rt == object.ERROR_OBJ {
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
	switch {
	case left.Type() == object.INTEGER_OBJ && right.Type() == object.INTEGER_OBJ:
		return evalIntegerInfixExpression(operator, left, right)
	case isNumeric(left) && isNumeric(right):
		return evalMixedNumericInfixExpression(operator, left, right)
	case left.Type() == object.FLOAT_OBJ && right.Type() == object.FLOAT_OBJ:
		return evalFloatInfixExpression(operator, left, right)
	case left.Type() == object.STRING_OBJ && right.Type() == object.STRING_OBJ:
		return evalStringInfixExpression(operator, left, right)
	case left.Type() == object.STRING_OBJ && operator == "+":
		return evalStringConcatWithCoercion(left, right)
	case left.Type() == object.CHAR_OBJ && right.Type() == object.CHAR_OBJ:
		return evalCharInfixExpression(operator, left, right)
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

// applyFunction calls a function with arguments
func applyFunction(fn object.Object, args []object.Object) object.Object {
	if builtin, ok := fn.(*object.Builtin); ok {
		return builtin.Fn(args...)
	}
	function, ok := fn.(*object.Function)
	if !ok {
		return newError("not a function: %s", fn.Type())
	}

	// Create new enclosed environment and bind parameters
	extendedEnv := extendFunctionEnv(function, args)
	// Evaluate function body in new environment
	evaluated := Eval(function.Body, extendedEnv)
	// Unwrap return value (functions return the value, not the ReturnValue object)
	return unwrapReturnValue(evaluated)
}

// extendFunctionEnv creates a new scope with parameters bound to arguments
func extendFunctionEnv(fn *object.Function, args []object.Object) *object.Environment {
	env := object.NewEnclosedEnvironment(fn.Env)

	for paramIdx, param := range fn.Parameters {
		env.Set(param.Value, args[paramIdx])
	}

	return env
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
	switch obj.(type) {
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
	case *object.Null:
		return "null"
	case *object.TypeValue:
		return "type"
	default:
		return "unknown"
	}
}

func isAssignableToType(targetType string, valueType string) bool {
	if targetType == "" {
		return true
	}
	if valueType == "null" {
		return true
	}
	return targetType == valueType
}

func isKnownTypeName(t string) bool {
	switch t {
	case "int", "float", "string", "char", "bool", "null", "type":
		return true
	default:
		return false
	}
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
