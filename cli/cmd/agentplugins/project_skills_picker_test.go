package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type projectSkillsPickerDetector struct{ clients []domain.DetectedClient }

func (d projectSkillsPickerDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	return append([]domain.DetectedClient(nil), d.clients...), nil
}

// Start the actual public picker bridge from production project composition.
// Reading target choices must show selected project roots and remain read-only.
func TestProjectSkillsPickerReportsScopedDestinations(t *testing.T) {
	source := newProjectSkillsSource(t, "git")
	journey := newProjectJourney(t, source.transport)
	journey.app = projectSkillsGitAcquirer(journey.app, source.transport.remote)
	for _, unsafe := range []bool{false, true} {
		t.Run(map[bool]string{false: "available", true: "unsafe-user-profile"}[unsafe], func(t *testing.T) {
			app, err := journey.app.ForScope(domain.ScopeProject, journey.a)
			if err != nil {
				t.Fatal(err)
			}
			detected := make([]domain.DetectedClient, 0, 4)
			for _, id := range []domain.ClientID{domain.ClientClaude, domain.ClientCodex, domain.ClientGemini, domain.ClientChatGPT} {
				root := filepath.Join(journey.home, "."+string(id))
				if unsafe && id == domain.ClientCodex {
					root = "relative-profile"
				}
				detected = append(detected, domain.DetectedClient{ClientID: id, ConfigRoot: root, Status: domain.DetectionDetected})
			}
			app.Detector = projectSkillsPickerDetector{clients: detected}
			before := securityFixtureTree(t, journey.a)
			server, err := app.StartSkillsPicker(context.Background(), agentpluginscli.SkillsPickerRequest{Input: source.input})
			if err != nil {
				t.Fatal(err)
			}
			defer server.Close()
			endpoint, err := url.Parse(server.URL())
			if err != nil {
				t.Fatal(err)
			}
			fragment, err := url.ParseQuery(endpoint.Fragment)
			if err != nil {
				t.Fatal(err)
			}
			endpoint.Fragment = ""
			origin := strings.TrimSuffix(endpoint.String(), "/")
			request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, origin+"/api/v1/targets", http.NoBody)
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Authorization", "Bearer "+fragment.Get("capability"))
			request.Header.Set("Origin", origin)
			response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := response.Body.Close(); err != nil {
					t.Error(err)
				}
			}()
			if response.StatusCode != http.StatusOK {
				t.Fatalf("target API status: %d", response.StatusCode)
			}
			var body skillspicker.Targets
			if err = json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			targets := map[string]skillspicker.Target{}
			for _, target := range body.Targets {
				if target.Scope != "project" {
					t.Fatalf("picker exposed wrong scope: %+v", target)
				}
				targets[target.Id] = target
			}
			for _, id := range []string{"claude", "codex", "gemini"} {
				target, ok := targets[id]
				if !ok {
					t.Fatalf("project target missing: %s", id)
				}
				if unsafe && id == "codex" {
					if target.Available || target.DisplayPath != "" || target.Reason == nil {
						t.Fatalf("unsafe profile gained fallback destination: %+v", target)
					}
					continue
				}
				if !target.Available || !strings.HasPrefix(target.DisplayPath, journey.a+string(filepath.Separator)) || strings.Contains(target.DisplayPath, journey.home) {
					t.Fatalf("picker substituted user path: %+v", target)
				}
			}
			refused, ok := targets["chatgpt"]
			if !ok || refused.Available || refused.Reason == nil || refused.DisplayPath != "" {
				t.Fatalf("unsupported project target not disabled: %+v", refused)
			}
			if !reflect.DeepEqual(before, securityFixtureTree(t, journey.a)) {
				t.Fatal("reading picker targets changed project files")
			}
			projectJourneyAbsent(t, filepath.Join(journey.cache, "state-v2.json"), filepath.Join(journey.a, ".uap"), filepath.Join(journey.b, ".uap"))
		})
	}
}
