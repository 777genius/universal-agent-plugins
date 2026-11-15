package hermes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"

	"gopkg.in/yaml.v3"
)

// Explicit run consent admits these three bounded native observations. They do
// not connect MCP or call a model; any native bootstrap remains in its selected
// runtime root and must pass the same qualified source boundary afterwards.
func verifySessionNativeSource(ctx context.Context, env clients.Env, plan clients.SessionPlan) (resultErr error) {
	env.SessionEnvironment = sessionEnvironment(env.SessionEnvironment)
	if err := ctx.Err(); err != nil {
		return err
	}
	if env.Runner == nil {
		return fmt.Errorf("hermes native configuration observer unavailable")
	}
	if len(plan.Arguments) != 0 {
		history, owned, err := sessionVerificationHistory(ctx, env, plan)
		if err != nil {
			return err
		}
		if owned {
			defer func() { resultErr = errors.Join(resultErr, history.Revalidate(), history.Close()) }()
		}
		ctx, env.Runner = history.Context(), history
	}
	if err := verifySessionAuthority(ctx, env, plan); err != nil {
		return err
	}
	source, err := verifiedSessionSource(ctx, plan, sessionRootObject(plan.Directory), env.SessionEnvironment)
	if err != nil {
		return err
	}
	bounded := ctx
	if len(plan.Arguments) == 0 {
		var cancel context.CancelFunc
		bounded, cancel = context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
	}
	return verifySessionMetadata(bounded, env, plan, source.body)
}

func verifySessionMetadata(ctx context.Context, env clients.Env, plan clients.SessionPlan, body []byte) error {
	bundle, err := qualifyHermesBundle(plan.Executable)
	if err != nil {
		return err
	}
	version, err := observeSessionConfig(ctx, env, plan, "--version")
	if err != nil {
		return err
	}
	if err := hermesInstallDirectory(version, filepath.Join(bundle.root, "hermes-agent")); err != nil {
		return err
	}
	path, err := observeSessionConfig(ctx, env, plan, "config", "path")
	if err != nil {
		return err
	}
	expected := filepath.Join(projectSessionRoot(plan.Directory), "config.yaml")
	if string(path) != expected+"\n" && string(path) != expected+"\r\n" {
		return fmt.Errorf("hermes selected a different native configuration path")
	}
	servers, err := observeSessionConfig(ctx, env, plan, "config", "get", "mcp_servers", "--json")
	if err != nil {
		return err
	}
	if err := compareSessionNativeSource(body, servers); err != nil {
		return err
	}
	return verifySessionAuthority(ctx, env, plan)
}

