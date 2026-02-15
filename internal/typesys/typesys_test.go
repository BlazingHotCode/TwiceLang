package typesys

import "testing"

func TestTypeDescriptorParsingAndFormatting(t *testing.T) {
	base, dims, ok := ParseTypeDescriptor("int[2][3]")
	if !ok || base != "int" || len(dims) != 2 || dims[0] != 2 || dims[1] != 3 {
		t.Fatalf("unexpected parse result: %q %v %v", base, dims, ok)
	}
	if got := FormatTypeDescriptor(base, dims); got != "int[2][3]" {
		t.Fatalf("FormatTypeDescriptor=%q", got)
	}
	if got := FormatTypeDescriptor("int||string", []int{2}); got != "(int||string)[2]" {
		t.Fatalf("union array format=%q", got)
	}
	if _, _, ok := ParseTypeDescriptor("int[-1]"); ok {
		t.Fatalf("expected invalid negative size parse")
	}
}

func TestArrayHelpers(t *testing.T) {
	elem, n, ok := PeelArrayType("int[4]")
	if !ok || elem != "int" || n != 4 {
		t.Fatalf("PeelArrayType unexpected: %q %d %v", elem, n, ok)
	}
	if got := WithArrayDimension("int", -1); got != "int[]" {
		t.Fatalf("WithArrayDimension unsized=%q", got)
	}
}

func TestResolveNormalizeKnownAndAssign(t *testing.T) {
	aliases := map[string]string{"Num": "int", "Pair": "(int,string)", "Nums": "Num[2]"}
	resolve := func(name string) (string, bool) {
		v, ok := aliases[name]
		return v, ok
	}

	if got, ok := NormalizeTypeName("Nums", resolve); !ok || got != "int[2]" {
		t.Fatalf("NormalizeTypeName=%q %v", got, ok)
	}
	if got, ok := ResolveTypeName("Pair", resolve, map[string]struct{}{}); !ok || got != "(int,string)" {
		t.Fatalf("ResolveTypeName=%q %v", got, ok)
	}
	if !IsBuiltinTypeName("int") || IsBuiltinTypeName("X") {
		t.Fatalf("IsBuiltinTypeName unexpected")
	}
	if !IsBuiltinTypeName("any") {
		t.Fatalf("expected any to be a builtin type")
	}
	if !IsKnownTypeName("Num", resolve) || !IsKnownTypeName("(Num,string)", resolve) {
		t.Fatalf("IsKnownTypeName unexpected")
	}
	if IsKnownTypeName("Num||Nope", resolve) {
		t.Fatalf("unknown union member should fail known-type check")
	}
	if !IsAssignableTypeName("int||string", "int", resolve) {
		t.Fatalf("assign union failed")
	}
	if !IsAssignableTypeName("(int,string)", "(int,string)", resolve) {
		t.Fatalf("assign tuple equality failed")
	}
	if IsAssignableTypeName("(int,string)", "(int,bool)", resolve) {
		t.Fatalf("tuple mismatch should fail")
	}
	if !IsAssignableTypeName("int[]", "int[3]", resolve) {
		t.Fatalf("unsized dim should accept sized")
	}
	if !IsAssignableTypeName("int||string", "int||string", resolve) {
		t.Fatalf("same union assign should pass")
	}
	if !IsAssignableTypeName("any", "int||string", resolve) {
		t.Fatalf("any target should accept union value")
	}
	if !IsAssignableTypeName("int", "any", resolve) {
		t.Fatalf("concrete target should accept any source for runtime-checked flow")
	}
	if !IsAssignableTypeName("int||string", "int||int", resolve) {
		t.Fatalf("union value members should each be assignable")
	}
	if IsAssignableTypeName("int", "int||string", resolve) {
		t.Fatalf("target int should not accept mixed union value")
	}
	if IsAssignableTypeName("int[2]", "int[3]", resolve) {
		t.Fatalf("different fixed dims should fail")
	}
	if !IsAssignableTypeName("int", "null", resolve) {
		t.Fatalf("null should be assignable by current rules")
	}
	if IsAssignableTypeName("int", "string", resolve) {
		t.Fatalf("unexpected assign success")
	}
}

