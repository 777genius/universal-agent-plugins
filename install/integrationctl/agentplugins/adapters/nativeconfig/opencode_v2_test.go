package nativeconfig

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tailscale/hujson"
)

// Regression: adding V2 must not change previously persisted V1 digests or bytes.
// Entry/digest fixtures were independently calculated before the V2 edits.
// The byte fixture specifies the existing renderer; dependency availability is
// required to execute this regression against main and the candidate.
func TestOpenCodeV1FrozenNativeBytesAndDigest(t *testing.T) {
	for _, tc := range []struct {
		name          string
		server        Server
		entry, digest string
	}{
		{"local", Server{Type: "stdio", Command: "node", Args: []string{"${PLUGIN_ROOT}/run.js"}, Env: map[string]string{"DATA": "${PLUGIN_DATA}"}, CWD: "${PLUGIN_ROOT}/work"}, `{"command":["node","/pkg/run.js"],"cwd":"/pkg/work","environment":{"DATA":"/data"},"type":"local"}`, "sha256:8ee58e1af57bcc73f7dffd74d0f00f6adfe206c33bda6a564907607affcb722f"},
		{"remote", Server{Type: "remote", URL: "https://mcp.test", Headers: map[string]string{"X-Test": "fixture"}}, `{"headers":{"X-Test":"fixture"},"type":"remote","url":"https://mcp.test"}`, "sha256:f906e48e07f66ae5765c789375fe6894139ab24f2539ebf5a6dcc478c4d961ea"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			paths := Paths{JSON: filepath.Join(t.TempDir(), "opencode.json")}
			p := Placeholders{PackageRoot: "/pkg", DataRoot: "/data"}
			want := Receipt{Version: "1", Path: paths.JSON, Codec: CodecOpenCode, Name: "owned", Digest: tc.digest}
			desired, err := DesiredReceipt(paths.JSON, CodecOpenCode, "owned", tc.server, p)
			if err != nil || desired != want {
				t.Fatalf("V1 desired receipt changed: %+v; %v", desired, err)
			}
			got, err := New().Apply(Request{Paths: paths, Codec: CodecOpenCode, Action: ActionAdd, Name: "owned", Server: tc.server, Placeholders: p, Desired: &desired})
			if err != nil || got != want {
				t.Fatalf("V1 receipt changed: %+v; %v", got, err)
			}
			wantBytes := "{\n  \"mcp\":{\n  \"owned\":" + tc.entry + "\n}\n}\n"
			if body := mustRead(t, paths.JSON); body != wantBytes {
				t.Fatalf("V1 bytes changed:\n%q", body)
			}
		})
	}
}

func readV2Config(t *testing.T, path string) map[string]any {
	t.Helper()
	body, err := hujson.Standardize([]byte(mustRead(t, path)))
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		t.Fatal(err)
	}
	return root
}

