//go:build linux || (darwin && arm64)

package vscodelocal_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/vscode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/vscodelocalhooks"
)

// Use actual Local activation and real selected-profile bytes, with no Engine,
// native app, provider or installed-product claim. The public kernel seams
// inject only failures at the writer boundary, never receipts/OS authority.
func localTransactionRequest(t *testing.T) (*vscode.LocalAdapter, domain.ActivationRequest, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	must(t, err)
	settings := filepath.Join(root, "settings.json")
	writeLocal(t, settings, []byte(`{"foreign":17}`), 0600)
	tuple := vscode.SourceQualifiedTESTTuple(runtime.GOOS)
	if runtime.GOOS == "darwin" {
		tuple = retainedB17Tuple()
	}
	a, err := vscode.NewLocal(vscode.LocalConfig{ProfileSettingsPath: settings, QualifiedTuple: tuple, TargetShell: vscodelocalhooks.Target{Shell: vscodelocalhooks.Shell(tuple.TargetShell)}, Skills: []string{"notify"}})
	must(t, err)
	active := filepath.Join(root, "TEST-plugin")
	enabled := true
	objectHash := sha256.Sum256([]byte(settings + "\x00" + active))
	facts := domain.LocalDeliveryFacts{ProfileRoot: root, SettingsPath: settings, ProfileIdentity: root, SettingsIdentity: settings, Tuple: tuple, Skills: []string{"notify"}, CanonicalDigest: testDigest([]byte("TEST canonical")), ProjectionDigest: testDigest([]byte("TEST projected")), Registration: domain.OwnedProfileEntry{ObjectID: fmt.Sprintf("vscode-local:sha256:%x", objectHash), Selector: active, DesiredValue: &enabled}}
	selected, err := domain.NewLocalDelivery(facts)
	must(t, err)
	plan := domain.DeliveryPlan{ClientID: domain.ClientVSCode, NativeRegistryRoot: root, ActivePath: active, SelectedDelivery: selected, InstallIntent: domain.InstallIntentAutomatic, Components: []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "notify", Support: domain.SupportNative}}}
	return a, domain.ActivationRequest{Client: domain.DetectedClient{ClientID: domain.ClientVSCode, ConfigRoot: root}, Plan: plan, Delivery: domain.StagedDelivery{ActivePath: active, ArtifactDigest: facts.ProjectionDigest}}, settings
}

// Regression: Darwin accidentally uses portable path IO; Linux accidentally
// uses the plain-only refusal. The custom IO must be unused for Darwin writes,
// while Linux must perform its existing write/readback/rollback transaction.
func TestLocalPlatformTransactionCustomIO(t *testing.T) {
	a, req, settings := localTransactionRequest(t)
	before := readLocal(t, settings)
	files := &localFailureIO{body: bytes.Clone(before), fail: errors.New("TEST write visible before error")}
	out, err := a.Activate(t.Context(), clients.Env{NativeConfig: nativeconfig.NewWithFileIO(files)}, req)
	if err == nil || len(out.NativeObjects) != 0 || out.LocalEntryObservation != nil {
		t.Fatalf("failed mutation granted ownership: %+v %v", out, err)
	}
	if runtime.GOOS == "darwin" {
		if files.writes != 0 || out.NativeEffect != domain.NativeEffectUncertain {
			t.Fatalf("Darwin custom IO bypassed plain guard: writes=%d effect=%s", files.writes, out.NativeEffect)
		}
	} else {
		if !errors.Is(err, files.fail) || files.writes != 2 || out.NativeEffect != domain.NativeEffectUnchanged {
			t.Fatalf("Linux write/rollback contract changed: writes=%d effect=%s err=%v", files.writes, out.NativeEffect, err)
		}
	}
	if !bytes.Equal(files.body, before) || !bytes.Equal(readLocal(t, settings), before) {
		t.Fatal("refusal/rollback lost original bytes")
	}
}

var errLocalRollback = errors.New("TEST rollback refused")

type localFailureIO struct {
	body         []byte
	fail         error
	writes       int
	rollbackFail bool
}

func (f *localFailureIO) ReadNoFollow(string) ([]byte, os.FileMode, bool, error) {
	return bytes.Clone(f.body), 0600, true, nil
}
func (f *localFailureIO) WriteAtomic(_ string, body []byte, _ os.FileMode) error {
	f.writes++
	if f.writes == 1 {
		f.body = bytes.Clone(body)
		return f.fail
	}
	if f.rollbackFail {
		return errLocalRollback
	}
	f.body = bytes.Clone(body)
	return nil
}
func (*localFailureIO) RemoveNoFollow(string) error { return errors.New("unexpected remove") }

