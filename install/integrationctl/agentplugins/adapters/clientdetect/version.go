package clientdetect

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const maximumVersionOutput = 4096

type cappedBuffer struct {
	bytes.Buffer
	remaining int
}

func (buffer *cappedBuffer) Write(value []byte) (int, error) {
	if len(value) > buffer.remaining {
		return 0, fmt.Errorf("version output exceeds %d bytes", maximumVersionOutput)
	}
	buffer.remaining -= len(value)
	return buffer.Buffer.Write(value)
}

func probeExecutableVersion(ctx context.Context, executable string) (string, error) {
	isolatedDir, err := os.MkdirTemp("", "agentplugins-version-probe-")
	if err != nil {
		return "", fmt.Errorf("create isolated version probe directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(isolatedDir) }()

	output := &cappedBuffer{remaining: maximumVersionOutput}
	command := exec.CommandContext(ctx, executable, "--version")
	command.Dir = isolatedDir
	command.Env = []string{}
	if path := os.Getenv("PATH"); strings.TrimSpace(path) != "" {
		// PATH is the sole inherited variable so /usr/bin/env shebangs can
		// resolve their runtime without exposing HOME, tokens, or credentials.
		command.Env = append(command.Env, "PATH="+path)
	}
	command.Stdin = strings.NewReader("")
	command.Stdout, command.Stderr = output, output
	command.WaitDelay = 100 * time.Millisecond
	if err := command.Run(); err != nil {
		return "", err
	}
	return output.String(), nil
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
