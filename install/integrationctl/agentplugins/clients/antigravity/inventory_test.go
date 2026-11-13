package antigravity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type inventoryRunner struct {
	calls []legacyports.Command
	reply func(context.Context, legacyports.Command) (legacyports.CommandResult, error)
}

func (runner *inventoryRunner) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	runner.calls = append(runner.calls, command)
	return runner.reply(ctx, command)
}

func inventoryFixture(t *testing.T) (domain.ActivationRequest, clients.Env, *inventoryRunner) {
	t.Helper()
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(home, ".gemini", "config")
	active := filepath.Join(root, "plugins", "owned-physical")
	if err = os.MkdirAll(active, 0700); err != nil {
		t.Fatal(err)
	}
	if err = shared.WriteJSON(filepath.Join(active, "plugin.json"), map[string]any{"$schema": "https://antigravity.google/schemas/v1/plugin.json", "name": "demo"}); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(home, "agy")
	if err = os.WriteFile(binary, []byte("inert fixture; never executed"), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "config.json"), []byte(`{"plugins":{"owned-physical":{"enabled":true}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	req := domain.ActivationRequest{
		Client: domain.DetectedClient{ClientID: domain.ClientAntigravity, ConfigRoot: root, NativeConfigRoot: root, ExecutablePath: binary, Version: "decorative"},
		Plan: domain.DeliveryPlan{ClientID: domain.ClientAntigravity, Scope: domain.ScopeUser, NativeRegistryRoot: root,
			NativeRegistryExecutable: binary, TargetRoot: filepath.Dir(active), ActivePath: active, DeclaredVersion: "1.0.0",
			Components: []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "skill", Support: domain.SupportNative}, {Kind: domain.ComponentMCPServer, Name: "server", Support: domain.SupportNative}}},
		Delivery:     domain.StagedDelivery{ClientID: domain.ClientAntigravity, ActivePath: active},
		DeclaredName: "demo", BackendExecutable: binary,
	}
	runner := &inventoryRunner{}
	runner.reply = func(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
		body := []byte("1.5.3\n")
		if len(command.Argv) == 4 {
			body = inventoryBody(t, req.Plan, true)
		}
		return legacyports.CommandResult{CleanupComplete: true, Stdout: body}, nil
	}
	return req, clients.Env{Runner: runner}, runner
}

func inventoryBody(t *testing.T, plan domain.DeliveryPlan, enabled bool) []byte {
	t.Helper()
	body, err := json.Marshal([]any{map[string]any{"name": filepath.Base(plan.ActivePath), "path": plan.ActivePath,
		"version": plan.DeclaredVersion, "enabled": enabled, "skills": []string{"skill"}, "mcpServers": []string{"server"}}})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestInventoryUsesOnlyQualifiedUserMetadataAndRetainsPrepared(t *testing.T) {
	req, env, runner := inventoryFixture(t)
	outcome, err := New().Activate(context.Background(), env, req)
	if err != nil || outcome.Activation != domain.ActivationPrepared || outcome.Verification != domain.VerificationInstalled ||
		outcome.ActivationAttested || outcome.AuthenticationAttested || len(outcome.NativeObjects) != 0 || outcome.NativeEffect != domain.NativeEffectUnchanged {
		t.Fatal("metadata promoted activation or failed", outcome, err)
	}
	home := filepath.Dir(filepath.Dir(req.Client.ConfigRoot))
	if len(runner.calls) != 2 || strings.Join(runner.calls[0].Argv[1:], " ") != "--version" ||
		strings.Join(runner.calls[1].Argv[1:], " ") != "plugin list --json" {
		t.Fatal("unexpected native actions", runner.calls)
	}
	for _, command := range runner.calls {
		if command.Dir != req.Client.ConfigRoot || command.StdoutLimitBytes != 1<<20 ||
			strings.Join(command.Env, "\n") != "HOME="+home+"\nUSERPROFILE="+home {
			t.Fatal("native observation was rerouted or inherited provider state", command)
		}
	}
}

func TestInventoryUnavailableCannotSucceedOnResume(t *testing.T) {
	for _, unavailable := range []string{"backend", "runner", "provenance", "missing-config", "missing-override", "malformed-config", "unknown-config"} {
		for _, phase := range []string{"initial", "resume", "verify-only"} {
			t.Run(unavailable+"/"+phase, func(t *testing.T) {
				req, env, runner := inventoryFixture(t)
				req.PackageUnchanged, req.VerifyOnly = phase == "resume", phase == "verify-only"
				switch unavailable {
				case "runner":
					env.Runner = nil
				case "backend":
					req.BackendExecutable = ""
				case "provenance":
					req.Client.NativeConfigRoot = ""
				case "missing-config":
					if err := os.Remove(filepath.Join(req.Client.ConfigRoot, "config.json")); err != nil {
						t.Fatal(err)
					}
				default:
					body := []byte("{")
					if unavailable == "missing-override" {
						body = []byte(`{"plugins":{}}`)
					}
					if unavailable == "unknown-config" {
						body = []byte(`{"plugins":{},"hooks":{"unexpected":true}}`)
					}
					if err := os.WriteFile(filepath.Join(req.Client.ConfigRoot, "config.json"), body, 0600); err != nil {
						t.Fatal(err)
					}
				}
				outcome, err := New().Activate(context.Background(), env, req)
				if (err != nil) != (phase != "initial") || outcome.Activation != domain.ActivationPrepared || len(runner.calls) != 0 {
					t.Fatal("unavailable observation became successful resume or started a child", outcome, err, runner.calls)
				}
			})
		}
	}
}

func TestInventoryDisabledAndAbsentAreRecognizedWithoutEnable(t *testing.T) {
	for _, absent := range []bool{false, true} {
		req, env, runner := inventoryFixture(t)
		if err := os.WriteFile(filepath.Join(req.Client.ConfigRoot, "config.json"), []byte(`{"plugins":{"owned-physical":{"enabled":false}}}`), 0600); err != nil {
			t.Fatal(err)
		}
		runner.reply = func(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
			body := []byte("1.5.3\n")
			if len(command.Argv) == 4 {
				body = inventoryBody(t, req.Plan, false)
				if absent {
					body = []byte("[]")
				}
			}
			return legacyports.CommandResult{CleanupComplete: true, Stdout: body}, nil
		}
		out, err := New().Activate(context.Background(), env, req)
		if err != nil || out.Activation != domain.ActivationPrepared || !out.AuthoritativeObservation || len(runner.calls) != 2 {
			t.Fatal("recognized negative was lost or activated", out, err)
		}
	}
}

func TestInventoryRefusesUnsettledDirtyOversizedUnknownVersionAndLateContext(t *testing.T) {
	for _, fault := range []string{"cleanup", "not-started", "exit", "stderr", "cap", "error", "version", "context"} {
		t.Run(fault, func(t *testing.T) {
			req, env, runner := inventoryFixture(t)
			if fault == "version" {
				req.Client.Version = "1.5.3"
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			runner.reply = func(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
				result := legacyports.CommandResult{CleanupComplete: true, Stdout: []byte("1.5.3\n")}
				switch fault {
				case "cleanup":
					result.CleanupComplete = false
				case "not-started":
					result.NoChildStarted = true
				case "exit":
					result.ExitCode = 1
				case "stderr":
					result.Stderr = []byte("unrecognized diagnostic")
				case "cap":
					result.Stdout = make([]byte, 1<<20)
				case "version":
					result.Stdout = []byte("1.5.4\n")
				case "context":
					cancel()
				case "error":
					return result, errors.New("execution refused")
				}
				return result, nil
			}
			out, err := New().Activate(ctx, env, req)
			if err == nil || out.Activation == domain.ActivationActive || len(runner.calls) != 1 || out.NativeEffect != domain.NativeEffectUncertain {
				t.Fatal("invalid native exchange became observation", out, err, runner.calls)
			}
		})
	}
}

func TestInventoryHeldFilesRefuseSameBytesReplacementAndConflictingBoolean(t *testing.T) {
	for _, effect := range []string{"config", "binary", "boolean"} {
		t.Run(effect, func(t *testing.T) {
			req, env, runner := inventoryFixture(t)
			runner.reply = func(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
				if effect != "boolean" {
					path := req.BackendExecutable
					if effect == "config" {
						path = filepath.Join(req.Client.ConfigRoot, "config.json")
					}
					body, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					if err = os.Rename(path, path+"-original"); err != nil {
						t.Fatal(err)
					}
					if err = os.WriteFile(path, body, 0600); err != nil {
						t.Fatal(err)
					}
				}
				body := []byte("1.5.3\n")
				if len(command.Argv) == 4 {
					body = inventoryBody(t, req.Plan, false)
				}
				return legacyports.CommandResult{CleanupComplete: true, Stdout: body}, nil
			}
			if out, err := New().Activate(context.Background(), env, req); err == nil || out.Activation == domain.ActivationActive {
				t.Fatal("unknown write or contradictory boolean accepted", out, err)
			}
			want := 1
			if effect == "boolean" {
				want = 2
			}
			if len(runner.calls) != want {
				t.Fatal("continued after first refusal", runner.calls)
			}
		})
	}
}

func TestInventoryParserRejectsAmbiguousOrUnrecognizedRows(t *testing.T) {
	req, _, _ := inventoryFixture(t)
	good := inventoryBody(t, req.Plan, true)
	for _, body := range [][]byte{
		append(append([]byte{}, good...), []byte("{}")...), []byte("{}"), []byte("null"),
		[]byte(`[{"name":"other","path":"/foreign","version":1,"enabled":false,"skills":[],"mcpServers":[]}]`),
		[]byte(strings.Replace(string(good), `"enabled":true`, `"enabled":1`, 1)),
		[]byte(strings.Replace(string(good), `"version":"1.0.0"`, `"version":"2.0.0"`, 1)),
		[]byte(strings.Replace(string(good), `"skills":["skill"]`, `"skills":["skill","skill"]`, 1)),
		[]byte(strings.Replace(string(good), `"mcpServers":["server"]`, `"mcpServers":[]`, 1)),
		[]byte(strings.Replace(string(good), `"enabled":true`, `"enabled":true,"Enabled":true`, 1)),
		[]byte(strings.Replace(string(good), `"name":`, `"unknown":0,"name":`, 1)),
		[]byte("[" + string(good[1:len(good)-1]) + "," + string(good[1:len(good)-1]) + "]"),
		[]byte(strings.Replace(string(good), filepath.Base(req.Plan.ActivePath), "different-physical", 1)),
	} {
		if _, _, err := selectedInventory(body, req.Plan); err == nil {
			t.Fatal("unrecognized row accepted", string(body))
		}
	}
}

func TestInventoryProjectRemainsFileOnlyEvenWithQualifiedUserBackend(t *testing.T) {
	req, env, runner := inventoryFixture(t)
	project := t.TempDir()
	root := filepath.Join(project, ".agents")
	active := filepath.Join(root, "plugins", "owned-physical")
	if err := os.MkdirAll(active, 0700); err != nil {
		t.Fatal(err)
	}
	if err := shared.WriteJSON(filepath.Join(active, "plugin.json"), map[string]any{"$schema": "https://antigravity.google/schemas/v1/plugin.json", "name": "demo"}); err != nil {
		t.Fatal(err)
	}
	req.Plan.Scope = domain.ScopeProject
	req.Plan.TargetContext = domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: project, UserConfigRoot: req.Client.ConfigRoot}
	req.Plan.NativeRegistryRoot, req.Plan.TargetRoot, req.Plan.ActivePath = root, filepath.Dir(active), active
	req.Delivery.ActivePath = active
	req.VerifyOnly = true
	if New().VerifierAvailable(req.Client, req.Plan, req.BackendExecutable) {
		t.Fatal("User routing trait promoted Project native verification")
	}
	out, err := New().Activate(context.Background(), env, req)
	if err != nil || out.Activation != domain.ActivationPrepared || out.Verification != domain.VerificationInstalled || len(runner.calls) != 0 {
		t.Fatal("Project file verification became native User observation", out, err, runner.calls)
	}
}

type cancelInventoryReader struct {
	io.ReadSeeker
	cancel context.CancelFunc
	reads  int
}

func (reader *cancelInventoryReader) Read(buffer []byte) (int, error) {
	reader.reads++
	n, err := reader.ReadSeeker.Read(buffer)
	reader.cancel()
	return n, err
}

func TestInventoryDigestCancellationStopsAfterFirstChunkBeforeAnyCommand(t *testing.T) {
	req, env, runner := inventoryFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader := &cancelInventoryReader{ReadSeeker: bytes.NewReader(make([]byte, 3*32768)), cancel: cancel}
	if _, err := inventoryDigest(ctx, reader, 512<<20); !errors.Is(err, context.Canceled) || reader.reads != 1 {
		t.Fatal("digest continued after original-context cancellation", err, reader.reads)
	}
	if _, err := New().Activate(ctx, env, req); !errors.Is(err, context.Canceled) || len(runner.calls) != 0 {
		t.Fatal("canceled observation started a native command", err, runner.calls)
	}
}

func TestInventoryRefusesCasefoldSelectedAliasesAndDuplicateIDs(t *testing.T) {
	req, _, _ := inventoryFixture(t)
	good := inventoryBody(t, req.Plan, true)
	var rows []map[string]any
	if err := json.Unmarshal(good, &rows); err != nil {
		t.Fatal(err)
	}
	for _, exactAlso := range []bool{false, true} {
		for _, kind := range []string{"name-alias", "path-alias", "duplicate-name", "duplicate-path"} {
			alias := map[string]any{}
			for key, value := range rows[0] {
				alias[key] = value
			}
			switch kind {
			case "name-alias":
				alias["name"], alias["path"] = strings.ToUpper(filepath.Base(req.Plan.ActivePath)), filepath.Join(filepath.Dir(req.Plan.ActivePath), "foreign", strings.ToUpper(filepath.Base(req.Plan.ActivePath)))
			case "path-alias":
				alias["name"], alias["path"] = strings.ToUpper(filepath.Base(req.Plan.ActivePath)), strings.ToUpper(req.Plan.ActivePath)
			case "duplicate-name":
				alias["name"], alias["path"] = "foreign", filepath.Join(filepath.Dir(req.Plan.ActivePath), "first", "foreign")
			case "duplicate-path":
				alias["name"], alias["path"] = "foreign", filepath.Join(filepath.Dir(req.Plan.ActivePath), "foreign")
			}
			input := []map[string]any{alias}
			if strings.HasPrefix(kind, "duplicate") {
				other := map[string]any{}
				for key, value := range alias {
					other[key] = value
				}
				if kind == "duplicate-name" {
					other["name"] = "FOREIGN"
					other["path"] = filepath.Join(filepath.Dir(req.Plan.ActivePath), "second", "FOREIGN")
				} else {
					other["name"] = "foreign"
				}
				input = append(input, other)
			}
			if exactAlso {
				input = append(input, rows[0])
			}
			body, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err = selectedInventory(body, req.Plan); err == nil {
				t.Fatal("ambiguous identity became presence or authoritative absence", kind, exactAlso)
			}
		}
	}
}

type boundedInventoryReader struct {
	io.ReadSeeker
	maximum int
}

func (reader *boundedInventoryReader) Read(buffer []byte) (int, error) {
	reader.maximum = max(reader.maximum, len(buffer))
	return reader.ReadSeeker.Read(buffer)
}

func TestInventoryDigestPreservesLimitPlusOneReadBound(t *testing.T) {
	reader := &boundedInventoryReader{ReadSeeker: bytes.NewReader(make([]byte, 32768))}
	if _, err := inventoryDigest(context.Background(), reader, 16384); err == nil || reader.maximum != 16385 {
		t.Fatal("digest read beyond the admitted limit plus one", err, reader.maximum)
	}
}

func TestInventoryInitialPrerequisiteFallbackNeverSwallowsCancellation(t *testing.T) {
	req, env, runner := inventoryFixture(t)
	if err := os.Remove(filepath.Join(req.Client.ConfigRoot, "config.json")); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	outcome := domain.ActivationOutcome{Activation: domain.ActivationPrepared}
	if _, err := userInventoryOutcome(ctx, env, req, outcome); !errors.Is(err, context.Canceled) || len(runner.calls) != 0 {
		t.Fatal("initial missing-config fallback swallowed cancellation", err, runner.calls)
	}
}
