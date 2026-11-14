package domain

import "sort"

// SelectedMCPNames is the deterministic delivery set, after static support and
// local readiness decisions. Consumers must not infer delivery from the envelope.
func SelectedMCPNames(plan DeliveryPlan) []string {
	names := []string{}
	seen := map[string]bool{}
	for _, c := range plan.Components {
		if c.Kind == ComponentMCPServer && c.Support != SupportUnsupported && !seen[c.Name] {
			names = append(names, c.Name)
			seen[c.Name] = true
		}
	}
	sort.Strings(names)
	return names
}

// ComponentReadinessError describes an expected local delivery limitation.
// Unexpected ownership, filesystem, and consistency errors remain fatal.
type ComponentReadinessError struct{ Code, Message string }

func (e *ComponentReadinessError) Error() string { return e.Message }
