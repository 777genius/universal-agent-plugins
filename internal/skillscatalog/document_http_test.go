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
	r := httptest.NewRequest(http.MethodGet, "/v1/skills/document?"+q.Encode(), nil)
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
	handler.ServeHTTP(w, documentRequest("skills-sh:open.feishu.cn:lark-mail", "BAD$(echo attack)", ""))
	if w.Code != 429 && w.Code != 200 {
		t.Fatal(w.Code)
	}
}

func TestPublisherIPRefusesLocalReservedAndMappedAddresses(t *testing.T) {
	for _, candidate := range []string{
		"127.0.0.1", "10.1.2.3", "169.254.169.254", "192.168.0.1",
		"0.1.2.3", "100.64.1.1", "192.0.2.11", "192.88.99.1", "198.18.1.1",
		"::1", "fc00::12", "::ffff:127.0.0.1", "2001:db8::2",
		"64:ff9b::a00:1", "64:ff9b:1::1", "2001::1", "2001:10::1",
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
