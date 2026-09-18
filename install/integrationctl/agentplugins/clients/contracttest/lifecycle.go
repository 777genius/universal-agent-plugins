package contracttest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// RunLifecycle asserts the activation half of the contract when the adapter
// implements it. A client without Lifecycle is activated by the operator, and
// that absence is not a violation.
func RunLifecycle(t *testing.T, adapter clients.Adapter) {
	t.Helper()
	RunAdapter(t, adapter)
	lifecycle, ok := adapter.(clients.Lifecycle)
	if !ok {
		return
	}
	for _, violation := range lifecycleViolations(t, lifecycle, adapter.ID()) {
		t.Error(violation)
	}
}

func lifecycleViolations(t *testing.T, lifecycle clients.Lifecycle, id domain.ClientID) []string {
	t.Helper()
	violations := make([]string, 0, 4)
	violations = append(violations, lifecycleVerifyOnlyViolations(t, lifecycle, id)...)
	violations = append(violations, lifecycleMismatchViolations(t, lifecycle, id)...)
	violations = append(violations, lifecycleUnconfirmedDeactivateViolations(t, lifecycle, id)...)
	violations = append(violations, lifecycleNilRunnerViolations(t, lifecycle, id)...)
	return violations
}

func lifecycleVerifyOnlyViolations(t *testing.T, lifecycle clients.Lifecycle, id domain.ClientID) []string {
	t.Helper()
	runner := &lifecycleRunner{}
	request := lifecycleActivationRequest(t, id)
	request.VerifyOnly = true
	_, _ = lifecycle.Activate(context.Background(), clients.Env{Runner: runner}, request)
	for _, command := range runner.commands {
		if mutatingClientArgv(command.Argv) {
			return []string{fmt.Sprintf("VerifyOnly ran mutating command %q", strings.Join(command.Argv, " "))}
		}
	}
	return nil
}

func lifecycleMismatchViolations(t *testing.T, lifecycle clients.Lifecycle, id domain.ClientID) []string {
	t.Helper()
	request := lifecycleActivationRequest(t, id)
	request.Plan.ClientID = "other"
	if _, err := lifecycle.Activate(context.Background(), clients.Env{}, request); err == nil {
		return []string{"Activate accepted a request whose plan client id did not match the delivery"}
	}
	return nil
}

func lifecycleUnconfirmedDeactivateViolations(t *testing.T, lifecycle clients.Lifecycle, id domain.ClientID) []string {
	t.Helper()
	runner := &lifecycleRunner{}
	request := lifecycleDeactivationRequest(id)
	request.Confirmed = false
	_, _ = lifecycle.Deactivate(context.Background(), clients.Env{Runner: runner}, request)
	if len(runner.commands) > 0 {
		return []string{fmt.Sprintf("unconfirmed Deactivate ran %d commands", len(runner.commands))}
	}
	return nil
}

func lifecycleNilRunnerViolations(t *testing.T, lifecycle clients.Lifecycle, id domain.ClientID) []string {
	t.Helper()
	var panicked any
	func() {
		defer func() { panicked = recover() }()
		request := lifecycleActivationRequest(t, id)
		_, _ = lifecycle.Activate(context.Background(), clients.Env{}, request)
		_, _ = lifecycle.Deactivate(context.Background(), clients.Env{}, lifecycleDeactivationRequest(id))
	}()
	if panicked != nil {
		return []string{fmt.Sprintf("Runner==nil panicked: %v", panicked)}
	}
	return nil
}

func mutatingClientArgv(argv []string) bool {
	if len(argv) < 3 || argv[1] != "plugin" {
		return false
	}
	switch argv[2] {
	case "list":
		return false
	case "marketplace":
		return len(argv) >= 4 && (argv[3] == "add" || argv[3] == "update" || argv[3] == "remove")
	case "install", "add", "update", "uninstall", "remove":
		return true
	default:
		return false
	}
}

type lifecycleRunner struct {
	commands []legacyports.Command
}

func (runner *lifecycleRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	runner.commands = append(runner.commands, command)
	return legacyports.CommandResult{}, nil
}

func lifecycleActivationRequest(t *testing.T, id domain.ClientID) domain.ActivationRequest {
	t.Helper()
	root := t.TempDir()
	base := filepath.Join(root, "owned")
	active := filepath.Join(base, "demo")
	if err := os.MkdirAll(active, 0o755); err != nil {
		t.Fatal(err)
	}
	return domain.ActivationRequest{
		Client:       domain.DetectedClient{ClientID: id, ConfigRoot: filepath.Join(root, "config")},
		DeclaredName: "demo",
		Plan: domain.DeliveryPlan{
			ClientID: id, ActivePath: active, PhysicalArtifactID: "demo-0123456789ab",
			DeclaredVersion: "1.0.0",
		},
		Delivery:          domain.StagedDelivery{ClientID: id, OwnedBase: base, ActivePath: active},
		BackendExecutable: "/test/bin/" + string(id),
	}
}

func lifecycleDeactivationRequest(id domain.ClientID) domain.DeactivationRequest {
	return domain.DeactivationRequest{
		Client:             domain.DetectedClient{ClientID: id},
		DeclaredName:       "demo",
		PhysicalArtifactID: "demo-0123456789ab",
		BackendExecutable:  "/test/bin/" + string(id),
	}
}
