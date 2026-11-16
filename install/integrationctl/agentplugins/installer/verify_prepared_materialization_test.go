package installer

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/opencodehost"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// This inert native protocol reads the real installed config and copied Skill.
// It never launches an MCP process or substitutes saved lifecycle facts.
type materializationOpenCodeRunner struct {
	profile, executable string
	starts              int
	unclosed            bool
}

func (r *materializationOpenCodeRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	if len(command.Argv) != 3 || command.Argv[0] != r.executable || command.Dir != r.profile || !slices.Contains(command.Env, "OPENCODE_CONFIG_DIR="+r.profile) {
		return legacyports.CommandResult{NoChildStarted: true}, errors.New("inert native query lost exact profile")
	}
	if r.unclosed {
		return legacyports.CommandResult{}, &domain.UnclosedClientProbe{Operation: "inert native query", Err: errors.New("fixture cleanup incomplete")}
	}
	switch strings.Join(command.Argv[1:], " ") {
	case "debug config":
		body, err := os.ReadFile(filepath.Join(r.profile, "opencode.json"))
		return legacyports.CommandResult{Stdout: body, CleanupComplete: true}, err
	case "debug skill":
		path := filepath.Join(r.profile, "skills", "sample-notify", "SKILL.md")
		body, err := os.ReadFile(path)
		if err != nil {
			return legacyports.CommandResult{CleanupComplete: true}, err
		}
		_, content, ok := strings.Cut(strings.TrimPrefix(string(body), "---\n"), "\n---\n")
		if !ok {
			return legacyports.CommandResult{CleanupComplete: true}, errors.New("inert native Skill metadata is missing")
		}
		rows, err := json.Marshal([]map[string]string{{"name": "sample-notify", "location": path, "content": content}})
		return legacyports.CommandResult{Stdout: rows, CleanupComplete: true}, err
	case "mcp list":
		r.starts++
		return legacyports.CommandResult{Stdout: []byte("┌  MCP Servers\n● ✓ sample-notify connected\n└  1 server(s)\n"), CleanupComplete: true}, nil
	default:
		return legacyports.CommandResult{NoChildStarted: true}, errors.New("unexpected inert native command")
	}
}

func newMaterializationOpenCodeFixture(t *testing.T) (*Engine, Request, *materializationOpenCodeRunner) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(root, "opencode.exe")
	if err := os.WriteFile(executable, []byte("inert native fixture; never launched"), 0700); err != nil {
		t.Fatal(err)
	}
	request := openCodeRequest(t, root, executable)
	request.InstallationID, request.OperationID = "00000000-0000-4000-8000-000000009932", "materialization-install"
	if err := os.MkdirAll(request.ClientConfigRoot, 0700); err != nil {
		t.Fatal(err)
	}
	deathWrite(t, filepath.Join(request.ClientConfigRoot, "opencode.json"), `{"theme":"foreign"}`)
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	runner := &materializationOpenCodeRunner{profile: request.ClientConfigRoot, executable: executable}
	engine := openCodeEngine(t, Config{StateRoot: filepath.Join(root, "state"), HelperExecutable: helper, Runner: runner, EnableNativeObserver: true,
		OpenCodeProbe: func(_ context.Context, target clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
			return clientdetect.ProbeEvidence{VersionEvidence: opencodehost.VersionEvidence{Version: "1.18.34", Source: "executable_version", ProbeStatus: "ok", ExecutableIdentity: "inert:" + target.Executable}}, nil
		}})
	prepared, err := engine.Prepare(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.Apply(t.Context(), prepared, Decision{Confirmed: true})
	closeOpenCodeTestHandle(t, prepared)
	if err != nil || result.Client.Activation != string(domain.ActivationActive) || runner.starts != 1 {
		t.Fatalf("fixture must complete native startup only in confirmed apply: %+v starts=%d %v", result, runner.starts, err)
	}
	request.Operation, request.OperationID = OpRepair, "materialization-proof"
	return engine, request, runner
}

