package vscodelocal_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/vscode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// Red: repeat saves identical state or stages an attempt despite a confirmed
// owned selector. File identity/mtime expose Saves that byte equality misses.
func TestLocalP0EmptyRepeatsDoNotSave(t *testing.T) {
	for _, owned := range []bool{false, true} {
		name := "unowned"
		if owned {
			name = "owned-native-false"
		}
		t.Run(name, func(t *testing.T) {
			f := freshLocal(t, false)
			if !owned {
				deselectP0Local(t, f)
			}
			installed := applyLocal(t, f.engine(t, true), f.request(installer.OpInstall, ""))
			if owned {
				disableP0Local(t, f)
				deselectP0Local(t, f)
				applyLocal(t, f.engine(t, true), f.request(installer.OpRefreshProjection, installed.InstallationID))
			}
			for _, op := range []installer.Operation{installer.OpInstall, installer.OpRepair, installer.OpRefreshProjection} {
				before := captureP0Files(t, f)
				// Reload all production state and construct a fresh public Engine.
				result := applyLocal(t, f.engine(t, true), f.request(op, installed.InstallationID))
				assertP0FilesUnchanged(t, f, before)
				binding := onlyLocalBinding(t, loadLocalState(t, f.state))
				if result.Mutated || binding.SelectedDelivery.OwnsProfileEntry(binding.NativeObjects) != owned || binding.PendingNativeIntent != nil || binding.NativeActivationAttempt != "" {
					t.Fatalf("repeat changed ownership/attempt: %+v", result)
				}
				assertForeign(t, readLocal(t, f.settings))
				t.Logf("%s %s no Save; state=%s profile=%s updated_at=%s", name, op, testDigest(before[0].body), testDigest(before[1].body), binding.UpdatedAt)
			}
		})
	}
}

// An uncertain acknowledgement after a real adapter call must retain the
// previously owned selector and matching intent against the newly committed
// package. This observes actual durable state in the public adapter callback.
func TestLocalP0OwnedEmptyUncertainCommit(t *testing.T) {
	for _, op := range []installer.Operation{installer.OpRefreshProjection, installer.OpUpdate, installer.OpRepair} {
		t.Run(string(op), func(t *testing.T) {
			f := freshLocal(t, false)
			installed := applyLocal(t, f.engine(t, true), f.request(installer.OpInstall, ""))
			disableP0Local(t, f)
			deselectP0Local(t, f)
			if op != installer.OpRefreshProjection {
				applyLocal(t, f.engine(t, true), f.request(installer.OpRefreshProjection, installed.InstallationID))
			}
			if op == installer.OpUpdate {
				path := filepath.Join(f.pkg, "plugin.json")
				writeLocal(t, path, bytes.Replace(readLocal(t, path), []byte(`"version":"1.0.0"`), []byte(`"version":"1.0.1"`), 1), 0600)
			}
			if op == installer.OpRepair {
				writeLocal(t, filepath.Join(installed.Binding.TargetPath, "TEST-damaged"), []byte("TEST changed managed bytes"), 0600)
			}
			wrapped := &p0UncertainLocal{LocalAdapter: f.adapter, t: t, state: f.state, fail: true}
			var err error
			f.registry, err = clients.NewRegistry(wrapped)
			must(t, err)
			profile := readLocal(t, f.settings)
			engine := f.engine(t, true)
			handle, err := engine.Prepare(t.Context(), f.request(op, installed.InstallationID))
			must(t, err)
			defer func() { _ = handle.Close() }()
			_, err = engine.Apply(t.Context(), handle, installer.Decision{Confirmed: true})
			if !errors.Is(err, errP0Acknowledgement) || wrapped.calls != 1 {
				t.Fatalf("uncertain callback did not execute once: %v calls=%d", err, wrapped.calls)
			}
			binding := onlyLocalBinding(t, loadLocalState(t, f.state))
			if !binding.SelectedDelivery.OwnsProfileEntry(binding.NativeObjects) || binding.PendingNativeIntent == nil {
				t.Fatal("uncertainty dropped independently recorded ownership/intent")
			}
			must(t, binding.PendingNativeIntent.Validate(binding))
			if !bytes.Equal(profile, readLocal(t, f.settings)) {
				t.Fatal("uncertain empty commit changed disabled/foreign native bytes")
			}
			assertForeign(t, profile)
			before := captureP0Files(t, f)
			view, err := engine.Inspect(t.Context())
			must(t, err)
			if !view.Recovery.Required {
				t.Fatal("uncertain matching attempt was not observable")
			}
			assertP0RetryRefuses(t, f, installed.InstallationID)
			assertP0FilesUnchanged(t, f, before)
			if wrapped.calls != 1 {
				t.Fatal("unresolved attempt retried native activation")
			}
			t.Logf("%s retained matching pending=%s package=%s; state=%s profile=%s", op, binding.NativeActivationAttempt, binding.PackageRevision.TreeDigest, testDigest(before[0].body), testDigest(profile))
		})
	}
}

