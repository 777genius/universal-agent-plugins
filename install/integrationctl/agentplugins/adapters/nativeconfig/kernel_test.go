package nativeconfig

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestMCPServersAddPreservesUnrelatedConfigAndResolvesExplicitPaths(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "client.json")
	original := `{"theme":"night","mcpServers":{"foreign":{"url":"https://foreign.test"}}}`
	mustWrite(t, path, original)

	receipt, err := New().Apply(Request{
		Paths:        Paths{JSON: path, JSONC: filepath.Join(root, "client.jsonc")},
		Codec:        CodecMCPServers,
		Action:       ActionAdd,
		Name:         "owned",
		Server:       Server{Type: "stdio", Command: "node", Args: []string{"${PLUGIN_ROOT}/server.js", "--data=${PLUGIN_DATA}"}, Env: map[string]string{"ROOT": "${PLUGIN_ROOT}"}},
		Placeholders: Placeholders{PackageRoot: "/explicit/pkg", DataRoot: "/explicit/data"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Version != "1" || receipt.Path != path || receipt.Codec != CodecMCPServers || receipt.Name != "owned" || !strings.HasPrefix(receipt.Digest, "sha256:") {
		t.Fatalf("unexpected receipt: %#v", receipt)
	}
	var doc map[string]any
	mustReadJSON(t, path, &doc)
	if doc["theme"] != "night" {
		t.Fatalf("unrelated config lost: %#v", doc)
	}
	servers := doc["mcpServers"].(map[string]any)
	if servers["foreign"].(map[string]any)["url"] != "https://foreign.test" {
		t.Fatalf("foreign entry changed: %#v", servers)
	}
	owned := servers["owned"].(map[string]any)
	if owned["command"] != "node" || owned["args"].([]any)[0] != "/explicit/pkg/server.js" || owned["args"].([]any)[1] != "--data=/explicit/data" || owned["env"].(map[string]any)["ROOT"] != "/explicit/pkg" {
		t.Fatalf("wrong projection: %#v", owned)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("existing mode changed: %o", info.Mode().Perm())
	}
}

func TestOpenCodeJSONCAddPreservesCommentsAndProjectsBothTransports(t *testing.T) {
	root := t.TempDir()
	jsonPath := filepath.Join(root, "opencode.json")
	jsoncPath := filepath.Join(root, "opencode.jsonc")
	mustWriteMode(t, jsoncPath, `{
  // keep this theme comment
  "theme": "night",
  "mcp": {
    // foreign entry comment
    "foreign": {"type":"remote", "url":"https://foreign.test"},
  },
}
`, 0o600)
	kernel := New()
	local, err := kernel.Apply(Request{
		Paths: Paths{JSON: jsonPath, JSONC: jsoncPath}, Codec: CodecOpenCode, Action: ActionAdd, Name: "local",
		Server:       Server{Type: "stdio", Command: "node", Args: []string{"${PLUGIN_ROOT}/run.js"}, Env: map[string]string{"DATA": "${PLUGIN_DATA}"}, CWD: "${PLUGIN_ROOT}/work"},
		Placeholders: Placeholders{PackageRoot: "/pkg", DataRoot: "/data"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = kernel.Apply(Request{
		Paths: Paths{JSON: jsonPath, JSONC: jsoncPath}, Codec: CodecOpenCode, Action: ActionAdd, Name: "remote",
		Server: Server{Type: "remote", URL: "https://mcp.test", Headers: map[string]string{"Authorization": "Bearer token"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	body := mustRead(t, jsoncPath)
	for _, comment := range []string{"// keep this theme comment", "// foreign entry comment"} {
		if !strings.Contains(body, comment) {
			t.Fatalf("comment %q was lost:\n%s", comment, body)
		}
	}
	doc, err := parseDocument([]byte(body), true)
	if err != nil {
		t.Fatal(err)
	}
	mcp, _ := collection(doc, "mcp", false)
	localMember, _ := objectMember(mcp, "local")
	var projected map[string]any
	standard, _ := entryCanonical(localMember)
	_ = json.Unmarshal(standard, &projected)
	if projected["type"] != "local" || projected["command"].([]any)[1] != "/pkg/run.js" || projected["environment"].(map[string]any)["DATA"] != "/data" || projected["cwd"] != "/pkg/work" {
		t.Fatalf("wrong OpenCode local projection: %#v", projected)
	}
	if local.Path != jsoncPath {
		t.Fatalf("receipt selected wrong config: %#v", local)
	}
	remoteMember, _ := objectMember(mcp, "remote")
	standard, _ = entryCanonical(remoteMember)
	_ = json.Unmarshal(standard, &projected)
	if projected["type"] != "remote" || projected["url"] != "https://mcp.test" {
		t.Fatalf("wrong OpenCode remote projection: %#v", projected)
	}
}

func TestCollisionUpdateAndRemoveRequireExactOwnership(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.json")
	mustWrite(t, path, `{"mcpServers":{"docs":{"command":"foreign"}}}`)
	kernel := New()
	req := Request{Paths: Paths{JSON: path}, Codec: CodecMCPServers, Action: ActionAdd, Name: "docs", Server: Server{Type: "stdio", Command: "node"}}
	before := mustRead(t, path)
	if _, err := kernel.Apply(req); !errors.Is(err, ErrCollision) {
		t.Fatalf("expected collision, got %v", err)
	}
	assertBytes(t, path, before)

	req.Name = "owned"
	receipt, err := kernel.Apply(req)
	if err != nil {
		t.Fatal(err)
	}
	forged := receipt
	forged.Digest = "sha256:00"
	update := req
	update.Action, update.Owned, update.Server = ActionUpdate, &forged, Server{Type: "stdio", Command: "deno"}
	before = mustRead(t, path)
	if _, err := kernel.Apply(update); !errors.Is(err, ErrNotOwned) {
		t.Fatalf("expected ownership rejection, got %v", err)
	}
	assertBytes(t, path, before)
	update.Owned = &receipt
	updated, err := kernel.Apply(update)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Digest == receipt.Digest {
		t.Fatal("updated entry digest did not change")
	}
	remove := update
	remove.Action, remove.Owned = ActionRemove, &receipt
	if _, err := kernel.Apply(remove); !errors.Is(err, ErrNotOwned) {
		t.Fatalf("stale receipt removed updated entry: %v", err)
	}
	remove.Owned = &updated
	if _, err := kernel.Apply(remove); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	mustReadJSON(t, path, &doc)
	servers := doc["mcpServers"].(map[string]any)
	if _, exists := servers["owned"]; exists || servers["docs"].(map[string]any)["command"] != "foreign" {
		t.Fatalf("remove touched wrong entry: %#v", servers)
	}
}

func TestApplyBatchWritesAllEntriesOnceAndFailsClosedOnCollision(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "settings.json")
	mustWrite(t, path, `{"theme":"night","mcpServers":{"foreign":{"url":"https://foreign.test"}}}`)
	kernel := New()
	requests := []Request{
		{Paths: Paths{JSON: path}, Codec: CodecGemini, Action: ActionAdd, Name: "local", Server: Server{Type: "stdio", Command: "node", CWD: "${PLUGIN_ROOT}/server"}, Placeholders: Placeholders{PackageRoot: "/pkg"}},
		{Paths: Paths{JSON: path}, Codec: CodecGemini, Action: ActionAdd, Name: "docs", Server: Server{Type: "remote", URL: "https://docs.test/mcp", RemoteTransport: "streamable-http"}},
		{Paths: Paths{JSON: path}, Codec: CodecGemini, Action: ActionAdd, Name: "events", Server: Server{Type: "remote", URL: "https://events.test/sse", RemoteTransport: "sse"}},
	}
	receipts, err := kernel.ApplyBatch(requests)
	if err != nil {
		t.Fatal(err)
	}
	if len(receipts) != 3 || receipts[0].Codec != CodecGemini {
		t.Fatalf("unexpected receipts: %#v", receipts)
	}
	var doc map[string]any
	mustReadJSON(t, path, &doc)
	servers := doc["mcpServers"].(map[string]any)
	if servers["local"].(map[string]any)["cwd"] != "/pkg/server" {
		t.Fatalf("Gemini cwd was not projected: %#v", servers["local"])
	}
	if servers["docs"].(map[string]any)["httpUrl"] != "https://docs.test/mcp" {
		t.Fatalf("Gemini streamable HTTP key was not projected: %#v", servers["docs"])
	}
	if servers["events"].(map[string]any)["url"] != "https://events.test/sse" {
		t.Fatalf("Gemini SSE key was not projected: %#v", servers["events"])
	}
	present, owned, err := kernel.Inspect(Paths{JSON: path}, CodecGemini, "docs", &receipts[1])
	if err != nil || !present || !owned {
		t.Fatalf("exact Gemini ownership was not observed: present=%v owned=%v err=%v", present, owned, err)
	}
	forged := receipts[1]
	forged.Digest = "sha256:00"
	_, owned, err = kernel.Inspect(Paths{JSON: path}, CodecGemini, "docs", &forged)
	if err != nil || owned {
		t.Fatalf("forged Gemini ownership was accepted: owned=%v err=%v", owned, err)
	}

	before := mustRead(t, path)
	_, err = kernel.ApplyBatch([]Request{
		{Paths: Paths{JSON: path}, Codec: CodecGemini, Action: ActionAdd, Name: "new", Server: Server{Type: "stdio", Command: "node"}},
		{Paths: Paths{JSON: path}, Codec: CodecGemini, Action: ActionAdd, Name: "foreign", Server: Server{Type: "stdio", Command: "node"}},
	})
	if !errors.Is(err, ErrCollision) {
		t.Fatalf("expected batch collision, got %v", err)
	}
	assertBytes(t, path, before)
}

func TestApplyBatchReturnsReceiptsWithTypedCleanupDegradationAfterCommit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	releaseErr := errors.New("injected lock release failure")
	kernel := Kernel{
		files: conditionalOSFiles{},
		acquireLocks: func(Paths, Codec) (func() error, error) {
			return func() error { return releaseErr }, nil
		},
	}
	requests := []Request{{
		Paths: Paths{JSON: path}, Codec: CodecCline, Action: ActionAdd, Name: "docs",
		Server: Server{Type: "stdio", Command: "node"},
	}}
	receipts, err := kernel.ApplyBatch(requests)
	if !IsCommittedCleanup(err) || !errors.Is(err, releaseErr) {
		t.Fatalf("post-commit release error = %v", err)
	}
	if len(receipts) != 1 || receipts[0].Name != "docs" {
		t.Fatalf("committed receipts = %#v", receipts)
	}
	present, owned, inspectErr := New().Inspect(Paths{JSON: path}, CodecCline, "docs", &receipts[0])
	if inspectErr != nil || !present || !owned {
		t.Fatalf("committed config/receipt diverged: present=%v owned=%v err=%v", present, owned, inspectErr)
	}
}

func TestApplyBatchJoinsPrimaryAndReleaseErrorsBeforeCommit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	mustWrite(t, path, `{"mcpServers":{"docs":{"transport":{"type":"stdio","command":"foreign"}}}}`)
	releaseErr := errors.New("injected lock release failure")
	kernel := Kernel{
		files: conditionalOSFiles{},
		acquireLocks: func(Paths, Codec) (func() error, error) {
			return func() error { return releaseErr }, nil
		},
	}
	_, err := kernel.Apply(Request{
		Paths: Paths{JSON: path}, Codec: CodecCline, Action: ActionAdd, Name: "docs",
		Server: Server{Type: "stdio", Command: "node"},
	})
	if !errors.Is(err, ErrCollision) || !errors.Is(err, releaseErr) {
		t.Fatalf("primary/release error precedence lost: %v", err)
	}
	if IsCommittedCleanup(err) {
		t.Fatalf("pre-commit failure was marked committed: %v", err)
	}
}

func TestDesiredReceiptBindingRejectsResolvedIdentityDriftBeforeWrite(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*Receipt)
	}{
		{name: "path", mutate: func(receipt *Receipt) { receipt.Path += ".jsonc" }},
		{name: "codec", mutate: func(receipt *Receipt) { receipt.Codec = CodecGemini }},
		{name: "name", mutate: func(receipt *Receipt) { receipt.Name = "other" }},
		{name: "digest", mutate: func(receipt *Receipt) { receipt.Digest = "sha256:00" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "opencode.json")
			server := Server{Type: "stdio", Command: "node", CWD: "${PLUGIN_ROOT}/work"}
			placeholders := Placeholders{PackageRoot: "/pkg"}
			desired, err := DesiredReceipt(path, CodecOpenCode, "docs", server, placeholders)
			if err != nil {
				t.Fatal(err)
			}
			test.mutate(&desired)
			_, err = New().Apply(Request{Paths: Paths{JSON: path}, Codec: CodecOpenCode, Action: ActionAdd,
				Name: "docs", Server: server, Placeholders: placeholders, Desired: &desired})
			if !errors.Is(err, ErrConcurrentChange) {
				t.Fatalf("expected pre-write desired receipt rejection, got %v", err)
			}
			if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
				t.Fatalf("desired identity drift wrote native config: %v", statErr)
			}
		})
	}
}

func TestMalformedDuplicateAndAmbiguousInputsFailClosed(t *testing.T) {
	tests := []string{
		`{"mcpServers":{},"mcpServers":{}}`,
		`{"mcpServers":{"x":{"env":{"A":"1","A":"2"}}}}`,
		`{"mcpServers":`,
		`[]`,
		`{"mcpServers":[]}`,
	}
	for i, body := range tests {
		t.Run(string(rune('a'+i)), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			mustWrite(t, path, body)
			before := mustRead(t, path)
			_, err := New().Apply(Request{Paths: Paths{JSON: path}, Codec: CodecMCPServers, Action: ActionAdd, Name: "owned", Server: Server{Type: "stdio", Command: "node"}})
			if !errors.Is(err, ErrMalformed) {
				t.Fatalf("expected malformed error, got %v", err)
			}
			assertBytes(t, path, before)
		})
	}
	root := t.TempDir()
	jsonPath, jsoncPath := filepath.Join(root, "config.json"), filepath.Join(root, "config.jsonc")
	mustWrite(t, jsonPath, `{}`)
	mustWrite(t, jsoncPath, `{}`)
	_, err := New().Apply(Request{Paths: Paths{JSON: jsonPath, JSONC: jsoncPath}, Codec: CodecMCPServers, Action: ActionAdd, Name: "owned", Server: Server{Type: "stdio", Command: "node"}})
	if !errors.Is(err, ErrAmbiguousConfig) {
		t.Fatalf("expected ambiguity error, got %v", err)
	}
}

func TestPlaceholdersAndSchemaValidationFailBeforeWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	mustWrite(t, path, `{"unrelated":true}`)
	before := mustRead(t, path)
	cases := []Server{
		{Type: "stdio", Command: "node", Args: []string{"${PLUGIN_ROOT}/run.js"}},
		{Type: "stdio", Command: "node", Headers: map[string]string{"X": "ignored"}},
		{Type: "remote", URL: "https://mcp.test", Command: "node"},
		{Type: "stdio", Command: "", URL: "https://mcp.test"},
	}
	for _, server := range cases {
		_, err := New().Apply(Request{Paths: Paths{JSON: path}, Codec: CodecMCPServers, Action: ActionAdd, Name: "owned", Server: server})
		if err == nil {
			t.Fatalf("expected projection failure for %#v", server)
		}
		assertBytes(t, path, before)
	}
}

