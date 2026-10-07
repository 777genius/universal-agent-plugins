package installer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tailscale/hujson"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/opencode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// The old ConfigV2 exclusion makes Prepare red. Once admitted, the old provider
// filter/flat namespace/receipt codec makes these real facade effects red:
// nested stdio+HTTP delivery, persisted ownership, update and host-free removal.
// These fixtures execute only the bounded --version probe, never an MCP host.
func TestOpenCodeV2PublicLifecycle(t *testing.T) {
	for _, ext := range []string{"json", "jsonc"} {
		t.Run(ext, func(t *testing.T) {
			root := openCodeTestRoot(t)
			executable := buildOpenCodeTarget(t, filepath.Join(root, "v2"), "2.0.21", "ok")
			req := openCodeRequest(t, root, executable)
			req.InstallationID = "TEST-v2-lifecycle" // identity-fix lane is integrated separately
			writeV2MCP(t, req.PackageRoot, "https://fixture.invalid/first")
			foreign := `"foreign":{"type":"remote","url":"https://foreign.invalid/mcp","disabled":true}`
			body := "{\"mcp\":{\"servers\":{" + foreign + "}},\"theme\":\"fixture\"}\n"
			if ext == "jsonc" {
				body = "{ // keep this user comment\n\"mcp\":{\"servers\":{" + foreign + "}},\"theme\":\"fixture\",}\n"
			}
			mustWriteV2(t, filepath.Join(req.ClientConfigRoot, "opencode."+ext), []byte(body))
			probes := 0
			engine := openCodeEngine(t, Config{StateRoot: filepath.Join(root, "state"), HelperExecutable: executable, OpenCodeProbeEnvironment: []string{"PATH="}, OpenCodeProbe: func(ctx context.Context, target clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
				probes++
				return clientdetect.ProbeOpenCodeTarget(ctx, target)
			}})
			first, err := engine.Prepare(testCtx(t), req)
			if err != nil {
				t.Fatal(err)
			}
			defer closeOpenCodeTestHandle(t, first)
			result, err := engine.Apply(testCtx(t), first, Decision{Confirmed: true})
			if err != nil || result.Outcome != OutcomeCompleted {
				t.Fatalf("install: %+v %v", result, err)
			}
			assertV2Delivery(t, engine, req.ClientConfigRoot, ext, "https://fixture.invalid/first", true)
			if probes != 2 {
				t.Fatalf("desired prepare/apply probes=%d", probes)
			}
			req.Operation, req.InstallationID = OpUpdate, result.InstallationID
			writeV2MCP(t, req.PackageRoot, "https://fixture.invalid/updated")
			skillSource := filepath.Join(req.PackageRoot, "skills", "sample-notify", "SKILL.md")
			skill, err := os.ReadFile(skillSource)
			if err != nil {
				t.Fatal(err)
			}
			mustWriteV2(t, skillSource, append(skill, []byte("Updated fixture skill.\n")...))
			update, err := engine.Prepare(testCtx(t), req)
			if err != nil {
				t.Fatal(err)
			}
			defer closeOpenCodeTestHandle(t, update)
			if result, err = engine.Apply(testCtx(t), update, Decision{Confirmed: true}); err != nil || result.Outcome != OutcomeCompleted {
				t.Fatalf("update: %+v %v", result, err)
			}
			assertV2Delivery(t, engine, req.ClientConfigRoot, ext, "https://fixture.invalid/updated", true)
			skill, err = os.ReadFile(filepath.Join(req.ClientConfigRoot, "skills", "sample-notify", "SKILL.md"))
			if err != nil || !bytes.Contains(skill, []byte("Updated fixture skill.")) {
				t.Fatal("skill update was lost")
			}
			if probes != 4 {
				t.Fatalf("desired update probes=%d", probes)
			}
			// Reopen from persisted receipts, remove the host, and use a port that
			// fails the test on every attempted invocation. Read-only/owned effects
			// must not derive the codec from a current executable or PATH.
			if err := os.Remove(executable); err != nil {
				t.Fatal(err)
			}
			engine = openCodeEngine(t, Config{StateRoot: filepath.Join(root, "state"), Runner: forbiddenV2Runner{t}, EnableNativeObserver: true, OpenCodeProbe: func(context.Context, clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
				t.Fatal("stored ownership probed")
				return clientdetect.ProbeEvidence{}, errors.New("forbidden")
			}})
			inspection, err := engine.Inspect(testCtx(t))
			if err != nil {
				t.Fatal(err)
			}
			if _, err = engine.Recover(testCtx(t), inspection); err != nil {
				t.Fatal(err)
			}
			remove, err := engine.Prepare(testCtx(t), Request{Operation: OpRemove, ClientID: "opencode", ClientConfigRoot: req.ClientConfigRoot, InstallationID: req.InstallationID})
			if err != nil {
				t.Fatal(err)
			}
			defer closeOpenCodeTestHandle(t, remove)
			if result, err = engine.Apply(testCtx(t), remove, Decision{Confirmed: true}); err != nil || result.Outcome != OutcomeCompleted {
				t.Fatalf("remove: %+v %v", result, err)
			}
			assertV2Delivery(t, engine, req.ClientConfigRoot, ext, "", false)
		})
	}
}

