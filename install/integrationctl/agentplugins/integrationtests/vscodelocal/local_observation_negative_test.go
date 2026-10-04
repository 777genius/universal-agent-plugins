package vscodelocal_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/vscode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/vscodeprofile"
)

// Regression: corrupt nonnull authority becomes historical nil, or a forged
// valid digest rebases stale authority. The existing decoder matrix covers wire
// breadth; these representative attacks use actual persisted consumer state.
func TestLocalObservedStoreRefusalsRetainOriginalBytes(t *testing.T) {
	for _, attack := range []string{"digest", "basis", "linkage", "duplicate-ancestor", "unknown", "bytes", "depth", "stale-canonical", "stale-projection"} {
		t.Run(attack, func(t *testing.T) {
			f := freshLocal(t, false)
			engine := f.engine(t, true)
			installed := applyLocal(t, engine, f.request(installer.OpInstall, ""))
			facts, _ := installed.Binding.SelectedDelivery.LocalFacts()
			disableLocal(t, f, facts.Registration.Selector)
			applyLocal(t, engine, f.request(installer.OpInstall, installed.InstallationID))
			path := filepath.Join(f.state, "state-v2.json")
			originalView, viewErr := engine.Inspect(t.Context())
			must(t, viewErr)
			raw := readLocal(t, path)
			bad := malformedLocalState(t, f, attack, raw)
			writeLocal(t, path, bad, 0600)
			before := snapshotLocalFiles(t, filepath.Join(f.state, "state-v2.json"), f.settings)
			_, loadErr := (statev2.Store{Path: path}).Load()
			stale := strings.HasPrefix(attack, "stale-")
			if !stale && loadErr == nil {
				t.Fatal("real Store accepted corrupted observation")
			}
			if stale && loadErr != nil {
				t.Fatal("stale valid receipt should reach consumer validation", loadErr)
			}
			h, err := engine.Prepare(t.Context(), f.request(installer.OpRepair, installed.InstallationID))
			if h != nil {
				defer func() { must(t, h.Close()) }()
			}
			if err == nil {
				_, err = engine.Apply(t.Context(), h, installer.Decision{Confirmed: true})
			}
			if err == nil {
				t.Fatal("corrupt/stale authority was adopted or silently rebased")
			}
			if attack == "digest" || attack == "duplicate-ancestor" {
				_, recoverErr := engine.Recover(t.Context(), originalView)
				if recoverErr == nil {
					t.Fatal("public Recover accepted corrupt nonnull authority")
				}
			}
			_, inspectErr := engine.Inspect(t.Context())
			if !stale && inspectErr == nil {
				t.Fatal("Inspect ignored corrupt nonnull observation")
			}
			assertLocalFiles(t, before)
		})
	}
}

func malformedLocalState(t *testing.T, f *localFixture, attack string, raw []byte) []byte {
	t.Helper()
	var wire map[string]any
	must(t, json.Unmarshal(raw, &wire))
	installation := wire["installations"].([]any)[0].(map[string]any)
	bindings := installation["clients"].(map[string]any)
	var binding map[string]any
	for _, v := range bindings {
		binding = v.(map[string]any)
	}
	observation := binding["local_entry_observation"].(map[string]any)
	switch attack {
	case "digest":
		observation["receipt_digest"] = "sha256:" + strings.Repeat("0", 64)
	case "basis":
		observation["revision_basis"].(map[string]any)["local"].(map[string]any)["canonical_digest"] = "sha256:" + strings.Repeat("0", 64)
	case "linkage":
		binding["native_objects"] = []any{}
	case "unknown":
		observation["unreviewed_authority"] = true
	case "bytes":
		observation["unreviewed_authority"] = strings.Repeat("x", 1<<20)
	case "depth":
		var deep any = true
		for range 65 {
			deep = []any{deep}
		}
		observation["unreviewed_authority"] = deep
	case "duplicate-ancestor":
		clientsRaw, err := json.Marshal(bindings)
		must(t, err)
		body, err := json.Marshal(wire)
		must(t, err)
		target := append([]byte(`"clients":`), clientsRaw...)
		replacement := append(bytes.Clone(target), []byte(`,"clientſ":{}`)...)
		changed := bytes.Replace(body, target, replacement, 1)
		if bytes.Equal(changed, body) {
			t.Fatal("duplicate ancestor fixture did not change")
		}
		return changed
	case "stale-canonical", "stale-projection":
		b := onlyLocalBinding(t, loadLocalState(t, f.state))
		old := b.LocalEntryObservation.Facts()
		facts, _ := old.RevisionBasis.LocalFacts()
		if attack == "stale-canonical" {
			facts.CanonicalDigest = "sha256:" + strings.Repeat("a", 64)
		} else {
			facts.ProjectionDigest = "sha256:" + strings.Repeat("b", 64)
		}
		basis, err := domain.NewLocalDelivery(facts)
		must(t, err)
		parsed, err := vscodeprofile.VerifyRecordedEntry(readLocal(t, f.settings), testLocalIdentity(facts), false)
		must(t, err)
		forged, err := domain.NewLocalEntryObservation(domain.LocalEntryObservationFacts{RevisionBasis: basis, Enabled: false, ReceiptDigest: parsed.Receipt.Digest})
		must(t, err)
		body, err := json.Marshal(forged)
		must(t, err)
		var obj any
		must(t, json.Unmarshal(body, &obj))
		binding["local_entry_observation"] = obj
	default:
		t.Fatal("unknown attack")
	}
	body, err := json.Marshal(wire)
	must(t, err)
	return body
}

