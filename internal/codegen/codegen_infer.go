package codegen

import (
	"fmt"
	"math"
	"strings"

	"twice/internal/ast"
)

func (cg *CodeGen) inferExpressionType(expr ast.Expression) (out valueType) {
	if expr == nil {
		return typeUnknown
	}
	if cached, ok := cg.inferTypeCache[expr]; ok {
		return cached
	}
	defer func() {
		cg.inferTypeCache[expr] = out
	}()
	switch e := expr.(type) {
	case *ast.IntegerLiteral:
		return typeInt
	case *ast.FloatLiteral:
		return typeFloat
	case *ast.StringLiteral:
		return typeString
	case *ast.CharLiteral:
		return typeChar
	case *ast.NullLiteral:
		return typeNull
	case *ast.Boolean:
		return typeBool
	case *ast.Identifier:
		if _, ok := cg.intVals[e.Value]; ok {
			return typeInt
		}
		if _, ok := cg.floatVals[e.Value]; ok {
			return typeFloat
		}
		if _, ok := cg.charVals[e.Value]; ok {
			return typeChar
		}
		if _, ok := cg.stringVals[e.Value]; ok {
			return typeString
		}
		if cg.varIsNull[e.Value] {
			return typeNull
		}
		if t, ok := cg.varTypes[e.Value]; ok {
			return t
		}
		if tn, ok := cg.varTypeNames[e.Value]; ok {
			return cg.parseTypeName(tn)
		}
		if isTypeLiteralIdentifier(e.Value) {
			return typeType
		}
		return typeUnknown
	case *ast.ArrayLiteral:
		return typeArray
	case *ast.TupleLiteral:
		return typeUnknown
	case *ast.PrefixExpression:
		switch e.Operator {
		case "!":
			return typeBool
		case "-":
			t := cg.inferExpressionType(e.Right)
			if t == typeInt || t == typeFloat {
				return t
			}
		}
		return typeUnknown
	case *ast.InfixExpression:
		left := cg.inferExpressionType(e.Left)
		right := cg.inferExpressionType(e.Right)
		switch e.Operator {
		case "??":
			// Null-coalescing evaluates to the non-null operand type.
			if left == typeNull {
				return right
			}
			if right == typeNull {
				return left
			}
			if left == right {
				return left
			}
			if left == typeUnknown {
				return right
			}
			if right == typeUnknown {
				return left
			}
			return typeUnknown
		case "&&", "||", "^^":
			if left == typeBool && right == typeBool {
				return typeBool
			}
			return typeUnknown
		case "+":
			if left == typeInt && right == typeInt {
				return typeInt
			}
			if isNumericType(left) && isNumericType(right) && (left == typeFloat || right == typeFloat) {
				return typeFloat
			}
			if left == typeString && (right == typeString || right == typeInt || right == typeFloat || right == typeChar) {
				return typeString
			}
			if right == typeString && (left == typeString || left == typeInt || left == typeFloat || left == typeChar) {
				return typeString
			}
			if left == typeChar && right == typeChar {
				return typeChar
			}
			if left == typeChar && right == typeInt {
				return typeChar
			}
			return typeUnknown
		case "-", "*", "/", "%":
			if left == typeInt && right == typeInt {
				return typeInt
			}
			if isNumericType(left) && isNumericType(right) {
				return typeFloat
			}
			return typeUnknown
		case "&", "|", "^", "<<", ">>":
			if left == typeInt && right == typeInt {
				return typeInt
			}
			return typeUnknown
		case "<", ">", "==", "!=":
			return typeBool
		}
		return typeUnknown
	case *ast.IfExpression:
		if e.Alternative == nil {
			return typeUnknown
		}
		cons := cg.inferBlockType(e.Consequence)
		alt := cg.inferBlockType(e.Alternative)
		if cons == alt {
			return cons
		}
		return typeUnknown
	case *ast.CallExpression:
		if fl, ok := e.Function.(*ast.FunctionLiteral); ok {
			if fl.ReturnType == "" {
				if functionReturnsOnlyNull(fl.Body) {
					return typeNull
				}
				return typeUnknown
			}
			return cg.parseTypeName(fl.ReturnType)
		}
		if fn, ok := e.Function.(*ast.Identifier); ok {
			switch fn.Value {
			case "typeof", "typeofValue", "typeofvalue":
				return typeType
			case "hasField":
				return typeBool
			case "int":
				return typeInt
			case "float":
				return typeFloat
			case "string":
				return typeString
			case "char":
				return typeChar
			case "bool":
				return typeBool
			}
			if key, ok := cg.varFuncs[fn.Value]; ok {
				fl := cg.functions[key]
				if fl.Literal.ReturnType == "" {
					if functionReturnsOnlyNull(fl.Literal.Body) {
						return typeNull
					}
					return typeUnknown
				}
				return cg.parseTypeName(fl.Literal.ReturnType)
			}
			if key, ok := cg.funcByName[fn.Value]; ok {
				fl := cg.functions[key]
				if fl.Literal.ReturnType == "" {
					if functionReturnsOnlyNull(fl.Literal.Body) {
						return typeNull
					}
					return typeUnknown
				}
				return cg.parseTypeName(fl.Literal.ReturnType)
			}
		}
		return typeUnknown
	case *ast.IndexExpression:
		if cg.inferExpressionTypeName(e.Left) == "string" {
			return typeChar
		}
		elemTypeName, _, ok := peelArrayType(cg.inferExpressionTypeName(e.Left))
		if !ok {
			return typeUnknown
		}
		return cg.parseTypeName(elemTypeName)
	case *ast.MethodCallExpression:
		if e.Method != nil && e.Method.Value == "length" {
			return typeInt
		}
		if e.NullSafe {
			return typeNull
		}
		return typeUnknown
	case *ast.MemberAccessExpression:
		if e.Property != nil && e.Property.Value == "length" {
			return typeInt
		}
		return typeUnknown
	case *ast.NullSafeAccessExpression:
		if e.Property != nil && e.Property.Value == "length" {
			return typeInt
		}
		return typeNull
	case *ast.TupleAccessExpression:
		leftType := cg.inferCurrentValueTypeName(e.Left)
		if resolved, ok := cg.normalizeTypeName(leftType); ok {
			leftType = resolved
		}
		elem, ok := tupleMemberType(leftType, e.Index)
		if !ok {
			return typeUnknown
		}
		return cg.parseTypeName(elem)
	case *ast.NamedArgument:
		return cg.inferExpressionType(e.Value)
	default:
		return typeUnknown
	}
}

