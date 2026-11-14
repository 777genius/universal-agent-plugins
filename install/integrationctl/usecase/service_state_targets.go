package usecase

import (
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func resolveRequestedTargets(manifest domain.IntegrationManifest, requested []string) ([]domain.TargetID, error) {
	if len(requested) == 0 {
		targets := make([]domain.TargetID, 0, len(manifest.Deliveries))
		for _, delivery := range manifest.Deliveries {
			targets = append(targets, delivery.TargetID)
		}
		return targets, nil
	}
	out := make([]domain.TargetID, 0, len(requested))
	available := map[domain.TargetID]struct{}{}
	for _, delivery := range manifest.Deliveries {
		available[delivery.TargetID] = struct{}{}
	}
	for _, raw := range requested {
		target := domain.TargetID(strings.ToLower(strings.TrimSpace(raw)))
		if _, ok := available[target]; !ok {
			return nil, domain.NewError(domain.ErrUnsupportedTarget, "manifest does not expose target "+string(target), nil)
		}
		out = append(out, target)
	}
	return out, nil
}

func findDelivery(items []domain.Delivery, target domain.TargetID) *domain.Delivery {
	for i := range items {
		if items[i].TargetID == target {
			return &items[i]
		}
	}
	return nil
}

func sortedTargets(m map[domain.TargetID]domain.TargetInstallation) []domain.TargetID {
	out := make([]domain.TargetID, 0, len(m))
	for id := range m {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func targetIDsToStrings(items []domain.TargetID) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, string(item))
	}
	return out
}
