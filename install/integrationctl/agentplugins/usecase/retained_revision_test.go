package usecase

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type retainedRevisionFixture struct {
	service Service
	store   statev2.Store
	input   AddInput
	data    string
}

func TestRetainedDirectoryRevisionRefusalUsesAuthoritativeState(t *testing.T) {
	for _, test := range []struct {
		name, state string
		marked      bool
	}{
		{"retained data", "retained", true},
		{"purged data with retained preferences", "purged", true},
		{"materialized binding", "active", false},
		{"absent binding still recorded", "absent", false},
		{"direct retained installation", "direct", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newRetainedRevisionFixture(t, test.state)
			incoming := fixture.nextRelease()
			before := fixture.snapshot(t)
			result, err := fixture.service.Add(t.Context(), incoming)
			if err == nil || errors.Is(err, domain.ErrRetainedRevisionChange) != test.marked || result.Mutated {
				t.Fatalf("retained revision classification: result=%+v err=%v", result, err)
			}
			if test.marked {
				assertRetainedRevisionHasNoCommand(t, err)
			} else if !strings.Contains(err.Error(), "run update separately") {
				t.Fatalf("ordinary update advice changed: %v", err)
			}
			if !reflect.DeepEqual(before, fixture.snapshot(t)) {
				t.Fatal("refusal changed recorded state or retained data")
			}
		})
	}
}

func TestRetainedDirectoryRevisionPreservesIntegrityPriority(t *testing.T) {
	for _, test := range []struct {
		name, refusal string
		change        func(*AddInput)
	}{
		{"same release revision", "package-source revision", func(in *AddInput) {
			in.Envelope.Source.ResolvedRevision = strings.Repeat("b", 40)
		}},
		{"same release bytes", "package bytes", func(in *AddInput) {
			in.Envelope.TreeDigest = "sha256:replacement-tree"
		}},
		{"different feed", "feed change requires switch", func(in *AddInput) {
			in.DirectoryResolution.DesiredReleaseSequence++
			in.DirectoryResolution.FeedID = "another-feed"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newRetainedRevisionFixture(t, "retained")
			incoming := fixture.input
			directory := *incoming.DirectoryResolution
			incoming.DirectoryResolution = &directory
			test.change(&incoming)
			before := fixture.snapshot(t)
			result, err := fixture.service.Add(t.Context(), incoming)
			if err == nil || errors.Is(err, domain.ErrRetainedRevisionChange) || !strings.Contains(err.Error(), test.refusal) || result.Mutated {
				t.Fatalf("integrity refusal lost priority: result=%+v err=%v", result, err)
			}
			if !reflect.DeepEqual(before, fixture.snapshot(t)) {
				t.Fatal("integrity refusal changed recorded state or data")
			}
		})
	}
}

func TestRetainedDirectorySameRevisionIsStillAdmitted(t *testing.T) {
	fixture := newRetainedRevisionFixture(t, "retained")
	before := fixture.snapshot(t)
	incoming := fixture.input
	incoming.DryRun, incoming.Confirmed = true, false
	result, err := fixture.service.Add(t.Context(), incoming)
	if err != nil || result.Mutated {
		t.Fatalf("same recorded release preview: result=%+v err=%v", result, err)
	}
	if !reflect.DeepEqual(before, fixture.snapshot(t)) {
		t.Fatal("same-release preview changed retained state or data")
	}
}

func TestRetainedDirectoryGroupRefusalResolvesWithoutInstallationID(t *testing.T) {
	for _, test := range []struct {
		name, state string
		marked      bool
	}{
		{"retained data", "retained", true},
		{"purged preferences record", "purged", true},
		{"materialized binding", "active", false},
		{"absent binding remains recorded", "absent", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newRetainedRevisionFixture(t, test.state)
			request := fixture.groupRequest(t, fixture.nextRelease())
			before := fixture.snapshot(t)
			result, err := fixture.service.AddGroup(t.Context(), request)
			if err == nil || errors.Is(err, domain.ErrRetainedRevisionChange) != test.marked || result.Mutated || len(result.Receipts) != 0 {
				t.Fatalf("group retained revision classification: result=%+v err=%v", result, err)
			}
			if test.marked {
				assertRetainedRevisionHasNoCommand(t, err)
			} else if !strings.Contains(err.Error(), "update separately") {
				t.Fatalf("ordinary group update advice changed: %v", err)
			}
			if !reflect.DeepEqual(before, fixture.snapshot(t)) {
				t.Fatal("group preflight refusal changed retained state or data")
			}
		})
	}
}