func (cg *CodeGen) inferExpressionTypeName(expr ast.Expression) (out string) {
	if expr == nil {
		return "unknown"
	}
	if cached, ok := cg.inferNameCache[expr]; ok {
		return cached
	}
	defer func() {
		cg.inferNameCache[expr] = out
	}()
	switch e := expr.(type) {
	case *ast.Identifier:
		if t, ok := cg.varTypeNames[e.Value]; ok && t != "" {
			return t
		}
		return typeName(cg.inferExpressionType(e))
	case *ast.ArrayLiteral:
		t, ok := cg.inferArrayLiteralTypeName(e)
		if !ok {
			return "unknown"
		}
		return t
	case *ast.TupleLiteral:
		if len(e.Elements) == 0 {
			return "unknown"
		}
		parts := make([]string, 0, len(e.Elements))
		for _, el := range e.Elements {
			parts = append(parts, cg.inferExpressionTypeName(el))
		}
		return "(" + strings.Join(parts, ",") + ")"
	case *ast.IndexExpression:
		if cg.inferExpressionTypeName(e.Left) == "string" {
			return "char"
		}
		elem, _, ok := peelArrayType(cg.inferExpressionTypeName(e.Left))
		if !ok {
			return "unknown"
		}
		return elem
	case *ast.MethodCallExpression:
		if e.Method != nil && e.Method.Value == "length" {
			return "int"
		}
		if e.NullSafe {
			return "null"
		}
		return "unknown"
	case *ast.MemberAccessExpression:
		if e.Property != nil && e.Property.Value == "length" {
			return "int"
		}
		return "unknown"
	case *ast.NullSafeAccessExpression:
		if e.Property != nil && e.Property.Value == "length" {
			return "int"
		}
		return "null"
	case *ast.TupleAccessExpression:
		leftType := cg.inferCurrentValueTypeName(e.Left)
		if resolved, ok := cg.normalizeTypeName(leftType); ok {
			leftType = resolved
		}
		elem, ok := tupleMemberType(leftType, e.Index)
		if !ok {
			return "unknown"
		}
		return elem
	case *ast.CallExpression:
		if fl, ok := e.Function.(*ast.FunctionLiteral); ok {
			if fl.ReturnType == "" {
				if functionReturnsOnlyNull(fl.Body) {
					return "null"
				}
				return "unknown"
			}
			return fl.ReturnType
		}
		if fn, ok := e.Function.(*ast.Identifier); ok && (fn.Value == "int" || fn.Value == "float" || fn.Value == "string" || fn.Value == "char" || fn.Value == "bool" || fn.Value == "typeof" || fn.Value == "typeofValue" || fn.Value == "typeofvalue") {
			return typeName(cg.inferExpressionType(e))
		}
	}
	return typeName(cg.inferExpressionType(expr))
}

