package manifest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/authoredpath"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
	"gopkg.in/yaml.v3"
)

type Loader struct{}

type pluginYAML struct {
	APIVersion  string   `yaml:"api_version"`
	Name        string   `yaml:"name"`
	Version     string   `yaml:"version"`
	Description string   `yaml:"description"`
	Targets     []string `yaml:"targets"`
}

func (Loader) Load(_ context.Context, source ports.ResolvedSource) (domain.IntegrationManifest, error) {
	_, data, err := readPluginYAML(source.LocalPath)
	if err != nil {
		return domain.IntegrationManifest{}, domain.NewError(domain.ErrManifestLoad, "read plugin.yaml", err)
	}
	var raw pluginYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return domain.IntegrationManifest{}, domain.NewError(domain.ErrManifestLoad, "parse plugin.yaml", err)
	}
	if strings.TrimSpace(raw.APIVersion) != "v1" {
		return domain.IntegrationManifest{}, domain.NewError(domain.ErrManifestLoad, "plugin.yaml api_version must be v1", nil)
	}
	if strings.TrimSpace(raw.Name) == "" || strings.TrimSpace(raw.Version) == "" || strings.TrimSpace(raw.Description) == "" {
		return domain.IntegrationManifest{}, domain.NewError(domain.ErrManifestLoad, "plugin.yaml requires name, version, description", nil)
	}
	if err := pathpolicy.ValidateLeafID(raw.Name); err != nil {
		return domain.IntegrationManifest{}, domain.NewError(domain.ErrManifestLoad, "plugin.yaml name is not a safe portable identifier", err)
	}
	if len(raw.Targets) == 0 {
		return domain.IntegrationManifest{}, domain.NewError(domain.ErrManifestLoad, "plugin.yaml targets must not be empty", nil)
	}
	deliveries := make([]domain.Delivery, 0, len(raw.Targets))
	seen := map[domain.TargetID]struct{}{}
	for _, target := range raw.Targets {
		delivery, err := mapTarget(strings.ToLower(strings.TrimSpace(target)), raw.Name, source.LocalPath)
		if err != nil {
			return domain.IntegrationManifest{}, err
		}
		if _, ok := seen[delivery.TargetID]; ok {
			continue
		}
		seen[delivery.TargetID] = struct{}{}
		deliveries = append(deliveries, delivery)
	}
	sum := sha256.Sum256(data)
	return domain.IntegrationManifest{
		IntegrationID:  strings.TrimSpace(raw.Name),
		Version:        strings.TrimSpace(raw.Version),
		Description:    strings.TrimSpace(raw.Description),
		RequestedRef:   source.Requested,
		ResolvedRef:    source.Resolved,
		SourceDigest:   source.SourceDigest,
		ManifestDigest: "sha256:" + hex.EncodeToString(sum[:]),
		Deliveries:     deliveries,
	}, nil
}

func readPluginYAML(root string) (string, []byte, error) {
	for _, path := range authoredpath.ManifestCandidates(root) {
		data, err := os.ReadFile(path)
		if err == nil {
			return path, data, nil
		}
		if !os.IsNotExist(err) {
			return "", nil, err
		}
	}
	return "", nil, os.ErrNotExist
}

func mapTarget(target, name, sourceRoot string) (domain.Delivery, error) {
	switch target {
	case "claude":
		return domain.Delivery{TargetID: domain.TargetClaude, DeliveryKind: domain.DeliveryClaudeMarketplace, Name: name, NativeRefHint: name, CapabilitySurface: []string{"skills", "commands", "agents", "hooks", "mcp"}}, nil
	case "codex-package":
		return domain.Delivery{TargetID: domain.TargetCodex, DeliveryKind: domain.DeliveryCodexMarketplace, Name: name, NativeRefHint: name, CapabilitySurface: []string{"plugin_bundle", "skills", "mcp", "app"}}, nil
	case "gemini":
		return domain.Delivery{TargetID: domain.TargetGemini, DeliveryKind: domain.DeliveryGeminiExtension, Name: name, NativeRefHint: name, CapabilitySurface: []string{"contexts", "settings", "themes", "commands", "policies", "hooks", "agents", "skills", "mcp"}}, nil
	case "cursor":
		if cursorPluginPackageExists(sourceRoot) {
			return domain.Delivery{TargetID: domain.TargetCursor, DeliveryKind: domain.DeliveryCursorPlugin, Name: name, NativeRefHint: name, CapabilitySurface: []string{"plugin_bundle", "skills", "rules", "agents", "commands", "hooks", "mcp"}}, nil
		}
		return domain.Delivery{TargetID: domain.TargetCursor, DeliveryKind: domain.DeliveryCursorMCP, Name: name, NativeRefHint: name, CapabilitySurface: []string{"mcp"}}, nil
	case "opencode":
		return domain.Delivery{TargetID: domain.TargetOpenCode, DeliveryKind: domain.DeliveryOpenCodePlugin, Name: name, NativeRefHint: name, CapabilitySurface: []string{"plugin", "mcp", "skills", "commands", "agents", "themes", "tools"}}, nil
	default:
		return domain.Delivery{}, domain.NewError(domain.ErrUnsupportedTarget, fmt.Sprintf("unsupported integrationctl target: %s", target), nil)
	}
}

func cursorPluginPackageExists(sourceRoot string) bool {
	if strings.TrimSpace(sourceRoot) == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(sourceRoot, ".cursor-plugin", "plugin.json"))
	return err == nil && !info.IsDir()
}
