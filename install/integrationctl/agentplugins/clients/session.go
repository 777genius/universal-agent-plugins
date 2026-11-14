package clients

import (
	"context"
	"runtime"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
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

// SessionExecutionPlanner optionally retains client-specific read authority for
// a confirmed execution, including verification, replanning and the final child.
// A nil execution preserves the ordinary session path.
type SessionExecutionPlanner interface {
	BeginSessionExecution(context.Context, Env, SessionPlan) (SessionExecution, error)
}

// SessionExecution is transient, fully initialized authority, never plan data.
// Its context carries one original deadline. Runner wraps the original observer;
// the caller still invokes SessionPlanner.VerifySession. Revalidate and Close
// run on every exit; Close releases only this execution's retained resources.
type SessionExecution interface {
	Context() context.Context
	Runner() ports.CommandRunner
	Revalidate() error
	Close() error
}

// ChildEnvironment returns a copy, replacing only this plan's explicit keys.
func (plan SessionPlan) ChildEnvironment(base []string) []string {
	return plan.childEnvironment(base, runtime.GOOS == "windows")
}

// Windows treats environment keys as case-insensitive. Strip inherited aliases
// of the two explicit Cline overrides before launching the selected client.
func (plan SessionPlan) childEnvironment(base []string, caseInsensitive bool) []string {
	normalize := func(key string) string {
		if caseInsensitive {
			return strings.ToUpper(key)
		}
		return key
	}
	overrides := make(map[string]struct{}, len(plan.Environment))
	for key := range plan.Environment {
		overrides[normalize(key)] = struct{}{}
	}
	result := make([]string, 0, len(base)+len(plan.Environment))
	for _, item := range base {
		key, _, _ := strings.Cut(item, "=")
		if _, replace := overrides[normalize(key)]; !replace {
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
