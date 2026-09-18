package gemini

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func BuildGeminiNativeObjects(stagingRoot string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, pluginDataPath string) ([]domain.NativeObjectOwnership, error) {
	configRoot := strings.TrimSpace(plan.NativeRegistryRoot)
	if configRoot == "" || !filepath.IsAbs(configRoot) {
		return nil, fmt.Errorf("the Gemini config root is unavailable")
	}
	objects := make([]domain.NativeObjectOwnership, 0, len(envelope.Skills)+len(envelope.MCP.Servers))
	for _, component := range plan.Components {
		object, err := geminiPlannedOwnership(stagingRoot, configRoot, envelope, plan, pluginDataPath, component)
		if err != nil {
			return nil, err
		}
		if object == nil {
			continue
		}
		objects = append(objects, *object)
	}
	descriptorBody, err := json.Marshal(GeminiDescriptor{DataRoot: pluginDataPath})
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(stagingRoot, GeminiDescriptorName), append(descriptorBody, '\n'), 0o600); err != nil {
		return nil, fmt.Errorf("write Gemini projection descriptor: %w", err)
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].ObjectID < objects[j].ObjectID })
	return objects, nil
}

func geminiPlannedOwnership(stagingRoot, configRoot string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, pluginDataPath string, component domain.ComponentDecision) (*domain.NativeObjectOwnership, error) {
	if component.Support == domain.SupportUnsupported {
		return nil, nil
	}
	if err := pathpolicy.ValidateLeafID(component.Name); err != nil {
		return nil, fmt.Errorf("invalid Gemini component name %q: %w", component.Name, err)
	}
	switch component.Kind {
	case domain.ComponentSkill:
		object, err := geminiSkillOwnership(stagingRoot, configRoot, envelope, component)
		if err != nil {
			return nil, err
		}
		return &object, nil
	case domain.ComponentMCPServer:
		object, err := geminiMCPOwnership(stagingRoot, configRoot, envelope, plan, pluginDataPath, component)
		if err != nil {
			return nil, err
		}
		return &object, nil
	default:
		return nil, nil
	}
}

func geminiSkillOwnership(stagingRoot, configRoot string, envelope domain.PackageEnvelope, component domain.ComponentDecision) (domain.NativeObjectOwnership, error) {
	skill, ok := envelope.Skills[component.Name]
	if !ok {
		return domain.NativeObjectOwnership{}, fmt.Errorf("planned Gemini skill %q is missing", component.Name)
	}
	relative := filepath.FromSlash(strings.TrimSpace(skill.RelativePath))
	if relative == "" {
		relative = filepath.Join("skills", component.Name, "SKILL.md")
	}
	sourceRelative := filepath.Dir(relative)
	sourceRoot := filepath.Join(stagingRoot, sourceRelative)
	if err := pathpolicy.RequireContainedChild(stagingRoot, sourceRoot); err != nil {
		return domain.NativeObjectOwnership{}, fmt.Errorf("unsafe Gemini skill source %q: %w", component.Name, err)
	}
	digest, err := shared.DigestSkillDirectory(sourceRoot)
	if err != nil {
		return domain.NativeObjectOwnership{}, fmt.Errorf("digest Gemini skill %q: %w", component.Name, err)
	}
	return domain.NativeObjectOwnership{
		ObjectID: "gemini-skill:" + component.Name, Kind: GeminiSkillObjectKind,
		LogicalName: component.Name, Path: filepath.Join(configRoot, "skills", component.Name),
		SourceRelative: filepath.ToSlash(sourceRelative), ManagedDigest: digest, ProtectionClass: "managed",
	}, nil
}

func geminiMCPOwnership(stagingRoot, configRoot string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, pluginDataPath string, component domain.ComponentDecision) (domain.NativeObjectOwnership, error) {
	server, ok := envelope.MCP.Servers[component.Name]
	if !ok {
		return domain.NativeObjectOwnership{}, fmt.Errorf("planned Gemini MCP server %q is missing", component.Name)
	}
	native, err := GeminiNativeServer(server)
	if err != nil {
		return domain.NativeObjectOwnership{}, fmt.Errorf("project Gemini MCP server %q: %w", component.Name, err)
	}
	native, err = MaterializeGeminiServer(native, plan.ActivePath, pluginDataPath, stagingRoot)
	if err != nil {
		return domain.NativeObjectOwnership{}, fmt.Errorf("bind Gemini MCP server %q: %w", component.Name, err)
	}
	receipt, err := nativeconfig.DesiredReceipt(filepath.Join(configRoot, "settings.json"), nativeconfig.CodecGemini, component.Name, native, nativeconfig.Placeholders{PackageRoot: plan.ActivePath, DataRoot: pluginDataPath})
	if err != nil {
		return domain.NativeObjectOwnership{}, err
	}
	return domain.NativeObjectOwnership{
		ObjectID: "gemini-mcp:" + component.Name, Kind: GeminiMCPObjectKind,
		LogicalName: component.Name, Path: filepath.Join(configRoot, "settings.json"),
		ManagedDigest: receipt.Digest, ProtectionClass: "managed",
	}, nil
}
