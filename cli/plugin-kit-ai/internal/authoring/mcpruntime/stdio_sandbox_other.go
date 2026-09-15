//go:build !linux

package mcpruntime

import (
	"context"
	"fmt"
)

func stdioSandboxCommand(context.Context, string, string, string, []string, string) ([]string, string, string, string, error) {
	return nil, "", "", "", fmt.Errorf("stdio filesystem and network containment is unavailable on this platform")
}
