package pluginkitairepo_test

// This suite drives the real OpenCode CLI through UAP's actual install route
// (global opencode.json under <XDG_CONFIG_HOME>/opencode, plus global
// skills). UAP also supports opencode.jsonc for this same route, but this
// checkpoint only exercises the .json path -- that is a real, currently
// unproven gap, not a claim that both are covered. It is opt-in and uses
// only disposable HOME/XDG roots and a fresh project directory; it never
// touches the invoking user's real OpenCode config. `opencode debug config`
// proves effective config, not a handshake or tool call -- that distinction
// is preserved in the recorded evidence, per the O2 contract.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

const openCodeNativeCommandTimeout = 30 * time.Second

type openCodeNativeStage struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type openCodeNativeFixture struct {
	Root, Home, XDGConfig, XDGData, XDGCache, XDGState, Project, PackageRoot, StateHome string
	transcriptSeq                                                                       int
	transcriptSHA256                                                                    map[string]string
}

func newOpenCodeNativeFixture(t *testing.T) *openCodeNativeFixture {
	t.Helper()
	root, err := os.MkdirTemp("", "uap-opencode-native-evidence-")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("opencode native evidence: %s", root)
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	f := &openCodeNativeFixture{
		Root: root, Home: filepath.Join(root, "home"), XDGConfig: filepath.Join(root, "xdg-config"),
		XDGData: filepath.Join(root, "xdg-data"), XDGCache: filepath.Join(root, "xdg-cache"), XDGState: filepath.Join(root, "xdg-state"),
		Project: filepath.Join(root, "project"), PackageRoot: filepath.Join(root, "fixture"), StateHome: filepath.Join(root, "installer-state"),
		transcriptSHA256: map[string]string{},
	}
	for _, p := range []string{f.Home, f.XDGConfig, f.XDGData, f.XDGCache, f.XDGState, f.Project, f.PackageRoot, f.StateHome, filepath.Join(root, "tmp")} {
		if err := os.MkdirAll(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

// env returns a bounded, isolated environment for both the `opencode` CLI and
// the `agentplugins` installer: disposable HOME/XDG roots, no inherited
// ambient config, OPENCODE_PURE disables any implicit project/global merge
// that could hide the owned entry.
func (f *openCodeNativeFixture) env(clientDir string) []string {
	return []string{
		"HOME=" + f.Home,
		"XDG_CONFIG_HOME=" + f.XDGConfig,
		"XDG_DATA_HOME=" + f.XDGData,
		"XDG_CACHE_HOME=" + f.XDGCache,
		"XDG_STATE_HOME=" + f.XDGState,
		"AGENTPLUGINS_HOME=" + f.StateHome,
		"TMPDIR=" + filepath.Join(f.Root, "tmp"),
		"OPENCODE_PURE=1",
		"PATH=" + clientDir + ":/usr/bin:/bin",
		"LANG=en_US.UTF-8",
	}
}

func (f *openCodeNativeFixture) record(t *testing.T, label string, out []byte) {
	t.Helper()
	f.transcriptSeq++
	name := fmt.Sprintf("%02d-%s.log", f.transcriptSeq, label)
	path := filepath.Join(f.Root, name)
	if err := os.WriteFile(path, out, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(out)
	f.transcriptSHA256[name] = hex.EncodeToString(sum[:])
}

func openCodeSHA256(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func openCodeRunInstaller(t *testing.T, f *openCodeNativeFixture, installer, clientDir, label string, args ...string) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), openCodeNativeCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, installer, append(args, "--format", "json")...)
	cmd.Env = f.env(clientDir)
	cmd.Dir = f.Root
	out, err := cmd.CombinedOutput()
	f.record(t, label, out)
	if ctx.Err() != nil {
		t.Fatalf("installer %v timed out after %s:\n%s", args, openCodeNativeCommandTimeout, out)
	}
	if err != nil {
		if _, isExit := err.(*exec.ExitError); !isExit {
			t.Fatalf("run installer %v: %v\n%s", args, err, out)
		}
	}
	line := openCodeLastJSONLine(out)
	var decoded map[string]any
	if line != "" {
		if jsonErr := json.Unmarshal([]byte(line), &decoded); jsonErr != nil {
			t.Fatalf("decode installer output for %v: %v\n%s", args, jsonErr, out)
		}
	}
	return decoded
}

func openCodeRunInstallerExpectError(t *testing.T, f *openCodeNativeFixture, installer, clientDir, label string, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), openCodeNativeCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, installer, append(args, "--format", "json")...)
	cmd.Env = f.env(clientDir)
	cmd.Dir = f.Root
	out, err := cmd.CombinedOutput()
	f.record(t, label, out)
	if ctx.Err() != nil {
		t.Fatalf("installer %v timed out after %s (expected a clean refusal, not a hang):\n%s", args, openCodeNativeCommandTimeout, out)
	}
	return string(out), err
}

