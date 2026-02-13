package codegen

import (
	"fmt"
	"strings"
)

func parseTypeDescriptor(t string) (string, []int, bool) {
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
			var size int
			if _, err := fmt.Sscanf(sizeLit, "%d", &size); err != nil || size < 0 {
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

func formatTypeDescriptor(base string, dims []int) string {
	if len(dims) > 0 {
		if _, isUnion := splitTopLevelUnion(base); isUnion && !isWrappedInParens(base) {
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

func peelArrayType(t string) (string, int, bool) {
	base, dims, ok := parseTypeDescriptor(t)
	if !ok || len(dims) == 0 {
		return "", 0, false
	}
	elem := formatTypeDescriptor(base, dims[:len(dims)-1])
	return elem, dims[len(dims)-1], true
}

func withArrayDimension(elem string, n int) string {
	if n < 0 {
		return formatTypeDescriptor(elem, []int{-1})
	}
	return formatTypeDescriptor(elem, []int{n})
}

func mergeTypeNames(a, b string) (string, bool) {
	if a == b {
		return a, true
	}
	baseA, dimsA, okA := parseTypeDescriptor(a)
	baseB, dimsB, okB := parseTypeDescriptor(b)
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
		return "", false
	}
	return formatTypeDescriptor(mergedBase, merged), true
}

func isAssignableTypeName(target, value string) bool {
	if target == "" || target == "unknown" {
		return true
	}
	if value == "null" {
		return true
	}
	if target == value {
		return true
	}
	if targetMembers, targetIsUnion := splitTopLevelUnion(target); targetIsUnion {
		for _, m := range targetMembers {
			if isAssignableTypeName(m, value) {
				return true
			}
		}
		return false
	}
	if valueMembers, valueIsUnion := splitTopLevelUnion(value); valueIsUnion {
		for _, m := range valueMembers {
			if !isAssignableTypeName(target, m) {
				return false
			}
		}
		return true
	}
	tb, td, okT := parseTypeDescriptor(target)
	vb, vd, okV := parseTypeDescriptor(value)
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
		targetTuple, targetIsTuple := splitTopLevelTuple(tb)
		valueTuple, valueIsTuple := splitTopLevelTuple(vb)
		if targetIsTuple || valueIsTuple {
			if !targetIsTuple || !valueIsTuple || len(targetTuple) != len(valueTuple) {
				return false
			}
			for i := range targetTuple {
				if !isAssignableTypeName(targetTuple[i], valueTuple[i]) {
					return false
				}
			}
			return true
		}
		return tb == vb
	}
	return isAssignableTypeName(tb, vb)
}

func isKnownTypeName(t string) bool {
	base, _, ok := parseTypeDescriptor(t)
	if !ok {
		return false
	}
	if parts, isUnion := splitTopLevelUnion(base); isUnion {
		for _, p := range parts {
			if !isKnownTypeName(p) {
				return false
			}
		}
		return true
	}
	if parts, isTuple := splitTopLevelTuple(base); isTuple {
		for _, p := range parts {
			if !isKnownTypeName(p) {
				return false
			}
		}
		return true
	}
	switch base {
	case "int", "bool", "float", "string", "char", "null", "type":
		return true
	default:
		return false
	}
}

func splitTopLevelUnion(t string) ([]string, bool) {
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

func stripOuterParens(s string) string {
	out := strings.TrimSpace(s)
	for isWrappedInParens(out) {
		out = strings.TrimSpace(out[1 : len(out)-1])
	}
	return out
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

func splitTopLevelTuple(t string) ([]string, bool) {
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

func tupleMemberType(typeName string, idx int) (string, bool) {
	parts, ok := splitTopLevelTuple(typeName)
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
	if parts, ok := splitTopLevelUnion(a); ok {
		listA = parts
	}
	listB := []string{stripOuterGroupingParens(b)}
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

func (cg *CodeGen) stringLabel(lit string) string {
	if label, ok := cg.stringLits[lit]; ok {
		return label
	}
	label := fmt.Sprintf("str_%d", len(cg.stringLits))
	cg.stringLits[lit] = label
	return label
}

func escapeAsmString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}
