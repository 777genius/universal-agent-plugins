//go:build linux

package pluginkitairepo_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

const copilotEmptyRegistryMode = "copilot-empty-registry-first-capture"

var copilotEmptyRegistryCommands = []string{"git-A", "git-B", "version", "native-list"}

func (f *copilotProjectAttempt) emptyRegistryCommand(label, cwd string, argv []string, timeout time.Duration) error {
	sequence, ok := f.evidence["commands"].([]string)
	if !ok || len(sequence) >= 4 || !slices.Equal(sequence, copilotEmptyRegistryCommands[:len(sequence)]) {
		return fmt.Errorf("Copilot empty registry command prefix refused")
	}
	if len(sequence) < 3 {
		return f.userRegistrationCommand(label, cwd, argv, timeout)
	}
	if label != "native-list" || cwd != f.outside || timeout != 30*time.Second || !slices.Equal(argv, []string{f.client, "plugin", "list"}) {
		return fmt.Errorf("Copilot empty registry fixed command/cwd/deadline refused")
	}
	return nil
}

func (f *copilotProjectAttempt) emptyRegistryEvidence() {
	f.userRegistrationEvidence()
	f.evidence["mode"] = copilotEmptyRegistryMode
}

func copilotEmptyRegistryMetadataAfter(ctx context.Context, root string, before *copilotUserMetadata) []byte {
	after, err := copilotUserMetadataCapture(ctx, root)
	if err != nil {
		return copilotMetadataRefusalAtStage("native-list")
	}
	return copilotMetadataDeltaAtStage(before, after, "native-list")
}

func (f *copilotProjectAttempt) observeEmptyRegistry(t *testing.T) {
	t.Helper()
	f.evidence["stage"] = "before-list"
	root := filepath.Join(f.base.Home, ".copilot")
	before, err := copilotUserMetadataCapture(f.ctx, root)
	if err != nil {
		f.keep(t, "user-profile-metadata.json", copilotMetadataRefusalAtStage("native-list"))
		t.Fatal("Copilot original pre-list metadata inventory refused")
	}
	// Only read held metadata after the command, including its original fatal
	// guard. A deferred diagnostic never starts a child or renews a baseline.
	defer func() {
		f.keep(t, "user-profile-metadata.json", copilotEmptyRegistryMetadataAfter(f.ctx, root, before))
	}()
	f.evidence["stage"] = "native-list"
	f.command(t, "native-list", f.outside, f.env(), []string{f.client, "plugin", "list"}, 30*time.Second)
	f.evidence["stage"] = "review-barrier"
	t.Fatal("genuine empty registry listing captured; exact raw review required; all qualification remains false")
}

func copilotEmptyRegistryFiles(files map[string][]byte, sequence []string, stage string) bool {
	if len(sequence) > 4 || !slices.Equal(sequence, copilotEmptyRegistryCommands[:len(sequence)]) {
		return false
	}
	switch stage {
	case "prepare":
		if len(sequence) > 3 {
			return false
		}
	case "before-list":
		if len(sequence) != 3 {
			return false
		}
	case "native-list":
		if len(sequence) < 3 {
			return false
		}
	case "review-barrier":
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
	if len(files) != len(expected) || len(files) > 13 {
		return false
	}
	for name, body := range files {
		if !expected[name] || len(body) > copilotProjectBodyLimit || (name == "user-profile-metadata.json" && len(body) > copilotUserMetadataLimit) {
			return false
		}
	}
	return true
}

func (f *copilotProjectAttempt) finalizeEmptyRegistry(t *testing.T) {
	t.Helper()
	f.evidence["runtime_attempt_complete"] = false
	f.evidence["vendor_authentication_qualified"], f.evidence["real_model_qualified"] = false, false
	f.evidence["qualification"] = map[string]bool{"project-lifecycle": false, "native-restart-removal": false, "mcp-initialize": false, "mcp-tools-list": false, "mcp-tools-call": false}
	f.evidence["probe_status"] = "absent"
	if _, err := os.Lstat(f.base.Events); !os.IsNotExist(err) {
		f.evidence["probe_status"] = "refused"
		t.Error("unexpected probe state in empty registry observation")
	}
	sequence, sequenceOK := f.evidence["commands"].([]string)
	stage, stageOK := f.evidence["stage"].(string)
	if !sequenceOK || !stageOK || !copilotEmptyRegistryFiles(f.files, sequence, stage) {
		t.Error("Copilot empty registry closed prefix/member contract refused")
	}
	if !t.Failed() {
		t.Error("Copilot empty registry observation cannot pass or qualify")
	}
}

func TestAgentpluginsCopilotNativeEmptyRegistryFirstObservation(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_COPILOT_EMPTY_REGISTRY_FIRST_CAPTURE") != "1" {
		t.Skip("opt-in genuine failed-only empty native registry observation")
	}
	f := newCopilotProjectAttempt(t)
	f.emptyRegistryEvidence()
	defer f.finish(t)
	f.preparePackage(t)
	if _, err := os.Lstat(f.base.Events); !os.IsNotExist(err) {
		t.Fatal("unexpected probe events before empty registry observation")
	}
	f.observeEmptyRegistry(t)
}
