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
	node := filepath.Join(root, "bin", "node.exe")
	script := filepath.Join(root, "node_modules", "@anthropic-ai", "claude-code", "cli.js")
	relativeScript := filepath.Join("node_modules", "@anthropic-ai", "claude-code", "cli.js")
	foreignScript := filepath.Join(root, "node_modules", "other", "cli.js")
	for _, tc := range []struct {
		name    string
		process legacyports.LiveProcess
		want    bool
	}{
		{"native plugin command", legacyports.LiveProcess{Executable: "/opt/claude/versions/2.1.325", Argv: []string{executable, "plugin", "install", "demo@m"}}, true},
		{"node plugin command", legacyports.LiveProcess{Executable: node, Argv: []string{node, script, "plugin", "install", "demo@m"}}, true},
		{"relative node plugin command", legacyports.LiveProcess{Executable: node, Argv: []string{node, "." + string(filepath.Separator) + relativeScript, "plugin", "install", "demo@m"}}, true},
		{"relative node script on this profile", legacyports.LiveProcess{Executable: node, Argv: []string{node, relativeScript}, Env: []string{"CLAUDE_CONFIG_DIR=" + profile}}, true},
		{"foreign relative node script on this profile", legacyports.LiveProcess{Executable: node, Argv: []string{node, filepath.Join("node_modules", "other", "cli.js"), "plugin"}, Env: []string{"CLAUDE_CONFIG_DIR=" + profile}}, false},
		{"node rewritten arguments on this profile", legacyports.LiveProcess{Executable: node, Argv: []string{node, script}, Env: []string{"CLAUDE_CONFIG_DIR=" + profile}}, true},
		{"node session on another profile", legacyports.LiveProcess{Executable: node, Argv: []string{node, script, "--resume"}, Env: []string{"CLAUDE_CONFIG_DIR=/elsewhere"}}, false},
		{"foreign node script on this profile", legacyports.LiveProcess{Executable: node, Argv: []string{node, foreignScript, "plugin", "install"}, Env: []string{"CLAUDE_CONFIG_DIR=" + profile}}, false},
		{"node script only named in a later argument", legacyports.LiveProcess{Executable: node, Argv: []string{node, foreignScript, script, "plugin"}, Env: []string{"CLAUDE_CONFIG_DIR=" + profile}}, false},
		{"non-node program naming script on this profile", legacyports.LiveProcess{Executable: "/bin/sleep", Argv: []string{"sleep", script, "plugin"}, Env: []string{"CLAUDE_CONFIG_DIR=" + profile}}, false},
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
