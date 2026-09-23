// Package kimi projects portable skills and MCP into Kimi Code's user plugin registry.
package kimi

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type Adapter struct{}

func New() *Adapter                  { return &Adapter{} }
func (*Adapter) ID() domain.ClientID { return domain.ClientKimi }

var (
	_ clients.HostDetector              = (*Adapter)(nil)
	_ clients.TargetLayout              = (*Adapter)(nil)
	_ clients.PlanRefiner               = (*Adapter)(nil)
	_ clients.SelectionReader           = (*Adapter)(nil)
	_ clients.Projector                 = (*Adapter)(nil)
	_ clients.Lifecycle                 = (*Adapter)(nil)
	_ clients.ActivationPreflighter     = (*Adapter)(nil)
	_ clients.AutomaticActivator        = (*Adapter)(nil)
	_ clients.ReadOnlyVerifier          = (*Adapter)(nil)
	_ clients.RegistryInspector         = (*Adapter)(nil)
	_ clients.PreparedRegistryInspector = (*Adapter)(nil)
)

func (*Adapter) DetectSurfaces(host clients.Host) clients.Detection {
	root := strings.TrimSpace(host.Env("KIMI_CODE_HOME"))
	if root == "" {
		root = filepath.Join(host.HomeDir(), ".kimi-code")
	}
	executable := host.LookPath("kimi")
	return clients.Detection{ConfigRoot: root, ExecutablePath: executable, Surfaces: []domain.ClientSurface{
		host.ResolvedBinarySurface("kimi_cli", executable), host.DirectorySurface("kimi_config", root),
	}}
}
func (*Adapter) TargetRoot(client domain.DetectedClient, _ domain.PackageMode, _ string) (string, string, error) {
	if !filepath.IsAbs(client.ConfigRoot) {
		return "", "", fmt.Errorf("Kimi configuration root must be absolute")
	}
	return client.ConfigRoot, filepath.Join(client.ConfigRoot, "plugins", "managed"), nil
}
func (*Adapter) RefinePlan(_ context.Context, in clients.PlanInput, plan *domain.DeliveryPlan) error {
	shared.PromoteNativeReady(plan, in.Client.ConfigRoot, shared.OnlyNativeComponents(plan.Components))
	plan.UserActions = shared.AppendUnique(plan.UserActions, reloadAction)
	return nil
}
func (*Adapter) ManagedMCPSelection() clients.SelectionLayout {
	return clients.SelectionLayout{File: filepath.Join(".kimi-plugin", "plugin.json"), Nested: true}
}

const reloadAction = "Run /reload or /new in Kimi Code to load the installed plugin; runtime MCP connections and authentication have not been verified."