// Regression: a flat V1 writer, inverted enabled flag, numeric timeout, or
// generic args/env remote transport tag would be accepted locally but ignored
// or rejected by pinned native V2. Assert the independently specified schema
// at the filesystem boundary, with both JSON and JSONC selection.
func TestOpenCodeV2PinnedNativeShapeAndQuery(t *testing.T) {
	for _, ext := range []string{"json", "jsonc"} {
		t.Run(ext, func(t *testing.T) {
			root := t.TempDir()
			paths := Paths{JSON: filepath.Join(root, "opencode.json"), JSONC: filepath.Join(root, "opencode.jsonc")}
			path := filepath.Join(root, "opencode."+ext)
			mustWrite(t, path, `{}`)
			kernel := New()
			requests := []Request{
				{Paths: paths, Codec: CodecOpenCodeV2, Action: ActionAdd, Name: "local/server", Server: Server{Type: "stdio", Command: "node", Args: []string{"${PLUGIN_ROOT}/run.js"}, Env: map[string]string{"DATA": "${PLUGIN_DATA}"}, CWD: "${PLUGIN_ROOT}/work", OpenCodeV2: &OpenCodeV2Options{Timeout: &OpenCodeV2Timeout{Startup: 5000, Catalog: 6000, Execution: 7000}}}, Placeholders: Placeholders{PackageRoot: "/pkg", DataRoot: "/data"}},
				{Paths: paths, Codec: CodecOpenCodeV2, Action: ActionAdd, Name: `remote"server`, Server: Server{Type: "remote", URL: "https://mcp.test", RemoteTransport: "streamable-http", Headers: map[string]string{"X-Test": "fixture"}, OpenCodeV2: &OpenCodeV2Options{Disabled: true, Timeout: &OpenCodeV2Timeout{Execution: 8000}}}},
			}
			for i := range requests {
				desired, err := DesiredReceipt(path, requests[i].Codec, requests[i].Name, requests[i].Server, requests[i].Placeholders)
				if err != nil {
					t.Fatal(err)
				}
				requests[i].Desired = &desired
			}
			receipts, err := kernel.ApplyBatch(requests)
			if err != nil {
				t.Fatal(err)
			}
			wantJSON := `{"mcp":{"servers":{"local/server":{"type":"local","command":["node","/pkg/run.js"],"environment":{"DATA":"/data"},"cwd":"/pkg/work","disabled":false,"timeout":{"startup":5000,"catalog":6000,"execution":7000}},"remote\"server":{"type":"remote","url":"https://mcp.test","headers":{"X-Test":"fixture"},"disabled":true,"timeout":{"execution":8000}}}}}`
			var want map[string]any
			if err := json.Unmarshal([]byte(wantJSON), &want); err != nil {
				t.Fatal(err)
			}
			if got := readV2Config(t, path); !reflect.DeepEqual(got, want) {
				t.Fatalf("not pinned native shape: %#v", got)
			}
			for i, receipt := range receipts {
				if receipt != *requests[i].Desired {
					t.Fatalf("project/apply disagree: %+v", receipt)
				}
				present, owned, err := kernel.Inspect(paths, CodecOpenCodeV2, requests[i].Name, &receipt)
				if err != nil || !present || !owned {
					t.Fatalf("V2 lookup: %v %v %v", present, owned, err)
				}
				present, owned, err = kernel.Inspect(paths, CodecOpenCode, requests[i].Name, &receipt)
				if err != nil || present || owned {
					t.Fatalf("V1 lookup traversed V2: %v %v %v", present, owned, err)
				}
			}
		})
	}
}