func TestProjectionExpandsOnlyPortableStdioValueVariables(t *testing.T) {
	placeholders := Placeholders{PackageRoot: "/pkg", DataRoot: "/data"}
	stdio, err := projectServer(CodecMCPServers, Server{
		Type:    "stdio",
		Command: "${PLUGIN_ROOT}",
		Args:    []string{"${PLUGIN_ROOT}/run.js", "${PLUGIN_CACHE}/literal"},
		Env:     map[string]string{"CACHE": "${PLUGIN_DATA}/state", "UNKNOWN": "${HOME}"},
		CWD:     "${PLUGIN_ROOT}/work",
	}, placeholders)
	if err != nil {
		t.Fatal(err)
	}
	if stdio["command"] != "${PLUGIN_ROOT}" || stdio["cwd"] != "/pkg/work" {
		t.Fatalf("stdio command/cwd = %+v", stdio)
	}
	args := stdio["args"].([]string)
	if args[0] != "/pkg/run.js" || args[1] != "${PLUGIN_CACHE}/literal" {
		t.Fatalf("stdio args = %+v", args)
	}
	env := stdio["env"].(map[string]string)
	if env["CACHE"] != "/data/state" || env["UNKNOWN"] != "${HOME}" {
		t.Fatalf("stdio env = %+v", env)
	}

	remote, err := projectServer(CodecMCPServers, Server{
		Type:    "remote",
		URL:     "https://example.test/${PLUGIN_ROOT}",
		Headers: map[string]string{"X-Path": "${PLUGIN_DATA}"},
	}, placeholders)
	if err != nil {
		t.Fatal(err)
	}
	if remote["url"] != "https://example.test/${PLUGIN_ROOT}" {
		t.Fatalf("remote URL was expanded: %+v", remote)
	}
	headers := remote["headers"].(map[string]string)
	if headers["X-Path"] != "${PLUGIN_DATA}" {
		t.Fatalf("remote headers were expanded: %+v", headers)
	}

	nonRecursive, err := projectServer(CodecMCPServers, Server{
		Type: "stdio", Command: "node", Args: []string{"${PLUGIN_ROOT}/run.js"},
	}, Placeholders{PackageRoot: "/pkg/${PLUGIN_DATA}", DataRoot: "/data"})
	if err != nil {
		t.Fatal(err)
	}
	if got := nonRecursive["args"].([]string)[0]; got != "/pkg/${PLUGIN_DATA}/run.js" {
		t.Fatalf("replacement text was recursively expanded: %q", got)
	}
}

