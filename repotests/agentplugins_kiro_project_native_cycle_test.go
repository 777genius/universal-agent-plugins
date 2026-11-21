//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

const kiroCycleMode = "kiro-authenticated-project-mcp-cycle"

var kiroCycleOrder = []string{
	"chat-help", "agent-help", "add-b", "validate-a", "validate-b",
	"native-write-a", "native-write-b", "repair-a", "native-restart-a",
	"remove-a", "native-removed-a", "native-remaining-b", "remove-b", "native-removed-b",
}

type kiroNativeCycle struct {
	f         *projectNativeCLIAttempt
	roles     map[string]*kiroCycleRole
	pins      map[string]kiroCycleFile
	roots     map[string]os.FileInfo
	steps     []string
	proofs    []map[string]any
	events    []byte
	eventInfo os.FileInfo
	sessions  map[string]bool
	complete  bool
}

func kiroAuthenticatedNativeCycle(t *testing.T) {
	spec, err := projectNativeCLISpecification("kiro")
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv(spec.optin) != "1" {
		t.Skip("explicit authenticated Kiro native lifecycle")
	}
	f := newProjectNativeCLIAttempt(t, spec)
	c := &kiroNativeCycle{f: f, roles: map[string]*kiroCycleRole{}, pins: map[string]kiroCycleFile{}, roots: map[string]os.FileInfo{}, sessions: map[string]bool{}, steps: []string{}, proofs: []map[string]any{}}
	defer c.finish(t)
	f.evidence["capture_mode"] = kiroCycleMode
	if f.kiroAuth.key == "" {
		t.Fatal("dedicated Kiro API key is required for this authenticated lane")
	}
	f.setup(t)
	c.start(t)
	c.help(t)
	c.public(t, "add-b")
	c.observe(t, "b", "add-b", false)
	c.authorAgents(t)
	c.run(t, "validate-a")
	c.run(t, "validate-b")
	c.native(t, "native-write-a", "a", "write")
	c.native(t, "native-write-b", "b", "write")
	c.missingSkill(t)
	c.public(t, "repair-a")
	c.observe(t, "a", "repair-a", false)
	c.native(t, "native-restart-a", "a", "read")
	c.public(t, "remove-a")
	c.observe(t, "a", "remove-a", true)
	c.native(t, "native-removed-a", "a", "")
	c.native(t, "native-remaining-b", "b", "read")
	c.public(t, "remove-b")
	c.observe(t, "b", "remove-b", true)
	c.native(t, "native-removed-b", "b", "")
	c.check(t)
	c.complete = reflect.DeepEqual(c.steps, kiroCycleOrder) && len(c.sessions) == 4 && len(c.proofs) == 5 && c.roles["a"].removed && c.roles["b"].removed
	if !c.complete {
		t.Fatal("Kiro complete lifecycle closure is unproved")
	}
}

func (c *kiroNativeCycle) finish(t *testing.T) {
	f := c.f
	complete := c.complete && !t.Failed() && f.ctx.Err() == nil && time.Since(f.started) < 150*time.Second
	f.evidence["runtime_attempt_complete"] = complete
	f.evidence["vendor_authentication_qualified"], f.evidence["real_model_qualified"] = complete, complete
	f.evidence["qualification"] = map[string]bool{"project-lifecycle": complete, "native-restart-removal": complete, "mcp-initialize": complete, "mcp-tools-list": complete, "mcp-tools-call": complete}
	f.evidence["kiro_cycle"] = map[string]any{"steps": c.steps, "complete": complete, "positive_sessions": len(c.sessions), "nonce": f.base.Nonce, "root": f.base.Root, "projects": map[string]string{"a": f.base.Project, "b": f.other}, "home": f.base.Home, "source": f.base.Package, "name": f.name}
	raw, err := json.Marshal(c.proofs)
	if err != nil {
		t.Error(err)
	} else {
		f.keep(t, "kiro-cycle-proof.json", raw)
	}
	f.finish(t)
}

