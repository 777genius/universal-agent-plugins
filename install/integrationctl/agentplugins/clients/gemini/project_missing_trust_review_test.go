package gemini

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type geminiUntrustedReviewProbe struct{ calls int }

func (probe *geminiUntrustedReviewProbe) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	probe.calls++
	return legacyports.CommandResult{CleanupComplete: true, Stdout: []byte("Discovered Agent Skills:\n\n")}, nil
}

func TestGeminiMissingTrustReviewOffersOnlyOwnedLocalCompletion(t *testing.T) {
	for _, mode := range []string{"missing", "distrust", "unknown-trust", "disabled", "drift", "remote"} {
		t.Run(mode, func(t *testing.T) {
			request, skill := geminiIntactHealthFixture(t)
			if mode == "remote" {
				request, _ = geminiMCPReviewFixture(t, nativeconfig.Server{Type: "remote", RemoteTransport: "streamable-http", URL: "https://example.test/mcp"})
			}
			request.VerifyOnly = true
			profile := filepath.Join(request.Client.ConfigRoot, "trustedFolders.json")
			mapping := map[string]string{}
			if mode == "distrust" {
				mapping[request.Plan.TargetContext.ProjectRoot] = "DO_NOT_TRUST"
			}
			if mode == "unknown-trust" {
				mapping[request.Plan.TargetContext.ProjectRoot] = "UNKNOWN"
			}
			body, err := json.Marshal(mapping)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(profile, body, 0600); err != nil {
				t.Fatal(err)
			}
			settings := filepath.Join(request.Plan.NativeRegistryRoot, "settings.json")
			if mode == "disabled" {
				body, err := os.ReadFile(settings)
				if err != nil {
					t.Fatal(err)
				}
				body = append([]byte(`{"mcp":{"excluded":["audit"]},`), body[1:]...)
				if err := os.WriteFile(settings, body, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "drift" {
				if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("foreign"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			files := &geminiIntactFiles{}
			probe := &geminiUntrustedReviewProbe{}
			env := clients.Env{NativeConfig: nativeconfig.NewWithFileIO(files), Runner: probe}
			out, err := New().Activate(t.Context(), env, request)
			offered := slices.Contains(out.UserActions, geminiMCPConfirmationAction)
			if mode == "missing" {
				if err != nil || !offered || !New().CanCompletePreparedSetup(request, out) {
					t.Fatalf("missing trust cannot reach confirmed completion: %+v %v", out, err)
				}
			} else if offered || New().CanCompletePreparedSetup(request, out) || out.Activation == domain.ActivationActive {
				t.Fatalf("unsafe trust completion offered for %s: %+v %v", mode, out, err)
			}
			if files.writes != 0 || probe.calls != 0 {
				t.Fatalf("review acted: writes=%d calls=%d", files.writes, probe.calls)
			}
			after, readErr := os.ReadFile(profile)
			if readErr != nil || string(after) != string(body) {
				t.Fatal("review changed trust baseline", readErr)
			}
		})
	}
}
