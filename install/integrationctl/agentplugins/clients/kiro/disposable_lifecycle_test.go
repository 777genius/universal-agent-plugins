package kiro

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestKiroDisposableAddUpdateACPFailureRemove(t *testing.T) {
	root := t.TempDir()
	home, project, state := filepath.Join(root, "home"), filepath.Join(root, "project"), filepath.Join(root, "state")
	for _, path := range []string{home, project, state} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	configRoot := filepath.Join(home, ".kiro")
	path := filepath.Join(configRoot, "settings", "mcp.json")
	original := `{
  // independent telemetry preference
  "telemetry": { "enabled": false },
  "mcpServers": {
    // foreign service and its formatting must survive
    "unmanaged": { "url" : "https://foreign.test/mcp", "limit": 9007199254740993 },
  },
}
`
	writeTestFile(t, path, original)
	adapter := &Adapter{}
	var previous []domain.NativeObjectOwnership
	for _, version := range []string{"add", "update"} {
		active, desired := kiroNativeFixture(t, configRoot, version, "https://docs.test/"+version)
		request := kiroActivationRequest(configRoot, active, previous, desired)
		outcome, err := adapter.Activate(context.Background(), clients.Env{}, request)
		if err != nil || outcome.Activation != domain.ActivationPrepared || outcome.NativeEffect != domain.NativeEffectCommitted || !reflect.DeepEqual(outcome.NativeObjects, desired) {
			t.Fatalf("%s outcome: %+v %v", version, outcome, err)
		}
		assertKiroDisposableBytes(t, path, version)
		if err := VerifyNativeObjects(configRoot, outcome.NativeObjects, false); err != nil {
			t.Fatal(err)
		}
		// Read receipts back from synthetic state to drive the next operation.
		receiptBody, err := json.Marshal(outcome.NativeObjects)
		if err != nil {
			t.Fatal(err)
		}
		receiptPath := filepath.Join(state, "receipts.json")
		if err := os.WriteFile(receiptPath, receiptBody, 0600); err != nil {
			t.Fatal(err)
		}
		readback, err := os.ReadFile(receiptPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(readback, &previous); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(previous, desired) {
			t.Fatal("receipt round-trip changed exact ownership")
		}
	}
	// A recognized authentication failure occurs only after the native commit.
	active, desired := kiroNativeFixture(t, configRoot, "auth-failure", "https://docs.test/auth-failure")
	request := kiroActivationRequest(configRoot, active, previous, desired)
	request.Plan.InstallIntent = domain.InstallIntentAutomatic
	runner := &recordingRunner{duplexOutput: acpResponse(0, `{"protocolVersion":1}`) + acpResponse(1, `{"sessionId":"s"}`) + acpStatus("s", "docs", "auth-required", "")}
	outcome, err := adapter.Activate(context.Background(), clients.Env{Runner: runner}, request)
	if !errors.Is(err, shared.ErrRecognizedNegativeEvidence) || outcome.Activation != domain.ActivationFailed || outcome.NativeEffect != domain.NativeEffectCommitted || !reflect.DeepEqual(outcome.NativeObjects, desired) {
		t.Fatalf("auth failure erased native commit: %+v %v", outcome, err)
	}
	if len(runner.commands) != 1 || runner.commands[0].Dir != active {
		t.Fatalf("ACP fixture calls: %+v", runner.commands)
	}
	assertKiroDisposableBytes(t, path, "auth-failure")
	if err := VerifyNativeObjects(configRoot, desired, false); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(desired[0].Path, "SKILL.md"))
	if string(body) != "auth-failure\n" {
		t.Fatalf("ACP failure restored old skill: %s", body)
	}
	removed, err := adapter.Deactivate(context.Background(), clients.Env{}, domain.DeactivationRequest{Client: request.Client, NativeObjects: outcome.NativeObjects, Confirmed: true})
	if err != nil || !removed.ExternalRemovalComplete {
		t.Fatalf("remove: %+v %v", removed, err)
	}
	assertKiroDisposableBytes(t, path, "")
	if _, err := os.Lstat(desired[0].Path); !os.IsNotExist(err) {
		t.Fatalf("managed skill survived remove: %v", err)
	}
	recovery, err := filepath.Glob(filepath.Join(configRoot, "skills", ".agentplugins-native-*"))
	if err != nil || len(recovery) != 0 {
		t.Fatalf("completed operations left recovery: %v %v", recovery, err)
	}
	t.Log("disposable add -> update -> ACP auth failure -> remove: bytes, exact receipts, foreign comments/numbers and recovery paths verified; ACP runner is a fixture")
}

