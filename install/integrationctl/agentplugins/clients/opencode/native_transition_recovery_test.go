package opencode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/opencodehost"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
)

type transitionFixtureData struct {
	request domain.ActivationRequest
	store   statev2.Store
	source  []byte
}

func transitionTestWrite(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
}
func providerTransitionFixture(t *testing.T) transitionFixtureData {
	t.Helper()
	root := t.TempDir()
	config, active := filepath.Join(root, "config"), filepath.Join(root, "package")
	paths := transitionPaths(config)
	transitionTestWrite(t, paths.JSON, []byte("{ // keep user trivia\n\"theme\":\"fixture\",\"mcp\":{}}"))
	// JSONC keeps exact user comments, whitespace and original permissions.
	if err := os.Rename(paths.JSON, paths.JSONC); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(paths.JSONC, 0640); err != nil {
		t.Fatal(err)
	}
	server := nativeconfig.Server{Type: "remote", URL: "https://fixture.invalid/mcp"}
	receipt, err := nativeconfig.New().Apply(nativeconfig.Request{Paths: paths, Codec: nativeconfig.CodecOpenCode, Name: "owned", Action: nativeconfig.ActionAdd, Server: server})
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(paths.JSONC)
	if err != nil {
		t.Fatal(err)
	}
	desired, err := nativeconfig.DesiredReceipt(paths.JSONC, nativeconfig.CodecOpenCodeV2, "owned", server, nativeconfig.Placeholders{})
	if err != nil {
		t.Fatal(err)
	}
	projection := OpenCodeProjection{Version: 2, Dialect: opencodehost.DialectV2, ConfigPath: paths.JSONC, ConfigJSON: paths.JSON, ConfigJSONC: paths.JSONC, PackageRoot: active, MCPServers: map[string]nativeconfig.Server{"owned": server}}
	body, _ := json.Marshal(projection)
	transitionTestWrite(t, filepath.Join(active, OpenCodeProjectionFile), body)
	old := domain.NativeObjectOwnership{ObjectID: "opencode-mcp:owned", Kind: OpenCodeMCPObjectKind, LogicalName: "owned", Path: paths.JSONC, ManagedDigest: receipt.Digest, ProtectionClass: "managed"}
	next := old
	next.Kind = OpenCodeV2MCPObjectKind
	next.ManagedDigest = desired.Digest
	oldSkill := domain.NativeObjectOwnership{ObjectID: "opencode-skill:skill", Kind: openCodeSkillKind, LogicalName: "skill", Path: filepath.Join(config, "skills", "skill"), SourceRelative: "skills/skill", ProtectionClass: "managed"}
	transitionTestWrite(t, filepath.Join(oldSkill.Path, "SKILL.md"), []byte("source skill\n"))
	oldSkill.ManagedDigest, err = shared.DigestSkillDirectory(oldSkill.Path)
	if err != nil {
		t.Fatal(err)
	}
	newSkill := oldSkill
	transitionTestWrite(t, filepath.Join(active, "skills", "skill", "SKILL.md"), []byte("target skill\n"))
	newSkill.ManagedDigest, err = shared.DigestSkillDirectory(filepath.Join(active, "skills", "skill"))
	if err != nil {
		t.Fatal(err)
	}
	id := domain.NativeAttemptIdentity{OperationID: "TEST-attempt", InstallationID: "TEST-installation", BindingID: domain.ComputeClientBindingID("TEST-installation", "opencode", "user", active), NativeRoot: config}
	b := domain.ClientBinding{ClientBindingID: id.BindingID, ClientID: "opencode", Scope: "user", TargetLocator: active, PhysicalArtifact: domain.ComputePhysicalArtifactID("demo", id.InstallationID), Materialization: domain.MaterializationMaterialized, Activation: domain.ActivationPrepared, Authentication: domain.AuthenticationNotRequired, Verification: domain.VerificationPackageValid, Policy: domain.PolicyAllowed, NativeActivationAttempt: id.OperationID, NativeObjects: []domain.NativeObjectOwnership{old, oldSkill}, PackageRevision: &domain.ClientPackageRevision{TreeDigest: "sha256:tree", ManifestDigest: "sha256:manifest"}}
	artifact, err := (packagesnapshot.Builder{}).Build(context.Background(), active)
	if err != nil {
		t.Fatal(err)
	}
	digest := artifact.Digest
	if err := artifact.Close(); err != nil {
		t.Fatal(err)
	}
	packageObject := domain.NativeObjectOwnership{ObjectID: "package:opencode:" + b.PhysicalArtifact, Kind: "managed_package_directory", LogicalName: "demo", Path: active, ManagedDigest: digest, ProtectionClass: "managed"}
	b.NativeObjects = append(b.NativeObjects, packageObject)
	state := domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{{InstallationID: id.InstallationID, DeclaredName: "demo", Source: domain.SourceBinding{SourceBindingID: "src_demo", RequestedSource: "demo", CanonicalSource: "https://fixture.invalid/demo", ResolvedRevision: "abc", TreeDigest: "sha256:tree"}, Package: domain.PackageBinding{LoaderKind: domain.LoaderKindAgentPlugins, FormatID: domain.FormatIDAgentPluginsV1, SchemaURI: domain.PluginSchemaV1, DeclaredName: "demo", ManifestDigest: "sha256:manifest"}, Clients: map[string]domain.ClientBinding{id.BindingID: b}}}}
	store := statev2.Store{Path: filepath.Join(root, "state.json")}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	return transitionFixtureData{request: domain.ActivationRequest{NativeAttempt: id, Client: domain.DetectedClient{ClientID: domain.ClientOpenCode, ConfigRoot: config}, Delivery: domain.StagedDelivery{ActivePath: active, NativeObjects: []domain.NativeObjectOwnership{next, newSkill, packageObject}}, PreviousNativeObjects: b.NativeObjects}, store: store, source: source}
}
func freshTransitions(f transitionFixtureData) NativeTransitions {
	return NativeTransitions{PackageVerifier: transitionTestPackageVerifier{}, State: transaction.Kernel{StateStore: statev2.Store{Path: f.store.Path}}, Kernel: nativeconfig.New()}
}

