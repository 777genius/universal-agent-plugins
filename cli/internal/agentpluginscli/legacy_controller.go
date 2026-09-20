package agentpluginscli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	legacy "github.com/777genius/plugin-kit-ai/install/integrationctl"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

type legacyController struct {
	statePath string
}

func NewLegacyLifecycle(statePath string) ports.LegacyLifecycle {
	return legacyController{statePath: statePath}
}

func (controller legacyController) Exists(_ context.Context, name string) (bool, error) {
	body, err := os.ReadFile(controller.statePath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var state struct {
		Installations []struct {
			IntegrationID string `json:"integration_id"`
		} `json:"installations"`
	}
	if err := json.Unmarshal(body, &state); err != nil {
		return false, fmt.Errorf("decode legacy state index: %w", err)
	}
	for _, installation := range state.Installations {
		if installation.IntegrationID == name {
			return true, nil
		}
	}
	return false, nil
}

func (controller legacyController) PlanRemove(ctx context.Context, name string) (ports.LegacyRemovalPlan, error) {
	return controller.remove(ctx, name, true)
}

func (controller legacyController) Remove(ctx context.Context, name string) (ports.LegacyRemovalPlan, error) {
	return controller.remove(ctx, name, false)
}

func (controller legacyController) remove(ctx context.Context, name string, dryRun bool) (ports.LegacyRemovalPlan, error) {
	result, err := legacy.Remove(ctx, legacy.RemoveParams{Name: name, DryRun: dryRun})
	if err != nil {
		return ports.LegacyRemovalPlan{}, err
	}
	targetSet := map[string]struct{}{}
	for _, target := range result.Report.Targets {
		if target.IntegrationID == name && strings.TrimSpace(target.TargetID) != "" {
			targetSet[target.TargetID] = struct{}{}
		}
	}
	targets := make([]string, 0, len(targetSet))
	for target := range targetSet {
		targets = append(targets, target)
	}
	sort.Strings(targets)
	return ports.LegacyRemovalPlan{Summary: result.Summary, TargetIDs: targets}, nil
}