func openCodeLastJSONLine(out []byte) string {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "{") {
			return trimmed
		}
	}
	return ""
}

// openCodeAssertMutated decodes an installer JSON result's single-target
// mutated flag. A no_change field alone does not prove the physical write
// was skipped; mutated is the field the kernel actually sets.
func openCodeAssertMutated(t *testing.T, result map[string]any, want bool) {
	t.Helper()
	data, _ := result["data"].(map[string]any)
	targets, _ := data["targets"].([]any)
	if len(targets) != 1 {
		t.Fatalf("expected exactly one target in result: %+v", result)
	}
	target, _ := targets[0].(map[string]any)
	output, _ := target["output"].(map[string]any)
	resultObj, ok := output["result"].(map[string]any)
	if !ok {
		t.Fatalf("result object missing from installer output: %+v", output)
	}
	mutated, ok := resultObj["mutated"].(bool)
	if !ok {
		t.Fatalf("mutated field missing from result: %+v", resultObj)
	}
	if mutated != want {
		t.Fatalf("mutated = %v, want %v: %+v", mutated, want, resultObj)
	}
}

// openCodeAssertComponentUnsupported reads the installer's own plan output
// and requires the named component to be explicitly marked unsupported,
// rather than inferring non-support only from its absence in the effective
// config (which could also mean it silently failed to project for an
// unrelated reason).
func openCodeAssertComponentUnsupported(t *testing.T, result map[string]any, componentName, wantReason string) {
	t.Helper()
	data, _ := result["data"].(map[string]any)
	targets, _ := data["targets"].([]any)
	if len(targets) != 1 {
		t.Fatalf("expected exactly one target in result: %+v", result)
	}
	target, _ := targets[0].(map[string]any)
	output, _ := target["output"].(map[string]any)
	resultObj, _ := output["result"].(map[string]any)
	plan, _ := resultObj["plan"].(map[string]any)
	components, _ := plan["components"].([]any)
	for _, raw := range components {
		component, _ := raw.(map[string]any)
		if component["name"] != componentName {
			continue
		}
		if component["support"] != "unsupported" {
			t.Fatalf("component %q support = %v, want unsupported: %+v", componentName, component["support"], component)
		}
		// The specific reason distinguishes "declared SSE, this client
		// doesn't support SSE" from a generic not-supported-at-all reason;
		// checking only "support" cannot tell those apart.
		if component["reason"] != wantReason {
			t.Fatalf("component %q reason = %v, want %q: %+v", componentName, component["reason"], wantReason, component)
		}
		return
	}
	t.Fatalf("component %q not present in installer's own plan output: %+v", componentName, components)
}

// openCodeAssertCompleted requires both the top-level result marker and the
// nested data.status to indicate genuine success. data.status can carry
// error-path values (for example "managed_committed_activation_failed" or a
// bare "managed_committed" set together with a non-nil error) that must not
// be mistaken for success.
func openCodeAssertCompleted(t *testing.T, label string, result map[string]any) map[string]any {
	t.Helper()
	if result == nil {
		t.Fatalf("%s: no decoded installer result", label)
	}
	if result["result"] != "success" {
		t.Fatalf("%s: top-level result = %v, want success: %+v", label, result["result"], result)
	}
	data, _ := result["data"].(map[string]any)
	if data == nil || data["status"] != "completed" {
		t.Fatalf("%s did not complete: %+v", label, result)
	}
	return data
}

// openCodePhysicalArtifactID extracts the physical_artifact_id from an
// installer JSON result, which names the managed package directory under
// <AGENTPLUGINS_HOME>/managed/clients/opencode/.
// openCodeAssertJSONDataStatus parses the last JSON line of a raw CLI
// transcript (as produced by openCodeRunInstallerExpectError, which returns
// unparsed text since it may not be valid JSON on some failure paths) and
// requires data.status to equal want exactly.
func openCodeAssertJSONDataStatus(t *testing.T, rawOutput, want string) {
	t.Helper()
	line := openCodeLastJSONLine([]byte(rawOutput))
	if line == "" {
		t.Fatalf("no JSON line found in output:\n%s", rawOutput)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(line), &decoded); err != nil {
		t.Fatalf("decode output: %v\n%s", err, rawOutput)
	}
	data, _ := decoded["data"].(map[string]any)
	if data == nil || data["status"] != want {
		t.Fatalf("data.status = %v, want %q: %+v", data["status"], want, decoded)
	}
}

