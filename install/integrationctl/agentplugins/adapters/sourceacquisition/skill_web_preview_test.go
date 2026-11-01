package sourceacquisition

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type websitePreviewTransport func(*http.Request) (*http.Response, error)

func (fn websitePreviewTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestPreviewHTTPSkillReadsOnlyDeclaredMarkdown(t *testing.T) {
	requests := []string{}
	client := &http.Client{Transport: websitePreviewTransport(func(r *http.Request) (*http.Response, error) {
		requests = append(requests, r.URL.Path)
		var content string
		switch r.URL.Path {
		case "/.well-known/agent-skills/index.json":
			content = `{"skills":[{"name":"lark-mail","files":["SKILL.md","references/guide.md"]}]}`
		case "/.well-known/agent-skills/lark-mail/SKILL.md":
			content = "---\nname: lark-mail\nmetadata:\n  requires:\n    bins: [lark-cli]\n---\n# Lark Mail"
		default:
			t.Errorf("unexpected companion fetch: %s", r.URL.Path)
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(content)), Header: make(http.Header)}, nil
	})}
	acquirer := Acquirer{HTTPClient: client}
	markdown, err := acquirer.PreviewHTTPSkill(context.Background(), "https://publisher.example", "lark-mail")
	if err != nil || !strings.Contains(markdown, "# Lark Mail") {
		t.Fatal(markdown, err)
	}
	if len(requests) != 2 {
		t.Fatalf("preview must not download companion resources: %v", requests)
	}
	if _, err := acquirer.PreviewHTTPSkill(context.Background(), "https://publisher.example", "-danger"); err == nil {
		t.Fatal("accepted unsafe Skill name")
	}
}

func TestPreviewHTTPSkillLegacyDiscoveryFallback(t *testing.T) {
	requests := []string{}
	client := &http.Client{Transport: websitePreviewTransport(func(r *http.Request) (*http.Response, error) {
		requests = append(requests, r.URL.Path)
		var content string
		switch r.URL.Path {
		case "/.well-known/agent-skills/index.json":
			content = "<!DOCTYPE html><html><body>App</body></html>"
		case "/.well-known/skills/index.json":
			content = `{"skills":[{"name":"lark-mail","files":["SKILL.md"]}]}`
		case "/.well-known/skills/lark-mail/SKILL.md":
			content = "---\nname: lark-mail\ndescription: Mail\n---\n# Lark Mail"
		default:
			t.Fatalf("unexpected preview request %s", r.URL.Path)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(content)), Header: make(http.Header)}, nil
	})}
	markdown, err := (Acquirer{HTTPClient: client}).PreviewHTTPSkill(context.Background(), "https://publisher.example", "lark-mail")
	if err != nil || !strings.Contains(markdown, "# Lark Mail") || len(requests) != 3 {
		t.Fatal(markdown, err, requests)
	}
}