// Regression: nested edits might adopt an equal foreign entry or serialize the
// whole config, dropping comments/unknown fields or the same name elsewhere.
// Exercise project -> add -> inspect -> update -> stale receipt -> remove.
func TestOpenCodeV2LosslessOwnedLifecycle(t *testing.T) {
	for _, ext := range []string{"json", "jsonc"} {
		t.Run(ext, func(t *testing.T) {
			root := t.TempDir()
			paths := Paths{JSON: filepath.Join(root, "opencode.json"), JSONC: filepath.Join(root, "opencode.jsonc")}
			path := filepath.Join(root, "opencode."+ext)
			foreign := `"foreign": { "type":"remote", "url":"https://foreign.test", "disabled":true, "unknown": [1, {"keep":true}] }`
			outside := `"mcpServers": {"owned":{"command":"foreign-same-name"}}`
			unknown := `"unknownRoot": { "spacing": [ 1, 2 ], "keep": true }`
			original := "{" + unknown + "," + outside + `,"mcp":{"servers":{` + foreign + `}}}`
			if ext == "jsonc" {
				original = "// root comment\n" + strings.Replace(original, foreign, "// foreign comment\n"+foreign, 1)
			}
			mustWrite(t, path, original)
			kernel := New()
			assertForeign := func() {
				t.Helper()
				body := mustRead(t, path)
				for _, fragment := range []string{foreign, outside, unknown} {
					if !strings.Contains(body, fragment) {
						t.Fatalf("foreign bytes lost: %q in %s", fragment, body)
					}
				}
				if ext == "jsonc" && (!strings.Contains(body, "// root comment") || !strings.Contains(body, "// foreign comment")) {
					t.Fatalf("comments lost: %s", body)
				}
			}
			req := Request{Paths: paths, Codec: CodecOpenCodeV2, Action: ActionAdd, Name: "owned", Server: Server{Type: "remote", URL: "https://mcp.test"}}
			desired, err := DesiredReceipt(path, req.Codec, req.Name, req.Server, req.Placeholders)
			if err != nil {
				t.Fatal(err)
			}
			req.Desired = &desired
			owned, err := kernel.Apply(req)
			if err != nil || owned != desired {
				t.Fatalf("add: %+v %v", owned, err)
			}
			assertForeign()
			// Equality is not an ownership proof.
			before := mustRead(t, path)
			if _, err := kernel.Apply(req); !errors.Is(err, ErrCollision) {
				t.Fatalf("equal entry adopted: %v", err)
			}
			assertBytes(t, path, before)
			req.Action, req.Owned, req.Desired = ActionUpdate, &owned, nil
			req.Server.URL = "https://mcp.test/updated"
			updated, err := kernel.Apply(req)
			if err != nil || updated.Digest == owned.Digest {
				t.Fatalf("update: %+v %v", updated, err)
			}
			assertForeign()
			present, exact, err := kernel.Inspect(paths, CodecOpenCodeV2, "owned", &updated)
			if err != nil || !present || !exact {
				t.Fatalf("updated receipt: %v %v %v", present, exact, err)
			}
			req.Action = ActionRemove
			before = mustRead(t, path)
			if _, err := kernel.Apply(req); !errors.Is(err, ErrNotOwned) {
				t.Fatalf("stale receipt removed update: %v", err)
			}
			assertBytes(t, path, before)
			req.Owned = &updated
			if _, err := kernel.Apply(req); err != nil {
				t.Fatal(err)
			}
			assertForeign()
			servers := readV2Config(t, path)["mcp"].(map[string]any)["servers"].(map[string]any)
			if len(servers) != 1 || servers["foreign"] == nil {
				t.Fatalf("remove touched foreign leaf: %#v", servers)
			}
		})
	}
}

// Regression: checking only desired fields would overwrite a user's addition
// inside the owned leaf. The complete native entry is receipt authority.
func TestOpenCodeV2EditedOwnedLeafConflicts(t *testing.T) {
	paths := Paths{JSON: filepath.Join(t.TempDir(), "opencode.json")}
	kernel := New()
	req := Request{Paths: paths, Codec: CodecOpenCodeV2, Action: ActionAdd, Name: "owned", Server: Server{Type: "remote", URL: "https://mcp.test"}}
	owned, err := kernel.Apply(req)
	if err != nil {
		t.Fatal(err)
	}
	before := mustRead(t, paths.JSON)
	edited := strings.Replace(before, `"disabled":false`, `"userAdded":{"keep":true},"disabled":false`, 1)
	if edited == before {
		t.Fatal("fixture did not edit leaf")
	}
	mustWrite(t, paths.JSON, edited)
	present, exact, err := kernel.Inspect(paths, CodecOpenCodeV2, "owned", &owned)
	if err != nil || !present || exact {
		t.Fatalf("edited leaf still owned: %v %v %v", present, exact, err)
	}
	for _, action := range []Action{ActionUpdate, ActionRemove} {
		req.Action, req.Owned = action, &owned
		if _, err := kernel.Apply(req); !errors.Is(err, ErrNotOwned) {
			t.Fatalf("%s overwrote edited leaf: %v", action, err)
		}
		assertBytes(t, paths.JSON, edited)
	}
}

