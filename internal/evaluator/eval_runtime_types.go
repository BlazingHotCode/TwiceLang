package evaluator

import (
	"fmt"
	"strconv"
	"strings"

	"twice/internal/ast"
	"twice/internal/object"
	"twice/internal/typesys"
)

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

	if node.NullSafe && obj == NULL {
		return NULL
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

func evalTupleLiteral(lit *ast.TupleLiteral, env *object.Environment) object.Object {
	if len(lit.Elements) == 0 {
		return newError("empty tuple literals are not supported")
	}
	elements := make([]object.Object, 0, len(lit.Elements))
	types := make([]string, 0, len(lit.Elements))
	for _, el := range lit.Elements {
		val := Eval(el, env)
		if isError(val) {
			return val
		}
		elements = append(elements, val)
		types = append(types, runtimeTypeName(val))
	}
	return &object.Tuple{
		ElementTypes: types,
		Elements:     elements,
	}
}

func evalTupleAccessExpression(left object.Object, idx int) object.Object {
	tup, ok := left.(*object.Tuple)
	if !ok {
		return newError("tuple access is only supported on tuples")
	}
	if idx < 0 || idx >= len(tup.Elements) {
		return newError("tuple index out of bounds: %d", idx)
	}
	return tup.Elements[idx]
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

type runtimeTypeKind int

const (
	rtUnknown runtimeTypeKind = iota
	rtInt
	rtFloat
	rtString
	rtChar
	rtBool
	rtArray
	rtTuple
	rtNull
	rtType
)

func runtimeTypeKindOf(obj object.Object) runtimeTypeKind {
	switch obj.(type) {
	case *object.Integer:
		return rtInt
	case *object.Float:
		return rtFloat
	case *object.String:
		return rtString
	case *object.Char:
		return rtChar
	case *object.Boolean:
		return rtBool
	case *object.Array:
		return rtArray
	case *object.Tuple:
		return rtTuple
	case *object.Null:
		return rtNull
	case *object.TypeValue:
		return rtType
	default:
		return rtUnknown
	}
}

func runtimeTypeName(obj object.Object) string {
	switch runtimeTypeKindOf(obj) {
	case rtInt:
		return "int"
	case rtFloat:
		return "float"
	case rtString:
		return "string"
	case rtChar:
		return "char"
	case rtBool:
		return "bool"
	case rtArray:
		v := obj.(*object.Array)
		return formatTypeName(v.ElementType, []int{len(v.Elements)})
	case rtTuple:
		v := obj.(*object.Tuple)
		return "(" + strings.Join(v.ElementTypes, ",") + ")"
	case rtNull:
		return "null"
	case rtType:
		return "type"
	default:
		return "unknown"
	}
}

func typeAliasResolver(env *object.Environment) typesys.AliasResolver {
	if env == nil {
		return nil
	}
	return func(name string) (string, bool) {
		return env.TypeAlias(name)
	}
}

func isAssignableToType(targetType string, valueType string, env *object.Environment) bool {
	return typesys.IsAssignableTypeName(targetType, valueType, typeAliasResolver(env))
}

func isKnownTypeName(t string, env *object.Environment) bool {
	return typesys.IsKnownTypeName(t, typeAliasResolver(env))
}

func mergeTypeNames(a, b string, env *object.Environment) (string, bool) {
	return typesys.MergeTypeNames(a, b, typeAliasResolver(env))
}

func parseTypeName(t string) (string, []int, bool) {
	return typesys.ParseTypeDescriptor(t)
}

func formatTypeName(base string, dims []int) string {
	return typesys.FormatTypeDescriptor(base, dims)
}

func normalizeTypeName(t string, env *object.Environment) string {
	resolved, ok := typesys.NormalizeTypeName(t, typeAliasResolver(env))
	if !ok {
		return t
	}
	return resolved
}

func resolveTypeName(t string, env *object.Environment, _ map[string]struct{}) (string, bool) {
	return typesys.NormalizeTypeName(t, typeAliasResolver(env))
}

func isBuiltinTypeName(name string) bool {
	return typesys.IsBuiltinTypeName(name)
}

func typeAllowsNull(t string, env *object.Environment) bool {
	return typesys.TypeAllowsNull(t, typeAliasResolver(env))
}

func splitTopLevelUnion(t string) ([]string, bool) {
	return typesys.SplitTopLevelUnion(t)
}

func splitTopLevelTuple(t string) ([]string, bool) {
	return typesys.SplitTopLevelTuple(t)
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
