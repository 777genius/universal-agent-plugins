package cursorhooks

import (
	"fmt"
	"path"
	"strings"
	"unicode/utf8"
)

// RenderArgv emits literal argv for the explicitly qualified user-hook grammar.
// It does not launch anything or neutralize output, failure or shell expansion
// using a wrapper. Source excerpts establish plugin-only root replacement at
// the hook service boundary, but the entire backend preprocessing source is
// unavailable here. Dollar/backtick tokens remain unsupported, including the
// dollar literal seen in the supplied narrow workspaceOpen spike.
func RenderArgv(shell ShellContract, argv []string) (string, error) {
	if shell != LinuxUserShell32212 {
		return "", fmt.Errorf("%w: native shell qualification required", ErrUnsupported)
	}
	if len(argv) == 0 || len(argv) > MaxArgs {
		return "", fmt.Errorf("%w: argv count", ErrUnsupported)
	}
	if err := absolutePath(argv[0]); err != nil {
		return "", err
	}
	tokens := make([]string, len(argv))
	total := 0
	for i, arg := range argv {
		if err := literalToken(arg); err != nil {
			return "", err
		}
		total += len(arg)
		if total > MaxArgvBytes {
			return "", fmt.Errorf("%w: argv size", ErrUnsupported)
		}
		tokens[i] = "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
	}
	return strings.Join(tokens, " "), nil
}

func literalToken(s string) error {
	if len(s) > MaxTokenBytes || !utf8.ValidString(s) || strings.ContainsAny(s, "\x00\r\n") {
		return fmt.Errorf("%w: token encoding or size", ErrUnsupported)
	}
	if strings.ContainsAny(s, "$`") {
		return fmt.Errorf("%w: unqualified variable or substitution literal", ErrUnsupported)
	}
	return nil
}

func absolutePath(s string) error {
	if err := literalToken(s); err != nil {
		return err
	}
	if s == "/" || !path.IsAbs(s) || strings.HasPrefix(s, "//") || path.Clean(s) != s {
		return fmt.Errorf("%w: explicit canonical absolute path required", ErrUnsupported)
	}
	return nil
}

func specArgv(spec HookSpec) []string {
	return []string{spec.Executable, "cursor-event", "stop", "--binding", spec.Selector}
}
