package environment_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// The host maps a verified lock pin to its own acquired snapshot and explicit
// client profile. The coordinator never chooses a profile or acquires bytes.
type exampleHost struct {
	engine  *installer.Engine
	request func(environment.PreparedBinding) (installer.Request, error)
}

func (h exampleHost) Prepare(ctx context.Context, b environment.PreparedBinding) (*installer.PreparedOperation, error) {
	req, err := h.request(b)
	if err != nil {
		return nil, err
	}
	req.EnvironmentAuthority = b.Authority
	req.ActivationComplete, req.AuthComplete = b.ActivationComplete, b.AuthComplete
	return h.engine.Prepare(ctx, req)
}
func (h exampleHost) Apply(ctx context.Context, p *installer.PreparedOperation) (installer.Result, error) {
	return h.engine.Apply(ctx, p, installer.Decision{Confirmed: true})
}

func ExampleCoordinator_PrepareSyncFor() {
	root, err := os.MkdirTemp("", "uap-sdk-example-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(root)
	registry, err := clients.NewRegistry()
	if err != nil {
		panic(err)
	}
	engine, err := installer.New(installer.Config{StateRoot: filepath.Join(root, "state"), Registry: registry})
	if err != nil {
		panic(err)
	}
	host := exampleHost{engine: engine, request: func(b environment.PreparedBinding) (installer.Request, error) {
		// A real host supplies acquired, assessed bytes for install/update/repair,
		// and maps ActionRemove to OpRemove using the receipt's InstallationID.
		return installer.Request{}, fmt.Errorf("no acquired snapshot for %s", b.Alias)
	}}
	coordinator := environment.Coordinator{Mutator: host}
	view, err := engine.Inspect(context.Background())
	if err != nil {
		panic(err)
	}
	observed, err := environment.Observe(view)
	if err != nil {
		panic(err)
	}
	intent := environment.Requirements{SchemaVersion: environment.ManifestSchemaVersion}
	digest, err := environment.ManifestDigest(intent)
	if err != nil {
		panic(err)
	}
	// The alias may have disappeared from intent: remaining receipts would still
	// produce removals. An empty installation yields no mutations.
	plan, err := coordinator.PrepareSyncFor(intent, environment.Lock{LockVersion: environment.LockVersion, ManifestDigest: digest}, observed, []string{"demo"}, nil)
	if err != nil {
		panic(err)
	}
	fmt.Println(len(plan.Bindings))
	// Review and publish guarded intent/lock bytes before confirmed ApplySync.
	// This example deliberately stops before Apply and touches no client profile.

	// Output: 0
}
