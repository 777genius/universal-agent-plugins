//go:build linux

package terminalprompts

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/promptio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"golang.org/x/sys/unix"
)

// The controller writes one actual PTY batch at the selection frame. There is
// no delay between that submission and the queued confirmation keys.
func TestConsentPTYBoundary(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	for _, tc := range []struct {
		name, selection, confirmation, remaining string
		accepted                                 bool
		wantErr                                  error
	}{
		{name: "queued-space-enter", selection: "\r \r"},
		{name: "queued-enters", selection: "\r\r"},
		{name: "queued-yes", selection: "\ry\r"},
		{name: "queued-left", selection: "\r\x1b[D\r"},
		{name: "queued-right", selection: "\r\x1b[C\r"},
		{name: "queued-enhanced-space", selection: "\r\x1b[32u\r"},
		{name: "queued-space-fresh-enter", selection: "\r ", confirmation: "\r"},
		{name: "queued-space-fresh-consent", selection: "\r ", confirmation: " \r", accepted: true},
		{name: "queued-paste-fresh-consent", selection: "\r\x1b[200~ \ry\n\x1b[201~", confirmation: " \r", accepted: true},
		{name: "incomplete-paste", selection: "\r\x1b[200~ \r", wantErr: prompt.ErrPromptCanceled},
		{name: "queued-escape", selection: "\r\x1b", wantErr: prompt.ErrPromptCanceled},
		{name: "queued-ctrl-c", selection: "\r\x03", wantErr: prompt.ErrPromptCanceled},
		{name: "queued-ctrl-d", selection: "\r\x04", wantErr: prompt.ErrPromptCanceled},
		{name: "fresh-space-enter", selection: "\r", confirmation: " \r", accepted: true},
		{name: "fresh-left-enter", selection: "\r", confirmation: "\x1b[D\r", accepted: true},
		{name: "fresh-paste-default-no", selection: "\r", confirmation: "\x1b[200~ \ry\n\x1b[201~\r"},
		{name: "fresh-default-no", selection: "\r", confirmation: "\r"},
		{name: "fresh-escape", selection: "\r", confirmation: "\x1b", wantErr: prompt.ErrPromptCanceled},
		{name: "fresh-ctrl-c", selection: "\r", confirmation: "\x03", wantErr: prompt.ErrPromptCanceled},
		{name: "fresh-ctrl-d", selection: "\r", confirmation: "\x04", wantErr: prompt.ErrPromptCanceled},
		{name: "stale-next-owner", selection: "\r \rnext-owner\n", remaining: "next-owner"},
		{name: "fresh-next-owner", selection: "\r", confirmation: " \rnext-owner\n", accepted: true, remaining: "next-owner"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY|unix.O_NONBLOCK, 0)
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
			slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", number), os.O_RDWR|unix.O_NOCTTY, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer slave.Close()
			if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: 30, Col: 100}); err != nil {
				t.Fatal(err)
			}
			before, err := unix.IoctlGetTermios(int(slave.Fd()), unix.TCGETS)
			if err != nil {
				t.Fatal(err)
			}
			drained := make(chan []byte, 1)
			go func() {
				var output bytes.Buffer
				var buf [4096]byte
				selectionSent, confirmationSent := false, false
				for {
					n, err := master.Read(buf[:])
					output.Write(buf[:n])
					if !selectionSent && bytes.Contains(output.Bytes(), []byte("Choose targets")) {
						selectionSent = true
						if n, e := io.WriteString(master, tc.selection); e != nil || n != len(tc.selection) {
							t.Errorf("selection write=%d %v", n, e)
						}
					}
					if selectionSent && !confirmationSent && tc.confirmation != "" && bytes.Contains(output.Bytes(), []byte("Yes")) {
						confirmationSent = true
						if n, e := io.WriteString(master, tc.confirmation); e != nil || n != len(tc.confirmation) {
							t.Errorf("confirmation write=%d %v", n, e)
						}
					}
					if err != nil {
						drained <- output.Bytes()
						return
					}
				}
			}()
			defer func() {
				slave.Close()
				master.Close()
				select {
				case <-drained:
				case <-time.After(time.Second):
					t.Error("controller did not join")
				}
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			p := HuhPrompter{Input: slave, Output: slave, NoColor: true}
			_, err = p.SelectTargets(ctx, prompt.TargetSelectionRequest{Choices: []prompt.TargetChoice{{ID: "cursor", Label: "Fixture"}}, DefaultIDs: []domain.ClientID{"cursor"}})
			if err != nil {
				t.Fatal(err)
			}
			result, err := p.Confirm(ctx, prompt.ConfirmationRequest{Title: "Apply fixture?"})
			after, restoreErr := unix.IoctlGetTermios(int(slave.Fd()), unix.TCGETS)
			if restoreErr != nil || *after != *before {
				t.Errorf("mode restoration: %v", restoreErr)
			}
			if tc.remaining != "" {
				line, e := promptio.ReadLine(ctx, slave)
				if e != nil || line != tc.remaining {
					t.Errorf("next owner: %q %v", line, e)
				}
			}
			slave.Close()
			var output []byte
			select {
			case output = <-drained:
			case <-ctx.Done():
				t.Fatal("output did not drain")
			}
			drained <- output
			t.Logf("selection batch=%x separate confirmation=%x accepted=%v error=%v\n%s", tc.selection, tc.confirmation, result.Accepted, err, output)
			if !errors.Is(err, tc.wantErr) || result.Accepted != tc.accepted {
				t.Fatalf("confirmation=%+v %v; want accepted=%v", result, err, tc.accepted)
			}
			if bytes.LastIndex(output, []byte("\x1b[?25h")) <= bytes.LastIndex(output, []byte("\x1b[?25l")) {
				t.Error("cursor not restored")
			}
		})
	}
}
