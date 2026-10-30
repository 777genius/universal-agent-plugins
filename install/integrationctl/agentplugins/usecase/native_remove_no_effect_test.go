package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

func removeNoEffectFixture(t *testing.T) (Service, string, string, domain.DetectedClient) {
	t.Helper()
	service, store, _ := serviceFixture(t)
	client := domain.DetectedClient{ClientID: domain.ClientGemini, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".gemini")}
	input := kiroMCPInput(t, client)
	input.Confirmed = true
	added, err := service.Add(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}
	return service, store.Path, added.InstallationID, client
}

func removeNoEffectCall(t *testing.T, service Service, installationID string, client domain.DetectedClient, grouped bool) error {
	t.Helper()
	input := RemoveInput{Selector: installationID, Client: client, Scope: domain.ScopeUser, Confirmed: true}
	if grouped {
		_, err := service.RemoveGroup(t.Context(), RemoveGroupInput{Selector: installationID, Targets: []RemoveInput{input}, Confirmed: true})
		return err
	}
	_, err := service.Remove(t.Context(), input)
	return err
}

func TestNativeRemovalNoEffectRestoresExactPredecessor(t *testing.T) {
	for _, grouped := range []bool{false, true} {
		t.Run(fmt.Sprintf("grouped-%t", grouped), func(t *testing.T) {
			service, statePath, id, client := removeNoEffectFixture(t)
			settings := filepath.Join(client.ConfigRoot, "settings.json")
			original, err := os.ReadFile(settings)
			if err != nil {
				t.Fatal(err)
			}
			edited := bytes.ReplaceAll(original, []byte("https://example.test/mcp"), []byte("https://user-edited.test/keep"))
			if bytes.Equal(original, edited) {
				t.Fatal("fixture did not install expected native MCP")
			}
			if err := os.WriteFile(settings, edited, 0600); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(statePath)
			if err != nil {
				t.Fatal(err)
			}
			err = removeNoEffectCall(t, service, id, client, grouped)
			if err == nil || !strings.Contains(err.Error(), "changed outside agentplugins") {
				t.Fatalf("native ownership refusal=%v", err)
			}
			after, err := os.ReadFile(statePath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatalf("proven no-effect refusal changed durable predecessor\nbefore=%s\nafter=%s", before, after)
			}
			if got, err := os.ReadFile(settings); err != nil || !bytes.Equal(got, edited) {
				t.Fatalf("native user edit changed: %s %v", got, err)
			}
			if err := os.WriteFile(settings, original, 0600); err != nil {
				t.Fatal(err)
			}
			if err := removeNoEffectCall(t, service, id, client, grouped); err != nil {
				t.Fatalf("same removal after resolving edit: %v", err)
			}
		})
	}
}

type removalNoEffectControl struct {
	inner    ports.ClientActivator
	store    transaction.StateStore
	t        *testing.T
	effect   domain.NativeEffectState
	change   string
	cause    error
	called   bool
	seen     string
	retained domain.StateFileV2
}

func (a *removalNoEffectControl) Activate(ctx context.Context, req domain.ActivationRequest) (domain.ActivationOutcome, error) {
	return a.inner.Activate(ctx, req)
}
func (a *removalNoEffectControl) Deactivate(ctx context.Context, req domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	if !req.Confirmed {
		return a.inner.Deactivate(ctx, req)
	}
	a.t.Helper()
	state, err := a.store.Load()
	if err != nil {
		a.t.Fatal(err)
	}
	binding := onlyBinding(state.Installations[0])
	a.seen = binding.NativeActivationAttempt
	if a.seen == "" {
		a.t.Fatal("callback ran without this invocation's persisted native attempt")
	}
	switch a.change {
	case "source":
		state.Installations[0].Source.Publisher = "callback changed source authority"
	case "attempt":
		binding.NativeActivationAttempt = "callback-replaced-attempt"
	case "ownership":
		binding.NativeObjects[0].ProtectionClass = "callback-changed-ownership"
	}
	if a.change != "" {
		state.Installations[0].Clients[binding.ClientBindingID] = binding
		if err := a.store.Save(state); err != nil {
			a.t.Fatal(err)
		}
	}
	a.retained, err = a.store.Load()
	if err != nil {
		a.t.Fatal(err)
	}
	a.called = true
	return domain.DeactivationOutcome{NativeEffect: a.effect}, a.cause
}