func TestRelativePathsAreRejected(t *testing.T) {
	_, err := New().Apply(Request{Paths: Paths{JSON: "config.json"}, Codec: CodecMCPServers, Action: ActionAdd, Name: "owned", Server: Server{Type: "stdio", Command: "node"}})
	if err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("expected absolute path rejection, got %v", err)
	}
}

func TestUncleanPathsAreRejected(t *testing.T) {
	root := t.TempDir()
	unclean := root + string(filepath.Separator) + "nested" + string(filepath.Separator) + ".." + string(filepath.Separator) + "config.json"
	_, err := New().Apply(Request{Paths: Paths{JSON: unclean}, Codec: CodecMCPServers, Action: ActionAdd, Name: "owned", Server: Server{Type: "stdio", Command: "node"}})
	if err == nil || !strings.Contains(err.Error(), "clean") {
		t.Fatalf("expected unclean path rejection, got %v", err)
	}
	if _, err := DesiredReceipt(unclean, CodecMCPServers, "owned", Server{Type: "stdio", Command: "node"}, Placeholders{}); err == nil || !strings.Contains(err.Error(), "clean") {
		t.Fatalf("expected pure receipt unclean path rejection, got %v", err)
	}
}

func TestRemoteTransportIsCodecSpecific(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if _, err := DesiredReceipt(path, CodecMCPServers, "docs", Server{Type: "remote", URL: "https://docs.test/mcp"}, Placeholders{}); err != nil {
		t.Fatalf("explicit generic url mapping regressed: %v", err)
	}
	for _, codec := range []Codec{CodecMCPServers, CodecOpenCode} {
		_, err := DesiredReceipt(path, codec, "docs", Server{Type: "remote", URL: "https://docs.test/mcp", RemoteTransport: "streamable-http"}, Placeholders{})
		if err == nil || !strings.Contains(err.Error(), "does not accept remote_transport") {
			t.Fatalf("codec %s accepted a foreign transport shape: %v", codec, err)
		}
	}
	if _, err := DesiredReceipt(path, CodecGemini, "docs", Server{Type: "remote", URL: "https://docs.test/mcp", RemoteTransport: "streamable-http"}, Placeholders{}); err != nil {
		t.Fatalf("Gemini transport-aware projection failed: %v", err)
	}
	for _, test := range []struct {
		transport string
		wantKey   string
	}{
		{transport: "streamable-http", wantKey: "serverUrl"},
		{transport: "sse", wantKey: "url"},
	} {
		receipt, err := DesiredReceipt(path, CodecWindsurf, "docs", Server{Type: "remote", URL: "https://docs.test/mcp", RemoteTransport: test.transport}, Placeholders{})
		if err != nil {
			t.Fatalf("Windsurf %s projection failed: %v", test.transport, err)
		}
		projected, err := projectServer(CodecWindsurf, Server{Type: "remote", URL: "https://docs.test/mcp", RemoteTransport: test.transport}, Placeholders{})
		if err != nil || projected[test.wantKey] != "https://docs.test/mcp" || !strings.HasPrefix(receipt.Digest, "sha256:") {
			t.Fatalf("wrong Windsurf %s projection: %#v receipt=%#v err=%v", test.transport, projected, receipt, err)
		}
	}
}