// Regression: reinterpreting a legacy receipt as V2 (or vice versa) would let a
// same-name entry in a different container be verified or deleted after swap.
func TestOpenCodeReceiptsCannotCrossDialects(t *testing.T) {
	paths := Paths{JSON: filepath.Join(t.TempDir(), "opencode.json")}
	kernel := New()
	server := Server{Type: "remote", URL: "https://mcp.test"}
	v1, err := kernel.Apply(Request{Paths: paths, Codec: CodecOpenCode, Action: ActionAdd, Name: "owned", Server: server})
	if err != nil {
		t.Fatal(err)
	}
	v1Body := mustRead(t, paths.JSON)
	mustWrite(t, paths.JSON, `{}`)
	v2, err := kernel.Apply(Request{Paths: paths, Codec: CodecOpenCodeV2, Action: ActionAdd, Name: "owned", Server: server})
	if err != nil {
		t.Fatal(err)
	}
	v2Body := mustRead(t, paths.JSON)
	for _, tc := range []struct {
		codec          Codec
		body           string
		correct, wrong Receipt
	}{
		{CodecOpenCode, v1Body, v1, v2}, {CodecOpenCodeV2, v2Body, v2, v1},
	} {
		mustWrite(t, paths.JSON, tc.body)
		for _, receipt := range []Receipt{tc.wrong, {Version: tc.wrong.Version, Path: tc.wrong.Path, Codec: tc.codec, Name: tc.wrong.Name, Digest: tc.wrong.Digest}} {
			present, exact, err := kernel.Inspect(paths, tc.codec, "owned", &receipt)
			if err != nil || !present || exact {
				t.Fatalf("cross-dialect receipt accepted: %v %v %v", present, exact, err)
			}
			if _, err := kernel.Apply(Request{Paths: paths, Codec: tc.codec, Action: ActionRemove, Name: "owned", Owned: &receipt}); !errors.Is(err, ErrNotOwned) {
				t.Fatalf("cross-dialect delete: %v", err)
			}
			assertBytes(t, paths.JSON, tc.body)
		}
		if present, exact, err := kernel.Inspect(paths, tc.codec, "owned", &tc.correct); err != nil || !present || !exact {
			t.Fatalf("original authority lost: %v %v %v", present, exact, err)
		}
	}
	// Keep canonical leaf bytes identical, so even changing the receipt's codec
	// tag cannot transfer ownership: the digest itself must bind the dialect.
	var flat map[string]json.RawMessage
	if err := json.Unmarshal([]byte(v1Body), &flat); err != nil {
		t.Fatal(err)
	}
	moved := `{"mcp":{"servers":` + string(flat["mcp"]) + `}}`
	mustWrite(t, paths.JSON, moved)
	retagged := v1
	retagged.Codec = CodecOpenCodeV2
	present, exact, err := kernel.Inspect(paths, CodecOpenCodeV2, "owned", &retagged)
	if err != nil || !present || exact {
		t.Fatalf("retagging transferred identical bytes: %v %v %v", present, exact, err)
	}
	if _, err := kernel.Apply(Request{Paths: paths, Codec: CodecOpenCodeV2, Action: ActionRemove, Name: "owned", Owned: &retagged}); !errors.Is(err, ErrNotOwned) {
		t.Fatalf("retagged receipt deleted identical leaf: %v", err)
	}
	assertBytes(t, paths.JSON, moved)
}

// Regression: a V2 operation could silently convert/extend a foreign V1 root,
// including a flat V1 server literally named servers. Reject before writing.
func TestOpenCodeV2RejectsUnqualifiedRoots(t *testing.T) {
	for _, body := range []string{
		`{"mcp":{"foreign":{"type":"remote","url":"https://foreign.test"}}}`,
		`{"mcp":{"foreign":{"type":"remote","url":"https://foreign.test"},"servers":{}}}`,
		`{"mcp":{"servers":{"type":"local","command":["node"]}}}`,
		`{"mcp":{"unknownMCPKey":{},"servers":{}}}`,
	} {
		paths := Paths{JSON: filepath.Join(t.TempDir(), "opencode.json")}
		mustWrite(t, paths.JSON, body)
		kernel := New()
		if err := kernel.CheckOpenCodeNamespaceForCodec(paths, CodecOpenCodeV2, []string{"owned"}, nil); !errors.Is(err, ErrNativeMigrationRequired) {
			t.Fatalf("root preflight: %v", err)
		}
		if _, err := kernel.Apply(Request{Paths: paths, Codec: CodecOpenCodeV2, Action: ActionAdd, Name: "owned", Server: Server{Type: "remote", URL: "https://mcp.test"}}); !errors.Is(err, ErrNativeMigrationRequired) {
			t.Fatalf("root write: %v", err)
		}
		assertBytes(t, paths.JSON, body)
	}
}

