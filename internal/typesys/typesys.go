package typesys

import (
	"fmt"
	"strconv"
	"strings"
)

type AliasResolver func(name string) (string, bool)

func ParseTypeDescriptor(t string) (string, []int, bool) {
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
	base = stripOuterGroupingParens(base)
	if base == "" {
		return "", nil, false
	}
	dims := make([]int, len(dimsRev))
	for i := range dimsRev {
		dims[len(dimsRev)-1-i] = dimsRev[i]
	}
	return base, dims, true
}

func FormatTypeDescriptor(base string, dims []int) string {
	if len(dims) > 0 {
		if _, isUnion := SplitTopLevelUnion(base); isUnion && !isWrappedInParens(base) {
			base = "(" + base + ")"
		}
	}
	out := base
	for _, d := range dims {
		if d < 0 {
			out += "[]"
		} else {
			out += fmt.Sprintf("[%d]", d)
		}
	}
	return out
}

func PeelArrayType(t string) (string, int, bool) {
	base, dims, ok := ParseTypeDescriptor(t)
	if !ok || len(dims) == 0 {
		return "", 0, false
	}
	elem := FormatTypeDescriptor(base, dims[:len(dims)-1])
	return elem, dims[len(dims)-1], true
}

func WithArrayDimension(elem string, n int) string {
	if n < 0 {
		return FormatTypeDescriptor(elem, []int{-1})
	}
	return FormatTypeDescriptor(elem, []int{n})
}

func NormalizeTypeName(t string, resolve AliasResolver) (string, bool) {
	return resolveWithMemo(t, resolve, map[string]struct{}{}, map[string]resolveMemoEntry{})
}

func ResolveTypeName(t string, resolve AliasResolver, visiting map[string]struct{}) (string, bool) {
	return resolveWithMemo(t, resolve, visiting, map[string]resolveMemoEntry{})
}

type resolveMemoEntry struct {
	value string
	ok    bool
}

func resolveWithMemo(t string, resolve AliasResolver, visiting map[string]struct{}, memo map[string]resolveMemoEntry) (string, bool) {
	if v, ok := memo[t]; ok {
		return v.value, v.ok
	}
	base, dims, ok := ParseTypeDescriptor(t)
	if !ok {
		memo[t] = resolveMemoEntry{"", false}
		return "", false
	}
	resolvedBase, ok := resolveTypeBase(base, resolve, visiting, memo)
	if !ok {
		memo[t] = resolveMemoEntry{"", false}
		return "", false
	}
	resolvedType := resolvedBase
	if len(dims) > 0 {
		rb, rd, ok := ParseTypeDescriptor(resolvedBase)
		if !ok {
			memo[t] = resolveMemoEntry{"", false}
			return "", false
		}
		allDims := append(append([]int{}, rd...), dims...)
		resolvedType = FormatTypeDescriptor(rb, allDims)
	}
	memo[t] = resolveMemoEntry{resolvedType, true}
	return resolvedType, true
}

func resolveTypeBase(base string, resolve AliasResolver, visiting map[string]struct{}, memo map[string]resolveMemoEntry) (string, bool) {
	base = stripOuterGroupingParens(base)
	if v, ok := memo["base:"+base]; ok {
		return v.value, v.ok
	}
	if parts, isUnion := SplitTopLevelUnion(base); isUnion {
		resolved := make([]string, 0, len(parts))
		for _, p := range parts {
			r, ok := resolveTypeBase(p, resolve, visiting, memo)
			if !ok {
				memo["base:"+base] = resolveMemoEntry{"", false}
				return "", false
			}
			resolved = append(resolved, r)
		}
		out := strings.Join(resolved, "||")
		memo["base:"+base] = resolveMemoEntry{out, true}
		return out, true
	}
	if parts, isTuple := SplitTopLevelTuple(base); isTuple {
		resolved := make([]string, 0, len(parts))
		for _, p := range parts {
			r, ok := resolveTypeBase(p, resolve, visiting, memo)
			if !ok {
				memo["base:"+base] = resolveMemoEntry{"", false}
				return "", false
			}
			resolved = append(resolved, r)
		}
		out := "(" + strings.Join(resolved, ",") + ")"
		memo["base:"+base] = resolveMemoEntry{out, true}
		return out, true
	}
	if resolve != nil {
		if alias, ok := resolve(base); ok {
			if _, seen := visiting[base]; seen {
				memo["base:"+base] = resolveMemoEntry{"", false}
				return "", false
			}
			visiting[base] = struct{}{}
			defer delete(visiting, base)
			out, ok := resolveWithMemo(alias, resolve, visiting, memo)
			memo["base:"+base] = resolveMemoEntry{out, ok}
			return out, ok
		}
	}
	memo["base:"+base] = resolveMemoEntry{base, true}
	return base, true
}

func IsBuiltinTypeName(name string) bool {
	switch name {
	case "int", "bool", "float", "string", "char", "null", "type":
		return true
	default:
		return false
	}
}

func IsKnownTypeName(t string, resolve AliasResolver) bool {
	if resolved, ok := NormalizeTypeName(t, resolve); ok {
		t = resolved
	}
	base, _, ok := ParseTypeDescriptor(t)
	if !ok {
		return false
	}
	if parts, isUnion := SplitTopLevelUnion(base); isUnion {
		for _, p := range parts {
			if !IsKnownTypeName(p, resolve) {
				return false
			}
		}
		return true
	}
	if parts, isTuple := SplitTopLevelTuple(base); isTuple {
		for _, p := range parts {
			if !IsKnownTypeName(p, resolve) {
				return false
			}
		}
		return true
	}
	return IsBuiltinTypeName(base)
}

