//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func copilotEmptyRegistryContractFixture(t *testing.T) *copilotProjectAttempt {
	t.Helper()
	f := copilotProjectConfigFixture(t, []byte("{\"trustedFolders\":[]}\n"))
	home := filepath.Dir(filepath.Dir(f.originalConfigPath))
	root := filepath.Dir(home)
	f.base = &nativeFixture{Root: root, Home: home, Project: filepath.Join(root, "project"), Package: filepath.Join(root, "package"), Events: filepath.Join(root, "events.jsonl")}
	f.other, f.outside = filepath.Join(root, "project-B"), filepath.Join(root, "outside")
	f.client, f.installer, f.git = filepath.Join(root, "inert-list"), filepath.Join(root, "agentplugins"), filepath.Join(root, "git")
	for _, path := range []string{f.base.Project, f.other, f.outside} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	f.emptyRegistryEvidence()
	return f
}

func TestCopilotEmptyRegistryCommandBoundaryHasNoPublicMutation(t *testing.T) {
	f := copilotEmptyRegistryContractFixture(t)
	for index, label := range copilotEmptyRegistryCommands {
		f.evidence["commands"] = append([]string{}, copilotEmptyRegistryCommands[:index]...)
		cwd, argv := f.outside, []string{f.client, "--version"}
		if index < 2 {
			cwd = []string{f.base.Project, f.other}[index]
			argv = []string{f.git, "-c", "core.hooksPath=/dev/null", "init", "--quiet"}
		} else if index == 3 {
			argv = []string{f.client, "plugin", "list"}
		}
		if err := f.emptyRegistryCommand(label, cwd, argv, 30*time.Second); err != nil {
			t.Fatal(err)
		}
		for _, wrong := range []struct {
			label, cwd string
			argv       []string
			limit      time.Duration
		}{
			{"add-user", cwd, argv, 30 * time.Second}, {label, f.base.Home, argv, 30 * time.Second},
			{label, cwd, append(slices.Clone(argv), "--external-uninstalled"), 30 * time.Second}, {label, cwd, argv, time.Minute},
		} {
			if f.emptyRegistryCommand(wrong.label, wrong.cwd, wrong.argv, wrong.limit) == nil {
				t.Fatal("empty registry accepted mutation/foreign cwd/extra flag/extended deadline")
			}
		}
	}
	for _, sequence := range [][]string{{"git-B", "git-A", "version"}, copilotEmptyRegistryCommands, {"native-list"}} {
		f.evidence["commands"] = sequence
		if f.emptyRegistryCommand("native-list", f.outside, []string{f.client, "plugin", "list"}, 30*time.Second) == nil {
			t.Fatal("reordered/repeated/unprepared listing accepted")
		}
	}
}

func TestCopilotEmptyRegistryClosedFilesExcludeEveryProbeAndProvider(t *testing.T) {
	for count := range 5 {
		sequence := append([]string{}, copilotEmptyRegistryCommands[:count]...)
		files := map[string][]byte{}
		for _, label := range sequence {
			for _, suffix := range []string{".command.json", ".stdout.log", ".stderr.log"} {
				files[label+suffix] = nil
			}
		}
		stage := "prepare"
		if count == 4 {
			stage = "review-barrier"
			files["user-profile-metadata.json"] = copilotMetadataRefusalAtStage("native-list")
		}
		if !copilotEmptyRegistryFiles(files, sequence, stage) {
			t.Fatal("fixed empty-registry prefix refused")
		}
		for _, extra := range []string{"probe-events.jsonl", "provider-request-1.json", "add-user.stdout.log", "native-attempt.stdout.log", "private-name"} {
			files[extra] = []byte("PRIVATE-BODY")
			if copilotEmptyRegistryFiles(files, sequence, stage) {
				t.Fatal("unknown/provider/probe/public mutation evidence accepted")
			}
			delete(files, extra)
		}
		if count > 0 {
			name := sequence[count-1] + ".stderr.log"
			files[name] = bytes.Repeat([]byte{'x'}, copilotProjectBodyLimit+1)
			if copilotEmptyRegistryFiles(files, sequence, stage) {
				t.Fatal("oversized stream admitted")
			}
			delete(files, name)
			if copilotEmptyRegistryFiles(files, sequence, stage) {
				t.Fatal("partial command triple admitted")
			}
		}
	}
}

func TestCopilotEmptyRegistryMetadataStageKeepsOriginalBytesAndBounds(t *testing.T) {
	root := copilotUserMetadataFixture(t)
	before := copilotUserMetadataHeld(t, root)
	for index := range 18 {
		projectNativeRuntimeWrite(t, filepath.Join(root, fmt.Sprintf("PRIVATE-%02d", index)), []byte("PRIVATE-BODY"), 0600)
	}
	after := copilotUserMetadataHeld(t, root)
	original := copilotUserMetadataDelta(before, after)
	if !bytes.Equal(original, copilotMetadataDeltaAtStage(before, after, "add-user")) {
		t.Fatal("original add-user projection bytes changed")
	}
	body := copilotMetadataDeltaAtStage(before, after, "native-list")
	var row copilotUserMetadataProjection
	if json.Unmarshal(body, &row) != nil || row.Stage != "native-list" || row.Status != "observed" || len(body) > 2048 || row.BeforeEntries != len(before.nodes) || row.AfterEntries != len(after.nodes) || row.ChangedEntries != len(row.Entries)+row.OmittedCount || row.OmittedCount <= 0 || len(row.Entries) > 16 || bytes.Contains(body, []byte("PRIVATE")) {
		t.Fatal("new metadata stage changed bounded/private/truthful inventory rules")
	}
	if !bytes.Equal(body, copilotMetadataDeltaAtStage(before, after, "native-list")) || !bytes.Equal(original, copilotUserMetadataDelta(before, after)) {
		t.Fatal("projection changed held maps or deterministic original bytes")
	}
	for _, stage := range []string{"", "PRIVATE", "review-barrier"} {
		if copilotMetadataDeltaAtStage(before, after, stage) != nil || copilotMetadataRefusalAtStage(stage) != nil {
			t.Fatal("unknown metadata stage admitted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !bytes.Equal(copilotEmptyRegistryMetadataAfter(ctx, root, before), []byte(`{"stage":"native-list","status":"refused"}`)) {
		t.Fatal("expired context allowed deferred metadata capture")
	}
}
