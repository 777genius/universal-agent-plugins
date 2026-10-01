package geminihooks

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Exact variable syntax used by Gemini CLI 0.62.0 before shell execution.
var interpolation = regexp.MustCompile(`\$(?:(\w+)|\{([^}]+?)(?::-([^}]*))?\})`)

func safeString(s string) error {
	if !utf8.ValidString(s) || strings.ContainsAny(s, "\x00\r\n") {
		return fmt.Errorf("%w: invalid UTF-8 or NUL/newline", ErrConflict)
	}
	if interpolation.MatchString(s) {
		return fmt.Errorf("%w: unsupported settings interpolation", ErrConflict)
	}
	return nil
}

// RenderArgv emits one fixed invocation for the caller-qualified native shell.
// Gemini expands settings variables before shell quoting, so variable-like
// tokens are rejected. PowerShell uses the common safe subset of 7 and 5.1:
// empty arguments and embedded double quotes cannot be qualified on 5.1.
func RenderArgv(shell Shell, argv []string) (string, error) {
	if shell != Bash && shell != PowerShell {
		return "", fmt.Errorf("%w: unsupported target shell", ErrConflict)
	}
	if len(argv) == 0 || argv[0] == "" {
		return "", fmt.Errorf("%w: missing executable", ErrConflict)
	}
	tokens := make([]string, len(argv))
	for i, arg := range argv {
		if err := safeString(arg); err != nil {
			return "", err
		}
		if shell == Bash {
			tokens[i] = "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
		} else {
			if arg == "" || strings.Contains(arg, `"`) {
				return "", fmt.Errorf("%w: unsupported PowerShell native argument", ErrConflict)
			}
			tokens[i] = "'" + strings.ReplaceAll(arg, "'", "''") + "'"
		}
	}
	command := strings.Join(tokens, " ")
	if shell == PowerShell {
		command = "& " + command
	} else {
		command = "exec -- " + command
	}
	// Quoting apostrophes can introduce new regex boundaries. Check the final
	// native settings string too, rather than assuming token checks suffice.
	if err := safeString(command); err != nil {
		return "", err
	}
	return command, nil
}

// Keep the generic invocation/validation unchanged. An observer may report
// nothing to the model, even after its executable is removed while Gemini
// still holds a cached command. Native timeout enforcement remains unchanged.
func renderCommand(shell Shell, argv []string, observer bool) (string, error) {
	command, err := RenderArgv(shell, argv)
	if err != nil || !observer {
		return command, err
	}
	if shell == Bash {
		// exec stays inside the subshell so failure cannot replace/exit the
		// outer shell. The guard also handles a caller's errexit setting.
		command = "(" + command + ") >/dev/null 2>&1 || :; printf '{}\\n'; exit 0"
	} else {
		// Consume all PowerShell streams, catch invocation errors, and exit
		// before Gemini's appended LASTEXITCODE check. Console.Write emits LF
		// exactly on Windows too. No settings-variable tokens are introduced.
		command = "try { " + command + " *>&1 | Out-Null } catch {}; [Console]::Out.Write(\"{}`n\"); exit 0"
	}
	if err := safeString(command); err != nil {
		return "", err
	}
	return command, nil
}
