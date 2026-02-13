package diag

import "strings"

type CodeError struct {
	Message string
	Context string
	Line    int
	Column  int
}

func LocateContext(source string, context string) (line int, col int, ok bool) {
	ctx := strings.TrimSpace(context)
	if ctx == "" {
		return 0, 0, false
	}
	lines := strings.Split(source, "\n")
	normalize := func(s string) string {
		s = strings.TrimSpace(s)
		s = strings.ReplaceAll(s, " ", "")
		s = strings.ReplaceAll(s, "\t", "")
		return s
	}
	normalizedCtx := normalize(strings.Trim(ctx, "`"))

	matchLine := -1
	for i, ln := range lines {
		if normalize(ln) == normalizedCtx {
			if matchLine != -1 {
				matchLine = -2
				break
			}
			matchLine = i
		}
	}
	if matchLine >= 0 {
		ln := lines[matchLine]
		col := strings.Index(ln, strings.TrimSpace(strings.Trim(ctx, "`")))
		if col < 0 {
			col = strings.Index(ln, "return")
		}
		if col < 0 {
			col = 0
		}
		return matchLine + 1, col + 1, true
	}

	candidates := []string{ctx, strings.Trim(ctx, "`")}
	bestLine := -1
	bestCol := -1
	for i, ln := range lines {
		for _, c := range candidates {
			if c == "" {
				continue
			}
			if idx := strings.Index(ln, c); idx >= 0 {
				if bestLine != -1 {
					return 0, 0, false
				}
				bestLine = i + 1
				bestCol = idx + 1
			}
		}
	}
	if bestLine != -1 {
		return bestLine, bestCol, true
	}
	return 0, 0, false
}
