//go:build linux

package installerui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/777genius/plugin-kit-ai/cli/internal/promptio"
)

// The driver waits for actual rendered frames, owns only a new test PTY, and
// joins before fixture descriptors close. No real profile or agent is used.
func terminalDriver(t *testing.T, master *os.File, step func(string)) func() string {
	t.Helper()
	stop := make(chan struct{})
	joined := make(chan string, 1)
	go func() {
		var transcript strings.Builder
		for {
			select {
			case <-stop:
				joined <- transcript.String()
				return
			default:
			}
			fds := []unix.PollFd{{Fd: int32(master.Fd()), Events: unix.POLLIN}}
			n, err := unix.Poll(fds, 20)
			if errors.Is(err, unix.EINTR) {
				continue
			}
			if err != nil {
				joined <- transcript.String()
				return
			}
			if n == 0 {
				continue
			}
			var b [8192]byte
			n, err = master.Read(b[:])
			transcript.Write(b[:n])
			if step != nil {
				step(transcript.String())
			}
			if err != nil {
				joined <- transcript.String()
				return
			}
		}
	}()
	finished := false
	finish := func() string {
		if finished {
			return ""
		}
		finished = true
		close(stop)
		select {
		case s := <-joined:
			return s
		case <-time.After(time.Second):
			t.Error("terminal driver did not join")
			return ""
		}
	}
	t.Cleanup(func() { finish() })
	return finish
}

func publicTestTerminal(t *testing.T, mode TerminalMode) (*Terminal, *os.File, *os.File) {
	t.Helper()
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("HOME", t.TempDir())
	master, slave := consentPTY(t)
	p, err := NewTerminal(TerminalConfig{Input: slave, Output: slave, Mode: mode, NoColor: true})
	if err != nil {
		t.Fatal(err)
	}
	return p, master, slave
}

func TestTerminalMultiSelectContract(t *testing.T) {
	for _, mode := range []TerminalMode{ModePlain, ModeRich} {
		for _, tc := range []struct {
			name, plain, rich string
			defaults, want    []string
			min               int
			cancel            bool
			wantErr           error
		}{
			{name: "canonical", plain: "2,1\n", rich: " \x1b[B \r", want: []string{"alpha", "beta"}},
			{name: "defaults-copy", plain: "\n", rich: "\r", defaults: []string{"beta", "alpha"}, want: []string{"alpha", "beta"}},
			{name: "accepted-empty", plain: "none\n", rich: "\r"},
			{name: "all", plain: "all\n", rich: " \x1b[B \r", want: []string{"alpha", "beta"}},
			{name: "cancel", plain: "Cancel\n", rich: "\x1b", cancel: true},
			{name: "retry-invalid", plain: "1,alpha\nall,beta\n2\n", rich: "\r \x1b[B \r", min: 1, want: []string{"alpha", "beta"}},
			{name: "invalid-limit", plain: "none\n0\nalpha,alpha\n", min: 1, wantErr: ErrInvalidSelection},
		} {
			if mode == ModeRich && tc.name == "invalid-limit" {
				continue
			}
			t.Run(string(mode)+"/"+tc.name, func(t *testing.T) {
				p, master, slave := publicTestTerminal(t, mode)
				before, err := unix.IoctlGetTermios(int(slave.Fd()), unix.TCGETS)
				if err != nil {
					t.Fatal(err)
				}
				keys := tc.plain
				if mode == ModeRich {
					keys = tc.rich
				}
				sent := false
				finish := terminalDriver(t, master, func(out string) {
					if !sent && strings.Contains(out, "Choose fixture") {
						sent = true
						_, _ = io.WriteString(master, keys)
					}
				})
				req := MultiSelectRequest{SelectRequest: SelectRequest{Title: "Choose fixture", Options: []Option{{"alpha", "Alpha"}, {"beta", "Beta"}}, Defaults: tc.defaults}, MinSelected: tc.min}
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				got, err := p.SelectMany(ctx, req)
				out := finish()
				want := tc.want
				if tc.name == "retry-invalid" && mode == ModePlain {
					want = []string{"beta"}
				}
				if !errors.Is(err, tc.wantErr) || got.Cancelled != tc.cancel || got.Accepted != (tc.wantErr == nil && !tc.cancel) || !reflect.DeepEqual(nonNil(got.IDs), nonNil(want)) {
					t.Fatalf("%+v %v want=%v output=%q", got, err, want, out)
				}
				if len(got.IDs) > 0 {
					got.IDs[0] = "changed"
					if len(req.Defaults) > 0 && req.Defaults[0] == "changed" {
						t.Fatal("aliased defaults")
					}
				}
				after, e := unix.IoctlGetTermios(int(slave.Fd()), unix.TCGETS)
				if e != nil || *after != *before {
					t.Fatalf("terminal not restored: %v", e)
				}
			})
		}
	}
}
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func TestTerminalModePromptDescriptors(t *testing.T) {
	p, master, slave := publicTestTerminal(t, ModeAuto)
	if p.Mode() != ModeRich {
		t.Fatal("prompt pair lost rich capability")
	}
	data, err := os.CreateTemp(t.TempDir(), "data-stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer data.Close()
	if _, err := NewTerminal(TerminalConfig{Input: slave, Output: data, Mode: ModePlain}); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	pipe, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer pipe.Close()
	defer writer.Close()
	if _, err := NewTerminal(TerminalConfig{Input: pipe, Output: slave}); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if _, err := NewTerminal(TerminalConfig{Mode: "typo"}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal(err)
	}
	for _, term := range []string{"", "dumb"} {
		t.Setenv("TERM", term)
		auto, err := NewTerminal(TerminalConfig{Input: slave, Output: slave})
		if err != nil || auto.Mode() != ModePlain {
			t.Fatal(auto, err)
		}
		if _, err := NewTerminal(TerminalConfig{Input: slave, Output: slave, Mode: ModeRich}); !errors.Is(err, ErrUnavailable) {
			t.Fatal(err)
		}
	}
	t.Setenv("TERM", "xterm-256color")
	if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: 8, Col: 30}); err != nil {
		t.Fatal(err)
	}
	tiny, err := NewTerminal(TerminalConfig{Input: slave, Output: slave})
	if err != nil || tiny.Mode() != ModePlain {
		t.Fatal(tiny, err)
	}
	_ = master
}