func (cg *CodeGen) inferCurrentValueTypeName(expr ast.Expression) string {
	if id, ok := expr.(*ast.Identifier); ok {
		if t, ok := cg.varValueTypeName[id.Value]; ok && t != "" {
			return t
		}
	}
	return cg.inferExpressionTypeName(expr)
}

func (cg *CodeGen) inferArrayLiteralTypeName(al *ast.ArrayLiteral) (string, bool) {
	if al == nil || len(al.Elements) == 0 {
		return "", false
	}
	elemType := ""
	for _, el := range al.Elements {
		cur := cg.inferExpressionTypeName(el)
		if cur == "null" || cur == "unknown" {
			return "", false
		}
		if elemType == "" {
			elemType = cur
			continue
		}
		merged, ok := mergeTypeNames(elemType, cur)
		if !ok {
			return "", false
		}
		elemType = merged
	}
	return withArrayDimension(elemType, len(al.Elements)), true
}

func (cg *CodeGen) inferBlockType(block *ast.BlockStatement) valueType {
	if block == nil || len(block.Statements) == 0 {
		return typeUnknown
	}
	last := block.Statements[len(block.Statements)-1]
	switch s := last.(type) {
	case *ast.ExpressionStatement:
		return cg.inferExpressionType(s.Expression)
	case *ast.ReturnStatement:
		return cg.inferExpressionType(s.ReturnValue)
	default:
		return typeUnknown
	}
}

func functionReturnsOnlyNull(block *ast.BlockStatement) bool {
	if block == nil {
		return true
	}
	foundReturn := false
	for _, st := range block.Statements {
		switch s := st.(type) {
		case *ast.ReturnStatement:
			foundReturn = true
			if s.ReturnValue != nil {
				return false
			}
		case *ast.BlockStatement:
			if !functionReturnsOnlyNull(s) {
				return false
			}
		case *ast.ExpressionStatement:
			if ie, ok := s.Expression.(*ast.IfExpression); ok {
				if !functionReturnsOnlyNull(ie.Consequence) {
					return false
				}
				if ie.Alternative != nil && !functionReturnsOnlyNull(ie.Alternative) {
					return false
				}
			}
		}
	}
	return foundReturn
}

func (cg *CodeGen) inferTypeofType(expr ast.Expression) valueType {
	if id, ok := expr.(*ast.Identifier); ok {
		if declared, ok := cg.varDeclared[id.Value]; ok && declared != typeUnknown {
			return declared
		}
		if t, ok := cg.varTypes[id.Value]; ok && t != typeUnknown {
			return t
		}
	}
	return cg.inferExpressionType(expr)
}

