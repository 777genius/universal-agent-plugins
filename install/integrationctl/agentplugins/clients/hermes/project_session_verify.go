package hermes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
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
	body, err := projectSessionConfig(ctx, plan.Directory, sessionRootObject(plan.Directory), plan.Servers, len(plan.Arguments) != 0)
	if err != nil {
		return err
	}
	bounded := ctx
	if len(plan.Arguments) == 0 {
		var cancel context.CancelFunc
		bounded, cancel = context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
	}
	return verifySessionMetadata(bounded, env, plan, body)
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
	environment := plan.ChildEnvironment(os.Environ())
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
	return verifyProjectSessionSource(ctx, plan, owner)
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

type sessionInspectMember struct {
	name string
	file *os.File
	info os.FileInfo
	body []byte
}

type sessionInspectHistory struct {
	ctx      context.Context
	cancel   context.CancelFunc
	env      clients.Env
	plan     clients.SessionPlan
	root     *os.Root
	rootInfo os.FileInfo
	logs     *os.Root
	logsInfo os.FileInfo
	members  []sessionInspectMember
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
		if history == nil || ctx != history.Context() || !reflect.DeepEqual(plan, history.plan) {
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
	var err error
	history.rootInfo, err = noAliasDirectory(projectSessionRoot(history.plan.Directory))
	if err != nil {
		return err
	}
	history.root, err = os.OpenRoot(projectSessionRoot(history.plan.Directory))
	if err != nil {
		return err
	}
	history.logsInfo, err = history.root.Lstat("logs")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil {
		history.logs, err = history.root.OpenRoot("logs")
		if err != nil {
			return err
		}
	}
	names := []string{"spawn-ledger.json", "spawn-ledger.json.lock", "logs/mcp-stderr.log"}
	if runtime.GOOS == "windows" {
		names = append(names, "logs/.__agent.lock")
	}
	for _, name := range names {
		if err := history.captureMember(name); err != nil {
			return err
		}
	}
	return history.Revalidate()
}

func (history *sessionInspectHistory) captureMember(name string) error {
	if _, err := history.parents(); err != nil {
		return err
	}
	history.members = append(history.members, sessionInspectMember{name: name})
	member := &history.members[len(history.members)-1]
	info, err := history.root.Lstat(filepath.FromSlash(name))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || !bootstrapMemberMetadata(name, info, true) {
		return fmt.Errorf("hermes inspect history metadata differs")
	}
	member.info = info
	parent, leaf := history.root, name
	if strings.HasPrefix(name, "logs/") {
		parent, leaf = history.logs, filepath.Base(name)
		if parent == nil {
			return fmt.Errorf("hermes inspect logs appeared during capture")
		}
	}
	member.file, err = openSessionFile(parent, leaf)
	if err != nil {
		return err
	}
	member.body, err = history.memberBody(*member)
	return err
}

func (history *sessionInspectHistory) Context() context.Context { return history.ctx }

func (history *sessionInspectHistory) Runner() ports.CommandRunner { return history }

func (history *sessionInspectHistory) parents() ([2]os.FileInfo, error) {
	var result [2]os.FileInfo
	if err := history.ctx.Err(); err != nil {
		return result, err
	}
	if history.root == nil {
		return result, fmt.Errorf("hermes inspect history is closed")
	}
	named, err := noAliasDirectory(projectSessionRoot(history.plan.Directory))
	if err != nil || !os.SameFile(history.rootInfo, named) || history.rootInfo.Mode() != named.Mode() {
		return result, fmt.Errorf("hermes inspect original root changed")
	}
	for index, item := range []struct {
		root *os.Root
		info os.FileInfo
		name string
	}{{history.root, history.rootInfo, "."}, {history.logs, history.logsInfo, "logs"}} {
		if item.root == nil {
			continue
		}
		held, statErr := item.root.Stat(".")
		current, nameErr := history.root.Lstat(item.name)
		if statErr != nil || nameErr != nil || !os.SameFile(item.info, held) || !os.SameFile(held, current) ||
			item.info.Mode() != held.Mode() || !sameSessionMember(held, current) {
			return result, fmt.Errorf("hermes inspect history directory changed")
		}
		if err := validateSessionRootHandle(item.root); err != nil {
			return result, err
		}
		result[index] = held
	}
	return result, nil
}

func (history *sessionInspectHistory) memberBody(member sessionInspectMember) ([]byte, error) {
	parents, err := history.parents()
	if err != nil {
		return nil, err
	}
	held, err := member.file.Stat()
	named, nameErr := history.root.Lstat(filepath.FromSlash(member.name))
	if err != nil || nameErr != nil || !sameSessionMember(member.info, held) || !sameSessionMember(held, named) {
		return nil, fmt.Errorf("hermes inspect history identity changed")
	}
	if err := validateSessionOwnedHandle(member.file, held); err != nil {
		return nil, err
	}
	body, readErr := io.ReadAll(io.NewSectionReader(member.file, 0, member.info.Size()+1))
	after, statErr := member.file.Stat()
	named, nameErr = history.root.Lstat(filepath.FromSlash(member.name))
	finalParents, parentErr := history.parents()
	if readErr != nil || statErr != nil || nameErr != nil || parentErr != nil || int64(len(body)) != member.info.Size() ||
		!sameSessionMember(member.info, after) || !sameSessionMember(after, named) {
		return nil, fmt.Errorf("hermes inspect history changed during read")
	}
	for index, before := range parents {
		if before != nil && !sameSessionMember(before, finalParents[index]) {
			return nil, fmt.Errorf("hermes inspect history parent changed during read")
		}
	}
	return body, errors.Join(history.ctx.Err(), validateSessionOwnedHandle(member.file, after))
}

func (history *sessionInspectHistory) Revalidate() error {
	if _, err := history.parents(); err != nil {
		return err
	}
	if err := verifySessionAuthority(history.ctx, history.env, history.plan); err != nil {
		return err
	}
	for _, member := range history.members {
		if member.info == nil {
			if _, err := history.root.Lstat(filepath.FromSlash(member.name)); !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("hermes inspect history appeared after original absence")
			}
			continue
		}
		body, err := history.memberBody(member)
		if err != nil || !bytes.Equal(member.body, body) {
			return errors.Join(fmt.Errorf("hermes inspect history changed"), err)
		}
	}
	return history.ctx.Err()
}

func (history *sessionInspectHistory) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	if ctx != history.ctx {
		return legacyports.CommandResult{}, fmt.Errorf("hermes inspect deadline differs from original capture")
	}
	if err := history.Revalidate(); err != nil {
		return legacyports.CommandResult{}, err
	}
	result, err := history.env.Runner.Run(ctx, command)
	return result, errors.Join(err, history.Revalidate())
}

func (history *sessionInspectHistory) Close() error {
	var result error
	for index := range history.members {
		member := &history.members[index]
		if member.file != nil {
			result = errors.Join(result, member.file.Close())
			member.file = nil
		}
	}
	for _, directory := range []*os.Root{history.logs, history.root} {
		if directory != nil {
			result = errors.Join(result, directory.Close())
		}
	}
	history.logs, history.root = nil, nil
	history.cancel()
	return result
}
