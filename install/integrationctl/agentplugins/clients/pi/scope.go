package pi

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
    "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
    "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// ProjectAdapter adds only the explicitly selected project layout. The default
// Pi registry keeps its existing user-only behavior and cannot silently
// redirect a user installation into a workspace.
type ProjectAdapter struct{ *Adapter }

func NewProject() *ProjectAdapter { return &ProjectAdapter{Adapter: New()} }

var (
    _ clients.ProjectTargetLayout = (*ProjectAdapter)(nil)
    _ clients.NativeRegistryLayout = (*ProjectAdapter)(nil)
)

func (*ProjectAdapter) NativeRegistry(in clients.PlanInput) (string, string) {
    if in.TargetContext.Scope == domain.ScopeProject {
        root, err := projectPiRoot(in.Client, in.TargetContext, "")
        if err != nil {
            // The interface cannot return an error; an empty native root
            // makes the planner refuse this project before source loading.
            return "", ""
        }
        return root, in.Client.ExecutablePath
    }
    return in.Client.ConfigRoot, in.Client.ExecutablePath
}

func (*ProjectAdapter) ProjectTargetRoot(client domain.DetectedClient, mode domain.PackageMode, _ string, target domain.TargetContext) (string, string, error) {
    if mode != domain.PackageNative {
        return "", "", fmt.Errorf("Pi project extensions require native package delivery")
    }
    root, err := projectPiRoot(client, target, "")
    if err != nil {
        return "", "", err
    }
    return root, filepath.Join(root, "extensions"), nil
}

func projectPiRoot(client domain.DetectedClient, target domain.TargetContext, planned string) (string, error) {
    scoped, err := domain.NewTargetContext(target.Scope, target.ProjectRoot, target.UserConfigRoot)
    if err != nil {
        return "", err
    }
    if client.ClientID != domain.ClientPi || scoped.Scope != domain.ScopeProject ||
        scoped.UserConfigRoot != client.ConfigRoot || client.ConfigRoot == "" {
        return "", fmt.Errorf("Pi project selection differs from the original user profile")
    }
    if err := validateProfile(client.ConfigRoot); err != nil {
        return "", fmt.Errorf("Pi project requires a canonical original user profile: %w", err)
    }
    root := filepath.Join(scoped.ProjectRoot, ".pi")
    if planned != "" && planned != root {
        return "", fmt.Errorf("Pi project registry differs from selected workspace")
    }
    if err := pathpolicy.RequireContainedChild(scoped.ProjectRoot, root); err != nil {
        return "", err
    }
    user := client.ConfigRoot
    if user != "" && (root == user || strings.HasPrefix(root, user+string(filepath.Separator)) ||
        strings.HasPrefix(user, root+string(filepath.Separator))) {
        return "", fmt.Errorf("Pi project and user profile overlap")
    }
    for _, path := range []string{root, filepath.Join(root, "extensions")} {
        info, err := os.Lstat(path)
        if err == nil {
            if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
                return "", fmt.Errorf("Pi project discovery directory is not a real directory")
            }
        } else if !os.IsNotExist(err) {
            return "", err
        }
    }
    return root, nil
}

func (*ProjectAdapter) CheckPlanPrecondition(in clients.PlanInput, plan *domain.DeliveryPlan) error {
    if in.TargetContext.Scope != domain.ScopeProject || plan.Scope != domain.ScopeProject {
        return fmt.Errorf("Pi project adapter requires explicit project scope")
    }
    root, err := projectPiRoot(in.Client, in.TargetContext, plan.NativeRegistryRoot)
    if err != nil {
        return err
    }
    return checkDiscoveryRoot(filepath.Join(root, "extensions"))
}