func (cg *CodeGen) constStringValue(expr ast.Expression) (string, bool) {
	switch e := expr.(type) {
	case *ast.StringLiteral:
		return e.Value, true
	case *ast.Identifier:
		v, ok := cg.stringVals[e.Value]
		return v, ok
	case *ast.InfixExpression:
		if e.Operator != "+" {
			return "", false
		}
		left, ok := cg.constStringValue(e.Left)
		if !ok {
			return "", false
		}
		if right, ok := cg.constStringValue(e.Right); ok {
			return left + right, true
		}
		if right, ok := cg.constIntValue(e.Right); ok {
			return left + fmt.Sprintf("%d", right), true
		}
		if right, ok := cg.constFloatValue(e.Right); ok {
			return left + fmt.Sprintf("%g", right), true
		}
		if right, ok := cg.constCharValue(e.Right); ok {
			return left + string(right), true
		}
		return "", false
	default:
		return "", false
	}
}

func (cg *CodeGen) constFloatValue(expr ast.Expression) (float64, bool) {
	switch e := expr.(type) {
	case *ast.IntegerLiteral:
		return float64(e.Value), true
	case *ast.FloatLiteral:
		return e.Value, true
	case *ast.Identifier:
		if v, ok := cg.floatVals[e.Value]; ok {
			return v, true
		}
		if v, ok := cg.intVals[e.Value]; ok {
			return float64(v), true
		}
		return 0, false
	case *ast.PrefixExpression:
		if e.Operator != "-" {
			return 0, false
		}
		v, ok := cg.constFloatValue(e.Right)
		if !ok {
			return 0, false
		}
		return -v, true
	case *ast.InfixExpression:
		left, ok := cg.constFloatValue(e.Left)
		if !ok {
			return 0, false
		}
		right, ok := cg.constFloatValue(e.Right)
		if !ok {
			return 0, false
		}
		switch e.Operator {
		case "+":
			return left + right, true
		case "-":
			return left - right, true
		case "*":
			return left * right, true
		case "/":
			return left / right, true
		case "%":
			return math.Mod(left, right), true
		default:
			return 0, false
		}
	default:
		return 0, false
	}
}

func (cg *CodeGen) constIntValue(expr ast.Expression) (int64, bool) {
	switch e := expr.(type) {
	case *ast.IntegerLiteral:
		return e.Value, true
	case *ast.Identifier:
		v, ok := cg.intVals[e.Value]
		return v, ok
	case *ast.PrefixExpression:
		if e.Operator != "-" {
			return 0, false
		}
		v, ok := cg.constIntValue(e.Right)
		if !ok {
			return 0, false
		}
		return -v, true
	case *ast.InfixExpression:
		left, ok := cg.constIntValue(e.Left)
		if !ok {
			return 0, false
		}
		right, ok := cg.constIntValue(e.Right)
		if !ok {
			return 0, false
		}
		switch e.Operator {
		case "+":
			return left + right, true
		case "-":
			return left - right, true
		case "*":
			return left * right, true
		case "/":
			return left / right, true
		case "%":
			return left % right, true
		default:
			return 0, false
		}
	default:
		return 0, false
	}
}

func (cg *CodeGen) constCharValue(expr ast.Expression) (rune, bool) {
	switch e := expr.(type) {
	case *ast.CharLiteral:
		return e.Value, true
	case *ast.Identifier:
		v, ok := cg.charVals[e.Value]
		return v, ok
	case *ast.InfixExpression:
		if e.Operator == "+" {
			if left, ok := cg.constCharValue(e.Left); ok {
				if right, ok := cg.constCharValue(e.Right); ok {
					return left + right, true
				}
				if right, ok := cg.constIntValue(e.Right); ok {
					return left + rune(right), true
				}
			}
		}
		return 0, false
	default:
		return 0, false
	}
}

func (cg *CodeGen) trackKnownValue(name string, t valueType, expr ast.Expression) {
	delete(cg.intVals, name)
	delete(cg.charVals, name)
	delete(cg.stringVals, name)
	delete(cg.floatVals, name)
	if expr == nil {
		return
	}
	switch t {
	case typeInt:
		if v, ok := cg.constIntValue(expr); ok {
			cg.intVals[name] = v
		}
	case typeChar:
		if v, ok := cg.constCharValue(expr); ok {
			cg.charVals[name] = v
		}
	case typeString:
		if v, ok := cg.constStringValue(expr); ok {
			cg.stringVals[name] = v
		}
	case typeFloat:
		if v, ok := cg.constFloatValue(expr); ok {
			cg.floatVals[name] = v
		}
	}
}