// Regression: V1 enabled logic would treat a V2 disabled foreign server as
// active or overlook an active nested name. Preflight and locked write agree.
func TestOpenCodeV2NestedNamespaceAndDisabled(t *testing.T) {
	for _, disabled := range []string{"true", "false", `"false"`, "null"} {
		paths := Paths{JSON: filepath.Join(t.TempDir(), "opencode.json")}
		body := `{"mcp":{"servers":{"api/server":{"type":"remote","url":"https://foreign.test","disabled":` + disabled + `}}}}`
		mustWrite(t, paths.JSON, body)
		kernel := New()
		preflight := kernel.CheckOpenCodeNamespaceForCodec(paths, CodecOpenCodeV2, []string{"api server"}, nil)
		_, applied := kernel.Apply(Request{Paths: paths, Codec: CodecOpenCodeV2, Action: ActionAdd, Name: "api server", Server: Server{Type: "remote", URL: "https://mcp.test"}})
		if disabled == "true" {
			if preflight != nil || applied != nil {
				t.Fatalf("disabled entry blocked: %v %v", preflight, applied)
			}
		} else {
			var collision *OpenCodeNamespaceConflict
			for _, err := range []error{preflight, applied} {
				if disabled == "false" {
					if !errors.As(err, &collision) {
						t.Fatalf("active entry missed: %v", err)
					}
				} else if !errors.Is(err, ErrMalformed) {
					t.Fatalf("invalid disabled accepted: %v", err)
				}
			}
			assertBytes(t, paths.JSON, body)
		}
	}
}

// Regression: invalid neutral inputs could commit a native entry or silently
// downgrade to a different codec/transport. Projection also rejects before IO.
func TestOpenCodeV2RejectsInvalidContractsBeforeWrite(t *testing.T) {
	for _, tc := range []struct {
		name   string
		codec  Codec
		server Server
	}{
		{"unknown codec", Codec("unknown"), Server{Type: "remote", URL: "https://mcp.test"}},
		{"SSE", CodecOpenCodeV2, Server{Type: "remote", URL: "https://mcp.test", RemoteTransport: "sse"}},
		{"unknown transport", CodecOpenCodeV2, Server{Type: "remote", URL: "https://mcp.test", RemoteTransport: "auto"}},
		{"stdio with transport", CodecOpenCodeV2, Server{Type: "stdio", Command: "node", RemoteTransport: "streamable-http"}},
		{"remote with process", CodecOpenCodeV2, Server{Type: "remote", URL: "https://mcp.test", Command: "node"}},
		{"negative startup", CodecOpenCodeV2, Server{Type: "remote", URL: "https://mcp.test", OpenCodeV2: &OpenCodeV2Options{Timeout: &OpenCodeV2Timeout{Startup: -1}}}},
		{"negative catalog", CodecOpenCodeV2, Server{Type: "remote", URL: "https://mcp.test", OpenCodeV2: &OpenCodeV2Options{Timeout: &OpenCodeV2Timeout{Catalog: -1}}}},
		{"negative execution", CodecOpenCodeV2, Server{Type: "remote", URL: "https://mcp.test", OpenCodeV2: &OpenCodeV2Options{Timeout: &OpenCodeV2Timeout{Execution: -1}}}},
		{"V2 options on V1", CodecOpenCode, Server{Type: "remote", URL: "https://mcp.test", OpenCodeV2: &OpenCodeV2Options{Disabled: true}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			paths := Paths{JSON: filepath.Join(t.TempDir(), "opencode.json")}
			body := `{"unknownRoot":{"keep":true}}`
			mustWrite(t, paths.JSON, body)
			if _, err := DesiredReceipt(paths.JSON, tc.codec, "owned", tc.server, Placeholders{}); err == nil {
				t.Fatal("invalid projection accepted")
			}
			if _, err := New().Apply(Request{Paths: paths, Codec: tc.codec, Action: ActionAdd, Name: "owned", Server: tc.server}); err == nil {
				t.Fatal("invalid write accepted")
			}
			assertBytes(t, paths.JSON, body)
		})
	}
}

