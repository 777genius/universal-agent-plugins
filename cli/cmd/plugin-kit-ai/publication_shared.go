package main

import "strings"

func appendUniqueStrings(base []string, extra ...string) []string {
	seen := make(map[string]struct{}, len(base))
	out := append([]string(nil), base...)
	for _, item := range out {
		seen[item] = struct{}{}
	}
	for _, item := range extra {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
