package pluginkitairepo_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// This scripted loopback provider supplies protocol responses, not a real model.
// Its captured request proves the actual native session's tool namespace. MCP
// calls and their returned nonce prove dispatch independently of config/status.
func TestAgentpluginsOpenCodeNativeToolCollision(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_OPENCODE_NATIVE_E2E") != "1" {
		t.Skip("opt-in native client execution")
	}
	for _, names := range [][]string{{"api/server"}, {"api server"}, {"api/server", "api server"}} {
		t.Run(strings.Join(names, "+"), func(t *testing.T) {
			f := newOpenCodeNativeFixture(t)
			client := openCodeNativeBinary(t, "AGENTPLUGINS_OPENCODE_BIN")
			installer := openCodeNativeBinary(t, "AGENTPLUGINS_INSTALLER_BIN")
			scanner := openCodePrepareNative(t, f, client)
			started := time.Now().UTC().Format(time.RFC3339Nano)
			identity, err := nativeSourceIdentity(os.Getenv("AGENTPLUGINS_INSTALLER_COMMIT"), os.Getenv("AGENTPLUGINS_INSTALLER_TREE"), os.Getenv("AGENTPLUGINS_INSTALLER_PATCH_SHA256"))
			if err != nil {
				t.Fatal(err)
			}
			measured := openCodeVersionString(t, f, client)
			if measured != "1.18.29" {
				t.Fatalf("collision contract pinned to 1.18.29, got %s", measured)
			}
			nonce := filepath.Base(f.Root)
			var mu sync.Mutex
			methods := map[string][]string{}
			var calls, offered []string
			var requests []json.RawMessage
			var resultSeen bool
			mcp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" {
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				var req struct {
					ID     json.RawMessage `json:"id"`
					Method string          `json:"method"`
					Params struct {
						Name      string            `json:"name"`
						Arguments map[string]string `json:"arguments"`
					} `json:"params"`
				}
				if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
					http.Error(w, "json", 400)
					return
				}
				name := strings.TrimPrefix(r.URL.Path, "/")
				mu.Lock()
				methods[name] = append(methods[name], req.Method)
				mu.Unlock()
				if len(req.ID) == 0 {
					w.WriteHeader(http.StatusAccepted)
					return
				}
				var result any
				switch req.Method {
				case "initialize":
					result = map[string]any{"protocolVersion": "2025-03-26", "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]any{"name": name, "version": "1.0.0"}}
				case "tools/list":
					result = map[string]any{"tools": []any{map[string]any{"name": "inspect_runtime", "description": "Return disposable test identity", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"nonce": map[string]any{"type": "string"}}, "required": []string{"nonce"}}}}}
				case "tools/call":
					if req.Params.Name != "inspect_runtime" || req.Params.Arguments["nonce"] != nonce {
						http.Error(w, "invalid fixture call", 400)
						return
					}
					mu.Lock()
					calls = append(calls, name)
					mu.Unlock()
					result = map[string]any{"content": []any{map[string]any{"type": "text", "text": fmt.Sprintf("native-dispatch:%s:%s", name, nonce)}}}
				default:
					result = map[string]any{}
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
			}))
			defer mcp.Close()
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
				if err != nil {
					http.Error(w, "read", 400)
					return
				}
				var req struct {
					Tools []struct {
						Function struct {
							Name string `json:"name"`
						} `json:"function"`
					} `json:"tools"`
					Messages []struct {
						Role    string          `json:"role"`
						Content json.RawMessage `json:"content"`
					} `json:"messages"`
				}
				if json.Unmarshal(body, &req) != nil {
					http.Error(w, "json", 400)
					return
				}
				mu.Lock()
				defer mu.Unlock()
				requests = append(requests, append(json.RawMessage(nil), body...))
				var nativeNames []string
				for _, tool := range req.Tools {
					if strings.HasPrefix(tool.Function.Name, "api_server_") {
						nativeNames = append(nativeNames, tool.Function.Name)
					}
				}
				if len(nativeNames) > 0 {
					offered = append([]string(nil), nativeNames...)
				}
				done := false
				for _, msg := range req.Messages {
					if msg.Role == "tool" && len(calls) == 1 && strings.Contains(string(msg.Content), "native-dispatch:"+calls[0]+":"+nonce) {
						done = true
						resultSeen = true
					}
				}
				w.Header().Set("Content-Type", "text/event-stream")
				emit := func(delta any, finish any) {
					b, _ := json.Marshal(map[string]any{"id": "uap-scripted", "object": "chat.completion.chunk", "created": 1, "model": "fixture", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}})
					fmt.Fprintf(w, "data: %s\n\n", b)
				}
				if !done && len(nativeNames) == 1 {
					arguments, _ := json.Marshal(map[string]string{"nonce": nonce})
					emit(map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": "uap-call", "type": "function", "function": map[string]any{"name": nativeNames[0], "arguments": string(arguments)}}}}, nil)
					emit(map[string]any{}, "tool_calls")
				} else {
					emit(map[string]any{"role": "assistant", "content": "Disposable scripted provider complete."}, nil)
					emit(map[string]any{}, "stop")
				}
				fmt.Fprint(w, "data: [DONE]\n\n")
			}))
			defer provider.Close()
			config := map[string]any{"provider": map[string]any{"uap-fixture": map[string]any{"name": "Disposable fixture", "npm": "@ai-sdk/openai-compatible", "env": []string{}, "models": map[string]any{"fixture": map[string]any{"name": "Fixture", "tool_call": true, "limit": map[string]any{"context": 32000, "output": 1024}}}, "options": map[string]any{"apiKey": "disposable-not-a-secret", "baseURL": provider.URL + "/v1"}}}, "model": "uap-fixture/fixture", "small_model": "uap-fixture/fixture", "permission": "allow"}
			nativeJSON(t, filepath.Join(f.XDGConfig, "opencode", "opencode.json"), config)
			openCodeWriteFixturePackage(t, f.PackageRoot, "opencode-native-proof", "1.0.0")
			servers := map[string]any{}
			for i, name := range names {
				servers[name] = map[string]any{"type": "streamable-http", "url": fmt.Sprintf("%s/server-%d", mcp.URL, i)}
			}
			nativeJSON(t, filepath.Join(f.PackageRoot, "mcp.json"), map[string]any{"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json", "mcpServers": servers})
			added := openCodeRunInstaller(t, f, installer, filepath.Dir(client), "add-tool-fixture", "add", f.PackageRoot, "--target", "opencode")
			openCodeAssertCompleted(t, "tool fixture install", added)
			installed := openCodeConfigMCP(t, openCodeDebugConfig(t, f, client, "tool-fixture-effective-config"))
			if len(installed) != len(names) {
				t.Fatalf("installed MCP count %d, want %d", len(installed), len(names))
			}
			for _, name := range names {
				if _, ok := installed[name]; !ok {
					t.Fatalf("installed MCP missing %q", name)
				}
			}
			stages := map[string]any{}
			evidence := map[string]any{"status": "failed", "source_identity": identity, "scanner": scanner, "os": runtime.GOOS, "arch": runtime.GOARCH, "started_utc": started, "nonce": nonce, "config_route": "opencode.json", "transcript_sha256": f.transcriptSHA256, "scenario": map[bool]string{true: "observed_client_tool_id_collision", false: "single_key_native_dispatch"}[len(names) == 2], "client_version": measured, "client_sha256": openCodeSHA256(t, client), "installer_sha256": openCodeSHA256(t, installer), "logical_servers": names, "stages": stages, "model": "scripted_loopback_no_real_model", "oauth": "not_evaluated"}
			defer func() {
				evidence["finished_utc"] = time.Now().UTC().Format(time.RFC3339Nano)
				nativeJSON(t, filepath.Join(f.Root, "tool-collision-evidence.json"), evidence)
			}()
			runSession := func(label, revision string, removed bool) {
				t.Helper()
				mu.Lock()
				methods = map[string][]string{}
				calls = nil
				offered = nil
				requests = nil
				resultSeen = false
				mu.Unlock()
				ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, client, "run", "--model", "uap-fixture/fixture", "--format", "json", "Call the fixture inspect_runtime tool once if available, then finish.")
				cmd.Dir = f.Project
				cmd.Env = f.env(filepath.Dir(client))
				out, err := cmd.CombinedOutput()
				f.record(t, "native-"+label, out)
				mu.Lock()
				defer mu.Unlock()
				stage := map[string]any{"status": "failed", "native_tool_names": offered, "mcp_methods": methods, "dispatch_servers": calls, "tool_result_returned_to_provider": resultSeen, "provider_requests": requests, "expected_revision": revision}
				stages[label] = stage
				if err != nil {
					t.Fatalf("native %s: %v\n%s", label, err, out)
				}
				if len(requests) == 0 {
					t.Fatal("native session never contacted scripted provider")
				}
				if removed {
					if len(offered) != 0 || len(calls) != 0 || len(methods) != 0 {
						t.Fatalf("removed tool still active: offered=%v calls=%v methods=%v", offered, calls, methods)
					}
				} else {
					if len(offered) != 1 || offered[0] != "api_server_inspect_runtime" {
						t.Fatalf("unexpected native namespace: %v", offered)
					}
					if len(calls) != 1 || !resultSeen {
						t.Fatalf("dispatch missing: calls=%v returned=%v\n%s", calls, resultSeen, out)
					}
					if len(names) == 1 && calls[0] != "server-0"+revision {
						t.Fatalf("stale dispatch: %q, want %q", calls[0], "server-0"+revision)
					}
					for i := range names {
						for _, method := range []string{"initialize", "tools/list"} {
							if !strings.Contains(strings.Join(methods[fmt.Sprintf("server-%d%s", i, revision)], ","), method) {
								t.Fatalf("server %d missing %s", i, method)
							}
						}
					}
				}
				stage["status"] = "passed"
			}
			runSession("install", "", false)
			if len(names) == 1 && names[0] == "api/server" {
				// Runtime revision lives in the installed URL, not mutable server
				// state: a stale native config necessarily returns the old marker.
				for _, update := range []struct{ version, revision, label string }{{"2.0.0", "/B", "version-update"}, {"2.0.0", "/C", "same-version-refresh"}} {
					openCodeWriteFixturePackage(t, f.PackageRoot, "opencode-native-proof", update.version)
					servers[names[0]] = map[string]any{"type": "streamable-http", "url": mcp.URL + "/server-0" + update.revision}
					nativeJSON(t, filepath.Join(f.PackageRoot, "mcp.json"), map[string]any{"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json", "mcpServers": servers})
					changed := openCodeRunInstaller(t, f, installer, filepath.Dir(client), update.label, "update", f.PackageRoot, "--target", "opencode")
					openCodeAssertCompleted(t, update.label, changed)
					openCodeAssertMutated(t, changed, true)
					runSession(update.label, update.revision, false)
				}
				configPath := filepath.Join(f.XDGConfig, "opencode", "opencode.json")
				body, err := os.ReadFile(configPath)
				if err != nil {
					t.Fatal(err)
				}
				var doc map[string]any
				openCodeReadConfigDocument(t, body, &doc)
				foreign := map[string]any{"type": "local", "command": []string{"sh", "-c", "cat"}, "enabled": false}
				doc["mcp"].(map[string]any)["foreign-untouched"] = foreign
				nativeJSON(t, configPath, doc)
				// packageNeedsPluginData allocates only for selected stdio MCP.
				// This package uses HTTP exclusively; do not fabricate a receipt.
				if _, err := os.Stat(filepath.Join(f.StateHome, "plugin-data")); !os.IsNotExist(err) {
					t.Fatalf("HTTP-only fixture unexpectedly has plugin-data: %v", err)
				}
				stages["plugin_data_retention"] = map[string]any{"status": "not_applicable", "reason": "HTTP-only MCP package has no allocated PLUGIN_DATA receipt; native stdio data semantics are not tested"}
				managedDir := filepath.Join(f.StateHome, "managed", "clients", "opencode", openCodePhysicalArtifactID(t, added))
				if _, err := os.Stat(filepath.Join(managedDir, "plugin.json")); err != nil {
					t.Fatal(err)
				}
				if err := os.RemoveAll(managedDir); err != nil {
					t.Fatal(err)
				}
				repaired := openCodeRunInstaller(t, f, installer, filepath.Dir(client), "runtime-repair", "repair", "opencode-native-proof", "--target", "opencode")
				openCodeAssertCompleted(t, "runtime repair", repaired)
				runSession("repair", "/C", false)
				removed := openCodeRunInstaller(t, f, installer, filepath.Dir(client), "runtime-remove", "remove", "opencode-native-proof", "--target", "opencode")
				data, _ := removed["data"].(map[string]any)
				if removed["result"] != "success" || data["status"] != "data_retained" {
					t.Fatalf("remove: %+v", removed)
				}
				runSession("remove", "", true)
				post := openCodeConfigMCP(t, openCodeDebugConfig(t, f, client, "runtime-post-remove-config"))
				got, _ := json.Marshal(post["foreign-untouched"])
				want, _ := json.Marshal(foreign)
				if string(got) != string(want) {
					t.Fatalf("foreign config changed: %s != %s", got, want)
				}
				stages["foreign_config_preservation"] = map[string]any{"status": "passed"}
			}
			evidence["status"] = "passed"

			// With two logical servers, one offered tool and one dispatch establishes
			// native tool-ID collision; this is an upstream limitation, not success
			// evidence for separately addressable tools.
			if len(names) == 2 {
				t.Log("native collision observed: two MCP catalogs become one callable tool ID")
			}
		})
	}
}
