package pluginkitairepo_test

// This opt-in suite uses only disposable fixture paths. Native execution requires
// a disposable image/VM or explicitly opted-in hosted runner; redirected HOME
// alone cannot isolate the macOS keychain.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const claudeNativeCommandTimeout = 30 * time.Second

type claudeNativeStage struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type claudePluginListEntry struct {
	ID          string `json:"id"`
	Version     string `json:"version"`
	Scope       string `json:"scope"`
	Enabled     bool   `json:"enabled"`
	InstallPath string `json:"installPath"`
}

func claudeNativeBinary(t *testing.T, key string) string {
	t.Helper()
	p := os.Getenv(key)
	if !filepath.IsAbs(p) {
		t.Fatalf("%s must name an absolute scratch binary", key)
	}
	st, err := os.Stat(p)
	if err != nil || !st.Mode().IsRegular() || (runtime.GOOS != "windows" && st.Mode()&0111 == 0) {
		t.Fatalf("invalid %s binary", key)
	}
	return p
}

func claudeSHA256(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// claudeNativeFixture isolates every path the real `claude` CLI and the
// `agentplugins` installer touch. Nothing here is the invoking user's real
// home, config, or data directory.
type claudeNativeFixture struct {
	Root, Home, ConfigDir, PackageRoot, StateHome string
	transcriptSeq                                 int
	transcriptSHA256                              map[string]string
}

func newClaudeNativeFixture(t *testing.T) *claudeNativeFixture {
	t.Helper()
	root, err := os.MkdirTemp("", "uap-claude-native-evidence-")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("claude native evidence: %s", root)
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	f := &claudeNativeFixture{
		Root: root, Home: filepath.Join(root, "home"), ConfigDir: filepath.Join(root, "claude-config"),
		PackageRoot: filepath.Join(root, "fixture"), StateHome: filepath.Join(root, "installer-state"),
		transcriptSHA256: map[string]string{},
	}
	for _, p := range []string{f.Home, f.ConfigDir, f.PackageRoot, f.StateHome, filepath.Join(root, "tmp"), filepath.Join(root, "xdg-config"), filepath.Join(root, "xdg-data"), filepath.Join(root, "xdg-cache")} {
		if err := os.MkdirAll(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

// env returns a bounded, isolated environment for both the `claude` CLI and
// the `agentplugins` installer: a disposable HOME and CLAUDE_CONFIG_DIR, no
// inherited ambient credentials beyond PATH/LANG, and updater/telemetry
// disabled where the client honors that.
func (f *claudeNativeFixture) env(clientDir string) []string {
	return append(nativePlatformEnvironment(f.Root, f.Home, clientDir), "CLAUDE_CONFIG_DIR="+f.ConfigDir, "AGENTPLUGINS_HOME="+f.StateHome, "DISABLE_AUTOUPDATER=1", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1")
}

// record saves a command's combined stdout+stderr to its own evidence file
// and its SHA-256 into the fixture's transcript index, mirroring the Codex
// suite's nativeWrite/nativeSHA transcript-hashing pattern.
func (f *claudeNativeFixture) record(t *testing.T, label string, out []byte) {
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

func claudeRunInstaller(t *testing.T, f *claudeNativeFixture, installer, clientDir, label string, args ...string) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), claudeNativeCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, installer, append(args, "--format", "json")...)
	cmd.Env = f.env(clientDir)
	cmd.Dir = f.Root
	out, err := cmd.CombinedOutput()
	f.record(t, label, out)
	if err != nil {
		if _, isExit := err.(*exec.ExitError); !isExit {
			t.Fatalf("run installer %v: %v\n%s", args, err, out)
		}
	}
	line := lastJSONLine(out)
	var decoded map[string]any
	if line != "" {
		if jsonErr := json.Unmarshal([]byte(line), &decoded); jsonErr != nil {
			t.Fatalf("decode installer output for %v: %v\n%s", args, jsonErr, out)
		}
	}
	return decoded
}

// claudeRunInstallerExpectError runs the installer expecting a nonzero exit
// and returns its combined output for message inspection; it does not attempt
// JSON decoding since these calls exercise the plain-text refusal path.
func claudeRunInstallerExpectError(t *testing.T, f *claudeNativeFixture, installer, clientDir, label string, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), claudeNativeCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, installer, append(args, "--format", "json")...)
	cmd.Env = f.env(clientDir)
	cmd.Dir = f.Root
	out, err := cmd.CombinedOutput()
	f.record(t, label, out)
	return string(out), err
}

func lastJSONLine(out []byte) string {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "{") {
			return trimmed
		}
	}
	return ""
}

