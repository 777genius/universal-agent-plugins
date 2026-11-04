package pluginkitairepo_test

// This opt-in suite uses only disposable fixture paths. Native execution requires
// a disposable image/VM or explicitly opted-in hosted runner; redirected HOME
// alone cannot isolate the macOS keychain.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
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
	if runtime.GOOS == "windows" && len(args) > 0 && (args[0] == "add" || args[0] == "update" || args[0] == "repair") {
		// Both fixture families use a fixed, explicit stdio inventory.
		names := []string{"local"}
		if strings.Contains(string(out), `"plugin":"native-proof"`) {
			names = []string{"default", "explicit"}
		}
		if err := claudeWindowsStdioPlan(decoded, names...); err != nil {
			t.Fatal(err)
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
		"stdio_discovery":         {Status: "not_evaluated", Reason: "not reached"},
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
		if runtime.GOOS == "windows" {
			evidence["stdio"] = stages["stdio_discovery"]
			evidence["stdio_runtime_cwd_argv_env_data"] = "not_evaluated"
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
	if !strings.Contains(details, "demo-skill") || !strings.Contains(details, claudeExpectedMCPInventory()) {
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
	} {
		if !strings.Contains(mcpList, want) {
			t.Fatalf("claude mcp list did not classify the declared transports as expected (missing %q): %s", want, mcpList)
		}
	}
	if runtime.GOOS == "windows" {
		claudeAssertWindowsStdio(t, f.Root, installPath, "local")
	}
	if !strings.Contains(mcpList, "plugin:claude-native-proof:local:") {
		t.Fatal("missing local stdio discovery")
	}
	stages["stdio_discovery"] = claudeNativeStage{Status: "passed", Reason: "local stdio enumerated by native CLI; no successful tool call claimed"}
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
	if !strings.Contains(repairDetails, claudeExpectedMCPInventory()) {
		t.Fatalf("claude plugin details after repair did not attribute the restored MCP inventory: %s", repairDetails)
	}
	if runtime.GOOS == "windows" {
		claudeAssertWindowsStdio(t, f.Root, installPath, "local")
	}
	// The repair receipt must describe the physically absent directory, not
	// the last recorded owned digest. Locate it by this repair's operation ID
	// so an unrelated earlier receipt cannot make the check pass.
	repairOperationID, _ := repairResult["data"].(map[string]any)["operation_id"].(string)
	if repairOperationID == "" {
		t.Fatalf("repair result did not report an operation_id: %+v", repairResult)
	}
	if before, after := claudeStateReceiptForOperation(t, f, installationID, repairOperationID); before != "" || after == "" {
		t.Fatalf("absent-directory repair receipt must have an empty before_digest and a nonempty after_digest; got before=%q after=%q", before, after)
	}
	stages["owned_repair"] = claudeNativeStage{Status: "passed", Reason: "managed directory re-staged from the still-present, exact-revision-matching local source after deletion; restored content bytes, .mcp.json, and Claude plugin details verified; receipt records the physically absent before-state"}

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
	removeInventoryBefore := nativeFileInventory(t, f.ConfigDir, f.StateHome)
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
	// Exact removal accounting: every deleted path must be plugin-owned, and
	// no file named for the plugin may survive anywhere in the client config.
	// The full removed/added/changed inventory is recorded as evidence so the
	// claim "what exactly was removed" is inspectable, not inferred.
	removedPaths, addedPaths, changedPaths := nativeInventoryDiff(removeInventoryBefore, nativeFileInventory(t, f.ConfigDir, f.StateHome))
	within := func(path, root string) bool {
		return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
	}
	for _, path := range removedPaths {
		if !within(path, installPath) && !strings.Contains(path, "claude-native-proof") && !strings.Contains(path, "agentplugins-") {
			t.Fatalf("remove deleted a file outside the plugin's owned scope: %s (removed=%v)", path, removedPaths)
		}
	}
	for _, path := range addedPaths {
		if within(path, filepath.Join(f.ConfigDir, "skills")) || strings.Contains(path, "claude-native-proof") {
			t.Fatalf("remove created a file in the client's plugin-visible surface: %s (added=%v)", path, addedPaths)
		}
	}
	survivors := []string{}
	for path := range nativeFileInventory(t, f.ConfigDir) {
		if strings.Contains(path, "claude-native-proof") {
			survivors = append(survivors, path)
		}
	}
	sort.Strings(survivors)
	if len(survivors) != 0 {
		t.Fatalf("plugin-named files survived removal in the client config: %v", survivors)
	}
	nativeJSON(t, filepath.Join(f.Root, "remove-inventory-diff.json"), map[string]any{"removed": removedPaths, "added": addedPaths, "changed": changedPaths})
	stages["remove"] = claudeNativeStage{Status: "passed", Reason: fmt.Sprintf("managed directory removed; sibling foreign plugin's sentinel content and its sole remaining claude plugin list entry both confirmed; exact inventory diff recorded (removed=%d all plugin-owned, added=%d installer-state only, changed=%d)", len(removedPaths), len(addedPaths), len(changedPaths))}

	// repeat remove must be refused, not silently succeed or crash
	repeatOut, repeatErr := claudeRunInstallerExpectError(t, f, installer, clientDir, "repeat-remove", "remove", "claude-native-proof", "--target", "claude", "--purge-data")
	if repeatErr == nil || !strings.Contains(repeatOut, "was not found") {
		t.Fatalf("repeated remove of an absent installation was not refused: err=%v out=%s", repeatErr, repeatOut)
	}
	stages["repeat_remove"] = claudeNativeStage{Status: "passed", Reason: "exact expected refusal message"}
}

func claudeExpectedMCPInventory() string {
	return "MCP servers (3)"
}

// Windows has the same projected stdio contract as the POSIX clients.
func claudeWindowsStdioPlan(r map[string]any, names ...string) error {
	data, _ := r["data"].(map[string]any)
	targets, _ := data["targets"].([]any)
	var result map[string]any
	if len(targets) == 1 {
		target, _ := targets[0].(map[string]any)
		if target["target"] != "claude" {
			return fmt.Errorf("unexpected Claude target: %+v", target)
		}
		output, _ := target["output"].(map[string]any)
		result, _ = output["result"].(map[string]any)
	} else if len(targets) == 0 {
		// The single-target CLI can return an unchanged operation directly.
		result, _ = data["result"].(map[string]any)
	} else {
		return fmt.Errorf("expected one Claude target: %+v", r)
	}
	plan, _ := result["plan"].(map[string]any)
	components, _ := plan["components"].([]any)
	if plan["client_id"] != "claude" {
		return fmt.Errorf("wrong client plan: %+v", plan)
	}
	for _, name := range names {
		count := 0
		for _, raw := range components {
			c, _ := raw.(map[string]any)
			if c["kind"] == "mcp_server" && c["name"] == name {
				count++
				if c["support"] != "projected" || (c["reason"] != nil && c["reason"] != "") {
					return fmt.Errorf("unexpected stdio classification: %+v", c)
				}
			}
		}
		if count != 1 {
			return fmt.Errorf("expected exactly one projected stdio component %q, got %d", name, count)
		}
	}
	return nil
}

func claudeAssertWindowsStdio(t *testing.T, root, installPath string, names ...string) {
	t.Helper()
	nativeRequireContained(t, root, installPath)
	body, err := os.ReadFile(filepath.Join(installPath, ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := claudeWindowsMCPProjection(body, names...); err != nil {
		t.Fatal(err)
	}
	var servers map[string]struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(body, &servers); err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		command := servers[name].Command
		nativeRequireContained(t, installPath, command)
		if st, err := os.Stat(command); err != nil || !st.Mode().IsRegular() {
			t.Fatalf("missing managed stdio helper %q: %v", command, err)
		}
	}
}

func TestClaudeWindowsStdioPlan(t *testing.T) {
	// A repeated user-scope add returns data.result rather than data.targets.
	const single = `{"data":{"result":{"mutated":false,"no_change":true,"plan":{"client_id":"claude","components":[{"kind":"mcp_server","name":"default","support":"projected"}]}}}}`
	if err := claudeWindowsStdioPlan(map[string]any{}, "default"); err == nil {
		t.Fatal("malformed result was accepted")
	}
	var direct map[string]any
	if err := json.Unmarshal([]byte(single), &direct); err != nil || claudeWindowsStdioPlan(direct, "default") != nil {
		t.Fatalf("legitimate no-change single-target result refused: %v", err)
	}
	const fixture = `{"data":{"targets":[{"target":"claude","output":{"result":{"plan":{"client_id":"claude","components":[{"kind":"mcp_server","name":"default","support":"projected"},{"kind":"mcp_server","name":"explicit","support":"projected"}]}}}}]}}`
	for _, tc := range []struct {
		name, input string
		wantError   bool
	}{
		{"exact", fixture, false},
		{"wrong support", strings.Replace(fixture, `"projected"`, `"unsupported"`, 1), true},
		{"wrong reason", strings.Replace(fixture, `"support":"projected"`, `"support":"projected","reason":"managed_stdio_platform_unsupported"`, 1), true},
		{"missing explicit", strings.Replace(fixture, `"explicit"`, `"other"`, 1), true},
		{"duplicate default", strings.Replace(fixture, `"explicit"`, `"default"`, 1), true},
		{"wrong client", strings.Replace(fixture, `"client_id":"claude"`, `"client_id":"codex"`, 1), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var r map[string]any
			if err := json.Unmarshal([]byte(tc.input), &r); err != nil {
				t.Fatal(err)
			}
			if err := claudeWindowsStdioPlan(r, "default", "explicit"); (err != nil) != tc.wantError {
				t.Fatalf("classification error = %v, wantError = %v", err, tc.wantError)
			}
		})
	}
}

// Claude's generated .mcp.json is a flat server-name map, not the portable
// package mcp.json envelope. Require the private managed launcher for every
// expected stdio server, preserving the independent network transport checks.
func claudeWindowsMCPProjection(body []byte, expected ...string) error {
	var servers map[string]map[string]any
	if err := json.Unmarshal(body, &servers); err != nil {
		return err
	}
	if len(servers) == 0 {
		return fmt.Errorf("missing projected HTTP inventory")
	}
	stdio := map[string]bool{}
	for _, name := range expected {
		server, ok := servers[name]
		if !ok {
			return fmt.Errorf("missing projected stdio: %s", name)
		}
		command, _ := server["command"].(string)
		args, _ := server["args"].([]any)
		if !strings.HasSuffix(strings.ReplaceAll(command, `\`, "/"), "/io.github.777genius.agentplugins/managed-stdio-v1/agentplugins.exe") || len(args) < 7 || args[0] != "--internal-stdio-v1" || args[5] != "--" || server["cwd"] != nil || server["url"] != nil || (server["type"] != nil && server["type"] != "stdio") {
			return fmt.Errorf("invalid managed stdio projection: %s %+v", name, server)
		}
		for _, value := range args {
			if _, ok := value.(string); !ok {
				return fmt.Errorf("non-string stdio argument: %s", name)
			}
		}
		if args[1] == "" || args[2] == "" || args[3] == "" || args[6] == "" || (args[4] != "plugin" && args[4] != "data") {
			return fmt.Errorf("invalid managed stdio roots/anchor/command: %s", name)
		}
		stdio[name] = true
	}
	for name, server := range servers {
		if stdio[name] {
			continue
		}
		if server["command"] != nil || server["args"] != nil || server["type"] == "stdio" {
			return fmt.Errorf("stdio command projected: %s %+v", name, server)
		}
		url, _ := server["url"].(string)
		if (server["type"] != "http" && server["type"] != "sse") || url == "" {
			return fmt.Errorf("invalid projected network server: %s %+v", name, server)
		}
	}
	return nil
}

func TestClaudeWindowsMCPProjection(t *testing.T) {
	// Exact flat shape written by projectClaudeMCP; streamable-http becomes http.
	const httpProjection = `{"http":{"type":"http","url":"http://127.0.0.1:1234/mcp"}}`
	const projection = `{"default":{"command":"C:/fixture/io.github.777genius.agentplugins/managed-stdio-v1/agentplugins.exe","args":["--internal-stdio-v1","C:/fixture","C:/data","C:/fixture","plugin","--","node","opaque"]},"http":{"type":"http","url":"http://127.0.0.1:1234/mcp"}}`
	for _, tc := range []struct {
		name, body string
		wantError  bool
	}{
		{"projected stdio and HTTP", projection, false},
		{"projected stdio HTTP and SSE", strings.TrimSuffix(projection, "}") + `,"sse":{"type":"sse","url":"http://127.0.0.1:9/sse"}}`, false},
		{"missing stdio", httpProjection, true},
		{"wrong mode", strings.Replace(projection, "--internal-stdio-v1", "--help", 1), true},
		{"wrong helper", strings.Replace(projection, "managed-stdio-v1/agentplugins.exe", "other/agentplugins.exe", 1), true},
		{"wrong anchor", strings.Replace(projection, `"plugin"`, `"unknown"`, 1), true},
		{"wrong separator", strings.Replace(projection, `"--"`, `"-"`, 1), true},
		{"non-string argv", strings.Replace(projection, `"opaque"`, `7`, 1), true},
		{"missing data", strings.Replace(projection, `"C:/data"`, `""`, 1), true},
		{"portable wrapper", `{"mcpServers":` + httpProjection + `}`, true},
		{"empty", `{}`, true},
		{"stdio replaced with network", `{"default":{"type":"http","url":"http://127.0.0.1/mcp"}}`, true},
		{"unexpected implicit stdio", strings.TrimSuffix(projection, "}") + `,"other":{"command":"sh","args":["-c","cat"]}}`, true},
		{"unexpected explicit stdio", strings.TrimSuffix(projection, "}") + `,"other":{"type":"stdio"}}`, true},
		{"hidden network command", strings.Replace(projection, `"type":"http"`, `"type":"http","command":"sh"`, 1), true},
		{"missing network URL", strings.Replace(projection, `"http://127.0.0.1:1234/mcp"`, `""`, 1), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := claudeWindowsMCPProjection([]byte(tc.body), "default"); (err != nil) != tc.wantError {
				t.Fatalf("projection error = %v, wantError = %v", err, tc.wantError)
			}
		})
	}
}

// claudeProfileFiles records only metadata and content hashes, never contents.
// WalkDir does not traverse links; symlink targets are hashed without opening.
type claudeProfileFile struct {
	Kind string `json:"kind"`
	SHA256 string `json:"sha256"`
	Mode string `json:"mode"`
	Size int64 `json:"size"`
}

type claudeProfileChange struct {
	Path string `json:"path"`
	Before *claudeProfileFile `json:"before,omitempty"`
	After *claudeProfileFile `json:"after,omitempty"`
}

func claudeProfileFiles(t *testing.T, root string) map[string]claudeProfileFile {
	t.Helper()
	files := map[string]claudeProfileFile{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		f := claudeProfileFile{Mode: info.Mode().String(), Size: info.Size()}
		var body []byte
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			f.Kind = "symlink"
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			body = []byte(target) // link text only; never open its destination
		case info.Mode().IsRegular():
			f.Kind = "file"
			body, err = os.ReadFile(path)
			if err != nil || int64(len(body)) != info.Size() {
				return fmt.Errorf("incomplete native profile inventory for %q: %v", rel, err)
			}
		default:
			return fmt.Errorf("unsupported native profile entry type at %q", rel)
		}
		h := sha256.Sum256(body)
		f.SHA256 = hex.EncodeToString(h[:])
		files[filepath.ToSlash(rel)] = f
		return nil
	})
	if err != nil {
		t.Fatalf("incomplete Claude native profile inventory: %v", err)
	}
	return files
}

// TestAgentpluginsClaudeNativeSecondaryProfileIsolation is a genuine pinned
// user-scope A→B→A test with a shared disposable HOME, not two unrelated users.
// Every new root belongs to this fixture; no native auth or model is requested.
func TestAgentpluginsClaudeNativeSecondaryProfileIsolation(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_CLAUDE_NATIVE_E2E") != "1" {
		t.Skip("opt-in real Claude A/B user profile E2E")
	}
	nativeRequireDisposable(t)
	client := claudeNativeBinary(t, "AGENTPLUGINS_CLAUDE_BIN")
	installer := claudeNativeBinary(t, "AGENTPLUGINS_INSTALLER_BIN")
	if expected := os.Getenv("AGENTPLUGINS_CLAUDE_SHA256"); len(expected) != 64 || claudeSHA256(t, client) != expected {
		t.Fatal("Claude binary pin mismatch")
	}
	f := newClaudeNativeFixture(t)
	clientDir := filepath.Dir(client)
	rootA, stateA := f.ConfigDir, f.StateHome
	rootB, stateB := filepath.Join(f.Root, "claude-config-B"), filepath.Join(f.Root, "installer-state-B")
	for _, path := range []string{rootB, stateB} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, root := range []string{rootA, rootB} {
		nativeWrite(t, filepath.Join(root, "skills", "foreign-neighbor", "SKILL.md"), []byte("foreign fixture data\n"), 0600)
	}
	shaA, shaB := nativeSHA(t, filepath.Join(rootA, "skills", "foreign-neighbor", "SKILL.md")), nativeSHA(t, filepath.Join(rootB, "skills", "foreign-neighbor", "SKILL.md"))
	assertSentinels := func() {
		t.Helper()
		if nativeSHA(t, filepath.Join(rootA, "skills", "foreign-neighbor", "SKILL.md")) != shaA ||
			nativeSHA(t, filepath.Join(rootB, "skills", "foreign-neighbor", "SKILL.md")) != shaB {
			t.Fatal("native cross-profile operation modified unrelated Skill bytes")
		}
	}
	claudeWriteFixturePackage(t, f.PackageRoot, "claude-native-proof", "1.0.0")
	wantSkill, err := os.ReadFile(filepath.Join(f.PackageRoot, "skills", "demo-skill", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	requireProfilePlugin := func(root, label string, installed bool) {
		t.Helper()
		count := 0
		for _, entry := range claudePluginList(t, f, client, label) {
			if entry.ID != "claude-native-proof@skills-dir" {
				continue
			}
			count++
			if !entry.Enabled || entry.Scope != "user" ||
				!strings.HasPrefix(filepath.Clean(entry.InstallPath), root+string(filepath.Separator)) {
				t.Fatalf("native plugin identity was not bound to the selected user profile")
			}
			if body := claudeReadInstalledSkill(t, entry.InstallPath); !bytes.Equal(body, wantSkill) {
				t.Fatal("selected profile plugin content differs from pinned fixture")
			}
		}
		if count > 1 || installed != (count == 1) {
			t.Fatalf("native selected profile plugin count=%d wantInstalled=%t", count, installed)
		}
	}
	// Both installer state roots and all shared XDG roots are independent
	// observed boundaries. A cold B is not initialized by enumerating files.
	roots := map[string]string{
		"profile_A": rootA, "profile_B": rootB,
		"state_A": stateA, "state_B": stateB,
		"shared_home": f.Home,
		"xdg_config": filepath.Join(f.Root, "xdg-config"),
		"xdg_data": filepath.Join(f.Root, "xdg-data"),
		"xdg_cache": filepath.Join(f.Root, "xdg-cache"),
		"xdg_state": filepath.Join(f.Root, "xdg-state"),
	}
	if err := os.MkdirAll(roots["xdg_state"], 0700); err != nil {
		t.Fatal(err)
	}
	snapshots := map[string]map[string]string{}
	fileInventories := map[string]map[string]map[string]claudeProfileFile{}
	fileChanges := map[string]map[string][]claudeProfileChange{}
	previous := map[string]map[string]claudeProfileFile{}
	capture := func(stage string) {
		t.Helper()
		snapshots[stage] = map[string]string{}
		fileInventories[stage] = map[string]map[string]claudeProfileFile{}
		fileChanges[stage] = map[string][]claudeProfileChange{}
		for label, path := range roots {
			snapshots[stage][label] = nativeTreeDigest(t, path)
			current := claudeProfileFiles(t, path)
			fileInventories[stage][label] = current
			if prior, ok := previous[label]; ok {
				changes := []claudeProfileChange{}
				for file, before := range prior {
					after, found := current[file]
					if !found {
						b := before
						changes = append(changes, claudeProfileChange{Path: file, Before: &b})
					} else if before != after {
						b, a := before, after
						changes = append(changes, claudeProfileChange{Path: file, Before: &b, After: &a})
					}
				}
				for file, after := range current {
					if _, found := prior[file]; !found {
						a := after
						changes = append(changes, claudeProfileChange{Path: file, After: &a})
					}
				}
				sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
				fileChanges[stage][label] = changes
			}
			previous[label] = current
		}
	}
	capture("before_any_native_command")
	if measured, want := claudeVersionString(t, f, client), os.Getenv("AGENTPLUGINS_CLAUDE_VERSION"); want == "" || measured != want {
		t.Fatalf("unqualified Claude binary/version: %q", measured)
	}
	capture("after_version_probe")
	stages := map[string]claudeNativeStage{
		"native_only_probe": {Status: "not_evaluated"},
		"profile_A_install": {Status: "not_evaluated"},
		"profile_B_install": {Status: "not_evaluated"},
		"A_replay":          {Status: "not_evaluated"},
		"B_only_remove":     {Status: "not_evaluated"},
	}
	defer func() {
		nativeJSON(t, filepath.Join(f.Root, "claude-secondary-profile-evidence.json"), map[string]any{
			"source_commit": os.Getenv("AGENTPLUGINS_INSTALLER_COMMIT"),
			"claude_binary_sha256": claudeSHA256(t, client),
			"source_version": os.Getenv("AGENTPLUGINS_CLAUDE_VERSION"),
			"shared_disposable_home": true, "distinct_config_and_state_roots": true,
			"model_execution": "NOT_RUN", "stages": stages, "snapshots": snapshots,
			"file_inventories": fileInventories, "file_changes": fileChanges,
		})
	}()
	// Record A's native-only initialization separately. B intentionally
	// remains cold: probing B first could hide the original #641 failure.
	for _, entry := range claudePluginList(t, f, client, "A-native-only") {
		if entry.ID == "claude-native-proof@skills-dir" {
			t.Fatal("cold A profile unexpectedly owns the plugin before UAP")
		}
	}
	stages["native_only_probe"] = claudeNativeStage{Status: "passed", Reason: "A native-only initialized; B was deliberately left cold for the exact secondary-profile regression"}
	capture("after_A_native_only_initialization")
	preB := nativeTreeDigest(t, rootB)
	f.ConfigDir, f.StateHome = rootA, stateA
	first := claudeRunInstaller(t, f, installer, clientDir, "A-install", "add", f.PackageRoot, "--target", "claude")
	if first["result"] != "success" {
		t.Fatalf("A install did not succeed: %+v", first)
	}
	requireProfilePlugin(rootA, "A-list-after-install", true)
	stages["profile_A_install"] = claudeNativeStage{Status: "passed"}
	capture("after_A_install")
	assertSentinels()
	if nativeTreeDigest(t, rootB) != preB {
		t.Fatal("selected profile A changed unselected profile B")
	}
	f.ConfigDir, f.StateHome = rootB, stateB
	stableA := nativeTreeDigest(t, rootA)
	second := claudeRunInstaller(t, f, installer, clientDir, "B-install", "add", f.PackageRoot, "--target", "claude")
	capture("after_B_install_attempt")
	assertSentinels()
	if nativeTreeDigest(t, rootA) != stableA {
		t.Fatal("selected profile B changed unselected profile A")
	}
	if second["result"] != "success" {
		stages["profile_B_install"] = claudeNativeStage{Status: "blocked", Reason: "native identity ownership unresolved in the cold selected B profile"}
	} else {
		requireProfilePlugin(rootB, "B-list-after-install", true)
		stages["profile_B_install"] = claudeNativeStage{Status: "passed"}
	}
	f.ConfigDir, f.StateHome = rootA, stateA
	stableB := nativeTreeDigest(t, rootB)
	replayed := claudeRunInstaller(t, f, installer, clientDir, "A-replay", "add", f.PackageRoot, "--target", "claude")
	if replayed["result"] != "success" || nativeTreeDigest(t, rootB) != stableB {
		t.Fatalf("A replay modified B or failed: %+v", replayed)
	}
	assertSentinels()
	requireProfilePlugin(rootA, "A-list-after-replay", true)
	stages["A_replay"] = claudeNativeStage{Status: "passed"}
	capture("after_A_replay")
	if second["result"] != "success" {
		// A probe only AFTER the failed B attempt cannot turn a cold-profile
		// refusal into a false successful setup. Record native list evidence
		// while preserving A's independently proven replay.
		f.ConfigDir, f.StateHome = rootB, stateB
		entries := claudePluginList(t, f, client, "B-native-list-after-refusal")
		fromA := false
		for _, entry := range entries {
			if strings.HasPrefix(filepath.Clean(entry.InstallPath), rootA+string(filepath.Separator)) {
				fromA = true
			}
		}
		capture("after_B_native_list_diagnostic")
		t.Fatalf("cold B install blocked; A replay preserved, B native list entries=%d, sibling_A_visible=%t", len(entries), fromA)
	}
	f.ConfigDir, f.StateHome = rootB, stateB
	stableA = nativeTreeDigest(t, rootA)
	removed := claudeRunInstaller(t, f, installer, clientDir, "B-only-remove", "remove", "claude-native-proof", "--target", "claude")
	if removed["result"] != "success" || nativeTreeDigest(t, rootA) != stableA {
		t.Fatalf("B-only removal changed A or failed: %+v", removed)
	}
	assertSentinels()
	requireProfilePlugin(rootB, "B-list-after-remove", false)
	stages["B_only_remove"] = claudeNativeStage{Status: "passed"}
	capture("after_B_only_remove")
	f.ConfigDir, f.StateHome = rootA, stateA
	requireProfilePlugin(rootA, "A-list-after-B-remove", true)
}