func TestTerminalPlainMenuCancellationPTY(t *testing.T) {
	p, master, slave := publicTestTerminal(t, ModeRich)
	menu, err := p.PlainUI()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finish := terminalDriver(t, master, func(out string) {
		if strings.Contains(out, "Choose one by number") {
			cancel()
		}
	})
	got, err := menu.SelectOne(ctx, SelectRequest{Title: "Action fixture", Options: []Option{{"inspect", "Inspect"}}})
	finish()
	if got.Accepted || !errors.Is(err, context.Canceled) {
		t.Fatal(got, err)
	}
	_, _ = io.WriteString(master, "next-owner\n")
	ctx, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()
	line, err := promptio.ReadLine(ctx, slave)
	if err != nil || line != "next-owner" {
		t.Fatalf("borrowed handoff %q %v", line, err)
	}
}

func TestTerminalPromptOutputAndRestoreFailure(t *testing.T) {
	for _, mode := range []TerminalMode{ModePlain, ModeRich} {
		t.Run(string(mode), func(t *testing.T) {
			p, master, slave := publicTestTerminal(t, mode)
			before, _ := unix.IoctlGetTermios(int(slave.Fd()), unix.TCGETS)
			sent := false
			finish := terminalDriver(t, master, func(out string) {
				if !sent && (strings.Contains(out, "[Y/n]") || strings.Contains(out, "✓ Yes")) {
					sent = true
					_, _ = io.WriteString(master, "\r")
				}
			})
			fault := errors.New("fixture restore failure")
			ops := &terminalOps{snapshot: func(fd int) (func() error, error) {
				restore, err := promptio.SnapshotTerminal(fd)
				return func() error { return errors.Join(restore(), fault) }, err
			}}
			ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), terminalOpsKey{}, ops), 3*time.Second)
			defer cancel()
			got, err := p.Confirm(ctx, ConfirmRequest{Title: "Apply fixture?", Default: true})
			finish()
			if got.Accepted || !errors.Is(err, fault) {
				t.Fatal(got, err)
			}
			after, e := unix.IoctlGetTermios(int(slave.Fd()), unix.TCGETS)
			if e != nil || *after != *before {
				t.Fatal("actual mode not restored", e)
			}
		})
	}
	t.Run("measured-overflow", func(t *testing.T) {
		p, master, slave := publicTestTerminal(t, ModePlain)
		before, _ := unix.IoctlGetTermios(int(slave.Fd()), unix.TCGETS)
		_, _ = io.WriteString(master, "yes\nnext-owner\n")
		ctx := context.WithValue(context.Background(), terminalOpsKey{}, &terminalOps{queued: func() (int, error) { return 4097, nil }})
		got, err := p.Confirm(ctx, ConfirmRequest{Title: "Apply fixture?", Default: true})
		if got.Accepted || err == nil {
			t.Fatal(got, err)
		}
		line, err := promptio.ReadLine(context.Background(), slave)
		if err != nil || line != "yes" {
			t.Fatal("snapshot consumed queued suffix", line, err)
		}
		after, e := unix.IoctlGetTermios(int(slave.Fd()), unix.TCGETS)
		if e != nil || *after != *before {
			t.Fatal("overflow mode not restored", e)
		}
	})
}