func TestCodecSpecificUnsupportedFieldsFailClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.json")
	projected, err := projectServer(CodecOpenCode, Server{Type: "stdio", Command: "node", CWD: "${PLUGIN_ROOT}/server"}, Placeholders{PackageRoot: "/pkg"})
	if err != nil || projected["cwd"] != "/pkg/server" {
		t.Fatalf("OpenCode cwd was not faithfully projected: %#v, %v", projected, err)
	}
	_, err = DesiredReceipt(path, CodecWindsurf, "local", Server{Type: "stdio", Command: "node", CWD: "/server"}, Placeholders{})
	if err == nil || !strings.Contains(err.Error(), "does not accept cwd") {
		t.Fatalf("Windsurf silently discarded cwd: %v", err)
	}
	_, err = DesiredReceipt(path, CodecCline, "local", Server{Type: "stdio", Command: "node", CWD: "/server"}, Placeholders{})
	if err != nil {
		t.Fatalf("Cline native cwd rejected: %v", err)
	}
}

func TestClineNestedTransportProjectionAndCompatibleLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cline_mcp_settings.json")
	kernel := New()
	requests := []Request{
		{Paths: Paths{JSON: path}, Codec: CodecCline, Action: ActionAdd, Name: "local", Server: Server{Type: "stdio", Command: "node", Args: []string{"server.js"}, Env: map[string]string{"A": "1"}}},
		{Paths: Paths{JSON: path}, Codec: CodecCline, Action: ActionAdd, Name: "http", Server: Server{Type: "remote", URL: "https://mcp.test", RemoteTransport: "streamable-http", Headers: map[string]string{"X": "1"}}},
		{Paths: Paths{JSON: path}, Codec: CodecCline, Action: ActionAdd, Name: "events", Server: Server{Type: "remote", URL: "https://mcp.test/sse", RemoteTransport: "sse"}},
	}
	if _, err := kernel.ApplyBatch(requests); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	mustReadJSON(t, path, &doc)
	servers := doc["mcpServers"].(map[string]any)
	local := servers["local"].(map[string]any)["transport"].(map[string]any)
	http := servers["http"].(map[string]any)["transport"].(map[string]any)
	events := servers["events"].(map[string]any)["transport"].(map[string]any)
	if local["type"] != "stdio" || local["command"] != "node" || http["type"] != "streamableHttp" || http["url"] != "https://mcp.test" || events["type"] != "sse" {
		t.Fatalf("wrong Cline projections: local=%#v http=%#v events=%#v", local, http, events)
	}
	if _, err := os.Stat(path + ".lock"); !os.IsNotExist(err) {
		t.Fatalf("Cline lock directory was not released: %v", err)
	}
}