func isNumericType(t valueType) bool {
	return t == typeInt || t == typeFloat
}

func (cg *CodeGen) parseTypeName(s string) valueType {
	if resolved, ok := cg.normalizeTypeName(s); ok {
		s = resolved
	}
	if _, dims, ok := parseTypeDescriptor(s); ok && len(dims) > 0 {
		return typeArray
	}
	switch s {
	case "int":
		return typeInt
	case "bool":
		return typeBool
	case "float":
		return typeFloat
	case "string":
		return typeString
	case "char":
		return typeChar
	case "type":
		return typeType
	case "null":
		return typeNull
	default:
		return typeUnknown
	}
}

func (cg *CodeGen) isAssignableTypeName(target, value string) bool {
	if resolved, ok := cg.normalizeTypeName(target); ok {
		target = resolved
	}
	if resolved, ok := cg.normalizeTypeName(value); ok {
		value = resolved
	}
	return isAssignableTypeName(target, value)
}

func (cg *CodeGen) isKnownTypeName(t string) bool {
	if resolved, ok := cg.normalizeTypeName(t); ok {
		t = resolved
	}
	return isKnownTypeName(t)
}

func (cg *CodeGen) typeAllowsNullTypeName(t string) bool {
	if resolved, ok := cg.normalizeTypeName(t); ok {
		t = resolved
	}
	if t == "null" {
		return true
	}
	if members, isUnion := splitTopLevelUnion(t); isUnion {
		for _, m := range members {
			if cg.typeAllowsNullTypeName(m) {
				return true
			}
		}
	}
	return false
}

func (cg *CodeGen) normalizeTypeName(t string) (string, bool) {
	return cg.resolveTypeName(t, map[string]struct{}{})
}

func (cg *CodeGen) resolveTypeName(t string, visiting map[string]struct{}) (string, bool) {
	base, dims, ok := parseTypeDescriptor(t)
	if !ok {
		return "", false
	}
	resolvedBase, ok := cg.resolveTypeBase(base, visiting)
	if !ok {
		return "", false
	}
	resolvedType := resolvedBase
	if len(dims) > 0 {
		rb, rd, ok := parseTypeDescriptor(resolvedBase)
		if !ok {
			return "", false
		}
		allDims := append(append([]int{}, rd...), dims...)
		resolvedType = formatTypeDescriptor(rb, allDims)
	}
	return resolvedType, true
}

func (cg *CodeGen) resolveTypeBase(base string, visiting map[string]struct{}) (string, bool) {
	base = stripOuterGroupingParens(base)
	if parts, isUnion := splitTopLevelUnion(base); isUnion {
		resolved := make([]string, 0, len(parts))
		for _, p := range parts {
			r, ok := cg.resolveTypeBase(p, visiting)
			if !ok {
				return "", false
			}
			resolved = append(resolved, r)
		}
		return strings.Join(resolved, "||"), true
	}
	if parts, isTuple := splitTopLevelTuple(base); isTuple {
		resolved := make([]string, 0, len(parts))
		for _, p := range parts {
			r, ok := cg.resolveTypeBase(p, visiting)
			if !ok {
				return "", false
			}
			resolved = append(resolved, r)
		}
		return "(" + strings.Join(resolved, ",") + ")", true
	}
	if alias, ok := cg.typeAliases[base]; ok {
		if _, seen := visiting[base]; seen {
			return "", false
		}
		visiting[base] = struct{}{}
		defer delete(visiting, base)
		return cg.resolveTypeName(alias, visiting)
	}
	return base, true
}

func isBuiltinTypeName(name string) bool {
	switch name {
	case "int", "bool", "float", "string", "char", "null", "type":
		return true
	default:
		return false
	}
}

func typeName(t valueType) string {
	switch t {
	case typeInt:
		return "int"
	case typeBool:
		return "bool"
	case typeFloat:
		return "float"
	case typeString:
		return "string"
	case typeChar:
		return "char"
	case typeNull:
		return "null"
	case typeType:
		return "type"
	case typeArray:
		return "array"
	default:
		return "unknown"
	}
}
