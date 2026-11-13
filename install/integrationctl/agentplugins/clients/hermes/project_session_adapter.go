package hermes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// ProjectAdapter is the selected-root candidate. The public registry continues
// to use New until genuine native admission qualifies this complete route.
type ProjectAdapter struct{ *Adapter }

func NewProject() *ProjectAdapter { return &ProjectAdapter{&Adapter{selectedProject: true}} }

var _ clients.SessionPlanner = (*ProjectAdapter)(nil)

func (adapter *ProjectAdapter) QualifyPlan(in clients.PlanInput, plan *domain.DeliveryPlan) error {
	if plan.Scope == domain.ScopeProject {
		for index, component := range plan.Components {
			server := in.Envelope.MCP.Servers[component.Name]
			if component.Kind == domain.ComponentMCPServer && (server.Type == "stdio" || server.Type == "streamable-http") {
				plan.Components[index].Support, plan.Components[index].Reason = domain.SupportNative, ""
			}
		}
	}
	return adapter.Adapter.QualifyPlan(in, plan)
}

func (adapter *Adapter) hasProjectSession(plan domain.DeliveryPlan) bool {
	return adapter.selectedProject && plan.Scope == domain.ScopeProject && shared.HasSupportedMCP(plan.Components)
}

func (adapter *Adapter) componentLayout(plan domain.DeliveryPlan, staged string) (shared.ProjectComponents, error) {
	layout, err := components(plan, staged)
	if err == nil && adapter.selectedProject && plan.Scope == domain.ScopeProject {
		layout.MCPKind, layout.Codec = projectSessionMCPKind, nativeconfig.CodecHermes
		layout.ConfigPaths.JSON = filepath.Join(projectSessionRoot(plan.TargetContext.ProjectRoot), "config.yaml")
	}
	return layout, err
}

func (adapter *Adapter) componentObjects(plan domain.DeliveryPlan, objects []domain.NativeObjectOwnership) ([]domain.NativeObjectOwnership, error) {
	if !adapter.selectedProject || plan.Scope != domain.ScopeProject {
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
	adapter := NewProject()
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
	if !adapter.selectedProject || plan.Scope != domain.ScopeProject {
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

func (*ProjectAdapter) PlanSession(ctx context.Context, env clients.Env, req clients.SessionRequest) (clients.SessionPlan, error) {
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

func (*ProjectAdapter) VerifySession(ctx context.Context, env clients.Env, plan clients.SessionPlan) error {
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
