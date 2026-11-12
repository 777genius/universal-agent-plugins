package mcpskills

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

const sampleSkill = "---\nname: review\ndescription: Review changed files\nmetadata:\n  owner: tests\n---\n# Review\n"
const sampleURI = "skill://acme/review/SKILL.md"

type mockRemote struct {
	mu            sync.Mutex
	methods       []string
	sse           bool
	disabled      bool
	badDigest     bool
	wrongDocument bool
	pageLoop      bool
	dynamic       bool
	badCache      bool
}

func (m *mockRemote) handler(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r.Method != http.MethodPost || r.Header.Get("Mcp-Method") == "" ||
		r.Header.Get("MCP-Protocol-Version") != protocolVersion ||
		!strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		w.WriteHeader(400)
		return
	}
	var req struct {
		Method string `json:"method"`
		Params struct {
			URI    string                     `json:"uri"`
			Cursor string                     `json:"cursor"`
			Meta   map[string]json.RawMessage `json:"_meta"`
		} `json:"params"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Method != r.Header.Get("Mcp-Method") ||
		string(req.Params.Meta["io.modelcontextprotocol/protocolVersion"]) != `"2026-07-28"` {
		w.WriteHeader(400)
		return
	}
	if req.Method == "resources/read" && r.Header.Get("Mcp-Name") != req.Params.URI {
		w.WriteHeader(400)
		return
	}
	m.methods = append(m.methods, req.Method)
	cache := map[string]any{"resultType": "complete", "ttlMs": 0, "cacheScope": "private"}
	if m.badCache {
		delete(cache, "cacheScope")
	}
	for k, v := range m.response(req.Method, req.Params.URI, req.Params.Cursor) {
		cache[k] = v
	}
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "result": cache})
	if m.sse {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, ": ready\n\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/progress\",\"params\":{}}\n\n")
		_, _ = fmt.Fprintf(w, "data: %s\n\n", body)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}

func (m *mockRemote) response(method, uri, cursor string) map[string]any {
	digest := sha256.Sum256([]byte(sampleSkill))
	hash := "sha256:" + hex.EncodeToString(digest[:])
	if m.badDigest {
		hash = "sha256:" + strings.Repeat("0", 64)
	}
	resources := any([]Resource{{URI: sampleURI, Digest: hash, Size: int64(len(sampleSkill))}})
	if m.dynamic {
		resources = "dynamic"
	}
	skill := Skill{URI: sampleURI, Frontmatter: map[string]any{"name": "review", "description": "Review changed files", "metadata": map[string]any{"owner": "tests"}}, Resources: nil}
	b, _ := json.Marshal(resources)
	skill.Resources = b
	switch method {
	case "server/discover":
		ex := map[string]any{extensionID: map[string]any{}}
		if m.disabled {
			delete(ex, extensionID)
		}
		return map[string]any{"supportedVersions": []string{protocolVersion}, "capabilities": map[string]any{"resources": map[string]any{}, "extensions": ex}}
	case "skills/list":
		next := ""
		// Two identical names but distinct paths are allowed; no shadowing.
		var skills []Skill
		if cursor == "" {
			skills = []Skill{skill}
			next = "second"
		} else {
			other := skill
			other.URI = "skill://other/review/SKILL.md"
			other.Resources, _ = json.Marshal([]Resource{{URI: other.URI, Digest: hash, Size: int64(len(sampleSkill))}})
			skills = []Skill{other}
			if m.pageLoop {
				next = "second"
			}
		}
		return map[string]any{"skills": skills, "nextCursor": next}
	case "skills/get":
		return map[string]any{"skill": skill}
	case "resources/read":
		body := sampleSkill
		if m.wrongDocument {
			body = "---\nname: review\ndescription: changed\n---\n# Review\n"
		}
		return map[string]any{"contents": []any{map[string]any{"uri": uri, "text": body, "mimeType": "text/markdown"}}}
	}
	return map[string]any{}
}

func newMockClient(t *testing.T, m *mockRemote) *Client {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(m.handler))
	t.Cleanup(server.Close)
	serverURL, _ := url.Parse(server.URL)
	original := server.Client()
	source := original.Transport
	original.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		copy := request.Clone(request.Context())
		copy.URL.Scheme, copy.URL.Host = "https", serverURL.Host
		copy.Host = serverURL.Host
		return source.RoundTrip(copy)
	})
	c, err := NewWithHTTPClient("https://example.com/mcp", original)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestMCPReadOnlyListAndInspectE2E(t *testing.T) {
	for _, sse := range []bool{false, true} {
		t.Run(fmt.Sprint("sse-", sse), func(t *testing.T) {
			m := &mockRemote{sse: sse}
			client := newMockClient(t, m)
			items, err := client.List(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(items) != 2 || items[0].Name != items[1].Name || items[0].URI == items[1].URI {
				t.Fatalf("wrong collision behavior: %+v", items)
			}
			for _, method := range m.methods {
				if method == "resources/read" || method == "tools/call" {
					t.Fatalf("list triggered %s", method)
				}
			}
			result, err := client.Inspect(context.Background(), sampleURI)
			if err != nil {
				t.Fatal(err)
			}
			if result.Integrity != "verified" || !strings.HasPrefix(result.Digest, "sha256:") {
				t.Fatalf("not verified: %+v", result)
			}
			if strings.Contains(fmt.Sprint(result), "# Review") {
				t.Fatal("instruction body leaked")
			}
		})
	}
}

func TestMCPRefusesInvalidResults(t *testing.T) {
	cases := []struct {
		name   string
		change func(*mockRemote)
		list   bool
	}{
		{"missing extension", func(m *mockRemote) { m.disabled = true }, true},
		{"bad digest", func(m *mockRemote) { m.badDigest = true }, false},
		{"frontmatter conflict", func(m *mockRemote) { m.wrongDocument = true }, false},
		{"pagination loop", func(m *mockRemote) { m.pageLoop = true }, true},
		{"missing cache hints", func(m *mockRemote) { m.badCache = true }, true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			m := &mockRemote{}
			test.change(m)
			c := newMockClient(t, m)
			var err error
			if test.list {
				_, err = c.List(context.Background())
			} else {
				_, err = c.Inspect(context.Background(), sampleURI)
			}
			if err == nil {
				t.Fatal("expected refusal")
			}
		})
	}
}

func TestDynamicNeverFetchedOrVerified(t *testing.T) {
	m := &mockRemote{dynamic: true}
	client := newMockClient(t, m)
	result, err := client.Inspect(context.Background(), sampleURI)
	if err != nil || result.Integrity != "unverifiable_dynamic" {
		t.Fatalf("got %+v, %v", result, err)
	}
	for _, method := range m.methods {
		if method == "resources/read" {
			t.Fatal("dynamic resource read")
		}
	}
}

func TestURIAndManifestValidation(t *testing.T) {
	for _, uri := range []string{"skill://acme/review/../SKILL.md", "skill://acme/review/%2e%2e/SKILL.md", "skill://acme/review/%2fSKILL.md", "skill://acme/review/SKILL.md?token=secret", "skill://acme/review/SKILL.md#x", "skill://acme/review/\nSKILL.md"} {
		if validateURI(uri) == nil {
			t.Errorf("accepted URI %q", uri)
		}
	}
	if validateURI("skill://acme/review/SKILL.md") != nil {
		t.Fatal("valid URI refused")
	}
	if !publicAddress(netIP(t, "8.8.8.8")) || publicAddress(netIP(t, "127.0.0.1")) || publicAddress(netIP(t, "192.168.1.1")) {
		t.Fatal("public IP policy regression")
	}
}

func netIP(t *testing.T, value string) net.IP {
	t.Helper()
	v := net.ParseIP(value)
	if v == nil {
		t.Fatal(value)
	}
	return v
}

func TestMCPRejectsMalformedManifests(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	cases := []struct {
		name      string
		resources string
		skillName string
	}{
		{"missing file size", fmt.Sprintf(`[{"uri":%q,"digest":%q}]`, sampleURI, digest), "review"},
		{"missing document", fmt.Sprintf(`[{"uri":%q,"digest":%q,"size":1}]`, "skill://acme/review/other.md", digest), "review"},
		{"outside root", fmt.Sprintf(`[{"uri":%q,"digest":%q,"size":1}]`, "skill://foreign/review/SKILL.md", digest), "review"},
		{"duplicate URI", fmt.Sprintf(`[{"uri":%q,"digest":%q,"size":1},{"uri":%q,"digest":%q,"size":1}]`, sampleURI, digest, sampleURI, digest), "review"},
		{"negative size", fmt.Sprintf(`[{"uri":%q,"digest":%q,"size":-1}]`, sampleURI, digest), "review"},
		{"name mismatch", fmt.Sprintf(`[{"uri":%q,"digest":%q,"size":1}]`, sampleURI, digest), "different"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := Skill{URI: sampleURI, Frontmatter: map[string]any{"name": tc.skillName, "description": "Review changed files"}, Resources: json.RawMessage(tc.resources)}
			if _, _, err := validateSkill(s); err == nil {
				t.Fatal("invalid manifest accepted")
			}
		})
	}
}

func TestMCPHeadersAndSSEFailClosed(t *testing.T) {
	encoded := encodeHeaderValue("skill://учёт/SKILL.md")
	if !strings.HasPrefix(encoded, "=?base64?") || !strings.HasSuffix(encoded, "?=") {
		t.Fatalf("invalid nonascii Mcp-Name: %s", encoded)
	}
	valid := `{"jsonrpc":"2.0","id":1,"result":{"resultType":"complete"}}`
	if _, err := decodeEvents([]byte("data: " + valid + "\n\ndata: " + valid + "\n\n")); err == nil {
		t.Fatal("accepted duplicate final SSE response")
	}
	if _, err := decodeEvents([]byte("data: {\"jsonrpc\":\"2.0\",\"id\":2,\"result\":{}}\n\n")); err == nil {
		t.Fatal("accepted response with wrong ID")
	}
}

func TestMCPServerIdentityContainsEndpointPath(t *testing.T) {
	client := &http.Client{}
	a, err := NewWithHTTPClient("https://mcp.example.com/team-a", client)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewWithHTTPClient("https://mcp.example.com/team-b", client)
	if err != nil {
		t.Fatal(err)
	}
	if a.Origin() == b.Origin() || strings.Contains(a.Origin(), "team-a") || strings.Contains(b.Origin(), "team-b") {
		t.Fatalf("origin identity is colliding or leaking: %s %s", a.Origin(), b.Origin())
	}
}
