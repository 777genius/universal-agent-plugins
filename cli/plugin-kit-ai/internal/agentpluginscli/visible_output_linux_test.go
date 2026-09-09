//go:build linux

package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/promptio"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/sys/unix"
)

type visibleSecurityEvaluator struct{}

func (visibleSecurityEvaluator) Evaluate(context.Context, domain.SecurityEvaluationInput) (domain.SecurityAssessment, error) {
	return *testSecurityAssessment(1, 1), nil
}

func consentPTY(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { master.Close() })
	if err := unix.IoctlSetPointerInt(int(master.Fd()), unix.TIOCSPTLCK, 0); err != nil {
		t.Fatal(err)
	}
	number, err := unix.IoctlGetInt(int(master.Fd()), unix.TIOCGPTN)
	if err != nil {
		t.Fatal(err)
	}
	terminal, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", number), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { terminal.Close() })
	return master, terminal
}

func readConsentPTY(t *testing.T, master *os.File) string {
	t.Helper()
	var visible strings.Builder
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(visible.String(), "[y/N]") {
		if time.Now().After(deadline) {
			t.Fatalf("consent not visible on terminal: %q", visible.String())
		}
		fds := []unix.PollFd{{Fd: int32(master.Fd()), Events: unix.POLLIN}}
		n, err := unix.Poll(fds, 100)
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			continue
		}
		buf := make([]byte, 4096)
		n, err = master.Read(buf)
		if err != nil {
			t.Fatal(err)
		}
		visible.Write(buf[:n])
	}
	return visible.String()
}

func TestNewRootVisibleConsent(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")
	for _, command := range []string{"rebind", "migrate-format", "add"} {
		for _, color := range []string{"--color=always", "--color=auto", "--no-color"} {
			for _, answer := range []string{"y", "n", "cancel"} {
				t.Run(command+"/"+color+"/"+answer, func(t *testing.T) {
					master, terminal := consentPTY(t)
					out, err := os.CreateTemp(t.TempDir(), "stdout")
					if err != nil {
						t.Fatal(err)
					}
					defer out.Close()
					mode := usecase.BindingChangeRebind
					if command == "migrate-format" {
						mode = usecase.BindingChangeMigrateFormat
					}
					fixture, source := bindingReviewFixture(t, mode)
					args := []string{command, "demo", source, color}
					question := "Apply this binding change?"
					markers := []string{"Plugin:", "Format:", "Schema:", "Source:", "Components:", "Native objects:", "PLUGIN_DATA: not transferred"}
					if command == "add" {
						fixture = newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
						fixture.app.SecurityEvaluator = visibleSecurityEvaluator{}
						args = []string{"add", source, "--target=cursor", "--security-details", color}
						question = "Blocking security findings were detected. Continue anyway?"
						markers = []string{"Automated review:", "test blocking finding", "test review note", "Evidence:"}
					}
					before, err := fixture.store.Load()
					if err != nil {
						t.Fatal(err)
					}
					app := fixture.app
					app.Terminal, app.Input, app.Output, app.ErrorOutput = true, terminal, out, terminal
					root := NewRoot(app)
					root.SetArgs(args)
					ctx, cancel := context.WithCancel(context.Background())
					done := make(chan error, 1)
					go func() { done <- root.ExecuteContext(ctx) }()
					finished := false
					defer func() {
						cancel()
						if !finished {
							select {
							case <-done:
							case <-time.After(5 * time.Second):
								t.Error("command did not stop")
							}
						}
					}()
					// Supply no input until the complete review and question reach the OS PTY.
					visible := readConsentPTY(t, master)
					plain := ansi.Strip(visible)
					for _, marker := range markers {
						index := strings.Index(plain, marker)
						if index < 0 || index > strings.Index(plain, question) {
							t.Fatalf("review missing or after question: %q", visible)
						}
					}
					if command != "add" && strings.Contains(visible, "\x1b[") != (color != "--no-color") {
						t.Fatalf("selected writer lost color policy: %q", visible)
					}
					logged, err := os.ReadFile(out.Name())
					if err != nil {
						t.Fatal(err)
					}
					if strings.Contains(string(logged), markers[0]) || strings.Contains(string(logged), question) {
						t.Fatalf("review leaked to log: %q", logged)
					}
					pending, err := fixture.store.Load()
					if err != nil || !reflect.DeepEqual(before, pending) {
						t.Fatalf("mutation before consent: %v", err)
					}
					if answer == "cancel" {
						cancel()
					} else if _, err := fmt.Fprintln(master, answer); err != nil {
						t.Fatal(err)
					}
					if command == "add" && answer == "y" {
						// The explicit target authorizes installation after security consent.
						// Decline the later manual activation attestation in this synthetic client.
						activation := readConsentPTY(t, master)
						if !strings.Contains(activation, "Have you completed activation") {
							t.Fatalf("unexpected follow-up: %q", activation)
						}
						if _, err := fmt.Fprintln(master, "n"); err != nil {
							t.Fatal(err)
						}
					}

					select {
					case err = <-done:
						finished = true
					case <-time.After(5 * time.Second):
						t.Fatal("command did not complete")
					}
					if answer == "cancel" {
						if !errors.Is(err, context.Canceled) {
							t.Fatalf("cancel error: %v", err)
						}
					} else if command == "add" && answer == "n" {
						if err == nil || !strings.Contains(err.Error(), "cancelled after automated security review") {
							t.Fatalf("decline error: %v", err)
						}
					} else if err != nil {
						t.Fatal(err)
					}
					after, err := fixture.store.Load()
					if err != nil {
						t.Fatal(err)
					}
					if changed := !reflect.DeepEqual(before, after); changed != (answer == "y") {
						t.Fatalf("state changed=%v after %s", changed, answer)
					}
				})
			}
		}
	}
}

