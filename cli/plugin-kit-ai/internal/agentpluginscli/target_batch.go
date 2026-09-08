package agentpluginscli

import (
	"crypto/rand"
	"fmt"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func newOperationGroupID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("create operation group ID: %w", err)
	}
	return fmt.Sprintf("op-%x", value[:]), nil
}

type batchTargetResult struct {
	Target string `json:"target"`
	Output any    `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

func parseTargetOption(value string) ([]domain.ClientID, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}

	parts := strings.Split(trimmed, ",")
	targets := make([]domain.ClientID, 0, len(parts))
	seen := make(map[domain.ClientID]struct{}, len(parts))
	for _, part := range parts {
		raw := strings.TrimSpace(part)
		if raw == "" {
			return nil, fmt.Errorf("--target contains an empty client; use a comma-separated list such as codex,cursor")
		}
		if strings.EqualFold(raw, "all") {
			return nil, fmt.Errorf("--target all is not supported; choose clients explicitly")
		}
		if strings.EqualFold(raw, "openai") {
			return nil, fmt.Errorf("target %q is ambiguous; use codex, chatgpt, or both", raw)
		}

		target := normalizeTarget(raw)
		if target == "legacy-all" {
			if len(parts) != 1 {
				return nil, fmt.Errorf("--target legacy-all cannot be combined with Agent Plugins clients")
			}
			return []domain.ClientID{target}, nil
		}
		if !supportedTarget(target) {
			return nil, fmt.Errorf("unsupported target %q; choose %s", raw, supportedTargetHelp())
		}
		if _, duplicate := seen[target]; duplicate {
			return nil, fmt.Errorf("--target contains duplicate client %q", target)
		}
		seen[target] = struct{}{}
		targets = append(targets, target)
	}
	sortTargets(targets)
	return targets, nil
}

func sortTargets(targets []domain.ClientID) {
	sort.SliceStable(targets, func(i, j int) bool { return targetOrder(targets[i]) < targetOrder(targets[j]) })
}

func targetOrder(target domain.ClientID) int {
	for index, supported := range domain.SupportedClientIDs() {
		if target == supported {
			return index
		}
	}
	return len(domain.SupportedClientIDs())
}

func supportedTarget(target domain.ClientID) bool {
	return domain.IsSupportedClient(target)
}

func supportedTargetHelp() string {
	ids := domain.SupportedClientIDs()
	values := make([]string, len(ids))
	for index, id := range ids {
		values[index] = string(id)
	}
	return strings.Join(values, ", ")
}

func joinTargets(targets []domain.ClientID) string {
	values := make([]string, len(targets))
	for index, target := range targets {
		values[index] = string(target)
	}
	return strings.Join(values, ", ")
}
