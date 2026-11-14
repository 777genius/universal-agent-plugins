package antigravity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

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
			NativeRegistryExecutable: binary, TargetRoot: filepath.Dir(active), ActivePath: active, DeclaredName: "demo", DeclaredVersion: "1.0.0",
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

func TestInventoryIneligibleKeepsFileOnlyAndEligibleFailureRefusesResume(t *testing.T) {
	for _, unavailable := range []string{"backend", "runner", "provenance", "missing-override", "malformed-config", "unknown-config"} {
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
				eligible := unavailable != "backend" && unavailable != "runner" && unavailable != "provenance"
				if (err != nil) != (eligible && phase != "initial") || outcome.Activation != domain.ActivationPrepared || len(runner.calls) != 0 ||
					strings.Contains(strings.Join(outcome.UserActions, " "), "enabled in CLI inventory") {
					t.Fatal("file-only fallback claimed native evidence or eligible failure succeeded", outcome, err, runner.calls)
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

func TestInventoryRefusesForeignManifestCreatedDuringFinalList(t *testing.T) {
	req, env, runner := inventoryFixture(t)
	foreign := filepath.Join(req.Plan.TargetRoot, "late-foreign")
	body := []byte(`{"$schema":"https://antigravity.google/schemas/v1/plugin.json","name":"demo"}`)
	runner.reply = func(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
		stdout := []byte("1.5.3\n")
		if len(command.Argv) == 4 {
			if err := os.Mkdir(foreign, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(foreign, "plugin.json"), body, 0600); err != nil {
				t.Fatal(err)
			}
			stdout = inventoryBody(t, req.Plan, true)
		}
		return legacyports.CommandResult{CleanupComplete: true, Stdout: stdout}, nil
	}
	outcome, err := New().Activate(t.Context(), env, req)
	if err == nil || len(runner.calls) != 2 || outcome.NativeEffect != domain.NativeEffectUncertain || outcome.Activation != domain.ActivationPrepared {
		t.Fatal("final list borrowed newly colliding namespace evidence", outcome, err, runner.calls)
	}
	after, err := os.ReadFile(filepath.Join(foreign, "plugin.json"))
	if err != nil || !bytes.Equal(after, body) {
		t.Fatal("failed observation rewrote or removed foreign evidence", err)
	}
}

func TestInventoryVersionAcceptsOnlyCapturedLF(t *testing.T) {
	for _, version := range []string{"1.5.3\n", "1.5.3", "1.5.3\r\n", " 1.5.3\n", "1.5.3\n\n", "\u00a01.5.3\n"} {
		t.Run(fmt.Sprintf("%q", version), func(t *testing.T) {
			req, env, runner := inventoryFixture(t)
			runner.reply = func(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
				body := []byte(version)
				if len(command.Argv) == 4 {
					body = inventoryBody(t, req.Plan, true)
				}
				return legacyports.CommandResult{CleanupComplete: true, Stdout: body}, nil
			}
			outcome, err := New().Activate(t.Context(), env, req)
			accepted, calls := version == "1.5.3\n", 1
			if accepted {
				calls = 2
			}
			if (err == nil) != accepted || len(runner.calls) != calls || outcome.Activation != domain.ActivationPrepared {
				t.Fatal("version framing changed the admitted native protocol", outcome, err, runner.calls)
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

func TestInventoryUserRegistryLayoutPreservesSelectionWithoutInventingProvenance(t *testing.T) {
	for _, provenance := range []string{"original", "absent", "foreign"} {
		t.Run(provenance, func(t *testing.T) {
			req, env, runner := inventoryFixture(t)
			if provenance == "absent" {
				req.Client.NativeConfigRoot = ""
			}
			if provenance == "foreign" {
				req.Client.NativeConfigRoot = filepath.Join(t.TempDir(), ".gemini", "config")
			}
			beforeRoot, beforeNativeRoot := req.Client.ConfigRoot, req.Client.NativeConfigRoot
			layout, ok := any(New()).(clients.UserNativeRegistryLayout)
			if !ok || layout.UserNativeRegistry(req.Client) != beforeRoot || req.Client.NativeConfigRoot != beforeNativeRoot {
				t.Fatal("User layout changed selection or invented original profile provenance")
			}
			req.VerifyOnly = true
			if available := New().VerifierAvailable(req.Client, req.Plan, req.BackendExecutable); available != (provenance == "original") {
				t.Fatal("User layout alone granted inventory authority", provenance, available)
			}
			outcome, err := New().Activate(t.Context(), env, req)
			calls := 0
			if provenance == "original" {
				calls = 2
			}
			if err != nil || len(runner.calls) != calls || outcome.Activation != domain.ActivationPrepared ||
				outcome.ActivationAttested || outcome.AuthenticationAttested {
				t.Fatal("profile provenance changed read-only inventory qualification", outcome, err, runner.calls)
			}
			if provenance != "original" && strings.Contains(strings.Join(outcome.UserActions, " "), "enabled in CLI inventory") {
				t.Fatal("unqualified profile manufactured native observation", outcome)
			}
		})
	}
}

func missingInventoryFixture(t *testing.T) (domain.ActivationRequest, clients.Env, *inventoryRunner) {
	t.Helper()
	req, env, runner := inventoryFixture(t)
	if err := os.Remove(filepath.Join(req.Client.ConfigRoot, "config.json")); err != nil {
		t.Fatal(err)
	}
	return req, env, runner
}

func inventoryFixtureBytes(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		key := fmt.Sprintf("%s:%s", path, info.Mode())
		if info.IsDir() {
			files[key] = ""
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unexpected fixture type at %s", path)
		}
		body, err := os.ReadFile(path)
		files[key] = string(body)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestInventoryMissingConfigRequiredObservationPreservesOriginalProfile(t *testing.T) {
	for _, phase := range []string{"resume", "verify-only"} {
		for _, state := range []string{"enabled", "disabled", "absent"} {
			t.Run(phase+"/"+state, func(t *testing.T) {
				req, env, runner := missingInventoryFixture(t)
				req.PackageUnchanged, req.VerifyOnly = phase == "resume", phase == "verify-only"
				home := filepath.Dir(filepath.Dir(req.Client.ConfigRoot))
				before := inventoryFixtureBytes(t, home)
				parent, err := os.Lstat(req.Client.ConfigRoot)
				if err != nil {
					t.Fatal(err)
				}
				var firstContext context.Context
				var deadline time.Time
				runner.reply = func(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
					current, ok := ctx.Deadline()
					if !ok || time.Until(current) > 10*time.Second {
						t.Fatal("native observation lacks its original bounded context")
					}
					if firstContext == nil {
						firstContext, deadline = ctx, current
					} else if ctx != firstContext || !current.Equal(deadline) {
						t.Fatal("native pair renewed the context or its deadline")
					}
					body := []byte("1.5.3\n")
					if len(command.Argv) == 4 {
						body = inventoryBody(t, req.Plan, state == "enabled")
						if state == "absent" {
							body = []byte("[]")
						}
					}
					return legacyports.CommandResult{CleanupComplete: true, Stdout: body}, nil
				}
				outcome, err := New().Activate(t.Context(), env, req)
				guidance := map[string]string{
					"enabled":  "The selected User plugin is enabled in CLI inventory; open or reload Antigravity to verify IDE loading",
					"disabled": "The selected User plugin remains disabled; decide whether to enable it in Antigravity",
					"absent":   "The exact UAP-owned plugin is absent from native User inventory; inspect it in Antigravity",
				}[state]
				if err != nil || len(runner.calls) != 2 || outcome.Activation != domain.ActivationPrepared ||
					outcome.Verification != domain.VerificationInstalled || outcome.NativeEffect != domain.NativeEffectUnchanged ||
					outcome.ActivationAttested || outcome.AuthenticationAttested || len(outcome.NativeObjects) != 0 ||
					outcome.AuthoritativeObservation != (state != "enabled") || !reflect.DeepEqual(outcome.UserActions, []string{guidance}) {
					t.Fatal("missing override changed actual native evidence or qualification", outcome, err, runner.calls)
				}
				after, err := os.Lstat(req.Client.ConfigRoot)
				if err != nil || !sameInventoryFile(parent, after) || !reflect.DeepEqual(before, inventoryFixtureBytes(t, home)) {
					t.Fatal("missing-config observation changed original profile or package", err)
				}
				if _, err := os.Lstat(filepath.Join(req.Client.ConfigRoot, "config.json")); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("observation created or adopted config", err)
				}
			})
		}
	}
}

func TestInventoryMissingConfigInitialDeliveryAndInvalidParentsStayDistinct(t *testing.T) {
	for _, phase := range []string{"initial", "resume", "verify-only"} {
		for _, fault := range []string{"none", "parent", "backend"} {
			t.Run(phase+"/"+fault, func(t *testing.T) {
				req, env, runner := missingInventoryFixture(t)
				req.PackageUnchanged, req.VerifyOnly = phase == "resume", phase == "verify-only"
				switch fault {
				case "parent":
					if err := os.Rename(req.Client.ConfigRoot, req.Client.ConfigRoot+"-original"); err != nil {
						t.Fatal(err)
					}
				case "backend":
					if err := os.Remove(req.BackendExecutable); err != nil {
						t.Fatal(err)
					}
				}
				prepared := domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled, NativeEffect: domain.NativeEffectUnchanged}
				outcome, err := userInventoryOutcome(t.Context(), env, req, prepared)
				wantError := fault == "parent" || fault == "backend" && phase != "initial"
				wantCalls := 0
				if fault == "none" && phase != "initial" {
					wantCalls = 2
				}
				if (err != nil) != wantError || len(runner.calls) != wantCalls || outcome.Activation != domain.ActivationPrepared {
					t.Fatal("parent/backend absence borrowed the config-only exception", outcome, err, runner.calls)
				}
				if wantCalls == 0 && strings.Contains(strings.Join(outcome.UserActions, " "), "enabled in CLI inventory") {
					t.Fatal("initial or refused observation manufactured native evidence", outcome)
				}
			})
		}
	}
}

func inventoryAppear(t *testing.T, path, kind string) {
	t.Helper()
	var err error
	switch kind {
	case "file":
		err = os.WriteFile(path, []byte(`{"plugins":{}}`), 0600)
	case "directory":
		err = os.Mkdir(path, 0700)
	case "symlink":
		err = os.Symlink("plugins", path)
	case "dangling-symlink":
		err = os.Symlink("not-created", path)
	default:
		t.Fatal("unknown fixture type", kind)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestInventoryMissingConfigRejectsLeafAppearanceAtEveryBoundary(t *testing.T) {
	kinds := []string{"file", "directory"}
	if runtime.GOOS != "windows" {
		kinds = append(kinds, "symlink", "dangling-symlink")
	}
	for _, kind := range kinds {
		for _, afterCall := range []int{0, 1, 2} {
			t.Run(fmt.Sprintf("%s/after-%d", kind, afterCall), func(t *testing.T) {
				req, env, runner := missingInventoryFixture(t)
				req.VerifyOnly = true
				path := filepath.Join(req.Client.ConfigRoot, "config.json")
				pending := filepath.Join(req.Client.ConfigRoot, "absent.json")
				inventoryAppear(t, pending, kind)
				parent, err := os.Lstat(req.Client.ConfigRoot)
				if err != nil {
					t.Fatal(err)
				}
				appear := func() {
					if err := os.Rename(pending, path); err != nil {
						t.Fatal(err)
					}
					if err := os.Chtimes(req.Client.ConfigRoot, parent.ModTime(), parent.ModTime()); err != nil {
						t.Fatal(err)
					}
					after, err := os.Lstat(req.Client.ConfigRoot)
					if err != nil || !sameInventoryFile(parent, after) {
						t.Fatal("leaf fixture must retain original parent metadata", err)
					}
				}
				if afterCall == 0 {
					record, err := openInventoryFile(t.Context(), path, 16384, true)
					if err != nil {
						t.Fatal(err)
					}
					defer func() {
						if err := record.close(); err != nil {
							t.Error(err)
						}
					}()
					appear()
					if err := record.verify(t.Context()); err == nil || len(runner.calls) != 0 {
						t.Fatal("held original absence accepted a new leaf before native", err, runner.calls)
					}
					return
				}
				reply := runner.reply
				runner.reply = func(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
					if len(runner.calls) == afterCall {
						appear()
					}
					return reply(ctx, command)
				}
				outcome, err := New().Activate(t.Context(), env, req)
				if err == nil || len(runner.calls) != afterCall || outcome.Activation != domain.ActivationPrepared ||
					outcome.NativeEffect != domain.NativeEffectUncertain {
					t.Fatal("native pair adopted a leaf after original absence", outcome, err, runner.calls)
				}
				if _, err := os.Lstat(path); err != nil {
					t.Fatal("refusal removed the unowned appearing leaf", err)
				}
			})
		}
	}
}

func TestInventoryMissingConfigCancellationStopsAtOriginalDeadline(t *testing.T) {
	for _, afterCall := range []int{0, 1, 2} {
		t.Run(fmt.Sprintf("after-%d", afterCall), func(t *testing.T) {
			req, env, runner := missingInventoryFixture(t)
			req.PackageUnchanged = true
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if afterCall == 0 {
				cancel()
			}
			reply := runner.reply
			runner.reply = func(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
				if len(runner.calls) == afterCall {
					cancel()
				}
				return reply(ctx, command)
			}
			outcome, err := New().Activate(ctx, env, req)
			if !errors.Is(err, context.Canceled) || len(runner.calls) != afterCall ||
				outcome.Activation == domain.ActivationActive || afterCall > 0 && outcome.Activation != domain.ActivationPrepared ||
				outcome.ActivationAttested || outcome.AuthenticationAttested {
				t.Fatal("absent config swallowed cancellation or renewed native work", outcome, err, runner.calls)
			}
		})
	}
}

func inventoryParentSpare(t *testing.T, req domain.ActivationRequest) (string, os.FileInfo) {
	t.Helper()
	original, err := os.Lstat(req.Client.ConfigRoot)
	if err != nil {
		t.Fatal(err)
	}
	spare := req.Client.ConfigRoot + "-spare"
	active := filepath.Join(spare, "plugins", filepath.Base(req.Plan.ActivePath))
	if err = os.MkdirAll(active, 0700); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(req.Plan.ActivePath, "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(active, "plugin.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(spare, original.Mode().Perm()); err != nil {
		t.Fatal(err)
	}
	if err = os.Chtimes(spare, original.ModTime(), original.ModTime()); err != nil {
		t.Fatal(err)
	}
	replacement, err := os.Lstat(spare)
	if err != nil || os.SameFile(original, replacement) || original.Mode() != replacement.Mode() ||
		original.Size() != replacement.Size() || !original.ModTime().Equal(replacement.ModTime()) {
		t.Fatal("replacement must differ only in parent incarnation", err)
	}
	return spare, original
}

func inventoryRenamePrevented(err error, from, to string) bool {
	var link *os.LinkError
	return runtime.GOOS == "windows" && errors.As(err, &link) && link.Op == "rename" &&
		link.Old == from && link.New == to && errors.Is(link.Err, syscall.Errno(32))
}

func TestInventoryMissingConfigParentLeaseRefusesReplacementBeforeNative(t *testing.T) {
	req, _, runner := missingInventoryFixture(t)
	spare, original := inventoryParentSpare(t, req)
	parent, moved := req.Client.ConfigRoot, req.Client.ConfigRoot+"-original"
	home := filepath.Dir(filepath.Dir(parent))
	before := inventoryFixtureBytes(t, home)
	record, err := openInventoryFile(t.Context(), filepath.Join(parent, "config.json"), 16384, true)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if record != nil {
			if err := record.close(); err != nil {
				t.Error(err)
			}
		}
	}()
	renameErr := os.Rename(parent, moved)
	if renameErr != nil {
		if !inventoryRenamePrevented(renameErr, parent, moved) {
			t.Fatal("unexpected parent rename failure", renameErr)
		}
		named, namedErr := os.Lstat(parent)
		held, heldErr := record.root.Stat(".")
		_, destinationErr := os.Lstat(moved)
		if namedErr != nil || heldErr != nil || !sameInventoryFile(original, named) ||
			!sameInventoryFile(record.rootHeld, held) || !errors.Is(destinationErr, os.ErrNotExist) ||
			!reflect.DeepEqual(before, inventoryFixtureBytes(t, home)) || record.verify(t.Context()) != nil || len(runner.calls) != 0 {
			t.Fatal("rename denial changed the original held authority", namedErr, heldErr, destinationErr)
		}
		if err := record.close(); err != nil {
			t.Fatal(err)
		}
		record = nil
		if err := os.Rename(parent, moved); err != nil {
			t.Fatal("same rename must succeed after lease release", err)
		}
		if err := os.Rename(moved, parent); err != nil {
			t.Fatal(err)
		}
		return
	}
	if err := os.Rename(spare, parent); err != nil {
		t.Fatal(err)
	}
	if err := record.verify(t.Context()); err == nil || len(runner.calls) != 0 {
		t.Fatal("same-metadata parent replacement renewed original absence", err, runner.calls)
	}
}

func TestInventoryMissingConfigParentRaceRetainsOriginalAuthority(t *testing.T) {
	for _, afterCall := range []int{1, 2} {
		t.Run(fmt.Sprintf("after-%d", afterCall), func(t *testing.T) {
			req, env, runner := missingInventoryFixture(t)
			req.VerifyOnly = true
			spare, original := inventoryParentSpare(t, req)
			parent, moved := req.Client.ConfigRoot, req.Client.ConfigRoot+"-original"
			home := filepath.Dir(filepath.Dir(parent))
			before := inventoryFixtureBytes(t, home)
			prevented := false
			reply := runner.reply
			runner.reply = func(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
				if len(runner.calls) == afterCall {
					if err := os.Rename(parent, moved); err != nil {
						if !inventoryRenamePrevented(err, parent, moved) {
							t.Fatal("unexpected attempted parent rename failure", err)
						}
						named, namedErr := os.Lstat(parent)
						_, destinationErr := os.Lstat(moved)
						if namedErr != nil || !sameInventoryFile(original, named) ||
							!errors.Is(destinationErr, os.ErrNotExist) || !reflect.DeepEqual(before, inventoryFixtureBytes(t, home)) {
							t.Fatal("rename denial changed original fixture", namedErr, destinationErr)
						}
						prevented = true
					} else if err := os.Rename(spare, parent); err != nil {
						t.Fatal(err)
					}
				}
				return reply(ctx, command)
			}
			outcome, err := New().Activate(t.Context(), env, req)
			if prevented {
				// The real lease prevented mutation; Activate must revalidate it and close it.
				if err != nil || len(runner.calls) != 2 || outcome.NativeEffect != domain.NativeEffectUnchanged ||
					outcome.Activation != domain.ActivationPrepared || !reflect.DeepEqual(before, inventoryFixtureBytes(t, home)) {
					t.Fatal("prevented mutation failed original held-parent verification", outcome, err, runner.calls)
				}
				if err := os.Rename(parent, moved); err != nil {
					t.Fatal("same rename must succeed after actual operation releases its lease", err)
				}
				if err := os.Rename(moved, parent); err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || len(runner.calls) != afterCall || outcome.Activation != domain.ActivationPrepared ||
				outcome.NativeEffect != domain.NativeEffectUncertain {
				t.Fatal("native observation borrowed a replacement parent", outcome, err, runner.calls)
			}
		})
	}
}

func TestInventoryParentAcquisitionRejectsReplacedFIFOWithoutBlocking(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("POSIX FIFO acquisition fixture")
	}
	const childEnv = "UAP_ANTIGRAVITY_PARENT_FIFO_TEST_CHILD"
	if os.Getenv(childEnv) == "1" {
		parent, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		original, err := os.Lstat(parent)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(parent, parent+"-original"); err != nil {
			t.Fatal(err)
		}
		if output, err := exec.CommandContext(t.Context(), "mkfifo", parent).CombinedOutput(); err != nil {
			t.Fatal("create actual FIFO", err, string(output))
		}
		root, _, err := openInventoryParent(t.Context(), parent, original)
		if root != nil {
			_ = root.Close()
		}
		after, statErr := os.Lstat(parent)
		if err == nil || statErr != nil || after.Mode()&os.ModeNamedPipe == 0 {
			t.Fatal("substituted FIFO was opened or modified", err, statErr)
		}
		return
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestInventoryParentAcquisitionRejectsReplacedFIFOWithoutBlocking$")
	command.Env = append(os.Environ(), childEnv+"=1")
	output, err := command.CombinedOutput()
	if err != nil || ctx.Err() != nil {
		t.Fatal("parent acquisition blocked before its incarnation check", err, ctx.Err(), string(output))
	}
}