func kiroCycleAgent(name, server string) map[string]any {
	selector := "@" + server + "/inspect_runtime"
	return map[string]any{
		"name": name, "description": "Disposable UAP native MCP fixture",
		"prompt": "Use only the named disposable MCP inspect_runtime tool, exactly as the user requests. Never run shell, file, web, or subagent tools.",
		"tools":  []string{selector}, "allowedTools": []string{selector},
		"mcpServers": map[string]any{}, "resources": []string{},
		"hooks": map[string]any{}, "toolAliases": map[string]string{},
	}
}

func (c *kiroNativeCycle) authorAgents(t *testing.T) {
	t.Helper()
	for _, key := range []string{"a", "b"} {
		r := c.roles[key]
		path := filepath.Join(r.project, ".kiro", "agents", kiroCycleAgentName(key)+".json")
		raw, err := json.Marshal(kiroCycleAgent(kiroCycleAgentName(key), c.f.name))
		if err != nil {
			t.Fatal(err)
		}
		projectNativeRuntimeWrite(t, path, append(raw, '\n'), 0600)
		c.pin(t, path)
		c.f.keep(t, "kiro-agent-"+key+".json", append(raw, '\n'))
	}
	c.check(t)
}

func kiroCycleAgentName(key string) string {
	return "uap-kiro-native-" + key
}

func (c *kiroNativeCycle) args(label string) (string, []string, bool, error) {
	key := "a"
	if strings.HasSuffix(label, "-b") {
		key = "b"
	}
	r := c.roles[key]
	if r == nil {
		return "", nil, false, fmt.Errorf("Kiro selected role is absent")
	}
	switch label {
	case "chat-help":
		return c.f.outside, []string{c.f.client, "chat", "--help"}, false, nil
	case "agent-help":
		return c.f.outside, []string{c.f.client, "agent", "validate", "--help"}, false, nil
	case "validate-a", "validate-b":
		path := filepath.Join(r.project, ".kiro", "agents", kiroCycleAgentName(key)+".json")
		return r.project, []string{c.f.client, "agent", "validate", "--path", path}, true, nil
	case "add-b":
		return r.project, []string{c.f.installer, "add", c.f.base.Package, "--no-save", "--scope", "project", "--project-root", r.project, "--target", "kiro", "--format", "json"}, false, nil
	case "repair-a", "remove-a", "remove-b":
		verb := strings.Split(label, "-")[0]
		return r.project, []string{c.f.installer, verb, r.installation.InstallationID, "--scope", "project", "--project-root", r.project, "--target", "kiro", "--format", "json", "--yes"}, false, nil
	case "native-write-a", "native-write-b", "native-restart-a", "native-remaining-b", "native-removed-a", "native-removed-b":
		operation := "read"
		if strings.HasPrefix(label, "native-write-") {
			operation = "write"
		}
		prompt := "Call @" + c.f.name + "/inspect_runtime exactly once with nonce " + c.f.base.Nonce + ", operation " + operation + " and marker " + r.marker + ". Do not use any other tool."
		if strings.HasPrefix(label, "native-removed-") {
			prompt = "Call @" + c.f.name + "/inspect_runtime only if that tool is available, with nonce " + c.f.base.Nonce + " and operation read. If unavailable, finish without using any tool. Do not use any other tool."
		}
		return r.project, []string{c.f.client, "chat", "--v2", "--no-interactive", "--agent", kiroCycleAgentName(key), "--trust-tools", "@" + c.f.name + "/inspect_runtime", prompt}, true, nil
	default:
		return "", nil, false, fmt.Errorf("unregistered Kiro native cycle command")
	}
}

