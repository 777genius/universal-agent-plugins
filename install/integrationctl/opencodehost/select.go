package opencodehost

import (
	"errors"
	"slices"
	"sort"
)

var (
	ErrInvalidRequirement    = errors.New("invalid_requirement")
	ErrUnverifiedCapability  = errors.New("unverified_capability")
	ErrUnsupportedCapability = errors.New("unsupported_capability")
	ErrNoAdapter             = errors.New("no_adapter")
)

type ArtifactRequirement struct {
	ID       string
	Adapter  AdapterID
	Required []Capability
}
type Selection struct {
	ArtifactID          string
	Adapter             AdapterID
	Missing, Unverified []Capability
}

func (s Selection) Clone() Selection {
	s.Missing = slices.Clone(s.Missing)
	s.Unverified = slices.Clone(s.Unverified)
	return s
}

// Select validates every candidate before choosing. Failure returns diagnostics
// only: ArtifactID and Adapter are empty, so it cannot be used as a codec choice.
func Select(p Profile, requirements []ArtifactRequirement) (Selection, error) {
	candidates := slices.Clone(requirements)
	ids := map[string]bool{}
	for _, r := range candidates {
		if r.ID == "" || ids[r.ID] || !validRequirement(r) {
			return Selection{}, ErrInvalidRequirement
		}
		ids[r.ID] = true
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].ID == candidates[j].ID {
			return candidates[i].Adapter < candidates[j].Adapter
		}
		return candidates[i].ID < candidates[j].ID
	})
	var best *Selection
	for _, r := range candidates {
		if !eligible(p, r.Adapter) {
			continue
		}
		s := Selection{ArtifactID: r.ID, Adapter: r.Adapter}
		for _, c := range r.Required {
			switch p.Capabilities[c] {
			case Supported:
			case Unverified:
				s.Unverified = append(s.Unverified, c)
			default:
				s.Missing = append(s.Missing, c)
			}
		}
		slices.Sort(s.Missing)
		slices.Sort(s.Unverified)
		if len(s.Missing) == 0 && len(s.Unverified) == 0 {
			return s, nil
		}
		if best == nil {
			best = &s
		}
	}
	if best == nil {
		return Selection{}, ErrNoAdapter
	}
	out := Selection{Missing: best.Missing, Unverified: best.Unverified}
	if len(out.Missing) > 0 {
		return out, ErrUnsupportedCapability
	}
	return out, ErrUnverifiedCapability
}

func eligible(p Profile, a AdapterID) bool {
	switch a {
	case DualPlacement:
		return true // owned bytes only, independent of runtime
	case SkillDirectory:
		return p.Family == ModernV1 || p.Family == LegacyV1 || p.Family == V2
	case ConfigV1:
		return p.ConfigDialect == DialectV1
	case ConfigV2:
		return p.ConfigDialect == DialectV2
	case ObserverV1:
		return p.Family == ModernV1 && p.Qualification == "tested_exact"
	case ObserverV2:
		return p.Family == V2 && p.Qualification == "tested_exact"
	}
	return false
}

func validRequirement(r ArtifactRequirement) bool {
	seen := map[Capability]bool{}
	for _, c := range r.Required {
		if seen[c] || !allowed(r.Adapter, c) {
			return false
		}
		seen[c] = true
	}
	switch r.Adapter {
	case DualPlacement:
		return len(seen) == 0
	case SkillDirectory:
		return seen[GlobalSkillDirectory] && len(seen) == 1
	case ConfigV1, ConfigV2:
		return len(seen) > 0
	case ObserverV1, ObserverV2:
		return seen[LocalPluginDual] && len(seen) > 1
	}
	return false
}
func allowed(a AdapterID, c Capability) bool {
	switch a {
	case SkillDirectory:
		return c == GlobalSkillDirectory
	case ConfigV1, ConfigV2:
		return c == MCPStdio || c == MCPStreamableHTTP
	case ObserverV1, ObserverV2:
		return c == LocalPluginDual || c == ObserverCompletion || c == ObserverQuestion || c == ObserverPermission || c == ObserverTerminalError
	}
	return false
}
