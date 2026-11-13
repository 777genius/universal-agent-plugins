// Package hermes integrates native Hermes Skills and user-scoped MCP.
// Ownership-checked YAML changes preserve foreign config; project MCP remains
// unsupported, and project Skills require native project trust.
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

type Adapter struct{ selectedProject bool }

func New() *Adapter                  { return &Adapter{} }
func (*Adapter) ID() domain.ClientID { return domain.ClientHermes }

var (
	_ clients.Adapter                 = (*Adapter)(nil)
	_ clients.HostDetector            = (*Adapter)(nil)
	_ clients.ProfileResolver         = (*Adapter)(nil)
	_ clients.ProfileBindingValidator = (*Adapter)(nil)
	_ clients.ProjectTargetLayout     = (*Adapter)(nil)
	_ clients.NativeRegistryLayout    = (*Adapter)(nil)
	_ clients.PlanPrecondition        = (*Adapter)(nil)
	_ clients.PlanQualifier           = (*Adapter)(nil)
	_ clients.PlanRefiner             = (*Adapter)(nil)
)

func (*Adapter) DetectSurfaces(host clients.Host) clients.Detection {
	root, err := selectedProfileRoot(host)
	if err != nil {
		return clients.Detection{Err: err}
	}
	if !cleanRoot(root) {
		return clients.Detection{Err: fmt.Errorf("hermes home must be an explicit clean absolute directory")}
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

// Hermes 0.23 hermes_constants.py selects the platform default independently
// of existing directories. A legacy Windows ~/.hermes is not a fallback.
func selectedProfileRoot(host clients.Host) (string, error) {
	if root := host.Env("HERMES_HOME"); root != "" {
		return root, nil
	}
	base, name := host.HomeDir(), ".hermes"
	if host.GOOS() == "windows" {
		base = strings.TrimSpace(host.Env("LOCALAPPDATA"))
		if base == "" {
			base = filepath.Join(host.HomeDir(), "AppData", "Local")
		}
		name = "hermes"
	}
	name += host.Env("HERMES_DATA_DIR_SUFFIX")
	if err := pathpolicy.ValidatePortablePathSegment(name); err != nil {
		return "", fmt.Errorf("hermes data directory suffix is invalid: %w", err)
	}
	return filepath.Join(base, name), nil
}

func cleanRoot(path string) bool {
	return path != "" && filepath.IsAbs(path) && filepath.Clean(path) == path &&
		filepath.Dir(path) != path && strings.TrimSpace(path) == path &&
		utf8.ValidString(path) && strings.IndexFunc(path, unicode.IsControl) < 0
}

func (*Adapter) ResolveProfileRoot(root string) (string, error) {
	if !cleanRoot(root) {
		return "", fmt.Errorf("hermes profile root is not a clean absolute directory")
	}
	canonical, err := clients.CanonicalDirectory(root, os.Lstat, filepath.EvalSymlinks)
	if err != nil {
		return "", err
	}
	if !cleanRoot(canonical) {
		return "", fmt.Errorf("hermes canonical profile root is invalid")
	}
	return canonical, nil
}

// ValidateBindingProfile refuses historical or replayed receipts that point
// into a different selected native profile. No legacy Hermes migrations exist.
func (adapter *Adapter) ValidateBindingProfile(root string, binding domain.ClientBinding) error {
	if adapter.selectedProject && binding.Scope == string(domain.ScopeProject) {
		return validateProjectSessionBinding(root, binding)
	}
	if binding.ClientID != string(domain.ClientHermes) || !cleanRoot(root) {
		return fmt.Errorf("hermes recorded binding has different client or profile authority")
	}
	selected, err := New().ResolveProfileRoot(root)
	if err != nil {
		return err
	}
	if selected != root || binding.NativeProfileRoot != root {
		return fmt.Errorf("hermes binding has no exact native profile proof")
	}
	for _, item := range binding.NativeObjects {
		if item.Kind == "managed_package_directory" {
			continue
		}
		switch domain.InstallScope(binding.Scope) {
		case domain.ScopeUser:
			if item.Kind != userSkillKind && item.Kind != userMCPKind {
				return fmt.Errorf("hermes user binding contains foreign scope ownership")
			}
		case domain.ScopeProject:
			if item.Kind != projectSkillKind {
				return fmt.Errorf("hermes project binding contains a User native entry")
			}
		default:
			return fmt.Errorf("hermes binding has unsupported scope")
		}
		expected := filepath.Join(root, "skills", item.LogicalName)
		if item.Kind == userMCPKind {
			expected = filepath.Join(root, "config.yaml")
		} else if err := pathpolicy.ValidateLeafID(item.LogicalName); err != nil {
			return err
		}
		if item.Path != expected {
			return fmt.Errorf("hermes binding owns a path outside the selected native scope")
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
		return "", "", fmt.Errorf("hermes project root is unavailable")
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

// Hermes looks for workspace Skills at the nearest Git root. A user-selected
// nested directory is not that root, and silently installing there would be a
// false success. Require an explicit repository root; support ordinary .git
// directories and regular worktree pointer files without following symlinks.
func (*Adapter) CheckPlanPrecondition(in clients.PlanInput, plan *domain.DeliveryPlan) error {
	if in.TargetContext.Scope != domain.ScopeProject && plan.Scope != domain.ScopeProject {
		return nil
	}
	root := plan.TargetContext.ProjectRoot
	if root == "" {
		root = in.TargetContext.ProjectRoot
	}
	if !cleanRoot(root) {
		return fmt.Errorf("hermes project Skills require an exact Git repository root")
	}
	meta, err := os.Lstat(filepath.Join(root, ".git"))
	if err != nil {
		return fmt.Errorf("hermes project Skills require a Git root at the selected project: %w", err)
	}
	if meta.Mode()&os.ModeSymlink != 0 || (!meta.IsDir() && !meta.Mode().IsRegular()) {
		return fmt.Errorf("hermes project Git root marker must be a real directory or worktree pointer")
	}
	return nil
}

// A package with required MCP/hooks/extensions must not silently become
// Skills-only. Hermes' MCP configuration has User authority even in projects.
func (adapter *Adapter) QualifyPlan(_ clients.PlanInput, plan *domain.DeliveryPlan) error {
	if plan.Status == domain.PlanUnsupported {
		return nil
	}
	for _, component := range plan.Components {
		allowed := component.Kind == domain.ComponentSkill ||
			component.Kind == domain.ComponentMCPServer && (plan.Scope == domain.ScopeUser || adapter.selectedProject)
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
		return fmt.Errorf("hermes native Skills root is unavailable")
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
