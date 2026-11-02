package skillscatalog

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/777genius/plugin-kit-ai/internal/skillsresolve"
)

type documentWebsiteStub struct {
	calls atomic.Int32
}

func (stub *documentWebsiteStub) PreviewHTTPSkill(_ context.Context, source, name string) (string, error) {
	stub.calls.Add(1)
	if source != "https://open.feishu.cn" || name != "lark-mail" {
		panic("unvalidated website preview coordinates")
	}
	return "---\nname: lark-mail\ndescription: Mail\n---\n# Mail\n\n**Hello**", nil
}

type documentGitStub struct{}

func (documentGitStub) Resolve(_ context.Context, r skillsresolve.Request) (skillsresolve.Result, error) {
	if r.Repository != "anthropics/skills" || r.SkillNameHint == nil || *r.SkillNameHint != "frontend-design" {
		panic("unexpected Git preview input")
	}
	return skillsresolve.Result{
		Status: "resolved", Commit: strings.Repeat("a", 40),
		Skill: &skillsresolve.Skill{RawContent: "# Design", RawSHA256: strings.Repeat("1", 64)},
	}, nil
}

func documentRequest(id, name, origin string) *http.Request {
	q := url.Values{"id": {id}, "name": {name}}
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/skills/document?"+q.Encode(), http.NoBody)
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	return r
}

func TestPublicDocumentPreviewSourceAuthorityCachingAndCORS(t *testing.T) {
	website := &documentWebsiteStub{}
	handler, err := NewDocumentHandler(documentGitStub{}, website, []string{"https://universal-agent-plugins.com"})
	if err != nil {
		t.Fatal(err)
	}
	id := "skills-sh:open.feishu.cn:lark-mail"
	for range 2 {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, documentRequest(id, "lark-mail", "https://universal-agent-plugins.com"))
		var response documentResponse
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &response) != nil ||
			response.Status != "resolved" || response.Source != "open.feishu.cn" ||
			!strings.Contains(response.Markdown, "**Hello**") || len(response.SHA256) != 64 {
			t.Fatal(w.Code, w.Body.String())
		}
		if w.Header().Get("Access-Control-Allow-Origin") != "https://universal-agent-plugins.com" {
			t.Fatal(w.Header())
		}
	}
	if website.calls.Load() != 1 {
		t.Fatal("cache did not prevent a second source fetch")
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, documentRequest(id, "lark-mail", "https://competitor.invalid"))
	if w.Code != 403 || website.calls.Load() != 1 {
		t.Fatal(w.Code)
	}
	for _, id := range []string{
		"skills-sh:127.0.0.1:lark-mail", "skills-sh:https://open.feishu.cn:lark-mail",
		"skills-sh:owner/repo/tree/main:mail", "skills-sh:open.feishu.cn:bad\nname",
	} {
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, documentRequest(id, "lark-mail", ""))
		if w.Code != 400 {
			t.Fatalf("unsafe id %q returned %d", id, w.Code)
		}
	}
}

