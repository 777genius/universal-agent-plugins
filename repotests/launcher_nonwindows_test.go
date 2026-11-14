//go:build !windows

package pluginkitairepo_test

import "os/exec"

func windowsCmdLauncherCommand(entry string, args ...string) *exec.Cmd {
	return exec.Command(entry, args...)
}