func openCodePhysicalArtifactID(t *testing.T, result map[string]any) string {
	t.Helper()
	data, _ := result["data"].(map[string]any)
	targets, _ := data["targets"].([]any)
	if len(targets) != 1 {
		t.Fatalf("expected exactly one target in result: %+v", result)
	}
	target, _ := targets[0].(map[string]any)
	output, _ := target["output"].(map[string]any)
	resultObj, _ := output["result"].(map[string]any)
	plan, _ := resultObj["plan"].(map[string]any)
	id, _ := plan["physical_artifact_id"].(string)
	if id == "" {
		t.Fatalf("physical_artifact_id missing from result: %+v", result)
	}
	return id
}

func openCodeVersionString(t *testing.T, f *openCodeNativeFixture, client string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), openCodeNativeCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, client, "--version")
	cmd.Env = f.env(filepath.Dir(client))
	cmd.Dir = f.Home
	out, err := cmd.CombinedOutput()
	f.record(t, "opencode-version", out)
	if err != nil {
		t.Fatalf("opencode --version: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// openCodeDebugConfig runs the real client's `debug config` and parses it as
// exact JSON (never substring matching, per the O2 contract).
func openCodeDebugConfig(t *testing.T, f *openCodeNativeFixture, client, label string) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), openCodeNativeCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, client, "debug", "config")
	cmd.Env = f.env(filepath.Dir(client))
	cmd.Dir = f.Project
	out, err := cmd.CombinedOutput()
	f.record(t, label, out)
	if ctx.Err() != nil {
		t.Fatalf("opencode debug config timed out after %s:\n%s", openCodeNativeCommandTimeout, out)
	}
	if err != nil {
		t.Fatalf("opencode debug config: %v\n%s", err, out)
	}
	var decoded map[string]any
	if jsonErr := json.Unmarshal(out, &decoded); jsonErr != nil {
		t.Fatalf("decode opencode debug config: %v\n%s", jsonErr, out)
	}
	return decoded
}

var openCodeANSIEscape = regexp.MustCompile("\x1b\\[[0-9;]*m")

// openCodeMCPList runs the real client's `mcp list`, which makes it actually
// spawn and attempt a live handshake with every configured server, and
// returns the ANSI-stripped output for per-server assertions. This is
// stronger evidence than config readback: it proves the runtime enumerated
// and attempted each entry as a separate connection.
func openCodeMCPList(t *testing.T, f *openCodeNativeFixture, client, label string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), openCodeNativeCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, client, "mcp", "list")
	cmd.Env = f.env(filepath.Dir(client))
	cmd.Dir = f.Project
	out, err := cmd.CombinedOutput()
	f.record(t, label, out)
	if ctx.Err() != nil {
		t.Fatalf("opencode mcp list timed out after %s:\n%s", openCodeNativeCommandTimeout, out)
	}
	if err != nil {
		t.Fatalf("opencode mcp list: %v\n%s", err, out)
	}
	return openCodeANSIEscape.ReplaceAllString(string(out), "")
}

// openCodeAssertMCPListAttempted requires the exact logical server name to
// appear as its own attempted-connection line in `mcp list` output (not just
// anywhere in the text), anchored on the status marker every entry line
// carries.
func openCodeAssertMCPListAttempted(t *testing.T, output, serverName string) {
	t.Helper()
	pattern := regexp.MustCompile(`(?m)^.*[✗✓].*` + regexp.QuoteMeta(serverName) + `\s*(failed|connected)`)
	if !pattern.MatchString(output) {
		t.Fatalf("opencode mcp list did not show a real attempted connection for %q:\n%s", serverName, output)
	}
}

func openCodeConfigMCP(t *testing.T, config map[string]any) map[string]any {
	t.Helper()
	mcp, ok := config["mcp"].(map[string]any)
	if !ok {
		t.Fatalf("effective config has no top-level mcp object: %+v", config)
	}
	return mcp
}

