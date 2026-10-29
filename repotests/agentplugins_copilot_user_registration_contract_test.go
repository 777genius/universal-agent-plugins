//go:build linux

package pluginkitairepo_test

import (
	"path/filepath"
	"testing"
	"time"
)

func TestCopilotUserRegistrationClosedCommandMatrixCannotBroadenScope(t *testing.T) {
	root := t.TempDir()
	f := &copilotProjectAttempt{base: &nativeFixture{Root: root, Home: filepath.Join(root, "home"), Project: filepath.Join(root, "project"), Package: filepath.Join(root, "package")},
		other: filepath.Join(root, "project-B"), outside: filepath.Join(root, "outside"), client: filepath.Join(root, "copilot"), installer: filepath.Join(root, "agentplugins"), git: filepath.Join(root, "git"), evidence: map[string]any{}}
	f.userRegistrationEvidence()
	if _, found := f.evidence["provider_kind"]; found {
		t.Fatal("user observation inherited an unobserved provider")
	}
	if _, found := f.evidence["activation"]; found {
		t.Fatal("user observation inherited project activation")
	}
	for i, label := range copilotUserRegistrationCommands {
		f.evidence["commands"] = append([]string{}, copilotUserRegistrationCommands[:i]...)
		cwd := f.outside
		argv := []string{f.client, "--version"}
		switch label {
		case "git-A", "git-B":
			cwd = f.base.Project
			if label == "git-B" {
				cwd = f.other
			}
			argv = []string{f.git, "-c", "core.hooksPath=/dev/null", "init", "--quiet"}
		case "add-user":
			argv = []string{f.installer, "add", f.base.Package, "--no-save", "--scope", "user", "--target", "copilot", "--format", "json"}
		}
		if err := f.userRegistrationCommand(label, cwd, argv, 30*time.Second); err != nil {
			t.Fatal(err)
		}
		for _, wrong := range []struct {
			label, cwd string
			argv       []string
			deadline   time.Duration
		}{{"native-attempt", cwd, argv, 30 * time.Second}, {label, cwd + "/wrong", argv, 30 * time.Second}, {label, cwd, append(append([]string{}, argv...), "--external-uninstalled"), 30 * time.Second}, {label, cwd, argv, 60 * time.Second}} {
			if err := f.userRegistrationCommand(wrong.label, wrong.cwd, wrong.argv, wrong.deadline); err == nil {
				t.Fatal("foreign command/cwd/flag/deadline accepted")
			}
		}
	}
	f.evidence["commands"] = append([]string{}, copilotUserRegistrationCommands...)
	if err := f.userRegistrationCommand("add-user", f.outside, nil, 30*time.Second); err == nil {
		t.Fatal("repeated command accepted")
	}
}

func TestCopilotUserRegistrationClosedArtifactPrefixHasNoProviderOrNativeAllowance(t *testing.T) {
	for n := range 5 {
		sequence := append([]string{}, copilotUserRegistrationCommands[:n]...)
		files := map[string][]byte{}
		for _, label := range sequence {
			for _, suffix := range []string{".command.json", ".stdout.log", ".stderr.log"} {
				files[label+suffix] = nil
			}
		}
		stage := "prepare"
		if n == 4 {
			stage = "public-user"
			files["user-profile-metadata.json"] = copilotUserMetadataRefusal()
		}
		if !copilotUserRegistrationFiles(files, sequence, stage) {
			t.Fatal("genuine finite prefix refused")
		}
		for _, unknown := range []string{"provider-request-1.json", "native-attempt.stdout.log", "mcp.json.installed", "private-file"} {
			files[unknown] = nil
			if copilotUserRegistrationFiles(files, sequence, stage) {
				t.Fatal("unexpected provider/native/file allowance")
			}
			delete(files, unknown)
		}
		if n > 0 {
			delete(files, sequence[n-1]+".stderr.log")
			if copilotUserRegistrationFiles(files, sequence, stage) {
				t.Fatal("partial command triple accepted")
			}
		}
	}
}
