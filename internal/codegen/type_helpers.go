package codegen

import (
	"fmt"
	"strings"

	"twice/internal/typesys"
)

func parseTypeDescriptor(t string) (string, []int, bool) {
	return typesys.ParseTypeDescriptor(t)
}

func formatTypeDescriptor(base string, dims []int) string {
	return typesys.FormatTypeDescriptor(base, dims)
}

func peelArrayType(t string) (string, int, bool) {
	return typesys.PeelArrayType(t)
}

func withArrayDimension(elem string, n int) string {
	return typesys.WithArrayDimension(elem, n)
}

func mergeTypeNames(a, b string) (string, bool) {
	return typesys.MergeTypeNames(a, b, nil)
}

func isAssignableTypeName(target, value string) bool {
	return typesys.IsAssignableTypeName(target, value, nil)
}

func isKnownTypeName(t string) bool {
	return typesys.IsKnownTypeName(t, nil)
}

func splitTopLevelUnion(t string) ([]string, bool) {
	return typesys.SplitTopLevelUnion(t)
}

func splitTopLevelTuple(t string) ([]string, bool) {
	return typesys.SplitTopLevelTuple(t)
}

func stripOuterGroupingParens(s string) string {
	out := strings.TrimSpace(s)
	for len(out) >= 2 && out[0] == '(' && out[len(out)-1] == ')' {
		inner := strings.TrimSpace(out[1 : len(out)-1])
		if inner == "" {
			break
		}
		parts, isTuple := splitTopLevelTuple(out)
		if isTuple && len(parts) > 0 {
			break
		}
		out = inner
	}
	return out
}

func tupleMemberType(typeName string, idx int) (string, bool) {
	return typesys.TupleMemberType(typeName, idx)
}

func mergeUnionBases(a, b string) (string, bool) {
	if merged, ok := typesys.MergeTypeNames(a, b, nil); ok {
		base, dims, ok := typesys.ParseTypeDescriptor(merged)
		if ok && len(dims) == 0 {
			return base, true
		}
	}
	return "", false
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
