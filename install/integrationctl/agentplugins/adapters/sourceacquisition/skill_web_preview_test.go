package sourceacquisition

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPreviewHTTPSkillReadsOnlyDeclaredMarkdown(t *testing.T) {
	requests := []string{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Path)
		switch r.URL.Path {
		case "/.well-known/agent-skills/index.json":
			_, _ = w.Write([]byte(`{"skills":[{"name":"lark-mail","files":["SKILL.md","references/guide.md"]}]}`))
		case "/.well-known/agent-skills/lark-mail/SKILL.md":
			_, _ = w.Write([]byte("---\nname: lark-mail\nmetadata:\n  requires:\n    bins: [lark-cli]\n---\n# Lark Mail"))
		default:
			t.Errorf("unexpected companion fetch: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	acquirer := Acquirer{HTTPClient: server.Client()}
	markdown, err := acquirer.PreviewHTTPSkill(context.Background(), server.URL, "lark-mail")
	if err != nil || !strings.Contains(markdown, "# Lark Mail") {
		t.Fatal(markdown, err)
	}
	if len(requests) != 2 {
		t.Fatalf("preview must not download companion resources: %v", requests)
	}
	if _, err := acquirer.PreviewHTTPSkill(context.Background(), server.URL, "-danger"); err == nil {
		t.Fatal("accepted unsafe Skill name")
	}
}
