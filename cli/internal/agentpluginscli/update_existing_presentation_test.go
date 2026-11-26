package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

type updatePresentationActivator struct {
	ports.ClientActivator
	cause error
	calls int
}

func (activator *updatePresentationActivator) Activate(ctx context.Context, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	activator.calls++
	if request.Client.ClientID == domain.ClientCursor {
		return domain.ActivationOutcome{Activation: domain.ActivationFailed, Authentication: domain.AuthenticationNotRequired, Policy: domain.PolicyAllowed, Verification: domain.VerificationFailed}, activator.cause
	}
	return activator.ClientActivator.Activate(ctx, request)
}

func TestPublicOrdinaryUpdateMixedAttentionPreservesSuccessfulPeer(t *testing.T) {
	for _, format := range []string{"human", "json"} {
		t.Run(format, func(t *testing.T) {
			fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCodex), fixtureClient(t, domain.ClientCursor)})
			plugin := writeNamedCLIPlugin(t, "demo", "1.0.0")
			if stdout, _, err := fixture.execute(false, "add", plugin, "--target", "codex,cursor", "--no-save", "--yes"); err != nil {
				t.Fatalf("seed mixed installation: %v\n%s", err, stdout)
			}
			writeNamedCLIPluginVersion(t, plugin, "demo", "2.0.0")
			cause := fmt.Errorf("private-mixed-update: %w", os.ErrPermission)
			activator := &updatePresentationActivator{ClientActivator: fixture.app.Lifecycle.Activator, cause: cause}
			fixture.app.Lifecycle.Activator = activator
			fixture.app.InstallLogDir = ""
			stdout, stderr, err := fixture.execute(false, "update", "demo", "--target", "codex,cursor", "--scope", "user", "--format", format, "--yes")
			if !errors.Is(err, cause) || activator.calls != 2 {
				t.Fatalf("mixed update lost typed failure or peer execution: calls=%d err=%v stdout=%s", activator.calls, err, stdout)
			}
			if format == "json" {
				decodeOneDiagnosticEnvelope(t, stdout)
				var envelope struct {
					Data updateMultiResult `json:"data"`
				}
				if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
					t.Fatal(err)
				}
				if envelope.Data.Succeeded != 1 || envelope.Data.Failed != 1 || len(envelope.Data.Targets) != 2 {
					t.Fatalf("mixed authoritative results changed: %+v", envelope.Data)
				}
				return
			}
			public := stdout + stderr + err.Error()
			if !strings.Contains(stdout, "codex") || !strings.Contains(stdout, "cursor") || !strings.Contains(public, "Needs attention") || !strings.Contains(public, "does not have permission") || strings.Contains(public, "private-mixed-update") {
				t.Fatalf("mixed public update hid peer or classified attention: %s", public)
			}
		})
	}
}

func TestPublicRepeatAddGuidanceKeepsProjectContext(t *testing.T) {
	user, _ := ordinaryUpdatePresentationFixture(t)
	fixture := newProjectClaudeCLI(t)
	plugin := writeNamedCLIPlugin(t, "demo", "1.0.0")
	project := fixture.project
	if stdout, _, err := fixture.execute(false, "add", plugin, "--target", "claude", "--scope", "project", "--project-root", project, "--no-save", "--yes"); err != nil {
		t.Fatalf("seed Project installation: %v\n%s", err, stdout)
	}
	before := installedProjection(t, fixture.cliFixture)
	userBefore := installedProjection(t, user)
	writeNamedCLIPluginVersion(t, plugin, "demo", "2.0.0")
	fixture.app.InstallLogDir = diagnosticLogDirectory(t, "disabled")
	stdout, stderr, err := fixture.execute(false, "add", plugin, "--target", "claude", "--scope", "project", "--project-root", project, "--no-save", "--yes")
	if !errors.Is(err, installer.ErrUpdateRequired) {
		t.Fatalf("changed-source repeat lost typed update refusal: %v", err)
	}
	if !reflect.DeepEqual(before, installedProjection(t, fixture.cliFixture)) || !reflect.DeepEqual(userBefore, installedProjection(t, user)) {
		t.Fatal("refused repeat changed a retained lifecycle projection")
	}
	public := stdout + stderr + err.Error()
	t.Logf("repeat refusal: %s", public)
	if strings.Contains(public, "`agentplugins update`") || !strings.Contains(public, "claude") || !strings.Contains(public, "project") || !strings.Contains(public, filepath.Clean(project)) {
		t.Fatalf("repeat guidance omitted selector or effective Project context: %s", public)
	}
}