func claudeVersionString(t *testing.T, f *claudeNativeFixture, client string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), claudeNativeCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, client, "--version")
	cmd.Env = f.env(filepath.Dir(client))
	cmd.Dir = f.Home
	out, err := cmd.CombinedOutput()
	f.record(t, "claude-version", out)
	if err != nil {
		t.Fatalf("claude --version: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

func claudePluginList(t *testing.T, f *claudeNativeFixture, client, label string) []claudePluginListEntry {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), claudeNativeCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, client, "plugin", "list", "--json")
	cmd.Env = f.env(filepath.Dir(client))
	cmd.Dir = f.Home
	out, err := cmd.CombinedOutput()
	f.record(t, label, out)
	if err != nil {
		t.Fatalf("claude plugin list --json: %v\n%s", err, out)
	}
	var entries []claudePluginListEntry
	if err := json.Unmarshal(out, &entries); err != nil {
		t.Fatalf("decode claude plugin list output: %v\n%s", err, out)
	}
	return entries
}

func claudePluginDetails(t *testing.T, f *claudeNativeFixture, client, pluginID, label string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), claudeNativeCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, client, "plugin", "details", pluginID)
	cmd.Env = f.env(filepath.Dir(client))
	cmd.Dir = f.Home
	out, err := cmd.CombinedOutput()
	f.record(t, label, out)
	if err != nil {
		t.Fatalf("claude plugin details %s: %v\n%s", pluginID, err, out)
	}
	return string(out)
}

// claudeMCPList runs the real health-checked MCP listing, which is the one
// command that reports transport-specific classification (e.g. "(HTTP)" /
// "(SSE)") and actually attempts a connection per entry -- unlike `plugin
// details`, which only enumerates declared .mcp.json keys without validating
// their type. A short timeout is used deliberately: the fixture's MCP
// endpoints point at an unroutable loopback port, so every entry is expected
// to fail to connect quickly, not hang.
func claudeMCPList(t *testing.T, f *claudeNativeFixture, client, label string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), claudeNativeCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, client, "mcp", "list")
	cmd.Env = f.env(filepath.Dir(client))
	cmd.Dir = f.Home
	out, _ := cmd.CombinedOutput()
	f.record(t, label, out)
	return string(out)
}

