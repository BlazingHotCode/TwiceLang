package imports

import "strings"

const (
	ModuleTwiceMath = "twice.math"
)

func JoinPath(parts []string) string {
	return strings.Join(parts, ".")
}

func DefaultAlias(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func IsBuiltinPath(parts []string) bool {
	return len(parts) >= 1 && parts[0] == "twice"
}

func BuiltinMemberTarget(path string) (string, bool) {
	switch path {
	case "twice.math.abs", "twice.math.min", "twice.math.max", "twice.math.sqrt":
		return path, true
	default:
		return "", false
	}
}

func BuiltinNamespace(path string) bool {
	return path == ModuleTwiceMath
}