func observeSessionConfig(ctx context.Context, env clients.Env, plan clients.SessionPlan, arguments ...string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := verifySessionAuthority(ctx, env, plan); err != nil {
		return nil, err
	}
	environment := plan.ChildEnvironment(env.SessionEnvironment)
	if err := hermesSessionSignals(projectSessionRoot(plan.Directory), environment); err != nil {
		return nil, err
	}
	const limit = 1 << 20
	result, err := env.Runner.Run(ctx, legacyports.Command{
		Argv: append([]string{plan.Executable}, arguments...), Dir: plan.Directory,
		Env: environment, StdoutLimitBytes: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("hermes native configuration observation failed: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if result.ExitCode != 0 || len(result.Stdout) >= limit || len(result.Stderr) > 32768 || !result.CleanupComplete || result.NoChildStarted {
		return nil, fmt.Errorf("hermes configuration observation failed, exceeded its bound or lacked process cleanup")
	}
	if err := verifySessionAuthority(ctx, env, plan); err != nil {
		return nil, err
	}
	return result.Stdout, nil
}

func hermesInstallDirectory(body []byte, expected string) error {
	lines := strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n")
	if lines[0] != "Hermes Agent vunknown (2026.9.24)" {
		return fmt.Errorf("hermes native version differs from qualified bundle")
	}
	found := 0
	for _, line := range lines[1:] {
		if strings.HasPrefix(line, "Hermes Agent v") {
			return fmt.Errorf("hermes native version observation is ambiguous")
		}
		if value, ok := strings.CutPrefix(line, "Install directory: "); ok {
			found++
			if value != expected {
				return fmt.Errorf("hermes native install directory differs from selected bundle")
			}
		}
	}
	if found != 1 {
		return fmt.Errorf("hermes native version/install-directory observation is missing")
	}
	return nil
}

func verifySessionAuthority(ctx context.Context, env clients.Env, plan clients.SessionPlan) error {
	if env.ProjectRoots == nil {
		return fmt.Errorf("hermes selected-root verifier is unavailable")
	}
	owner := sessionRootObject(plan.Directory)
	if err := env.ProjectRoots.VerifyProjectRoot(ctx, projectSessionRoot(plan.Directory), owner); err != nil {
		return err
	}
	_, err := verifiedSessionSource(ctx, plan, owner, env.SessionEnvironment)
	return err
}

func compareSessionNativeSource(source, observed []byte) error {
	var config struct {
		Servers map[string]any `yaml:"mcp_servers"`
	}
	if err := yaml.Unmarshal(source, &config); err != nil {
		return fmt.Errorf("hermes owned YAML could not be compared")
	}
	wanted, err := json.Marshal(config.Servers)
	if err != nil {
		return err
	}
	document, err := shared.DecodeStrictJSONObject(observed)
	if err != nil {
		return fmt.Errorf("hermes native MCP observation is not one unambiguous JSON object")
	}
	actual, err := json.Marshal(document)
	if err != nil || !bytes.Equal(wanted, actual) {
		return fmt.Errorf("hermes effective MCP differs from exact owned Project source")
	}
	return nil
}

var _ clients.SessionExecutionPlanner = (*Adapter)(nil)

type sessionInspectHistory struct {
	ctx    context.Context
	cancel context.CancelFunc
	env    clients.Env
	plan   clients.SessionPlan
	state  *sessionRuntimeSnapshot
}

func (*Adapter) BeginSessionExecution(ctx context.Context, env clients.Env, plan clients.SessionPlan) (clients.SessionExecution, error) {
	if len(plan.Arguments) == 0 {
		return nil, nil
	}
	history, err := captureSessionInspectHistory(ctx, env, plan)
	if err != nil {
		return nil, err
	}
	return history, nil
}

func sessionVerificationHistory(ctx context.Context, env clients.Env, plan clients.SessionPlan) (*sessionInspectHistory, bool, error) {
	if history, ok := env.Runner.(*sessionInspectHistory); ok {
		if history == nil || ctx != history.Context() || !reflect.DeepEqual(plan, history.plan) || !slices.Equal(env.SessionEnvironment, history.env.SessionEnvironment) {
			return nil, false, fmt.Errorf("hermes inspect execution differs from original capture")
		}
		return history, false, nil
	}
	history, err := captureSessionInspectHistory(ctx, env, plan)
	return history, true, err
}

func captureSessionInspectHistory(ctx context.Context, env clients.Env, plan clients.SessionPlan) (*sessionInspectHistory, error) {
	if !slices.Equal(plan.Arguments, []string{"config", "get", "mcp_servers", "--json"}) || env.Runner == nil {
		return nil, fmt.Errorf("hermes history is admitted only for exact inspect")
	}
	bounded, cancel := context.WithTimeout(ctx, 60*time.Second)
	plan.Arguments, plan.Servers = slices.Clone(plan.Arguments), slices.Clone(plan.Servers)
	plan.Environment = maps.Clone(plan.Environment)
	env.SessionEnvironment = sessionEnvironment(env.SessionEnvironment)
	history := &sessionInspectHistory{ctx: bounded, cancel: cancel, env: env, plan: plan}
	if err := history.capture(); err != nil {
		return nil, errors.Join(err, history.Close())
	}
	return history, nil
}

func (history *sessionInspectHistory) capture() error {
	if err := verifySessionAuthority(history.ctx, history.env, history.plan); err != nil {
		return err
	}
	effective, err := sessionPlanEnvironment(history.plan, history.env.SessionEnvironment)
	if err != nil {
		return err
	}
	bundle, err := qualifyHermesBundle(history.plan.Executable)
	if err != nil {
		return err
	}
	source, state, err := acquireSessionSource(history.ctx, history.plan.Directory, sessionRootObject(history.plan.Directory), history.plan.Servers,
		sessionRuntimeSelection{true, bundle, effective})
	if err != nil {
		return err
	}
	history.state = state
	if source.digest != history.plan.ConfigDigest {
		return fmt.Errorf("hermes original planned history changed before execution")
	}
	return history.Revalidate()
}

func (history *sessionInspectHistory) Context() context.Context { return history.ctx }

func (history *sessionInspectHistory) Runner() ports.CommandRunner { return history }

func (history *sessionInspectHistory) Revalidate() error {
	if history.state == nil {
		return fmt.Errorf("hermes inspect history is closed")
	}
	if err := history.state.rejoinExecution(history.ctx); err != nil {
		return err
	}
	if err := verifySessionAuthority(history.ctx, history.env, history.plan); err != nil {
		return err
	}
	return history.state.rejoinExecution(history.ctx)
}

func (history *sessionInspectHistory) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	if ctx != history.ctx {
		return legacyports.CommandResult{}, fmt.Errorf("hermes inspect deadline differs from original capture")
	}
	if !slices.Equal(command.Env, history.plan.ChildEnvironment(history.env.SessionEnvironment)) {
		return legacyports.CommandResult{}, fmt.Errorf("hermes inspect environment differs from original capture")
	}
	if err := history.Revalidate(); err != nil {
		return legacyports.CommandResult{}, err
	}
	result, err := history.env.Runner.Run(ctx, command)
	return result, errors.Join(err, history.Revalidate())
}

func (history *sessionInspectHistory) Close() error {
	var result error
	if history.state != nil {
		result = history.state.close()
		history.state = nil
	}
	history.cancel()
	return result
}
