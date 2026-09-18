package cline

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestClineSkillRollbackRetainsBackupWhenRestoreRenameFails(t *testing.T) {
	root := t.TempDir()
	configRoot := filepath.Join(root, ".cline")
	activeV1 := filepath.Join(root, "managed", "v1")
	writeTestFile(t, filepath.Join(activeV1, "skills", "docs", "SKILL.md"), "v1\n")
	desiredV1 := clineFixtureObjects(t, configRoot, activeV1, "docs", "", nativeconfig.Server{})
	if err := applyClineNativeMutation(configRoot, activeV1, desiredV1); err != nil {
		t.Fatal(err)
	}
	activeV2 := filepath.Join(root, "managed", "v2")
	writeTestFile(t, filepath.Join(activeV2, "skills", "docs", "SKILL.md"), "v2\n")
	desiredV2 := clineFixtureObjects(t, configRoot, activeV2, "docs", "", nativeconfig.Server{})
	activationErr := errors.New("injected Cline activation rename failure")
	restoreErr := errors.New("injected Cline restore rename failure")
	rename := func(oldPath, newPath string) error {
		switch {
		case strings.HasPrefix(filepath.Base(oldPath), "new-"):
			return activationErr
		case strings.HasPrefix(filepath.Base(oldPath), "old-"):
			return restoreErr
		default:
			return os.Rename(oldPath, newPath)
		}
	}

	err := applyClineNative(clients.Env{NativeConfig: nativeconfig.New()}, clients.Ops{Rename: rename}, configRoot, activeV2, desiredV1, desiredV2, nil)
	if err == nil || !strings.Contains(err.Error(), activationErr.Error()) || !strings.Contains(err.Error(), restoreErr.Error()) || !strings.Contains(err.Error(), "recovery retained at") {
		t.Fatalf("rollback error = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(configRoot, "skills", "docs")); !os.IsNotExist(statErr) {
		t.Fatalf("failed restore unexpectedly recreated target: %v", statErr)
	}
	transactions, globErr := filepath.Glob(filepath.Join(configRoot, "skills", ".agentplugins-native-*"))
	if globErr != nil || len(transactions) != 1 {
		t.Fatalf("retained Cline transactions = %v, %v", transactions, globErr)
	}
	if !strings.Contains(err.Error(), transactions[0]) {
		t.Fatalf("rollback error omitted exact recovery path %q: %v", transactions[0], err)
	}
	backup := filepath.Join(transactions[0], "old-docs", "SKILL.md")
	body, readErr := os.ReadFile(backup)
	if readErr != nil || string(body) != "v1\n" {
		t.Fatalf("recoverable Cline backup = %q, %v", body, readErr)
	}
}

func TestClineSkillBackupDigestMismatchRestoresLiveDirectoryAndAborts(t *testing.T) {
	root := t.TempDir()
	configRoot := filepath.Join(root, ".cline")
	activeV1 := filepath.Join(root, "managed", "v1")
	writeTestFile(t, filepath.Join(activeV1, "skills", "docs", "SKILL.md"), "v1\n")
	desiredV1 := clineFixtureObjects(t, configRoot, activeV1, "docs", "", nativeconfig.Server{})
	if err := applyClineNativeMutation(configRoot, activeV1, desiredV1); err != nil {
		t.Fatal(err)
	}
	activeV2 := filepath.Join(root, "managed", "v2")
	writeTestFile(t, filepath.Join(activeV2, "skills", "docs", "SKILL.md"), "v2\n")
	desiredV2 := clineFixtureObjects(t, configRoot, activeV2, "docs", "", nativeconfig.Server{})
	liveSkill := filepath.Join(configRoot, "skills", "docs")
	rename := func(oldPath, newPath string) error {
		if shared.SameCleanPath(oldPath, liveSkill) && strings.HasPrefix(filepath.Base(newPath), "old-") {
			writeTestFile(t, filepath.Join(oldPath, "SKILL.md"), "concurrent user change\n")
		}
		return os.Rename(oldPath, newPath)
	}

	err := applyClineNative(clients.Env{NativeConfig: nativeconfig.New()}, clients.Ops{Rename: rename}, configRoot, activeV2, desiredV1, desiredV2, nil)
	if err == nil || !strings.Contains(err.Error(), "isolated Cline skill backup") {
		t.Fatalf("TOCTOU activation error = %v", err)
	}
	body, readErr := os.ReadFile(filepath.Join(liveSkill, "SKILL.md"))
	if readErr != nil || string(body) != "concurrent user change\n" {
		t.Fatalf("restored live Cline skill = %q, %v", body, readErr)
	}
}

func TestRenameClineDirectoryNoReplacePreservesLateTarget(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "staged")
	target := filepath.Join(root, "live")
	writeTestFile(t, filepath.Join(source, "SKILL.md"), "managed\n")
	writeTestFile(t, filepath.Join(target, "SKILL.md"), "late unmanaged\n")
	err := renameClineDirectoryNoReplace(source, target, shared.RenameDirectoryExclusive)
	if err == nil {
		t.Fatalf("no-replace result = %v", err)
	}
	body, readErr := os.ReadFile(filepath.Join(target, "SKILL.md"))
	if readErr != nil || string(body) != "late unmanaged\n" {
		t.Fatalf("late target = %q, %v", body, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(source, "SKILL.md")); statErr != nil {
		t.Fatalf("staged source was consumed: %v", statErr)
	}
}

func TestClineProjectsChromeLikeStdioWithoutAuthorCWD(t *testing.T) {
	root := t.TempDir()
	envelope := domain.PackageEnvelope{MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{
		"chrome": {Name: "chrome", Type: "stdio", Decoded: map[string]any{
			"type": "stdio", "command": "npx",
			"args": []any{"-y", "chrome-devtools-mcp@latest", "--cache", "${PLUGIN_DATA}/cache"},
			"env":  map[string]any{"MODE": "safe"},
		}},
	}}}
	plan := domain.DeliveryPlan{ActivePath: filepath.Join(root, "active"), Components: []domain.ComponentDecision{
		{Kind: domain.ComponentMCPServer, Name: "chrome", Support: domain.SupportPrepared},
	}}
	dataPath := filepath.Join(root, "data")
	if err := ProjectNative(root, envelope, plan, dataPath); err != nil {
		t.Fatalf("project Chrome-like Cline stdio server: %v", err)
	}
	projection, err := readClineProjection(root)
	if err != nil {
		t.Fatal(err)
	}
	server := projection.Servers["chrome"]
	if server.Command != "npx" || server.CWD != plan.ActivePath || len(server.Args) != 4 || server.Args[3] != filepath.Join(dataPath, "cache") {
		t.Fatalf("Cline stdio projection = %+v", server)
	}
	if server.Env["PLUGIN_ROOT"] != plan.ActivePath || server.Env["PLUGIN_DATA"] != dataPath || server.Env["MODE"] != "safe" {
		t.Fatalf("Cline stdio environment = %+v", server.Env)
	}
}

