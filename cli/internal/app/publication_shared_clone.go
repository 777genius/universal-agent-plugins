package app

import "strings"

func clonePublishResults(items []PluginPublishResult) []PluginPublishResult {
	if len(items) == 0 {
		return []PluginPublishResult{}
	}
	out := make([]PluginPublishResult, 0, len(items))
	for _, item := range items {
		cloned := item
		cloned.Details = cloneStringMap(item.Details)
		cloned.Issues = append([]PluginPublishIssue(nil), item.Issues...)
		cloned.Warnings = cloneStrings(item.Warnings)
		cloned.NextSteps = cloneStrings(item.NextSteps)
		cloned.Channels = clonePublishResults(item.Channels)
		cloned.Lines = nil
		out = append(out, cloned)
	}
	return out
}

func cloneStrings(items []string) []string {
	if len(items) == 0 {
		return []string{}
	}
	return append([]string(nil), items...)
}

func cloneStringMap(items map[string]string) map[string]string {
	if len(items) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(items))
	for key, value := range items {
		out[key] = value
	}
	return out
}

func appendUniquePublishSteps(steps []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(steps))
	for _, step := range steps {
		step = strings.TrimSpace(step)
		if step == "" {
			continue
		}
		if _, ok := seen[step]; ok {
			continue
		}
		seen[step] = struct{}{}
		out = append(out, step)
	}
	return out
}