func writeV2MCP(t *testing.T, root, url string) {
	t.Helper()
	body := `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"sample-notify":{"type":"stdio","command":"./bin/probe","args":["--fixture"],"env":{}},"remote":{"type":"streamable-http","url":"` + url + `","headers":{"X-Test":"inert"}}}}`
	mustWriteV2(t, filepath.Join(root, "mcp.json"), []byte(body))
}
func mustWriteV2(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
}
func assertV2Delivery(t *testing.T, engine *Engine, root, ext, url string, present bool) {
	t.Helper()
	paths := nativeconfig.Paths{JSON: filepath.Join(root, "opencode.json"), JSONC: filepath.Join(root, "opencode.jsonc")}
	body, err := os.ReadFile(filepath.Join(root, "opencode."+ext))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte(`"foreign":{"type":"remote","url":"https://foreign.invalid/mcp","disabled":true}`)) || ext == "jsonc" && !strings.Contains(string(body), "keep this user comment") {
		t.Fatalf("foreign bytes/comment lost: %s", body)
	}
	standard, err := hujson.Standardize(body)
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Theme string
		MCP   struct {
			Servers map[string]struct {
				Type        string
				Command     []string
				Environment map[string]string
				Disabled    bool
				URL         string
				Headers     map[string]string
			}
		}
	}
	if err := json.Unmarshal(standard, &config); err != nil {
		t.Fatal(err)
	}
	if config.Theme != "fixture" {
		t.Fatal("foreign root field lost")
	}
	if present {
		local, remote := config.MCP.Servers["sample-notify"], config.MCP.Servers["remote"]
		if local.Type != "local" || len(local.Command) != 2 || !filepath.IsAbs(local.Command[0]) || local.Command[1] != "--fixture" || local.Disabled || local.Environment["PLUGIN_ROOT"] == "" || local.Environment["PLUGIN_DATA"] == "" || remote.Type != "remote" || remote.URL != url || remote.Disabled || remote.Headers["X-Test"] != "inert" {
			t.Fatal("incorrect V2 native transport projection")
		}
	}
	rawState, err := os.ReadFile(engine.cfg.StateFile)
	if err != nil {
		t.Fatal(err)
	}
	var state domain.StateFileV2
	if err := json.Unmarshal(rawState, &state); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, installation := range state.Installations {
		for _, binding := range installation.Clients {
			projection, err := opencode.ReadOpenCodeProjection(binding.TargetLocator)
			if err != nil {
				t.Fatal(err)
			}
			if projection.Version != 2 || string(projection.Dialect) != "opencode_v2" {
				t.Fatal("desired projection lost prepared dialect")
			}
			if err := opencode.VerifyOpenCodeNativeObjects(root, binding.TargetLocator, binding.NativeObjects, nativeconfig.New(), false); err != nil {
				t.Fatal(err)
			}
			components := []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "sample-notify", Support: domain.SupportPrepared}, {Kind: domain.ComponentMCPServer, Name: "remote", Support: domain.SupportPrepared}}
			adapter := opencode.New()
			finding, err := adapter.InspectNativeRegistry(testCtx(t), clients.Env{Runner: forbiddenV2Runner{t}, NativeConfig: nativeconfig.New()}, domain.DetectedClient{ClientID: domain.ClientOpenCode, ConfigRoot: root}, domain.DeliveryPlan{NativeRegistryRoot: root, Components: components}, &binding)
			if err != nil || finding != clients.RegistryExpected {
				t.Fatalf("stored registry inspect: %v %v", finding, err)
			}
			for _, object := range binding.NativeObjects {
				if !strings.HasPrefix(object.ObjectID, "opencode-mcp:") {
					continue
				}
				codec, mcp, err := nativeconfig.OpenCodeCodecForKind(object.Kind)
				if err != nil || !mcp || codec != nativeconfig.CodecOpenCodeV2 {
					t.Fatalf("stored receipt dialect: %s %v", object.Kind, err)
				}
				receipt := nativeconfig.Receipt{Version: "1", Path: object.Path, Codec: codec, Name: object.LogicalName, Digest: object.ManagedDigest}
				if object.ObjectID != "opencode-mcp:"+object.LogicalName {
					t.Fatalf("logical identity: %s", object.ObjectID)
				}

				exists, owned, err := nativeconfig.New().Inspect(paths, receipt.Codec, receipt.Name, &receipt)
				if err != nil || !exists || !owned {
					t.Fatalf("receipt: %+v %t %t %v", receipt, exists, owned, err)
				}
				count++
			}
		}
	}
	if present && count != 2 || !present && count != 0 {
		t.Fatalf("persisted MCP objects=%d", count)
	}
	for _, name := range []string{"sample-notify", "remote"} {
		exists, _, err := nativeconfig.New().Inspect(paths, nativeconfig.CodecOpenCodeV2, name, nil)
		if err != nil || exists != present {
			t.Fatalf("entry %s: %t %v", name, exists, err)
		}
	}
	_, err = os.Stat(filepath.Join(root, "skills", "sample-notify", "SKILL.md"))
	if present && err != nil || !present && !os.IsNotExist(err) {
		t.Fatalf("skill presence: %v", err)
	}
}