func TestClinePreservesExplicitStdioCWD(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "workspace"), 0o700); err != nil {
		t.Fatal(err)
	}
	envelope := domain.PackageEnvelope{MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{
		"local": {Name: "local", Type: "stdio", Decoded: map[string]any{"type": "stdio", "command": "node", "cwd": "./workspace"}},
	}}}
	plan := domain.DeliveryPlan{ActivePath: filepath.Join(root, "active"), Components: []domain.ComponentDecision{
		{Kind: domain.ComponentMCPServer, Name: "local", Support: domain.SupportPrepared},
	}}
	err := ProjectNative(root, envelope, plan, filepath.Join(root, "data"))
	if err != nil {
		t.Fatal(err)
	}
	projection, err := readClineProjection(root)
	if err != nil {
		t.Fatal(err)
	}
	if projection.Servers["local"].CWD != filepath.Join(plan.ActivePath, "workspace") {
		t.Fatalf("cwd = %q", projection.Servers["local"].CWD)
	}

}

func TestClineCollisionAndBusyLockLeaveNoPartialActivation(t *testing.T) {
	for _, test := range []struct {
		name        string
		invalidLock bool
	}{
		{name: "foreign collision"},
		{name: "invalid lock", invalidLock: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			configRoot := filepath.Join(root, ".cline")
			settings := filepath.Join(root, "settings.json")
			t.Setenv("CLINE_MCP_SETTINGS_PATH", settings)
			foreign := `{"mcpServers":{"docs":{"transport":{"type":"stdio","command":"foreign"}}}}`
			if test.invalidLock {
				foreign = `{"mcpServers":{}}`
				writeTestFile(t, settings+".lock", "not a compatible Cline lock")
			}
			writeTestFile(t, settings, foreign)
			active := filepath.Join(root, "managed", "demo")
			writeTestFile(t, filepath.Join(active, "skills", "guide", "SKILL.md"), "guide")
			server := nativeconfig.Server{Type: "stdio", Command: "node"}
			writeClineProjectionFixture(t, active, map[string]nativeconfig.Server{"docs": server})
			desired := clineFixtureObjects(t, configRoot, active, "guide", "docs", server)
			err := applyClineNativeMutation(configRoot, active, desired)
			if err == nil {
				t.Fatal("unsafe activation unexpectedly succeeded")
			}
			if test.invalidLock && !strings.Contains(err.Error(), "Cline native config lock") {
				t.Fatalf("wrong lock error: %v", err)
			}
			body, _ := os.ReadFile(settings)
			if string(body) != foreign {
				t.Fatalf("config changed on failure: %s", body)
			}
			if _, err := os.Lstat(filepath.Join(configRoot, "skills", "guide")); !os.IsNotExist(err) {
				t.Fatalf("partial skill survived: %v", err)
			}
		})
	}
}

