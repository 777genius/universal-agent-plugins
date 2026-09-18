package sourceacquisition

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type Command struct {
	Dir            string
	Args           []string
	Stdin          []byte
	MaxOutputBytes int
	Progress       io.Writer
}

type Runner interface {
	Run(context.Context, Command) ([]byte, error)
}

type OSRunner struct{}

func (OSRunner) Run(ctx context.Context, command Command) ([]byte, error) {
	if len(command.Stdin) != 0 {
		return nil, fmt.Errorf("git stdin is unsupported by the contained runner")
	}
	result, err := (processadapter.OS{}).RunWithTreeExitGrace(ctx, legacyports.Command{
		Argv: append([]string{"git"}, command.Args...), Dir: command.Dir,
		Env: isolatedGitEnvironment(os.Environ(), command.Dir), StdoutLimitBytes: command.MaxOutputBytes,
		Stderr: command.Progress,
	}, 5*time.Second)
	output := append(append([]byte(nil), result.Stdout...), result.Stderr...)
	if err != nil {
		if processadapter.IsOnlyStdoutLimitExceeded(err) ||
			processadapter.IsExactStdoutLimitExitCode(err, 141) {
			return nil, processadapter.ErrStdoutLimitExceeded
		}
		return nil, fmt.Errorf("git %s failed: %w: %s", strings.Join(command.Args, " "), err, strings.TrimSpace(string(output)))
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("git %s failed with exit code %d: %s", strings.Join(command.Args, " "), result.ExitCode, strings.TrimSpace(string(output)))
	}
	return output, nil
}

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