func assertKiroDisposableBytes(t *testing.T, path, version string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"// independent telemetry preference", "// foreign service and its formatting must survive", `"unmanaged": { "url" : "https://foreign.test/mcp", "limit": 9007199254740993 }`, `"telemetry": { "enabled": false }`} {
		if !bytes.Contains(body, []byte(text)) {
			t.Fatalf("foreign bytes/comments lost: %s", body)
		}
	}
	servers, _, _, _, err := ReadMCPConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if version == "" {
		if _, ok := servers["docs"]; ok {
			t.Fatalf("managed MCP survives removal: %s", body)
		}
	} else {
		server, ok := servers["docs"].(map[string]any)
		if !ok || server["url"] != "https://docs.test/"+version {
			t.Fatalf("MCP mismatch: %s", body)
		}
	}
}

func TestKiroHuJSONRetainsStrictOwnershipChecks(t *testing.T) {
	for name, body := range map[string]string{
		"duplicate":             `{/*comment*/"mcpServers":{},"mcpServers":{}}`,
		"case":                  `{"mcpServers":{},"McpServers":{}}`,
		"wrong collection case": `{"McpServers":{}}`,
		"nested case":           `{"mcpServers":{"foreign":{"env":{"A":1,"a":2}}}}`,
		"unicode case":          `{"mcpServers":{"foreign":{"K":1,"K":2}}}`,
		"structure":             `{"mcpServers":[]}`,
		"root":                  `[]`,
		"numeric":               `{"unused":1e999,"mcpServers":{}}`,
		"trailing":              `{} {}`,
	} {
		t.Run(name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), ".kiro")
			path := filepath.Join(root, "settings", "mcp.json")
			writeTestFile(t, path, body)
			active, desired := kiroNativeFixture(t, root, "new", "https://new.test")
			if err := applyKiroNativeMutation(root, active, nil, desired); err == nil {
				t.Fatal("unsafe configuration accepted")
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != body {
				t.Fatalf("failed parse mutated bytes: %s %v", got, err)
			}
			if _, err := os.Lstat(desired[0].Path); !os.IsNotExist(err) {
				t.Fatalf("failed parse installed skill: %v", err)
			}
		})
	}
}

func TestKiroReadMCPRejectsSymlinkBeforeParsing(t *testing.T) {
	root := t.TempDir()
	target, link := filepath.Join(root, "foreign.json"), filepath.Join(root, "mcp.json")
	writeTestFile(t, target, `{"mcpServers":{}}`)
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink privilege: %v", err)
	}
	if _, _, _, _, err := ReadMCPConfig(link); err == nil {
		t.Fatal("symlink accepted")
	}
	body, err := os.ReadFile(target)
	if err != nil || strings.TrimSpace(string(body)) != `{"mcpServers":{}}` {
		t.Fatalf("foreign file changed: %s %v", body, err)
	}
}

func TestKiroReadMCPKeepsOriginalBytesAndLegacyDigest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp.json")
	original := []byte("{ // keep this comment\n\"mcpServers\":{\"docs\":{\"url\":\"https://docs.test\",\"n\":9007199254740993,\"exponent\":1e2}},}\n")
	writeTestFile(t, path, string(original))
	servers, body, _, exists, err := ReadMCPConfig(path)
	if err != nil || !exists || !bytes.Equal(body, original) {
		t.Fatalf("read changed original bytes: %s %v", body, err)
	}
	expected := map[string]any{"url": "https://docs.test", "n": json.Number("9007199254740993"), "exponent": json.Number("1e2")}
	got, ok := servers["docs"].(map[string]any)
	if !ok || shared.DigestJSONObject(got) != shared.DigestJSONObject(expected) {
		t.Fatalf("ownership digest changed: %+v", servers)
	}
	if err := verifyKiroMCPDigest(servers, domain.NativeObjectOwnership{LogicalName: "docs", ManagedDigest: shared.DigestJSONObject(expected)}, false); err != nil {
		t.Fatal(err)
	}
	rendered, err := encodeKiroMCPConfig(body, servers)
	if err != nil || !bytes.Equal(rendered, original) {
		t.Fatalf("no-op changed bytes: %s %v", rendered, err)
	}
}
