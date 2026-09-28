//go:build linux || darwin

package terminalprompts

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/promptio"
)

func TestCanonicalConsentPTYBoundary(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	for _, plain := range []bool{true, false} {
		for _, tc := range []struct {
			name, queued, fresh, rest string
			accepted                  bool
			canceled                  bool
		}{
			{name: "queued-enter", queued: "\n"},
			{name: "queued-yes-suffix", queued: "y\nnext-owner\n", rest: "next-owner"},
			{name: "queued-enter-suffix", queued: "\nnext-owner\n", rest: "next-owner"},
			{name: "queued-cancel", queued: "\x1b\n", canceled: true},
			{name: "fresh-enter", fresh: "\r", accepted: true},
			{name: "fresh-no", fresh: "n\n"},
		} {
			name := tc.name
			if plain {
				name = "plain/" + name
			} else {
				name = "rich/" + name
			}
			t.Run(name, func(t *testing.T) {
				master, slave := consentPTY(t)
				before, e := unix.IoctlGetTermios(int(slave.Fd()), consentGetTermios)
				if e != nil {
					t.Fatal(e)
				}
				if tc.queued != "" {
					if _, e := io.WriteString(master, tc.queued); e != nil {
						t.Fatal(e)
					}
					fds := []unix.PollFd{{Fd: int32(slave.Fd()), Events: unix.POLLIN}}
					if n, e := unix.Poll(fds, 1000); e != nil || n == 0 {
						t.Fatalf("queue: %d %v", n, e)
					}
				}
				done := make(chan []byte, 1)
				go func() {
					var out bytes.Buffer
					var b [4096]byte
					sent := false
					for {
						n, e := master.Read(b[:])
						out.Write(b[:n])
						marker := "Yes"
						if plain {
							marker = "[Y/n]"
						}
						if !sent && tc.fresh != "" && bytes.Contains(out.Bytes(), []byte(marker)) {
							sent = true
							fresh := tc.fresh
							if !plain && fresh == "n\n" {
								fresh = " \r"
							}
							_, _ = io.WriteString(master, fresh)
						}
						if e != nil {
							done <- out.Bytes()
							return
						}
					}
				}()
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				var p prompt.Prompter = HuhPrompter{Input: slave, Output: slave, NoColor: true}
				if plain {
					p = PlainPrompter{Input: slave, Output: slave}
				}
				result, err := p.Confirm(ctx, prompt.ConfirmationRequest{Title: "Apply fixture?", Default: true})
				after, e := unix.IoctlGetTermios(int(slave.Fd()), consentGetTermios)
				if e != nil || *after != *before {
					t.Errorf("restore: %v %+v != %+v", e, after, before)
				}
				if tc.rest != "" {
					line, e := promptio.ReadLine(ctx, slave)
					if e != nil || line != tc.rest {
						t.Errorf("suffix=%q %v", line, e)
					}
				}
				if e := slave.Close(); e != nil {
					t.Error(e)
				}
				if e := master.Close(); e != nil {
					t.Error(e)
				}
				select {
				case out := <-done:
					t.Logf("%s", out)
				case <-time.After(time.Second):
					t.Error("controller did not join")
				}
				if tc.canceled {
					if !errors.Is(err, prompt.ErrPromptCanceled) {
						t.Errorf("cancel=%v", err)
					}
				} else if err != nil {
					t.Errorf("confirm: %v", err)
				}
				if result.Accepted != tc.accepted {
					t.Errorf("accepted=%v want %v", result.Accepted, tc.accepted)
				}
			})
		}
	}
}
