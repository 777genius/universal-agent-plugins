//go:build darwin

package process

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Native Codex can exit before a same-group Git helper finishes. A short-lived
// helper must be allowed to settle naturally; a live helper must still fail.
func TestOSRunnerCompletionSettlesNaturalGroupHelpers(t *testing.T) {
	mode := os.Getenv("UAP_COMPLETION_GROUP_HELPER")
	if mode == "settling-child" || mode == "slow-child" || mode == "live-child" {
		delay := 250 * time.Millisecond
		if mode == "slow-child" {
			delay = duplexExitGrace + 250*time.Millisecond
		}
		if mode == "live-child" {
			delay = 20 * time.Second
		}
		time.Sleep(delay)
		if err := os.WriteFile(os.Getenv("UAP_COMPLETION_GROUP_MARKER"), []byte("naturally exited"), 0600); err != nil {
			os.Exit(31)
		}
		os.Exit(0)
	}
	if mode != "" {
		reader := bufio.NewReader(os.Stdin)
		if line, err := reader.ReadString('\n'); err != nil || line != "query\n" {
			os.Exit(32)
		}
		child := exec.CommandContext(context.Background(), os.Args[0], "-test.run=^TestOSRunnerCompletionSettlesNaturalGroupHelpers$")
		child.Env = append(os.Environ(), "UAP_COMPLETION_GROUP_HELPER="+mode+"-child")
		if err := child.Start(); err != nil {
			os.Exit(33)
		}
		if _, err := fmt.Fprintln(os.Stdout, "observed"); err != nil {
			os.Exit(35)
		}
		if _, err := reader.ReadByte(); err != io.EOF {
			os.Exit(34)
		}
		os.Exit(0)
	}
	for _, mode := range []string{"settling", "slow", "live"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			marker := filepath.Join(root, "natural-exit")
			budget := 5 * time.Second
			if mode == "live" {
				budget = 500 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(t.Context(), budget)
			defer cancel()
			err := (OS{}).RunDuplexToCompletion(ctx, ports.Command{
				Argv: []string{os.Args[0], "-test.run=^TestOSRunnerCompletionSettlesNaturalGroupHelpers$"},
				Dir:  root, Env: append(os.Environ(), "UAP_COMPLETION_GROUP_HELPER="+mode, "UAP_COMPLETION_GROUP_MARKER="+marker, "HOME="+root),
			}, func(input io.Writer, output io.Reader) error {
				if _, err := io.WriteString(input, "query\n"); err != nil {
					return err
				}
				line, err := bufio.NewReader(output).ReadString('\n')
				if err != nil {
					return err
				}
				if line != "observed\n" {
					return fmt.Errorf("unexpected response %q", line)
				}
				return nil
			})
			if mode != "live" {
				if err != nil {
					t.Fatalf("natural helper settlement failed: %v", err)
				}
				if _, err := os.Stat(marker); err != nil {
					t.Fatalf("returned before helper naturally finished: %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "forced") {
				t.Fatalf("live helper cleanup must fail completion: %v", err)
			} else if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("helper settlement hid caller deadline: %v", err)
			}
		})
	}
}
