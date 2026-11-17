package claude

import (
	"path/filepath"
	"testing"

	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// A missed survivor would let a confirmation close an attempt whose command is
// still changing Claude's registry; an over-broad match would block every user
// with an ordinary Claude Code session open.
func TestSurvivingClaudeCommandRecognizesPluginAndProfileProcesses(t *testing.T) {
	root := t.TempDir()
	executable, profile := filepath.Join(root, "bin", "claude"), filepath.Join(root, "profile")
	for _, tc := range []struct {
		name    string
		process legacyports.LiveProcess
		want    bool
	}{
		{"native plugin command", legacyports.LiveProcess{Executable: "/opt/claude/versions/2.1.325", Argv: []string{executable, "plugin", "install", "demo@m"}}, true},
		{"wrapper script", legacyports.LiveProcess{Executable: "/bin/sh", Argv: []string{"/bin/sh", "/wrap/claude", "plugin", "uninstall", "demo@m"}}, true},
		{"rewritten arguments on this profile", legacyports.LiveProcess{Executable: "claude", Argv: []string{"claude"}, Env: []string{"CLAUDE_CONFIG_DIR=" + profile}}, true},
		{"session on another profile", legacyports.LiveProcess{Argv: []string{"claude", "--resume"}, Env: []string{"CLAUDE_CONFIG_DIR=/elsewhere"}}, false},
		{"other program on this profile", legacyports.LiveProcess{Executable: "/bin/sleep", Argv: []string{"sleep", "30"}, Env: []string{"CLAUDE_CONFIG_DIR=" + profile}}, false},
		{"other program naming claude", legacyports.LiveProcess{Argv: []string{"npm", "plugin", "claude"}}, false},
		{"unreadable claude process", legacyports.LiveProcess{Executable: "claude"}, true},
		{"unreadable other process", legacyports.LiveProcess{Executable: "sleep"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, found := survivingClaudeCommand([]legacyports.LiveProcess{tc.process}, executable, profile); found != tc.want {
				t.Fatalf("survivingClaudeCommand(%+v) = %v, want %v", tc.process, found, tc.want)
			}
		})
	}
}