func TestRetainedDirectoryGroupPreservesIntegrityPriority(t *testing.T) {
	for _, test := range []struct {
		name, refusal string
		change        func(*AddInput)
	}{
		{"same release revision", "package-source revision", func(in *AddInput) {
			in.Envelope.Source.ResolvedRevision = strings.Repeat("b", 40)
		}},
		{"same release bytes", "package bytes", func(in *AddInput) {
			in.Envelope.TreeDigest = "sha256:replacement-tree"
		}},
		{"different feed", "feed change requires switch", func(in *AddInput) {
			in.DirectoryResolution.DesiredReleaseSequence++
			in.DirectoryResolution.FeedID = "another-feed"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newRetainedRevisionFixture(t, "retained")
			incoming := fixture.input
			directory := *incoming.DirectoryResolution
			incoming.DirectoryResolution = &directory
			test.change(&incoming)
			before := fixture.snapshot(t)
			result, err := fixture.service.AddGroup(t.Context(), fixture.groupRequest(t, incoming))
			if err == nil || errors.Is(err, domain.ErrRetainedRevisionChange) || !strings.Contains(err.Error(), test.refusal) || result.Mutated {
				t.Fatalf("group integrity refusal lost priority: result=%+v err=%v", result, err)
			}
			if !reflect.DeepEqual(before, fixture.snapshot(t)) {
				t.Fatal("group integrity refusal changed recorded state or data")
			}
		})
	}
}

func (fixture retainedRevisionFixture) groupRequest(t *testing.T, incoming AddInput) GroupInput {
	t.Helper()
	incoming.InstallationID = ""
	incoming.DryRun, incoming.Confirmed = true, false
	incoming.Envelope.Source.CanonicalSource = "https://example.com/new-directory-capture"
	incoming.Envelope.Source.RequestedSource = incoming.Envelope.Source.CanonicalSource
	if domain.ComputeSourceBindingID(incoming.Envelope.Source) == domain.ComputeSourceBindingID(fixture.input.Envelope.Source) {
		t.Fatal("group fixture must resolve the stored Directory identity without a matching source binding ID")
	}
	peer := incoming
	peer.Client = domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected,
		ConfigRoot: filepath.Join(t.TempDir(), ".kiro")}
	return GroupInput{Targets: []AddInput{incoming, peer}, OperationGroupID: "retained-group-preview", DryRun: true}
}

func newRetainedRevisionFixture(t *testing.T, mode string) retainedRevisionFixture {
	t.Helper()
	service, store, client := serviceFixture(t)
	input := addInput(t, client, "https://example.com/retained-revision")
	input.OriginMode = domain.OriginModeDirectory
	input.DirectoryResolution = &domain.DirectoryOrigin{ProductID: "demo", DistributionID: "owner/demo",
		DistributionKind: domain.DistributionUpstream, DesiredReleaseSequence: 1,
		SnapshotSchema: 1, SnapshotSequence: 1, SnapshotDigest: "sha256:directory-snapshot"}
	input.Envelope.Source.ResolvedRevision = strings.Repeat("a", 40)
	if mode == "direct" {
		input.OriginMode, input.DirectoryResolution = domain.OriginModeDirect, nil
	}
	input.Confirmed = true
	if _, err := service.Add(t.Context(), input); err != nil {
		t.Fatal(err)
	}
	fixture := retainedRevisionFixture{service: service, store: store, input: input}
	if mode != "active" && mode != "absent" {
		fixture.remove(t)
	}
	if mode == "purged" {
		fixture.purgeKeepingPreferences(t)
	}
	if mode == "absent" {
		fixture.recordAbsentBinding(t)
	}
	return fixture
}

