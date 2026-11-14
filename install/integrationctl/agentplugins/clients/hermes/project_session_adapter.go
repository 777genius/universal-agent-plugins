package hermes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

var _ clients.SessionPlanner = (*Adapter)(nil)

func (adapter *Adapter) hasProjectSession(plan domain.DeliveryPlan) bool {
	return plan.Scope == domain.ScopeProject && shared.HasSupportedMCP(plan.Components)
}

func (adapter *Adapter) componentLayout(plan domain.DeliveryPlan, staged string) (shared.ProjectComponents, error) {
	layout, err := components(plan, staged)
	if err == nil && plan.Scope == domain.ScopeProject {
		layout.MCPKind, layout.Codec = projectSessionMCPKind, nativeconfig.CodecHermes
		layout.ConfigPaths.JSON = filepath.Join(projectSessionRoot(plan.TargetContext.ProjectRoot), "config.yaml")
	}
	return layout, err
}

func (adapter *Adapter) componentObjects(plan domain.DeliveryPlan, objects []domain.NativeObjectOwnership) ([]domain.NativeObjectOwnership, error) {
	if plan.Scope != domain.ScopeProject {
		return ownedObjects(plan.Scope, objects)
	}
	var result []domain.NativeObjectOwnership
	rootFound, mcpFound := false, false
	for _, object := range objects {
		switch object.Kind {
		case "managed_package_directory":
		case projectSessionRootKind:
			if rootFound || object != sessionRootObject(plan.TargetContext.ProjectRoot) {
				return nil, fmt.Errorf("hermes Project root ownership differs")
			}
			rootFound = true
		case projectSessionMCPKind:
			if !validSessionMCPObject(object, plan.TargetContext.ProjectRoot) {
				return nil, fmt.Errorf("hermes Project MCP ownership differs")
			}
			mcpFound = true
			result = append(result, object)
		case projectSkillKind:
			result = append(result, object)
		default:
			return nil, fmt.Errorf("hermes refused foreign Project ownership")
		}
	}
	if mcpFound && !rootFound {
		return nil, fmt.Errorf("hermes Project MCP has no selected-root ownership")
	}
	return result, nil
}

func validateProjectSessionBinding(root string, binding domain.ClientBinding) error {
	selected, err := New().ResolveProfileRoot(root)
	if err != nil || selected != root || binding.ClientID != string(domain.ClientHermes) || binding.NativeProfileRoot != root || filepath.Base(root) != ".hermes" {
		return fmt.Errorf("hermes Project binding has different profile authority")
	}
	plan := domain.DeliveryPlan{Scope: domain.ScopeProject, NativeRegistryRoot: root,
		TargetContext: domain.TargetContext{ProjectRoot: filepath.Dir(root)}}
	adapter := New()
	objects, err := adapter.componentObjects(plan, binding.NativeObjects)
	if err != nil {
		return err
	}
	layout, err := adapter.componentLayout(plan, "")
	if err != nil {
		return err
	}
	return layout.ValidateObjects(objects)
}

func (adapter *Adapter) preflightProjectRoot(env clients.Env, req domain.ActivationRequest) error {
	if !adapter.hasProjectSession(req.Plan) {
		return nil
	}
	if env.ProjectRoots == nil {
		return fmt.Errorf("hermes selected-root transaction is unavailable")
	}
	root := projectSessionRoot(req.Plan.TargetContext.ProjectRoot)
	if _, err := os.Lstat(root); os.IsNotExist(err) {
		if req.VerifyOnly && req.Delivery.ActivePath != "" {
			return fmt.Errorf("hermes selected root is missing")
		}
		return nil
	} else if err != nil {
		return err
	}
	return env.ProjectRoots.VerifyProjectRoot(context.Background(), root, sessionRootObject(req.Plan.TargetContext.ProjectRoot))
}

func (adapter *Adapter) publishProjectRoot(ctx context.Context, env clients.Env, req domain.ActivationRequest) error {
	if !adapter.hasProjectSession(req.Plan) {
		return nil
	}
	if env.ProjectRoots == nil {
		return fmt.Errorf("hermes selected-root transaction is unavailable")
	}
	project := req.Plan.TargetContext.ProjectRoot
	if err := env.ProjectRoots.PublishProjectRoot(ctx, req, ports.NativeProjectRoot{
		Path: projectSessionRoot(project), Marker: sessionRootObject(project), MarkerBody: sessionRootBody(project),
	}); err != nil {
		return err
	}
	return verifyProjectSessionRoot(ctx, project, sessionRootObject(project))
}

func (adapter *Adapter) verifyProjectRoot(ctx context.Context, env clients.Env, plan domain.DeliveryPlan, objects []domain.NativeObjectOwnership) error {
	if plan.Scope != domain.ScopeProject {
		return nil
	}
	for _, object := range objects {
		if object.Kind != projectSessionRootKind {
			continue
		}
		if object != sessionRootObject(plan.TargetContext.ProjectRoot) || env.ProjectRoots == nil {
			return fmt.Errorf("hermes selected-root transaction authority differs")
		}
		if err := env.ProjectRoots.VerifyProjectRoot(ctx, projectSessionRoot(plan.TargetContext.ProjectRoot), object); err != nil {
			return err
		}
		return verifyProjectSessionRoot(ctx, plan.TargetContext.ProjectRoot, object)
	}
	if adapter.hasProjectSession(plan) {
		return fmt.Errorf("hermes Project MCP has no selected-root receipt")
	}
	return nil
}

