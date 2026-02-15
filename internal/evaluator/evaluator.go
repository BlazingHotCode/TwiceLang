package evaluator

import (
	"fmt"
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
	"print": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("print expects 1 argument, got=%d", len(args))
			}
			fmt.Print(args[0].Inspect())
			return NULL
		},
	},
	"println": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("println expects 1 argument, got=%d", len(args))
			}
			fmt.Println(args[0].Inspect())
			return NULL
		},
	},
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
	"hasField": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newError("hasField expects 2 arguments, got=%d", len(args))
			}
			field, ok := args[1].(*object.String)
			if !ok {
				return newError("hasField field must be string, got %s", runtimeTypeName(args[1]))
			}
			target := args[0]
			if deref, err := derefPointerObject(target); err != nil {
				return err
			} else {
				target = deref
			}

			switch obj := target.(type) {
			case *object.Array:
				return nativeBoolToBooleanObject(field.Value == "length" && obj != nil)
			case *object.List:
				return nativeBoolToBooleanObject(field.Value == "length" && obj != nil)
			case *object.Map:
				return nativeBoolToBooleanObject(field.Value == "length" && obj != nil)
			case *object.Struct:
				_, ok := obj.Fields[field.Value]
				return nativeBoolToBooleanObject(ok)
			case *object.String:
				return nativeBoolToBooleanObject(field.Value == "length" && obj != nil)
			default:
				return FALSE
			}
		},
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
		if node.ReturnValue == nil {
			return &object.ReturnValue{Value: NULL}
		}
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
		varType := node.TypeName
		if varType != "" {
			if msg, ok := genericTypeArityError(varType, env, nil); ok {
				return newError(msg)
			}
			if !isKnownTypeName(varType, env) {
				return newError("unknown type: %s", varType)
			}
		}
		var val object.Object = NULL
		if node.Value != nil {
			switch lit := node.Value.(type) {
			case *ast.ArrayLiteral:
				if len(lit.Elements) == 0 && varType != "" {
					if target, ok := pickTypedEmptyLiteralTarget(varType, true, env); ok {
						arr, _ := instantiateTypedArray(target, env)
						val = arr
					} else {
						return newError("empty array literal requires array type context")
					}
				} else {
					val = Eval(node.Value, env)
				}
			case *ast.TupleLiteral:
				if len(lit.Elements) == 0 && varType != "" {
					if target, ok := pickTypedEmptyLiteralTarget(varType, false, env); ok {
						tup, _ := instantiateTypedTuple(target, env)
						val = tup
					} else {
						return newError("empty tuple literal requires tuple type context")
					}
				} else {
					val = Eval(node.Value, env)
				}
			default:
				val = Eval(node.Value, env)
			}
			if isError(val) {
				return val
			}
		} else if varType != "" {
			if isNonNullablePointerType(varType, env) {
				return newError("pointer variable %s requires initializer unless nullable", node.Name.Value)
			}
			if _, isUnion := splitTopLevelUnion(normalizeTypeName(varType, env)); !isUnion {
				if arr, ok := instantiateTypedArray(varType, env); ok {
					val = arr
				} else if tup, ok := instantiateTypedTuple(varType, env); ok {
					val = tup
				}
			}
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
		varType := node.TypeName
		if varType != "" {
			if msg, ok := genericTypeArityError(varType, env, nil); ok {
				return newError(msg)
			}
			if !isKnownTypeName(varType, env) {
				return newError("unknown type: %s", varType)
			}
		}
		var val object.Object
		switch lit := node.Value.(type) {
		case *ast.ArrayLiteral:
			if len(lit.Elements) == 0 && varType != "" {
				if target, ok := pickTypedEmptyLiteralTarget(varType, true, env); ok {
					val, _ = instantiateTypedArray(target, env)
				} else {
					return newError("empty array literal requires array type context")
				}
			} else {
				val = Eval(node.Value, env)
			}
		case *ast.TupleLiteral:
			if len(lit.Elements) == 0 && varType != "" {
				if target, ok := pickTypedEmptyLiteralTarget(varType, false, env); ok {
					val, _ = instantiateTypedTuple(target, env)
				} else {
					return newError("empty tuple literal requires tuple type context")
				}
			} else {
				val = Eval(node.Value, env)
			}
		default:
			val = Eval(node.Value, env)
		}
		if isError(val) {
			return val
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
		if env.HasTypeAliasInCurrentScope(node.Name.Value) || env.HasGenericTypeAliasInCurrentScope(node.Name.Value) {
			return newError("type already declared: %s", node.Name.Value)
		}
		typeParamSet := make(map[string]struct{}, len(node.TypeParams))
		for _, tp := range node.TypeParams {
			typeParamSet[tp] = struct{}{}
		}
		if msg, ok := genericTypeArityError(node.TypeName, env, typeParamSet); ok {
			return newError(msg)
		}
		resolved, ok := resolveTypeNameWithParams(node.TypeName, env, map[string]struct{}{}, typeParamSet)
		if !ok || !isKnownTypeNameWithParams(resolved, env, typeParamSet) {
			return newError("unknown type: %s", node.TypeName)
		}
		if len(node.TypeParams) == 0 {
			env.SetTypeAlias(node.Name.Value, resolved)
		} else {
			env.SetGenericTypeAlias(node.Name.Value, object.GenericTypeAlias{
				TypeParams: append([]string{}, node.TypeParams...),
				TypeName:   resolved,
			})
		}
	case *ast.StructStatement:
		if node.Name == nil {
			return newError("invalid struct declaration")
		}
		if isBuiltinTypeName(node.Name.Value) || node.Name.Value == "type" {
			return newError("cannot redefine builtin type: %s", node.Name.Value)
		}
		if env.HasStructInCurrentScope(node.Name.Value) || env.HasTypeAliasInCurrentScope(node.Name.Value) || env.HasGenericTypeAliasInCurrentScope(node.Name.Value) {
			return newError("type already declared: %s", node.Name.Value)
		}
		seen := map[string]struct{}{}
		for _, f := range node.Fields {
			if f == nil || f.Name == nil {
				return newError("invalid struct field")
			}
			if _, exists := seen[f.Name.Value]; exists {
				return newError("duplicate struct field: %s", f.Name.Value)
			}
			seen[f.Name.Value] = struct{}{}
			if msg, ok := genericTypeArityError(f.TypeName, env, nil); ok {
				return newError(msg)
			}
			if !isKnownTypeName(f.TypeName, env) {
				return newError("unknown type: %s", f.TypeName)
			}
		}
		env.SetStruct(node.Name.Value, node)
	case *ast.AssignStatement:
		if !env.Has(node.Name.Value) {
			return newError("identifier not found: " + node.Name.Value)
		}
		if env.IsConst(node.Name.Value) {
			return newError("cannot reassign const: %s", node.Name.Value)
		}
		targetType, ok := env.TypeOf(node.Name.Value)
		if !ok {
			targetType = "unknown"
		}
		var val object.Object
		switch lit := node.Value.(type) {
		case *ast.ArrayLiteral:
			if len(lit.Elements) == 0 {
				if target, ok := pickTypedEmptyLiteralTarget(targetType, true, env); ok {
					val, _ = instantiateTypedArray(target, env)
				} else {
					return newError("empty array literal requires array type context")
				}
			} else {
				val = Eval(node.Value, env)
			}
		case *ast.TupleLiteral:
			if len(lit.Elements) == 0 {
				if target, ok := pickTypedEmptyLiteralTarget(targetType, false, env); ok {
					val, _ = instantiateTypedTuple(target, env)
				} else {
					return newError("empty tuple literal requires tuple type context")
				}
			} else {
				val = Eval(node.Value, env)
			}
		default:
			val = Eval(node.Value, env)
		}
		if isError(val) {
			return val
		}
		if targetType == "unknown" {
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
	case *ast.MemberAssignStatement:
		left := node.Left
		if left == nil || left.Object == nil || left.Property == nil {
			return newError("invalid member assignment")
		}
		if id, ok := left.Object.(*ast.Identifier); ok && env.IsConst(id.Value) {
			return newError("cannot reassign const: %s", id.Value)
		}
		objVal := Eval(left.Object, env)
		if isError(objVal) {
			return objVal
		}
		if deref, err := derefPointerObject(objVal); err != nil {
			return err
		} else {
			objVal = deref
		}
		st, ok := objVal.(*object.Struct)
		if !ok {
			return newError("member assignment is only supported on structs")
		}
		structDecl, ok := env.Struct(st.TypeName)
		if !ok || structDecl == nil {
			return newError("unknown struct type: %s", st.TypeName)
		}
		fieldType := ""
		found := false
		for _, f := range structDecl.Fields {
			if f != nil && f.Name != nil && f.Name.Value == left.Property.Value {
				fieldType = f.TypeName
				found = true
				break
			}
		}
		if !found {
			return newError("unknown member: %s", left.Property.Value)
		}
		val := Eval(node.Value, env)
		if isError(val) {
			return val
		}
		valType := runtimeTypeName(val)
		if !isAssignableToType(fieldType, valType, env) {
			return newError("cannot assign %s to %s", valType, fieldType)
		}
		st.Fields[left.Property.Value] = val
		return val
	case *ast.DerefAssignStatement:
		if node.Left == nil {
			return newError("invalid dereference assignment")
		}
		ptrObj := Eval(node.Left.Right, env)
		if isError(ptrObj) {
			return ptrObj
		}
		ptr, ok := ptrObj.(*object.Pointer)
		if !ok {
			return newError("left side of dereference assignment must be a pointer")
		}
		val := Eval(node.Value, env)
		if isError(val) {
			return val
		}
		if ptr.TargetType != "" && !isAssignableToType(ptr.TargetType, runtimeTypeName(val), env) {
			return newError("cannot assign %s to %s", runtimeTypeName(val), ptr.TargetType)
		}
		if ptr.Env == nil || !ptr.Env.Assign(ptr.Name, val) {
			return newError("pointer target not found: %s", ptr.Name)
		}
		return val
	case *ast.IndexAssignStatement:
		return annotateErrorWithNode(evalIndexAssignStatement(node, env), node)
	case *ast.FunctionStatement:
		fnObj := Eval(node.Function, env)
		if isError(fnObj) {
			return fnObj
		}
		if node.Receiver != nil {
			receiverType := normalizeTypeName(node.Receiver.TypeName, env)
			env.SetStructMethod(receiverType, node.Name.Value, fnObj)
			return nil
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
		return annotateErrorWithNode(evalArrayLiteral(node, env), node)
	case *ast.TupleLiteral:
		return annotateErrorWithNode(evalTupleLiteral(node, env), node)
	case *ast.NewExpression:
		return annotateErrorWithNode(evalNewExpression(node, env), node)

	case *ast.Boolean:
		return nativeBoolToBooleanObject(node.Value)

	case *ast.Identifier:
		return evalIdentifier(node, env)

	case *ast.PrefixExpression:
		if node.Operator == "&" {
			id, ok := node.Right.(*ast.Identifier)
			if !ok {
				return annotateErrorWithNode(newError("address-of operator requires identifier"), node)
			}
			if !env.Has(id.Value) {
				return annotateErrorWithNode(newError("identifier not found: %s", id.Value), node)
			}
			targetType, _ := env.TypeOf(id.Value)
			return &object.Pointer{Name: id.Value, Env: env, TargetType: targetType}
		}
		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return annotateErrorWithNode(evalPrefixExpression(node.Operator, right), node)

	case *ast.InfixExpression:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return annotateErrorWithNode(evalInfixExpression(node.Operator, left, right), node)
	case *ast.IndexExpression:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		index := Eval(node.Index, env)
		if isError(index) {
			return index
		}
		return annotateErrorWithNode(evalIndexExpression(left, index), node)

	case *ast.IfExpression:
		return annotateErrorWithNode(evalIfExpression(node, env), node)

	case *ast.FunctionLiteral:
		typeParamSet := make(map[string]struct{}, len(node.TypeParams))
		for _, tp := range node.TypeParams {
			typeParamSet[tp] = struct{}{}
		}
		for _, param := range node.Parameters {
			if param.TypeName != "" {
				if msg, ok := genericTypeArityError(param.TypeName, env, typeParamSet); ok {
					return newError(msg)
				}
				if !isKnownTypeNameWithParams(param.TypeName, env, typeParamSet) {
					return newError("unknown type: %s", param.TypeName)
				}
			}
		}
		if node.ReturnType != "" {
			if msg, ok := genericTypeArityError(node.ReturnType, env, typeParamSet); ok {
				return newError(msg)
			}
			if !isKnownTypeNameWithParams(node.ReturnType, env, typeParamSet) {
				return newError("unknown type: %s", node.ReturnType)
			}
		}
		name := ""
		if node.Name != nil {
			name = node.Name.Value
		}
		return &object.Function{
			Name:       name,
			TypeParams: append([]string{}, node.TypeParams...),
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
					return annotateErrorWithNode(applyFunction(builtin, args, namedArgs, node.TypeArguments), node)
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
		return annotateErrorWithNode(applyFunction(function, args, namedArgs, node.TypeArguments), node)
	case *ast.MethodCallExpression:
		return annotateErrorWithNode(evalMethodCallExpression(node, env), node)
	case *ast.MemberAccessExpression:
		obj := Eval(node.Object, env)
		if isError(obj) {
			return obj
		}
		return annotateErrorWithNode(evalMemberAccess(obj, node.Property, false), node)
	case *ast.TupleAccessExpression:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		return annotateErrorWithNode(evalTupleAccessExpression(left, node.Index), node)
	case *ast.NullSafeAccessExpression:
		obj := Eval(node.Object, env)
		if isError(obj) {
			return obj
		}
		if obj == NULL {
			return NULL
		}
		return annotateErrorWithNode(evalMemberAccess(obj, node.Property, true), node)
	}

	return nil
}

// evalProgram evaluates a list of statements
// Stops early if it hits a return or error
