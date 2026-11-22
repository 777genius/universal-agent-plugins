package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

type dryRunClaudeDetector struct{ root, executable string }

func (detector dryRunClaudeDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	return []domain.DetectedClient{{ClientID: domain.ClientClaude, ConfigRoot: detector.root, ExecutablePath: detector.executable, Status: domain.DetectionDetected}}, nil
}

func configureDryRunClaude(app agentpluginscli.App, detector dryRunClaudeDetector) agentpluginscli.App {
	app.Detector = detector
	app.SecurityIndex, app.DirectoryClient, app.DiscoveryClient = nil, nil, nil
	app.SecurityEvaluator = projectJourneySecurity{}
	app.SecurityEvaluatorForDryRun = func(string) ports.PackageSecurityEvaluator { return projectJourneySecurity{} }
	app.DirectoryClientForDryRun, app.CommitDirectoryPreview = nil, nil
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return configureDryRunClaude(selected, detector), nil
	}
	return app
}

// A project dry-run previews through the local installer facade. Its package
// snapshot belongs in disposable storage: the selected project, including a
// read-only checkout, must not gain a .uap namespace from a preview.
func TestProjectAddDryRunDoesNotCreateProjectNamespace(t *testing.T) {
	for _, readOnly := range []bool{false, true} {
		name := "writable project"
		if readOnly {
			name = "read-only project"
		}
		t.Run(name, func(t *testing.T) {
			if readOnly && (runtime.GOOS == "windows" || os.Geteuid() == 0) {
				t.Skip("directory write permission bits are not enforced for this user")
			}
			home, project, source := projectJourneyTemp(t), projectJourneyTemp(t), projectJourneyTemp(t)
			projectJourneyWrite(t, filepath.Join(source, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"dry-run-preview","version":"1.0.0"}`)
			projectJourneyWrite(t, filepath.Join(source, "skills", "docs", "SKILL.md"), projectJourneySkill)
			// The preview must not start the client; this executable only refuses.
			client := filepath.Join(home, "bin", "claude")
			projectJourneyWrite(t, client, "#!/bin/sh\nexit 97\n")
			if err := os.Chmod(client, 0o700); err != nil {
				t.Fatal(err)
			}
			registry, err := specregistry.New()
			if err != nil {
				t.Fatal(err)
			}
			app, err := newAgentpluginsCLIApp(home, filepath.Join(home, "uap"), nil, nil, nil, registry)
			if err != nil {
				t.Fatal(err)
			}
			journey := projectJourney{app: configureDryRunClaude(app, dryRunClaudeDetector{root: filepath.Join(home, ".claude"), executable: client}), home: home}
			if readOnly {
				if err := os.Chmod(project, 0o500); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(project, 0o700) })
			}
			before := securityFixtureTree(t, project)
			output, err := journey.execute(project, "add", source, "--no-save", "--target", "claude", "--dry-run")
			if err != nil {
				t.Fatalf("project dry-run failed: %v\n%s", err, output)
			}
			var envelope struct {
				Result string `json:"result"`
				Data   struct {
					DryRun bool `json:"dry_run"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(lastJSONDocument(t, output)), &envelope); err != nil || envelope.Result != "success" || !envelope.Data.DryRun {
				t.Fatalf("unexpected dry-run envelope (%v):\n%s", err, output)
			}
			projectJourneyAbsent(t, filepath.Join(project, ".uap"))
			if after := securityFixtureTree(t, project); !reflect.DeepEqual(before, after) {
				t.Fatalf("dry-run changed the project tree: before=%v after=%v", before, after)
			}
		})
	}
}

func lastJSONDocument(t *testing.T, output string) string {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(output))
	var last json.RawMessage
	for decoder.More() {
		var document json.RawMessage
		if err := decoder.Decode(&document); err != nil {
			break
		}
		last = document
	}
	if len(last) == 0 {
		t.Fatalf("no JSON document in output:\n%s", output)
	}
	return string(last)
}