func TestDesiredReceiptIsPureAndMatchesApply(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	server := Server{Type: "stdio", Command: "node", Args: []string{"${PLUGIN_ROOT}/server.js"}, Env: map[string]string{"DATA": "${PLUGIN_DATA}"}}
	placeholders := Placeholders{PackageRoot: "/pkg", DataRoot: "/data"}
	want, err := DesiredReceipt(path, CodecMCPServers, "docs", server, placeholders)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("DesiredReceipt mutated the filesystem: %v", err)
	}
	got, err := New().Apply(Request{Paths: Paths{JSON: path}, Codec: CodecMCPServers, Action: ActionAdd, Name: "docs", Server: server, Placeholders: placeholders})
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("Apply receipt %#v differs from DesiredReceipt %#v", got, want)
	}
}

type interleavingIO struct {
	osFiles
	reads int
}

func (io *interleavingIO) ReadNoFollow(path string) ([]byte, os.FileMode, bool, error) {
	io.reads++
	if io.reads == 2 {
		if err := os.WriteFile(path, []byte(`{"client":"concurrent"}`), 0o640); err != nil {
			return nil, 0, false, err
		}
	}
	return io.osFiles.ReadNoFollow(path)
}

func TestConcurrentChangeBeforeReplaceIsNotOverwritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	mustWrite(t, path, `{"client":"original"}`)
	_, err := NewWithFileIO(&interleavingIO{}).Apply(Request{Paths: Paths{JSON: path}, Codec: CodecMCPServers, Action: ActionAdd, Name: "owned", Server: Server{Type: "stdio", Command: "node"}})
	if !errors.Is(err, ErrConcurrentChange) {
		t.Fatalf("expected concurrent change, got %v", err)
	}
	assertBytes(t, path, `{"client":"concurrent"}`)
}

