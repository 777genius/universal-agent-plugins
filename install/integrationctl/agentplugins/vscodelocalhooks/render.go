package vscodelocalhooks

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"unicode/utf8"
)

// RenderArgv returns a shell command for the exact target executor. All values
// are quoted as literals; no shell/native/environment interpolation is offered.
// The caller checks artifact identity, executable existence and target admission.
func RenderArgv(target Target, executable string, args []string) (string, error) {
	if err := validateTarget(target); err != nil {
		return "", err
	}
	if len(args) > MaxArgs {
		return "", fmt.Errorf("%w: too many arguments", ErrInvalid)
	}
	if err := validateExecutable(target.Shell, executable); err != nil {
		return "", err
	}
	values := append([]string{executable}, args...)
	quoted := make([]string, len(values))
	for i, value := range values {
		if err := validateLiteral(value); err != nil {
			return "", err
		}
		if target.Shell == WindowsPowerShell51 {
			if err := validateWindowsArgument(value); err != nil {
				return "", err
			}
			quoted[i] = "'" + strings.ReplaceAll(value, "'", "''") + "'"
		} else {
			quoted[i] = "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
		}
	}
	command := strings.Join(quoted, " ")
	if target.Shell == WindowsPowerShell51 {
		command = "& " + command
	}
	if len(command) > MaxCommandBytes {
		return "", fmt.Errorf("%w: command exceeds byte limit", ErrInvalid)
	}
	return command, nil
}

// Render emits a complete strict native hook file with one flat entry per
// requested event, timeout in seconds and only the selected platform command.
// It never emits a version, matcher, cwd, env, or cross-platform fallback.
// Inputs are consumed synchronously; no caller slices are retained.
func Render(target Target, specs []Spec) ([]byte, error) {
	if len(specs) == 0 || len(specs) > 2 {
		return nil, fmt.Errorf("%w: request one or two events", ErrInvalid)
	}
	hooks := make(map[Event][]map[string]any, len(specs))
	for _, spec := range specs {
		if spec.Event != Stop && spec.Event != SubagentStop {
			return nil, fmt.Errorf("%w: event", ErrUnsupported)
		}
		if _, exists := hooks[spec.Event]; exists || spec.TimeoutSeconds != TimeoutSeconds {
			return nil, fmt.Errorf("%w: duplicate event or timeout must be five seconds", ErrInvalid)
		}
		command, err := RenderArgv(target, spec.Executable, spec.Args)
		if err != nil {
			return nil, err
		}
		field := "linux"
		if target.Shell == WindowsPowerShell51 {
			field = "windows"
		}
		hooks[spec.Event] = []map[string]any{{"type": "command", field: command, "timeout": TimeoutSeconds}}
	}
	body, err := json.MarshalIndent(map[string]any{"hooks": hooks}, "", "  ")
	if err != nil {
		return nil, err
	}
	if len(body)+1 > MaxDocumentBytes {
		return nil, fmt.Errorf("%w: document exceeds byte limit", ErrInvalid)
	}
	return append(body, '\n'), nil
}

func validateLiteral(value string) error {
	if len(value) > MaxLiteralBytes || !utf8.ValidString(value) || strings.ContainsAny(value, "\x00\r\n") {
		return fmt.Errorf("%w: literal byte limit, UTF-8, NUL or CR/LF", ErrInvalid)
	}
	// The fixed-projection contract requires the caller to resolve declared
	// root/data references once. The standard AgentPlugin namespace does not
	// perform legacy plugin-root interpolation, even inside quotes.
	for _, token := range []string{"${PLUGIN_ROOT}", "${PLUGIN_DATA}", "${CLAUDE_PLUGIN_ROOT}"} {
		if strings.Contains(value, token) {
			return fmt.Errorf("%w: resolve portable/native placeholders before rendering", ErrUnsupported)
		}
	}
	return nil
}

func validateExecutable(shell Shell, executable string) error {
	if err := validateLiteral(executable); err != nil {
		return err
	}
	if shell == LinuxSH {
		if !path.IsAbs(executable) || executable == "/" || strings.HasSuffix(executable, "/") {
			return fmt.Errorf("%w: require an absolute executable path", ErrInvalid)
		}
		return nil
	}
	if !windowsAbsolute(executable) || !strings.HasSuffix(strings.ToLower(executable), ".exe") {
		return fmt.Errorf("%w: require an absolute Windows .exe path", ErrUnsupported)
	}
	return nil
}
