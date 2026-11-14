package gemini

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (a Adapter) isDisabled(ctx context.Context, in ports.InspectInput) (bool, error) {
	if in.Record == nil {
		return false, nil
	}
	scope := scopeFromInspectInput(in)
	workspaceRoot := workspaceRootFromInspectInput(in)
	if disabled, handled, err := a.disabledByEnablement(ctx, scope, workspaceRoot, strings.TrimSpace(in.Record.IntegrationID)); err != nil {
		return false, err
	} else if handled {
		return disabled, nil
	}
	return a.disabledBySettings(ctx, scope, workspaceRoot, strings.TrimSpace(in.Record.IntegrationID))
}

func (a Adapter) disabledBySettings(ctx context.Context, scope string, workspaceRoot string, name string) (bool, error) {
	body, err := a.fs().ReadFile(ctx, a.settingsPath(scope, workspaceRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, domain.NewError(domain.ErrMutationApply, "read Gemini settings during inspect", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return false, domain.NewError(domain.ErrMutationApply, "parse Gemini settings during inspect", err)
	}
	extensions, ok := doc["extensions"].(map[string]any)
	if !ok || extensions == nil {
		return false, nil
	}
	raw, ok := extensions["disabled"].([]any)
	if !ok {
		return false, nil
	}
	for _, item := range raw {
		if s, ok := item.(string); ok && strings.TrimSpace(s) == name {
			return true, nil
		}
	}
	return false, nil
}

func (a Adapter) disabledByEnablement(ctx context.Context, scope string, workspaceRoot string, name string) (bool, bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return false, false, nil
	}
	body, err := a.fs().ReadFile(ctx, a.enablementPath())
	if err != nil {
		if os.IsNotExist(err) {
			return false, false, nil
		}
		return false, false, domain.NewError(domain.ErrMutationApply, "read Gemini extension enablement during inspect", err)
	}
	var doc map[string]struct {
		Overrides []string `json:"overrides"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return false, false, domain.NewError(domain.ErrMutationApply, "parse Gemini extension enablement during inspect", err)
	}
	entry, ok := doc[name]
	if !ok {
		return false, false, nil
	}
	return disabledByScopeOverride(scope, workspaceRoot, entry.Overrides)
}

func disabledByScopeOverride(scope string, workspaceRoot string, overrides []string) (bool, bool, error) {
	if scope == "project" {
		root := workspaceRootForScope(scope, workspaceRoot)
		if root == "" {
			return false, false, nil
		}
		expected := filepath.Clean(root) + string(os.PathSeparator) + "*"
		for _, override := range overrides {
			override = strings.TrimSpace(override)
			if override == "" {
				continue
			}
			if strings.TrimPrefix(override, "!") != expected {
				continue
			}
			return strings.HasPrefix(override, "!"), true, nil
		}
		return false, false, nil
	}
	for _, override := range overrides {
		override = strings.TrimSpace(override)
		if override == "!*" {
			return true, true, nil
		}
		if override == "*" {
			return false, true, nil
		}
	}
	return false, false, nil
}
