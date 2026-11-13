//go:build !darwin && !linux && !windows

package managedstdio

import (
	"context"
	"fmt"
)

func replaceProcessEnvironment(string, []string, string, []string) error {
	return fmt.Errorf("native sessions are unsupported on this platform")
}

func replaceSessionContext(ctx context.Context, command string, args []string, cwd string, environment []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return replaceProcessEnvironment(command, args, cwd, environment)
}