// Reopened recovery must infer bytes/state, never trust a phase. Each crash
// occurs after real persisted preimage/rename/write/state/cleanup effects.
func TestNativeTransitionReopensEveryDurableBoundary(t *testing.T) {
	for _, crash := range []string{"prepared", "backup", "skill", "native", "state", "cleanup"} {
		t.Run(crash, func(t *testing.T) {
			f := providerTransitionFixture(t)
			t0 := freshTransitions(f)
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			wire := transitionCrashWire{Identity: f.request.NativeAttempt, ActivePath: f.request.Delivery.ActivePath, Previous: f.request.PreviousNativeObjects, Desired: f.request.Delivery.NativeObjects, StatePath: f.store.Path}
			body, _ := json.Marshal(wire)
			fixture := filepath.Join(filepath.Dir(f.store.Path), "TEST-child.json")
			transitionTestWrite(t, fixture, body)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, executable, "-test.run=^TestNativeTransitionCrashChild$", "--", fixture, crash)
			cmd.Dir = filepath.Dir(f.store.Path)
			cmd.Env = []string{"UAP_TEST_TRANSITION_CHILD=1", "HOME=" + cmd.Dir, "TMPDIR=" + cmd.Dir}
			output, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 81 {
				t.Fatalf("TEST child did not reach crash boundary: %v %s", err, output)
			}
			pending, err := t0.Pending()
			if err != nil || len(pending) != 1 || pending[0].Root == "" {
				t.Fatalf("child lost record: %+v %v", pending, err)
			}
			journalRoot := pending[0].Root

			reopened := freshTransitions(f)
			if err := reopened.Recover(context.Background()); err != nil {
				t.Fatal(err)
			}
			if err := reopened.Recover(context.Background()); err != nil {
				t.Fatal("non-idempotent recovery", err)
			}
			state, err := f.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			b := state.Installations[0].Clients[f.request.NativeAttempt.BindingID]
			if b.NativeActivationAttempt != "" {
				t.Fatal("unresolved attempt after known outcome")
			}
			want := nativeconfig.CodecOpenCode
			skill := "source skill\n"
			if crash == "native" || crash == "state" || crash == "cleanup" {
				want = nativeconfig.CodecOpenCodeV2
				skill = "target skill\n"
			}
			codec, _, _ := nativeconfig.OpenCodeCodecForKind(b.NativeObjects[0].Kind)
			if codec != want {
				t.Fatalf("confirmed receipt codec %s, want %s", codec, want)
			}
			if err := VerifyOpenCodeNativeObjects(f.request.Client.ConfigRoot, "", b.NativeObjects, nativeconfig.New(), false); err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(filepath.Join(f.request.Client.ConfigRoot, "skills", "skill", "SKILL.md"))
			if err != nil || string(content) != skill {
				t.Fatal("wrong recovered skill", err)
			}
			if want == nativeconfig.CodecOpenCode {
				body, _ := os.ReadFile(transitionPaths(f.request.Client.ConfigRoot).JSONC)
				if !bytes.Equal(body, f.source) {
					t.Fatal("source recovery changed exact foreign bytes")
				}
			}
			if _, err := os.Stat(journalRoot); !os.IsNotExist(err) {
				t.Fatal("settled journal retained", err)
			}
		})
	}
}

