package ports

import "context"

type LegacyRemovalPlan struct {
	Summary   string   `json:"summary"`
	TargetIDs []string `json:"target_ids,omitempty"`
}

// LegacyLifecycle keeps migrated plugin.yaml installations removable through
// their original engine. It never parses or converts plugin.yaml as a standard
// Agent Plugin.
type LegacyLifecycle interface {
	Exists(context.Context, string) (bool, error)
	PlanRemove(context.Context, string) (LegacyRemovalPlan, error)
	Remove(context.Context, string) (LegacyRemovalPlan, error)
}