// Regression: introducing a nested codec must not loosen ApplyBatch into a
// same-name dialect transition or let duplicate names partially commit.
func TestOpenCodeV2BatchContractUnchanged(t *testing.T) {
	paths := Paths{JSON: filepath.Join(t.TempDir(), "opencode.json")}
	mustWrite(t, paths.JSON, `{}`)
	base := Request{Paths: paths, Codec: CodecOpenCodeV2, Action: ActionAdd, Name: "owned", Server: Server{Type: "remote", URL: "https://mcp.test"}}
	v1 := base
	v1.Codec = CodecOpenCode
	for _, requests := range [][]Request{{base, base}, {base, v1}} {
		if _, err := New().ApplyBatch(requests); err == nil {
			t.Fatal("invalid batch accepted")
		}
		assertBytes(t, paths.JSON, `{}`)
	}
}

// Regression: bypassing the existing kernel for nested V2 edits would clobber
// an independent writer's change at the replacement boundary. Reuse the
// established FileIO race seam and inspect the actual resulting file.
func TestOpenCodeV2ConcurrentReplacementPreservesForeignWrite(t *testing.T) {
	paths := Paths{JSON: filepath.Join(t.TempDir(), "opencode.json")}
	mustWrite(t, paths.JSON, `{"unknownRoot":"original"}`)
	_, err := NewWithFileIO(&interleavingIO{}).Apply(Request{Paths: paths, Codec: CodecOpenCodeV2, Action: ActionAdd, Name: "owned", Server: Server{Type: "remote", URL: "https://mcp.test"}})
	if !errors.Is(err, ErrConcurrentChange) {
		t.Fatalf("concurrent write overwritten: %v", err)
	}
	assertBytes(t, paths.JSON, `{"client":"concurrent"}`)
}

// Regression: a nested collection implementation could accept null/array
// containers or duplicate native keys, then mutate the wrong document node.
func TestOpenCodeV2MalformedContainersFailClosed(t *testing.T) {
	for _, body := range []string{
		`{"mcp":null}`, `{"mcp":[]}`, `{"mcp":{"servers":null}}`,
		`{"mcp":{"servers":[]}}`, `{"mcp":{"servers":{"a":{},"a":{}}}}`,
	} {
		paths := Paths{JSON: filepath.Join(t.TempDir(), "opencode.json")}
		mustWrite(t, paths.JSON, body)
		kernel := New()
		if _, _, err := kernel.Inspect(paths, CodecOpenCodeV2, "owned", nil); !errors.Is(err, ErrMalformed) {
			t.Fatalf("malformed lookup accepted: %v", err)
		}
		if _, err := kernel.Apply(Request{Paths: paths, Codec: CodecOpenCodeV2, Action: ActionAdd, Name: "owned", Server: Server{Type: "remote", URL: "https://mcp.test"}}); !errors.Is(err, ErrMalformed) {
			t.Fatalf("malformed write accepted: %v", err)
		}
		assertBytes(t, paths.JSON, body)
	}
}