func (*Adapter) PlanSession(ctx context.Context, env clients.Env, req clients.SessionRequest) (clients.SessionPlan, error) {
	if req.Client.Status != domain.DetectionDetected || !hermesCLISurface(req.Client.Surfaces) {
		return clients.SessionPlan{}, fmt.Errorf("hermes CLI surface is required for an explicit Project session")
	}
	if env.ProjectRoots == nil {
		return clients.SessionPlan{}, fmt.Errorf("hermes selected-root verifier is unavailable")
	}
	owner := sessionRootObject(req.ProjectRoot)
	if err := env.ProjectRoots.VerifyProjectRoot(ctx, projectSessionRoot(req.ProjectRoot), owner); err != nil {
		return clients.SessionPlan{}, err
	}
	plan, err := prepareProjectSession(ctx, req, owner)
	if err != nil {
		return clients.SessionPlan{}, err
	}
	if err := verifySessionAuthority(ctx, env, plan); err != nil {
		return clients.SessionPlan{}, err
	}
	return plan, nil
}

func (*Adapter) VerifySession(ctx context.Context, env clients.Env, plan clients.SessionPlan) error {
	return verifySessionNativeSource(ctx, env, plan)
}

func hermesCLISurface(surfaces []domain.ClientSurface) bool {
	for _, surface := range surfaces {
		if surface.ID == "hermes_cli" && surface.Detected {
			return true
		}
	}
	return false
}

func hermesProjectAuthority(project, user string, environment []string) error {
	home := projectSessionRoot(project)
	canonical, err := New().ResolveProfileRoot(user)
	if err != nil || canonical != user || home == user ||
		pathpolicy.RequireContainedChild(user, home) == nil || pathpolicy.RequireContainedChild(home, user) == nil {
		return fmt.Errorf("hermes Project MCP home overlaps or aliases selected User authority")
	}
	return hermesSessionSignals(home, environment)
}

// The pinned CLI reads active_profile before its normal configuration imports.
// Presence of any node is a refusal; its foreign contents are never consumed.
func hermesSessionSignals(home string, environment []string) error {
	if !cleanRoot(home) || strings.Contains(home, "$") || runtime.GOOS == "windows" && strings.Contains(home, "%") {
		return fmt.Errorf("hermes Project home contains native environment expansion")
	}
	canonical, err := New().ResolveProfileRoot(home)
	if err != nil || canonical != home {
		return fmt.Errorf("hermes Project home aliases another directory")
	}
	values := make(map[string]string)
	for _, entry := range environment {
		key, value, _ := strings.Cut(entry, "=")
		if runtime.GOOS == "windows" {
			key = strings.ToUpper(key)
		}
		values[key] = value
	}
	if err := hermesNativeSourceSignals(home, values); err != nil {
		return err
	}
	userHome := values["HOME"]
	if runtime.GOOS == "windows" {
		userHome = values["USERPROFILE"]
	}
	if !cleanRoot(userHome) {
		return fmt.Errorf("hermes inherited native home cannot be determined exactly")
	}
	// The platform default ignores HERMES_HOME; it is a separate auth fallback.
	delete(values, "HERMES_HOME")
	native, err := selectedProfileRoot(clients.NewHost(clients.HostProbes{
		HomeDir: userHome, GOOS: runtime.GOOS, Environment: values,
	}))
	if err != nil {
		return err
	}
	native, err = New().ResolveProfileRoot(native)
	if err != nil || home == native || pathpolicy.RequireContainedChild(native, home) == nil ||
		pathpolicy.RequireContainedChild(home, native) == nil {
		return fmt.Errorf("hermes Project home overlaps native default authority")
	}
	if _, err := os.Lstat(filepath.Join(home, "active_profile")); !os.IsNotExist(err) {
		return fmt.Errorf("hermes Project session has an active-profile redirect or unavailable marker authority")
	}
	return nil
}

// The pinned backend and managed-overlay selectors can replace the primary
// file source. Refuse them without loading or changing administrator policy.
func hermesNativeSourceSignals(home string, values map[string]string) error {
	backend := strings.ToLower(strings.TrimSpace(values["HERMES_CONFIG_BACKEND"]))
	if backend != "" && backend != "file" || strings.TrimSpace(values["HERMES_MANAGED_DIR"]) != "" {
		return fmt.Errorf("hermes Project session has a different native configuration source")
	}
	// Python Path('/etc/hermes') uses the command directory's volume on Windows,
	// including a UNC share. The private pytest waiver grants no authority here.
	managed := filepath.Join(filepath.VolumeName(home)+string(os.PathSeparator), "etc", "hermes")
	if _, err := os.Lstat(managed); !os.IsNotExist(err) {
		return fmt.Errorf("hermes Project session has a managed source or unavailable managed authority")
	}
	return nil
}
