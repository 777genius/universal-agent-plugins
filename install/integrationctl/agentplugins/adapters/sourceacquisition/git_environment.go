// Git subprocesses use a credential-free, isolated environment.
// Keep this distinct from source resolution and on-disk ownership policy.
package sourceacquisition

import (
	"os"
	"strings"
	"runtime"
)

func isolatedGitEnvironment(environ []string, isolatedHome string) []string {
	clean := make([]string, 0, len(environ)+9)
	for _, item := range environ {
		key, _, _ := strings.Cut(item, "=")
		switch upper := strings.ToUpper(key); {
		case upper == "HOME", upper == "USERPROFILE", upper == "XDG_CONFIG_HOME":
			continue
		case upper == "NETRC", upper == "SSH_AUTH_SOCK", upper == "SSH_ASKPASS":
			continue
		case strings.HasPrefix(upper, "GIT_"):
			continue
		}
		clean = append(clean, item)
	}
	clean = append(clean,
		"HOME="+isolatedHome,
		"USERPROFILE="+isolatedHome,
		"XDG_CONFIG_HOME="+isolatedHome,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=credential.helper",
		"GIT_CONFIG_VALUE_0=",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_LFS_SKIP_SMUDGE=1",
	)
	return clean
}

// isolatedCheckoutSettings fixes the worktree policy without borrowing
// the user's Git configuration. core.longpaths is meaningful on Windows only.
func isolatedCheckoutSettings() [][]string {
	settings := [][]string{{"core.autocrlf", "false"}, {"core.eol", "lf"}, {"core.filemode", "true"}, {"core.symlinks", "true"}, {"fetch.recurseSubmodules", "false"}}
	if runtime.GOOS == "windows" {
		settings = append(settings, []string{"core.longpaths", "true"})
	}
	return settings
}