// Disabled equal-name entries still reserve ownership. Normalized namespace
// collisions, edited source and incompatible roots must stop before effects.
func TestOpenCodeV2FacadeRefusesForeignAndEditedOwnership(t *testing.T) {
	for _, scenario := range []string{"equal-disabled", "namespace", "v1-root", "mixed-root", "edited"} {
		t.Run(scenario, func(t *testing.T) {
			root := openCodeTestRoot(t)
			executable := buildOpenCodeTarget(t, filepath.Join(root, "v2"), "2.0.21", "ok")
			req := openCodeRequest(t, root, executable)
			req.InstallationID = "TEST-refusal"
			engine := openCodeEngine(t, Config{StateRoot: filepath.Join(root, "state"), HelperExecutable: executable, OpenCodeProbeEnvironment: []string{"PATH="}})
			path := filepath.Join(req.ClientConfigRoot, "opencode.json")
			switch scenario {
			case "equal-disabled":
				mustWriteV2(t, path, []byte(`{"mcp":{"servers":{"sample-notify":{"type":"remote","url":"https://foreign.invalid","disabled":true}}}}`))
			case "namespace":
				mustWriteV2(t, path, []byte(`{"mcp":{"servers":{"sample-notify_tool":{"type":"remote","url":"https://foreign.invalid"}}}}`))
			case "v1-root":
				mustWriteV2(t, path, []byte(`{"mcp":{"foreign":{"type":"remote","url":"https://foreign.invalid"}}}`))
			case "mixed-root":
				mustWriteV2(t, path, []byte(`{"mcp":{"servers":{},"foreign":{"type":"remote","url":"https://foreign.invalid"}}}`))
			case "edited":
				first, err := engine.Prepare(testCtx(t), req)
				if err != nil {
					t.Fatal(err)
				}
				result, err := engine.Apply(testCtx(t), first, Decision{Confirmed: true})
				closeOpenCodeTestHandle(t, first)
				if err != nil {
					t.Fatal(err)
				}
				req.Operation, req.InstallationID = OpUpdate, result.InstallationID
				body, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				edited := strings.Replace(string(body), `"disabled":false`, `"disabled":true`, 1)
				if edited == string(body) {
					edited = strings.Replace(string(body), `"disabled": false`, `"disabled": true`, 1)
				}
				if edited == string(body) {
					t.Fatal("edited fixture unchanged")
				}
				mustWriteV2(t, path, []byte(edited))
			}
			before := v2EffectSnapshot(t, engine)
			handle, err := engine.Prepare(testCtx(t), req)
			if handle != nil {
				closeOpenCodeTestHandle(t, handle)
			}
			if err == nil {
				t.Fatal("unsafe proposal accepted")
			}
			if (scenario == "v1-root" || scenario == "mixed-root") && !errors.Is(err, nativeconfig.ErrNativeMigrationRequired) {
				t.Fatalf("migration error: %v", err)
			}
			after := v2EffectSnapshot(t, engine)
			if before != after {
				t.Fatal("refusal changed config/skill/package/state")
			}
		})
	}
}

