package prompt

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// SkippedClientsHeading introduces clients that cannot be selected.
const SkippedClientsHeading = "Not available for automatic install"

// SkipKind classifies why a client is not offered. The kind decides the default
// explanation, how entries are grouped and in which order groups are shown.
type SkipKind int

const (
	// SkipOther carries a client-specific reason supplied by the planner.
	SkipOther SkipKind = iota
	// SkipManual needs a manual step or check before the client can be used.
	SkipManual
	// SkipNotSaveable cannot be recorded in the dependency list.
	SkipNotSaveable
	// SkipNoRelease has no compatible release of the selected package.
	SkipNoRelease
	// SkipNotDetected is not found on this computer.
	SkipNotDetected
)

// TargetPlaceholder in a reason is replaced with the public --target name of a
// single client, or with "<name>" when the explanation is shared by several.
const TargetPlaceholder = "{target}"

const (
	reasonNotDetected  = "not found on this computer; install or configure it first"
	reasonNotSaveable  = "cannot be recorded in the dependency list; install it separately with --no-save"
	reasonNoRelease    = "the catalog has no compatible release of this package; check one with --target " + TargetPlaceholder
	unnamedTargetLabel = "<name>"
)

// SkippedTarget is one client that cannot be selected.
type SkippedTarget struct {
	ID    domain.ClientID
	Label string
	// Target is the public --target name; the client ID is used when empty.
	Target string
	Kind   SkipKind
	// Reason overrides the default explanation for Kind.
	Reason string
}

// UnavailableGroup is one explanation shared by one or more clients.
type UnavailableGroup struct {
	Names  []string
	Detail string
}

type groupKey struct {
	kind   SkipKind
	reason string
}

// GroupSkipped sanitizes, deduplicates and groups skipped clients by kind and
// reason. Group order is fixed (specific problems first, missing clients last);
// names keep the caller's order. Entries without a label are ignored.
func GroupSkipped(skipped []SkippedTarget) []UnavailableGroup {
	order := []SkipKind{SkipOther, SkipManual, SkipNotSaveable, SkipNoRelease, SkipNotDetected}
	type group struct {
		names   []string
		targets []string
		seen    map[string]bool
	}
	groups := map[groupKey]*group{}
	var keys []groupKey
	for _, item := range skipped {
		label := SafeText(strings.TrimSpace(item.Label))
		if label == "" {
			continue
		}
		reason := SafeText(strings.TrimSpace(item.Reason))
		if reason == "" {
			reason = defaultReason(item.Kind)
		}
		if reason == "" {
			continue
		}
		key := groupKey{item.Kind, reason}
		g := groups[key]
		if g == nil {
			g = &group{seen: map[string]bool{}}
			groups[key] = g
			keys = append(keys, key)
		}
		if g.seen[label] {
			continue
		}
		g.seen[label] = true
		g.names = append(g.names, label)
		target := SafeText(item.Target)
		if target == "" {
			target = SafeText(string(item.ID))
		}
		g.targets = append(g.targets, target)
	}
	var result []UnavailableGroup
	for _, kind := range order {
		for _, key := range keys {
			if key.kind != kind {
				continue
			}
			g := groups[key]
			target := unnamedTargetLabel
			if len(g.targets) == 1 && g.targets[0] != "" {
				target = g.targets[0]
			}
			result = append(result, UnavailableGroup{Names: append([]string(nil), g.names...), Detail: strings.ReplaceAll(key.reason, TargetPlaceholder, target)})
		}
	}
	return result
}

func defaultReason(kind SkipKind) string {
	switch kind {
	case SkipNotDetected:
		return reasonNotDetected
	case SkipNotSaveable:
		return reasonNotSaveable
	case SkipNoRelease:
		return reasonNoRelease
	}
	return ""
}

// SkippedClientsNotice renders the grouped explanations as plain text for
// commands that show them without a selection screen.
func SkippedClientsNotice(skipped []SkippedTarget) string {
	groups := GroupSkipped(skipped)
	if len(groups) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(SkippedClientsHeading)
	b.WriteString(":\n")
	for _, g := range groups {
		b.WriteString("  ")
		b.WriteString(strings.Join(g.Names, ", "))
		b.WriteString(": ")
		b.WriteString(g.Detail)
		b.WriteString("\n")
	}
	return b.String()
}