func TestTerminalViewportResizePTY(t *testing.T) {
	for _, tiny := range []bool{false, true} {
		t.Run(fmt.Sprint("tiny=", tiny), func(t *testing.T) {
			p, master, slave := publicTestTerminal(t, ModeRich)
			options := make([]Option, 64)
			for i := range options {
				options[i] = Option{ID: fmt.Sprintf("agent%d", i), Label: fmt.Sprintf("Agent%02d 世界世界世界世界 long label", i)}
			}
			phase := 0
			screen := newTerminalScreen(30, 100)
			offset := 0
			finish := terminalDriver(t, master, func(out string) {
				screen.feed(out[offset:])
				offset = len(out)
				view := screen.text()
				switch phase {
				case 0:
					if strings.Contains(view, "Agent00") {
						phase = 1
						_, _ = io.WriteString(master, " "+strings.Repeat("\x1b[B", 63))
					}
				case 1:
					if strings.Contains(view, "Agent63") {
						phase = 2
						size := unix.Winsize{Row: 12, Col: 43}
						if tiny {
							size = unix.Winsize{Row: 3, Col: 8}
						}
						if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &size); err != nil {
							t.Error(err)
						}
						// Prompt files are intentionally not the process controlling TTY. Tea's
						// joined SIGWINCH listener queries this exact output descriptor.
						screen.resize(int(size.Row), int(size.Col))
						_ = unix.Kill(os.Getpid(), unix.SIGWINCH)
					}
				case 2:
					if !tiny && strings.Contains(view, "Agent63") && strings.Contains(view, "submit") {
						phase = 3
						_, _ = io.WriteString(master, " \r")
					}
				}
			})
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			got, err := p.SelectMany(ctx, MultiSelectRequest{SelectRequest: SelectRequest{Title: "Viewport fixture", Options: options}})
			out := finish()
			if tiny {
				if err == nil || got.Accepted || errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("tiny %+v %v phase=%d screen=%q output=%q", got, err, phase, screen.text(), out)
				}
			} else if err != nil || !reflect.DeepEqual(got.IDs, []string{"agent0", "agent63"}) || !bytes.Contains([]byte(out), []byte("submit")) {
				t.Fatalf("resize %+v %v phase=%d output=%q screen=%q", got, err, phase, out, screen.text())
			}
		})
	}
}

func TestTerminalConfirmationSummaryPTY(t *testing.T) {
	p, master, _ := publicTestTerminal(t, ModeRich)
	summary := []string{strings.Repeat("a", 1000) + "/first-scope-suffix"}
	for i := 0; i < 20; i++ {
		summary = append(summary, fmt.Sprintf("Unit%d: explicit choice", i))
	}
	summary = append(summary, "/other-complete-scope/chosen-final-suffix")
	screen := newTerminalScreen(30, 100)
	offset, phase := 0, 0
	finish := terminalDriver(t, master, func(out string) {
		screen.feed(out[offset:])
		offset = len(out)
		view := screen.text()
		if phase == 0 && strings.Contains(view, "Apply summary?") && strings.Contains(view, "No") {
			phase = 1
			_, _ = io.WriteString(master, "\x1b[F")
		}
		if phase == 1 && strings.Contains(view, "chosen-final-suffix") && strings.Contains(view, "No") {
			phase = 2
			_, _ = io.WriteString(master, "\r")
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	got, err := p.Confirm(ctx, ConfirmRequest{Title: "Apply summary?", Summary: summary})
	out := finish()
	if err != nil || got.Accepted || phase != 2 {
		t.Fatalf("summary consent %+v %v phase=%d screen=%q output=%q", got, err, phase, screen.text(), out)
	}
}

func TestTerminalSelectionValidationResizePTY(t *testing.T) {
	p, master, slave := publicTestTerminal(t, ModeRich)
	screen := newTerminalScreen(30, 100)
	offset, phase := 0, 0
	finish := terminalDriver(t, master, func(out string) {
		screen.feed(out[offset:])
		offset = len(out)
		view := screen.text()
		switch phase {
		case 0:
			if strings.Contains(view, "Minimum fixture") && strings.Contains(view, "submit") {
				phase = 1
				_, _ = io.WriteString(master, "\x1b[B\r")
			}
		case 1:
			if strings.Contains(view, "> [ ] Beta") && strings.Contains(strings.Join(strings.Fields(view), " "), "select at least 1") {
				phase = 2
				size := unix.Winsize{Row: 12, Col: 43}
				if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &size); err != nil {
					t.Error(err)
				}
				screen.resize(int(size.Row), int(size.Col))
				_ = unix.Kill(os.Getpid(), unix.SIGWINCH)
			}
		case 2:
			if strings.Contains(strings.Join(strings.Fields(view), " "), "select at least 1") && strings.Contains(view, "submit") {
				phase = 3
				_, _ = io.WriteString(master, " \r")
			}
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	got, err := p.SelectMany(ctx, MultiSelectRequest{SelectRequest: SelectRequest{Title: "Minimum fixture", Options: []Option{{"alpha", "Alpha"}, {"beta", "Beta"}}}, MinSelected: 1})
	out := finish()
	if err != nil || !reflect.DeepEqual(got.IDs, []string{"beta"}) || phase != 3 {
		t.Fatalf("validation resize %+v %v phase=%d screen=%q output=%q", got, err, phase, screen.text(), out)
	}
}
