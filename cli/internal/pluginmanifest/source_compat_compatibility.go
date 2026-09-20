package pluginmanifest

import (
	"fmt"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/sdk/platformmeta"
)

func buildSourceCompatibility(graph PackageGraph, originTargets []string, target string, droppedKinds []string) ([]SourceCompatibility, error) {
	selectedTargets, err := compatibilityTargets(target)
	if err != nil {
		return nil, err
	}
	portableKinds := sourcePortableKinds(graph)
	droppedKinds = uniqueSortedKinds(droppedKinds)
	allOriginNativeKinds := uniqueSortedKinds(append(sourceNativeKinds(graph, originTargets), droppedKinds...))
	originSet := setOf(originTargets)
	out := make([]SourceCompatibility, 0, len(selectedTargets))
	for _, candidate := range selectedTargets {
		profile, ok := platformmeta.Lookup(candidate)
		if !ok {
			return nil, fmt.Errorf("unsupported target %q", candidate)
		}
		supported := intersectKinds(portableKinds, profile.Contract.PortableComponentKinds)
		unsupported := differenceKinds(portableKinds, profile.Contract.PortableComponentKinds)
		if originSet[candidate] {
			supported = append(supported, sourceNativeKinds(graph, []string{candidate})...)
			unsupported = append(unsupported, droppedKinds...)
		} else {
			unsupported = append(unsupported, allOriginNativeKinds...)
		}
		supported = uniqueSortedKinds(supported)
		unsupported = uniqueSortedKinds(unsupported)
		status := CompatibilityFull
		switch {
		case len(unsupported) == 0:
			status = CompatibilityFull
		case len(supported) > 0:
			status = CompatibilityPartial
		case len(portableKinds) == 0 && len(allOriginNativeKinds) == 0:
			status = CompatibilityFull
		default:
			status = CompatibilityUnsupported
		}
		notes := compatibilityNotes(originTargets, candidate, status, supported, unsupported, droppedKinds)
		out = append(out, SourceCompatibility{
			Target:           candidate,
			Status:           status,
			SupportedKinds:   supported,
			UnsupportedKinds: unsupported,
			Notes:            notes,
		})
	}
	return out, nil
}

func compatibilityNotes(originTargets []string, candidate string, status CompatibilityStatus, supported, unsupported, droppedKinds []string) []string {
	var notes []string
	originTargets = uniqueSortedKinds(originTargets)
	if !slices.Contains(originTargets, candidate) && len(unsupported) > 0 {
		notes = append(notes, fmt.Sprintf("target-native surfaces from %s do not project to %s without an overlay", strings.Join(originTargets, ","), candidate))
	}
	if len(droppedKinds) > 0 {
		notes = append(notes, fmt.Sprintf("source normalization dropped unsupported canonical surfaces: %s", strings.Join(droppedKinds, ",")))
	}
	switch status {
	case CompatibilityPartial:
		notes = append(notes, "installable with degraded surface coverage")
	case CompatibilityUnsupported:
		if len(supported) == 0 && len(unsupported) > 0 {
			notes = append(notes, "no functional source surfaces map to this target")
		}
	}
	return notes
}

func compatibilityTargets(target string) ([]string, error) {
	target = normalizeTarget(target)
	switch target {
	case "", "all":
		return platformmeta.IDs(), nil
	default:
		if _, ok := platformmeta.Lookup(target); !ok {
			return nil, fmt.Errorf("unsupported target %q", target)
		}
		return []string{target}, nil
	}
}

func sourcePortableKinds(graph PackageGraph) []string {
	var kinds []string
	if len(graph.Portable.Paths("skills")) > 0 {
		kinds = append(kinds, "skills")
	}
	if graph.Portable.MCP != nil {
		kinds = append(kinds, "mcp_servers")
	}
	return uniqueSortedKinds(kinds)
}

func sourceNativeKinds(graph PackageGraph, targets []string) []string {
	var kinds []string
	for _, target := range targets {
		state, ok := graph.Targets[target]
		if !ok {
			continue
		}
		kinds = append(kinds, discoveredTargetKinds(state)...)
	}
	return uniqueSortedKinds(kinds)
}

func intersectKinds(actual, allowed []string) []string {
	allowedSet := setOf(allowed)
	var out []string
	for _, item := range actual {
		if allowedSet[item] {
			out = append(out, item)
		}
	}
	return uniqueSortedKinds(out)
}

func differenceKinds(actual, allowed []string) []string {
	allowedSet := setOf(allowed)
	var out []string
	for _, item := range actual {
		if !allowedSet[item] {
			out = append(out, item)
		}
	}
	return uniqueSortedKinds(out)
}

func uniqueSortedKinds(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	slices.Sort(out)
	return slices.Compact(out)
}
