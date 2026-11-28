package agentpluginscli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
)

type allWebsitePublisher struct {
	mu                    sync.Mutex
	sibling, stale        bool
	indexes, siblingReads int
}

func (p *allWebsitePublisher) serve(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if r.URL.Path == "/.well-known/agent-skills/index.json" {
		p.indexes++
		names := []string{"review", "testing"}
		if p.sibling {
			names = append(names, "later")
		}
		skills := make([]map[string]any, 0, len(names))
		for _, name := range names {
			description := "Read harmless documentation"
			if p.stale && p.indexes > 1 {
				description = "Publisher changed after discovery"
			}
			skills = append(skills, map[string]any{"name": name, "description": description, "files": []string{"SKILL.md"}})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"skills": skills})
		return
	}
	for _, name := range []string{"review", "testing", "later"} {
		if r.URL.Path == "/.well-known/agent-skills/"+name+"/SKILL.md" {
			if name == "later" {
				p.siblingReads++
			}
			_, _ = fmt.Fprint(w, allWebsiteDocument(name))
			return
		}
	}
	http.NotFound(w, r)
}

func allWebsiteDocument(name string) string {
	return "---\nname: " + name + "\ndescription: Read harmless documentation\n---\nPublisher marker: " + name + ".\n"
}

func allWebsiteJourney(t *testing.T, p *allWebsitePublisher) (savedJourney, string) {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(p.serve))
	t.Cleanup(server.Close)
	journey := newSavedJourney(t, t.TempDir())
	journey.cli.app.SourceAcquirer = sourceacquisition.Acquirer{TempRoot: t.TempDir(), HTTPClient: server.Client()}
	journey.cli.app.SourceAcquirerForDryRun = nil
	return journey, server.URL
}

// Real TLS source acquisition and ordinary command composition; native client
// responses and security assessments remain the existing explicit fixtures.
func TestSkillsAllWebsiteComposedLifecycle(t *testing.T) {
	publisher := &allWebsitePublisher{}
	journey, endpoint := allWebsiteJourney(t, publisher)
	args := []string{"skills", "add", endpoint, "--all", "--target", "codex,claude", "--yes", "--no-ui", "--format", "json"}
	journey.run(t, args...)
	requirements, locked := journey.intent(t)
	if len(requirements.Dependencies) != 2 || len(locked.Packages) != 2 || len(requirements.Targets) != 2 {
		t.Fatalf("unexpected agreement: %+v %+v", requirements, locked)
	}
	state := journey.state(t)
	if len(state.EnvironmentReceipts) != 4 {
		t.Fatalf("missing two-agent bindings: %+v", state.EnvironmentReceipts)
	}
	for _, installation := range state.Installations {
		for _, binding := range installation.Clients {
			skillRoot := binding.TargetLocator
			if binding.ClientID == "claude" {
				skillRoot = filepath.Join(skillRoot, ".agentplugins-runtime")
			}
			documents, err := filepath.Glob(filepath.Join(skillRoot, "skills", "*", "SKILL.md"))
			if err != nil || len(documents) != 1 {
				t.Fatalf("unexpected projected documents: %v %v", documents, err)
			}
			name := filepath.Base(filepath.Dir(documents[0]))
			if name != "review" && name != "testing" {
				t.Fatalf("unexpected projected Skill %s", name)
			}
			if got := dependencyRead(t, documents[0]); got != allWebsiteDocument(name) {
				t.Fatalf("projected bytes changed: %q", got)
			}
		}
	}

	journey.run(t, args...)
	repeatedRequirements, repeatedLock := journey.intent(t)
	if !reflect.DeepEqual(requirements, repeatedRequirements) || !reflect.DeepEqual(locked, repeatedLock) {
		t.Fatal("repeated --all changed saved pins")
	}
	publisher.mu.Lock()
	publisher.sibling = true
	publisher.mu.Unlock()
	journey.run(t, "update", "--all", "--format", "json")
	updatedRequirements, updatedLock := journey.intent(t)
	if len(updatedRequirements.Dependencies) != 2 || len(updatedLock.Packages) != 2 {
		t.Fatalf("saved update enrolled sibling: %+v %+v", updatedRequirements, updatedLock)
	}
	for _, pin := range updatedLock.Packages {
		if pin.Source.Web == nil || pin.Source.Web.Skill == "later" {
			t.Fatalf("unexpected updated pin: %+v", pin)
		}
	}
	publisher.mu.Lock()
	reads := publisher.siblingReads
	publisher.mu.Unlock()
	if reads != 0 {
		t.Fatal("saved update downloaded unselected sibling")
	}
}

func TestSkillsAllWebsiteChangedIndexRefusesBeforeEffects(t *testing.T) {
	publisher := &allWebsitePublisher{stale: true}
	journey, endpoint := allWebsiteJourney(t, publisher)
	_, _, err := journey.cli.execute(false, "skills", "add", endpoint, "--all", "--target", "codex,claude", "--yes", "--no-ui", "--format", "json")
	if !errors.Is(err, ErrSkillsPlanStale) {
		t.Fatalf("changed discovery index accepted: %v", err)
	}
	manifest, lock := journey.cli.app.dependencyPaths()
	for _, name := range []string{manifest, lock} {
		if _, err := os.Stat(name); !os.IsNotExist(err) {
			t.Fatalf("refusal published %s: %v", name, err)
		}
	}
	if journey.runner.calls != 0 || len(journey.state(t).EnvironmentReceipts) != 0 {
		t.Fatal("stale catalog changed native profiles or ownership")
	}
	publisher.mu.Lock()
	indexes := publisher.indexes
	publisher.mu.Unlock()
	if indexes < 2 {
		t.Fatal("fixture did not observe discovery and acquisition")
	}
}
