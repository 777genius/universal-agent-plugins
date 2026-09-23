package usecase

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providerstest"
)

func TestOpenCodeNamespaceGroupEarlyConflictChangesNoTarget(t *testing.T) {
	service, store, cursor := serviceFixture(t)
	service.NativeObserver = providerstest.NewObserver(providers.NativeIdentityObserver{Stager: service.Stager})
	openCode := domain.DetectedClient{ClientID: domain.ClientOpenCode, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), "opencode")}
	if err := os.MkdirAll(openCode.ConfigRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(openCode.ConfigRoot, "opencode.json")
	foreign := []byte(`{"mcp":{"docs_extra":{"type":"remote","url":"https://foreign.test"}}}`)
	if err := os.WriteFile(config, foreign, 0o600); err != nil {
		t.Fatal(err)
	}
	input := openCodePluginInput(t, openCode, "1.0.0", "sha256:namespace-early", "sha256:namespace-early-manifest", "sh")
	cursorInput := openCodePluginInput(t, cursor, "1.0.0", "sha256:namespace-early", "sha256:namespace-early-manifest", "sh")
	result, err := service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{cursorInput, input}, OperationGroupID: "namespace-early", Confirmed: true})
	if err == nil || !strings.Contains(err.Error(), `"docs"`) || !strings.Contains(err.Error(), `"docs_extra"`) {
		t.Fatalf("missing exact conflict: %v", err)
	}
	if result.Mutated || len(result.Receipts) != 0 {
		t.Fatalf("early refusal changed group: %+v", result)
	}
	state, err := store.Load()
	if err != nil || len(state.Installations) != 0 {
		t.Fatalf("early refusal changed state: %+v, %v", state, err)
	}
	if body, err := os.ReadFile(config); err != nil || string(body) != string(foreign) {
		t.Fatalf("foreign config changed: %s, %v", body, err)
	}
}

type injectOpenCodeConfigActivator struct {
	inner    ports.ClientActivator
	path     string
	injected bool
}

func (a *injectOpenCodeConfigActivator) Activate(ctx context.Context, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if request.Client.ClientID == domain.ClientOpenCode && !a.injected {
		a.injected = true
		if err := os.WriteFile(a.path, []byte(`{"mcp":{"docs_extra":{"type":"remote","url":"https://foreign.test"}}}`), 0o600); err != nil {
			return domain.ActivationOutcome{}, err
		}
	}
	return a.inner.Activate(ctx, request)
}

func (a *injectOpenCodeConfigActivator) Deactivate(ctx context.Context, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	return a.inner.Deactivate(ctx, request)
}

func TestOpenCodeNamespaceGroupLateConflictIsPartialAndRetryable(t *testing.T) {
	service, store, cursor := serviceFixture(t)
	service.NativeObserver = providerstest.NewObserver(providers.NativeIdentityObserver{Stager: service.Stager})
	openCode := domain.DetectedClient{ClientID: domain.ClientOpenCode, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), "opencode")}
	if err := os.MkdirAll(openCode.ConfigRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(openCode.ConfigRoot, "opencode.json")
	if err := os.WriteFile(config, []byte(`{"mcp":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	input := openCodePluginInput(t, openCode, "1.0.0", "sha256:namespace-late", "sha256:namespace-late-manifest", "sh")
	cursorInput := openCodePluginInput(t, cursor, "1.0.0", "sha256:namespace-late", "sha256:namespace-late-manifest", "sh")
	originalActivator := service.Activator
	service.Activator = &injectOpenCodeConfigActivator{inner: originalActivator, path: config}
	group := GroupInput{Targets: []AddInput{cursorInput, input}, OperationGroupID: "namespace-late", Confirmed: true}
	result, err := service.AddGroup(context.Background(), group)
	if err == nil || !strings.Contains(err.Error(), "docs_extra") {
		t.Fatalf("late conflict not reported: %v", err)
	}
	if result.Phase != GroupPhaseExternalPartialFailure || result.Targets[0].GroupPhase != GroupTargetExternalCompleted || result.Targets[1].GroupPhase != GroupTargetExternalFailed {
		t.Fatalf("late conflict was not partial: %+v", result)
	}
	state, loadErr := store.Load()
	if loadErr != nil || len(state.Installations) != 1 || len(state.Installations[0].Clients) != 2 {
		t.Fatalf("managed commit lost: %+v, %v", state, loadErr)
	}
	if body, readErr := os.ReadFile(config); readErr != nil || strings.Contains(string(body), `"docs"`) || !strings.Contains(string(body), "docs_extra") {
		t.Fatalf("OpenCode config changed despite refusal: %s, %v", body, readErr)
	}
	if err := os.WriteFile(config, []byte(`{"mcp":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	service.Activator = originalActivator
	group.OperationGroupID = "namespace-retry"
	retried, err := service.AddGroup(context.Background(), group)
	if err != nil {
		t.Fatalf("retry after removing foreign conflict: %v", err)
	}
	if retried.Phase != GroupPhaseCompleted {
		t.Fatalf("retry did not complete: %+v", retried)
	}
	if body, readErr := os.ReadFile(config); readErr != nil || !strings.Contains(string(body), `"docs"`) {
		t.Fatalf("retry did not activate OpenCode: %s, %v", body, readErr)
	}
}
