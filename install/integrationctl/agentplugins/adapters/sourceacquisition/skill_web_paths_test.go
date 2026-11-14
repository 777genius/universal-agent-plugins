package sourceacquisition

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestWebSkillRejectsUnicodeTreeCollisionsBeforeFileRequests(t *testing.T) {
	for name, paths := range map[string][]string{
		"files":              {"SKILL.md", "refs/σ.txt", "refs/ς.txt"},
		"file parent":        {"SKILL.md", "refs/σ", "refs/ς/nested.txt"},
		"directory spelling": {"SKILL.md", "refs/σ/a.txt", "refs/ς/b.txt"},
		"non NFC":            {"SKILL.md", "refs/e\u0301.txt"},
	} {
		t.Run(name, func(t *testing.T) {
			files := 0
			encoded, err := json.Marshal(paths)
			if err != nil {
				t.Fatal(err)
			}
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "index.json") {
					_, _ = fmt.Fprintf(w, `{"skills":[{"name":"pdf","files":%s}]}`, encoded)
					return
				}
				files++
				http.Error(w, "unexpected file request", http.StatusInternalServerError)
			}))
			defer server.Close()
			acquirer := Acquirer{TempRoot: t.TempDir(), HTTPClient: server.Client()}
			if _, _, err := acquirer.ResolveHTTPSkill(context.Background(), server.URL, "pdf"); err == nil {
				t.Fatal("ambiguous Unicode discovery tree accepted")
			}
			facts := webSkillPathTestFacts(t, server.URL, paths)
			if _, err := acquirer.AcquireHTTPSkillVerified(context.Background(), facts); err == nil {
				t.Fatal("ambiguous Unicode pinned tree accepted")
			}
			if files != 0 {
				t.Fatalf("invalid Unicode tree caused %d file requests", files)
			}
		})
	}
}

func webSkillPathTestFacts(t *testing.T, endpoint string, paths []string) domain.WebSkillSourceFacts {
	t.Helper()
	facts := domain.WebSkillSourceFacts{URL: endpoint, Skill: "pdf", IndexURL: endpoint + "/.well-known/agent-skills/index.json", IndexSHA256: strings.Repeat("a", 64), RawTreeDigest: "sha256:" + strings.Repeat("b", 64), Normalizer: "agent-skill-package/v1"}
	coordinate, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	for _, filePath := range paths {
		coordinate.Path = "/.well-known/agent-skills/pdf/" + filePath
		facts.Files = append(facts.Files, domain.WebSkillFile{Path: filePath, URL: coordinate.String(), SHA256: strings.Repeat("c", 64)})
	}
	return facts
}
