package opencode

import (
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func protectionForScope(scope string) domain.ProtectionClass {
	if strings.EqualFold(strings.TrimSpace(scope), "project") {
		return domain.ProtectionWorkspace
	}
	return domain.ProtectionUserMutable
}

func mapFromSlice[T any](values []T, keyFn func(T) string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		key := strings.TrimSpace(keyFn(value))
		if key != "" {
			out[key] = true
		}
	}
	return out
}

func subtractStrings(current, next []string) []string {
	nextSet := mapFromSlice(next, func(value string) string { return value })
	var out []string
	for _, item := range current {
		if !nextSet[item] {
			out = append(out, item)
		}
	}
	sort.Strings(out)
	return dedupeStrings(out)
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := values[:0]
	var last string
	for _, value := range values {
		if value == "" || value == last {
			continue
		}
		out = append(out, value)
		last = value
	}
	return out
}
