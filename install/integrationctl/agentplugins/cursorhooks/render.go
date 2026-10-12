package cursorhooks

import (
	"fmt"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

// RenderArgv emits literal argv for the explicitly qualified user-hook grammar.
// It does not launch anything or neutralize output, failure or shell expansion
// using a wrapper. Source excerpts establish plugin-only root replacement at
// the hook service boundary, but the entire backend preprocessing source is
// unavailable here. Dollar/backtick tokens remain unsupported, including the
// dollar literal seen in the supplied narrow workspaceOpen spike.
func RenderArgv(shell ShellContract, argv []string) (string, error) {
	if shell == WindowsUserPowerShell20261001 {
		return renderWindowsStop(argv)
	}
	if shell != LinuxUserShell32212 && shell != DarwinUserShell20260918 {
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

// Windows qualifies the fixed native Stop invocation only. Restricting its
// shape avoids promising generic PowerShell 5 native-argument behavior.
func renderWindowsStop(argv []string) (string, error) {
	if len(argv) != 5 || argv[1] != "cursor-event" || argv[2] != "stop" || argv[3] != "--binding" {
		return "", fmt.Errorf("%w: fixed Windows Stop argv required", ErrUnsupported)
	}
	if err := nativeAbsolutePath(WindowsUserPowerShell20261001, argv[0]); err != nil {
		return "", err
	}
	if err := nativeAbsolutePath(WindowsUserPowerShell20261001, argv[4]); err != nil {
		return "", err
	}
	if !strings.HasSuffix(strings.ToLower(argv[0]), ".exe") {
		return "", fmt.Errorf("%w: Windows observer must be an exe", ErrUnsupported)
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
		tokens[i] = "'" + strings.ReplaceAll(arg, "'", "''") + "'"
	}
	return "& " + strings.Join(tokens, " "), nil
}

// This is spelling validation, never filesystem authority. The host still
// verifies no-follow bytes and the captured profile identity before effects.
func nativeAbsolutePath(shell ShellContract, s string) error {
	if shell != WindowsUserPowerShell20261001 {
		return absolutePath(s)
	}
	if err := literalToken(s); err != nil {
		return err
	}
	// PowerShell treats typographic single quotes as string delimiters too.
	if strings.ContainsAny(s, "‘’") {
		return fmt.Errorf("%w: unqualified PowerShell quote delimiter", ErrUnsupported)
	}
	if len(s) <= 3 {
		return fmt.Errorf("%w: canonical local Windows file path required", ErrUnsupported)
	}
	drive := s[0] | 0x20
	if drive < 'a' || drive > 'z' || s[1:3] != ":\\" || strings.ContainsAny(s[3:], ":/<>\"|?*") || strings.IndexFunc(s, unicode.IsControl) >= 0 {
		return fmt.Errorf("%w: canonical local Windows file path required", ErrUnsupported)
	}
	for _, segment := range strings.Split(s[3:], "\\") {
		if segment == "" || segment == "." || segment == ".." || strings.TrimRight(segment, ". ") != segment {
			return fmt.Errorf("%w: ambiguous Windows path segment", ErrUnsupported)
		}
		if windowsDeviceSegment(segment) {
			return fmt.Errorf("%w: Windows device path", ErrUnsupported)
		}
	}
	return nil
}

func windowsDeviceSegment(segment string) bool {
	base := strings.ToUpper(strings.SplitN(segment, ".", 2)[0])
	if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" {
		return true
	}
	return len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9'
}
