// Package hermes integrates the native Hermes Agent Skills directories.
// MCP remains unqualified until the credential-bearing YAML document can be
// changed with exact-member ownership and concurrency-safe rollback.
package hermes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type Adapter struct{}

func New() *Adapter { return &Adapter{} }
func (*Adapter) ID() domain.ClientID { return domain.ClientHermes }

var (
	_ clients.Adapter = (*Adapter)(nil)
	_ clients.HostDetector = (*Adapter)(nil)
	_ clients.ProfileResolver = (*Adapter)(nil)
	_ clients.ProfileBindingValidator = (*Adapter)(nil)
	_ clients.ProjectTargetLayout = (*Adapter)(nil)
	_ clients.NativeRegistryLayout = (*Adapter)(nil)
	_ clients.PlanQualifier = (*Adapter)(nil)
	_ clients.PlanRefiner = (*Adapter)(nil)
)

func (*Adapter) DetectSurfaces(host clients.Host) clients.Detection {
	root := host.Env("HERMES_HOME")
	if root == "" {
		root = filepath.Join(host.HomeDir(), ".hermes")
	}
	if !cleanRoot(root) {
		return clients.Detection{Err: fmt.Errorf("Hermes home must be an explicit clean absolute directory")}
	}
	canonical, err := host.CanonicalDirectory(root)
	if err != nil {
		return clients.Detection{Err: err}
	}
	executable := host.LookPath("hermes")
	return clients.Detection{
		ConfigRoot: canonical, ExecutablePath: executable,
		Surfaces: []domain.ClientSurface{
			host.ResolvedBinarySurface("hermes_cli", executable),
			host.DirectorySurface("hermes_config", canonical),
		},
		SelectionSurfaceIDs: []string{"hermes_cli"},
	}
}

func cleanRoot(path string) bool {
	return path != "" && filepath.IsAbs(path) && filepath.Clean(path) == path &&
		filepath.Dir(path) != path && strings.TrimSpace(path) == path &&
		utf8.ValidString(path) && strings.IndexFunc(path, unicode.IsControl) < 0
}

func (*Adapter) ResolveProfileRoot(root string) (string, error) {
	if !cleanRoot(root) {
		return "", fmt.Errorf("Hermes profile root is not a clean absolute directory")
	}
	canonical, err := clients.CanonicalDirectory(root, os.Lstat, filepath.EvalSymlinks)
	if err != nil {
		return "", err
	}
	if !cleanRoot(canonical) {
		return "", fmt.Errorf("Hermes canonical profile root is invalid")
	}
	return canonical, nil
}

// ValidateBindingProfile refuses historical or replayed receipts that point
// into a different selected native profile. No legacy Hermes migrations exist.
func (*Adapter) ValidateBindingProfile(root string, binding domain.ClientBinding) error {
	if binding.ClientID != string(domain.ClientHermes) || !cleanRoot(root) {
		return fmt.Errorf("Hermes recorded binding has different client or profile authority")
	}
	selected, err := New().ResolveProfileRoot(root)
	if err != nil { return err }
	if selected != root || binding.NativeProfileRoot != root {
		return fmt.Errorf("Hermes binding has no exact native profile proof")
	}
	for _, item := range binding.NativeObjects {
		switch item.Kind {
		case "managed_package_directory", userSkillKind, projectSkillKind:
		default:
			return fmt.Errorf("Hermes binding contains a foreign native ownership type")
		}
	}
	return nil
}

func (*Adapter) NativeRegistry(in clients.PlanInput) (string, string) {
	if in.TargetContext.Scope == domain.ScopeProject {
		return filepath.Join(in.TargetContext.ProjectRoot, ".hermes"), in.Client.ExecutablePath
	}
	return in.Client.ConfigRoot, in.Client.ExecutablePath
}

func (*Adapter) ProjectTargetRoot(client domain.DetectedClient, mode domain.PackageMode, managedRoot string, target domain.TargetContext) (string, string, error) {
	if target.Scope != domain.ScopeProject || !cleanRoot(target.ProjectRoot) {
		return "", "", fmt.Errorf("Hermes project root is unavailable")
	}
	anchor, root, err := shared.ManagedTargetRoot(client, mode, managedRoot)
	if err != nil {
		return "", "", err
	}
	if err := pathpolicy.RequireContainedChild(target.ProjectRoot, root); err != nil {
		return "", "", err
	}
	return anchor, root, nil
}

// A package with required MCP/hooks/extensions must not silently become
// Skills-only. Hermes' MCP configuration has User authority even in projects.
func (*Adapter) QualifyPlan(_ clients.PlanInput, plan *domain.DeliveryPlan) error {
	if plan.Status == domain.PlanUnsupported {
		return nil
	}
	for _, component := range plan.Components {
		allowed := component.Kind == domain.ComponentSkill ||
			component.Kind == domain.ComponentMCPServer && plan.Scope == domain.ScopeUser
		if !allowed || component.Support == domain.SupportUnsupported {
			plan.Status = domain.PlanUnsupported
			plan.Warnings = shared.AppendUnique(plan.Warnings, "Hermes supports User Skills/MCP and Project Skills; project MCP and unqualified package components cannot be redirected or discarded")
			return nil
		}
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
	root := plan.NativeRegistryRoot
	if !cleanRoot(root) {
		return fmt.Errorf("Hermes native Skills root is unavailable")
	}
	shared.PromoteNativeReady(plan, root, shared.OnlyNativeComponents(plan.Components))
	plan.Activation = domain.ActivationPrepared
	if plan.Scope == domain.ScopeProject {
		plan.UserActions = shared.AppendUnique(plan.UserActions, "Review Hermes project trust before Skills can be discovered; UAP does not automatically trust repositories")
	} else {
		plan.UserActions = shared.AppendUnique(plan.UserActions, "Reload or restart Hermes and verify managed Skills are visible; copied files alone do not prove native activation")
	}
	return nil
}