type transitionCrashWire struct {
	Identity              domain.NativeAttemptIdentity
	ActivePath, StatePath string
	Previous, Desired     []domain.NativeObjectOwnership
}

// This inert compiled TEST child is always exercised by the parent matrix;
// abrupt exit drops all memory and lets the OS release native file leases.
func TestNativeTransitionCrashChild(t *testing.T) {
	if os.Getenv("UAP_TEST_TRANSITION_CHILD") != "1" {
		return
	}
	args := os.Args[len(os.Args)-2:]
	body, err := os.ReadFile(args[0])
	if err != nil {
		t.Fatal(err)
	}
	var wire transitionCrashWire
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatal(err)
	}
	crash := args[1]
	request := domain.ActivationRequest{NativeAttempt: wire.Identity, Client: domain.DetectedClient{ClientID: domain.ClientOpenCode, ConfigRoot: wire.Identity.NativeRoot}, Delivery: domain.StagedDelivery{ActivePath: wire.ActivePath, NativeObjects: wire.Desired}, PreviousNativeObjects: wire.Previous}
	f := transitionFixtureData{request: request, store: statev2.Store{Path: wire.StatePath}}
	transitions := freshTransitions(f)
	p, err := prepareOpenCodeNativeApply(request.Client.ConfigRoot, request.Delivery.ActivePath, request.PreviousNativeObjects, request.Delivery.NativeObjects, nativeconfig.New(), shared.RenameDirectoryExclusive, os.RemoveAll)
	if err != nil {
		t.Fatal(err)
	}
	req, err := openCodeTransitionRequest(p)
	if err != nil {
		t.Fatal(err)
	}
	txn := &openCodeSkillTxn{durable: true, rename: shared.RenameDirectoryExclusive, removeAll: os.RemoveAll, backups: map[string]openCodeBackup{}, installed: map[string]domain.NativeObjectOwnership{}}
	if err := txn.createRoot(request.Client.ConfigRoot); err != nil {
		t.Fatal(err)
	}
	staged, err := txn.stageDesired(request.Delivery.ActivePath, shared.ObjectMap(p.desired))
	if err != nil {
		t.Fatal(err)
	}
	if err := syncTransitionTree(txn.root); err != nil {
		t.Fatal(err)
	}
	req.PersistPrepared = func(v nativeconfig.PreparedTransition) error {
		if err := transitions.PersistPrepared(context.Background(), request.NativeAttempt, txn.root, transitionDTO(v), request.PreviousNativeObjects, request.Delivery.NativeObjects); err != nil {
			return err
		}
		if crash == "prepared" {
			os.Exit(81)
		}
		if err := txn.backupPrevious(shared.ObjectMap(p.previous)); err != nil {
			return err
		}
		if crash == "backup" {
			os.Exit(81)
		}
		if err := txn.installStaged(staged, shared.ObjectMap(p.desired)); err != nil {
			return err
		}
		if crash == "skill" {
			os.Exit(81)
		}
		return nil
	}
	if _, err := nativeconfig.New().ApplyDialectTransition(req); err != nil {
		t.Fatal(err)
	}
	if crash == "state" || crash == "cleanup" {
		r, err := readTransitionRecord(txn.root)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.store.Save(r.TargetState); err != nil {
			t.Fatal(err)
		}
		if crash == "cleanup" {
			r.Phase = "cleanup_pending"
			if err := writeTransitionRecord(r); err != nil {
				t.Fatal(err)
			}
			if err := os.RemoveAll(r.Skills[0].Backup); err != nil {
				t.Fatal(err)
			}
			if err := atomicfile.SyncDirectory(r.Root); err != nil {
				t.Fatal(err)
			}
		}
	}
	os.Exit(81)
}