var errP0Acknowledgement = errors.New("TEST uncertain native acknowledgement")

type p0UncertainLocal struct {
	*vscode.LocalAdapter
	t     *testing.T
	state string
	fail  bool
	calls int
}

func (a *p0UncertainLocal) Activate(ctx context.Context, env clients.Env, req domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if a.fail && !req.VerifyOnly {
		a.calls++
		binding := onlyLocalBinding(a.t, loadLocalState(a.t, a.state))
		if binding.PendingNativeIntent == nil || !binding.SelectedDelivery.OwnsProfileEntry(binding.NativeObjects) || !req.Plan.SelectedDelivery.OwnsProfileEntry(req.PreviousNativeObjects) {
			a.t.Fatal("native activation preceded retained durable selector/intent")
		}
		must(a.t, binding.PendingNativeIntent.Validate(binding))
		facts, _ := binding.SelectedDelivery.LocalFacts()
		if facts.ProjectionDigest != req.Delivery.ArtifactDigest || binding.PackageRevision.TreeDigest != facts.CanonicalDigest {
			a.t.Fatal("pending package/projection does not match committed delivery")
		}
	}
	outcome, err := a.LocalAdapter.Activate(ctx, env, req)
	if a.fail && !req.VerifyOnly && err == nil {
		outcome.NativeEffect = domain.NativeEffectUncertain
		return outcome, errP0Acknowledgement
	}
	return outcome, err
}

func deselectP0Local(t *testing.T, f *localFixture) {
	t.Helper()
	f.config.NativeStop, f.config.HookSpecs = false, nil
	adapter, err := vscode.NewLocal(f.config)
	must(t, err)
	*f.adapter = *adapter
}

func disableP0Local(t *testing.T, f *localFixture) {
	t.Helper()
	binding := onlyLocalBinding(t, loadLocalState(t, f.state))
	facts, _ := binding.SelectedDelivery.LocalFacts()
	before := readLocal(t, f.settings)
	after := strings.Replace(string(before), facts.Registration.Selector+`":true`, facts.Registration.Selector+`":false`, 1)
	if after == string(before) {
		t.Fatal("fixture did not disable owned selector")
	}
	writeLocal(t, f.settings, []byte(after), 0600)
}

type p0File struct {
	body []byte
	info os.FileInfo
}

func captureP0Files(t *testing.T, f *localFixture) []p0File {
	t.Helper()
	files := make([]p0File, 0, 2)
	for _, path := range []string{filepath.Join(f.state, "state-v2.json"), f.settings} {
		info, err := os.Stat(path)
		must(t, err)
		files = append(files, p0File{body: readLocal(t, path), info: info})
	}
	return files
}

func assertP0FilesUnchanged(t *testing.T, f *localFixture, before []p0File) {
	t.Helper()
	for i, file := range captureP0Files(t, f) {
		if !bytes.Equal(before[i].body, file.body) || !os.SameFile(before[i].info, file.info) || !before[i].info.ModTime().Equal(file.info.ModTime()) {
			t.Fatalf("file %d was saved or changed: before=%s after=%s", i, testDigest(before[i].body), testDigest(file.body))
		}
	}
}

func assertP0RetryRefuses(t *testing.T, f *localFixture, id string) {
	t.Helper()
	engine := f.engine(t, true)
	handle, err := engine.Prepare(t.Context(), f.request(installer.OpInstall, id))
	if handle != nil {
		defer func() { _ = handle.Close() }()
	}
	if err == nil {
		_, err = engine.Apply(t.Context(), handle, installer.Decision{Confirmed: true})
	}
	if err == nil {
		t.Fatal("unresolved attempt was resurrected as a new activation")
	}
}