func (c *kiroNativeCycle) run(t *testing.T, label string) ports.CommandResult {
	t.Helper()
	if len(c.steps) >= len(kiroCycleOrder) || kiroCycleOrder[len(c.steps)] != label {
		t.Fatal("Kiro closed one-shot command order")
	}
	cwd, argv, authenticated, err := c.args(label)
	if err != nil {
		t.Fatal(err)
	}
	c.check(t)
	c.eventPrefix(t)
	c.f.evidence["attempt_stage"] = label
	ctx, cancel := context.WithTimeout(c.f.ctx, 30*time.Second)
	defer cancel()
	env := c.f.env()
	if authenticated {
		env = c.f.kiroAuth.commandEnv("kiro", "native-attempt", env)
	}
	start := time.Now().UTC()
	result, runErr := (process.OS{}).RunWithDescendantExitGrace(ctx, ports.Command{Argv: argv, Env: env, Dir: cwd, StdoutLimitBytes: 1 << 20}, 5*time.Second)
	end := time.Now().UTC()
	meta := map[string]any{"argv": argv, "cwd": cwd, "exit": result.ExitCode, "error": runErr != nil, "deadline": ctx.Err() != nil, "elapsed_ms": end.Sub(start).Milliseconds(), "cleanup": "full descendant supervision; forced cleanup fails", "started_at": start.Format(time.RFC3339Nano), "ended_at": end.Format(time.RFC3339Nano), "event_offset": len(c.events)}
	c.f.keep(t, label+".stdout.log", result.Stdout)
	c.f.keep(t, label+".stderr.log", result.Stderr)
	raw, _ := json.Marshal(meta)
	c.f.keep(t, label+".command.json", raw)
	c.steps = append(c.steps, label)
	if runErr != nil || ctx.Err() != nil || end.Sub(start) > 35*time.Second || result.ExitCode != 0 || len(result.Stdout) > 1<<20 || len(result.Stderr) > 1<<20 || bytes.Contains(result.Stderr, []byte("stderr bytes omitted")) {
		t.Fatal("Kiro native/public command did not finish successfully with ordinary descendant cleanup")
	}
	c.check(t)
	if !strings.HasPrefix(label, "native-") {
		c.eventPrefix(t)
	}
	return result
}

func (c *kiroNativeCycle) help(t *testing.T) {
	t.Helper()
	r := c.run(t, "chat-help")
	for _, token := range []string{"--v2", "--no-interactive", "--agent", "--trust-tools"} {
		if !bytes.Contains(r.Stdout, []byte(token)) {
			t.Fatal("pinned Kiro chat help does not declare the exact candidate syntax")
		}
	}
	r = c.run(t, "agent-help")
	if !bytes.Contains(r.Stdout, []byte("--path")) {
		t.Fatal("pinned Kiro agent validation syntax is unavailable")
	}
}

func (c *kiroNativeCycle) public(t *testing.T, label string) {
	t.Helper()
	key := "a"
	if strings.HasSuffix(label, "-b") {
		key = "b"
	}
	r := c.roles[key]
	c.check(t)
	if strings.HasPrefix(label, "remove-") {
		for path := range c.f.immutable {
			if strings.HasPrefix(path, r.binding.TargetLocator+string(filepath.Separator)) {
				delete(c.f.immutable, path)
			}
		}
	}
	// Only this owned state/config projection can change during the matching
	// public operation. Other roles and the caller-authored agent stay pinned.
	for _, path := range []string{r.statePath(), r.configPath(), r.skillPath()} {
		delete(c.pins, path)
		delete(c.f.protected, path)
	}
	result := c.run(t, label)
	if err := kiroCyclePublicJSON(result.Stdout, strings.Split(label, "-")[0]); err != nil {
		t.Fatal(err)
	}
}

func kiroCyclePublicJSON(raw []byte, verb string) error {
	value, err := projectNativeRuntimeJSON(raw)
	object, ok := value.(map[string]any)
	if err != nil || !ok || nativeValidateInstallerJSON(raw) != nil || projectNativeRuntimeKeys(object, "schema_version command result data", "") != nil || object["schema_version"] != json.Number("1") || object["command"] != verb || object["result"] != "success" {
		return fmt.Errorf("Kiro public success envelope differs")
	}
	return nil
}
