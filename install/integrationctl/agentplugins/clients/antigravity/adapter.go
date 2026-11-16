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

// Folder trust and plugin enablement are deliberately not automated: trust is
// Antigravity's only consent before it loads .agents content such as foreign
// hooks or MCP, and the IDE keeps it in a state database UAP must never write.
// The text claims only reviewed facts: plugins are enabled by default, each
// folder asks for trust, and plugin MCP servers do not start without sign-in.
func preparedPluginAction(plan domain.DeliveryPlan) string {
	action := "Open a folder in agy or the Antigravity IDE and confirm its folder trust prompt"
	if plan.Scope == domain.ScopeProject {
		action = "Open this project in agy or the Antigravity IDE and confirm its folder trust prompt"
	}
	action += "; Antigravity enables installed plugins by default"
	proof := "installed files do not prove plugin discovery"
	if shared.HasSupportedMCP(plan.Components) {
		action += ". Sign in to Google, because plugin MCP servers do not start without sign-in"
		proof = "installed files do not prove plugin discovery or MCP server startup"
	}
	return action + "; " + proof
}

type Adapter struct{}

func New() *Adapter                  { return &Adapter{} }
func (*Adapter) ID() domain.ClientID { return domain.ClientAntigravity }

var (
	_ clients.Adapter                  = (*Adapter)(nil)
	_ clients.HostDetector             = (*Adapter)(nil)
	_ clients.ProfileResolver          = (*Adapter)(nil)
	_ clients.TargetLayout             = (*Adapter)(nil)
	_ clients.ProjectTargetLayout      = (*Adapter)(nil)
	_ clients.NativeRegistryLayout     = (*Adapter)(nil)
	_ clients.UserNativeRegistryLayout = (*Adapter)(nil)
	_ clients.SelectionReader          = (*Adapter)(nil)
	_ clients.PlanPrecondition         = (*Adapter)(nil)
	_ clients.PlanQualifier            = (*Adapter)(nil)
	_ clients.PlanRefiner              = (*Adapter)(nil)
)

// Both native scopes record selected MCP servers in mcp_config.json;
// the portable mcp.json is deliberately not present after projection.
func (*Adapter) ManagedMCPSelection() clients.SelectionLayout {
	return clients.SelectionLayout{File: "mcp_config.json", Nested: true}
}

func cleanAbsolute(path string) bool {
	return path != "" && filepath.IsAbs(path) && filepath.Clean(path) == path &&
		filepath.Dir(path) != path && strings.TrimSpace(path) == path &&
		!strings.ContainsAny(path, "\x00\r\n")
}

func (*Adapter) DetectSurfaces(host clients.Host) clients.Detection {
	if !cleanAbsolute(host.HomeDir()) {
		return clients.Detection{Err: fmt.Errorf("antigravity requires an unambiguous absolute home directory")}
	}
	root, err := host.CanonicalDirectory(filepath.Join(host.HomeDir(), ".gemini", "config"))
	if err != nil {
		return clients.Detection{Err: err}
	}
	nativeRoot := ""
	if root == filepath.Join(host.HomeDir(), ".gemini", "config") {
		nativeRoot = root // Deriving HOME is qualified only for the original unaliased spelling.
	}
	executable := host.LookPath("agy")
	return clients.Detection{
		ConfigRoot: root, NativeConfigRoot: nativeRoot, ExecutablePath: executable,
		Surfaces: []domain.ClientSurface{
			host.ResolvedBinarySurface("antigravity_cli", executable),
			host.DirectorySurface("antigravity_config", root),
		},
		SelectionSurfaceIDs: []string{"antigravity_cli"},
	}
}

func (*Adapter) ResolveProfileRoot(root string) (string, error) {
	if !cleanAbsolute(root) {
		return "", fmt.Errorf("antigravity native profile must be a clean absolute directory")
	}
	canonical, err := clients.CanonicalDirectory(root, os.Lstat, filepath.EvalSymlinks)
	if err != nil {
		return "", fmt.Errorf("resolve Antigravity native profile: %w", err)
	}
	if !cleanAbsolute(canonical) {
		return "", fmt.Errorf("antigravity resolved profile is not a clean absolute path")
	}
	return canonical, nil
}

func (*Adapter) TargetRoot(client domain.DetectedClient, _ domain.PackageMode, _ string) (string, string, error) {
	if !cleanAbsolute(client.ConfigRoot) {
		return "", "", fmt.Errorf("antigravity user profile is not selected")
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

// Keep the existing User layout while allowing callers to retain the detector's
// separate original-profile provenance for read-only native inventory.
func (*Adapter) UserNativeRegistry(client domain.DetectedClient) string {
	return client.ConfigRoot
}

func (*Adapter) ProjectTargetRoot(_ domain.DetectedClient, _ domain.PackageMode, managedRoot string, target domain.TargetContext) (string, string, error) {
	if target.Scope != domain.ScopeProject || !cleanAbsolute(target.ProjectRoot) {
		return "", "", fmt.Errorf("antigravity project scope must specify a real project root")
	}
	if err := pathpolicy.RequireContainedChild(target.ProjectRoot, managedRoot); err != nil {
		return "", "", err
	}
	anchor := filepath.Join(target.ProjectRoot, ".agents")
	return anchor, filepath.Join(anchor, "plugins"), nil
}

// Antigravity requires kebab-case plugin identities for strict CLI validation.
// Reject incompatible package names before staging, creating plugin roots or
// invoking any client-native operation; do not silently rename an identity.
func (*Adapter) CheckPlanPrecondition(in clients.PlanInput, plan *domain.DeliveryPlan) error {
	name := plan.DeclaredName
	if name == "" {
		name = in.Envelope.Manifest.Name
	}
	if !nativePluginID.MatchString(name) {
		plan.Status = domain.PlanUnsupported
		plan.Warnings = shared.AppendUnique(plan.Warnings,
			"Antigravity native plugins require a lowercase kebab-case package name; rename in the source manifest, not during installation")
	}
	return nil
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
		return fmt.Errorf("antigravity native registry root is unavailable")
	}
	shared.PromoteNativeReady(plan, plan.NativeRegistryRoot, shared.OnlyNativeComponents(plan.Components))
	plan.Activation = domain.ActivationPrepared
	plan.UserActions = shared.AppendUnique(plan.UserActions, preparedPluginAction(*plan))
	return nil
}
