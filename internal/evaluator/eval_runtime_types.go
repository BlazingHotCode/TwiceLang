package evaluator

import (
	"fmt"
	"strconv"
	"strings"

	"twice/internal/ast"
	"twice/internal/object"
	"twice/internal/typesys"
)

func mapKeyEquals(a, b object.Object) bool {
	eq := evalInfixExpression("==", a, b)
	return eq == TRUE
}

func derefPointerObject(obj object.Object) (object.Object, *object.Error) {
	ptr, ok := obj.(*object.Pointer)
	if !ok {
		return obj, nil
	}
	if ptr == nil || ptr.Env == nil {
		return nil, newError("invalid pointer")
	}
	val, ok := ptr.Env.Get(ptr.Name)
	if !ok {
		return nil, newError("pointer target not found: %s", ptr.Name)
	}
	return val, nil
}

func evalIndexExpression(left object.Object, index object.Object) object.Object {
	if deref, err := derefPointerObject(left); err != nil {
		return err
	} else {
		left = deref
	}
	if mp, ok := left.(*object.Map); ok {
		keyType := runtimeTypeName(index)
		if !typesys.IsAssignableTypeName(mp.KeyType, keyType, nil) {
			return newError("cannot use %s as map key type %s", keyType, mp.KeyType)
		}
		for i := range mp.Keys {
			if mapKeyEquals(mp.Keys[i], index) {
				return mp.Values[i]
			}
		}
		return NULL
	}
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
	if list, ok := left.(*object.List); ok {
		if idx < 0 || idx >= len(list.Elements) {
			return newError("list index out of bounds: %d", idx)
		}
		return list.Elements[idx]
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
	if mp, ok := targetObj.(*object.Map); ok {
		key := Eval(node.Left.Index, env)
		if isError(key) {
			return key
		}
		keyType := runtimeTypeName(key)
		if !isAssignableToType(mp.KeyType, keyType, env) {
			return newError("cannot assign key type %s to %s", keyType, mp.KeyType)
		}
		val := Eval(node.Value, env)
		if isError(val) {
			return val
		}
		valType := runtimeTypeName(val)
		if !isAssignableToType(mp.ValueType, valType, env) {
			return newError("cannot assign %s to %s", valType, mp.ValueType)
		}
		for i := range mp.Keys {
			if mapKeyEquals(mp.Keys[i], key) {
				mp.Values[i] = val
				return val
			}
		}
		mp.Keys = append(mp.Keys, key)
		mp.Values = append(mp.Values, val)
		return val
	}
	arr, ok := targetObj.(*object.Array)
	if !ok {
		list, ok := targetObj.(*object.List)
		if !ok {
			return newError("indexed assignment target is not an array/list")
		}
		indexObj := Eval(node.Left.Index, env)
		if isError(indexObj) {
			return indexObj
		}
		idxInt, ok := indexObj.(*object.Integer)
		if !ok {
			return newError("list index must be int, got %s", runtimeTypeName(indexObj))
		}
		idx := int(idxInt.Value)
		if idx < 0 || idx >= len(list.Elements) {
			return newError("list index out of bounds: %d", idx)
		}
		val := Eval(node.Value, env)
		if isError(val) {
			return val
		}
		valType := runtimeTypeName(val)
		if !isAssignableToType(list.ElementType, valType, env) {
			return newError("cannot assign %s to %s", valType, list.ElementType)
		}
		list.Elements[idx] = val
		return val
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
	if methodFn, receiverArg, ok := resolveStructMethodTarget(node, obj, env); ok {
		args, namedArgs, argErr := evalCallArguments(node.Arguments, env)
		if argErr != nil {
			return argErr
		}
		args = append([]object.Object{receiverArg}, args...)
		return applyFunction(methodFn, args, namedArgs, nil)
	}
	if deref, err := derefPointerObject(obj); err != nil {
		return err
	} else {
		obj = deref
	}
	if node.NullSafe && obj == NULL {
		return NULL
	}

	switch node.Method.Value {
	case "length":
		if len(node.Arguments) != 0 {
			if node.NullSafe {
				return NULL
			}
			return newError("length expects 0 arguments, got=%d", len(node.Arguments))
		}
		arr, ok := obj.(*object.Array)
		if ok {
			return &object.Integer{Value: int64(len(arr.Elements))}
		}
		list, ok := obj.(*object.List)
		if ok {
			return &object.Integer{Value: int64(len(list.Elements))}
		}
		if mp, ok := obj.(*object.Map); ok {
			return &object.Integer{Value: int64(len(mp.Keys))}
		}
		if !ok {
			if node.NullSafe {
				return NULL
			}
			return newError("length is only supported on arrays/lists/maps")
		}
		return &object.Integer{Value: int64(len(arr.Elements))}
	case "append":
		list, ok := obj.(*object.List)
		if !ok {
			if node.NullSafe {
				return NULL
			}
			return newError("append is only supported on lists")
		}
		if len(node.Arguments) != 1 {
			if node.NullSafe {
				return NULL
			}
			return newError("append expects 1 argument, got=%d", len(node.Arguments))
		}
		val := Eval(node.Arguments[0], env)
		if isError(val) {
			return val
		}
		valType := runtimeTypeName(val)
		if valType == "null" && !typeAllowsNull(list.ElementType, env) {
			return newError("cannot append null to %s", list.ElementType)
		}
		if !isAssignableToType(list.ElementType, valType, env) {
			return newError("cannot assign %s to %s", valType, list.ElementType)
		}
		list.Elements = append(list.Elements, val)
		return NULL
	case "has":
		mp, ok := obj.(*object.Map)
		if !ok {
			if node.NullSafe {
				return NULL
			}
			return newError("has is only supported on maps")
		}
		if len(node.Arguments) != 1 {
			if node.NullSafe {
				return NULL
			}
			return newError("has expects 1 argument, got=%d", len(node.Arguments))
		}
		key := Eval(node.Arguments[0], env)
		if isError(key) {
			return key
		}
		keyType := runtimeTypeName(key)
		if !isAssignableToType(mp.KeyType, keyType, env) {
			return FALSE
		}
		for i := range mp.Keys {
			if mapKeyEquals(mp.Keys[i], key) {
				return TRUE
			}
		}
		return FALSE
	case "removeKey":
		mp, ok := obj.(*object.Map)
		if !ok {
			if node.NullSafe {
				return NULL
			}
			return newError("removeKey is only supported on maps")
		}
		if len(node.Arguments) != 1 {
			if node.NullSafe {
				return NULL
			}
			return newError("removeKey expects 1 argument, got=%d", len(node.Arguments))
		}
		key := Eval(node.Arguments[0], env)
		if isError(key) {
			return key
		}
		keyType := runtimeTypeName(key)
		if !isAssignableToType(mp.KeyType, keyType, env) {
			return NULL
		}
		for i := range mp.Keys {
			if mapKeyEquals(mp.Keys[i], key) {
				removed := mp.Values[i]
				mp.Keys = append(mp.Keys[:i], mp.Keys[i+1:]...)
				mp.Values = append(mp.Values[:i], mp.Values[i+1:]...)
				return removed
			}
		}
		return NULL
	case "pop":
		list, ok := obj.(*object.List)
		if !ok {
			if node.NullSafe {
				return NULL
			}
			return newError("pop is only supported on lists")
		}
		if len(node.Arguments) != 0 {
			if node.NullSafe {
				return NULL
			}
			return newError("pop expects 0 arguments, got=%d", len(node.Arguments))
		}
		if len(list.Elements) == 0 {
			return NULL
		}
		last := list.Elements[len(list.Elements)-1]
		list.Elements = list.Elements[:len(list.Elements)-1]
		return last
	case "clear":
		mp, ok := obj.(*object.Map)
		if ok {
			if len(node.Arguments) != 0 {
				if node.NullSafe {
					return NULL
				}
				return newError("clear expects 0 arguments, got=%d", len(node.Arguments))
			}
			mp.Keys = nil
			mp.Values = nil
			return NULL
		}
		list, ok := obj.(*object.List)
		if !ok {
			if node.NullSafe {
				return NULL
			}
			return newError("clear is only supported on lists")
		}
		if len(node.Arguments) != 0 {
			if node.NullSafe {
				return NULL
			}
			return newError("clear expects 0 arguments, got=%d", len(node.Arguments))
		}
		list.Elements = nil
		return NULL
	case "remove":
		list, ok := obj.(*object.List)
		if !ok {
			if node.NullSafe {
				return NULL
			}
			return newError("remove is only supported on lists")
		}
		if len(node.Arguments) != 1 {
			if node.NullSafe {
				return NULL
			}
			return newError("remove expects 1 argument, got=%d", len(node.Arguments))
		}
		rawIdx := Eval(node.Arguments[0], env)
		if isError(rawIdx) {
			return rawIdx
		}
		idxObj, ok := rawIdx.(*object.Integer)
		if !ok {
			return newError("remove index must be int, got %s", runtimeTypeName(rawIdx))
		}
		idx := int(idxObj.Value)
		if idx < 0 || idx >= len(list.Elements) {
			return newError("list index out of bounds: %d", idx)
		}
		removed := list.Elements[idx]
		list.Elements = append(list.Elements[:idx], list.Elements[idx+1:]...)
		return removed
	case "insert":
		list, ok := obj.(*object.List)
		if !ok {
			if node.NullSafe {
				return NULL
			}
			return newError("insert is only supported on lists")
		}
		if len(node.Arguments) != 2 {
			if node.NullSafe {
				return NULL
			}
			return newError("insert expects 2 arguments, got=%d", len(node.Arguments))
		}
		rawIdx := Eval(node.Arguments[0], env)
		if isError(rawIdx) {
			return rawIdx
		}
		idxObj, ok := rawIdx.(*object.Integer)
		if !ok {
			return newError("insert index must be int, got %s", runtimeTypeName(rawIdx))
		}
		idx := int(idxObj.Value)
		if idx < 0 || idx > len(list.Elements) {
			return newError("list index out of bounds: %d", idx)
		}
		val := Eval(node.Arguments[1], env)
		if isError(val) {
			return val
		}
		valType := runtimeTypeName(val)
		if valType == "null" && !typeAllowsNull(list.ElementType, env) {
			return newError("cannot insert null into %s", list.ElementType)
		}
		if !isAssignableToType(list.ElementType, valType, env) {
			return newError("cannot assign %s to %s", valType, list.ElementType)
		}
		list.Elements = append(list.Elements, NULL)
		copy(list.Elements[idx+1:], list.Elements[idx:])
		list.Elements[idx] = val
		return NULL
	case "contains":
		list, ok := obj.(*object.List)
		if !ok {
			if node.NullSafe {
				return NULL
			}
			return newError("contains is only supported on lists")
		}
		if len(node.Arguments) != 1 {
			if node.NullSafe {
				return NULL
			}
			return newError("contains expects 1 argument, got=%d", len(node.Arguments))
		}
		val := Eval(node.Arguments[0], env)
		if isError(val) {
			return val
		}
		valType := runtimeTypeName(val)
		if !isAssignableToType(list.ElementType, valType, env) && !isAssignableToType(valType, list.ElementType, env) {
			return NULL
		}
		for _, el := range list.Elements {
			if runtimeTypeName(el) != valType {
				continue
			}
			if eq := evalInfixExpression("==", el, val); eq == TRUE {
				return TRUE
			}
		}
		return FALSE
	default:
		if node.NullSafe {
			return NULL
		}
		return newError("unknown method: %s", node.Method.Value)
	}
}

func resolveStructMethodTarget(node *ast.MethodCallExpression, obj object.Object, env *object.Environment) (object.Object, object.Object, bool) {
	if node == nil || node.Method == nil || env == nil || obj == nil || obj == NULL {
		return nil, nil, false
	}
	// Prefer pointer receiver methods when the caller is a pointer.
	if ptr, ok := obj.(*object.Pointer); ok {
		targetType := normalizeTypeName(ptr.TargetType, env)
		if targetType != "" && targetType != "unknown" {
			if fn, ok := env.StructMethod("*"+targetType, node.Method.Value); ok {
				return fn, obj, true
			}
		}
	}
	deref, err := derefPointerObject(obj)
	if err != nil {
		return nil, nil, false
	}
	st, ok := deref.(*object.Struct)
	if !ok || st == nil {
		return nil, nil, false
	}
	if fn, ok := env.StructMethod(normalizeTypeName(st.TypeName, env), node.Method.Value); ok {
		return fn, deref, true
	}
	return nil, nil, false
}

func evalMemberAccess(obj object.Object, property *ast.Identifier, nullSafe bool) object.Object {
	if property == nil {
		if nullSafe {
			return NULL
		}
		return newError("invalid member access")
	}
	if deref, err := derefPointerObject(obj); err != nil {
		return err
	} else {
		obj = deref
	}
	if nullSafe && obj == NULL {
		return NULL
	}
	switch property.Value {
	case "length":
		arr, ok := obj.(*object.Array)
		if ok {
			return &object.Integer{Value: int64(len(arr.Elements))}
		}
		list, ok := obj.(*object.List)
		if ok {
			return &object.Integer{Value: int64(len(list.Elements))}
		}
		if mp, ok := obj.(*object.Map); ok {
			return &object.Integer{Value: int64(len(mp.Keys))}
		}
		if nullSafe {
			return NULL
		}
		return newError("length is only supported on arrays/lists/maps")
	default:
		if st, ok := obj.(*object.Struct); ok {
			if v, exists := st.Fields[property.Value]; exists {
				return v
			}
		}
		if nullSafe {
			return NULL
		}
		return newError("unknown member: %s", property.Value)
	}
}

func evalNewExpression(node *ast.NewExpression, env *object.Environment) object.Object {
	if node == nil {
		return newError("invalid new expression")
	}
	typeName := normalizeTypeName(node.TypeName, env)
	base, _, ok := parseTypeName(typeName)
	if !ok {
		return newError("invalid type in new expression: %s", node.TypeName)
	}
	gbase, gargs, isGeneric := typesys.SplitGenericType(base)
	if !isGeneric {
		if st, ok := env.Struct(base); ok && st != nil {
			fields := map[string]object.Object{}
			fieldTypes := map[string]string{}
			for _, f := range st.Fields {
				if f == nil || f.Name == nil {
					return newError("invalid struct field in %s", st.Name.Value)
				}
				fieldTypes[f.Name.Value] = f.TypeName
				if f.DefaultValue != nil {
					dv := Eval(f.DefaultValue, env)
					if isError(dv) {
						return dv
					}
					dt := runtimeTypeName(dv)
					if !isAssignableToType(f.TypeName, dt, env) {
						return newError("cannot assign %s to %s", dt, f.TypeName)
					}
					fields[f.Name.Value] = dv
				} else {
					fields[f.Name.Value] = NULL
				}
			}
			if len(node.Arguments) > len(st.Fields) {
				return newError("too many constructor arguments for %s", base)
			}
			used := map[string]struct{}{}
			named := false
			for _, arg := range node.Arguments {
				if _, ok := arg.(*ast.NamedArgument); ok {
					named = true
					break
				}
			}
			if named {
				for _, arg := range node.Arguments {
					na, ok := arg.(*ast.NamedArgument)
					if !ok {
						return newError("cannot mix named and positional constructor arguments")
					}
					ft, ok := fieldTypes[na.Name]
					if !ok {
						return newError("unknown field %s in %s", na.Name, base)
					}
					v := Eval(na.Value, env)
					if isError(v) {
						return v
					}
					vt := runtimeTypeName(v)
					if !isAssignableToType(ft, vt, env) {
						return newError("cannot assign %s to %s", vt, ft)
					}
					fields[na.Name] = v
					used[na.Name] = struct{}{}
				}
			} else {
				for i, arg := range node.Arguments {
					f := st.Fields[i]
					v := Eval(arg, env)
					if isError(v) {
						return v
					}
					vt := runtimeTypeName(v)
					if !isAssignableToType(f.TypeName, vt, env) {
						return newError("cannot assign %s to %s", vt, f.TypeName)
					}
					fields[f.Name.Value] = v
					used[f.Name.Value] = struct{}{}
				}
			}
			for _, f := range st.Fields {
				if f == nil || f.Name == nil {
					continue
				}
				if _, ok := used[f.Name.Value]; ok {
					continue
				}
				if f.DefaultValue != nil || f.Optional {
					continue
				}
				return newError("missing required field %s in %s constructor", f.Name.Value, base)
			}
			return &object.Struct{TypeName: base, Fields: fields}
		}
		return newError("new is only supported for List<T>, Map<K,V>, and structs")
	}
	if gbase == "List" {
		if len(gargs) != 1 {
			return newError("wrong number of generic type arguments for List: expected 1, got %d", len(gargs))
		}
		elemType := gargs[0]
		elems := make([]object.Object, 0, len(node.Arguments))
		for _, arg := range node.Arguments {
			val := Eval(arg, env)
			if isError(val) {
				return val
			}
			valType := runtimeTypeName(val)
			if valType == "null" && !typeAllowsNull(elemType, env) {
				return newError("cannot add null to %s", elemType)
			}
			if !isAssignableToType(elemType, valType, env) {
				return newError("cannot assign %s to %s", valType, elemType)
			}
			elems = append(elems, val)
		}
		return &object.List{
			ElementType: elemType,
			Elements:    elems,
		}
	}
	if gbase == "Map" {
		if len(gargs) != 2 {
			return newError("wrong number of generic type arguments for Map: expected 2, got %d", len(gargs))
		}
		keyType := gargs[0]
		valType := gargs[1]
		keys := make([]object.Object, 0, len(node.Arguments))
		vals := make([]object.Object, 0, len(node.Arguments))
		for _, arg := range node.Arguments {
			pair, ok := arg.(*ast.TupleLiteral)
			if !ok || len(pair.Elements) != 2 {
				return newError("map constructor arguments must be tuple pairs: (key, value)")
			}
			key := Eval(pair.Elements[0], env)
			if isError(key) {
				return key
			}
			value := Eval(pair.Elements[1], env)
			if isError(value) {
				return value
			}
			gotKeyType := runtimeTypeName(key)
			gotValType := runtimeTypeName(value)
			if !isAssignableToType(keyType, gotKeyType, env) {
				return newError("cannot assign key type %s to %s", gotKeyType, keyType)
			}
			if !isAssignableToType(valType, gotValType, env) {
				return newError("cannot assign %s to %s", gotValType, valType)
			}
			replaced := false
			for i := range keys {
				if mapKeyEquals(keys[i], key) {
					vals[i] = value
					replaced = true
					break
				}
			}
			if !replaced {
				keys = append(keys, key)
				vals = append(vals, value)
			}
		}
		return &object.Map{
			KeyType:   keyType,
			ValueType: valType,
			Keys:      keys,
			Values:    vals,
		}
	}
	return newError("new is only supported for List<T>, Map<K,V>, and structs")
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

func instantiateTypedArray(typeName string, env *object.Environment) (*object.Array, bool) {
	normalized := normalizeTypeName(typeName, env)
	base, dims, ok := parseTypeName(normalized)
	if !ok || len(dims) == 0 {
		return nil, false
	}
	return instantiateArrayFromDims(base, dims), true
}

func instantiateArrayFromDims(base string, dims []int) *object.Array {
	if len(dims) == 0 {
		return nil
	}
	count := dims[0]
	if count < 0 {
		count = 0
	}
	elemType := base
	if len(dims) > 1 {
		elemType = formatTypeName(base, dims[1:])
	}
	elements := make([]object.Object, count)
	if len(dims) > 1 {
		for i := 0; i < count; i++ {
			elements[i] = instantiateArrayFromDims(base, dims[1:])
		}
	} else {
		for i := 0; i < count; i++ {
			elements[i] = NULL
		}
	}
	return &object.Array{
		ElementType: elemType,
		Elements:    elements,
	}
}

func instantiateTypedTuple(typeName string, env *object.Environment) (*object.Tuple, bool) {
	normalized := normalizeTypeName(typeName, env)
	parts, ok := splitTopLevelTuple(normalized)
	if !ok {
		return nil, false
	}
	elements := make([]object.Object, len(parts))
	for i := range elements {
		elements[i] = NULL
	}
	return &object.Tuple{
		ElementTypes: parts,
		Elements:     elements,
	}, true
}

func pickTypedEmptyLiteralTarget(typeName string, wantArray bool, env *object.Environment) (string, bool) {
	normalized := normalizeTypeName(typeName, env)
	if normalized == "" || normalized == "unknown" {
		return "", false
	}
	if wantArray {
		if _, dims, ok := parseTypeName(normalized); ok && len(dims) > 0 {
			return normalized, true
		}
	} else if _, ok := splitTopLevelTuple(normalized); ok {
		return normalized, true
	}

	parts, isUnion := splitTopLevelUnion(normalized)
	if !isUnion {
		return "", false
	}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "null" {
			continue
		}
		if wantArray {
			if _, dims, ok := parseTypeName(part); ok && len(dims) > 0 {
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
	rtList
	rtMap
	rtTuple
	rtPointer
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
	case *object.List:
		return rtList
	case *object.Map:
		return rtMap
	case *object.Tuple:
		return rtTuple
	case *object.Pointer:
		return rtPointer
	case *object.Struct:
		return rtUnknown
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
	case rtList:
		v := obj.(*object.List)
		return "List<" + v.ElementType + ">"
	case rtMap:
		v := obj.(*object.Map)
		return "Map<" + v.KeyType + "," + v.ValueType + ">"
	case rtTuple:
		v := obj.(*object.Tuple)
		return "(" + strings.Join(v.ElementTypes, ",") + ")"
	case rtPointer:
		v := obj.(*object.Pointer)
		if strings.TrimSpace(v.TargetType) == "" {
			return "*unknown"
		}
		return "*" + v.TargetType
	case rtUnknown:
		if v, ok := obj.(*object.Struct); ok {
			return v.TypeName
		}
		return "unknown"
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
	if resolved, ok := resolveTypeName(targetType, env, map[string]struct{}{}); ok {
		targetType = resolved
	}
	if resolved, ok := resolveTypeName(valueType, env, map[string]struct{}{}); ok {
		valueType = resolved
	}
	return typesys.IsAssignableTypeName(targetType, valueType, typeAliasResolver(env))
}

func isKnownTypeName(t string, env *object.Environment) bool {
	return isKnownTypeNameWithParams(t, env, nil)
}

func isKnownTypeNameWithParams(t string, env *object.Environment, typeParams map[string]struct{}) bool {
	if resolved, ok := resolveTypeNameWithParams(t, env, map[string]struct{}{}, typeParams); ok {
		t = resolved
	}
	base, _, ok := parseTypeName(t)
	if !ok {
		return false
	}
	if members, ok := splitTopLevelUnion(base); ok {
		for _, m := range members {
			if !isKnownTypeNameWithParams(m, env, typeParams) {
				return false
			}
		}
		return true
	}
	if members, ok := splitTopLevelTuple(base); ok {
		for _, m := range members {
			if !isKnownTypeNameWithParams(m, env, typeParams) {
				return false
			}
		}
		return true
	}
	if gbase, gargs, ok := typesys.SplitGenericType(base); ok {
		if gbase == "List" {
			if len(gargs) != 1 {
				return false
			}
			return isKnownTypeNameWithParams(gargs[0], env, typeParams)
		}
		if gbase == "Map" {
			if len(gargs) != 2 {
				return false
			}
			return isKnownTypeNameWithParams(gargs[0], env, typeParams) && isKnownTypeNameWithParams(gargs[1], env, typeParams)
		}
		if _, ok := env.GenericTypeAlias(gbase); ok {
			for _, a := range gargs {
				if !isKnownTypeNameWithParams(a, env, typeParams) {
					return false
				}
			}
			return true
		}
		return false
	}
	if inner, ok := typesys.PeelPointerType(base); ok {
		inner = strings.TrimSpace(inner)
		if inner == "" {
			return false
		}
		return isKnownTypeNameWithParams(inner, env, typeParams)
	}
	if typeParams != nil {
		if _, ok := typeParams[base]; ok {
			return true
		}
	}
	if env != nil {
		if _, ok := env.Struct(base); ok {
			return true
		}
	}
	return typesys.IsBuiltinTypeName(base)
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
	resolved, ok := resolveTypeName(t, env, map[string]struct{}{})
	if !ok {
		return t
	}
	return resolved
}

func resolveTypeName(t string, env *object.Environment, _ map[string]struct{}) (string, bool) {
	return resolveTypeNameWithParams(t, env, map[string]struct{}{}, nil)
}

func resolveTypeNameWithParams(t string, env *object.Environment, visiting map[string]struct{}, typeParams map[string]struct{}) (string, bool) {
	base, dims, ok := parseTypeName(t)
	if !ok {
		return "", false
	}
	resolvedBase, ok := resolveTypeBaseWithParams(base, env, visiting, typeParams)
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

func resolveTypeBaseWithParams(base string, env *object.Environment, visiting map[string]struct{}, typeParams map[string]struct{}) (string, bool) {
	if inner, ok := typesys.PeelPointerType(base); ok {
		r, ok := resolveTypeBaseWithParams(inner, env, visiting, typeParams)
		if !ok {
			return "", false
		}
		return "*" + r, true
	}
	if members, ok := splitTopLevelUnion(base); ok {
		parts := make([]string, 0, len(members))
		for _, m := range members {
			r, ok := resolveTypeBaseWithParams(m, env, visiting, typeParams)
			if !ok {
				return "", false
			}
			parts = append(parts, r)
		}
		return strings.Join(parts, "||"), true
	}
	if members, ok := splitTopLevelTuple(base); ok {
		parts := make([]string, 0, len(members))
		for _, m := range members {
			r, ok := resolveTypeBaseWithParams(m, env, visiting, typeParams)
			if !ok {
				return "", false
			}
			parts = append(parts, r)
		}
		return "(" + strings.Join(parts, ",") + ")", true
	}
	if gbase, gargs, ok := typesys.SplitGenericType(base); ok {
		resolvedArgs := make([]string, 0, len(gargs))
		for _, a := range gargs {
			r, ok := resolveTypeNameWithParams(a, env, visiting, typeParams)
			if !ok {
				return "", false
			}
			resolvedArgs = append(resolvedArgs, r)
		}
		if alias, ok := env.GenericTypeAlias(gbase); ok {
			if len(alias.TypeParams) != len(resolvedArgs) {
				return "", false
			}
			mapping := map[string]string{}
			for i, tp := range alias.TypeParams {
				mapping[tp] = resolvedArgs[i]
			}
			inst := typesys.SubstituteTypeParams(alias.TypeName, mapping)
			return resolveTypeNameWithParams(inst, env, visiting, typeParams)
		}
		return gbase + "<" + strings.Join(resolvedArgs, ",") + ">", true
	}
	if typeParams != nil {
		if _, ok := typeParams[base]; ok {
			return base, true
		}
	}
	if alias, ok := env.TypeAlias(base); ok {
		if _, seen := visiting[base]; seen {
			return "", false
		}
		visiting[base] = struct{}{}
		defer delete(visiting, base)
		return resolveTypeNameWithParams(alias, env, visiting, typeParams)
	}
	return base, true
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

func isNonNullablePointerType(t string, env *object.Environment) bool {
	n := normalizeTypeName(t, env)
	if _, ok := typesys.PeelPointerType(n); !ok {
		return false
	}
	return !typeAllowsNull(n, env)
}

func genericTypeArityError(typeName string, env *object.Environment, typeParams map[string]struct{}) (string, bool) {
	base, _, ok := parseTypeName(typeName)
	if !ok {
		return "", false
	}
	if members, isUnion := splitTopLevelUnion(base); isUnion {
		for _, m := range members {
			if msg, ok := genericTypeArityError(m, env, typeParams); ok {
				return msg, true
			}
		}
		return "", false
	}
	if members, isTuple := splitTopLevelTuple(base); isTuple {
		for _, m := range members {
			if msg, ok := genericTypeArityError(m, env, typeParams); ok {
				return msg, true
			}
		}
		return "", false
	}
	if gb, args, ok := typesys.SplitGenericType(base); ok {
		if gb == "List" {
			if len(args) != 1 {
				return fmt.Sprintf("wrong number of generic type arguments for %s: expected %d, got %d", gb, 1, len(args)), true
			}
		}
		if gb == "Map" {
			if len(args) != 2 {
				return fmt.Sprintf("wrong number of generic type arguments for %s: expected %d, got %d", gb, 2, len(args)), true
			}
		}
		if _, exists := env.TypeAlias(gb); exists {
			return fmt.Sprintf("wrong number of generic type arguments for %s: expected %d, got %d", gb, 0, len(args)), true
		} else if alias, exists := env.GenericTypeAlias(gb); exists {
			if len(alias.TypeParams) != len(args) {
				return fmt.Sprintf("wrong number of generic type arguments for %s: expected %d, got %d", gb, len(alias.TypeParams), len(args)), true
			}
		}
		for _, a := range args {
			if msg, ok := genericTypeArityError(a, env, typeParams); ok {
				return msg, true
			}
		}
		return "", false
	}
	if inner, ok := typesys.PeelPointerType(base); ok {
		return genericTypeArityError(inner, env, typeParams)
	}
	if typeParams != nil {
		if _, ok := typeParams[base]; ok {
			return "", false
		}
	}
	if _, exists := env.TypeAlias(base); exists {
		return "", false
	}
	if base == "List" {
		return fmt.Sprintf("wrong number of generic type arguments for %s: expected %d, got %d", base, 1, 0), true
	}
	if base == "Map" {
		return fmt.Sprintf("wrong number of generic type arguments for %s: expected %d, got %d", base, 2, 0), true
	}
	if alias, exists := env.GenericTypeAlias(base); exists && len(alias.TypeParams) > 0 {
		return fmt.Sprintf("wrong number of generic type arguments for %s: expected %d, got %d", base, len(alias.TypeParams), 0), true
	}
	return "", false
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