func TestNewRootRedirectedConsentUnavailable(t *testing.T) {
	for _, color := range []string{"--color=always", "--no-color"} {
		t.Run(color, func(t *testing.T) {
			fixture, source := bindingReviewFixture(t, usecase.BindingChangeRebind)
			before, _ := fixture.store.Load()
			out, err := os.CreateTemp(t.TempDir(), "log")
			if err != nil {
				t.Fatal(err)
			}
			defer out.Close()
			app := fixture.app
			app.Terminal, app.Input, app.Output, app.ErrorOutput = true, mustNotRead{t}, out, out
			root := NewRoot(app)
			root.SetArgs([]string{"rebind", "demo", source, color})
			if err := root.Execute(); !errors.Is(err, prompt.ErrPromptUnavailable) {
				t.Fatalf("error=%v", err)
			}
			after, _ := fixture.store.Load()
			if !reflect.DeepEqual(before, after) {
				t.Fatal("unavailable consent mutated state")
			}
		})
	}
}

func TestVisibleOutputPreservesSelectedTheme(t *testing.T) {
	_, terminal := consentPTY(t)
	redirected, err := os.CreateTemp(t.TempDir(), "log")
	if err != nil {
		t.Fatal(err)
	}
	defer redirected.Close()
	policy, format := terminaltheme.Policy{Mode: "always", Explicit: true}, "human"
	primary := terminaltheme.Wrap(terminal, &policy, &format)
	alternate := terminaltheme.Wrap(redirected, &policy, &format)
	selected, err := promptio.VisibleOutput(primary, alternate)
	if err != nil || selected != primary || !terminaltheme.For(selected).Enabled {
		t.Fatalf("terminal primary not retained: %v", err)
	}
	injected := terminaltheme.Wrap(&strings.Builder{}, &policy, &format)
	selected, err = promptio.VisibleOutput(injected, primary)
	if err != nil || selected != injected {
		t.Fatalf("injected writer contract changed: %v", err)
	}
}