// Regression: a root migration guard could prevent receipt-only cleanup or
// delete a foreign flat entry with the same name. A later foreign V1 addition
// blocks V2 activation/update, but exact-owned removal edits only the V2 leaf.
func TestOpenCodeV2MixedRootReceiptOnlyCleanup(t *testing.T) {
	paths := Paths{JSON: filepath.Join(t.TempDir(), "opencode.json")}
	kernel := New()
	req := Request{Paths: paths, Codec: CodecOpenCodeV2, Action: ActionAdd, Name: "owned", Server: Server{Type: "remote", URL: "https://mcp.test"}}
	owned, err := kernel.Apply(req)
	if err != nil {
		t.Fatal(err)
	}
	before := mustRead(t, paths.JSON)
	foreign := `"owned":{"type":"remote","url":"https://foreign.test","keep":true}`
	mixed := strings.Replace(before, `"mcp":{`, `"mcp":{`+foreign+`,`, 1)
	if mixed == before {
		t.Fatal("fixture did not add flat foreign entry")
	}
	mustWrite(t, paths.JSON, mixed)
	present, exact, err := kernel.Inspect(paths, CodecOpenCodeV2, "owned", &owned)
	if err != nil || !present || !exact {
		t.Fatalf("mixed root lost receipt authority: %v %v %v", present, exact, err)
	}
	req.Action, req.Owned = ActionUpdate, &owned
	if _, err := kernel.Apply(req); !errors.Is(err, ErrNativeMigrationRequired) {
		t.Fatalf("mixed update accepted: %v", err)
	}
	assertBytes(t, paths.JSON, mixed)
	req.Action = ActionRemove
	if _, err := kernel.Apply(req); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(mustRead(t, paths.JSON), foreign) {
		t.Fatal("same-name flat foreign entry removed")
	}
	present, _, err = kernel.Inspect(paths, CodecOpenCodeV2, "owned", &owned)
	if err != nil || present {
		t.Fatalf("owned nested entry remains: %v %v", present, err)
	}
}

// Regression: treating all proposed V2 names as active rejects a disabled
// managed server even though it exposes no tools. Enabling it must recheck the
// namespace and retain its disabled receipt/bytes if the name conflicts.
func TestOpenCodeV2DisabledOwnedNamespaceAndEnableConflict(t *testing.T) {
	paths := Paths{JSON: filepath.Join(t.TempDir(), "opencode.json")}
	foreign := `"api/server":{"type":"remote","url":"https://foreign.test"}`
	mustWrite(t, paths.JSON, `{"mcp":{"servers":{`+foreign+`}}}`)
	kernel := New()
	req := Request{Paths: paths, Codec: CodecOpenCodeV2, Action: ActionAdd, Name: "api server", Server: Server{Type: "remote", URL: "https://mcp.test", OpenCodeV2: &OpenCodeV2Options{Disabled: true}}}
	owned, err := kernel.Apply(req)
	if err != nil {
		t.Fatalf("disabled server claimed active namespace: %v", err)
	}
	before := mustRead(t, paths.JSON)
	entry := readV2Config(t, paths.JSON)["mcp"].(map[string]any)["servers"].(map[string]any)["api server"].(map[string]any)
	if entry["disabled"] != true {
		t.Fatalf("disabled native flag missing: %#v", entry)
	}
	req.Action, req.Owned = ActionUpdate, &owned
	req.Server.OpenCodeV2.Disabled = false
	var collision *OpenCodeNamespaceConflict
	if _, err := kernel.Apply(req); !errors.As(err, &collision) {
		t.Fatalf("enable bypassed namespace conflict: %v", err)
	}
	assertBytes(t, paths.JSON, before)
	present, exact, err := kernel.Inspect(paths, CodecOpenCodeV2, req.Name, &owned)
	if err != nil || !present || !exact {
		t.Fatalf("failed enable invalidated disabled receipt: %v %v %v", present, exact, err)
	}
	req.Action = ActionRemove
	if _, err := kernel.Apply(req); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(mustRead(t, paths.JSON), foreign) {
		t.Fatal("foreign active server changed")
	}
}
