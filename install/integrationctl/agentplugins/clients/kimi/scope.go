package kimi

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	_ clients.ProjectTargetLayout     = (*Adapter)(nil)
	_ clients.NativeRegistryLayout    = (*Adapter)(nil)
	_ clients.PlanQualifier           = (*Adapter)(nil)
	_ clients.PlanPrecondition        = (*Adapter)(nil)
	_ clients.ProfileBindingValidator = (*Adapter)(nil)
)

func (*Adapter) NativeRegistry(in clients.PlanInput) (string, string) {
	if in.TargetContext.Scope == domain.ScopeProject {
		return filepath.Join(in.TargetContext.ProjectRoot, ".kimi-code"), ""
	}
	return in.Client.ConfigRoot, in.Client.ExecutablePath
}

func (*Adapter) ProjectTargetRoot(client domain.DetectedClient, mode domain.PackageMode, managedRoot string, target domain.TargetContext) (string, string, error) {
	if _, err := kimiProjectRoot(client, target, ""); err != nil {
		return "", "", err
	}
	if err := pathpolicy.RequireContainedChild(target.ProjectRoot, managedRoot); err != nil {
		return "", "", err
	}
	return shared.ManagedTargetRoot(client, mode, managedRoot)
}

func (*Adapter) CheckPlanPrecondition(in clients.PlanInput, plan *domain.DeliveryPlan) error {
	if in.TargetContext.Scope != domain.ScopeProject {
		return nil
	}
	if _, err := kimiProjectRoot(in.Client, in.TargetContext, plan.NativeRegistryRoot); err != nil {
		return err
	}
	if err := currentKimi(in.Client); err != nil {
		return err
	}
	if len(in.Envelope.Skills) > 0 || len(in.Envelope.Commands) > 0 {
		return kimiSkillRoot(in.TargetContext.ProjectRoot)
	}
	return nil
}

func currentKimi(client domain.DetectedClient) error {
	name := filepath.Base(client.ExecutablePath)
	if (name != "kimi" && name != "kimi.exe") || !filepath.IsAbs(client.ExecutablePath) || filepath.Clean(client.ExecutablePath) != client.ExecutablePath || client.Version != "2.3.1" {
		return fmt.Errorf("kimi project components require the explicitly selected current Kimi Code 2.3.1 executable/version; archived or unknown harnesses are unqualified")
	}
	return nil
}

func kimiProjectRoot(client domain.DetectedClient, target domain.TargetContext, planned string) (string, error) {
	validated, err := domain.NewTargetContext(target.Scope, target.ProjectRoot, target.UserConfigRoot)
	if err != nil {
		return "", err
	}
	if validated.Scope != domain.ScopeProject {
		return "", fmt.Errorf("kimi components require project scope")
	}
	if validated.UserConfigRoot != client.ConfigRoot {
		return "", fmt.Errorf("kimi original user profile differs from selected target")
	}
	root := filepath.Join(validated.ProjectRoot, ".kimi-code")
	if planned != "" && planned != root {
		return "", fmt.Errorf("kimi native registry differs from selected project")
	}
	return root, pathpolicy.RequireContainedChild(validated.ProjectRoot, root)
}

func kimiPlanRoot(client domain.DetectedClient, plan domain.DeliveryPlan) (string, error) {
	if plan.Scope != domain.ScopeProject || plan.TargetContext.Scope != domain.ScopeProject || plan.ClientID != domain.ClientKimi {
		return "", fmt.Errorf("kimi project plan scope/client mismatch")
	}
	root, err := kimiProjectRoot(client, plan.TargetContext, plan.NativeRegistryRoot)
	if err != nil {
		return "", err
	}
	if err := pathpolicy.ValidateLeafID(plan.PhysicalArtifactID); err != nil {
		return "", err
	}
	expected := filepath.Join(plan.TargetContext.ProjectRoot, ".uap", "managed", "clients", "kimi", plan.PhysicalArtifactID)
	if plan.ActivePath != expected {
		return "", fmt.Errorf("kimi managed package differs from selected project/artifact")
	}
	return root, pathpolicy.RequireContainedChild(plan.TargetContext.ProjectRoot, expected)
}

