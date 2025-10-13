package runner

import (
	"os"
	"os/exec"
	"strings"
)

// Obvious shell metacharacters. If any token contains one, run via shell.
var shellMeta = []string{
	"|", "||", "&", "&&", ";", "(", ")", "{", "}", "<", ">", ">>", "2>&1", "`", "$(", "${",
}

// shouldUseShell decides whether to execute the provided argv via a shell.
//
// Rules:
//   - If no args: false.
//   - If only 1 token (e.g., "ll"): true (allows aliases/functions).
//   - If argv[0] not found in PATH: true (likely alias/function).
//   - If any token contains obvious shell metacharacters: true.
//
// Otherwise -> exec directly.
func shouldUseShell(commandAndArgs []string) bool {
	if len(commandAndArgs) == 0 {
		return false
	}
	if len(commandAndArgs) == 1 {
		return true
	}
	if _, lookErr := exec.LookPath(commandAndArgs[0]); lookErr != nil {
		return true
	}
	for _, token := range commandAndArgs {
		for _, meta := range shellMeta {
			if strings.Contains(token, meta) {
				return true
			}
		}
	}
	return false
}

// shellJoin rebuilds a command line that the shell will parse into the same argv.
// It conservatively single-quotes each token and escapes inner single quotes.
func shellJoin(tokens []string) string {
	quoted := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if t == "" {
			quoted = append(quoted, "''")
			continue
		}
		// Replace ' with '\'' (close-quote, escaped quote, reopen).
		q := "'" + strings.ReplaceAll(t, "'", `'\''`) + "'"
		quoted = append(quoted, q)
	}
	return strings.Join(quoted, " ")
}

func pickUserShell() string {
	sh := os.Getenv("SHELL")
	if sh == "" {
		// POSIX fallback; -c works across shells.
		return "/bin/sh"
	}
	return sh
}