func TestPreparedMaterializationDoesNotAttestReadinessOrStartMCP(t *testing.T) {
	for _, fault := range []string{"none", "missing-skill", "foreign-mcp", "policy", "unclosed-probe"} {
		t.Run(fault, func(t *testing.T) {
			engine, request, runner := newMaterializationOpenCodeFixture(t)
			starts := runner.starts
			prepared, err := engine.Prepare(t.Context(), request)
			if err != nil {
				t.Fatal("read-only repair preparation", err)
			}
			defer closeOpenCodeTestHandle(t, prepared)
			// Mutations after preparation must still be rejected by the sealed
			// observation; ownership preflight would refuse them even earlier.
			configPath := filepath.Join(request.ClientConfigRoot, "opencode.json")
			skillPath := filepath.Join(request.ClientConfigRoot, "skills", "sample-notify", "SKILL.md")
			if fault == "missing-skill" {
				if err := os.Remove(skillPath); err != nil {
					t.Fatal(err)
				}
			}
			if fault == "foreign-mcp" || fault == "policy" {
				body, err := os.ReadFile(configPath)
				if err != nil {
					t.Fatal(err)
				}
				var config map[string]any
				if err := json.Unmarshal(body, &config); err != nil {
					t.Fatal(err)
				}
				if fault == "policy" {
					config["permission"] = map[string]any{"*": "deny"}
				} else {
					server := config["mcp"].(map[string]any)["sample-notify"].(map[string]any)
					server["command"] = []string{filepath.Join(request.ClientConfigRoot, "foreign-server")}
				}
				body, err = json.Marshal(config)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(configPath, body, 0600); err != nil {
					t.Fatal(err)
				}
			}
			runner.unclosed = fault == "unclosed-probe"
			before, err := engine.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			configBefore, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			ready, readyErr := engine.VerifyPreparedBinding(t.Context(), prepared)
			complete, proofErr := engine.VerifyPreparedMaterialization(t.Context(), prepared)
			if ready || complete != (fault == "none") || fault == "none" && (proofErr != nil || readyErr != nil) {
				t.Fatalf("materialization confused with native readiness: ready=%t complete=%t err=%v readinessErr=%v", ready, complete, proofErr, readyErr)
			}
			if fault == "unclosed-probe" && !domain.IsUnclosedClientProbe(proofErr) {
				t.Fatal("unclosed probe refusal was lost", proofErr)
			}
			after, err := engine.store.Load()
			if err != nil || !reflect.DeepEqual(before, after) || runner.starts != starts {
				t.Fatal("read-only proof changed receipts, readiness or started MCP", err, runner.starts, starts)
			}
			configAfter, err := os.ReadFile(configPath)
			if err != nil || string(configAfter) != string(configBefore) {
				t.Fatal("proof overwrote native configuration", err)
			}
			if fault == "missing-skill" {
				if _, err := os.Stat(skillPath); !os.IsNotExist(err) {
					t.Fatal("proof repaired missing Skill before confirmation", err)
				}
			}
		})
	}
}

func TestPreparedMaterializationStillRequiresConfirmedNativeSetup(t *testing.T) {
	engine, request, runner := newMaterializationOpenCodeFixture(t)
	starts := runner.starts
	prepared, err := engine.Prepare(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	defer closeOpenCodeTestHandle(t, prepared)
	if prepared.Plan().Operation != OpRepair || prepared.Plan().NoChange || runner.starts != starts {
		t.Fatal("native setup must remain behind the existing repair review", prepared.Plan(), runner.starts, starts)
	}
	before, err := engine.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.Apply(t.Context(), prepared, Decision{})
	if !errors.Is(err, ErrCancelled) || result.Outcome != OutcomeCancelled || result.Mutated || runner.starts != starts {
		t.Fatal("declining repair started native setup", result, err, runner.starts, starts)
	}
	after, err := engine.store.Load()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("declining repair changed saved state", err)
	}
	result, err = engine.Apply(t.Context(), prepared, Decision{Confirmed: true})
	if err != nil || result.Client.Activation != string(domain.ActivationActive) || runner.starts != starts+1 {
		t.Fatal("confirmed repair did not use the fresh native startup verifier", result, err, runner.starts, starts)
	}
}