func openCodeWriteFixturePackage(t *testing.T, root, name, version string) {
	t.Helper()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(root, "skills", "demo-skill"), 0755))
	must(os.WriteFile(filepath.Join(root, "plugin.json"), []byte(fmt.Sprintf(`{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
  "name": %q,
  "version": %q,
  "description": "OpenCode native lifecycle proof fixture"
}`, name, version)), 0644))
	must(os.WriteFile(filepath.Join(root, "skills", "demo-skill", "SKILL.md"), []byte("---\nname: demo-skill\ndescription: A demo skill for native lifecycle verification.\n---\n\n# Demo Skill\n\nSay hello when asked to demo.\n"), 0644))
	// "api/server" and "api server" are the exact logical MCP keys O1 fixed:
	// they were previously rejected as invalid physical filesystem leaf names.
	// sse-server exercises OpenCode's own unsupported-transport handling
	// alongside two healthy stdio siblings, to observe (not assume) whether
	// an unsupported component is isolated or blocks the whole package.
	must(os.WriteFile(filepath.Join(root, "mcp.json"), []byte(`{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
  "mcpServers": {
    "api/server": {"type": "stdio", "command": "sh", "args": ["-c", "cat"]},
    "api server": {"type": "stdio", "command": "sh", "args": ["-c", "cat"]},
    "sse-server": {"type": "sse", "url": "http://127.0.0.1:9/sse"}
  }
}`), 0644))
}

