//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func clineContractClone(t *testing.T, m map[string]any) map[string]any {
	t.Helper()
	raw, _ := json.Marshal(m)
	v, err := projectNativeRuntimeJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	return v.(map[string]any)
}
func clineContractProvider() *clineProjectProvider {
	return &clineProjectProvider{nonce: "0123456789abcdef0123456789abcdef", callID: "owned-call", marker: "retained", prompt: "owned-prompt", issued: true}
}
func clineContractPermission(p *clineProjectProvider) map[string]any {
	return map[string]any{"sessionId": "1793012345678_abcD5_cli", "toolCall": map[string]any{"toolCallId": p.callID, "title": clineAttemptTitle(p.arguments()), "kind": "other", "status": "pending", "rawInput": p.arguments()}, "options": []any{map[string]any{"optionId": "allow_once", "name": "Allow once", "kind": "allow_once"}, map[string]any{"optionId": "allow_always", "name": "Allow always", "kind": "allow_always"}, map[string]any{"optionId": "reject_once", "name": "Reject", "kind": "reject_once"}}}
}

func TestClineProjectNativeOriginalDeadlineAndDelegationCannotBeReplaced(t *testing.T) {
	origin := int64(time.Hour)
	work, total, err := clineAttemptRemaining(origin, origin+int64(40*time.Second))
	if err != nil || work != 110*time.Second || total != 140*time.Second {
		t.Fatal("service startup did not consume original deadline")
	}
	for _, now := range []int64{origin - 1, origin + int64(150*time.Second), origin + int64(181*time.Second)} {
		if _, _, err := clineAttemptRemaining(origin, now); err == nil {
			t.Fatal("future/exhausted original origin accepted")
		}
	}
	unit := "uap-cline-project-0123456789abcdef0123456789abcdef.service"
	membership := "0::/system.slice/" + unit + "\n"
	if err := clineAttemptUnit(unit, membership, 1001, 1001, 1001, 1001); err != nil {
		t.Fatal(err)
	}
	for _, change := range []struct {
		unit, membership                   string
		uid, gid, expectedUID, expectedGID int
	}{{unit, membership, 0, 0, 0, 0}, {unit, membership, 1001, 1001, 1002, 1001}, {unit, "0::/alternate/" + unit + "\n", 1001, 1001, 1001, 1001}, {unit, membership + "0::/other\n", 1001, 1001, 1001, 1001}, {"foreign.service", membership, 1001, 1001, 1001, 1001}} {
		if clineAttemptUnit(change.unit, change.membership, change.uid, change.gid, change.expectedUID, change.expectedGID) == nil {
			t.Fatal("unrelated/root/fabricated membership admitted")
		}
	}
}
func TestClineProjectNativePermissionBindsNativeSessionProviderCallAndExactToolInput(t *testing.T) {
	p := clineContractProvider()
	good := clineContractPermission(p)
	session := good["sessionId"].(string)
	if err := clineAttemptPermission(good, session, p.callID, p.arguments()); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"session", "call", "builtin", "nonce", "operation", "marker", "extra-input", "extra-tool", "extra-request", "option", "autoapproval"} {
		m := clineContractClone(t, good)
		tool := m["toolCall"].(map[string]any)
		input := tool["rawInput"].(map[string]any)
		switch kind {
		case "session":
			m["sessionId"] = "different-native-session"
		case "call":
			tool["toolCallId"] = "other-provider-call"
		case "builtin":
			tool["title"] = "run_commands"
		case "nonce":
			input["nonce"] = "wrong"
		case "operation":
			input["operation"] = "read"
		case "marker":
			input["marker"] = "wrong"
		case "extra-input":
			input["shell"] = "never"
		case "extra-tool":
			tool["name"] = "guessed"
		case "extra-request":
			m["unknown"] = true
		case "option":
			m["options"].([]any)[0].(map[string]any)["optionId"] = "allow_always"
		case "autoapproval":
			tool["status"] = "in_progress"
		}
		if clineAttemptPermission(m, session, p.callID, p.arguments()) == nil {
			t.Fatal("unsafe permission admitted", kind)
		}
	}
}
func TestClineProjectNativeInstalledServerRejectsWrongManagedProjection(t *testing.T) {
	args := []any{"Owned Cline A", "/owned/A/package/argument"}
	env := map[string]any{"PLUGIN_ROOT": "/owned/A/package", "PLUGIN_DATA": "/owned/A/data"}
	good := map[string]any{"transport": map[string]any{"type": "stdio", "command": "/owned/A/package/bin/probe", "args": args, "env": env, "cwd": "/owned/A/package"}}
	if err := clineAttemptServer(good, "/owned/A/package/bin/probe", "/owned/A/package", args, env); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"command", "data", "package", "env", "cwd", "approval", "transport"} {
		m := clineContractClone(t, good)
		transport := m["transport"].(map[string]any)
		switch kind {
		case "command":
			transport["command"] = "/other/probe"
		case "data":
			transport["env"].(map[string]any)["PLUGIN_DATA"] = "/owned/B/data"
		case "package":
			transport["args"].([]any)[1] = "/owned/B/package/argument"
		case "env":
			transport["env"].(map[string]any)["PLUGIN_ROOT"] = "/owned/B/package"
		case "cwd":
			transport["cwd"] = "/other"
		case "approval":
			m["autoApprove"] = []any{"inspect_runtime"}
		case "transport":
			transport["type"] = "sse"
		}
		if clineAttemptServer(m, "/owned/A/package/bin/probe", "/owned/A/package", args, env) == nil {
			t.Fatal("wrong installed projection accepted", kind)
		}
	}
}
func TestClineProjectNativeClosureIncludesDirectoryMembershipModeAndBytes(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "owned")
	projectNativeRuntimeWrite(t, path, []byte("inert owned bytes"), 0600)
	before, err := clineAttemptTree(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "extra"), 0700); err != nil {
		t.Fatal(err)
	}
	after, err := clineAttemptTree(root)
	if err != nil || reflect.DeepEqual(before, after) {
		t.Fatal("empty directory addition omitted")
	}
	if err := os.Remove(filepath.Join(root, "extra")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0700); err != nil {
		t.Fatal(err)
	}
	after, err = clineAttemptTree(root)
	if err != nil || reflect.DeepEqual(before, after) {
		t.Fatal("same-byte chmod omitted")
	}
	if err := os.Symlink(path, filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	if _, err := clineAttemptTree(root); err == nil {
		t.Fatal("aliased owned tree admitted")
	}
}
func clineContractRequest(p *clineProjectProvider) map[string]any {
	return map[string]any{"model": clineAttemptModel, "stream": true, "messages": []any{map[string]any{"role": "system", "content": "inert source fixture"}, map[string]any{"role": "user", "content": p.prompt}}, "tools": []any{map[string]any{"type": "function", "function": map[string]any{"name": clineAttemptTool, "description": "Inspect only disposable fixture runtime facts.", "parameters": map[string]any{"type": "object", "properties": map[string]any{"nonce": map[string]any{"type": "string"}, "operation": map[string]any{"type": "string"}, "marker": map[string]any{"type": "string"}}, "required": []any{"nonce"}, "additionalProperties": false}}}}}
}
func TestClineProjectNativeProviderCannotInventToolSchemasOrHistory(t *testing.T) {
	p := clineContractProvider()
	good := clineContractRequest(p)
	raw, _ := json.Marshal(good)
	if _, err := p.validateRequest(raw, 0); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"model", "stream", "unknown", "schema", "foreign-tool", "duplicate-tool", "prompt", "premature-result"} {
		m := clineContractClone(t, good)
		switch kind {
		case "model":
			m["model"] = "external-model"
		case "stream":
			m["stream"] = false
		case "unknown":
			m["external_url"] = "https://invalid"
		case "schema":
			m["tools"].([]any)[0].(map[string]any)["function"].(map[string]any)["parameters"].(map[string]any)["additionalProperties"] = true
		case "foreign-tool":
			m["tools"].([]any)[0].(map[string]any)["function"].(map[string]any)["name"] = "foreign__inspect_runtime"
		case "duplicate-tool":
			m["tools"] = append(m["tools"].([]any), m["tools"].([]any)[0])
		case "prompt":
			m["messages"].([]any)[1].(map[string]any)["content"] = "unrelated"
		case "premature-result":
			m["messages"] = append(m["messages"].([]any), map[string]any{"role": "tool", "content": "{}", "tool_call_id": "foreign"})
		}
		raw, _ := json.Marshal(m)
		if _, err := p.validateRequest(raw, 0); err == nil {
			t.Fatal("unowned provider request accepted", kind)
		}
	}
}
func clineContractSession() map[string]any {
	return map[string]any{"sessionId": "1793012345678_abcD5_cli", "modes": map[string]any{"availableModes": []any{map[string]any{"id": "plan", "name": "Plan", "description": "Explore the codebase and plan changes without modifying files"}, map[string]any{"id": "act", "name": "Act", "description": "Make changes to the codebase"}}, "currentModeId": "act"}, "models": map[string]any{"availableModels": []any{map[string]any{"modelId": clineAttemptModel, "name": "UAP scripted model"}}, "currentModelId": clineAttemptModel}, "configOptions": []any{map[string]any{"type": "select", "id": "provider", "name": "Provider", "category": "model", "currentValue": clineAttemptProviderID, "options": []any{}}, map[string]any{"type": "select", "id": "model", "name": "Model", "category": "model", "currentValue": clineAttemptModel, "options": []any{}}, map[string]any{"type": "select", "id": "mode", "name": "Session Mode", "category": "mode", "currentValue": "act", "options": []any{}}, map[string]any{"type": "boolean", "id": "auto_approve", "name": "Auto-approve tools", "description": "Automatically approve all tool calls without asking for permission", "currentValue": false}}}
}
func TestClineProjectNativeACPRejectsUnknownAndDuplicatePermissionWithoutBroadApproval(t *testing.T) {
	for _, kind := range []string{"unknown-first", "duplicate"} {
		p := clineContractProvider()
		f := &clineProjectAttempt{base: &nativeFixture{Project: "/owned/A", Nonce: p.nonce}, provider: p, evidence: map[string]any{}}
		init := map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"protocolVersion": 1, "agentCapabilities": map[string]any{"loadSession": true, "promptCapabilities": map[string]any{"image": true, "audio": false, "embeddedContext": false}}, "agentInfo": map[string]any{"name": "cline", "version": "3.0.73"}, "authMethods": []any{map[string]any{"id": "cline", "name": "Sign in with Cline"}, map[string]any{"id": "cline-pass", "name": "Sign in with ClinePass"}, map[string]any{"id": "openai-codex", "name": "Sign in with ChatGPT Subscription"}}}}
		permission := clineContractPermission(p)
		if kind == "unknown-first" {
			permission["toolCall"].(map[string]any)["title"] = "run_commands"
		}
		frames := []any{init, map[string]any{"jsonrpc": "2.0", "id": 2, "result": clineContractSession()}, map[string]any{"jsonrpc": "2.0", "id": "permission", "method": "session/request_permission", "params": permission}}
		if kind == "duplicate" {
			frames = append(frames, frames[len(frames)-1])
		}
		var input bytes.Buffer
		for _, frame := range frames {
			raw, _ := json.Marshal(frame)
			input.Write(raw)
			input.WriteByte('\n')
		}
		var output, transcript bytes.Buffer
		if err := f.acp(&output, &input, &transcript); err == nil {
			t.Fatal("unknown/duplicate native request accepted")
		}
		want := 0
		if kind == "duplicate" {
			want = 1
		}
		if got := strings.Count(output.String(), `"optionId":"allow_once"`); got != want {
			t.Fatalf("positive decision count %d want%d: %s", got, want, output.String())
		}
		if !strings.Contains(output.String(), `"outcome":"`+clineACPRefusalOutcome+`"`) || !strings.Contains(output.String(), `"mcpServers":[]`) {
			t.Fatal("unknown request not canceled or source-only session injection changed")
		}
	}
}
func TestClineProjectNativeFactsRejectCaseAliasesAndUnknownFields(t *testing.T) {
	good := map[string]any{"nonce": "owned", "revision": "a-one", "cwd": "/owned/A", "argv": []any{}, "plugin_root": "/owned/A", "plugin_data": "/owned/data", "literal": "", "once": "", "marker": "retained", "pid": 123, "session": "owned-session"}
	raw, _ := json.Marshal(good)
	if _, err := clineAttemptFacts(raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"Nonce", "unknown"} {
		m := clineContractClone(t, good)
		m[key] = "wrong"
		if key == "Nonce" {
			delete(m, "nonce")
		}
		raw, _ := json.Marshal(m)
		if _, err := clineAttemptFacts(raw); err == nil {
			t.Fatal("case alias/extra fact admitted", key)
		}
	}
}
func TestClineProjectNativeProviderLiteralHostAndOriginFormAreRequired(t *testing.T) {
	request := func() *http.Request {
		r, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/chat/completions", http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		r.Host = "127.0.0.1:32100"
		r.Header.Set("Authorization", "Bearer inert-key")
		r.Header.Set("Content-Type", "application/json")
		return r
	}
	if !clineAttemptProviderRequest(request(), "127.0.0.1:32100", "inert-key") {
		t.Fatal("exact loopback request refused")
	}
	for _, kind := range []string{"host", "absolute", "query", "encoded-path", "method", "authorization", "duplicate-auth", "duplicate-content-type"} {
		r := request()
		switch kind {
		case "host":
			r.Host = "foreign.invalid"
		case "absolute":
			r.URL.Scheme = "http"
			r.URL.Host = "127.0.0.1:32100"
		case "query":
			r.URL.RawQuery = "unexpected=true"
		case "encoded-path":
			r.URL.RawPath = "/v1/chat/%63ompletions"
		case "method":
			r.Method = "GET"
		case "authorization":
			r.Header.Set("Authorization", "Bearer wrong")
		case "duplicate-auth":
			r.Header.Add("Authorization", "Bearer inert-key")
		case "duplicate-content-type":
			r.Header.Add("Content-Type", "application/json")
		}
		if clineAttemptProviderRequest(r, "127.0.0.1:32100", "inert-key") {
			t.Fatal("provider address/route authority changed", kind)
		}
	}
}
func TestClineProjectNativeReceiptIsBoundToFrozenSourceManifestAndTree(t *testing.T) {
	source := "/owned/authored"
	tree := "sha256:frozen-tree"
	manifest := []byte("frozen manifest")
	i := domain.Installation{InstallationID: "owned-installation", DeclaredName: "uap-cline-a", Source: domain.SourceBinding{SourceBindingID: "src_" + clineAttemptSHA([]byte(source + "\x00\x00"))[:24], RequestedSource: source, CanonicalSource: source, TreeDigest: tree}, Package: domain.PackageBinding{LoaderKind: domain.LoaderKindAgentPlugins, FormatID: domain.FormatIDAgentPluginsV1, SchemaURI: domain.PluginSchemaV1, DeclaredName: "uap-cline-a", Version: "1.0.0", ManifestDigest: "sha256:" + clineAttemptSHA(manifest), Inventory: domain.ComponentInventory{MCPPresent: true, MCPEnabled: true, MCPServers: []string{"uapclinea"}, Skills: []string{"uap-cline-a"}}}, Clients: map[string]domain.ClientBinding{"owned": {ClientID: "cline"}}}
	if err := clineAttemptReceipt(i, source, tree, manifest); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"source-tree", "source-id", "requested", "revision", "manifest", "format", "inventory"} {
		changed := i
		switch kind {
		case "source-tree":
			changed.Source.TreeDigest = "sha256:foreign"
		case "source-id":
			changed.Source.SourceBindingID = "src_foreign"
		case "requested":
			changed.Source.RequestedSource = "other"
		case "revision":
			changed.Source.ResolvedRevision = "unrecorded"
		case "manifest":
			changed.Package.ManifestDigest = "sha256:foreign"
		case "format":
			changed.Package.FormatID = "foreign"
		case "inventory":
			changed.Package.Inventory.MCPServers = []string{"foreign"}
		}
		if clineAttemptReceipt(changed, source, tree, manifest) == nil {
			t.Fatal("recorded source authority changed", kind)
		}
	}
}
func clineContractInstalled(t *testing.T) (string, map[string][]byte, string, map[string]any) {
	t.Helper()
	root := t.TempDir()
	source := map[string][]byte{"plugin.json": []byte("inert manifest"), "mcp.json": []byte(`{"mcpServers":{"uapclinea":{"type":"stdio","command":"./bin/probe"}}}`), "skills/uap-cline-a/SKILL.md": []byte("inert selected skill"), "bin/probe": []byte("inert never executed probe")}
	for path, body := range source {
		mode := os.FileMode(0600)
		if path == "bin/probe" {
			mode = 0700
		}
		projectNativeRuntimeWrite(t, filepath.Join(root, filepath.FromSlash(path)), body, mode)
	}
	projection := map[string]any{"servers": map[string]any{"uapclinea": map[string]any{"type": "stdio", "command": "/owned/active/bin/probe", "cwd": "/owned/active"}}}
	raw, _ := json.Marshal(projection)
	projectNativeRuntimeWrite(t, filepath.Join(root, ".agentplugins-cline-native.json"), raw, 0600)
	launcher := []byte("inert never executed public installer")
	digest := clineAttemptSHA(launcher)
	directory := filepath.Join(root, "io.github.777genius.agentplugins", "managed-stdio-v1")
	projectNativeRuntimeWrite(t, filepath.Join(directory, "agentplugins"), launcher, 0700)
	metadata, _ := json.Marshal(map[string]any{"protocol": 1, "sha256": digest, "cliVersion": "0.1.0-development"})
	projectNativeRuntimeWrite(t, filepath.Join(directory, "metadata.json"), metadata, 0600)
	return root, source, digest, projection
}
func TestClineProjectNativeInstalledPayloadCannotAdoptChangedProbeSourceOrProductionExtras(t *testing.T) {
	for _, kind := range []string{"valid", "probe", "manifest", "skill", "portable-mcp", "projection", "launcher", "metadata", "extra-file", "extra-directory", "executable"} {
		t.Run(kind, func(t *testing.T) {
			root, source, digest, projection := clineContractInstalled(t)
			var path string
			switch kind {
			case "probe":
				path = "bin/probe"
			case "manifest":
				path = "plugin.json"
			case "skill":
				path = "skills/uap-cline-a/SKILL.md"
			case "portable-mcp":
				path = "mcp.json"
			case "projection":
				path = ".agentplugins-cline-native.json"
			case "launcher":
				path = "io.github.777genius.agentplugins/managed-stdio-v1/agentplugins"
			case "metadata":
				path = "io.github.777genius.agentplugins/managed-stdio-v1/metadata.json"
			case "extra-file":
				path = "unexpected"
			case "extra-directory":
				if err := os.Mkdir(filepath.Join(root, "unexpected"), 0700); err != nil {
					t.Fatal(err)
				}
			case "executable":
				if err := os.Chmod(filepath.Join(root, "bin", "probe"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if path != "" {
				if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), []byte("wrong independently selected bytes"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			err := clineAttemptInstalled(root, source, digest, projection)
			if kind == "valid" && err != nil {
				t.Fatal(err)
			}
			if kind != "valid" && err == nil {
				t.Fatal("changed installed source/closure adopted", kind)
			}
			if kind == "valid" {
				entries, err := clineAttemptPayload(root)
				if err != nil {
					t.Fatal(err)
				}
				captured, err := packagedigest.DigestCaptured(context.Background(), entries)
				if err != nil {
					t.Fatal(err)
				}
				snapshot, err := (packagedigest.Builder{TempRoot: t.TempDir()}).Snapshot(context.Background(), root, domain.SourceIdentity{})
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := packagedigest.Remove(snapshot); err != nil {
						t.Errorf("remove captured snapshot: %v", err)
					}
				}()
				if captured != snapshot.TreeDigest {
					t.Fatal("held captured framing differs from real installer package digest")
				}
			}
		})
	}
}
func TestClineProjectNativeNativeObjectsCannotChangeIdentityDigestOrOwnership(t *testing.T) {
	good := domain.NativeObjectOwnership{ObjectID: "cline_project_mcp_server:uapclinea", Kind: "cline_project_mcp_server", LogicalName: "uapclinea", Path: "/owned/A/mcp.json", ManagedDigest: "sha256:recorded", ProtectionClass: "managed"}
	want := map[string]domain.NativeObjectOwnership{good.Kind: good}
	if clineAttemptOwnership([]domain.NativeObjectOwnership{good}, want) != nil {
		t.Fatal("owned receipt refused")
	}
	for _, kind := range []string{"id", "kind", "name", "path", "digest", "protection", "source", "modified"} {
		wrong := good
		switch kind {
		case "id":
			wrong.ObjectID = "foreign"
		case "kind":
			wrong.Kind = "foreign"
		case "name":
			wrong.LogicalName = "foreign"
		case "path":
			wrong.Path = "/owned/B/mcp.json"
		case "digest":
			wrong.ManagedDigest = "sha256:foreign"
		case "protection":
			wrong.ProtectionClass = "foreign"
		case "source":
			wrong.SourceRelative = "unrecorded"
		case "modified":
			wrong.UserModified = true
		}
		if clineAttemptOwnership([]domain.NativeObjectOwnership{wrong}, want) == nil {
			t.Fatal("wrong native object accepted", kind)
		}
	}
	if clineAttemptOwnership(nil, want) == nil || clineAttemptOwnership([]domain.NativeObjectOwnership{good, good}, want) == nil {
		t.Fatal("missing/duplicate native ownership accepted")
	}
}
func TestClineProjectNativeProbeEventsMustBeOrderedAndFreshWithinNativeInterval(t *testing.T) {
	start := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Second)
	facts := map[string]any{"nonce": "owned", "revision": "a-one", "cwd": "/owned/A", "argv": []any{}, "plugin_root": "/owned/A", "plugin_data": "/owned/data", "literal": "", "once": "", "marker": "retained", "pid": json.Number("123"), "session": "owned-session"}
	for _, kind := range []string{"valid", "list-first", "call-first", "call-before-list", "repeat-init", "list-after-call", "stale", "future", "wrong-session", "wrong-facts"} {
		methods := []string{"initialize", "tools/list", "tools/call"}
		switch kind {
		case "list-first":
			methods = []string{"tools/list", "initialize", "tools/call"}
		case "call-first":
			methods = []string{"tools/call", "initialize", "tools/list"}
		case "call-before-list":
			methods = []string{"initialize", "tools/call", "tools/list"}
		case "repeat-init":
			methods = []string{"initialize", "initialize", "tools/list", "tools/call"}
		case "list-after-call":
			methods = []string{"initialize", "tools/list", "tools/call", "tools/list"}
		}
		var raw bytes.Buffer
		for index, method := range methods {
			at := start.Add(time.Duration(index+1) * time.Millisecond)
			if kind == "stale" {
				at = at.Add(-time.Hour)
			}
			if kind == "future" {
				at = at.Add(time.Hour)
			}
			event := map[string]any{"method": method, "nonce": "owned", "pid": json.Number("123"), "session": "owned-session", "timestamp": at.Format(time.RFC3339Nano), "facts": nil}
			if method == "tools/call" {
				event["facts"] = facts
				if kind == "wrong-facts" {
					changed := clineContractClone(t, facts)
					changed["plugin_data"] = "/owned/B"
					event["facts"] = changed
				}
			}
			if kind == "wrong-session" {
				event["session"] = "foreign"
			}
			body, _ := json.Marshal(event)
			raw.Write(body)
			raw.WriteByte('\n')
		}
		err := clineAttemptEvents(raw.Bytes(), "owned", facts, start, end)
		if kind == "valid" && err != nil {
			t.Fatal(err)
		}
		if kind != "valid" && err == nil {
			t.Fatal("unfresh/unordered probe events accepted", kind)
		}
	}
}
