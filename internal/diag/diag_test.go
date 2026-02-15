package diag

import "testing"

func TestLocateContext(t *testing.T) {
	src := "let x = 1;\nreturn x;\nprint(x);\n"

	line, col, ok := LocateContext(src, "return x;")
	if !ok || line != 2 || col != 1 {
		t.Fatalf("exact locate got=(%d,%d,%v)", line, col, ok)
	}

	line, col, ok = LocateContext(src, "`return x;`")
	if !ok || line != 2 || col != 1 {
		t.Fatalf("backtick locate got=(%d,%d,%v)", line, col, ok)
	}

	if _, _, ok := LocateContext(src, ""); ok {
		t.Fatalf("empty context should fail")
	}
	if _, _, ok := LocateContext(src, "return x"); ok {
		t.Fatalf("duplicate-candidate behavior should fail for plain context")
	}
	if _, _, ok := LocateContext(src, "x"); ok {
		t.Fatalf("ambiguous context should fail")
	}
	if _, _, ok := LocateContext(src, "does not exist"); ok {
		t.Fatalf("missing context should fail")
	}
}
