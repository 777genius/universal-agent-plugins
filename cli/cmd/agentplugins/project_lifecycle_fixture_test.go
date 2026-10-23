package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/pem"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

type projectJourneyDetector struct{ root string }

func (detector projectJourneyDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	return []domain.DetectedClient{{ClientID: domain.ClientGemini, ConfigRoot: detector.root, Status: domain.DetectionDetected}}, nil
}

type projectJourneySecurity struct{}

func (projectJourneySecurity) Evaluate(context.Context, domain.SecurityEvaluationInput) (domain.SecurityAssessment, error) {
	return domain.SecurityAssessment{}, nil
}

type projectJourneyTransport struct {
	server   *httptest.Server
	endpoint string
	repo     string
	remote   string
	mu       sync.Mutex
	archive  []byte
}

type projectJourney struct {
	app       agentpluginscli.App
	home      string
	cache     string
	a, b      string
	transport *projectJourneyTransport
}

const projectJourneySkill = "---\nname: docs\ndescription: Inert project documentation\n---\nUAP-PROJECT-SOURCE-FIXTURE\n"
const projectJourneyAdvancedSkill = "---\nname: docs\ndescription: Inert project documentation\n---\nUAP-PROJECT-UPDATED-FIXTURE\n"

func projectJourneyWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func projectJourneyGit(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.CommandContext(context.Background(), "git", append([]string{"-c", "commit.gpgsign=false", "-c", "core.autocrlf=false"}, args...)...)
	command.Dir = root
	command.Env = append(os.Environ(), "GIT_AUTHOR_NAME=UAP fixture", "GIT_AUTHOR_EMAIL=fixture@example.test", "GIT_COMMITTER_NAME=UAP fixture", "GIT_COMMITTER_EMAIL=fixture@example.test")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

func newProjectJourneyTransport(t *testing.T, kind string) *projectJourneyTransport {
	t.Helper()
	gitHome := t.TempDir()
	for _, name := range []string{"HOME", "USERPROFILE", "XDG_CONFIG_HOME"} {
		t.Setenv(name, gitHome)
	}
	projectJourneyWrite(t, filepath.Join(gitHome, ".gitconfig"), "[credential]\n helper =\n[http]\n schannelUseSSLCAInfo = true\n")
	transport := &projectJourneyTransport{repo: t.TempDir()}
	projectJourneyWrite(t, filepath.Join(transport.repo, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"project-demo","version":"1.0.0"}`)
	projectJourneyWrite(t, filepath.Join(transport.repo, "skills", "docs", "SKILL.md"), projectJourneySkill)
	projectJourneyWrite(t, filepath.Join(transport.repo, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"stdio","command":"go","args":["version"],"env":{"DATA":"${PLUGIN_DATA}"}}}}`)
	projectJourneyGit(t, transport.repo, "init", "-b", "main")
	projectJourneyGit(t, transport.repo, "add", ".")
	projectJourneyGit(t, transport.repo, "commit", "-m", "disposable project source")
	remoteRoot := t.TempDir()
	transport.remote = filepath.Join(remoteRoot, "repo.git")
	projectJourneyGit(t, transport.repo, "clone", "--quiet", "--bare", transport.repo, transport.remote)
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	backend := &cgi.Handler{Path: git, Args: []string{"http-backend"}, Env: []string{"GIT_PROJECT_ROOT=" + remoteRoot, "GIT_HTTP_EXPORT_ALL=1"}}
	transport.archive = projectJourneyArchive(t, transport.repo)
	transport.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/repo.git/") {
			backend.ServeHTTP(w, r)
			return
		}
		transport.mu.Lock()
		defer transport.mu.Unlock()
		_, _ = w.Write(transport.archive)
	}))
	t.Cleanup(transport.server.Close)
	ca := filepath.Join(t.TempDir(), "ca.pem")
	projectJourneyWrite(t, ca, string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: transport.server.Certificate().Raw})))
	t.Setenv("GIT_SSL_CAINFO", ca)
	transport.endpoint = transport.server.URL + "/package.zip"
	if kind == "git" {
		transport.endpoint = transport.server.URL + "/repo.git"
	}
	return transport
}

func projectJourneyArchive(t *testing.T, root string) []byte {
	t.Helper()
	var body bytes.Buffer
	writer := zip.NewWriter(&body)
	for _, name := range []string{"plugin.json", "mcp.json", "skills/docs/SKILL.md"} {
		file, err := writer.Create("provider-wrapper/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(projectJourneyRead(t, filepath.Join(root, filepath.FromSlash(name))))); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}

func newProjectJourney(t *testing.T, transport *projectJourneyTransport) projectJourney {
	t.Helper()
	journey := projectJourney{home: t.TempDir(), a: t.TempDir(), b: t.TempDir(), transport: transport}
	journey.cache = filepath.Join(journey.home, "uap")
	for _, native := range []string{filepath.Join(journey.home, ".gemini"), filepath.Join(journey.a, ".gemini"), filepath.Join(journey.b, ".gemini")} {
		projectJourneyWrite(t, filepath.Join(native, "settings.json"), "{\n// retained foreign settings\n\"mcpServers\":{\"foreign\":{\"url\":\"https://foreign.test/mcp\"}},\"theme\":\"foreign\"\n}\n")
		projectJourneyWrite(t, filepath.Join(native, "skills", "foreign", "SKILL.md"), "foreign skill\n")
	}
	registry, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	app, err := newAgentpluginsCLIApp(journey.home, journey.cache, nil, nil, nil, registry)
	if err != nil {
		t.Fatal(err)
	}
	journey.app = configureProjectJourney(app, journey.home, transport)
	return journey
}

// Override only host boundaries. Reapply them after the production ForScope
// composer rebuilds its runtime; keep installer, native kernel and store intact.
func configureProjectJourney(app agentpluginscli.App, home string, transport *projectJourneyTransport) agentpluginscli.App {
	app.Detector = projectJourneyDetector{root: filepath.Join(home, ".gemini")}
	app.SecurityIndex, app.DirectoryClient, app.DiscoveryClient = nil, nil, nil
	app.SecurityEvaluator = projectJourneySecurity{}
	app.SecurityEvaluatorForDryRun = func(string) ports.PackageSecurityEvaluator { return projectJourneySecurity{} }
	app.SourceAcquirer = sourceacquisition.Acquirer{TempRoot: home, HTTPClient: transport.server.Client()}
	app.SourceAcquirerForDryRun = func(root string) agentpluginscli.SourceAcquirer {
		return sourceacquisition.Acquirer{TempRoot: root, HTTPClient: transport.server.Client()}
	}
	app.DirectoryClientForDryRun, app.CommitDirectoryPreview = nil, nil
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return configureProjectJourney(selected, home, transport), nil
	}
	return app
}

func projectJourneyRead(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