func TestMergeAndNullHelpers(t *testing.T) {
	if got, ok := MergeTypeNames("int", "string", nil); !ok || got != "int||string" {
		t.Fatalf("MergeTypeNames union=%q %v", got, ok)
	}
	if got, ok := MergeTypeNames("(int,string)", "(int,int)", nil); !ok || got != "(int,string)||(int,int)" {
		t.Fatalf("Merge tuple=%q %v", got, ok)
	}
	if !TypeAllowsNull("int||null", nil) || TypeAllowsNull("int", nil) {
		t.Fatalf("TypeAllowsNull unexpected")
	}
	if _, ok := MergeTypeNames("int[2]", "string", nil); ok {
		t.Fatalf("merge with mismatched dims should fail")
	}
	if got, ok := MergeTypeNames("int[2]", "int[3]", nil); !ok || got != "int[]" {
		t.Fatalf("merge fixed dims mismatch should produce unsized dim, got=%q ok=%v", got, ok)
	}
}

func TestUnionTupleSplitsAndTupleMember(t *testing.T) {
	if parts, ok := SplitTopLevelUnion("(int||string)||bool"); !ok || len(parts) != 2 {
		t.Fatalf("SplitTopLevelUnion unexpected: %v %v", parts, ok)
	}
	if _, ok := SplitTopLevelUnion("int||"); ok {
		t.Fatalf("expected invalid union split")
	}
	if _, ok := SplitTopLevelUnion("int|string"); ok {
		t.Fatalf("single pipe should not be union split")
	}
	if parts, ok := SplitTopLevelTuple("(int,(string,bool),char)"); !ok || len(parts) != 3 {
		t.Fatalf("SplitTopLevelTuple unexpected: %v %v", parts, ok)
	}
	if _, ok := SplitTopLevelTuple("(int)"); ok {
		t.Fatalf("single-item paren is not tuple")
	}
	if got, ok := TupleMemberType("(int,string,bool)", 1); !ok || got != "string" {
		t.Fatalf("TupleMemberType=%q %v", got, ok)
	}
	if _, ok := TupleMemberType("(int,string)", 9); ok {
		t.Fatalf("expected tuple member OOB failure")
	}

	if parts, ok := SplitTopLevelUnion("Box<int||string>||bool"); !ok || len(parts) != 2 {
		t.Fatalf("SplitTopLevelUnion with generics unexpected: %v %v", parts, ok)
	}
}

func TestNormalizeCycleFails(t *testing.T) {
	aliases := map[string]string{"A": "B", "B": "A"}
	resolve := func(name string) (string, bool) { v, ok := aliases[name]; return v, ok }
	if _, ok := NormalizeTypeName("A", resolve); ok {
		t.Fatalf("expected cycle resolution failure")
	}
}

func TestParseInvalidDescriptorsAndHelpers(t *testing.T) {
	bad := []string{"", "[]", "int[]]", "int[a]", "int[-2]", "( )[2]"}
	for _, in := range bad {
		if _, _, ok := ParseTypeDescriptor(in); ok {
			t.Fatalf("expected ParseTypeDescriptor failure for %q", in)
		}
	}

	if _, ok := ResolveTypeName("Unknown", nil, map[string]struct{}{}); !ok {
		t.Fatalf("ResolveTypeName without resolver should keep base known as string")
	}

	if !isWrappedInParens("(a)") || isWrappedInParens("(a))") || isWrappedInParens("a") {
		t.Fatalf("isWrappedInParens unexpected")
	}
	if !hasTopLevelComma("a,b") || hasTopLevelComma("(a,b)") {
		t.Fatalf("hasTopLevelComma unexpected")
	}
	if got := stripOuterGroupingParens("((a))"); got != "a" {
		t.Fatalf("stripOuterGroupingParens=%q", got)
	}
	if got := WithArrayDimension("int", 3); got != "int[3]" {
		t.Fatalf("WithArrayDimension sized=%q", got)
	}
	if _, n, ok := PeelArrayType("int"); ok || n != 0 {
		t.Fatalf("PeelArrayType non-array should fail")
	}
}

func TestGenericTypeHelpers(t *testing.T) {
	base, args, ok := SplitGenericType("Pair<int, Box<string>>")
	if !ok || base != "Pair" || len(args) != 2 || args[0] != "int" || args[1] != "Box<string>" {
		t.Fatalf("SplitGenericType unexpected: %q %v %v", base, args, ok)
	}

	got := SubstituteTypeParams("(T,U)||Box<T>", map[string]string{"T": "int", "U": "string"})
	if got != "(int,string)||Box<int>" {
		t.Fatalf("SubstituteTypeParams=%q", got)
	}

	parts := SplitTopLevelComma("A<int>, B<(int,string)>, C")
	if len(parts) != 3 || parts[0] != "A<int>" || parts[1] != "B<(int,string)>" || parts[2] != "C" {
		t.Fatalf("SplitTopLevelComma unexpected: %v", parts)
	}
}