func (fixture *retainedRevisionFixture) remove(t *testing.T) {
	t.Helper()
	_, err := fixture.service.Remove(t.Context(), RemoveInput{Selector: fixture.input.InstallationID,
		Client: fixture.input.Client, Scope: fixture.input.Scope, Confirmed: true, OperationID: "retained-remove"})
	if err != nil {
		t.Fatal(err)
	}
	state, err := fixture.store.Load()
	if err != nil || len(state.Installations) != 1 || len(state.Installations[0].Clients) != 0 || !state.Installations[0].DataRetained || len(state.Installations[0].DataReceipts) != 1 {
		t.Fatalf("last-binding removal did not retain exact record/data: %+v %v", state, err)
	}
	for _, receipt := range state.Installations[0].DataReceipts {
		fixture.data = receipt.Locator
	}
	if err := os.WriteFile(filepath.Join(fixture.data, "user-data"), []byte("keep retained bytes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (fixture retainedRevisionFixture) purgeKeepingPreferences(t *testing.T) {
	t.Helper()
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	state.Installations[0].InstallPreferences = []domain.InstallPreference{{ClientID: domain.ClientKiro,
		Scope: domain.ScopeUser, InstallIntent: domain.InstallIntentPrepare}}
	if err := fixture.store.Save(state); err != nil {
		t.Fatal(err)
	}
	if err := fixture.service.PurgeRetainedData(t.Context(), fixture.input.InstallationID, true); err != nil {
		t.Fatal(err)
	}
	state, err = fixture.store.Load()
	if err != nil || len(state.Installations) != 1 || state.Installations[0].DataRetained || len(state.Installations[0].DataReceipts) != 0 || len(state.Installations[0].InstallPreferences) != 1 || len(state.Installations[0].Clients) != 0 {
		t.Fatalf("purge did not retain only preferences and revision: %+v %v", state, err)
	}
	if _, err := os.Lstat(fixture.data); !os.IsNotExist(err) {
		t.Fatalf("purged data directory survived: %v", err)
	}
}

func (fixture retainedRevisionFixture) recordAbsentBinding(t *testing.T) {
	t.Helper()
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for key, binding := range state.Installations[0].Clients {
		binding.Materialization = domain.MaterializationAbsent
		state.Installations[0].Clients[key] = binding
	}
	if err := fixture.store.Save(state); err != nil {
		t.Fatal(err)
	}
}

func (fixture retainedRevisionFixture) nextRelease() AddInput {
	incoming := fixture.input
	incoming.OperationID = "retained-refusal"
	incoming.Envelope.TreeDigest = "sha256:next-tree"
	incoming.Envelope.ManifestDigest = "sha256:next-manifest"
	incoming.Envelope.Manifest.Version = "2.0.0"
	incoming.Envelope.Source.ResolvedRevision = strings.Repeat("b", 40)
	if incoming.DirectoryResolution != nil {
		directory := *incoming.DirectoryResolution
		directory.DesiredReleaseSequence++
		incoming.DirectoryResolution = &directory
	}
	return incoming
}

func (fixture retainedRevisionFixture) snapshot(t *testing.T) map[string]string {
	t.Helper()
	body, err := os.ReadFile(fixture.store.Path)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{"state": retainedRevisionStamp(t, fixture.store.Path, string(body))}
	if fixture.data != "" {
		if _, err := os.Lstat(fixture.data); os.IsNotExist(err) {
			files["data"] = "<absent>"
		} else {
			files[fixture.data] = retainedRevisionStamp(t, fixture.data, "directory")
			for path, content := range lifecycleFiles(t, fixture.data) {
				files[path] = retainedRevisionStamp(t, path, content)
			}
		}
	}
	return files
}

func retainedRevisionStamp(t *testing.T, path, body string) string {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%v/%d/%s", info.Mode(), info.ModTime().UnixNano(), body)
}

func assertRetainedRevisionHasNoCommand(t *testing.T, err error) {
	t.Helper()
	message := strings.ToLower(err.Error())
	for _, advice := range []string{"run update", "use update", "agentplugins update", "use add", "--purge-data"} {
		if strings.Contains(message, advice) {
			t.Fatalf("unsupported command advice survived: %v", err)
		}
	}
	if !strings.Contains(message, "no installed targets") || !strings.Contains(message, "not supported") {
		t.Fatalf("retained refusal did not explain the unsupported transition: %v", err)
	}
}