func IsAssignableTypeName(target, value string, resolve AliasResolver) bool {
	if resolved, ok := NormalizeTypeName(target, resolve); ok {
		target = resolved
	}
	if resolved, ok := NormalizeTypeName(value, resolve); ok {
		value = resolved
	}
	if target == "" || target == "unknown" {
		return true
	}
	if value == "null" {
		return true
	}
	if target == value {
		return true
	}
	if targetMembers, targetIsUnion := SplitTopLevelUnion(target); targetIsUnion {
		for _, m := range targetMembers {
			if IsAssignableTypeName(m, value, resolve) {
				return true
			}
		}
		return false
	}
	if valueMembers, valueIsUnion := SplitTopLevelUnion(value); valueIsUnion {
		for _, m := range valueMembers {
			if !IsAssignableTypeName(target, m, resolve) {
				return false
			}
		}
		return true
	}
	tb, td, okT := ParseTypeDescriptor(target)
	vb, vd, okV := ParseTypeDescriptor(value)
	if !okT || !okV || len(td) != len(vd) {
		return false
	}
	for i := range td {
		if td[i] == -1 {
			continue
		}
		if td[i] != vd[i] {
			return false
		}
	}
	if len(td) == 0 {
		targetTuple, targetIsTuple := SplitTopLevelTuple(tb)
		valueTuple, valueIsTuple := SplitTopLevelTuple(vb)
		if targetIsTuple || valueIsTuple {
			if !targetIsTuple || !valueIsTuple || len(targetTuple) != len(valueTuple) {
				return false
			}
			for i := range targetTuple {
				if !IsAssignableTypeName(targetTuple[i], valueTuple[i], resolve) {
					return false
				}
			}
			return true
		}
		return tb == vb
	}
	return IsAssignableTypeName(tb, vb, resolve)
}

func MergeTypeNames(a, b string, resolve AliasResolver) (string, bool) {
	if resolved, ok := NormalizeTypeName(a, resolve); ok {
		a = resolved
	}
	if resolved, ok := NormalizeTypeName(b, resolve); ok {
		b = resolved
	}
	if a == b {
		return a, true
	}
	baseA, dimsA, okA := ParseTypeDescriptor(a)
	baseB, dimsB, okB := ParseTypeDescriptor(b)
	if !okA || !okB || len(dimsA) != len(dimsB) {
		return "", false
	}
	merged := make([]int, len(dimsA))
	for i := range dimsA {
		if dimsA[i] == dimsB[i] {
			merged[i] = dimsA[i]
		} else {
			merged[i] = -1
		}
	}
	mergedBase, ok := mergeUnionBases(baseA, baseB)
	if !ok {
		tupleA, okA := SplitTopLevelTuple(baseA)
		tupleB, okB := SplitTopLevelTuple(baseB)
		if !okA || !okB || len(tupleA) != len(tupleB) {
			return "", false
		}
		parts := make([]string, len(tupleA))
		for i := range tupleA {
			mt, ok := MergeTypeNames(tupleA[i], tupleB[i], resolve)
			if !ok {
				return "", false
			}
			parts[i] = mt
		}
		mergedBase = "(" + strings.Join(parts, ",") + ")"
		return FormatTypeDescriptor(mergedBase, merged), true
	}
	return FormatTypeDescriptor(mergedBase, merged), true
}

func TypeAllowsNull(t string, resolve AliasResolver) bool {
	if resolved, ok := NormalizeTypeName(t, resolve); ok {
		t = resolved
	}
	if t == "null" {
		return true
	}
	if members, isUnion := SplitTopLevelUnion(t); isUnion {
		for _, m := range members {
			if TypeAllowsNull(m, resolve) {
				return true
			}
		}
	}
	return false
}

func SplitTopLevelUnion(t string) ([]string, bool) {
	s := stripOuterGroupingParens(t)
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
				parts = append(parts, stripOuterGroupingParens(part))
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
	parts = append(parts, stripOuterGroupingParens(last))
	return parts, true
}

func stripOuterGroupingParens(s string) string {
	out := strings.TrimSpace(s)
	for isWrappedInParens(out) {
		inner := strings.TrimSpace(out[1 : len(out)-1])
		if hasTopLevelComma(inner) {
			break
		}
		out = inner
	}
	return out
}

func hasTopLevelComma(s string) bool {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				return true
			}
		}
	}
	return false
}

func SplitTopLevelTuple(t string) ([]string, bool) {
	s := strings.TrimSpace(t)
	if !isWrappedInParens(s) {
		return nil, false
	}
	inner := strings.TrimSpace(s[1 : len(s)-1])
	if inner == "" || !hasTopLevelComma(inner) {
		return nil, false
	}
	parts := []string{}
	depth := 0
	start := 0
	for i := 0; i < len(inner); i++ {
		switch inner[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				part := strings.TrimSpace(inner[start:i])
				if part == "" {
					return nil, false
				}
				parts = append(parts, part)
				start = i + 1
			}
		}
	}
	last := strings.TrimSpace(inner[start:])
	if last == "" {
		return nil, false
	}
	parts = append(parts, last)
	return parts, true
}

func TupleMemberType(typeName string, idx int) (string, bool) {
	parts, ok := SplitTopLevelTuple(typeName)
	if !ok || idx < 0 || idx >= len(parts) {
		return "", false
	}
	return parts[idx], true
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
	listA := []string{stripOuterGroupingParens(a)}
	if parts, ok := SplitTopLevelUnion(a); ok {
		listA = parts
	}
	listB := []string{stripOuterGroupingParens(b)}
	if parts, ok := SplitTopLevelUnion(b); ok {
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