func testLocalIdentity(f domain.LocalDeliveryFacts) vscodeprofile.Identity {
	return vscodeprofile.Identity{SettingsPath: f.SettingsPath, ProfileID: f.ProfileIdentity, PluginRoot: f.Registration.Selector, PackageID: f.Registration.ObjectID, PackageDigest: f.CanonicalDigest, ProjectionDigest: f.ProjectionDigest}
}

// Regression: false-to-true drift, nonboolean bytes or a missing revision
// transition is accepted as a new receipt. Boundary: fresh actual Engine after
// real false persistence; refusal must retain complete state/profile snapshots.
func TestLocalObservedNativeAndRevisionRefusals(t *testing.T) {
	for _, attack := range []string{"true", "nonbool", "revision-absence", "refresh-absence"} {
		t.Run(attack, func(t *testing.T) {
			f := freshLocal(t, false)
			installed := applyLocal(t, f.engine(t, true), f.request(installer.OpInstall, ""))
			facts, _ := installed.Binding.SelectedDelivery.LocalFacts()
			disableLocal(t, f, facts.Registration.Selector)
			applyLocal(t, f.engine(t, true), f.request(installer.OpInstall, installed.InstallationID))
			assertLocalObservation(t, f, false)
			op := installer.OpRepair
			switch attack {
			case "true":
				writeLocal(t, f.settings, []byte(`{"chat.pluginLocations":{`+quoteLocal(facts.Registration.Selector)+`:true}}`), 0600)
			case "nonbool":
				writeLocal(t, f.settings, []byte(`{"chat.pluginLocations":{`+quoteLocal(facts.Registration.Selector)+`:42}}`), 0600)
			case "revision-absence":
				writeLocal(t, f.settings, []byte(`{"chat.pluginLocations":{},"TEST-foreign":false}`), 0600)
				writeLocal(t, filepath.Join(f.pkg, "plugin.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"test-local","version":"1.0.1"}`), 0600)
				op = installer.OpUpdate
			case "refresh-absence":
				writeLocal(t, f.settings, []byte(`{"chat.pluginLocations":{},"TEST-foreign":false}`), 0600)
				f.config.NativeStop = false
				f.config.HookSpecs = nil
				a, err := vscode.NewLocal(f.config)
				must(t, err)
				*f.adapter = *a
				op = installer.OpRefreshProjection
			}
			engine := f.engine(t, true)
			before := snapshotLocalFiles(t, filepath.Join(f.state, "state-v2.json"), f.settings)
			h, err := engine.Prepare(t.Context(), f.request(op, installed.InstallationID))
			if h != nil {
				defer func() { must(t, h.Close()) }()
			}
			if err == nil {
				_, err = engine.Apply(t.Context(), h, installer.Decision{Confirmed: true})
			}
			if err == nil {
				t.Fatal("native/revision drift minted authority")
			}
			assertLocalFiles(t, before)
		})
	}
}

// Regression: the adapter rebases before verifying old native bytes during
// update/refresh. Existing maintenance tests assert only settings bytes, so this
// checks actual old and new sealed receipts across successful revision changes.
func TestLocalObservedRevisionTransitionsKeepFalse(t *testing.T) {
	f := freshLocal(t, false)
	engine := f.engine(t, true)
	installed := applyLocal(t, engine, f.request(installer.OpInstall, ""))
	facts, _ := installed.Binding.SelectedDelivery.LocalFacts()
	disableLocal(t, f, facts.Registration.Selector)
	applyLocal(t, engine, f.request(installer.OpInstall, installed.InstallationID))
	prior := assertLocalObservation(t, f, false)
	before := snapshotLocalFiles(t, f.settings)
	writeLocal(t, filepath.Join(f.pkg, "plugin.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"test-local","version":"1.0.1"}`), 0600)
	applyLocal(t, engine, f.request(installer.OpUpdate, installed.InstallationID))
	updated := assertLocalObservation(t, f, false)
	if prior.LocalEntryObservation.Equal(updated.LocalEntryObservation) {
		t.Fatal("update retained old sealed receipt")
	}
	assertLocalFiles(t, before)
	f.config.NativeStop = false
	f.config.HookSpecs = nil
	a, err := vscode.NewLocal(f.config)
	must(t, err)
	*f.adapter = *a
	applyLocal(t, engine, f.request(installer.OpRefreshProjection, installed.InstallationID))
	refreshed := assertLocalObservation(t, f, false)
	if updated.LocalEntryObservation.Equal(refreshed.LocalEntryObservation) {
		t.Fatal("refresh did not carry reviewed selection/new projection")
	}
	assertLocalFiles(t, before)
	assertStableLocal(t, f, engine, installed.InstallationID, false)
}

// Regression: equal settings or constructing an observation grants ownership.
// Boundary: actual NewLocal VerifyOnly and Deactivate using real profile/store
// facts, but deliberately withholding independently recorded native objects.
func TestLocalObservationWithoutOwnedSelectorRefuses(t *testing.T) {
	f := freshLocal(t, false)
	applyLocal(t, f.engine(t, true), f.request(installer.OpInstall, ""))
	b := assertLocalObservation(t, f, true)
	env := clients.Env{NativeConfig: nativeconfig.New()}
	facts, _ := b.SelectedDelivery.LocalFacts()
	before := snapshotLocalFiles(t, f.state, filepath.Dir(f.settings))
	plan := domain.DeliveryPlan{SelectedDelivery: b.SelectedDelivery, LocalEntryObservation: b.LocalEntryObservation, ClientID: domain.ClientVSCode, NativeRegistryRoot: facts.ProfileRoot, ActivePath: b.TargetLocator, InstallIntent: domain.InstallIntentAutomatic}
	req := domain.ActivationRequest{Client: domain.DetectedClient{ClientID: domain.ClientVSCode, ConfigRoot: facts.ProfileRoot}, Plan: plan, Delivery: domain.StagedDelivery{ActivePath: b.TargetLocator, ArtifactDigest: facts.ProjectionDigest}, VerifyOnly: true}
	req.PreviousNativeObjects = append([]domain.NativeObjectOwnership(nil), b.NativeObjects...)
	control, controlErr := f.adapter.Activate(t.Context(), env, req)
	must(t, controlErr)
	if control.LocalEntryObservation == nil || !control.LocalEntryObservation.Equal(b.LocalEntryObservation) {
		t.Fatal("owned verification control did not reach actual receipt boundary")
	}
	req.PreviousNativeObjects = nil
	out, err := f.adapter.Activate(t.Context(), env, req)
	if err == nil || out.LocalEntryObservation != nil || out.NativeEffect != domain.NativeEffectUnchanged {
		t.Fatal("constructor/observation granted ownership")
	}
	_, err = f.adapter.Deactivate(t.Context(), env, domain.DeactivationRequest{SelectedDelivery: b.SelectedDelivery, LocalEntryObservation: b.LocalEntryObservation, Client: req.Client, ManagedArtifactPath: b.TargetLocator, Confirmed: true})
	if err == nil {
		t.Fatal("unowned removal adopted observation")
	}
	assertLocalFiles(t, before)
	// A valid independently owned receipt does not permit ordinary activation to
	// restore absence, and readonly verification must classify without a receipt.
	req.PreviousNativeObjects = append([]domain.NativeObjectOwnership(nil), b.NativeObjects...)
	absent := []byte(`{"chat.pluginLocations":{},"TEST-foreign":false}`)
	writeLocal(t, f.settings, absent, 0600)
	before = snapshotLocalFiles(t, f.state, filepath.Dir(f.settings))
	out, err = f.adapter.Activate(t.Context(), env, req)
	var classified *domain.LocalEntryAbsence
	if !errors.As(err, &classified) || !classified.Matches(b.LocalEntryObservation) || out.LocalEntryObservation != nil || out.NativeEffect != domain.NativeEffectUnchanged {
		t.Fatal("readonly missing entry became a verified receipt")
	}
	assertLocalFiles(t, before)
	req.VerifyOnly = false
	out, err = f.adapter.Activate(t.Context(), env, req)
	if err == nil || out.LocalEntryObservation != nil || out.NativeEffect != domain.NativeEffectUnchanged {
		t.Fatal("ordinary activation restored absence without explicit Repair")
	}
	assertLocalFiles(t, before)
}

// Regression: a real effect followed by error/cancellation/late binding drift
// acknowledges a new observation or permits an ordinary resend. S1 callback
// stand-ins did not establish the actual NewLocal effect on a real filesystem.
func TestLocalActualCallbackFailureRetainsPredecessorAndIntent(t *testing.T) {
	for _, mode := range []string{"error", "cancel", "binding-drift", "unknown"} {
		t.Run(mode, func(t *testing.T) {
			f := freshLocal(t, false)
			installed := applyLocal(t, f.engine(t, true), f.request(installer.OpInstall, ""))
			facts, _ := installed.Binding.SelectedDelivery.LocalFacts()
			disableLocal(t, f, facts.Registration.Selector)
			applyLocal(t, f.engine(t, true), f.request(installer.OpInstall, installed.InstallationID))
			prior := assertLocalObservation(t, f, false)
			writeLocal(t, f.settings, []byte(`{"chat.pluginLocations":{},"TEST-late-foreign":false}`), 0600)
			// Real managed-byte corruption selects the package repair path. This
			// qualifies callback fencing after an actual public native write; it does
			// not stand in for the separately required intact-package Repair proof.
			writeLocal(t, filepath.Join(facts.Registration.Selector, "com.example.opaque", "preserved.txt"), []byte("TEST actual managed drift before repair"), 0600)

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			wrapper := &afterActualLocal{LocalAdapter: f.adapter, t: t, f: f, mode: mode, cancel: cancel}
			var err error
			f.registry, err = clients.NewRegistry(wrapper)
			must(t, err)
			engine := f.engine(t, true)
			h, err := engine.Prepare(t.Context(), f.request(installer.OpRepair, installed.InstallationID))
			must(t, err)
			defer func() { must(t, h.Close()) }()
			result, err := engine.Apply(ctx, h, installer.Decision{Confirmed: true})
			if err == nil && result.Outcome == installer.OutcomeCompleted {
				t.Fatal("failed/unknown callback acknowledged")
			}
			if wrapper.effects != 1 {
				t.Fatal("did not delegate exactly one actual effect")
			}
			b := onlyLocalBinding(t, loadLocalState(t, f.state))
			if !b.LocalEntryObservation.Equal(prior.LocalEntryObservation) || b.PendingNativeIntent == nil || !b.PendingNativeIntent.LocalEntryObservation.Equal(prior.LocalEntryObservation) {
				t.Fatal("failure lost frozen predecessor/matching intent")
			}
			must(t, b.PendingNativeIntent.Validate(b))
			if !strings.Contains(string(readLocal(t, f.settings)), quoteLocal(facts.Registration.Selector)+":false") {
				t.Fatal("actual repair effect was not witnessed")
			}
			before := snapshotLocalFiles(t, filepath.Join(f.state, "state-v2.json"), f.settings)
			retry, retryErr := engine.Prepare(t.Context(), f.request(installer.OpRepair, installed.InstallationID))
			if retry != nil {
				must(t, retry.Close())
			}
			if retryErr == nil || wrapper.effects != 1 {
				t.Fatal("ordinary retry resent uncertain native effect")
			}
			assertLocalFiles(t, before)
		})
	}
}

type afterActualLocal struct {
	*vscode.LocalAdapter
	t       *testing.T
	f       *localFixture
	mode    string
	cancel  context.CancelFunc
	effects int
}

func (a *afterActualLocal) Activate(ctx context.Context, env clients.Env, req domain.ActivationRequest) (domain.ActivationOutcome, error) {
	out, err := a.LocalAdapter.Activate(ctx, env, req)
	if err != nil || req.VerifyOnly {
		return out, err
	}
	a.effects++
	if out.NativeEffect != domain.NativeEffectCommitted || out.LocalEntryObservation == nil {
		a.t.Fatal("callback test had no actual native effect")
	}
	b := onlyLocalBinding(a.t, loadLocalState(a.t, a.f.state))
	if b.PendingNativeIntent == nil || !b.PendingNativeIntent.LocalEntryObservation.Equal(req.Plan.LocalEntryObservation) {
		a.t.Fatal("effect without frozen durable intent")
	}
	switch a.mode {
	case "error":
		return out, fmt.Errorf("TEST error after actual NewLocal effect")
	case "cancel":
		a.cancel()
	case "unknown":
		out.NativeEffect = domain.NativeEffectUncertain
	case "binding-drift":
		path := filepath.Join(a.f.state, "state-v2.json")
		store := statev2.Store{Path: path}
		state, loadErr := store.Load()
		must(a.t, loadErr)
		for key, current := range state.Installations[0].Clients {
			current.UpdatedAt = "TEST-late-callback-drift"
			state.Installations[0].Clients[key] = current
		}
		must(a.t, store.Save(state))
	default:
		a.t.Fatal("unknown callback mode")
	}
	return out, nil
}

// Regression: a save error causes native resend or loses the frozen predecessor.
// Engine has no Store injection seam. This deliberately qualifies the existing
// public usecase/StateStore port with actual NewLocal and a real filesystem Store.
func TestLocalActualUsecaseAmbiguousAcknowledgement(t *testing.T) {
	for _, visibility := range []string{"old", "desired", "third"} {
		t.Run(visibility, func(t *testing.T) {
			f := freshLocal(t, false)
			installed := applyLocal(t, f.engine(t, true), f.request(installer.OpInstall, ""))
			facts, _ := installed.Binding.SelectedDelivery.LocalFacts()
			disableLocal(t, f, facts.Registration.Selector)
			applyLocal(t, f.engine(t, true), f.request(installer.OpInstall, installed.InstallationID))
			prior := assertLocalObservation(t, f, false)
			writeLocal(t, f.settings, []byte(`{"chat.pluginLocations":{},"TEST-ambiguous-foreign":false}`), 0600)
			counter := &countActualLocal{LocalAdapter: f.adapter}
			registry, err := clients.NewRegistry(counter)
			must(t, err)
			store := &ambiguousLocalStore{Store: statev2.Store{Path: filepath.Join(f.state, "state-v2.json")}, visibility: visibility, t: t}
			paths := pathpolicy.Policy{}
			native := nativeconfig.New()
			p := planner.Planner{Registry: registry, Paths: paths, ManagedRoot: filepath.Join(f.state, "managed")}
			stager := providers.Stager{Registry: registry, Paths: paths}
			service := usecase.Service{StateStore: store, Paths: paths, Planner: p, Targets: p, Stager: stager, Activator: providers.Activator{Registry: registry, Runner: noLocalCommands{t: t}, NativeConfig: &native}, PluginData: providers.PluginDataManager{Base: filepath.Join(f.state, "plugin-data")}, Lock: processlock.Lock{Path: filepath.Join(f.state, "mutation.lock")}, Kernel: transaction.Kernel{StateStore: store, Directory: dirswap.Manager{JournalDir: filepath.Join(f.state, "operations")}}, NativeObserver: providers.NativeIdentityObserver{Stager: stager, Runner: noLocalCommands{t: t}, Registry: registry, NativeConfig: &native}}
			envelope := f.envelope(t)
			source := loadLocalState(t, f.state).Installations[0].Source
			if source.TreeDigest != envelope.TreeDigest {
				t.Fatal("actual usecase input differs from recorded source snapshot")
			}
			envelope.Source = domain.SourceIdentity{RequestedSource: source.RequestedSource, CanonicalSource: source.CanonicalSource, Repository: source.Repository, PackageSubpath: source.PackageSubpath, ResolvedRevision: source.ResolvedRevision, SourceBindingHint: source.SourceBindingID}
			input := usecase.AddInput{Envelope: envelope, Client: domain.DetectedClient{ClientID: domain.ClientVSCode, Status: domain.DetectionDetected, ConfigRoot: filepath.Dir(f.settings)}, Scope: domain.ScopeUser, InstallationID: installed.InstallationID, OperationID: "TEST-ambiguous-" + visibility, Confirmed: true, InstallIntent: domain.InstallIntentAutomatic}
			_, err = service.Repair(t.Context(), input)
			if !store.fired || counter.effects != 1 {
				t.Fatalf("did not reach actual native effect and durable acknowledgement: fired=%v effects=%d err=%v", store.fired, counter.effects, err)
			}
			b := onlyLocalBinding(t, loadLocalState(t, f.state))
			profile := readLocal(t, f.settings)
			if !strings.Contains(string(profile), quoteLocal(facts.Registration.Selector)+":false") || !strings.Contains(string(profile), `"TEST-ambiguous-foreign":false`) {
				t.Fatal("actual effect lost false/foreign bytes")
			}
			if visibility == "desired" {
				if err != nil || b.PendingNativeIntent != nil || !b.LocalEntryObservation.Equal(prior.LocalEntryObservation) {
					t.Fatal("exact desired visibility was not acknowledged", err)
				}
			} else {
				if err == nil || b.PendingNativeIntent == nil || !b.LocalEntryObservation.Equal(prior.LocalEntryObservation) || !b.PendingNativeIntent.LocalEntryObservation.Equal(prior.LocalEntryObservation) {
					t.Fatal("ambiguous visibility lost predecessor/intent", err)
				}
				must(t, b.PendingNativeIntent.Validate(b))
				_, retryErr := service.Repair(t.Context(), input)
				if retryErr == nil || counter.effects != 1 {
					t.Fatal("ambiguous acknowledgement resent native effect")
				}
			}
			t.Logf("nearest public usecase StateStore visibility=%s effects=%d ack_error=%v; not Engine injection proof", visibility, counter.effects, err)
		})
	}
}

type countActualLocal struct {
	*vscode.LocalAdapter
	effects int
}

func (a *countActualLocal) Activate(ctx context.Context, env clients.Env, req domain.ActivationRequest) (domain.ActivationOutcome, error) {
	out, err := a.LocalAdapter.Activate(ctx, env, req)
	if !req.VerifyOnly {
		a.effects++
	}
	return out, err
}

type ambiguousLocalStore struct {
	statev2.Store
	visibility string
	fired      bool
	t          *testing.T
}

func (s *ambiguousLocalStore) Save(next domain.StateFileV2) error {
	old, err := s.Load()
	if err != nil {
		return err
	}
	if !s.fired && len(old.Installations) == 1 && len(next.Installations) == 1 {
		before := onlyLocalBinding(s.t, old)
		after := onlyLocalBinding(s.t, next)
		if before.PendingNativeIntent != nil && after.PendingNativeIntent == nil {
			s.fired = true
			switch s.visibility {
			case "old":
				return errors.New("TEST ack error with exact old visibility")
			case "desired":
				if err := s.Store.Save(next); err != nil {
					return err
				}
				return errors.New("TEST ack error after desired visibility")
			case "third":
				for key, b := range old.Installations[0].Clients {
					b.UpdatedAt = "TEST-third-visible-state"
					old.Installations[0].Clients[key] = b
				}
				if err := s.Store.Save(old); err != nil {
					return err
				}
				return errors.New("TEST ack error after third visibility")
			}
		}
	}
	return s.Store.Save(next)
}
