//go:build linux

package pluginkitairepo_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

const copilotUserRegistrationMode = "copilot-user-registry-first-capture"

var copilotUserRegistrationCommands = []string{"git-A", "git-B", "version", "add-user"}

func (f *copilotProjectAttempt) userRegistrationCommand(label, cwd string, argv []string, timeout time.Duration) error {
	sequence, ok := f.evidence["commands"].([]string)
	if !ok || len(sequence) >= len(copilotUserRegistrationCommands) || label != copilotUserRegistrationCommands[len(sequence)] || timeout != 30*time.Second {
		return fmt.Errorf("Copilot user registration command prefix/deadline refused")
	}
	expectedCWD := f.outside
	var expected []string
	switch label {
	case "git-A", "git-B":
		expectedCWD = f.base.Project
		if label == "git-B" {
			expectedCWD = f.other
		}
		expected = []string{f.git, "-c", "core.hooksPath=/dev/null", "init", "--quiet"}
	case "version":
		expected = []string{f.client, "--version"}
	case "add-user":
		expected = []string{f.installer, "add", f.base.Package, "--no-save", "--scope", "user", "--target", "copilot", "--format", "json"}
	}
	if cwd != expectedCWD || !reflect.DeepEqual(argv, expected) {
		return fmt.Errorf("Copilot user registration fixed argv/cwd refused")
	}
	return nil
}

func (f *copilotProjectAttempt) userRegistrationEvidence() {
	// Keep genuine acquisition/source/scanner receipts; no inherited provider or
	// project activation default describes this no-provider user-only operation.
	old := f.evidence
	f.evidence = map[string]any{
		"client": "copilot", "mode": copilotUserRegistrationMode,
		"acquisition": old["acquisition"], "scanner": old["scanner"], "source_identity": old["source_identity"],
		"runtime_attempt_complete": false, "vendor_authentication_qualified": false, "real_model_qualified": false,
		"qualification": map[string]bool{"project-lifecycle": false, "native-restart-removal": false, "mcp-initialize": false, "mcp-tools-list": false, "mcp-tools-call": false},
		"commands":      []string{}, "stage": "prepare",
		"layout": map[string]string{"root": f.base.Root, "home": f.base.Home, "a": f.base.Project, "b": f.other, "outside": f.outside, "package": f.base.Package, "installer": f.installer, "client": f.client, "git": f.git},
	}
}

func (f *copilotProjectAttempt) observeUserRegistration(t *testing.T) {
	t.Helper()
	f.evidence["stage"] = "before-user"
	root := filepath.Join(f.base.Home, ".copilot")
	before, err := copilotUserMetadataCapture(f.ctx, root)
	if err != nil {
		f.keep(t, "user-profile-metadata.json", copilotUserMetadataRefusal())
		t.Fatal("Copilot original pre-user metadata inventory refused")
	}
	// This read-only deferred projection also runs after a fatal post-command
	// config/process guard. It never starts a child or changes that failure.
	defer func() {
		f.keep(t, "user-profile-metadata.json", copilotUserMetadataAfter(f.ctx, root, before))
	}()
	f.evidence["stage"] = "public-user"
	r := f.command(t, "add-user", f.outside, f.env(), []string{f.installer, "add", f.base.Package, "--no-save", "--scope", "user", "--target", "copilot", "--format", "json"}, 30*time.Second)
	f.evidence["stage"] = "public-result"
	if _, err := projectNativeRuntimeJSON(r.Stdout); err != nil {
		t.Fatal("Copilot user public result is not strict bounded JSON")
	}
	if err := nativeValidateInstallerJSON(r.Stdout); err != nil {
		t.Fatal("Copilot user public result versioned envelope refused")
	}
	f.evidence["stage"] = "review-barrier"
	t.Fatal("genuine public user registration captured; exact native registry boundary requires independent raw review; all qualification remains false")
}

func copilotUserMetadataAfter(ctx context.Context, root string, before *copilotUserMetadata) []byte {
	after, err := copilotUserMetadataCapture(ctx, root)
	if err != nil {
		return copilotUserMetadataRefusal()
	}
	return copilotUserMetadataDelta(before, after)
}

func copilotUserRegistrationFiles(files map[string][]byte, sequence []string, stage string) bool {
	if len(sequence) > len(copilotUserRegistrationCommands) || !reflect.DeepEqual(sequence, copilotUserRegistrationCommands[:len(sequence)]) {
		return false
	}
	switch stage {
	case "prepare":
		if len(sequence) > 3 {
			return false
		}
	case "before-user":
		if len(sequence) != 3 {
			return false
		}
	case "public-user":
		if len(sequence) < 3 {
			return false
		}
	case "public-result", "review-barrier":
		if len(sequence) != 4 {
			return false
		}
	default:
		return false
	}
	expected := map[string]bool{}
	for _, label := range sequence {
		for _, suffix := range []string{".command.json", ".stdout.log", ".stderr.log"} {
			expected[label+suffix] = true
		}
	}
	if stage != "prepare" {
		expected["user-profile-metadata.json"] = true
	}
	if _, ok := files["probe-events.jsonl"]; ok {
		expected["probe-events.jsonl"] = true
	}
	if len(files) != len(expected) || len(files) > 14 {
		return false
	}
	for name := range files {
		if !expected[name] || (name == "user-profile-metadata.json" && len(files[name]) > copilotUserMetadataLimit) {
			return false
		}
	}
	return true
}

func (f *copilotProjectAttempt) finalizeUserRegistration(t *testing.T) {
	t.Helper()
	f.evidence["runtime_attempt_complete"] = false
	f.evidence["vendor_authentication_qualified"], f.evidence["real_model_qualified"] = false, false
	f.evidence["qualification"] = map[string]bool{"project-lifecycle": false, "native-restart-removal": false, "mcp-initialize": false, "mcp-tools-list": false, "mcp-tools-call": false}
	f.evidence["probe_status"] = "absent"
	if _, ok := f.files["probe-events.jsonl"]; ok {
		f.evidence["probe_status"] = "captured"
	} else if _, err := os.Lstat(f.base.Events); !os.IsNotExist(err) {
		f.evidence["probe_status"] = "refused"
	}
	sequence, sequenceOK := f.evidence["commands"].([]string)
	stage, stageOK := f.evidence["stage"].(string)
	if !sequenceOK || !stageOK || !copilotUserRegistrationFiles(f.files, sequence, stage) {
		t.Error("Copilot user registration closed prefix/member contract refused")
	}
	if !t.Failed() {
		t.Error("Copilot user registration observation cannot pass or qualify")
	}
}

func TestAgentpluginsCopilotNativeUserRegistrationFirstObservation(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_COPILOT_USER_REGISTRY_FIRST_CAPTURE") != "1" {
		t.Skip("opt-in genuine failed-only user registration observation")
	}
	f := newCopilotProjectAttempt(t)
	f.userRegistrationEvidence()
	defer f.finish(t)
	f.preparePackage(t)
	if _, err := os.Lstat(f.base.Events); !os.IsNotExist(err) {
		t.Fatal("unexpected probe events before public user registration")
	}
	f.observeUserRegistration(t)
}
