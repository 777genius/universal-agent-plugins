package kimi

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func validateKimiActive(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, data string) error {
	if _, err := kimiPlanRoot(domain.DetectedClient{ConfigRoot: plan.TargetContext.UserConfigRoot}, plan); err != nil {
		return err
	}
	if root != plan.ActivePath {
		return fmt.Errorf("kimi active package differs from selected plan")
	}
	if err := plan.SelectedDelivery.ValidatePlan(plan, envelope.TreeDigest); err != nil {
		return err
	}
	for _, name := range shared.SupportedMCPNames(plan) {
		if envelope.MCP.Servers[name].Type == "stdio" && (!filepath.IsAbs(data) || filepath.Clean(data) != data) {
			return fmt.Errorf("%w: kimi selected stdio data root must be absolute and clean", domain.ErrProjectionRefreshRequired)
		}
		expected, err := renderKimiProjectServer(envelope.MCP.Servers[name], plan.ActivePath, data, root)
		if err != nil {
			return err
		}
		actual, err := readKimiProjectEntry(root, name)
		if err != nil {
			return err
		}
		// JSON normalization equates []string projected in memory with decoded []any.
		body, err := json.Marshal(expected)
		if err != nil {
			return err
		}
		var normalized map[string]any
		if err := json.Unmarshal(body, &normalized); err != nil {
			return err
		}
		if !reflect.DeepEqual(actual, normalized) {
			return fmt.Errorf("%w: kimi immutable package/data/launcher projection differs", domain.ErrProjectionRefreshRequired)
		}
	}
	return nil
}
