package evaluator

import (
	"math"
	"strconv"

	"twice/internal/ast"
	"twice/internal/object"
)

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
	if operator == "??" {
		if left == NULL {
			return right
		}
		return left
	}
	if operator == "&&" || operator == "||" || operator == "^^"{
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
