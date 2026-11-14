package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

func executeRuntimeTestCommand(ctx context.Context, root string, args []string, stdin []byte) (string, string, int, error) {
	if len(args) == 0 {
		return "", "", 0, fmt.Errorf("missing command")
	}
	cmd := testCommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = root
	if len(stdin) > 0 {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return "", "", 0, fmt.Errorf("execute %s: %w", strings.Join(args, " "), err)
		}
		exitCode = exitErr.ExitCode()
	}
	return stdout.String(), stderr.String(), exitCode, nil
}
