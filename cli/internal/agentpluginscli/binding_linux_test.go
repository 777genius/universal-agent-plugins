//go:build linux

package agentpluginscli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
	"golang.org/x/sys/unix"
)

func TestBindingReviewRedirectedStdoutTerminalStderr(t *testing.T) {
	for _, mode := range []usecase.BindingChangeMode{usecase.BindingChangeRebind, usecase.BindingChangeMigrateFormat} {
		t.Run(bindingCommandName(mode), func(t *testing.T) {
			master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer master.Close()
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
			defer terminal.Close()
			out, err := os.CreateTemp(t.TempDir(), "stdout")
			if err != nil {
				t.Fatal(err)
			}
			defer out.Close()
			fixture, source := bindingReviewFixture(t, mode)
			input := &bindingConsentReader{input: strings.NewReader("y\n")}
			input.before = func() {
				if input.reads != 1 {
					return
				}
				var visible strings.Builder
				for !strings.Contains(visible.String(), "[y/N]") {
					fds := []unix.PollFd{{Fd: int32(master.Fd()), Events: unix.POLLIN}}
					n, err := unix.Poll(fds, 1000)
					if err != nil || n == 0 {
						t.Fatalf("visible plan unavailable: %v", err)
					}
					buf := make([]byte, 4096)
					n, err = master.Read(buf)
					if err != nil {
						t.Fatal(err)
					}
					visible.Write(buf[:n])
				}
				text := visible.String()
				plan := strings.Index(text, "PLUGIN_DATA: not transferred")
				question := strings.Index(text, "Apply this binding change?")
				if !strings.Contains(text, "Plugin:") || plan < 0 || question < plan {
					t.Fatalf("plan not visible before consent: %q", text)
				}
				info, _ := out.Stat()
				if info.Size() != 0 {
					t.Fatal("plan leaked to redirected stdout")
				}
			}
			if err := runBindingReview(t, fixture, source, mode, options{format: "human"}, true, input, out, terminal); err != nil {
				t.Fatal(err)
			}
			if input.reads == 0 {
				t.Fatal("consent was bypassed")
			}
			state, err := fixture.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if state.Installations[0].Source.CanonicalSource != source {
				t.Fatal("approved binding not committed")
			}
			if _, err := out.Seek(0, 0); err != nil {
				t.Fatal(err)
			}
			result, err := io.ReadAll(out)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(result), "Binding updated.") {
				t.Fatalf("missing result: %s", result)
			}
		})
	}
}