// Regression: rollback failure or a foreign same-value CAS is acknowledged;
// cleanup/cancellation failures are swallowed and ownership is invented.
func TestLocalTransactionErrorsAndCleanup(t *testing.T) {
	for _, scenario := range []string{"lock", "cancel", "concurrent", "close", "rollback"} {
		t.Run(scenario, func(t *testing.T) {
			a, req, settings := localTransactionRequest(t)
			before := readLocal(t, settings)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			failure := errors.New("TEST " + scenario)
			released := 0
			kernel := nativeconfig.NewWithLockAcquirer(func(nativeconfig.Paths, nativeconfig.Codec) (func() error, error) {
				if scenario == "lock" {
					return nil, failure
				}
				if scenario == "cancel" {
					cancel()
				}
				return func() error {
					released++
					if scenario == "close" {
						return failure
					}
					return nil
				}, nil
			})
			var files *localFailureIO
			var concurrentFiles *localConcurrentIO
			if scenario == "rollback" {
				files = &localFailureIO{body: bytes.Clone(before), fail: failure, rollbackFail: true}
				kernel = nativeconfig.NewWithFileIO(files)
			}
			if scenario == "concurrent" {
				// Drift after capture, before Apply: the original parent/file must
				// not be rediscovered to grant ownership of the foreign output.
				concurrentFiles = &localConcurrentIO{localFailureIO: localFailureIO{body: bytes.Clone(before)}}
				kernel = nativeconfig.NewWithFileIO(concurrentFiles)
			}
			out, err := a.Activate(ctx, clients.Env{NativeConfig: kernel}, req)
			if err == nil {
				t.Fatal("failure was hidden", scenario)
			}
			if scenario == "close" && runtime.GOOS == "linux" && !nativeconfig.IsCommittedCleanup(err) {
				t.Fatal("Linux committed cleanup lost receipt classification", err)
			}
			if scenario == "close" && nativeconfig.IsCommittedCleanup(err) {
				if !errors.Is(err, failure) || released != 1 || out.NativeEffect != domain.NativeEffectCommitted || len(out.NativeObjects) != 1 || out.LocalEntryObservation == nil {
					t.Fatalf("committed cleanup lost actual receipt: %+v %v", out, err)
				}
				return
			}
			if len(out.NativeObjects) != 0 || out.LocalEntryObservation != nil {
				t.Fatal("failure granted ownership")
			}
			if scenario == "cancel" && (!errors.Is(err, context.Canceled) || released != 1 || !bytes.Equal(readLocal(t, settings), before)) {
				t.Fatalf("cancel failed to retain bytes/release: %v released=%d", err, released)
			}
			if scenario == "lock" && !errors.Is(err, failure) {
				t.Fatal("lock error lost", err)
			}
			if scenario == "rollback" && out.NativeEffect != domain.NativeEffectUncertain {
				t.Fatal("failed rollback acknowledged unchanged", out.NativeEffect)
			}
			if scenario == "rollback" && runtime.GOOS == "linux" && (files.writes != 2 || !errors.Is(err, failure) || !errors.Is(err, errLocalRollback)) {
				t.Fatal("write/rollback failures not propagated", files.writes, err)
			}
			if scenario == "concurrent" && runtime.GOOS == "linux" && (!errors.Is(err, nativeconfig.ErrConcurrentChange) || out.NativeEffect != domain.NativeEffectUncertain || concurrentFiles.writes != 0 || bytes.Equal(concurrentFiles.body, before)) {
				t.Fatal("foreign same-value CAS was granted or restored", err)
			}
			if scenario == "close" && (released != 1 || !errors.Is(err, failure)) {
				t.Fatal("close error lost", err)
			}
		})
	}
}

type localConcurrentIO struct{ localFailureIO }

func (f *localConcurrentIO) CompareAndSwap(_ string, _ []byte, _ bool, body []byte, _ os.FileMode) error {
	f.body = bytes.Clone(body) // Another writer chose our desired bytes.
	return nativeconfig.ErrConcurrentChange
}
func (*localConcurrentIO) RemoveIfUnchanged(string, []byte) error {
	return nativeconfig.ErrConcurrentChange
}
