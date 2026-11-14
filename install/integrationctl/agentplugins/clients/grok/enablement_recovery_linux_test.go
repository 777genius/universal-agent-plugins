//go:build linux

package grok

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

type enablementTestState struct {
	statev2.Store
	fail bool
}

func (state *enablementTestState) Save(value domain.StateFileV2) error {
	if state.fail {
		return errors.New("inert state-save failure")
	}
	return state.Store.Save(value)
}

type enablementTestPackage struct{ path, digest string }

func (pkg enablementTestPackage) Verify(_ context.Context, path, digest string) error {
	if path != pkg.path || digest != pkg.digest {
		return errors.New("different package authority")
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return errors.New("owned package missing")
	}
	return nil
}

type enablementFixture struct {
	recovery       EnablementRecovery
	record         enablementRecord
	state          *enablementTestState
	original, next []byte
	config         string
}

func newEnablementFixture(t *testing.T) enablementFixture {
	t.Helper()
	root := t.TempDir()
	project, user, operations := filepath.Join(root, "project"), filepath.Join(root, "home", ".grok"), filepath.Join(root, "state", "operations-v2")
	installationID, bindingID, name := "install-fixture", "grok-project-fixture", "owned"
	artifactID := domain.ComputePhysicalArtifactID(name, installationID)
	artifact := filepath.Join(project, ".grok", "plugins", artifactID)
	for _, path := range []string{artifact, user, operations} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	original := []byte("# PRIVATE_TOKEN\n[plugins]\nenabled=['foreign']\ndisabled=['other']\n")
	config := filepath.Join(user, "config.toml")
	for path, body := range map[string][]byte{config: original, filepath.Join(user, ".config-init.lock"): {}} {
		if err := os.WriteFile(path, body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	packageDigest := enablementDigest([]byte("owned package"))
	binding := domain.ClientBinding{ClientID: "grok", ClientBindingID: bindingID, Scope: "project", NativeProfileRoot: filepath.Join(project, ".grok"),
		TargetLocator: artifact, PhysicalArtifact: artifactID, Materialization: domain.MaterializationMaterialized, Activation: domain.ActivationPrepared,
		Authentication: domain.AuthenticationNotRequired, Verification: domain.VerificationPackageValid, Policy: domain.PolicyAllowed,
		PackageRevision: &domain.ClientPackageRevision{TreeDigest: packageDigest, ManifestDigest: "sha256:manifest"},
		NativeObjects:   []domain.NativeObjectOwnership{{ObjectID: "package:grok:" + artifactID, Kind: "managed_package_directory", Path: artifact, ManagedDigest: packageDigest, ProtectionClass: "managed"}}}
	installation := domain.Installation{InstallationID: installationID, DeclaredName: name, OriginMode: domain.OriginModeDirect,
		Source:  domain.SourceBinding{SourceBindingID: "src_fixture", RequestedSource: "fixture", CanonicalSource: "https://fixture.invalid/owned", ResolvedRevision: "abc", TreeDigest: packageDigest},
		Package: domain.PackageBinding{LoaderKind: domain.LoaderKindAgentPlugins, FormatID: domain.FormatIDAgentPluginsV1, SchemaURI: domain.PluginSchemaV1, DeclaredName: name, ManifestDigest: "sha256:manifest"},
		Clients: map[string]domain.ClientBinding{bindingID: binding}}
	state := &enablementTestState{Store: statev2.Store{Path: filepath.Join(project, ".uap", "state.json"), ProjectRoot: project}}
	if err := state.Save(domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{installation}}); err != nil {
		t.Fatal(err)
	}
	recovery := EnablementRecovery{ProjectRoot: project, UserRoot: user, OperationsDir: operations, State: transaction.Kernel{StateStore: state},
		Kernel: nativeconfig.New(), PackageVerifier: enablementTestPackage{path: artifact, digest: packageDigest}}
	id := "project/" + enablementDigest([]byte(artifact))[7:15] + "/" + name
	edit, after, err := prepareEnablementEdit(nativeconfig.FileSnapshot{Body: original, Mode: 0600, Exists: true}, id, name)
	if err != nil {
		t.Fatal(err)
	}
	record := enablementRecord{Version: 1, Action: "enable", OperationID: "op-fixture", InstallationID: installationID, BindingID: bindingID,
		ProjectRoot: project, UserRoot: user, ArtifactPath: artifact, PackageDigest: packageDigest,
		PluginID: id, PluginName: name, Edit: edit}
	before, err := state.Load()
	if err != nil {
		t.Fatal(err)
	}
	attempt := binding
	attempt.NativeActivationAttempt = record.OperationID
	desired := binding
	desired.NativeObjects = append(append([]domain.NativeObjectOwnership(nil), binding.NativeObjects...), record.object())
	record.BeforeStateDigest, record.AttemptStateDigest, record.DesiredStateDigest = enablementStateDigest(before),
		enablementStateDigest(enablementState(before, 0, attempt)), enablementStateDigest(enablementState(before, 0, desired))
	if err := recovery.writeRecord(record); err != nil {
		t.Fatal(err)
	}
	return enablementFixture{recovery: recovery, record: record, state: state, original: original, next: after, config: config}
}

func (fixture enablementFixture) attempt(t *testing.T, applied, committed bool) {
	t.Helper()
	state := fixture.loaded(t)
	binding := state.Installations[0].Clients[fixture.record.BindingID]
	binding.NativeActivationAttempt = fixture.record.OperationID
	if committed {
		binding.NativeActivationAttempt = ""
		binding.NativeObjects = append(binding.NativeObjects, fixture.record.object())
	}
	state.Installations[0].Clients[fixture.record.BindingID] = binding
	if err := fixture.state.Save(state); err != nil {
		t.Fatal(err)
	}
	if applied {
		if err := os.WriteFile(fixture.config, fixture.next, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEnablementRecoveryReopensEachDurableBoundary(t *testing.T) {
	for _, stage := range []string{"journal", "attempt", "native-write", "state-commit"} {
		t.Run(stage, func(t *testing.T) {
			fixture := newEnablementFixture(t)
			if stage != "journal" {
				fixture.attempt(t, stage != "attempt", stage == "state-commit")
			}
			root, err := fixture.recovery.recordRoot(fixture.record.OperationID)
			if err != nil {
				t.Fatal(err)
			}
			body, err := os.ReadFile(root)
			if err != nil || bytes.Contains(body, []byte("PRIVATE_TOKEN")) {
				t.Fatal("journal persisted foreign configuration", err)
			}
			if err := fixture.recovery.Recover(t.Context()); err != nil {
				t.Fatal(err)
			}
			want := fixture.original
			if stage == "state-commit" {
				want = fixture.next
			}
			current, err := os.ReadFile(fixture.config)
			if err != nil || !bytes.Equal(current, want) {
				t.Fatalf("config after %s: %q, %v", stage, current, err)
			}
			if fixture.loaded(t).Installations[0].Clients[fixture.record.BindingID].NativeActivationAttempt != "" {
				t.Fatal("settled attempt remains pending")
			}
			if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("settled journal retained", err)
			}
		})
	}
}

func TestEnablementRecoveryRetainsConcurrentConfigAndPendingAuthority(t *testing.T) {
	fixture := newEnablementFixture(t)
	fixture.attempt(t, true, false)
	concurrent := append(bytes.Clone(fixture.next), []byte("[models]\nendpoint='FOREIGN_LATE'\n")...)
	if err := os.WriteFile(fixture.config, concurrent, 0600); err != nil {
		t.Fatal(err)
	}
	if err := fixture.recovery.Recover(t.Context()); !errors.Is(err, nativeconfig.ErrConcurrentChange) {
		t.Fatal("third version did not refuse", err)
	}
	current, _ := os.ReadFile(fixture.config)
	if !bytes.Equal(current, concurrent) || fixture.loaded(t).Installations[0].Clients[fixture.record.BindingID].NativeActivationAttempt != fixture.record.OperationID {
		t.Fatal("third version or pending attempt was changed")
	}
	if _, err := fixture.recovery.readRecord(fixture.record.OperationID); err != nil {
		t.Fatal("pending journal lost", err)
	}
}

func TestEnablementRecoveryPreservesStateOnBusyNativeLockAndLateStateFailure(t *testing.T) {
	for _, failure := range []string{"lock", "state"} {
		t.Run(failure, func(t *testing.T) {
			fixture := newEnablementFixture(t)
			fixture.attempt(t, true, false)
			if failure == "lock" {
				release, err := lockNativeEnablement(fixture.recovery.UserRoot)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = release() }()
			} else {
				fixture.state.fail = true
			}
			if err := fixture.recovery.Recover(t.Context()); err == nil {
				t.Fatal("failed recovery reported completion")
			}
			if fixture.loaded(t).Installations[0].Clients[fixture.record.BindingID].NativeActivationAttempt == "" {
				t.Fatal("failure cleared the pending attempt")
			}
			if _, err := fixture.recovery.readRecord(fixture.record.OperationID); err != nil {
				t.Fatal("failure deleted recovery authority", err)
			}
			if failure == "state" {
				fixture.state.fail = false
				if err := fixture.recovery.Recover(t.Context()); err != nil {
					t.Fatal("retry cannot settle restored bytes", err)
				}
			}
		})
	}
}

func TestEnablementRecoveryRejectsChangedCompleteBindingAndUnknownJournalMember(t *testing.T) {
	for _, change := range []string{"binding", "native-id", "record-mode"} {
		t.Run(change, func(t *testing.T) {
			fixture := newEnablementFixture(t)
			fixture.attempt(t, true, false)
			switch change {
			case "binding":
				state := fixture.loaded(t)
				binding := state.Installations[0].Clients[fixture.record.BindingID]
				binding.Activation = domain.ActivationActive
				state.Installations[0].Clients[fixture.record.BindingID] = binding
				if err := fixture.state.Save(state); err != nil {
					t.Fatal(err)
				}
			case "native-id":
				fixture.recovery.ProjectRoot += "-foreign"
			default:
				root, _ := fixture.recovery.recordRoot(fixture.record.OperationID)
				if err := os.Chmod(root, 0644); err != nil {
					t.Fatal(err)
				}
			}
			if err := fixture.recovery.Recover(t.Context()); err == nil {
				t.Fatal("changed recovery authority admitted")
			}
			current, _ := os.ReadFile(fixture.config)
			if !bytes.Equal(current, fixture.next) {
				t.Fatal("unowned restoration changed native config")
			}
		})
	}
}

func (fixture enablementFixture) loaded(t *testing.T) domain.StateFileV2 {
	t.Helper()
	state, err := fixture.state.Load()
	if err != nil {
		t.Fatal(err)
	}
	return state
}
