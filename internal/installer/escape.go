package installer

import (
	"strings"
)

// EscapeShellArg formats a path or argument safely for POSIX shell execution.
// It wraps the argument in single quotes, escaping any inner single quotes as '\”.
// Inside single quotes in POSIX shells, spaces, double quotes, dollar signs ($),
// backticks (`), and backslashes (\) retain their exact literal value.
func EscapeShellArg(arg string) string {
	if arg == "" {
		return "''"
	}
	escaped := strings.ReplaceAll(arg, "'", `'\''`)
	return "'" + escaped + "'"
}