// File-backed ambiguous atomic state saves exercise the real existing kernel
// authority. Desired-visible durability failure and unknown reload retain both
// target and complete record; old reload restores exact source and source skills.
type ambiguousTransitionStore struct {
	statev2.Store
	mode   string
	failed bool
	after  func()
}

func (s *ambiguousTransitionStore) Save(state domain.StateFileV2) error {
	if !s.failed {
		s.failed = true
		if s.after != nil {
			s.after()
		}
		if s.mode == "desired" || s.mode == "desired-unresolved" {
			if err := s.Store.Save(state); err != nil {
				return err
			}
		}
		return errors.New("TEST state sync failure")
	}
	if s.mode == "desired-unresolved" {
		return errors.New("TEST durability still unresolved")
	}
	return s.Store.Save(state)
}
func (s *ambiguousTransitionStore) Load() (domain.StateFileV2, error) {
	if s.failed && s.mode == "unknown" {
		return domain.StateFileV2{}, errors.New("TEST visibility unavailable")
	}
	return s.Store.Load()
}
func TestNativeTransitionStateSaveAmbiguity(t *testing.T) {
	for _, mode := range []string{"old", "desired", "desired-unresolved", "unknown", "old-user-edit"} {
		t.Run(mode, func(t *testing.T) {
			f := providerTransitionFixture(t)
			store := &ambiguousTransitionStore{Store: f.store, mode: mode}
			if mode == "old-user-edit" {
				store.after = func() {
					transitionTestWrite(t, transitionPaths(f.request.Client.ConfigRoot).JSONC, []byte(`{"foreign":"edited before restore"}`))
				}
			}
			rec := NativeTransitions{PackageVerifier: transitionTestPackageVerifier{}, State: transaction.Kernel{StateStore: store}, Kernel: nativeconfig.New()}
			err := applyOpenCodeTransition(context.Background(), f.request, nativeconfig.New(), rec)
			paths := transitionPaths(f.request.Client.ConfigRoot)
			body, _ := os.ReadFile(paths.JSONC)
			switch mode {
			case "old":
				if err == nil || !bytes.Equal(body, f.source) {
					t.Fatal("old state did not restore exact source")
				}
			case "desired":
				if err != nil {
					t.Fatal(err)
				}
				if bytes.Equal(body, f.source) {
					t.Fatal("desired-visible state rolled back")
				}
			case "old-user-edit":
				if err == nil || string(body) != `{"foreign":"edited before restore"}` {
					t.Fatal("foreign edit lost")
				}
			default:
				if err == nil || bytes.Equal(body, f.source) {
					t.Fatal("unknown/desired-unresolved guessed rollback")
				}
			}
			pending, pErr := freshTransitions(f).Pending()
			if pErr != nil {
				t.Fatal(pErr)
			}
			if mode == "old" || mode == "desired" {
				if len(pending) != 0 {
					t.Fatal("settled outcome retained residue")
				}
			} else {
				if len(pending) != 1 || pending[0].Root == "" {
					t.Fatal("uncertainty lost complete journal")
				}
				if mode != "old-user-edit" {
					if err := freshTransitions(f).Recover(context.Background()); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}

// A known committed unlock error cannot run the old deferred skill rollback;
// target receipts, state and skills are still the one confirmed effect.
func TestNativeTransitionCommittedUnlockKeepsReceiptsAndSkills(t *testing.T) {
	f := providerTransitionFixture(t)
	degraded := nativeconfig.NewWithLockAcquirer(func(nativeconfig.Paths, nativeconfig.Codec) (func() error, error) {
		return func() error { return errors.New("TEST unlock degradation") }, nil
	})
	err := applyOpenCodeTransition(context.Background(), f.request, degraded, freshTransitions(f))
	if !nativeconfig.IsCommittedCleanup(err) {
		t.Fatal("lost committed cleanup disposition", err)
	}
	state, err := f.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	b := state.Installations[0].Clients[f.request.NativeAttempt.BindingID]
	if len(b.NativeObjects) != 3 || b.NativeActivationAttempt != "" {
		t.Fatal("committed cleanup lost full receipts")
	}
	if err := VerifyOpenCodeNativeObjects(f.request.Client.ConfigRoot, "", b.NativeObjects, nativeconfig.New(), false); err != nil {
		t.Fatal(err)
	}
	skill, _ := os.ReadFile(filepath.Join(f.request.Client.ConfigRoot, "skills", "skill", "SKILL.md"))
	if string(skill) != "target skill\n" {
		t.Fatal("committed skills rolled back")
	}
}

// Corrupt/oversized/unbound records must remain visible without any overwrite.
// Re-sealed test corruption also exercises validation beyond the checksum.
func TestNativeTransitionRecordRejectsCorruptAuthority(t *testing.T) {
	for _, fault := range []string{"version", "type", "phase", "path", "codec", "target-hash", "receipt", "binding", "skill-path", "projection", "unknown-field", "duplicate-field", "trailing", "checksum", "oversize", "symlink", "foreign-native", "source-target-state"} {
		t.Run(fault, func(t *testing.T) {
			f := providerTransitionFixture(t)
			rec := freshTransitions(f)
			p, err := prepareOpenCodeNativeApply(f.request.Client.ConfigRoot, f.request.Delivery.ActivePath, f.request.PreviousNativeObjects, f.request.Delivery.NativeObjects, nativeconfig.New(), shared.RenameDirectoryExclusive, os.RemoveAll)
			if err != nil {
				t.Fatal(err)
			}
			req, err := openCodeTransitionRequest(p)
			if err != nil {
				t.Fatal(err)
			}
			txn := &openCodeSkillTxn{rename: shared.RenameDirectoryExclusive, removeAll: os.RemoveAll}
			if err := txn.createRoot(f.request.Client.ConfigRoot); err != nil {
				t.Fatal(err)
			}
			if _, err := txn.stageDesired(f.request.Delivery.ActivePath, shared.ObjectMap(p.desired)); err != nil {
				t.Fatal(err)
			}
			stop := errors.New("TEST stop after prepared")
			req.PersistPrepared = func(v nativeconfig.PreparedTransition) error {
				if err := rec.PersistPrepared(context.Background(), f.request.NativeAttempt, txn.root, transitionDTO(v), f.request.PreviousNativeObjects, f.request.Delivery.NativeObjects); err != nil {
					return err
				}
				return stop
			}
			if _, err := nativeconfig.New().ApplyDialectTransition(req); !errors.Is(err, stop) {
				t.Fatal(err)
			}
			r, err := readTransitionRecord(txn.root)
			if err != nil {
				t.Fatal(err)
			}
			switch fault {
			case "version":
				r.Version = 99
			case "type":
				r.Type = "foreign"
			case "phase":
				r.Phase = "unknown"
			case "path":
				r.Native.Path = filepath.Join(t.TempDir(), "foreign.json")
			case "codec":
				r.Native.SourceCodec = r.Native.TargetCodec
			case "target-hash":
				r.Native.TargetHash = r.ProjectionHash
			case "receipt":
				r.Native.Entries[0].TargetReceipt.Digest = r.Native.Entries[0].SourceReceipt.Digest
			case "binding":
				r.Identity.BindingID = "foreign-binding"
			case "skill-path":
				r.Skills[0].Backup = filepath.Join(t.TempDir(), "foreign-skill")
			case "projection":
				transitionTestWrite(t, filepath.Join(f.request.Delivery.ActivePath, OpenCodeProjectionFile), []byte(`{"foreign":true}`))
				transitionTestWrite(t, r.Native.Path, r.Native.TargetBytes)
			case "foreign-native":
				transitionTestWrite(t, r.Native.Path, []byte(`{"foreign":"edited"}`))
			case "source-target-state":
				if err := f.store.Save(r.TargetState); err != nil {
					t.Fatal(err)
				}
			}
			r.Hash = ""
			unsigned, _ := json.Marshal(r)
			r.Hash = transitionHash(unsigned)
			body, _ := json.Marshal(r)
			switch fault {
			case "unknown-field":
				body = append(body[:len(body)-1], []byte(`,"Unknown":true}`)...)
			case "duplicate-field":
				body = append([]byte(`{"Version":99,`), body[1:]...)
			case "trailing":
				body = append(body, []byte(` {}`)...)
			case "checksum":
				body = bytes.Replace(body, []byte("prepared"), []byte("cleanup_pending"), 1)
			case "oversize":
				body = make([]byte, maxTransitionRecordBytes+1)
			}
			journal := filepath.Join(txn.root, transitionRecordFile)
			transitionTestWrite(t, journal, body)
			if fault == "symlink" {
				other := filepath.Join(t.TempDir(), "record.json")
				transitionTestWrite(t, other, body)
				if err := os.Remove(journal); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(other, journal); err != nil {
					t.Fatal(err)
				}
			}
			nativeBefore, _ := os.ReadFile(r.Native.Path)
			stateBefore, _ := os.ReadFile(f.store.Path)
			skillBefore, _ := os.ReadFile(filepath.Join(f.request.Client.ConfigRoot, "skills", "skill", "SKILL.md"))
			if err := freshTransitions(f).Recover(context.Background()); err == nil {
				t.Fatal("corrupt/foreign authority accepted")
			}
			nativeAfter, _ := os.ReadFile(r.Native.Path)
			stateAfter, _ := os.ReadFile(f.store.Path)
			skillAfter, _ := os.ReadFile(filepath.Join(f.request.Client.ConfigRoot, "skills", "skill", "SKILL.md"))
			if !bytes.Equal(nativeBefore, nativeAfter) || !bytes.Equal(stateBefore, stateAfter) || !bytes.Equal(skillBefore, skillAfter) {
				t.Fatal("refusal changed foreign/source effects")
			}
			if _, err := os.Lstat(journal); err != nil {
				t.Fatal("recovery deleted corrupt residue")
			}
		})
	}
}

func TestNativeTransitionMCPOnlyUsesDurableProviderRoot(t *testing.T) {
	f := providerTransitionFixture(t)
	state, err := f.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	b := state.Installations[0].Clients[f.request.NativeAttempt.BindingID]
	b.NativeObjects = []domain.NativeObjectOwnership{b.NativeObjects[0], b.NativeObjects[2]}
	state.Installations[0].Clients[b.ClientBindingID] = b
	if err := f.store.Save(state); err != nil {
		t.Fatal(err)
	}
	f.request.PreviousNativeObjects = b.NativeObjects
	f.request.Delivery.NativeObjects = []domain.NativeObjectOwnership{f.request.Delivery.NativeObjects[0], f.request.Delivery.NativeObjects[2]}
	if err := applyOpenCodeTransition(context.Background(), f.request, nativeconfig.New(), freshTransitions(f)); err != nil {
		t.Fatal(err)
	}
	state, err = f.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	b = state.Installations[0].Clients[b.ClientBindingID]
	if len(b.NativeObjects) != 2 || b.NativeActivationAttempt != "" || b.NativeObjects[0].Kind != OpenCodeV2MCPObjectKind {
		t.Fatal("MCP-only state not durably confirmed")
	}
}

func TestNativeTransitionEqualForeignSkillRetainsBackup(t *testing.T) {
	f := providerTransitionFixture(t)
	p, err := prepareOpenCodeNativeApply(f.request.Client.ConfigRoot, f.request.Delivery.ActivePath, f.request.PreviousNativeObjects, f.request.Delivery.NativeObjects, nativeconfig.New(), shared.RenameDirectoryExclusive, os.RemoveAll)
	if err != nil {
		t.Fatal(err)
	}
	req, err := openCodeTransitionRequest(p)
	if err != nil {
		t.Fatal(err)
	}
	txn := &openCodeSkillTxn{durable: true, rename: shared.RenameDirectoryExclusive, removeAll: os.RemoveAll, backups: map[string]openCodeBackup{}}
	if err := txn.createRoot(f.request.Client.ConfigRoot); err != nil {
		t.Fatal(err)
	}
	if _, err := txn.stageDesired(f.request.Delivery.ActivePath, shared.ObjectMap(p.desired)); err != nil {
		t.Fatal(err)
	}
	stop := errors.New("TEST foreign creation before install")
	req.PersistPrepared = func(v nativeconfig.PreparedTransition) error {
		if err := freshTransitions(f).PersistPrepared(context.Background(), f.request.NativeAttempt, txn.root, transitionDTO(v), f.request.PreviousNativeObjects, f.request.Delivery.NativeObjects); err != nil {
			return err
		}
		if err := txn.backupPrevious(shared.ObjectMap(p.previous)); err != nil {
			return err
		}
		transitionTestWrite(t, filepath.Join(f.request.Client.ConfigRoot, "skills", "skill", "SKILL.md"), []byte("target skill\n"))
		return stop
	}
	if _, err := nativeconfig.New().ApplyDialectTransition(req); !errors.Is(err, stop) {
		t.Fatal(err)
	}
	if err := freshTransitions(f).Recover(context.Background()); err == nil {
		t.Fatal("equal foreign skill adopted/deleted")
	}
	body, _ := os.ReadFile(filepath.Join(f.request.Client.ConfigRoot, "skills", "skill", "SKILL.md"))
	if string(body) != "target skill\n" {
		t.Fatal("equal foreign skill overwritten")
	}
	backup, _ := os.ReadFile(filepath.Join(txn.root, "old-skill", "SKILL.md"))
	if string(backup) != "source skill\n" {
		t.Fatal("source backup lost")
	}
	native, _ := os.ReadFile(transitionPaths(f.request.Client.ConfigRoot).JSONC)
	if !bytes.Equal(native, f.source) {
		t.Fatal("foreign skill conflict changed native bytes")
	}
}

// Uses the independently shared artifact capture algorithm on real files.
type transitionTestPackageVerifier struct{}

func (transitionTestPackageVerifier) Verify(ctx context.Context, root, digest string) error {
	artifact, err := (packagesnapshot.Builder{}).Build(ctx, root)
	if err != nil {
		return err
	}
	actual := artifact.Digest
	if err := artifact.Close(); err != nil {
		return err
	}
	if actual != digest {
		return errors.New("package differs from bound artifact")
	}
	return nil
}

// The durability-recorder port may fail before a record exists or after its
// atomic rename became visible. At that boundary no external rename/write is
// permitted; physical files, not returned mock outcomes, prove ordering.
type failingTransitionRecorder struct {
	NativeTransitions
	t       *testing.T
	fixture transitionFixtureData
	visible bool
}

func (r failingTransitionRecorder) PersistPrepared(ctx context.Context, id domain.NativeAttemptIdentity, root string, p domain.OpenCodeTransitionPrepared, previous, desired []domain.NativeObjectOwnership) error {
	body, _ := os.ReadFile(p.Path)
	skill, _ := os.ReadFile(filepath.Join(id.NativeRoot, "skills", "skill", "SKILL.md"))
	if !bytes.Equal(body, r.fixture.source) || string(skill) != "source skill\n" {
		r.t.Fatal("external effect preceded durable record")
	}
	if r.visible {
		if err := r.NativeTransitions.PersistPrepared(ctx, id, root, p, previous, desired); err != nil {
			return err
		}
	}
	return errors.New("TEST record durability failure")
}
func TestNativeTransitionRecordDurabilityFence(t *testing.T) {
	for _, visible := range []bool{false, true} {
		t.Run(map[bool]string{false: "before-record", true: "visible-record"}[visible], func(t *testing.T) {
			f := providerTransitionFixture(t)
			rec := failingTransitionRecorder{NativeTransitions: freshTransitions(f), t: t, fixture: f, visible: visible}
			if err := applyOpenCodeTransition(context.Background(), f.request, nativeconfig.New(), rec); err == nil {
				t.Fatal("record failure ignored")
			}
			body, _ := os.ReadFile(transitionPaths(f.request.Client.ConfigRoot).JSONC)
			skill, _ := os.ReadFile(filepath.Join(f.request.Client.ConfigRoot, "skills", "skill", "SKILL.md"))
			if !bytes.Equal(body, f.source) || string(skill) != "source skill\n" {
				t.Fatal("record failure changed source effect")
			}
			state, err := f.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			b := state.Installations[0].Clients[f.request.NativeAttempt.BindingID]
			if b.NativeObjects[0].Kind != OpenCodeMCPObjectKind {
				t.Fatal("prewrite failure published target")
			}
			if !visible {
				if b.NativeActivationAttempt == "" {
					t.Fatal("absent journal uncertainty lost attempt")
				}
				if err := freshTransitions(f).Recover(context.Background()); err == nil {
					t.Fatal("absent journal blindly reconciled")
				}
			}
		})
	}
}

func TestNativeTransitionRecoveryPreservesUnrelatedInstallation(t *testing.T) {
	f := providerTransitionFixture(t)
	p, err := prepareOpenCodeNativeApply(f.request.Client.ConfigRoot, f.request.Delivery.ActivePath, f.request.PreviousNativeObjects, f.request.Delivery.NativeObjects, nativeconfig.New(), shared.RenameDirectoryExclusive, os.RemoveAll)
	if err != nil {
		t.Fatal(err)
	}
	req, err := openCodeTransitionRequest(p)
	if err != nil {
		t.Fatal(err)
	}
	txn := &openCodeSkillTxn{durable: true, rename: shared.RenameDirectoryExclusive, removeAll: os.RemoveAll, backups: map[string]openCodeBackup{}, installed: map[string]domain.NativeObjectOwnership{}}
	if err := txn.createRoot(f.request.Client.ConfigRoot); err != nil {
		t.Fatal(err)
	}
	staged, err := txn.stageDesired(f.request.Delivery.ActivePath, shared.ObjectMap(p.desired))
	if err != nil {
		t.Fatal(err)
	}
	req.PersistPrepared = func(v nativeconfig.PreparedTransition) error {
		if err := freshTransitions(f).PersistPrepared(context.Background(), f.request.NativeAttempt, txn.root, transitionDTO(v), f.request.PreviousNativeObjects, f.request.Delivery.NativeObjects); err != nil {
			return err
		}
		if err := txn.backupPrevious(shared.ObjectMap(p.previous)); err != nil {
			return err
		}
		return txn.installStaged(staged, shared.ObjectMap(p.desired))
	}
	if _, err := nativeconfig.New().ApplyDialectTransition(req); err != nil {
		t.Fatal(err)
	}
	state, err := f.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	other := cloneTransitionState(state).Installations[0]
	other.InstallationID = "TEST-other-installation"
	other.Source.SourceBindingID = "src_other"
	binding := other.Clients[f.request.NativeAttempt.BindingID]
	binding.ClientID = "codex"
	binding.TargetLocator = filepath.Join(filepath.Dir(f.store.Path), "TEST-other-package")
	binding.ClientBindingID = domain.ComputeClientBindingID(other.InstallationID, binding.ClientID, binding.Scope, binding.TargetLocator)
	binding.PhysicalArtifact = domain.ComputePhysicalArtifactID("demo", other.InstallationID)
	binding.NativeActivationAttempt = ""
	binding.NativeObjects = nil
	binding.Receipts = nil
	other.Clients = map[string]domain.ClientBinding{binding.ClientBindingID: binding}
	state.Installations = append(state.Installations, other)
	if err := f.store.Save(state); err != nil {
		t.Fatal(err)
	}
	otherBefore, _ := json.Marshal(other)
	if err := freshTransitions(f).Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	state, err = f.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, in := range state.Installations {
		if in.InstallationID == other.InstallationID {
			after, _ := json.Marshal(in)
			if !bytes.Equal(after, otherBefore) {
				t.Fatal("recovery overwrote unrelated installation")
			}
			found = true
		}
	}
	if !found {
		t.Fatal("recovery deleted unrelated installation")
	}
}
