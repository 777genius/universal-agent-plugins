package cline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.ProfileBindingValidator = (*Adapter)(nil)

func (*Adapter) ValidateBindingProfile(root string, binding domain.ClientBinding) error {
	if binding.Scope != string(domain.ScopeProject) {
		return nil
	}
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || binding.ClientID != "cline" || binding.NativeProfileRoot != root {
		return fmt.Errorf("cline project binding scope or namespace differs")
	}
	if err := pathpolicy.ValidateLeafID(binding.PhysicalArtifact); err != nil {
		return err
	}
	expected := filepath.Join(root, ".uap", "managed", "clients", "cline", binding.PhysicalArtifact)
	if binding.TargetLocator != expected {
		return fmt.Errorf("cline project binding artifact differs")
	}
	if err := pathpolicy.RequireContainedChild(root, expected); err != nil {
		return err
	}
	for _, object := range binding.NativeObjects {
		if object.Kind == "managed_package_directory" && (object.Path != expected || object.ManagedDigest == "") {
			return fmt.Errorf("cline project package ownership differs")
		}
	}
	return clineProjectComponents(root).ValidateObjects(clineProjectOwned(binding.NativeObjects))
}

func validateClineProjection(root string, plan domain.DeliveryPlan, active bool) error {
	if _, err := clineProjectPlanRoot(domain.DetectedClient{ClientID: domain.ClientCline, ConfigRoot: plan.TargetContext.UserConfigRoot}, plan); err != nil {
		return err
	}
	if active {
		if root != plan.ActivePath {
			return fmt.Errorf("cline active package differs from plan")
		}
	} else if filepath.Dir(root) != plan.TargetRoot || !strings.HasPrefix(filepath.Base(root), ".agentplugins-staging-") {
		return fmt.Errorf("cline staging differs from exact selected target")
	}
	if !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return fmt.Errorf("cline package path must be absolute and clean")
	}
	if err := pathpolicy.RequireContainedChild(plan.TargetContext.ProjectRoot, root); err != nil {
		return err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("cline package must be a real directory")
	}
	return nil
}

func clineProjectAction(plan domain.DeliveryPlan) string {
	project, _ := json.Marshal(plan.TargetContext.ProjectRoot)
	action := fmt.Sprintf("After Cline project component files are installed, start Cline from working directory %s and inspect skills. Native session discovery, IDE behavior, trust and MCP initialization remain unobserved", project)
	if !shared.ComponentKindPresent(plan.Components, domain.ComponentMCPServer) {
		return action
	}
	path, _ := json.Marshal(clineProjectComponents(plan.TargetContext.ProjectRoot).ConfigPaths.JSON)
	args, _ := json.Marshal([]string{"--auto-approve", "false", "--cwd", plan.TargetContext.ProjectRoot})
	return action + fmt.Sprintf(". For one Cline CLI process, set environment CLINE_MCP_SETTINGS_PATH to %s and CLINE_NO_AUTO_UPDATE to 1, and use arguments %s. This selects the whole MCP source; original user MCP entries are not merged or copied, and a later ordinary process returns to its user source", path, args)
}