func TestDocumentPreviewGitAndNameRefusal(t *testing.T) {
	handler, err := NewDocumentHandler(documentGitStub{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, documentRequest("skills-sh:anthropics/skills:frontend-design", "frontend-design", ""))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "# Design") ||
		!strings.Contains(w.Body.String(), strings.Repeat("a", 40)) {
		t.Fatal(w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	invalid := documentRequest("skills-sh:open.feishu.cn:lark-mail", "BAD$(echo attack)", "")
	invalid.RemoteAddr = "198.51.100.77:1234"
	handler.ServeHTTP(w, invalid)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "selection_required") {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestPublisherIPRefusesLocalReservedAndMappedAddresses(t *testing.T) {
	for _, candidate := range []string{
		"127.0.0.1", "10.1.2.3", "169.254.169.254", "192.168.0.1",
		"0.1.2.3", "100.64.1.1", "192.0.2.11", "192.88.99.1", "198.18.1.1",
		"::1", "fc00::12", "::ffff:127.0.0.1", "2001:db8::2",
		"64:ff9b::a00:1", "64:ff9b:1::1", "2001::1", "2001:10::1",
		"::127.0.0.1", "::169.254.169.254", "::ffff:0:169.254.169.254",
		"2002:7f00:1::1", "2002:a9fe:a9fe::1", "100::1", "2001:2::1",
		"3fff::1", "5f00::1", "fec0::1",
	} {
		if publicPublisherIP(net.ParseIP(candidate)) {
			t.Fatalf("accepted unsafe destination %s", candidate)
		}
	}
	for _, candidate := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"} {
		if !publicPublisherIP(net.ParseIP(candidate)) {
			t.Fatalf("refused public destination %s", candidate)
		}
	}
}

func TestDocumentPreviewPerClientAdmissionAndFailureCooldown(t *testing.T) {
	handler, err := NewDocumentHandler(documentGitStub{}, &documentWebsiteStub{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	h := handler.(*documentHandler)
	// A caller cannot hold multiple slots by changing the requested Skill.
	if !h.admitDocument("203.0.113.1") || h.admitDocument("203.0.113.1") {
		t.Fatal("same caller occupied more than one preview slot")
	}
	for _, other := range []string{"203.0.113.2", "203.0.113.3", "203.0.113.4"} {
		if !h.admitDocument(other) {
			t.Fatalf("available pool slot refused for %s", other)
		}
	}
	if h.admitDocument("203.0.113.5") {
		t.Fatal("bounded concurrency exceeded")
	}
	// Rejected callers must not be asked to retry before admission can reopen.
	rejected := documentRequest("skills-sh:open.feishu.cn:lark-mail", "lark-mail", "")
	rejected.RemoteAddr = "203.0.113.5:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, rejected)
	if w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") != "20" {
		t.Fatalf("incorrect admission retry guidance: status=%d retry-after=%q", w.Code, w.Header().Get("Retry-After"))
	}
	for _, client := range []string{"203.0.113.1", "203.0.113.2", "203.0.113.3", "203.0.113.4"} {
		h.releaseDocument(client)
	}
	// The same caller must cool down even after its request releases a slot.
	if h.admitDocument("203.0.113.1") {
		t.Fatal("client can monopolize source requests")
	}
	key := "skills-sh:publisher.example:mail\x00mail"
	h.rememberDocument(key, documentResponse{}, true)
	if _, found, failed := h.lookupDocument(key); !found || !failed {
		t.Fatal("failed source was not temporarily memoized")
	}
}

func TestDocumentClientIdentityCannotBeSpoofedByForwardedHeader(t *testing.T) {
	proxied := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/skills/document", http.NoBody)
	proxied.RemoteAddr = "127.0.0.1:23012"
	proxied.Header.Set("X-UAP-Preview-Client", "198.51.100.25")
	if got := documentClientKey(proxied); got != "198.51.100.25" {
		t.Fatalf("proxy-provided client identity ignored: %s", got)
	}
	direct := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/skills/document", http.NoBody)
	direct.RemoteAddr = "198.51.100.35:23012"
	direct.Header.Set("X-UAP-Preview-Client", "198.51.100.25")
	if got := documentClientKey(direct); got != "198.51.100.35" {
		t.Fatalf("untrusted peer spoofed client identity: %s", got)
	}
}

type abortOnceDocumentReader struct {
	cancel context.CancelFunc
	calls  atomic.Int32
}

func (stub *abortOnceDocumentReader) PreviewHTTPSkill(ctx context.Context, _, _ string) (string, error) {
	if stub.calls.Add(1) == 1 {
		stub.cancel()
		return "", ctx.Err()
	}
	return "---\nname: lark-mail\ndescription: Mail\n---\n# Mail", nil
}

func TestCancelledDocumentRequestDoesNotPoisonOtherClients(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	reader := &abortOnceDocumentReader{cancel: cancel}
	handler, err := NewDocumentHandler(nil, reader, nil)
	if err != nil {
		t.Fatal(err)
	}
	id := "skills-sh:open.feishu.cn:lark-mail"
	first := httptest.NewRequestWithContext(ctx, http.MethodGet,
		"/v1/skills/document?id=skills-sh%3Aopen.feishu.cn%3Alark-mail&name=lark-mail", http.NoBody)
	first.RemoteAddr = "198.51.100.111:12345"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, first)
	if w.Code != 503 {
		t.Fatalf("canceled request status: %d", w.Code)
	}
	if _, found, _ := handler.(*documentHandler).lookupDocument(id + "\x00lark-mail"); found {
		t.Fatal("canceled caller created a shared negative-cache entry")
	}
	second := documentRequest(id, "lark-mail", "")
	second.RemoteAddr = "198.51.100.112:12345"
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, second)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "# Mail") || reader.calls.Load() != 2 {
		t.Fatalf("other visitor should fetch the source: %d %s", w.Code, w.Body.String())
	}
}
