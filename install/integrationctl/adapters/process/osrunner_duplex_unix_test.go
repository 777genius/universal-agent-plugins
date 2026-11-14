//go:build !windows

package process

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestOSRunnerDuplexPreservesNaturalSIGKILLStatus(t *testing.T) {
	requireDuplexCapability(t)
	if os.Getenv("AGENTPLUGINS_PROCESS_NATURAL_SIGKILL_HELPER") == "1" {
		fmt.Fprintln(os.Stdout, "ready")
		if line, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil || line != "exit\n" {
			os.Exit(51)
		}
		_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)
		os.Exit(52)
	}
	environment := append(os.Environ(), "AGENTPLUGINS_PROCESS_NATURAL_SIGKILL_HELPER=1")
	err := (OS{}).RunDuplex(context.Background(), ports.Command{
		Argv: []string{os.Args[0], "-test.run=TestOSRunnerDuplexPreservesNaturalSIGKILLStatus"}, Env: environment,
	}, func(stdin io.Writer, stdout io.Reader) error {
		reader := bufio.NewReader(stdout)
		if line, err := reader.ReadString('\n'); err != nil || line != "ready\n" {
			return fmt.Errorf("ready barrier = %q: %w", line, err)
		}
		if _, err := io.WriteString(stdin, "exit\n"); err != nil {
			return err
		}
		_, err := reader.ReadByte()
		if !errors.Is(err, io.EOF) {
			return fmt.Errorf("natural exit barrier: %w", err)
		}
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "signal: killed") {
		t.Fatalf("error = %v, want unsuppressed natural SIGKILL status", err)
	}
}
