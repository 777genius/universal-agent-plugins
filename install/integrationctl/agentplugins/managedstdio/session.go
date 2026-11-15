package managedstdio

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// StartSession reuses the existing inherited-terminal process replacement / Job
// Object boundary. The caller must obtain explicit consent and verify all owned
// configuration before reaching this function. It performs no shell evaluation.
func StartSession(command string, args []string, cwd string, environment []string) error {
	return StartSessionContext(context.Background(), command, args, cwd, environment)
}

// StartSessionContext observes cancellation before launch and, on Windows,
// terminates and reaps the owned Job Object. Unix retains process replacement.
func StartSessionContext(ctx context.Context, command string, args []string, cwd string, environment []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !Supported() || !filepath.IsAbs(command) || !filepath.IsAbs(cwd) || len(args) == 0 || args[0] != command {
		return fmt.Errorf("native session requires a supported host and exact absolute command/directory")
	}
	for _, value := range append(append([]string{command, cwd}, args...), environment...) {
		if strings.ContainsRune(value, 0) {
			return fmt.Errorf("native session contains a NUL byte")
		}
	}
	return replaceSessionContext(ctx, command, args, cwd, environment)
}