type concurrentMemoryIO struct {
	mu        sync.Mutex
	body      []byte
	active    int
	maxActive int
}

func newConcurrentMemoryIO() *concurrentMemoryIO {
	return &concurrentMemoryIO{body: []byte(`{}`)}
}

func (io *concurrentMemoryIO) ReadNoFollow(string) ([]byte, os.FileMode, bool, error) {
	io.mu.Lock()
	body := append([]byte(nil), io.body...)
	io.mu.Unlock()
	return body, 0o600, true, nil
}

func (io *concurrentMemoryIO) WriteAtomic(_ string, body []byte, _ os.FileMode) error {
	io.mu.Lock()
	io.active++
	if io.active > io.maxActive {
		io.maxActive = io.active
	}
	io.mu.Unlock()
	time.Sleep(10 * time.Millisecond)
	io.mu.Lock()
	io.body = append([]byte(nil), body...)
	io.active--
	io.mu.Unlock()
	return nil
}

func (*concurrentMemoryIO) RemoveNoFollow(string) error { return nil }

func TestProcessScopedLockSerializesOurConcurrentWriters(t *testing.T) {
	files := newConcurrentMemoryIO()
	kernel := NewWithFileIO(files)
	path := filepath.Join(t.TempDir(), "config.json")
	errs := make(chan error, 2)
	for _, name := range []string{"first", "second"} {
		go func() {
			_, err := kernel.Apply(Request{Paths: Paths{JSON: path}, Codec: CodecMCPServers, Action: ActionAdd, Name: name, Server: Server{Type: "stdio", Command: "node"}})
			errs <- err
		}()
	}
	var successes int
	for range 2 {
		err := <-errs
		if err == nil {
			successes++
		} else {
			t.Fatalf("unexpected concurrent writer result: %v", err)
		}
	}
	if successes != 2 {
		t.Fatalf("results: successes=%d", successes)
	}
	files.mu.Lock()
	defer files.mu.Unlock()
	if files.maxActive != 1 {
		t.Fatalf("our writes overlapped: max active=%d", files.maxActive)
	}
}

type boundaryMutationIO struct {
	osFiles
	beforeCAS    func(path string, expected []byte, expectedExists bool, next []byte) error
	beforeRemove func(path string, expected []byte) error
	compareCalls int
	removeCalls  int
}

func (io *boundaryMutationIO) CompareAndSwap(path string, expected []byte, expectedExists bool, next []byte, mode os.FileMode) error {
	io.compareCalls++
	if io.beforeCAS != nil {
		if err := io.beforeCAS(path, expected, expectedExists, next); err != nil {
			return err
		}
	}
	return (conditionalOSFiles{io.osFiles}).CompareAndSwap(path, expected, expectedExists, next, mode)
}

func (io *boundaryMutationIO) RemoveIfUnchanged(path string, expected []byte) error {
	io.removeCalls++
	if io.beforeRemove != nil {
		if err := io.beforeRemove(path, expected); err != nil {
			return err
		}
	}
	return (conditionalOSFiles{io.osFiles}).RemoveIfUnchanged(path, expected)
}

func TestMutationAtReplacementBoundaryFailsClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	mustWrite(t, path, `{"client":"original"}`)
	files := &boundaryMutationIO{}
	files.beforeCAS = func(path string, _ []byte, _ bool, _ []byte) error {
		files.beforeCAS = nil
		return os.WriteFile(path, []byte(`{"client":"boundary-writer"}`), 0o640)
	}
	_, err := NewWithFileIO(files).Apply(Request{Paths: Paths{JSON: path}, Codec: CodecMCPServers, Action: ActionAdd, Name: "owned", Server: Server{Type: "stdio", Command: "node"}})
	if !errors.Is(err, ErrConcurrentChange) {
		t.Fatalf("expected boundary CAS rejection, got %v", err)
	}
	assertBytes(t, path, `{"client":"boundary-writer"}`)
}

func TestMutationAtRollbackBoundaryIsNotClobbered(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	mustWrite(t, path, `{"client":"original"}`)
	files := &boundaryMutationIO{}
	files.beforeCAS = func(path string, expected []byte, expectedExists bool, next []byte) error {
		if files.compareCalls == 1 {
			if err := (conditionalOSFiles{files.osFiles}).CompareAndSwap(path, expected, expectedExists, next, 0o640); err != nil {
				return err
			}
			return errors.New("injected post-write failure")
		}
		return os.WriteFile(path, []byte(`{"client":"rollback-writer"}`), 0o640)
	}
	_, err := NewWithFileIO(files).Apply(Request{Paths: Paths{JSON: path}, Codec: CodecMCPServers, Action: ActionAdd, Name: "owned", Server: Server{Type: "stdio", Command: "node"}})
	if !errors.Is(err, ErrConcurrentChange) {
		t.Fatalf("expected rollback CAS rejection, got %v", err)
	}
	assertBytes(t, path, `{"client":"rollback-writer"}`)
}

func TestAlternateAppearanceAfterResolveRestoresSelectedWithoutClobberingAlternate(t *testing.T) {
	root := t.TempDir()
	jsonPath := filepath.Join(root, "opencode.json")
	jsoncPath := filepath.Join(root, "opencode.jsonc")
	mustWrite(t, jsonPath, `{"theme":"night"}`)
	files := &boundaryMutationIO{}
	files.beforeCAS = func(_ string, _ []byte, _ bool, _ []byte) error {
		files.beforeCAS = nil
		return os.WriteFile(jsoncPath, []byte(`{"foreign":true}`), 0o640)
	}
	_, err := NewWithFileIO(files).Apply(Request{Paths: Paths{JSON: jsonPath, JSONC: jsoncPath}, Codec: CodecOpenCode, Action: ActionAdd, Name: "owned", Server: Server{Type: "stdio", Command: "node"}})
	if !errors.Is(err, ErrAmbiguousConfig) {
		t.Fatalf("expected alternate-path ambiguity, got %v", err)
	}
	assertBytes(t, jsonPath, `{"theme":"night"}`)
	assertBytes(t, jsoncPath, `{"foreign":true}`)
}

func TestNoFollowRefusesSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is not generally available to unprivileged Windows tests")
	}
	root := t.TempDir()
	target := filepath.Join(root, "target.json")
	link := filepath.Join(root, "config.json")
	mustWrite(t, target, `{"safe":true}`)
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	_, err := New().Apply(Request{Paths: Paths{JSON: link}, Codec: CodecMCPServers, Action: ActionAdd, Name: "owned", Server: Server{Type: "stdio", Command: "node"}})
	if err == nil {
		t.Fatal("expected symlink refusal")
	}
	assertBytes(t, target, `{"safe":true}`)
}

func TestInterprocessLockRefusesSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is not generally available to unprivileged Windows tests")
	}
	root := t.TempDir()
	path := filepath.Join(root, "config.json")
	target := filepath.Join(root, "lock-target")
	mustWrite(t, path, `{}`)
	mustWrite(t, target, `do-not-touch`)
	if err := os.Symlink(target, path+".agentplugins.lock"); err != nil {
		t.Fatal(err)
	}
	_, err := New().Apply(Request{Paths: Paths{JSON: path}, Codec: CodecMCPServers, Action: ActionAdd, Name: "owned", Server: Server{Type: "stdio", Command: "node"}})
	if err == nil || !strings.Contains(err.Error(), "lock native config") {
		t.Fatalf("expected lock symlink refusal, got %v", err)
	}
	assertBytes(t, path, `{}`)
	assertBytes(t, target, `do-not-touch`)
}

type faultIO struct {
	osFiles
	mode       string
	writeCount int
}

func (io *faultIO) WriteAtomic(path string, body []byte, mode os.FileMode) error {
	io.writeCount++
	if io.writeCount == 1 {
		written := []byte("corrupt")
		if io.mode == "error-exact" {
			written = body
		}
		if err := os.WriteFile(path, written, mode); err != nil {
			return err
		}
		if io.mode == "error-exact" {
			return errors.New("injected post-write failure")
		}
		return nil
	}
	return io.osFiles.WriteAtomic(path, body, mode)
}

