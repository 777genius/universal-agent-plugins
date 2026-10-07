package clientdetect

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const maximumVersionOutput = 4096

var errVersionOutputLimit = errors.New("probe_output_limit")

// Separate streams share one budget. exec can copy them concurrently.
type versionOutput struct {
	mu             sync.Mutex
	stdout, stderr bytes.Buffer
	remaining      int
	exceeded       bool
}
type versionWriter struct {
	output *versionOutput
	stderr bool
}

func (w versionWriter) Write(value []byte) (int, error) {
	w.output.mu.Lock()
	defer w.output.mu.Unlock()
	if len(value) > w.output.remaining {
		w.output.exceeded = true
		return 0, errVersionOutputLimit
	}
	w.output.remaining -= len(value)
	if w.stderr {
		return w.output.stderr.Write(value)
	}
	return w.output.stdout.Write(value)
}

func probeExecutableVersion(ctx context.Context, executable string) (string, error) {
	return probeExecutableVersionWithEnvironment(ctx, executable, nil)
}

func probeExecutableVersionWithEnvironment(ctx context.Context, executable string, environment []string) (string, error) {
	env := append([]string{}, environment...)
	if path := os.Getenv("PATH"); strings.TrimSpace(path) != "" {
		env = append(env, "PATH="+path)
	}
	stdout, stderr, err := runVersionProcess(ctx, executable, env)
	return stdout + stderr, err
}

// runVersionProcess is the single process primitive. Explicit authority callers
// supply the complete environment; it never appends ambient variables.
func runVersionProcess(ctx context.Context, executable string, environment []string) (string, string, error) {
	isolatedDir, err := os.MkdirTemp("", "agentplugins-version-probe-")
	if err != nil {
		return "", "", err
	}
	defer func() { _ = os.RemoveAll(isolatedDir) }()
	output := &versionOutput{remaining: maximumVersionOutput}
	command := exec.CommandContext(ctx, executable, "--version")
	command.Dir = isolatedDir
	command.Env = append([]string{}, environment...)
	command.Stdin = strings.NewReader("")
	command.Stdout = versionWriter{output: output}
	command.Stderr = versionWriter{output: output, stderr: true}
	command.WaitDelay = 100 * time.Millisecond
	err = command.Run()
	if output.exceeded {
		err = errVersionOutputLimit
	}
	return output.stdout.String(), output.stderr.String(), err
}

func normalizeVersion(value string) string {
	for _, field := range strings.Fields(value) {
		candidate := strings.TrimSuffix(strings.Trim(field, "vV,;()[]{}"), ".")
		parts := strings.SplitN(strings.SplitN(candidate, "+", 2)[0], "-", 2)
		core := strings.Split(parts[0], ".")
		if len(core) < 2 {
			continue
		}
		valid := true
		for _, segment := range core {
			if segment == "" || strings.Trim(segment, "0123456789") != "" {
				valid = false
				break
			}
		}
		if valid {
			return candidate
		}
	}
	return ""
}
