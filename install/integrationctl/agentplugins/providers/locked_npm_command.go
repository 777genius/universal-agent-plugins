package providers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func runLockedNPM(ctx context.Context, temporary, dataPath string, omitOptional bool) error {
	timeout, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	args := []string{"ci", "--ignore-scripts", "--omit=dev"}
	if omitOptional {
		args = append(args, "--omit=optional")
	}
	args = append(args, "--no-audit", "--no-fund")
	npm := "npm"
	if filepath.Separator == '\\' {
		npm = "npm.cmd"
	}
	command := ports.Command{Argv: append([]string{npm}, args...), Dir: temporary, DiscardStdout: true}
	cachePath := filepath.Join(dataPath, "npm-cache")
	if err := os.Mkdir(cachePath, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	if err := requireRealDirectory(cachePath); err != nil {
		return fmt.Errorf("unsafe npm cache: %w", err)
	}
	env := make([]string, 0, len(os.Environ())+8)
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "npm_") || lower == "node_options" || lower == "node_path" ||
			lower == "init_cwd" || lower == "home" || lower == "userprofile" {
			continue
		}
		env = append(env, item)
	}
	env = append(env, "HOME="+temporary, "npm_config_cache="+cachePath,
		"npm_config_userconfig="+filepath.Join(temporary, ".npmrc-user"),
		"npm_config_globalconfig="+filepath.Join(temporary, ".npmrc-global"),
		"npm_config_ignore_scripts=true", "npm_config_audit=false", "npm_config_fund=false",
		"npm_config_update_notifier=false", "npm_config_registry=https://registry.npmjs.org/")
	command.Env = env
	result, err := (process.OS{}).Run(timeout, command)
	if ctxErr := timeout.Err(); ctxErr != nil {
		err = errors.Join(err, ctxErr)
	}
	return lockedNPMCommandError(result, err)
}

func lockedNPMCommandError(result ports.CommandResult, err error) error {
	if err == nil && result.ExitCode == 0 {
		return nil
	}
	if err == nil {
		err = fmt.Errorf("command exited with code %d", result.ExitCode)
	}
	operation := "locked npm runtime preparation"
	if errors.Is(err, context.DeadlineExceeded) {
		operation += " timed out"
	} else if errors.Is(err, context.Canceled) {
		operation += " canceled"
	}
	// The shared runner bounds diagnostics while retaining their suffix. Keep
	// the existing npm error tail without dropping typed cleanup/cancel causes.
	const maxBytes = 1200
	diagnostic := result.Stderr
	if len(diagnostic) > maxBytes {
		diagnostic = diagnostic[len(diagnostic)-maxBytes:]
	}
	// Runner errors may themselves contain its larger diagnostic buffer. Keep
	// the full cause available to errors.Is/As while bounding the human message.
	cause := err.Error()
	const maxCauseBytes = 300
	if len(cause) > maxCauseBytes {
		cause = cause[:maxCauseBytes] + " [truncated]"
	}
	message := operation + ": " + cause
	if detail := strings.TrimSpace(string(diagnostic)); detail != "" {
		message += ": " + detail
	}
	return &lockedNPMFailure{message: message, cause: err}
}

type lockedNPMFailure struct {
	message string
	cause   error
}

func (failure *lockedNPMFailure) Error() string { return failure.message }
func (failure *lockedNPMFailure) Unwrap() error { return failure.cause }
