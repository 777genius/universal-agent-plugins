//go:build aix || dragonfly || freebsd || netbsd || openbsd || solaris

package process

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestOSRunnerFailsClosedOnUnsupportedPlatform(t *testing.T) {
	command := ports.Command{Argv: []string{"unused"}}
	if _, err := (OS{}).Run(context.Background(), command); err == nil || !strings.Contains(err.Error(), "process execution is unsupported") {
		t.Fatalf("Run error = %v, want explicit unsupported-platform failure", err)
	}
	if err := (OS{}).RunDuplex(context.Background(), command, func(io.Writer, io.Reader) error { return nil }); err == nil || !strings.Contains(err.Error(), "manual activation required") {
		t.Fatalf("RunDuplex error = %v, want explicit manual-activation containment failure", err)
	}
}
