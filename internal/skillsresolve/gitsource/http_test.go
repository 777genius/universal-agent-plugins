package gitsource

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/internal/skillscatalog"
	"github.com/777genius/plugin-kit-ai/internal/skillsresolve"
)

func TestResolveHTTPRealGit(t *testing.T) {
	root, source, scratch := fixture(t)
	put(t, root, "first/review/SKILL.md", document("review"))
	put(t, root, "second/review/SKILL.md", document("review"))
	sha := commit(t, root)
	const token = "real-http-git-fixture-capability-32-bytes"
	server := httptest.NewServer(skillscatalog.NewResolveHandler(skillsresolve.New(source), token))
	defer server.Close()
	client := server.Client()
	client.Timeout = 30 * time.Second
	discovery := callResolve(t, client, server.URL, token, skillsresolve.Request{Repository: "example/fixture", SkillNameHint: pointer("review")})
	if discovery.Status != "selection_required" || discovery.Commit != sha || len(discovery.Candidates) != 2 {
		t.Fatal("HTTP discovery was not the actual repository", discovery)
	}
	put(t, root, "first/review/SKILL.md", document("review")+"New default-branch text.")
	commit(t, root)
	selected := callResolve(t, client, server.URL, token, skillsresolve.Request{Repository: discovery.Repository, Commit: &discovery.Commit, Path: &discovery.Candidates[0].Path})
	if selected.Status != "resolved" || selected.Skill.RawContent != document("review") || selected.Commit != sha {
		t.Fatal("HTTP selection did not retain the immutable preview", selected)
	}
	assertClean(t, scratch)
}

func callResolve(t *testing.T, client *http.Client, origin, token string, body skillsresolve.Request) skillsresolve.Result {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, origin+"/v1/skills/resolve", bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	payload, err := io.ReadAll(io.LimitReader(response.Body, skillsresolve.MaxResponseBytes+1))
	if err != nil || response.StatusCode != 200 || len(payload) > skillsresolve.MaxResponseBytes {
		t.Fatalf("HTTP resolver failed: %d %v %s", response.StatusCode, err, payload)
	}
	var result skillsresolve.Result
	if err = json.Unmarshal(payload, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
