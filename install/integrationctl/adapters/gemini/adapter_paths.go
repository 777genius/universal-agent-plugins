package gemini

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (a Adapter) extensionDir(name string) string {
	return filepath.Join(a.userHome(), ".gemini", "extensions", name)
}

func (a Adapter) validateExtensionDir(name, candidate string) error {
	if err := pathpolicy.ValidateLeafID(name); err != nil {
		return fmt.Errorf("invalid Gemini integration id: %w", err)
	}
	if err := pathpolicy.RequireExactPath(a.extensionDir(name), candidate); err != nil {
		return fmt.Errorf("unsafe Gemini extension dir: %w", err)
	}
	return nil
}

func (a Adapter) managedSourceRoot(name string) string {
	return filepath.Join(a.userHome(), ".plugin-kit-ai", "materialized", "gemini", name)
}

func (a Adapter) validateManagedSourceRoot(name, candidate string) error {
	if err := pathpolicy.ValidateLeafID(name); err != nil {
		return fmt.Errorf("invalid Gemini integration id: %w", err)
	}
	if err := pathpolicy.RequireExactPath(a.managedSourceRoot(name), candidate); err != nil {
		return fmt.Errorf("unsafe Gemini managed source root: %w", err)
	}
	return nil
}

func scopeFromInspectInput(in ports.InspectInput) string {
	if in.Record != nil {
		return scopeFromRecord(*in.Record)
	}
	return defaultScope(in.Scope)
}

func scopeFromRecord(record domain.InstallationRecord) string {
	return defaultScope(record.Policy.Scope)
}

func defaultScope(scope string) string {
	return pathpolicy.NormalizeScope(scope)
}

func geminiToggleScope(scope string) string {
	if scope == "project" {
		return "workspace"
	}
	return "user"
}

func (a Adapter) settingsPath(scope string, workspaceRoot string) string {
	if scope == "project" {
		if root := effectiveWorkspaceRoot(workspaceRoot); root != "" {
			return filepath.Join(root, ".gemini", "settings.json")
		}
	}
	return filepath.Join(a.userHome(), ".gemini", "settings.json")
}

func (a Adapter) enablementPath() string {
	return filepath.Join(a.userHome(), ".gemini", "extensions", "extension-enablement.json")
}

func (a Adapter) systemSettingsPaths() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/Library/Application Support/GeminiCli/system-defaults.json",
			"/Library/Application Support/GeminiCli/settings.json",
		}
	case "windows":
		programData := strings.TrimSpace(os.Getenv("ProgramData"))
		if programData == "" {
			programData = `C:\ProgramData`
		}
		return []string{
			filepath.Join(programData, "gemini-cli", "system-defaults.json"),
			filepath.Join(programData, "gemini-cli", "settings.json"),
		}
	default:
		return []string{
			"/etc/gemini-cli/system-defaults.json",
			"/etc/gemini-cli/settings.json",
		}
	}
}

func workspaceRootFromInspectInput(in ports.InspectInput) string {
	if in.Record != nil {
		return workspaceRootFromRecord(*in.Record)
	}
	return workspaceRootForScope(defaultScope(in.Scope), "")
}

func workspaceRootFromRecord(record domain.InstallationRecord) string {
	return workspaceRootForScope(defaultScope(record.Policy.Scope), record.WorkspaceRoot)
}

func workspaceRootForScope(scope string, workspaceRoot string) string {
	if scope != "project" {
		return ""
	}
	return effectiveWorkspaceRoot(workspaceRoot)
}

func effectiveWorkspaceRoot(workspaceRoot string) string {
	return pathpolicy.ProjectRoot(workspaceRoot, "")
}

func workspaceSettingsPath(workspaceRoot string) string {
	if strings.TrimSpace(workspaceRoot) == "" {
		return ""
	}
	return filepath.Join(workspaceRoot, ".gemini", "settings.json")
}

func (a Adapter) commandDirForRecord(record domain.InstallationRecord) string {
	if strings.EqualFold(strings.TrimSpace(record.Policy.Scope), "project") {
		return workspaceRootFromRecord(record)
	}
	return ""
}