// Same logical ObjectIDs cannot turn codec changes into two ordinary Apply
// calls. The closed same-ID transition now succeeds; deleting all declarations
// retains the existing cross-profile fence.
func TestOpenCodeFacadeCrossDialectClosedTransition(t *testing.T) {
	for _, versions := range [][2]string{{"1.18.34", "2.0.21"}, {"2.0.21", "1.18.34"}} {
		for _, empty := range []bool{false, true} {
			t.Run(versions[0]+"-to-"+versions[1]+map[bool]string{true: "-empty"}[empty], func(t *testing.T) {
				root := openCodeTestRoot(t)
				source := buildOpenCodeTarget(t, filepath.Join(root, "source"), versions[0], "ok")
				target := buildOpenCodeTarget(t, filepath.Join(root, "target"), versions[1], "ok")
				req := openCodeRequest(t, root, source)
				req.InstallationID = "TEST-crossdialect"
				engine := openCodeEngine(t, Config{StateRoot: filepath.Join(root, "state"), HelperExecutable: source, OpenCodeProbeEnvironment: []string{"PATH="}})
				first, err := engine.Prepare(testCtx(t), req)
				if err != nil {
					t.Fatal(err)
				}
				result, err := engine.Apply(testCtx(t), first, Decision{Confirmed: true})
				closeOpenCodeTestHandle(t, first)
				if err != nil {
					t.Fatal(err)
				}
				req.Operation, req.InstallationID, req.ClientExecutable = OpUpdate, result.InstallationID, target
				if empty {
					if err := os.Remove(filepath.Join(req.PackageRoot, "mcp.json")); err != nil {
						t.Fatal(err)
					}
					if err := os.RemoveAll(filepath.Join(req.PackageRoot, "skills")); err != nil {
						t.Fatal(err)
					}
					req.RequiredComponents = nil
				}
				before := v2EffectSnapshot(t, engine)
				handle, err := engine.Prepare(testCtx(t), req)
				if !empty {
					if err != nil {
						t.Fatal(err)
					}
					if _, err := engine.Apply(testCtx(t), handle, Decision{Confirmed: true}); err != nil {
						t.Fatal(err)
					}
					closeOpenCodeTestHandle(t, handle)
					state, err := engine.store.Load()
					if err != nil {
						t.Fatal(err)
					}
					for _, b := range state.Installations[0].Clients {
						if b.NativeActivationAttempt != "" {
							t.Fatal("transition attempt retained")
						}
						if err := opencode.VerifyOpenCodeNativeObjects(req.ClientConfigRoot, b.TargetLocator, b.NativeObjects, nativeconfig.New(), false); err != nil {
							t.Fatal(err)
						}
					}
					return
				}
				if handle != nil {
					closeOpenCodeTestHandle(t, handle)
				}
				if !errors.Is(err, nativeconfig.ErrNativeMigrationRequired) {
					t.Fatalf("crosscodec proposal: %v", err)
				}
				if before != v2EffectSnapshot(t, engine) {
					t.Fatal("crosscodec refusal changed config/skills/package/state")
				}
			})
		}
	}
}
func v2EffectSnapshot(t *testing.T, e *Engine) string {
	t.Helper()
	values := map[string]string{}
	for _, root := range []string{e.cfg.ManagedRoot, e.cfg.StateRoot, filepath.Join(filepath.Dir(e.cfg.StateRoot), "config")} {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if os.IsNotExist(err) {
				return nil
			}
			if err != nil {
				return err
			}
			// The cooperating operation lock records process ownership. Its trace
			// is infrastructure, not an installed package/skill/config/state effect.
			if entry.IsDir() || path == e.cfg.LockFile {
				return nil
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			values[path] = string(body)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	body, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// A process runner is deliberately unusable even when native observation is
// enabled. Receipt/root operations cannot execute a helper or selected host.
type forbiddenV2Runner struct{ t *testing.T }

func (r forbiddenV2Runner) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	r.t.Helper()
	r.t.Fatal("stored ownership invoked process runner")
	return legacyports.CommandResult{}, errors.New("forbidden")
}

// A desired projection is read from the actual materialized facade output.
// Unknown schema/dialect/fields and trailing data must not become usable native
// ownership. Legacy version 1 still means V1 without guessing config bytes.
func TestOpenCodeProjectionClosedDialectDecoder(t *testing.T) {
	root := openCodeTestRoot(t)
	executable := buildOpenCodeTarget(t, filepath.Join(root, "v2"), "2.0.21", "ok")
	req := openCodeRequest(t, root, executable)
	req.InstallationID = "TEST-projection"
	engine := openCodeEngine(t, Config{StateRoot: filepath.Join(root, "state"), HelperExecutable: executable, OpenCodeProbeEnvironment: []string{"PATH="}})
	handle, err := engine.Prepare(testCtx(t), req)
	if err != nil {
		t.Fatal(err)
	}
	defer closeOpenCodeTestHandle(t, handle)
	if _, err := engine.Apply(testCtx(t), handle, Decision{Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(handle.Plan().TargetPath, opencode.OpenCodeProjectionFile))
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []string{"unknown-version", "missing-dialect", "unknown-dialect", "unknown-field", "unknown-server-field", "trailing", "legacy-v1"} {
		t.Run(change, func(t *testing.T) {
			var fields map[string]any
			if err := json.Unmarshal(body, &fields); err != nil {
				t.Fatal(err)
			}
			switch change {
			case "unknown-version":
				fields["version"] = 99
			case "missing-dialect":
				delete(fields, "dialect")
			case "unknown-dialect":
				fields["dialect"] = "opencode_v3"
			case "unknown-field":
				fields["future"] = true
			case "unknown-server-field":
				for _, value := range fields["mcp_servers"].(map[string]any) {
					value.(map[string]any)["future"] = true
				}
			case "legacy-v1":
				fields["version"] = 1
				delete(fields, "dialect")
			}
			edited, err := json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			if change == "trailing" {
				edited = append(edited, []byte(" {}")...)
			}
			fixture := t.TempDir()
			mustWriteV2(t, filepath.Join(fixture, opencode.OpenCodeProjectionFile), edited)
			projection, err := opencode.ReadOpenCodeProjection(fixture)
			if change != "legacy-v1" {
				if err == nil {
					t.Fatal("untrusted projection accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			// Select the installed MCP component; an empty package/plan requests
			// the metadata-only no-effect contract instead of legacy ownership.
			plan := domain.DeliveryPlan{Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "sample-notify", Support: domain.SupportPrepared}}}
			objects, err := opencode.BuildOpenCodeNativeObjects(fixture, handle.envelope, plan)
			if err != nil {
				t.Fatal(err)
			}
			if len(objects) != 1 {
				t.Fatal("legacy MCP ownership missing")
			}
			object := objects[0]
			if object.ObjectID != "opencode-mcp:sample-notify" || object.LogicalName != "sample-notify" {
				t.Fatalf("legacy MCP ownership identity changed: %+v", object)
			}
			codec, mcp, err := nativeconfig.OpenCodeCodecForKind(object.Kind)
			if err != nil || !mcp {
				t.Fatal(err)
			}
			desired, err := nativeconfig.DesiredReceipt(projection.ConfigPath, nativeconfig.CodecOpenCode, object.LogicalName, projection.MCPServers[object.LogicalName], nativeconfig.Placeholders{PackageRoot: projection.PackageRoot, DataRoot: projection.DataRoot})
			if err != nil || codec != desired.Codec || object.ManagedDigest != desired.Digest {
				t.Fatal("legacy projection changed V1 digest domain")
			}
		})
	}
}

// Prepared plans do not grant adoption authority if another writer creates a
// foreign entry or incompatible root before Apply. This is an actual facade
// replay of preflight, with a byte snapshot of every managed/config/state file.
func TestOpenCodeV2ApplyRepeatsForeignPreflight(t *testing.T) {
	for _, scenario := range []string{"equal-disabled", "v1-root"} {
		t.Run(scenario, func(t *testing.T) {
			root := openCodeTestRoot(t)
			executable := buildOpenCodeTarget(t, filepath.Join(root, "v2"), "2.0.21", "ok")
			req := openCodeRequest(t, root, executable)
			req.InstallationID = "TEST-late-foreign"
			engine := openCodeEngine(t, Config{StateRoot: filepath.Join(root, "state"), HelperExecutable: executable, OpenCodeProbeEnvironment: []string{"PATH="}})
			handle, err := engine.Prepare(testCtx(t), req)
			if err != nil {
				t.Fatal(err)
			}
			defer closeOpenCodeTestHandle(t, handle)
			body := `{"mcp":{"servers":{"sample-notify":{"type":"remote","url":"https://foreign.invalid","disabled":true}}}}`
			if scenario == "v1-root" {
				body = `{"mcp":{"foreign":{"type":"remote","url":"https://foreign.invalid"}}}`
			}
			mustWriteV2(t, filepath.Join(req.ClientConfigRoot, "opencode.json"), []byte(body))
			before := v2EffectSnapshot(t, engine)
			_, err = engine.Apply(testCtx(t), handle, Decision{Confirmed: true})
			if err == nil {
				t.Fatal("late foreign config adopted")
			}
			if scenario == "v1-root" && !errors.Is(err, nativeconfig.ErrNativeMigrationRequired) {
				t.Fatalf("migration: %v", err)
			}
			if before != v2EffectSnapshot(t, engine) {

				t.Fatal("late refusal changed config/skills/package/state")
			}
		})
	}
}

// No selected host turns SSE into a supported Streamable HTTP declaration.
func TestOpenCodeV2FacadeRejectsSSE(t *testing.T) {
	root := openCodeTestRoot(t)
	executable := buildOpenCodeTarget(t, filepath.Join(root, "v2"), "2.0.21", "ok")
	req := openCodeRequest(t, root, executable)
	req.InstallationID = "TEST-sse"
	mustWriteV2(t, filepath.Join(req.PackageRoot, "mcp.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"sample-notify":{"type":"sse","url":"https://fixture.invalid/sse"}}}`))
	engine := openCodeEngine(t, Config{StateRoot: filepath.Join(root, "state"), HelperExecutable: executable, OpenCodeProbeEnvironment: []string{"PATH="}})
	before := v2EffectSnapshot(t, engine)
	handle, err := engine.Prepare(testCtx(t), req)
	if handle != nil {
		closeOpenCodeTestHandle(t, handle)
	}
	if !errors.Is(err, errOpenCodeTransportUnsupported) {
		t.Fatalf("SSE: %v", err)
	}
	if before != v2EffectSnapshot(t, engine) {
		t.Fatal("SSE refusal caused effects")
	}
}

// Unknown claimed OpenCode kinds cannot be silently filtered out by stored
// ownership verification, registry inspection, removal, or desired Prepare.
func TestOpenCodeV2UnknownStoredKindFailsClosed(t *testing.T) {
	root := openCodeTestRoot(t)
	executable := buildOpenCodeTarget(t, filepath.Join(root, "v2"), "2.0.21", "ok")
	req := openCodeRequest(t, root, executable)
	req.InstallationID = "TEST-unknown-kind"
	engine := openCodeEngine(t, Config{StateRoot: filepath.Join(root, "state"), HelperExecutable: executable, OpenCodeProbeEnvironment: []string{"PATH="}})
	first, err := engine.Prepare(testCtx(t), req)
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.Apply(testCtx(t), first, Decision{Confirmed: true})
	closeOpenCodeTestHandle(t, first)
	if err != nil {
		t.Fatal(err)
	}
	state, err := engine.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	var binding domain.ClientBinding
	for key, value := range state.Installations[0].Clients {
		for i, object := range value.NativeObjects {
			if object.ObjectID == "opencode-mcp:sample-notify" {
				value.NativeObjects[i].Kind = "opencode_v3_global_mcp_server"
			}
		}
		state.Installations[0].Clients[key] = value
		binding = value
	}
	if err := engine.store.Save(state); err != nil {
		t.Fatal(err)
	}
	before := v2EffectSnapshot(t, engine)
	if err := opencode.VerifyOpenCodeNativeObjects(req.ClientConfigRoot, binding.TargetLocator, binding.NativeObjects, nativeconfig.New(), false); err == nil {
		t.Fatal("verify skipped unknown kind")
	}
	if err := opencode.ApplyOpenCodeNative(req.ClientConfigRoot, "", binding.NativeObjects, nil); err == nil {
		t.Fatal("remove skipped unknown kind")
	}
	plan := domain.DeliveryPlan{NativeRegistryRoot: req.ClientConfigRoot, Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "sample-notify", Support: domain.SupportPrepared}}}
	if _, err := opencode.InspectOpenCodeRegistry(plan, &binding, nativeconfig.New()); err == nil {
		t.Fatal("inspect skipped unknown kind")
	}
	engine = openCodeEngine(t, Config{StateRoot: filepath.Join(root, "state"), HelperExecutable: executable, OpenCodeProbe: func(context.Context, clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
		t.Fatal("unknown stored kind reached desired probe")
		return clientdetect.ProbeEvidence{}, errors.New("forbidden")
	}})
	req.Operation, req.InstallationID = OpUpdate, result.InstallationID
	next, err := engine.Prepare(testCtx(t), req)
	if next != nil {
		closeOpenCodeTestHandle(t, next)
	}
	if err == nil {
		t.Fatal("prepare ignored unknown kind")
	}
	if before != v2EffectSnapshot(t, engine) {
		t.Fatal("unknown kind refusal changed effects")
	}
	// Removing every desired native declaration must not reclassify an unknown
	// stored receipt as inert metadata or proceed to an executable requirement.
	if err := os.Remove(filepath.Join(req.PackageRoot, "mcp.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(req.PackageRoot, "skills")); err != nil {
		t.Fatal(err)
	}
	req.RequiredComponents, req.ClientExecutable = nil, ""
	next, err = engine.Prepare(testCtx(t), req)
	if next != nil {
		closeOpenCodeTestHandle(t, next)
	}
	if err == nil || !strings.Contains(err.Error(), "unsupported OpenCode native object kind") {
		t.Fatalf("metadata proposal bypassed unknown stored kind: %v", err)
	}
	if before != v2EffectSnapshot(t, engine) {
		t.Fatal("metadata unknown-kind refusal changed effects")
	}
}

// Desired bytes cannot select a host dialect when prepared authority is absent.
// The input comes from a real facade install; no profile/probe override is used.
func TestOpenCodeDesiredProjectionRequiresPreparedProfile(t *testing.T) {
	root := openCodeTestRoot(t)
	executable := buildOpenCodeTarget(t, filepath.Join(root, "v2"), "2.0.21", "ok")
	req := openCodeRequest(t, root, executable)
	req.InstallationID = "TEST-no-fallback"
	engine := openCodeEngine(t, Config{StateRoot: filepath.Join(root, "state"), HelperExecutable: executable, OpenCodeProbeEnvironment: []string{"PATH="}})
	handle, err := engine.Prepare(testCtx(t), req)
	if err != nil {
		t.Fatal(err)
	}
	defer closeOpenCodeTestHandle(t, handle)
	if _, err := engine.Apply(testCtx(t), handle, Decision{Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	plan := domain.DeliveryPlan{NativeRegistryRoot: req.ClientConfigRoot, ActivePath: handle.Plan().TargetPath}
	fixture := t.TempDir()
	before := v2EffectSnapshot(t, engine)
	if err := opencode.ProjectOpenCodeNative(fixture, handle.envelope, plan, ""); err == nil {
		t.Fatal("missing authority selected a desired dialect")
	}
	if _, err := os.Lstat(filepath.Join(fixture, opencode.OpenCodeProjectionFile)); !os.IsNotExist(err) {
		t.Fatal("unqualified desired projection written")
	}
	if before != v2EffectSnapshot(t, engine) {
		t.Fatal("missing authority changed native/config/state effects")
	}
}