func TestFailedExactWriteRestoresOriginalWithoutClobberingUnknownBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.jsonc")
	original := "{\n  // exact bytes must return\n  \"theme\": \"night\",\n}\n"
	mustWrite(t, path, original)
	files := &faultIO{mode: "error-exact"}
	_, err := NewWithFileIO(files).Apply(Request{Paths: Paths{JSON: filepath.Join(filepath.Dir(path), "config.json"), JSONC: path}, Codec: CodecOpenCode, Action: ActionAdd, Name: "owned", Server: Server{Type: "stdio", Command: "node"}})
	if err == nil {
		t.Fatal("expected injected write failure")
	}
	assertBytes(t, path, original)
	if files.writeCount != 2 {
		t.Fatalf("expected write plus rollback, got %d writes", files.writeCount)
	}

	files = &faultIO{mode: "incorrect"}
	_, err = NewWithFileIO(files).Apply(Request{Paths: Paths{JSON: filepath.Join(filepath.Dir(path), "config.json"), JSONC: path}, Codec: CodecOpenCode, Action: ActionAdd, Name: "owned", Server: Server{Type: "stdio", Command: "node"}})
	if err == nil || !errors.Is(err, ErrConcurrentChange) {
		t.Fatalf("expected no-clobber verification error, got %v", err)
	}
	assertBytes(t, path, "corrupt")
	if files.writeCount != 1 {
		t.Fatalf("unknown bytes were overwritten by rollback: %d writes", files.writeCount)
	}
}

type mutateOnVerifyIO struct {
	osFiles
	reads  int
	writes int
}

func (io *mutateOnVerifyIO) ReadNoFollow(path string) ([]byte, os.FileMode, bool, error) {
	io.reads++
	if io.writes == 1 && io.reads == 3 {
		if err := os.WriteFile(path, []byte(`{"client":"third-party"}`), 0o640); err != nil {
			return nil, 0, false, err
		}
	}
	return io.osFiles.ReadNoFollow(path)
}

func (io *mutateOnVerifyIO) WriteAtomic(path string, body []byte, mode os.FileMode) error {
	io.writes++
	return io.osFiles.WriteAtomic(path, body, mode)
}

func TestConcurrentChangeAfterWriteBeforeVerifyIsNotRolledBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	mustWrite(t, path, `{"client":"original"}`)
	files := &mutateOnVerifyIO{}
	_, err := NewWithFileIO(files).Apply(Request{Paths: Paths{JSON: path}, Codec: CodecMCPServers, Action: ActionAdd, Name: "owned", Server: Server{Type: "stdio", Command: "node"}})
	if !errors.Is(err, ErrConcurrentChange) {
		t.Fatalf("expected no-clobber concurrent change, got %v", err)
	}
	assertBytes(t, path, `{"client":"third-party"}`)
	if files.writes != 1 {
		t.Fatalf("third-party bytes were overwritten by rollback: %d writes", files.writes)
	}
}

func TestForeignFieldInsideOwnedEntryInvalidatesFullEntryReceipt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	req := Request{Paths: Paths{JSON: path}, Codec: CodecMCPServers, Action: ActionAdd, Name: "docs", Server: Server{Type: "stdio", Command: "node"}}
	receipt, err := New().Apply(req)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, path, `{"mcpServers":{"docs":{"command":"node","foreign":true}}}`)
	before := mustRead(t, path)
	for _, action := range []Action{ActionUpdate, ActionRemove} {
		mutate := req
		mutate.Action, mutate.Owned = action, &receipt
		if _, err := New().Apply(mutate); !errors.Is(err, ErrNotOwned) {
			t.Fatalf("%s accepted a receipt after foreign field insertion: %v", action, err)
		}
		assertBytes(t, path, before)
	}
}

func TestNewFileUsesPrivateModeAndStableDigest(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.json")
	req := Request{Paths: Paths{JSON: path}, Codec: CodecMCPServers, Action: ActionAdd, Name: "owned", Server: Server{Type: "stdio", Command: "node", Env: map[string]string{"B": "2", "A": "1"}}}
	receipt, err := New().Apply(req)
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("new config mode is %o", info.Mode().Perm())
	}
	update := req
	update.Action, update.Owned = ActionUpdate, &receipt
	next, err := New().Apply(update)
	if err != nil {
		t.Fatal(err)
	}
	if next.Digest != receipt.Digest {
		t.Fatalf("semantically identical entry digest changed: %s != %s", next.Digest, receipt.Digest)
	}
}

func mustWrite(t *testing.T, path, body string) { t.Helper(); mustWriteMode(t, path, body, 0o640) }

func mustWriteMode(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func assertBytes(t *testing.T, path, want string) {
	t.Helper()
	if got := mustRead(t, path); got != want {
		t.Fatalf("bytes changed:\nwant %q\n got %q", want, got)
	}
}

func mustReadJSON(t *testing.T, path string, out any) {
	t.Helper()
	if err := json.Unmarshal([]byte(mustRead(t, path)), out); err != nil {
		t.Fatal(err)
	}
}
