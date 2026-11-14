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
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
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
	server     *httptest.Server
	endpoint   string
	repo       string
	remote     string
	mu         sync.Mutex
	archive    []byte
	diagnostic *projectGitDiagnostic
	ctx        context.Context
	tempRoot   string
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

// Resolve existing fixture roots before composition and exact receipt assertions.
// Native observations and production path validation remain unchanged.
func projectJourneyTemp(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

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
	return newProjectJourneyTransportIn(context.Background(), t, kind, "")
}

func newProjectJourneyTransportIn(ctx context.Context, t *testing.T, kind string, tempRoot string) *projectJourneyTransport {
	t.Helper()
	transport := &projectJourneyTransport{tempRoot: tempRoot}
	if tempRoot != "" {
		transport.ctx = ctx
	}
	gitHome := transport.temporary(t)
	for _, name := range []string{"HOME", "USERPROFILE", "XDG_CONFIG_HOME"} {
		t.Setenv(name, gitHome)
	}
	projectJourneyWrite(t, filepath.Join(gitHome, ".gitconfig"), "[credential]\n helper =\n[http]\n schannelUseSSLCAInfo = true\n")
	transport.repo = transport.temporary(t)
	projectJourneyWrite(t, filepath.Join(transport.repo, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"project-demo","version":"1.0.0"}`)
	projectJourneyWrite(t, filepath.Join(transport.repo, "skills", "docs", "SKILL.md"), projectJourneySkill)
	projectJourneyWrite(t, filepath.Join(transport.repo, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"stdio","command":"go","args":["version"],"env":{"DATA":"${PLUGIN_DATA}"}}}}`)
	transport.git(t, transport.repo, "init", "-b", "main")
	transport.git(t, transport.repo, "add", ".")
	transport.git(t, transport.repo, "commit", "-m", "disposable project source")
	remoteRoot := transport.temporary(t)
	transport.remote = filepath.Join(remoteRoot, "repo.git")
	transport.git(t, transport.repo, "clone", "--quiet", "--bare", transport.repo, transport.remote)
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	backend := &cgi.Handler{Path: git, Args: []string{"http-backend"}, Env: []string{"GIT_PROJECT_ROOT=" + remoteRoot, "GIT_HTTP_EXPORT_ALL=1"}}
	transport.archive = projectJourneyArchive(t, transport.repo)
	transport.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ctx != nil && ctx.Err() != nil {
			http.Error(w, "fixture canceled", http.StatusServiceUnavailable)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/repo.git/") {
			backend.ServeHTTP(w, r)
			return
		}
		transport.mu.Lock()
		defer transport.mu.Unlock()
		_, _ = w.Write(transport.archive)
	}))
	t.Cleanup(transport.server.Close)
	ca := filepath.Join(transport.temporary(t), "ca.pem")
	projectJourneyWrite(t, ca, string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: transport.server.Certificate().Raw})))
	t.Setenv("GIT_SSL_CAINFO", ca)
	transport.endpoint = transport.server.URL + "/package.zip"
	if kind == "git" {
		transport.endpoint = transport.server.URL + "/repo.git"
	}
	return transport
}

// An explicit caller root belongs to the caller until it accepts the completed
// native evidence. Never register automatic deletion of these directories.
func (transport *projectJourneyTransport) temporary(t *testing.T) string {
	t.Helper()
	if transport.tempRoot == "" {
		return projectJourneyTemp(t)
	}
	root, err := os.MkdirTemp(transport.tempRoot, "journey-")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func (transport *projectJourneyTransport) git(t *testing.T, root string, args ...string) {
	t.Helper()
	if transport.ctx == nil {
		projectJourneyGit(t, root, args...)
		return
	}
	ctx, cancel := context.WithTimeout(transport.ctx, 15*time.Second)
	defer cancel()
	argv := append([]string{"git", "-c", "commit.gpgsign=false", "-c", "core.autocrlf=false"}, args...)
	env := append(os.Environ(), "GIT_AUTHOR_NAME=UAP fixture", "GIT_AUTHOR_EMAIL=fixture@example.test", "GIT_COMMITTER_NAME=UAP fixture", "GIT_COMMITTER_EMAIL=fixture@example.test")
	result, err := (processadapter.OS{}).RunWithTreeExitGrace(ctx, legacyports.Command{Argv: argv, Dir: root, Env: env, StdoutLimitBytes: 1 << 20}, time.Second)
	if !result.CleanupComplete && !result.NoChildStarted {
		t.Fatal("fixture Git cleanup is unconfirmed; caller retains its directories")
	}
	if err != nil || result.ExitCode != 0 {
		t.Fatalf("fixture Git failed after settlement: %v (exit %d)", err, result.ExitCode)
	}
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
	if transport.diagnostic == nil {
		transport.diagnostic = &projectGitDiagnostic{counts: map[string]uint16{}, endpoint: transport.server.URL + "/repo.git", log: func(record string) { t.Log(record) }}
	}
	journey := projectJourney{home: transport.temporary(t), a: transport.temporary(t), b: transport.temporary(t), transport: transport}
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
	runner := sourceacquisition.OSRunner{ReportCleanupEvidence: transport.ctx != nil}
	app.SourceAcquirer = sourceacquisition.Acquirer{TempRoot: home, HTTPClient: transport.server.Client(), Runner: projectGitDiagnosticRunner{recorder: transport.diagnostic, root: home, delegate: runner}}
	app.SourceAcquirerForDryRun = func(root string) agentpluginscli.SourceAcquirer {
		return sourceacquisition.Acquirer{TempRoot: root, HTTPClient: transport.server.Client(), Runner: projectGitDiagnosticRunner{recorder: transport.diagnostic, root: root, delegate: runner}}
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
