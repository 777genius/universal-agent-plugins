package clients

import (
	"context"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// SessionRequest describes an explicit, single-process start, not installation
// or a persistent trust grant. Bindings have been reconciled with their packages.
type SessionRequest struct {
	Client      domain.DetectedClient
	ProjectRoot string
	Bindings    []domain.ClientBinding
	Inspect     bool
}

// SessionPlan contains only a selected executable and nonsecret configuration
// overrides. The ordinary user environment is inherited by the process boundary.
// It is transient: no activation/authentication receipt may be derived from it.
type SessionPlan struct {
	ClientID     domain.ClientID
	Executable   string
	Arguments    []string
	Directory    string
	Environment  map[string]string
	ConfigDigest string
	Servers      []string
}

// SessionPlanner is opt-in: unsupported clients do not gain a command through
// fallback behavior. In particular, other IDE/Web surfaces stay unchanged.
type SessionPlanner interface {
	PlanSession(context.Context, Env, SessionRequest) (SessionPlan, error)
	VerifySession(context.Context, Env, SessionPlan) error
}

// ChildEnvironment returns a copy, replacing only this plan's explicit keys.
func (plan SessionPlan) ChildEnvironment(base []string) []string {
	result := make([]string, 0, len(base)+len(plan.Environment))
	for _, item := range base {
		key, _, _ := strings.Cut(item, "=")
		if _, replace := plan.Environment[key]; !replace {
			result = append(result, item)
		}
	}
	keys := make([]string, 0, len(plan.Environment))
	for key := range plan.Environment {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		result = append(result, key+"="+plan.Environment[key])
	}
	return result
}
