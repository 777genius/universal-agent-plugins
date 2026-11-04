package qoder

import (
    "context"
    "errors"
    "fmt"
    "os"
    "path/filepath"

    "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
    "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
    "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
    "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.ActiveNativeProjector = (*Adapter)(nil)

// Active projection reads the immutable selected package; it never treats the
// original user profile or the native import cache as writable authority.
func (*Adapter) ProjectActiveNative(ctx context.Context, root string, _ domain.PackageEnvelope, plan domain.DeliveryPlan, _ string) ([]domain.NativeObjectOwnership, error) {
    if err := ctx.Err(); err != nil { return nil, err }
    if !projectRequested(plan) { return nil, nil }
    if err := validateProjectPlan(domain.DetectedClient{ClientID: domain.ClientQoder, ConfigRoot: plan.TargetContext.UserConfigRoot}, plan); err != nil { return nil, err }
    if root != plan.ActivePath { return nil, fmt.Errorf("Qoder active project package differs from planned identity") }
    return qoderProjectObjects(root, plan)
}

const qoderProjectNativeAction = "Restart Qoder in the selected project and review native Skill/MCP discovery and MCP approval; installed files do not attest to initialized servers, tools, or authentication."

func qoderNativePreflight(env clients.Env, request domain.ActivationRequest) error {
    if err := validateQoderProjectSelection(request.Plan, qoderNativeOwned(request.Delivery.NativeObjects)); err != nil { return err }
    layout := qoderComponents(request.Plan.TargetContext.ProjectRoot)
    if err := layout.ValidateObjects(qoderNativeOwned(request.PreviousNativeObjects)); err != nil { return err }
    if err := env.NativeConfig.RequireFileIO(); err != nil { return err }
    return qoderHigherPriorityConflicts(request.Plan)
}

func activateQoderNative(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
    if err := qoderNativePreflight(env, request); err != nil { return domain.ActivationOutcome{}, err }
    layout := qoderComponents(request.Plan.TargetContext.ProjectRoot)
    desired, previous := qoderNativeOwned(request.Delivery.NativeObjects), qoderNativeOwned(request.PreviousNativeObjects)
    outcome, err := shared.CompleteNativeConfigActivation(ctx, request, shared.NativeConfigActivation{
        Automatic: request.Plan.InstallIntent == domain.InstallIntentAutomatic,
        UnavailableAction: qoderProjectNativeAction,
        RepairAction: "repair exact owned Qoder project Skills/MCP",
        RetryAction: "retry Qoder project native component installation",
        CompletedAction: qoderProjectNativeAction,
        Verify: func() error { return layout.Verify(desired, false, env.NativeConfig) },
        Activate: func(ctx context.Context, r domain.ActivationRequest) error {
            servers, err := qoderDesiredServers(r.Delivery.ActivePath, desired)
            if err != nil { return &shared.NativeEffectError{Effect: domain.NativeEffectUnchanged, Err: err} }
            return (shared.ProjectComponentMutation{Layout: layout, ActivePath:r.Delivery.ActivePath,
                Previous:previous, Desired:desired, Servers:servers, Kernel:env.NativeConfig}).Apply(ctx)
        },
    })
    if err == nil && outcome.Verification == domain.VerificationInstalled {
        outcome.Activation, outcome.ActivationAttested = domain.ActivationPrepared, false
    }
    return outcome, err
}

func deactivateQoderNative(ctx context.Context, env clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
    out := shared.StartedDeactivation()
    objects := qoderNativeOwned(request.NativeObjects)
    layout := qoderComponents(request.TargetContext.ProjectRoot)
    if err := layout.ValidateObjects(objects); err != nil { return out, err }
    if err := qoderHigherPriorityConflicts(domain.DeliveryPlan{
        TargetContext: request.TargetContext, Scope:domain.ScopeProject,
        Components: qoderDecisions(objects),
    }); err != nil { return out, err }
    if !request.Confirmed {
        out.UserActions = []string{"Remove only exactly owned Qoder project Skills and MCP members; leave the user profile and foreign config untouched."}
        return out, nil
    }
    err := (shared.ProjectComponentMutation{Layout:layout, Previous:objects, Kernel:env.NativeConfig}).Apply(ctx)
    if err != nil {
        var observed *shared.NativeEffectError
        if errors.As(err, &observed) && observed.Effect == domain.NativeEffectUnchanged {
            out.NativeEffect = domain.NativeEffectUnchanged
            return out, err
        }
        if !shared.CommittedNativeDeactivationCleanup(&out, err) { return out, err }
    }
    out.NativeEffect = domain.NativeEffectCommitted
    out.ExternalRemovalComplete = true
    out.UserActions = append(out.UserActions, "Restart Qoder to clear previously loaded project Skills and MCP tools.")
    return out, nil
}

func qoderDecisions(objects []domain.NativeObjectOwnership) []domain.ComponentDecision {
    selected := make([]domain.ComponentDecision, 0, len(objects))
    for _, object := range objects {
        kind := domain.ComponentSkill
        if object.Kind == qoderProjectMCPKind { kind = domain.ComponentMCPServer }
        selected = append(selected, domain.ComponentDecision{Name:object.LogicalName, Kind:kind, Support:domain.SupportNative})
    }
    return selected
}

// Foreign local MCP and user Skill wins native selection. A policy conflict
// refuses before any native write. Qoder's command-line --mcp-config overrides
// cannot be inferred here; runtime E2E must check those separately.
func qoderHigherPriorityConflicts(plan domain.DeliveryPlan) error {
    project, user := plan.TargetContext.ProjectRoot, plan.TargetContext.UserConfigRoot
    for _, component := range plan.Components {
        if component.Support == domain.SupportUnsupported { continue }
        if component.Kind == domain.ComponentSkill {
            source := filepath.Join(user, "skills", component.Name)
            if err := pathpolicy.RequireContainedChild(user, source); err != nil { return err }
            if _, err := os.Lstat(source); err == nil {
                return fmt.Errorf("Qoder user Skill %q shadows selected project Skill", component.Name)
            } else if !os.IsNotExist(err) { return err }
        }
    }
    local := filepath.Join(project, ".qoder", "settings.local.json")
    if err := pathpolicy.RequireContainedChild(project, local); err != nil { return err }
    info, err := os.Lstat(local)
    if os.IsNotExist(err) { return nil }
    if err != nil { return err }
    if !info.Mode().IsRegular() || info.Size() > 1<<20 { return fmt.Errorf("Qoder local settings are not a bounded regular file") }
    body, err := os.ReadFile(local)
    if err != nil { return err }
    settings, err := shared.DecodeStrictJSONObject(body)
    if err != nil { return err }
    native, ok := settings["mcpServers"]
    if !ok { return nil }
    servers, ok := native.(map[string]any)
    if !ok { return fmt.Errorf("Qoder local MCP settings are ambiguous") }
    for _, component := range plan.Components {
        if component.Support != domain.SupportUnsupported && component.Kind == domain.ComponentMCPServer {
            if _, found := servers[component.Name]; found {
                return fmt.Errorf("Qoder higher-priority local MCP server %q shadows selected project server", component.Name)
            }
        }
    }
    return nil
}