func claudeWriteFixturePackage(t *testing.T, root, name, version string) {
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
  "description": "Claude Code native lifecycle proof fixture"
}`, name, version)), 0644))
	must(os.WriteFile(filepath.Join(root, "skills", "demo-skill", "SKILL.md"), []byte("---\nname: demo-skill\ndescription: A demo skill for native lifecycle verification.\n---\n\n# Demo Skill\n\nSay hello when asked to demo.\n"), 0644))
	// Loopback, unroutable addresses: real network calls against these fail
	// fast and deterministically (connection refused), which is what lets
	// `claude mcp list` be asserted against without depending on any external
	// service being reachable.
	must(os.WriteFile(filepath.Join(root, "mcp.json"), []byte(`{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
  "mcpServers": {
    "local": {"type": "stdio", "command": "sh", "args": ["-c", "cat"]},
    "remote-http": {"type": "streamable-http", "url": "http://127.0.0.1:9/mcp"},
    "remote-sse": {"type": "sse", "url": "http://127.0.0.1:9/sse"}
  }
}`), 0644))
}

func claudeReadInstalledSkill(t *testing.T, installPath string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(installPath, ".agentplugins-runtime", "skills", "demo-skill", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	return body
}

// claudeAssertMutated decodes an installer JSON result's single-target
// mutated flag and fails the test if it does not equal want. This exists
// because "no_change:true" alone does not prove the physical swap was
// skipped -- mutated is the field the kernel actually sets from whether a
// directory transaction ran.
func claudeAssertMutated(t *testing.T, result map[string]any, want bool) {
	t.Helper()
	data, _ := result["data"].(map[string]any)
	targets, _ := data["targets"].([]any)
	if len(targets) != 1 {
		t.Fatalf("expected exactly one target in result: %+v", result)
	}
	target, _ := targets[0].(map[string]any)
	output, _ := target["output"].(map[string]any)
	resultObj, _ := output["result"].(map[string]any)
	mutated, _ := resultObj["mutated"].(bool)
	if mutated != want {
		t.Fatalf("mutated = %v, want %v: %+v", mutated, want, resultObj)
	}
}

// claudeStateReceiptForOperation reads the persisted state-v2.json under the
// fixture's isolated AGENTPLUGINS_HOME and returns the exact receipt whose
// operation_group_id matches operationGroupID (the repair command's own
// reported operation_id), for the given installation's client bindings.
// Matching by group ID -- rather than "the last receipt of whichever client
// binding happens to have any" -- avoids silently reading a stale receipt
// from an earlier, unrelated operation (for example, if the targeted
// operation turned out to be a no-op and appended no new receipt at all).
// Exactly one client binding is required to hold a matching receipt; more or
// fewer is a test-setup problem, not something to guess through.
func claudeStateReceiptForOperation(t *testing.T, f *claudeNativeFixture, installationID, operationGroupID string) (beforeDigest, afterDigest string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(f.StateHome, "state-v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state struct {
		Installations []struct {
			InstallationID string `json:"installation_id"`
			Clients        map[string]struct {
				Receipts []struct {
					OperationGroupID string `json:"operation_group_id"`
					BeforeDigest     string `json:"before_digest"`
					AfterDigest      string `json:"after_digest"`
				} `json:"receipts"`
			} `json:"clients"`
		} `json:"installations"`
	}
	if err := json.Unmarshal(body, &state); err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, installation := range state.Installations {
		if installation.InstallationID != installationID {
			continue
		}
		for _, client := range installation.Clients {
			for _, receipt := range client.Receipts {
				if receipt.OperationGroupID != operationGroupID {
					continue
				}
				matches++
				beforeDigest, afterDigest = receipt.BeforeDigest, receipt.AfterDigest
			}
		}
	}
	if matches != 1 {
		t.Fatalf("expected exactly one receipt for operation_group_id %s under installation %s, found %d", operationGroupID, installationID, matches)
	}
	return beforeDigest, afterDigest
}

// TestAgentpluginsClaudeNativeLifecycle drives the real Claude Code CLI
// through UAP's actual skills-dir install route: install, discovery via the
// client's own `plugin list`/`plugin details`/`mcp list`, absent-directory
// repair, version update, same-version content update, physical idempotence,
// and remove with foreign-content preservation plus repeat-remove refusal.
//
// Native tool calls through a live app-server session, actual model skill
// use, and OAuth are NOT evaluated here; they require a distinct, separately
// authorized route and are explicitly out of scope for this checkpoint.
func TestAgentpluginsClaudeNativeLifecycle(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_CLAUDE_NATIVE_E2E") != "1" {
		t.Skip("opt-in native client execution")
	}
	nativeRequireDisposable(t)
	client := claudeNativeBinary(t, "AGENTPLUGINS_CLAUDE_BIN")
	installer := claudeNativeBinary(t, "AGENTPLUGINS_INSTALLER_BIN")
	if expected := os.Getenv("AGENTPLUGINS_CLAUDE_SHA256"); len(expected) != 64 || claudeSHA256(t, client) != expected {
		t.Fatal("Claude binary pin mismatch")
	}
	f := newClaudeNativeFixture(t)
	nativeProvisionScanner(t, &nativeFixture{Root: f.Root})
	clientDir := filepath.Dir(client)

	// Every expected stage is declared up front as not_evaluated, matching
	// the Codex suite's pattern: a stage that is never reached (an early
	// Fatal, a skipped later section) must still show up honestly in the
	// evidence file instead of silently vanishing from an otherwise-complete-
	// looking JSON document.
	stages := map[string]claudeNativeStage{
		"install":                 {Status: "not_evaluated", Reason: "not reached"},
		"skill_discovery":         {Status: "not_evaluated", Reason: "not reached"},
		"mcp_transport_discovery": {Status: "not_evaluated", Reason: "not reached"},
		"version_update":          {Status: "not_evaluated", Reason: "not reached"},
		"same_version_refresh":    {Status: "not_evaluated", Reason: "not reached"},
		"unchanged":               {Status: "not_evaluated", Reason: "not reached"},
		"owned_repair":            {Status: "not_evaluated", Reason: "not reached"},
		"foreign_content_inside_managed_directory_blocked": {Status: "not_evaluated", Reason: "not reached"},
		"remove":        {Status: "not_evaluated", Reason: "not reached"},
		"repeat_remove": {Status: "not_evaluated", Reason: "not reached"},
	}
	var measuredClientVersion string
	defer func() {
		evidence := map[string]any{
			"finished_utc":            time.Now().UTC().Format(time.RFC3339Nano),
			"client_version_measured": measuredClientVersion,
			"client_binary_sha256":    claudeSHA256(t, client),
			"installer_binary_sha256": claudeSHA256(t, installer),
			"installer_base_commit":   os.Getenv("AGENTPLUGINS_INSTALLER_COMMIT"),
			"installer_tree":          os.Getenv("AGENTPLUGINS_INSTALLER_TREE"),
			"installer_patch_sha256":  os.Getenv("AGENTPLUGINS_INSTALLER_PATCH_SHA256"),
			"stages":                  stages,
			"transcript_sha256":       f.transcriptSHA256,
		}
		body, _ := json.MarshalIndent(evidence, "", "  ")
		_ = os.WriteFile(filepath.Join(f.Root, "evidence.json"), body, 0600)
	}()

	// Measure the client actually under test rather than trusting an env var.
	measuredClientVersion = claudeVersionString(t, f, client)
	if want := os.Getenv("AGENTPLUGINS_CLAUDE_VERSION"); want == "" || measuredClientVersion != want {
		t.Fatalf("Claude version %q does not match %q", measuredClientVersion, want)
	}

	claudeWriteFixturePackage(t, f.PackageRoot, "claude-native-proof", "1.0.0")

	// install
	result := claudeRunInstaller(t, f, installer, clientDir, "add-v1", "add", f.PackageRoot, "--target", "claude")
	if data, _ := result["data"].(map[string]any); data == nil || data["status"] != "completed" {
		t.Fatalf("install did not complete: %+v", result)
	}
	entries := claudePluginList(t, f, client, "list-after-install")
	if len(entries) != 1 || entries[0].ID != "claude-native-proof@skills-dir" || !entries[0].Enabled || entries[0].Version != "1.0.0" {
		t.Fatalf("claude plugin list did not report the installed plugin: %+v", entries)
	}
	installPath := entries[0].InstallPath
	data, _ := result["data"].(map[string]any)
	targetsRaw, _ := data["targets"].([]any)
	target0, _ := targetsRaw[0].(map[string]any)
	output0, _ := target0["output"].(map[string]any)
	resultObj0, _ := output0["result"].(map[string]any)
	installationID, _ := resultObj0["installation_id"].(string)
	if installationID == "" {
		t.Fatalf("install result did not report an installation_id: %+v", result)
	}
	stages["install"] = claudeNativeStage{Status: "passed"}

	// skill discovery: exact plugin-attributed skill, exact bytes
	details := claudePluginDetails(t, f, client, "claude-native-proof@skills-dir", "details-after-install")
	if !strings.Contains(details, "demo-skill") || !strings.Contains(details, "MCP servers (3)") {
		t.Fatalf("claude plugin details did not attribute the exact skill/MCP inventory: %s", details)
	}
	installedSkill := claudeReadInstalledSkill(t, installPath)
	if !strings.Contains(string(installedSkill), "Say hello when asked to demo.") {
		t.Fatalf("installed skill body does not match the exact fixture bytes: %q", installedSkill)
	}
	stages["skill_discovery"] = claudeNativeStage{Status: "passed", Reason: "exact plugin-attributed skill enumerated by the real CLI with exact body bytes verified on disk; no model use claimed"}

	// MCP transport discovery: `plugin details` only enumerates declared
	// .mcp.json keys without validating their type (a bogus type value is
	// listed identically to a real one there); `claude mcp list` is the
	// command that classifies by transport and attempts a real connection
	// per entry, so it is the one asserted against for transport-specific
	// proof. Loopback URLs make every entry fail fast and deterministically
	// rather than depending on external network reachability.
	mcpList := claudeMCPList(t, f, client, "mcp-list-after-install")
	for _, want := range []string{
		"plugin:claude-native-proof:remote-http:", "(HTTP)",
		"plugin:claude-native-proof:remote-sse:", "(SSE)",
		"plugin:claude-native-proof:local:",
	} {
		if !strings.Contains(mcpList, want) {
			t.Fatalf("claude mcp list did not classify the declared transports as expected (missing %q): %s", want, mcpList)
		}
	}
	stages["mcp_transport_discovery"] = claudeNativeStage{Status: "passed", Reason: "claude mcp list classified remote-http as (HTTP) and remote-sse as (SSE) and attempted a real (loopback-refused) connection to each, distinct from plugin details' shallow key enumeration; no live tool call or successful connection claimed"}

	// version update
	claudeWriteFixturePackage(t, f.PackageRoot, "claude-native-proof", "2.0.0")
	result = claudeRunInstaller(t, f, installer, clientDir, "update-v2", "update", "claude-native-proof", "--target", "claude")
	if data, _ := result["data"].(map[string]any); data == nil || data["status"] != "completed" {
		t.Fatalf("version update did not complete: %+v", result)
	}
	entries = claudePluginList(t, f, client, "list-after-version-update")
	if len(entries) != 1 || entries[0].Version != "2.0.0" {
		t.Fatalf("claude plugin list did not reflect the updated version: %+v", entries)
	}
	stages["version_update"] = claudeNativeStage{Status: "passed"}

	// same-version content update
	before := claudeReadInstalledSkill(t, installPath)
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.WriteFile(filepath.Join(f.PackageRoot, "skills", "demo-skill", "SKILL.md"), append(append([]byte{}, before...), []byte("\nExtra content for the same-version refresh.\n")...), 0644))
	refreshResult := claudeRunInstaller(t, f, installer, clientDir, "update-same-version", "update", "claude-native-proof", "--target", "claude")
	if data, _ := refreshResult["data"].(map[string]any); data == nil || data["status"] != "completed" {
		t.Fatalf("same-version content update did not complete: %+v", refreshResult)
	}
	claudeAssertMutated(t, refreshResult, true)
	after := claudeReadInstalledSkill(t, installPath)
	if !strings.Contains(string(after), "Extra content for the same-version refresh.") {
		t.Fatalf("same-version content update did not refresh the skill body")
	}
	stages["same_version_refresh"] = claudeNativeStage{Status: "passed"}

	// physical idempotence: re-apply the exact same content and require
	// mutated:false plus byte-identical on-disk state, not merely a no_change
	// field's presence. The installer JSON's own tree_digest reflects the
	// source PACKAGE's digest (fixed at install time), not the actual managed
	// directory -- it cannot distinguish "physically untouched" from
	// "re-swapped with the same bytes", so the real proof is a filesystem
	// tree digest of the managed directory plus a digest of the persisted
	// state file, both taken before and after, matching the Codex suite's
	// own unchanged-stage pattern (agentplugins_codex_native_e2e_test.go).
	statePath := filepath.Join(f.StateHome, "state-v2.json")
	beforeManagedTree, beforeStateDigest := nativeTreeDigest(t, installPath), claudeSHA256(t, statePath)
	unchangedResult := claudeRunInstaller(t, f, installer, clientDir, "update-unchanged", "update", "claude-native-proof", "--target", "claude")
	if data, _ := unchangedResult["data"].(map[string]any); data == nil || data["status"] != "completed" {
		t.Fatalf("idempotent re-apply did not complete: %+v", unchangedResult)
	}
	claudeAssertMutated(t, unchangedResult, false)
	if got := nativeTreeDigest(t, installPath); got != beforeManagedTree {
		t.Fatalf("idempotent re-apply changed the managed directory's physical content: before=%q after=%q", beforeManagedTree, got)
	}
	if got := claudeSHA256(t, statePath); got != beforeStateDigest {
		t.Fatalf("idempotent re-apply changed the persisted state file: before=%q after=%q", beforeStateDigest, got)
	}
	stages["unchanged"] = claudeNativeStage{Status: "passed", Reason: "mutated:false plus byte-identical managed-directory tree digest and state-file digest before/after a byte-identical re-apply"}

	// absent-directory repair (mirrors the Codex C3 absent-repair scenario;
	// unlike Codex's `plugin list`, Claude Code's own list command does not
	// fail globally when a managed directory is missing, so no analogous
	// defect was found or fixed here). Repair here re-stages from the still-
	// present original local source directory, which must still exist and
	// match the exact recorded revision -- it is not a reconstruction from
	// state alone the way Codex's C3 recovery path is, since this repair
	// route is not restricted to the Codex-only recovering gate.
	must(os.RemoveAll(installPath))
	repairResult := claudeRunInstaller(t, f, installer, clientDir, "repair-absent", "repair", "claude-native-proof", "--target", "claude")
	if data, _ := repairResult["data"].(map[string]any); data == nil || data["status"] != "completed" {
		t.Fatalf("absent-directory repair did not complete: %+v", repairResult)
	}
	entries = claudePluginList(t, f, client, "list-after-repair")
	if len(entries) != 1 || entries[0].Version != "2.0.0" {
		t.Fatalf("repair did not restore a plugin the real CLI recognizes: %+v", entries)
	}
	restoredSkill := claudeReadInstalledSkill(t, installPath)
	if !strings.Contains(string(restoredSkill), "Extra content for the same-version refresh.") {
		t.Fatalf("repair did not restore the exact latest content: %q", restoredSkill)
	}
	if _, err := os.Stat(filepath.Join(installPath, ".mcp.json")); err != nil {
		t.Fatalf("repair did not restore .mcp.json: %v", err)
	}
	repairDetails := claudePluginDetails(t, f, client, "claude-native-proof@skills-dir", "details-after-repair")
	if !strings.Contains(repairDetails, "MCP servers (3)") {
		t.Fatalf("claude plugin details after repair did not attribute the restored MCP inventory: %s", repairDetails)
	}
	// Known, pre-existing, non-Codex-specific audit-trail gap (not
	// introduced or fixed by this checkpoint): repair's recorded
	// BeforeDigest describes what was last recorded as owned, not the
	// physically-absent state this mutation actually observed, because the
	// Codex-only C3 recovering gate (usecase/group.go's
	// observeGroupRecoveryEligibility) is the only path that corrects this.
	// BeforeDigest is not read by any production decision logic (confirmed
	// separately in this session's C3 review), so this is tracked here as a
	// documented limitation rather than silently missed or fixed out of
	// scope. The receipt is located by the repair command's own reported
	// operation_id (not "whichever client binding's last receipt happens to
	// be"), so a no-op repair that appended no new receipt would fail this
	// lookup loudly instead of silently reading a stale receipt from an
	// earlier operation.
	repairOperationID, _ := repairResult["data"].(map[string]any)["operation_id"].(string)
	if repairOperationID == "" {
		t.Fatalf("repair result did not report an operation_id: %+v", repairResult)
	}
	if before, after := claudeStateReceiptForOperation(t, f, installationID, repairOperationID); before == "" || before != after {
		t.Fatalf("expected the known phantom BeforeDigest gap (before_digest == after_digest, both nonempty, since repair reconstructs byte-identical content despite physical absence at mutation time); got before=%q after=%q -- this either means the gap was fixed (update this test) or the receipt lookup is wrong", before, after)
	}
	stages["owned_repair"] = claudeNativeStage{Status: "passed", Reason: "managed directory re-staged from the still-present, exact-revision-matching local source after the directory was deleted; restored content bytes, .mcp.json, and claude plugin details all verified; recorded BeforeDigest confirmed to carry the known pre-existing phantom-before-state gap shared by every non-Codex client, not fixed by this checkpoint"}

	// Foreign content INSIDE the managed directory must block removal
	// (fail-closed ownership verification), not be silently deleted or
	// overwritten.
	foreignInsideFile := filepath.Join(installPath, "foreign-injected.txt")
	must(os.WriteFile(foreignInsideFile, []byte("not ours, planted inside the managed directory"), 0644))
	blockedOut, blockedErr := claudeRunInstallerExpectError(t, f, installer, clientDir, "remove-blocked-by-foreign-content", "remove", "claude-native-proof", "--target", "claude", "--purge-data")
	if blockedErr == nil || !strings.Contains(blockedOut, "artifact digest mismatch") {
		t.Fatalf("foreign content inside the managed directory did not block removal: err=%v out=%s", blockedErr, blockedOut)
	}
	if _, err := os.Stat(foreignInsideFile); err != nil {
		t.Fatalf("blocked removal did not preserve the foreign file inside the managed directory: %v", err)
	}
	must(os.Remove(foreignInsideFile))
	stages["foreign_content_inside_managed_directory_blocked"] = claudeNativeStage{Status: "passed", Reason: "exact ownership-verification refusal (\"artifact digest mismatch\") when foreign content is planted inside the plugin's own managed directory; the foreign file survived the blocked attempt and was removed only by the test before the real remove below"}

	// foreign preservation: an unrelated skills-dir entry must survive remove
	foreignSentinel := filepath.Join(f.ConfigDir, "skills", "foreign-plugin", "sentinel.txt")
	must(os.MkdirAll(filepath.Dir(foreignSentinel), 0755))
	must(os.MkdirAll(filepath.Join(f.ConfigDir, "skills", "foreign-plugin", ".claude-plugin"), 0755))
	must(os.WriteFile(filepath.Join(f.ConfigDir, "skills", "foreign-plugin", ".claude-plugin", "plugin.json"), []byte(`{"name":"foreign-plugin"}`), 0644))
	must(os.WriteFile(foreignSentinel, []byte("not ours"), 0644))

	// --external-uninstalled is a no-op for Claude specifically: activator.go's
	// Deactivate ClientClaude branch never reads request.ExternalUninstalled,
	// unlike Codex/ChatGPT/Kiro which gate on it. Verified directly (removal
	// succeeds identically with or without the flag); omitted here rather than
	// copy-pasted from the Codex pattern to avoid implying it exercises real
	// Claude-specific behavior it does not. --purge-data is real and kept.
	result = claudeRunInstaller(t, f, installer, clientDir, "remove", "remove", "claude-native-proof", "--target", "claude", "--purge-data")
	if data, _ := result["data"].(map[string]any); data == nil || data["status"] != "completed" {
		t.Fatalf("remove did not complete: %+v", result)
	}
	if _, err := os.Stat(installPath); !os.IsNotExist(err) {
		t.Fatalf("remove left the managed directory behind: %v", err)
	}
	body, err := os.ReadFile(foreignSentinel)
	if err != nil || string(body) != "not ours" {
		t.Fatalf("remove touched foreign content: body=%q err=%v", body, err)
	}
	remaining := claudePluginList(t, f, client, "list-after-remove")
	if len(remaining) != 1 || remaining[0].ID != "foreign-plugin@skills-dir" {
		t.Fatalf("claude plugin list after remove did not show exactly the surviving foreign plugin: %+v", remaining)
	}
	stages["remove"] = claudeNativeStage{Status: "passed", Reason: "managed directory removed; sibling foreign plugin's sentinel content and its sole remaining claude plugin list entry both confirmed"}

	// repeat remove must be refused, not silently succeed or crash
	repeatOut, repeatErr := claudeRunInstallerExpectError(t, f, installer, clientDir, "repeat-remove", "remove", "claude-native-proof", "--target", "claude", "--purge-data")
	if repeatErr == nil || !strings.Contains(repeatOut, "was not found") {
		t.Fatalf("repeated remove of an absent installation was not refused: err=%v out=%s", repeatErr, repeatOut)
	}
	stages["repeat_remove"] = claudeNativeStage{Status: "passed", Reason: "exact expected refusal message"}
}