// Native skill discovery chooses the nearest existing .git marker. Never move
// selected skills into a different ancestor, or qualify an unknown fallback.
func kimiSkillRoot(project string) error {
	root, err := kimiNearestGitRoot(project)
	if err != nil {
		return err
	}
	if root == "" {
		return fmt.Errorf("kimi project skill discovery requires an observed .git marker at the selected project root")
	}
	if root != project {
		return fmt.Errorf("kimi skills resolve at nearest Git ancestor %s, which differs from selected project", root)
	}
	return nil
}

func kimiNearestGitRoot(project string) (string, error) {
	for directory := project; ; directory = filepath.Dir(directory) {
		marker := filepath.Join(directory, ".git")
		info, err := os.Lstat(marker)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
				return "", fmt.Errorf("kimi nearest Git marker must be a real directory or regular gitfile")
			}
			return directory, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		if filepath.Dir(directory) == directory {
			return "", nil
		}
	}
}

func refineKimiProject(in clients.PlanInput, plan *domain.DeliveryPlan) error {
	root, err := kimiPlanRoot(in.Client, *plan)
	if err != nil {
		return err
	}
	plan.NativeRegistryRoot, plan.NativeRegistryExecutable = root, ""
	if plan.Status == domain.PlanUnsupported || !shared.OnlyNativeComponents(plan.Components) {
		return nil
	}
	plan.Status, plan.Activation, plan.Verification = domain.PlanReady, domain.ActivationPrepared, domain.VerificationPackageValid
	plan.UserActions = []string{KimiProjectAction}
	compatibilityRoot, err := kimiNearestGitRoot(in.TargetContext.ProjectRoot)
	if err != nil {
		return err
	}
	if compatibilityRoot == "" {
		compatibilityRoot = in.TargetContext.ProjectRoot
	}
	for _, path := range []string{filepath.Join(in.Client.ConfigRoot, "mcp.json"), filepath.Join(compatibilityRoot, ".mcp.json")} {
		if in.Client.ConfigRoot == "" && !filepath.IsAbs(path) {
			continue
		}
		names, err := kimiConfigNames(path)
		if err != nil {
			return err
		}
		for _, name := range shared.SupportedMCPNames(*plan) {
			if names[name] {
				plan.Warnings = shared.AppendUnique(plan.Warnings, "Kimi selected project MCP overrides a same-name lower-priority user or compatibility entry; effective trust/session loading remains unobserved")
			}
		}
	}
	return nil
}

func (*Adapter) ValidateBindingProfile(root string, binding domain.ClientBinding) error {
	if binding.Scope != string(domain.ScopeProject) {
		return nil
	}
	if binding.ClientID != "kimi" {
		return fmt.Errorf("kimi recorded project client differs")
	}
	if binding.NativeProfileRoot != root || filepath.Base(root) != ".kimi-code" {
		return fmt.Errorf("kimi recorded project registry differs")
	}
	if err := pathpolicy.ValidateLeafID(binding.PhysicalArtifact); err != nil {
		return err
	}
	expected := filepath.Join(filepath.Dir(root), ".uap", "managed", "clients", "kimi", binding.PhysicalArtifact)
	if binding.TargetLocator != expected {
		return fmt.Errorf("kimi recorded package differs from project/artifact")
	}
	if err := pathpolicy.RequireContainedChild(filepath.Dir(root), expected); err != nil {
		return err
	}
	return kimiComponents(root).ValidateObjects(kimiNativeObjects(binding.NativeObjects))
}

func kimiConfigNames(path string) (map[string]bool, error) {
	file, err := readKimiFile(path)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(file)) == "" {
		return map[string]bool{}, nil
	}
	doc, err := shared.DecodeStrictJSONObject(file)
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	raw, present := doc["mcpServers"]
	if !present {
		return names, nil
	}
	entries, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("kimi MCP collection must be an object")
	}
	for name := range entries {
		names[name] = true
	}
	return names, nil
}

// Current pinned Kimi supports SSE in the project file. Its preserved user
// plugin capability remains unsupported; qualify only this observed lane.
func (*Adapter) QualifyPlan(in clients.PlanInput, plan *domain.DeliveryPlan) error {
	if in.TargetContext.Scope != domain.ScopeProject {
		return nil
	}
	if err := currentKimi(in.Client); err != nil {
		return err
	}
	for i, component := range plan.Components {
		if component.Kind == domain.ComponentMCPServer && in.Envelope.MCP.Servers[component.Name].Type == "sse" {
			plan.Components[i].Support, plan.Components[i].Reason = domain.SupportNative, ""
		}
	}
	return nil
}
