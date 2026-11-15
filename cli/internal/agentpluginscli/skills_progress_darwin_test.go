//go:build darwin

package agentpluginscli

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/charmbracelet/x/ansi"
	"golang.org/x/sys/unix"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func TestSinglePluginUsesSharedLiveProgressPTY(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	master, slave := skillsProgressPTY(t)
	if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Col: 80, Row: 24}); err != nil {
		t.Fatal(err)
	}
	board := startGroupProgressBoard(App{Terminal: true, ErrorOutput: slave}, &options{format: "human"}, []domain.DetectedClient{{ClientID: domain.ClientCodex, DisplayName: "Codex"}})
	if board == nil {
		t.Fatal("single selected plugin agent lost the shared board")
	}
	defer board.finish(nil)
	if !board.live || len(board.rows) != 1 {
		t.Fatal("single-agent board was not live")
	}
	board.observe(usecase.GroupProgressEvent{ClientID: domain.ClientCodex, Phase: usecase.GroupProgressPreparing})
	if board.rows[0].step != progressPreparing {
		t.Fatal("native stage not observed")
	}
	fds := []unix.PollFd{{Fd: int32(master.Fd()), Events: unix.POLLIN}}
	if n, err := unix.Poll(fds, 100); err != nil || n == 0 {
		t.Fatalf("no rendered native progress: %d %v", n, err)
	}
	var buffer [4096]byte
	n, err := master.Read(buffer[:])
	if err != nil || !strings.Contains(string(buffer[:n]), "Codex") || !strings.Contains(string(buffer[:n]), "preparing") {
		t.Fatalf("missing native single-agent frame: %q %v", buffer[:n], err)
	}
}

func TestSkillsProgressPTYBindingCountsAndResize(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	master, slave := skillsProgressPTY(t)
	size := func(cols, rows uint16) {
		t.Helper()
		if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Col: cols, Row: rows}); err != nil {
			t.Fatal(err)
		}
	}
	read := func() string {
		t.Helper()
		var output strings.Builder
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			fds := []unix.PollFd{{Fd: int32(master.Fd()), Events: unix.POLLIN}}
			n, err := unix.Poll(fds, 20)
			if errors.Is(err, unix.EINTR) {
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				return output.String()
			}
			var buffer [4096]byte
			n, err = master.Read(buffer[:])
			if err != nil {
				t.Fatal(err)
			}
			output.Write(buffer[:n])
		}
		t.Fatal("progress PTY did not become idle")
		return ""
	}
	checkWide := func(frame string) {
		t.Helper()
		for _, line := range strings.Split(ansi.Strip(frame), "\n") {
			if ansi.StringWidth(strings.Trim(line, "\r")) >= 80 {
				t.Fatalf("wide frame wraps: %q", frame)
			}
		}
	}
	size(80, 24)
	actions := []SkillsAction{
		{Alias: "canvas-design", ClientID: "codex", Action: "install"}, {Alias: "trust-skill", ClientID: "codex", Action: "install"},
		{Alias: "canvas-design", ClientID: "claude", Action: "install"}, {Alias: "trust-skill", ClientID: "claude", Action: "install"},
	}
	progress := startSkillsProgressBoard(App{Terminal: true, ErrorOutput: slave}, &options{format: "human"}, actions)
	finished := false
	t.Cleanup(func() {
		if !finished {
			progress.finish(SkillsBatchResult{Status: "failed"})
		}
	})
	initial := read()
	if !progress.board.live || strings.Count(initial, "0/2 queued") != 2 {
		t.Fatalf("not a live two-agent initial board: %q", initial)
	}
	checkWide(initial)
	progress.observe(SkillsProgress{Alias: "canvas-design", ClientID: "codex", Action: "install", Phase: "preparing"})
	preparing := read()
	if !strings.Contains(preparing, "\x1b[2A") || !strings.Contains(preparing, "canvas-design") {
		t.Fatalf("active Skill not updated in place: %q", preparing)
	}
	checkWide(preparing)
	progress.observe(SkillsProgress{Alias: "canvas-design", ClientID: "codex", Action: "install", Phase: "finished", Result: installer.Result{Outcome: installer.OutcomeCompleted, Client: completedSkillClient()}})
	first := read()
	if !strings.Contains(first, "1/2 waiting") || strings.Contains(first, "✓ 1/2 installed") {
		t.Fatalf("first binding prematurely finished the agent: %q", first)
	}
	checkWide(first)
	attention := installer.ClientResult{Activation: string(domain.ActivationPrepared), Verification: string(domain.VerificationInstalled), Authentication: string(domain.AuthenticationNotRequired)}
	progress.observe(SkillsProgress{Alias: "trust-skill", ClientID: "codex", Action: "install", Phase: "installing"})
	installing := read()
	if !strings.Contains(installing, "trust-skill") {
		t.Fatalf("next Skill not visible: %q", installing)
	}
	checkWide(installing)
	progress.observe(SkillsProgress{Alias: "trust-skill", ClientID: "codex", Action: "install", Phase: "finished", Result: installer.Result{Outcome: installer.OutcomeCompleted, Client: attention}})
	prepared := read()
	if !strings.Contains(prepared, "! 2/2 setup required") || strings.Contains(prepared, "✓ 2/2 installed") {
		t.Fatalf("trust requirement painted green: %q", prepared)
	}
	checkWide(prepared)
	size(40, 12)
	progress.observe(SkillsProgress{Alias: "canvas-design", ClientID: "claude", Action: "install", Phase: "preparing"})
	shrunk := read()
	if progress.board.live || strings.Contains(shrunk, "\x1b[") {
		t.Fatalf("resize reused stale cursor coordinates: %q", shrunk)
	}
	size(80, 24)
	progress.finish(SkillsBatchResult{Status: "partial", Items: []SkillsBatchItem{
		{Alias: "canvas-design", ClientID: "codex", Action: "install", Status: "completed", Client: completedSkillClient()},
		{Alias: "trust-skill", ClientID: "codex", Action: "install", Status: "completed", Client: attention},
		{Alias: "canvas-design", ClientID: "claude", Action: "install", Status: "failed"},
		{Alias: "trust-skill", ClientID: "claude", Action: "install", Status: "not_attempted"},
	}})
	finished = true
	final := read()
	if strings.Contains(final, "\x1b[") || !strings.Contains(final, "0/2 failed") || !strings.Contains(final, "2/2 setup required") {
		t.Fatalf("final counts or fallback changed: %q", final)
	}
	if progress.items[skillsBindingKey("trust-skill", "claude", "install")].Status != "not_attempted" {
		t.Fatal("unattempted second Claude binding lost its state")
	}
}

// Each regression owns its new PTY; no inherited /dev/tty or user profile is used.
func skillsProgressPTY(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY|unix.O_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = master.Close() })
	var name [128]byte
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, master.Fd(), unix.TIOCPTYGNAME, uintptr(unsafe.Pointer(&name[0]))) //nolint:staticcheck // x/sys has no typed wrapper for this Darwin PTY name ioctl.
	if errno != 0 {
		t.Fatal(errno)
	}
	for _, op := range []uint{unix.TIOCPTYGRANT, unix.TIOCPTYUNLK} {
		if err := unix.IoctlSetInt(int(master.Fd()), op, 0); err != nil {
			t.Fatal(err)
		}
	}
	slave, err := os.OpenFile(strings.TrimRight(string(name[:]), "\x00"), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = slave.Close() })
	return master, slave
}