func TestClineTamperedReceiptFailsClosed(t *testing.T) {
	root := t.TempDir()
	configRoot := filepath.Join(root, ".cline")
	settings := filepath.Join(root, "settings.json")
	t.Setenv("CLINE_MCP_SETTINGS_PATH", settings)
	writeTestFile(t, settings, `{"mcpServers":{}}`)
	active := filepath.Join(root, "managed", "demo")
	server := nativeconfig.Server{Type: "stdio", Command: "node"}
	writeClineProjectionFixture(t, active, map[string]nativeconfig.Server{"docs": server})
	desired := clineFixtureObjects(t, configRoot, active, "", "docs", server)
	if err := applyClineNativeMutation(configRoot, active, desired); err != nil {
		t.Fatal(err)
	}
	desired[0].ManagedDigest = "sha256:00"
	if err := VerifyNativeObjects(configRoot, desired, false); !errors.Is(err, nativeconfig.ErrNotOwned) && !strings.Contains(err.Error(), "changed outside") {
		t.Fatalf("tamper was not rejected: %v", err)
	}
}

func writeClineProjectionFixture(t *testing.T, active string, servers map[string]nativeconfig.Server) {
	t.Helper()
	body, err := json.Marshal(clineProjection{Servers: servers})
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(active, clineProjectionFile), string(body))
}

func clineFixtureObjects(t *testing.T, configRoot, active, skillName, serverName string, server nativeconfig.Server) []domain.NativeObjectOwnership {
	t.Helper()
	var objects []domain.NativeObjectOwnership
	if skillName != "" {
		digest, err := shared.DigestSkillDirectory(filepath.Join(active, "skills", skillName))
		if err != nil {
			t.Fatal(err)
		}
		objects = append(objects, domain.NativeObjectOwnership{ObjectID: "cline-skill:" + skillName, Kind: clineSkillObjectKind, LogicalName: skillName,
			Path: filepath.Join(configRoot, "skills", skillName), SourceRelative: filepath.ToSlash(filepath.Join("skills", skillName)), ManagedDigest: digest, ProtectionClass: "managed"})
	}
	if serverName != "" {
		receipt, err := nativeconfig.DesiredReceipt(clineMCPSettingsPath(configRoot), nativeconfig.CodecCline, serverName, server, nativeconfig.Placeholders{})
		if err != nil {
			t.Fatal(err)
		}
		objects = append(objects, domain.NativeObjectOwnership{ObjectID: "cline-mcp:" + serverName, Kind: clineMCPObjectKind, LogicalName: serverName,
			Path: clineMCPSettingsPath(configRoot), SourceRelative: clineProjectionFile, ManagedDigest: receipt.Digest, ProtectionClass: "managed"})
	}
	return objects
}

func TestClineNeutralCWDTypeValidation(t *testing.T) {
	for _, value := range []any{false, 42, []string{"work"}} {
		if _, err := clineNeutralServer(domain.MCPServer{Type: "stdio", Decoded: map[string]any{"command": "node", "cwd": value}}); err == nil {
			t.Fatalf("accepted cwd %#v", value)
		}
	}
}

func TestClineProjectionRoundTripExpandsPortableValuesOnlyOnce(t *testing.T) {
	root := t.TempDir()
	active := filepath.Join(root, "${PLUGIN_DATA}", "${PLUGIN_ROOT}")
	envelope := domain.PackageEnvelope{MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{"local": {Type: "stdio", Decoded: map[string]any{"command": "node", "args": []any{"${PLUGIN_ROOT}/run.js", "${UNKNOWN}"}, "env": map[string]any{"ROOT": "${PLUGIN_ROOT}", "OTHER": "${HOME}"}}}}}}
	plan := domain.DeliveryPlan{ActivePath: active, Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "local", Support: domain.SupportNative}}}
	if err := ProjectNative(root, envelope, plan, filepath.Join(root, "data")); err != nil {
		t.Fatal(err)
	}
	projection, err := readClineProjection(root)
	if err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(root, "native.json")
	if _, err := nativeconfig.New().Apply(nativeconfig.Request{Paths: nativeconfig.Paths{JSON: settings}, Codec: nativeconfig.CodecCline, Action: nativeconfig.ActionAdd, Name: "local", Server: projection.Servers["local"], Placeholders: nativeconfig.Placeholders{PackageRoot: active, DataRoot: "/wrong-second-pass"}}); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	body, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	transport := doc["mcpServers"].(map[string]any)["local"].(map[string]any)["transport"].(map[string]any)
	if transport["cwd"] != active || transport["args"].([]any)[0] != filepath.Join(active, "run.js") || transport["env"].(map[string]any)["ROOT"] != active {
		t.Fatalf("second-pass expansion: %#v", transport)
	}
}