func TestAgentpluginsOpenCodeNativeLifecycle(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_OPENCODE_NATIVE_E2E") != "1" {
		t.Skip("opt-in native client execution")
	}
	client := openCodeNativeBinary(t, "AGENTPLUGINS_OPENCODE_BIN")
	installer := openCodeNativeBinary(t, "AGENTPLUGINS_INSTALLER_BIN")
	f := newOpenCodeNativeFixture(t)
	clientDir := filepath.Dir(client)

	stages := map[string]openCodeNativeStage{}
	for _, name := range []string{
		"install", "logical_key_collision_survey", "sse_unsupported", "version_update", "same_version_refresh",
		"unchanged", "owned_repair", "foreign_config_preservation", "foreign_key_collision", "remove", "repeat_remove",
		"native_tool_call", "actual_model_skill_use", "oauth",
	} {
		stages[name] = openCodeNativeStage{Status: "not_evaluated", Reason: "not reached"}
	}
	measuredVersion := openCodeVersionString(t, f, client)
	evidence := map[string]any{
		"started_utc": time.Now().UTC().Format(time.RFC3339Nano), "client_version_pinned": os.Getenv("AGENTPLUGINS_OPENCODE_VERSION"),
		"client_version_measured": measuredVersion, "client_sha256": openCodeSHA256(t, client), "installer_sha256": openCodeSHA256(t, installer),
		"installer_base_commit": os.Getenv("AGENTPLUGINS_INSTALLER_COMMIT"), "installer_tree": os.Getenv("AGENTPLUGINS_INSTALLER_TREE"),
		"installer_patch_sha256": os.Getenv("AGENTPLUGINS_INSTALLER_PATCH_SHA256"), "acquisition": "local_directory",
		"native_surface":         "opencode debug config (effective config proof; not a handshake or tool call) plus opencode mcp list (real per-server connection attempts, still not a handshake or tool call)",
		"config_route_exercised": "opencode.json only; opencode.jsonc is not exercised by this checkpoint despite the client accepting either",
		"network_dependency":     "the installer's security scan (lintai) is fetched from GitHub on first use during add if not already cached; this run is not fully offline despite local_directory acquisition",
		"stages":                 stages,
	}
	defer func() {
		evidence["finished_utc"] = time.Now().UTC().Format(time.RFC3339Nano)
		evidence["transcript_sha256"] = f.transcriptSHA256
		body, err := json.MarshalIndent(evidence, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(f.Root, "evidence.json"), body, 0600); err != nil {
			t.Fatal(err)
		}
	}()
	if evidence["installer_base_commit"] == "" || evidence["installer_tree"] == "" {
		t.Fatal("exact installer base commit and resulting tree required")
	}
	if os.Getenv("AGENTPLUGINS_INSTALLER_PATCH_SHA256") == "" {
		t.Fatal("accepted installer patch digests required for reviewed uncommitted source identity")
	}
	if pinned := os.Getenv("AGENTPLUGINS_OPENCODE_VERSION"); pinned != "" && pinned != measuredVersion {
		t.Fatalf("measured opencode version %q does not match pinned %q", measuredVersion, pinned)
	}

	openCodeWriteFixturePackage(t, f.PackageRoot, "opencode-native-proof", "1.0.0")
	added := openCodeRunInstaller(t, f, installer, clientDir, "add-v1", "add", f.PackageRoot, "--target", "opencode")
	openCodeAssertCompleted(t, "install", added)
	stages["install"] = openCodeNativeStage{Status: "passed"}

	config := openCodeDebugConfig(t, f, client, "debug-config-v1")
	mcp := openCodeConfigMCP(t, config)
	if _, slashOK := mcp["api/server"].(map[string]any); !slashOK {
		t.Fatalf("O1's exact logical key \"api/server\" was not present: mcp=%+v", mcp)
	}
	if _, spaceOK := mcp["api server"].(map[string]any); !spaceOK {
		t.Fatalf("O1's exact logical key \"api server\" was not present: mcp=%+v", mcp)
	}
	// Config readback alone cannot prove the runtime treats the two keys as
	// distinct connections (O2 explicitly forbids declaring the colliding
	// case passed from config readback alone). Run `opencode mcp list`, which
	// makes the real client spawn and attempt a live stdio handshake with
	// each configured server, and require each exact key to appear as its
	// own attempted connection. This is ONE mcp list call against the
	// simultaneous-collision fixture (both keys installed together), parsed
	// into two per-key assertions -- real evidence of two independent
	// process spawns and JSON-RPC round-trips, but not the two genuinely
	// separate single-key installs O2's "separate api/server and api server
	// runs" wording asks for. That stronger form was not performed this
	// checkpoint; say so plainly rather than implying it was.
	mcpList := openCodeMCPList(t, f, client, "mcp-list-v1")
	openCodeAssertMCPListAttempted(t, mcpList, "api/server")
	openCodeAssertMCPListAttempted(t, mcpList, "api server")
	// What is NOT proven here, per O2's own required framing: whether the two
	// servers' *tools* collide at the runtime tool-ID level. OpenCode 1.18.29
	// sanitizes tool IDs as sanitize(server)+"_"+sanitize(tool)` with
	// non-name characters replaced by "_", so "api/server" and "api server"
	// both sanitize to the same "api_server" prefix -- a same-named tool on
	// both servers is plausible to collide at that layer. Observing that
	// requires a live model session actually selecting a tool, which this
	// checkpoint does not have a safe no-auth route to. Recorded honestly as
	// not proven rather than forced closed, per O2's explicit instruction.
	stages["logical_key_collision_survey"] = openCodeNativeStage{Status: "not_proven", Reason: "verified beyond config readback: one opencode mcp list run over the simultaneous-collision fixture independently attempted a live connection to \"api/server\" and to \"api server\" as two distinct entries (two assertions against one run, not two genuinely separate single-key installs -- O2's stronger \"separate runs\" wording was not performed). Not proven: runtime tool-ID-level collision between the two servers' tools, which OpenCode's own sanitizer (replacing non-name characters with \"_\") makes plausible for same-named tools; observing that needs a live model session, which is out of scope here -- O2 requires this exact honesty rather than declaring the colliding case passed from config alone"}

	if _, sseInstalled := mcp["sse-server"]; sseInstalled {
		stages["sse_unsupported"] = openCodeNativeStage{Status: "failed", Reason: "sse-typed component was installed despite OpenCode SSE being unsupported"}
		t.Fatalf("sse-server should not have been installed: %+v", mcp)
	}
	openCodeAssertComponentUnsupported(t, added, "sse-server", "declared_sse_not_supported_by_client")
	stages["sse_unsupported"] = openCodeNativeStage{Status: "passed", Reason: "installer's own plan marks sse-server unsupported and absent from effective config; healthy stdio siblings installed"}

	openCodeWriteFixturePackage(t, f.PackageRoot, "opencode-native-proof", "2.0.0")
	updated := openCodeRunInstaller(t, f, installer, clientDir, "update-v2", "update", f.PackageRoot, "--target", "opencode")
	openCodeAssertCompleted(t, "version update", updated)
	stages["version_update"] = openCodeNativeStage{Status: "passed"}

	before := openCodeSHA256(t, filepath.Join(f.XDGConfig, "opencode", "skills", "demo-skill", "SKILL.md"))
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	skillBody, err := os.ReadFile(filepath.Join(f.PackageRoot, "skills", "demo-skill", "SKILL.md"))
	must(err)
	must(os.WriteFile(filepath.Join(f.PackageRoot, "skills", "demo-skill", "SKILL.md"), append(append([]byte{}, skillBody...), []byte("\nExtra content for the same-version refresh.\n")...), 0644))
	refreshed := openCodeRunInstaller(t, f, installer, clientDir, "same-version-refresh", "update", f.PackageRoot, "--target", "opencode")
	openCodeAssertCompleted(t, "same-version content refresh", refreshed)
	openCodeAssertMutated(t, refreshed, true)
	after := openCodeSHA256(t, filepath.Join(f.XDGConfig, "opencode", "skills", "demo-skill", "SKILL.md"))
	if before == after {
		t.Fatalf("same-version content refresh did not change the installed skill file digest")
	}
	stages["same_version_refresh"] = openCodeNativeStage{Status: "passed", Reason: "mutated:true and installed skill file digest changed on identical version, changed content"}

	beforeUnchanged := openCodeSHA256(t, filepath.Join(f.XDGConfig, "opencode", "opencode.json"))
	unchanged := openCodeRunInstaller(t, f, installer, clientDir, "unchanged-reapply", "update", f.PackageRoot, "--target", "opencode")
	openCodeAssertCompleted(t, "unchanged re-apply", unchanged)
	openCodeAssertMutated(t, unchanged, false)
	afterUnchanged := openCodeSHA256(t, filepath.Join(f.XDGConfig, "opencode", "opencode.json"))
	if beforeUnchanged != afterUnchanged {
		t.Fatalf("physically-unchanged re-apply changed the config file digest: %s -> %s", beforeUnchanged, afterUnchanged)
	}
	stages["unchanged"] = openCodeNativeStage{Status: "passed", Reason: "mutated:false and byte-identical config file digest on an identical re-apply"}

	// Plant an unrelated foreign top-level config key before any destructive
	// operation, and carry it through repair/remove to prove it survives.
	rawConfig, err := os.ReadFile(filepath.Join(f.XDGConfig, "opencode", "opencode.json"))
	must(err)
	var rawDoc map[string]any
	must(json.Unmarshal(rawConfig, &rawDoc))
	rawMCP, _ := rawDoc["mcp"].(map[string]any)
	if rawMCP == nil {
		rawMCP = map[string]any{}
		rawDoc["mcp"] = rawMCP
	}
	rawMCP["foreign-untouched"] = map[string]any{"type": "local", "command": []any{"sh", "-c", "cat"}}
	foreignBody, err := json.MarshalIndent(rawDoc, "", "  ")
	must(err)
	must(os.WriteFile(filepath.Join(f.XDGConfig, "opencode", "opencode.json"), foreignBody, 0644))

	// Genuine absent-directory repair: delete the whole managed package
	// directory UAP owns (the closest OpenCode analog to the Codex C3 /
	// Claude CL1 absent-repair scenario), not merely re-apply unchanged
	// content. The config entries and installed skill file are untouched by
	// this deletion -- only the staged source UAP would re-project from is
	// gone, so repair must genuinely reconstruct it.
	physicalArtifactID := openCodePhysicalArtifactID(t, updated)
	managedDir := filepath.Join(f.StateHome, "managed", "clients", "opencode", physicalArtifactID)
	if _, statErr := os.Stat(managedDir); statErr != nil {
		t.Fatalf("managed package directory not found before deleting it: %v", statErr)
	}
	installedSkillBefore := openCodeSHA256(t, filepath.Join(f.XDGConfig, "opencode", "skills", "demo-skill", "SKILL.md"))
	must(os.RemoveAll(managedDir))
	if _, statErr := os.Stat(managedDir); !os.IsNotExist(statErr) {
		t.Fatalf("managed package directory still exists after deletion: %v", statErr)
	}

	repaired := openCodeRunInstaller(t, f, installer, clientDir, "repair", "repair", "opencode-native-proof", "--target", "opencode")
	openCodeAssertCompleted(t, "repair", repaired)
	if _, statErr := os.Stat(filepath.Join(managedDir, "plugin.json")); statErr != nil {
		t.Fatalf("repair did not reconstruct the deleted managed package directory: %v", statErr)
	}
	installedSkillAfter := openCodeSHA256(t, filepath.Join(f.XDGConfig, "opencode", "skills", "demo-skill", "SKILL.md"))
	if installedSkillBefore != installedSkillAfter {
		t.Fatalf("repair changed the installed skill file's digest: %s -> %s", installedSkillBefore, installedSkillAfter)
	}
	postRepair := openCodeDebugConfig(t, f, client, "debug-config-post-repair")
	postRepairMCP := openCodeConfigMCP(t, postRepair)
	if _, ok := postRepairMCP["foreign-untouched"]; !ok {
		t.Fatalf("repair removed a foreign, unrelated config entry: %+v", postRepairMCP)
	}
	if _, ok := postRepairMCP["api/server"]; !ok {
		t.Fatalf("repair lost the managed api/server entry: %+v", postRepairMCP)
	}
	stages["owned_repair"] = openCodeNativeStage{Status: "passed", Reason: "the whole managed package directory was deleted and repair genuinely reconstructed it; installed skill file digest unchanged, config entries intact"}
	stages["foreign_config_preservation"] = openCodeNativeStage{Status: "passed", Reason: "foreign-untouched entry present in effective config after a genuine absent-directory repair"}

	// Foreign key collision: a raw config entry claims the exact logical key
	// our package also declares. O2 requires this be observed, not silently
	// resolved -- record actual behavior rather than assuming refusal. Bump
	// content first so the subsequent update attempt actually goes through
	// the real write path (openCodeMCPRequests), not the no-change/VerifyOnly
	// branch a same-content re-apply would take.
	openCodeWriteFixturePackage(t, f.PackageRoot, "opencode-native-proof", "3.0.0")
	preCollisionBytes, err := os.ReadFile(filepath.Join(f.XDGConfig, "opencode", "opencode.json"))
	must(err)
	collisionDoc := map[string]any{}
	must(json.Unmarshal(preCollisionBytes, &collisionDoc))
	collisionMCP, _ := collisionDoc["mcp"].(map[string]any)
	delete(collisionMCP, "api/server")
	collisionMCP["api/server"] = map[string]any{"type": "local", "command": []any{"sh", "-c", "echo foreign-owns-this-key"}}
	collisionBody, err := json.MarshalIndent(collisionDoc, "", "  ")
	must(err)
	must(os.WriteFile(filepath.Join(f.XDGConfig, "opencode", "opencode.json"), collisionBody, 0644))
	postCollisionPlantBytes, err := os.ReadFile(filepath.Join(f.XDGConfig, "opencode", "opencode.json"))
	must(err)
	beforeCollisionSkillDigest := openCodeSHA256(t, filepath.Join(f.XDGConfig, "opencode", "skills", "demo-skill", "SKILL.md"))

	collisionUpdateOut, collisionUpdateErr := openCodeRunInstallerExpectError(t, f, installer, clientDir, "foreign-key-collision-update", "update", f.PackageRoot, "--target", "opencode")
	if collisionUpdateErr == nil {
		stages["foreign_key_collision"] = openCodeNativeStage{Status: "failed", Reason: "update silently overwrote a foreign entry claiming a managed logical key instead of refusing"}
		t.Fatalf("expected refusal when a foreign entry occupies a managed logical key, got success:\n%s", collisionUpdateOut)
	}
	if !strings.Contains(collisionUpdateOut, "native MCP entry is not exactly owned") || !strings.Contains(collisionUpdateOut, "api/server") {
		t.Fatalf("update refusal did not contain the exact expected ownership message for api/server:\n%s", collisionUpdateOut)
	}
	// The overall operation is not a clean no-op: UAP's own bookkeeping (the
	// managed package directory and internal state) commits the new version
	// before external activation is attempted per key, so this specific
	// failure mode is the documented managed_committed_activation_failed
	// state, not a full rollback. Assert that precisely instead of only
	// checking that an error occurred.
	openCodeAssertJSONDataStatus(t, collisionUpdateOut, "managed_committed_activation_failed")
	afterCollisionUpdateBytes, err := os.ReadFile(filepath.Join(f.XDGConfig, "opencode", "opencode.json"))
	must(err)
	if string(afterCollisionUpdateBytes) != string(postCollisionPlantBytes) {
		t.Fatalf("config file changed despite a refused update:\nbefore=%s\nafter=%s", postCollisionPlantBytes, afterCollisionUpdateBytes)
	}
	afterCollisionUpdateSkillDigest := openCodeSHA256(t, filepath.Join(f.XDGConfig, "opencode", "skills", "demo-skill", "SKILL.md"))
	if afterCollisionUpdateSkillDigest != beforeCollisionSkillDigest {
		t.Fatalf("installed skill file changed despite the refused MCP entry write: %s -> %s", beforeCollisionSkillDigest, afterCollisionUpdateSkillDigest)
	}

	// The managed key is still foreign-occupied: repair must refuse this too
	// (it only reconstructs absent managed objects, never adopts a foreign
	// one -- same ownership discipline as C3), verified with a real repair
	// call and its exact refusal message, not merely asserted in a comment.
	collisionRepairOut, collisionRepairErr := openCodeRunInstallerExpectError(t, f, installer, clientDir, "foreign-key-collision-repair", "repair", "opencode-native-proof", "--target", "opencode")
	if collisionRepairErr == nil {
		stages["foreign_key_collision"] = openCodeNativeStage{Status: "failed", Reason: "repair silently adopted a foreign entry claiming a managed logical key instead of refusing"}
		t.Fatalf("expected repair to refuse when a foreign entry occupies a managed logical key, got success:\n%s", collisionRepairOut)
	}
	if !strings.Contains(collisionRepairOut, "native MCP entry is not exactly owned") || !strings.Contains(collisionRepairOut, "api/server") {
		t.Fatalf("repair refusal did not contain the exact expected ownership message for api/server:\n%s", collisionRepairOut)
	}
	openCodeAssertJSONDataStatus(t, collisionRepairOut, "managed_committed_activation_failed")
	afterCollisionRepairBytes, err := os.ReadFile(filepath.Join(f.XDGConfig, "opencode", "opencode.json"))
	must(err)
	if string(afterCollisionRepairBytes) != string(postCollisionPlantBytes) {
		t.Fatalf("config file changed despite a refused repair:\nbefore=%s\nafter=%s", postCollisionPlantBytes, afterCollisionRepairBytes)
	}
	afterCollisionRepairSkillDigest := openCodeSHA256(t, filepath.Join(f.XDGConfig, "opencode", "skills", "demo-skill", "SKILL.md"))
	if afterCollisionRepairSkillDigest != beforeCollisionSkillDigest {
		t.Fatalf("installed skill file changed despite the refused repair: %s -> %s", beforeCollisionSkillDigest, afterCollisionRepairSkillDigest)
	}
	stages["foreign_key_collision"] = openCodeNativeStage{Status: "passed", Reason: "both update and repair independently refused external activation with the exact ownership message naming the occupied key (data.status:managed_committed_activation_failed -- UAP's own managed bookkeeping commits the new version, but the client-facing config write for the occupied key is refused), while the package had genuinely changed content (not a no-change/VerifyOnly re-apply); config file bytes and installed skill file digest verified unchanged after each refusal"}
	// Restore the exact pre-collision bytes directly, simulating the foreign
	// writer reverting its own change, so remove below observes the actual
	// managed state.
	must(os.WriteFile(filepath.Join(f.XDGConfig, "opencode", "opencode.json"), preCollisionBytes, 0644))

	removed := openCodeRunInstaller(t, f, installer, clientDir, "remove", "remove", "opencode-native-proof", "--target", "opencode")
	if removed == nil || removed["result"] != "success" {
		t.Fatalf("remove did not report success: %+v", removed)
	}
	if data, _ := removed["data"].(map[string]any); data == nil || data["status"] != "data_retained" {
		t.Fatalf("remove data.status = %v, want data_retained: %+v", removed["data"], removed)
	}
	postRemove := openCodeDebugConfig(t, f, client, "debug-config-post-remove")
	postRemoveMCP := openCodeConfigMCP(t, postRemove)
	if _, ok := postRemoveMCP["api/server"]; ok {
		t.Fatalf("remove left the managed api/server entry behind: %+v", postRemoveMCP)
	}
	if _, ok := postRemoveMCP["api server"]; ok {
		t.Fatalf("remove left the managed \"api server\" entry behind: %+v", postRemoveMCP)
	}
	if _, ok := postRemoveMCP["foreign-untouched"]; !ok {
		t.Fatalf("remove deleted the unrelated foreign entry: %+v", postRemoveMCP)
	}
	// debug config only proves the MCP entries are gone from the client's
	// view; independently confirm the installed skill directory itself was
	// actually deleted from disk, not merely unreferenced.
	if _, statErr := os.Stat(filepath.Join(f.XDGConfig, "opencode", "skills", "demo-skill")); !os.IsNotExist(statErr) {
		t.Fatalf("remove did not delete the installed skill directory from disk: %v", statErr)
	}
	stages["remove"] = openCodeNativeStage{Status: "passed", Reason: "result:success and data.status:data_retained; managed MCP entries removed from effective config; installed skill directory actually deleted from disk (not just absent from debug config); foreign sibling config entry preserved"}

	repeatOut, repeatErr := openCodeRunInstallerExpectError(t, f, installer, clientDir, "repeat-remove", "remove", "opencode-native-proof", "--target", "opencode")
	if repeatErr == nil {
		stages["repeat_remove"] = openCodeNativeStage{Status: "failed", Reason: "repeated remove of an already-removed installation was silently accepted"}
		t.Fatalf("expected refusal for repeat remove, got success:\n%s", repeatOut)
	}
	stages["repeat_remove"] = openCodeNativeStage{Status: "passed", Reason: "repeated remove correctly refused"}
}

func openCodeNativeBinary(t *testing.T, key string) string {
	t.Helper()
	p := os.Getenv(key)
	if !filepath.IsAbs(p) {
		t.Fatalf("%s must name an absolute scratch binary", key)
	}
	st, err := os.Stat(p)
	if err != nil || !st.Mode().IsRegular() || st.Mode()&0111 == 0 {
		t.Fatalf("invalid %s binary", key)
	}
	return p
}
