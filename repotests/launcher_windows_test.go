//go:build windows

package pluginkitairepo_test

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func windowsCmdLauncherCommand(entry string, args ...string) *exec.Cmd {
	return &exec.Cmd{
		Path: windowsCmdExe(),
		SysProcAttr: &syscall.SysProcAttr{
			// Go's default Windows quoting does not match cmd.exe / batch parsing.
			// Build the full cmd.exe command line explicitly for .cmd launchers.
			CmdLine: windowsBatchCmdLine(entry, args...),
		},
	}
}

func windowsCmdExe() string {
	if path := strings.TrimSpace(os.Getenv("COMSPEC")); path != "" {
		return path
	}
	return "cmd"
}

func windowsBatchCmdLine(entry string, args ...string) string {
	parts := []string{quoteWindowsCmdArg(entry)}
	for _, arg := range args {
		parts = append(parts, quoteWindowsCmdArg(arg))
	}
	return `/d /s /c "` + strings.Join(parts, " ") + `"`
}

func quoteWindowsCmdArg(arg string) string {
	if arg == "" {
		return `""`
	}
	if !strings.ContainsAny(arg, " \t\"&()[]{}^=;!'+,`~|<>") {
		return arg
	}
	return `"` + strings.ReplaceAll(arg, `"`, `""`) + `"`
}
