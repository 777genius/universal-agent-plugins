package installer

import (
	"context"
	"fmt"
	"path/filepath"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// Config is copied by New. Later mutation of the caller's value is ignored.
type Config struct {
	temporaryRoot bool
	ProjectRoot   string `json:"-"`
	// OpenCodeProbe defaults to the explicit-target facade. New captures and pins
	// the allowlisted environment once; it never executes a probe.
	OpenCodeProbe            OpenCodeProbe
	OpenCodeProbeEnvironment []string
	// StateRoot is the owned UAP namespace. It is required and must be an
	// absolute clean path. New does not create it.
	StateRoot                                                                 string
	StateFile, LockFile, OperationsDir, PluginDataBase, ManagedRoot, TempRoot string
	HelperExecutable, HelperVersion                                           string
	// Registry is the explicit set of client adapters supported by this
	// executable. Nil is rejected rather than silently enabling every client.
	Registry *clients.Registry
	// EnableNativeObserver enables live client discovery for repair and recovery
	// when Runner is supplied. Project filesystem ownership checks remain
	// mandatory when it is disabled or no runner is available. User scope keeps
	// its existing filesystem-only compatibility path when disabled.
	EnableNativeObserver bool
	Runner               ports.CommandRunner
	// ServerName selects the declared MCP server whose args the host may replace.
	ServerName string
	// ProjectArgs replaces args of ServerName. It is host-owned and must be
	// deterministic. A missing declared server or a callback error fails staging.
	ProjectArgs func(BindingFacts) ([]string, error)
	// OnCommittedBinding runs after the package commit and before activation.
	// Failed handoffs can be retried for the same binding and digest. Hosts must
	// make effects idempotent using those identities, not the attempt OperationID.
	OnCommittedBinding func(context.Context, BindingFacts) error
	// Assess is optional and digest-bound. The constructor does not start a
	// download scanner. When set, block and unavailable never become allow.
	Assess func(context.Context, string, string) (Assessment, error)
	// TrustedLocalPackages is an explicit policy for bundled or otherwise
	// pre-authorized local bytes. When false, Prepare requires either Assess or
	// a digest-bound Request.Assessment; nil is never an implicit allow.
	TrustedLocalPackages bool
	// Progress reports coarse phases and group client checkpoints. It must not return an error, prompt, or
	// start a nested installer.
	Progress func(ProgressEvent)
	// IndependentActivations allows distinct Claude/Codex user CLI effects to run
	// together after serial state and host preparation. Other clients stay serial.
	// Runner must support simultaneous commands for distinct clients.
	IndependentActivations bool
	// ClientExecutables are optional explicit client paths Discover Lstats
	// without executing. Empty entries fall back to PATH presence of the
	// well-known binary name. New copies the map.
	ClientExecutables map[string]string
}

func (c Config) resolved() (Config, error) {
	out := c
	if out.ProjectRoot != "" {
		if _, err := domain.NewTargetContext(domain.ScopeProject, out.ProjectRoot, ""); err != nil {
			return Config{}, fmt.Errorf("%w: %w", ErrInvalidConfig, err)
		}
	}
	if !validRoot(out.StateRoot) {
		return Config{}, fmt.Errorf("%w: StateRoot must be an explicit absolute clean path", ErrInvalidConfig)
	}
	if out.Registry == nil {
		return Config{}, fmt.Errorf("%w: client Registry is required", ErrInvalidConfig)
	}
	if out.StateFile == "" {
		out.StateFile = filepath.Join(out.StateRoot, "state-v2.json")
	}
	if out.LockFile == "" {
		out.LockFile = filepath.Join(out.StateRoot, "mutation.lock")
	}
	if out.OperationsDir == "" {
		out.OperationsDir = filepath.Join(out.StateRoot, "operations")
	}
	if out.PluginDataBase == "" {
		out.PluginDataBase = filepath.Join(out.StateRoot, "plugin-data")
	}
	if out.ManagedRoot == "" {
		out.ManagedRoot = filepath.Join(out.StateRoot, "managed")
	}
	if out.TempRoot == "" {
		out.TempRoot = filepath.Join(out.StateRoot, "tmp")
	}
	if out.HelperVersion == "" {
		out.HelperVersion = "uap-installer-helper-v1"
	}
	for _, p := range []string{out.StateFile, out.LockFile, out.OperationsDir, out.PluginDataBase, out.ManagedRoot, out.TempRoot} {
		if !validRoot(p) {
			return Config{}, fmt.Errorf("%w: derived or explicit path must be absolute and clean", ErrInvalidConfig)
		}
	}
	if out.HelperExecutable != "" && !validRoot(out.HelperExecutable) {
		return Config{}, fmt.Errorf("%w: HelperExecutable must be an explicit absolute clean path", ErrInvalidConfig)
	}
	if out.ClientExecutables != nil {
		cloned := make(map[string]string, len(out.ClientExecutables))
		for id, p := range out.ClientExecutables {
			if p != "" && !validRoot(p) {
				return Config{}, fmt.Errorf("%w: ClientExecutables[%s] must be an explicit absolute clean path", ErrInvalidConfig, id)
			}
			cloned[id] = p
		}
		out.ClientExecutables = cloned
	}
	return out.resolvedOpenCodeProbe()
}

func (c Config) resolvedOpenCodeProbe() (Config, error) {
	if c.OpenCodeProbe == nil {
		c.OpenCodeProbe = clientdetect.ProbeOpenCodeTarget
	}
	env := c.OpenCodeProbeEnvironment
	if env == nil {
		env = clientdetect.OpenCodeProbeEnvironment()
	}
	var err error
	c.OpenCodeProbeEnvironment, err = clientdetect.CopyOpenCodeProbeEnvironment(env)
	if err != nil {
		return Config{}, fmt.Errorf("%w: host_target_invalid", ErrInvalidConfig)
	}
	return c, nil
}
