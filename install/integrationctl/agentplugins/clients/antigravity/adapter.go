// Package antigravity prepares verified UAP packages in Google's documented
// native Antigravity plugin discovery directories, without invoking an
// unreviewed external plugin installer or modifying another scope.
package antigravity

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type Adapter struct{}

func New() *Adapter { return &Adapter{} }
func (*Adapter) ID() domain.ClientID { return domain.ClientAntigravity }

var (
	_ clients.Adapter = (*Adapter)(nil)
	_ clients.HostDetector = (*Adapter)(nil)
	_ clients.ProfileResolver = (*Adapter)(nil)
	_ clients.TargetLayout = (*Adapter)(nil)
	_ clients.ProjectTargetLayout = (*Adapter)(nil)
	_ clients.NativeRegistryLayout = (*Adapter)(nil)
	_ clients.PlanQualifier = (*Adapter)(nil)
	_ clients.PlanRefiner = (*Adapter)(nil)
)

func cleanAbsolute(path string) bool {
	return path != "" && filepath.IsAbs(path) && filepath.Clean(path) == path &&
		filepath.Dir(path) != path && strings.TrimSpace(path) == path &&
		!strings.ContainsAny(path, "\x00\r\n")
}

func (*Adapter) DetectSurfaces(host clients.Host) clients.Detection {
	if !cleanAbsolute(host.HomeDir()) {
		return clients.Detection{Err: fmt.Errorf("Antigravity requires an unambiguous absolute home directory")}
	}
	root, err := host.CanonicalDirectory(filepath.Join(host.HomeDir(), ".gemini", "config"))
	if err != nil {
		return clients.Detection{Err: err}
	}
	executable := host.LookPath("agy")
	return clients.Detection{
		ConfigRoot: root, ExecutablePath: executable,
		Surfaces: []domain.ClientSurface{
			host.ResolvedBinarySurface("antigravity_cli", executable),
			host.DirectorySurface("antigravity_config", root),
		},
		SelectionSurfaceIDs: []string{"antigravity_cli"},
	}
}

func (*Adapter) ResolveProfileRoot(root string) (string, error) {
	if !cleanAbsolute(root) {
		return "", fmt.Errorf("Antigravity native profile must be a clean absolute directory")
	}
	canonical, err := clients.CanonicalDirectory(root, os.Lstat, filepath.EvalSymlinks)
	if err != nil || !cleanAbsolute(canonical) {
		return "", fmt.Errorf("Antigravity native profile cannot be resolved: %w", err)
	}
	return canonical, nil
}

func (*Adapter) TargetRoot(client domain.DetectedClient, _ domain.PackageMode, _ string) (string, string, error) {
	if !cleanAbsolute(client.ConfigRoot) {
		return "", "", fmt.Errorf("Antigravity user profile is not selected")
	}
	root := filepath.Join(client.ConfigRoot, "plugins")
	return client.ConfigRoot, root, nil
}

func (*Adapter) NativeRegistry(in clients.PlanInput) (string, string) {
	if in.TargetContext.Scope == domain.ScopeProject {
		return filepath.Join(in.TargetContext.ProjectRoot, ".agents"), in.Client.ExecutablePath
	}
	return in.Client.ConfigRoot, in.Client.ExecutablePath
}

func (*Adapter) ProjectTargetRoot(_ domain.DetectedClient, _ domain.PackageMode, managedRoot string, target domain.TargetContext) (string, string, error) {
	if target.Scope != domain.ScopeProject || !cleanAbsolute(target.ProjectRoot) {
		return "", "", fmt.Errorf("Antigravity project scope must specify a real project root")
	}
	if err := pathpolicy.RequireContainedChild(target.ProjectRoot, managedRoot); err != nil {
		return "", "", err
	}
	anchor := filepath.Join(target.ProjectRoot, ".agents")
	return anchor, filepath.Join(anchor, "plugins"), nil
}

// Only Skills and MCP have an explicit portable equivalence. UAP's generic
// extensions and commands do not imply an Antigravity native hook or subagent.
func (*Adapter) QualifyPlan(_ clients.PlanInput, plan *domain.DeliveryPlan) error {
	if plan.Status == domain.PlanUnsupported {
		return nil
	}
	for _, component := range plan.Components {
		switch component.Kind {
		case domain.ComponentSkill, domain.ComponentMCPServer:
			if component.Support == domain.SupportUnsupported {
				plan.Status = domain.PlanUnsupported
			}
		default:
			plan.Status = domain.PlanUnsupported
		}
	}
	if plan.Status == domain.PlanUnsupported {
		plan.Warnings = shared.AppendUnique(plan.Warnings,
			"Antigravity native projection currently admits Skills and stdio/Streamable HTTP MCP; unqualified required components are refused")
	}
	return nil
}

func (*Adapter) RefinePlan(ctx context.Context, _ clients.PlanInput, plan *domain.DeliveryPlan) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if plan.Status == domain.PlanUnsupported {
		return nil
	}
	if !cleanAbsolute(plan.NativeRegistryRoot) {
		return fmt.Errorf("Antigravity native registry root is unavailable")
	}
	shared.PromoteNativeReady(plan, plan.NativeRegistryRoot, shared.OnlyNativeComponents(plan.Components))
	plan.Activation = domain.ActivationPrepared
	plan.UserActions = shared.AppendUnique(plan.UserActions,
		"Open or restart Antigravity and verify the UAP-owned plugin in Customizations/Installed or with agy plugin list; native enablement and model access are not asserted")
	return nil
}
