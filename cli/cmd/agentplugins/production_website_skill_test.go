package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// The production composition must expose HTTPS Skill acquisition. Trust is
// granted through the default transport, which the acquirer uses when no
// client is injected, so this exercises the real wrapper and no test double.
func TestProductionSourceAcquirerAcquiresHTTPSkills(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/agent-skills/index.json":
			_, _ = fmt.Fprint(w, `{"skills":[{"name":"demo","files":["SKILL.md"]}]}`)
		case "/.well-known/agent-skills/demo/SKILL.md":
			_, _ = fmt.Fprint(w, "---\nname: demo\ndescription: Test skill\n---\nInert content.\n")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	previous := http.DefaultTransport
	http.DefaultTransport = server.Client().Transport
	t.Cleanup(func() { http.DefaultTransport = previous })

	home := t.TempDir()
	app, err := composeAgentpluginsApp(home, filepath.Join(home, "data"))
	if err != nil {
		t.Fatal(err)
	}
	acquirer, ok := app.SourceAcquirer.(interface {
		ResolveHTTPSkill(context.Context, string, string) (domain.PackageSnapshot, domain.WebSkillSourceFacts, error)
		AcquireHTTPSkillVerified(context.Context, domain.WebSkillSourceFacts) (domain.PackageSnapshot, error)
	})
	if !ok {
		t.Fatal("production source acquirer cannot acquire HTTPS Skills")
	}
	snapshot, facts, err := acquirer.ResolveHTTPSkill(context.Background(), server.URL, "demo")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = packagedigest.Remove(snapshot) }()
	restored, err := acquirer.AcquireHTTPSkillVerified(context.Background(), facts)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = packagedigest.Remove(restored) }()
	if restored.TreeDigest != snapshot.TreeDigest {
		t.Fatal("locked restore changed the raw tree digest")
	}
}
