package providers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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
	command := exec.CommandContext(timeout, npm, args...)
	command.Dir = temporary
	command.Stdin = nil
	command.Stdout = io.Discard
	var stderr runtimeStderrTail
	command.Stderr = &stderr
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
	if err := command.Run(); err != nil {
		if timeout.Err() != nil {
			return fmt.Errorf("locked npm runtime preparation timed out: %w", timeout.Err())
		}
		detail := strings.TrimSpace(string(stderr.tail))
		if detail == "" {
			return fmt.Errorf("locked npm runtime preparation: %w", err)
		}
		return fmt.Errorf("locked npm runtime preparation: %w: %s", err, detail)
	}
	return nil
}

// npm can emit unbounded diagnostics on a failing dependency graph. Keep only
// the useful tail without letting an untrusted package exhaust installer RAM.
type runtimeStderrTail struct{ tail []byte }

func (writer *runtimeStderrTail) Write(chunk []byte) (int, error) {
	const maxBytes = 1200
	length := len(chunk)
	if length >= maxBytes {
		writer.tail = append(writer.tail[:0], chunk[length-maxBytes:]...)
		return length, nil
	}
	if len(writer.tail)+length > maxBytes {
		writer.tail = append(writer.tail[:0], writer.tail[len(writer.tail)+length-maxBytes:]...)
	}
	writer.tail = append(writer.tail, chunk...)
	return length, nil
}