// Exercise the public Cobra route with the existing real store/planner/lock
// fixture and a controlled staging failure. This is command integration, not
// a native-client or separate-process qualification.
func TestPublicOrdinaryUpdateClassifiesFailureWithoutLogs(t *testing.T) {
	for _, format := range []string{"human", "json"} {
		t.Run(format, func(t *testing.T) {
			fixture, plugin := ordinaryUpdatePresentationFixture(t)
			writeNamedCLIPluginVersion(t, plugin, "demo", "2.0.0")
			cause := fmt.Errorf("private-update-provider-731254: %w", os.ErrPermission)
			stager := &diagnosticStageFailure{PackageStager: fixture.app.Lifecycle.Stager, cause: cause}
			fixture.app.Lifecycle.Stager = stager
			fixture.app.InstallLogDir = diagnosticLogDirectory(t, "disabled")
			stdout, stderr, err := fixture.execute(false, "update", "demo", "--target", "cursor", "--scope", "user", "--format", format, "--yes")
			if !errors.Is(err, cause) || stager.calls != 1 {
				t.Fatalf("public update lost apply cause or replayed: stages=%d err=%v stdout=%s", stager.calls, err, stdout)
			}
			if format == "json" {
				decodeOneDiagnosticEnvelope(t, stdout)
				return
			}
			public := stdout + stderr + err.Error()
			if !strings.Contains(public, "does not have permission") || !strings.Contains(public, "Needs attention") || strings.Contains(public, "private-update-provider") {
				t.Fatalf("ordinary public update bypassed safe classified attention: %s", public)
			}
		})
	}
}

func TestPublicOrdinaryUpdateJoinsApplyAndOutputFailures(t *testing.T) {
	fixture, plugin := ordinaryUpdatePresentationFixture(t)
	writeNamedCLIPluginVersion(t, plugin, "demo", "2.0.0")
	cause := fmt.Errorf("private-update-apply: %w", os.ErrPermission)
	stager := &diagnosticStageFailure{PackageStager: fixture.app.Lifecycle.Stager, cause: cause}
	fixture.app.Lifecycle.Stager = stager
	fixture.app.InstallLogDir = diagnosticLogDirectory(t, "disabled")
	writeCause := errors.New("controlled update output failure")
	broken := &diagnosticResultWriter{fail: func([]byte) bool { return stager.calls > 0 }, err: writeCause}
	var stderr bytes.Buffer
	app := fixture.app
	app.Output, app.ErrorOutput = broken, &stderr
	command := NewRoot(app)
	command.SetArgs([]string{"update", "demo", "--target", "cursor", "--scope", "user", "--yes"})
	err := command.ExecuteContext(t.Context())
	if !errors.Is(err, cause) || !errors.Is(err, writeCause) || !errors.Is(err, errInstallOutput) || stager.calls != 1 {
		t.Fatalf("public update lost operation/output cause or replayed: stages=%d writes=%d err=%v", stager.calls, broken.failures, err)
	}
}

func ordinaryUpdatePresentationFixture(t *testing.T) (cliFixture, string) {
	t.Helper()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	plugin := writeNamedCLIPlugin(t, "demo", "1.0.0")
	if stdout, _, err := fixture.execute(false, "add", plugin, "--target", "cursor", "--scope", "user", "--no-save", "--yes"); err != nil {
		t.Fatalf("seed imperative installation: %v\n%s", err, stdout)
	}
	return fixture, plugin
}