type removalNoEffectFailStore struct {
	transaction.StateStore
	control *removalNoEffectControl
	failure error
}

func (s removalNoEffectFailStore) Save(state domain.StateFileV2) error {
	if s.control.called {
		return s.failure
	}
	return s.StateStore.Save(state)
}

func TestNativeRemovalNoEffectAcknowledgmentRetainsUncertainty(t *testing.T) {
	for _, grouped := range []bool{false, true} {
		for _, mode := range []string{"unknown-effect", "uncertain-effect", "source", "attempt", "ownership", "save-failure"} {
			t.Run(fmt.Sprintf("grouped-%t/%s", grouped, mode), func(t *testing.T) {
				service, _, id, client := removeNoEffectFixture(t)
				cause := errors.New("injected native removal refusal")
				control := &removalNoEffectControl{inner: service.Activator, store: service.StateStore, t: t, effect: domain.NativeEffectUnchanged, cause: cause}
				switch mode {
				case "unknown-effect":
					control.effect = ""
				case "uncertain-effect":
					control.effect = domain.NativeEffectUncertain
				case "source", "attempt", "ownership":
					control.change = mode
				}
				saveFailure := errors.New("injected acknowledgment store failure before visibility")
				if mode == "save-failure" {
					service.StateStore = removalNoEffectFailStore{StateStore: service.StateStore, control: control, failure: saveFailure}
				}
				service.Activator = control
				err := removeNoEffectCall(t, service, id, client, grouped)
				if !errors.Is(err, cause) || !control.called {
					t.Fatalf("lost original reached-callback refusal: %v", err)
				}
				if mode == "save-failure" && !errors.Is(err, saveFailure) {
					t.Fatalf("lost acknowledgment save failure: %v", err)
				}
				after, err := control.store.Load()
				if err != nil {
					t.Fatal(err)
				}
				beforeJSON, _ := json.Marshal(control.retained)
				afterJSON, _ := json.Marshal(after)
				if !bytes.Equal(beforeJSON, afterJSON) {
					t.Fatalf("uncertain removal overwrote callback's persisted state\nbefore=%s\nafter=%s", beforeJSON, afterJSON)
				}
				binding := onlyBinding(after.Installations[0])
				want := control.seen
				if mode == "attempt" {
					want = "callback-replaced-attempt"
				}
				if binding.NativeActivationAttempt != want {
					t.Fatalf("uncertain removal discarded owned/replaced attempt: %+v", binding)
				}
				if _, err := os.Stat(binding.TargetLocator); err != nil {
					t.Fatalf("uncertain removal lost managed package: %v", err)
				}
			})
		}
	}
}

func TestNativeRemovalAttemptSnapshotDoesNotAliasState(t *testing.T) {
	service, _, _, _ := removeNoEffectFixture(t)
	before, err := service.StateStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	attempted, err := service.StateStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := onlyBinding(attempted.Installations[0])
	binding.NativeActivationAttempt = "owned-attempt"
	attempted.Installations[0].Clients[binding.ClientBindingID] = binding
	token, err := freezeNativeRemovalAttempt(before, attempted, "owned-attempt")
	if err != nil {
		t.Fatal(err)
	}
	expectedBefore, err := json.Marshal(token.before)
	if err != nil {
		t.Fatal(err)
	}
	expectedAttempted, err := json.Marshal(token.attempted)
	if err != nil {
		t.Fatal(err)
	}
	before.Installations[0].Source.Publisher = "changed source"
	binding.NativeObjects[0].ManagedDigest = "changed native ownership"
	binding.PackageRevision.TreeDigest = "changed package authority"
	attempted.Installations[0].Clients[binding.ClientBindingID] = binding
	// The token's fields are private; compare their persisted values explicitly.
	actualBefore, _ := json.Marshal(token.before)
	actualAttempted, _ := json.Marshal(token.attempted)
	if !bytes.Equal(expectedBefore, actualBefore) || !bytes.Equal(expectedAttempted, actualAttempted) {
		t.Fatalf("frozen removal token aliased mutable predecessor: %s / %s", actualBefore, actualAttempted)
	}
}
