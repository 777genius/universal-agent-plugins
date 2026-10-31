//go:build uappm_e2e

package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// These journeys preserve the production composer, CLI, HTTPS acquisition,
// immutable store, installer, native configuration kernel and transactions.
// Detection and security assessment are explicit inert fixtures. No agent,
// configured MCP command, model, or production scanner is executed.
type pmInstallOrigin struct {
	server   *httptest.Server
	mu       sync.Mutex
	payload  []byte
	requests int
}

func newPMInstallOrigin(t *testing.T) *pmInstallOrigin {
	t.Helper()
	o := &pmInstallOrigin{}
	o.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/rich.zip" {
			http.NotFound(w, r)
			return
		}
		o.mu.Lock()
		o.requests++
		body := append([]byte(nil), o.payload...)
		o.mu.Unlock()
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(body)
	}))
	t.Cleanup(o.server.Close)
	o.set(t, pmInstallFiles("v1", false))
	return o
}

func (o *pmInstallOrigin) url() string { return o.server.URL + "/rich.zip" }
func (o *pmInstallOrigin) count() int  { o.mu.Lock(); defer o.mu.Unlock(); return o.requests }
func (o *pmInstallOrigin) digest() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	digest := sha256.Sum256(o.payload)
	return hex.EncodeToString(digest[:])
}
func (o *pmInstallOrigin) set(t *testing.T, files map[string]string) {
	t.Helper()
	var body bytes.Buffer
	writer := zip.NewWriter(&body)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		member, err := writer.Create("provider-wrapper/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = member.Write([]byte(files[name])); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.payload = body.Bytes()
}

func pmInstallSkill(name, revision string) string {
	return fmt.Sprintf("---\nname: %s\ndescription: Inert installation acceptance skill\n---\nIndependent %s instructions at %s.\n", name, name, revision)
}

func pmInstallFiles(revision string, replacement bool) map[string]string {
	second, remote := "audit", "docs"
	if replacement {
		second, remote = "release", "release"
	}
	return map[string]string{
		"plugin.json":                    `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"install-rich","version":"1.0.0","description":"Inert rich installation fixture"}`,
		"skills/docs/SKILL.md":           pmInstallSkill("docs", revision),
		"skills/" + second + "/SKILL.md": pmInstallSkill(second, revision),
		"mcp.json":                       fmt.Sprintf(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{%q:{"type":"streamable-http","url":%q,"headers":{"X-Fixture":%q}},"worker":{"type":"stdio","command":"go","args":["version"],"env":{"DATA":"${PLUGIN_DATA}","FIXTURE":%q}}}}`, remote, "https://managed.test/"+revision, revision, revision),
		"marker.txt":                     "independent package payload " + revision + "\n",
	}
}

type pmInstallDetector struct{ clients []domain.DetectedClient }

func (d pmInstallDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	return append([]domain.DetectedClient(nil), d.clients...), nil
}

type pmInstallSecurity struct{ calls int }

func (s *pmInstallSecurity) Evaluate(_ context.Context, input domain.SecurityEvaluationInput) (domain.SecurityAssessment, error) {
	s.calls++
	if input.TreeDigest == "" || input.ManifestDigest == "" {
		return domain.SecurityAssessment{}, fmt.Errorf("fixture received no package identity")
	}
	return domain.SecurityAssessment{}, nil
}

type pmInstallJourney struct {
	app               agentpluginscli.App
	home, cache, a, b string
	origin            *pmInstallOrigin
	security          *pmInstallSecurity
}

func newPMInstallJourney(t *testing.T, origin *pmInstallOrigin) pmInstallJourney {
	t.Helper()
	base := projectJourneyTemp(t)
	f := pmInstallJourney{home: filepath.Join(base, "home space Ω"), a: filepath.Join(base, "project A é"), b: filepath.Join(base, "project B é"), origin: origin, security: &pmInstallSecurity{}}
	f.cache = filepath.Join(f.home, "uap")
	for _, root := range []string{f.home, f.a, f.b} {
		if err := os.MkdirAll(root, 0700); err != nil {
			t.Fatal(err)
		}
		projectJourneyWrite(t, filepath.Join(root, ".gemini", "settings.json"), `{"mcpServers":{"foreign":{"httpUrl":"https://foreign.test/mcp"}},"foreignPreference":"keep"}`+"\n")
		projectJourneyWrite(t, filepath.Join(root, ".cursor", "mcp.json"), `{"mcpServers":{"foreign":{"url":"https://foreign.test/mcp","disabled":true}},"foreignPreference":"keep"}`+"\n")
		for _, target := range []string{".gemini", ".cursor"} {
			projectJourneyWrite(t, filepath.Join(root, target, "skills", "foreign", "SKILL.md"), "foreign skill must survive\n")
		}
	}
	registry, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	app, err := newAgentpluginsCLIApp(f.home, f.cache, nil, nil, nil, registry)
	if err != nil {
		t.Fatal(err)
	}
	f.app = f.configure(app)
	return f
}

func (f pmInstallJourney) configure(app agentpluginscli.App) agentpluginscli.App {
	app.Detector = pmInstallDetector{[]domain.DetectedClient{
		{ClientID: domain.ClientGemini, ConfigRoot: filepath.Join(f.home, ".gemini"), Status: domain.DetectionDetected},
		{ClientID: domain.ClientCursor, ConfigRoot: filepath.Join(f.home, ".cursor"), Status: domain.DetectionDetected},
	}}
	app.SecurityIndex, app.DirectoryClient, app.DiscoveryClient = nil, nil, nil
	app.SecurityEvaluator = f.security
	app.SecurityEvaluatorForDryRun = func(string) ports.PackageSecurityEvaluator { return f.security }
	app.SourceAcquirer = sourceacquisition.Acquirer{TempRoot: f.home, HTTPClient: f.origin.server.Client()}
	app.SourceAcquirerForDryRun = func(root string) agentpluginscli.SourceAcquirer {
		return sourceacquisition.Acquirer{TempRoot: root, HTTPClient: f.origin.server.Client()}
	}
	app.DirectoryClientForDryRun, app.CommitDirectoryPreview = nil, nil
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return f.configure(selected), nil
	}
	return app
}

func (f pmInstallJourney) execute(t *testing.T, project, input string, terminal bool, args ...string) (string, error) {
	t.Helper()
	var out, stderr bytes.Buffer
	app := f.app
	app.Input, app.Output, app.ErrorOutput, app.Terminal = strings.NewReader(input), &out, &stderr, terminal
	cmd := agentpluginscli.NewRoot(app)
	cmd.SetIn(app.Input)
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	if project != "" {
		args = append(args, "--scope", "project", "--project-root", project)
	}
	cmd.SetArgs(args)
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	err := cmd.ExecuteContext(ctx)
	return out.String() + stderr.String(), err
}
func (f pmInstallJourney) run(t *testing.T, project string, args ...string) string {
	t.Helper()
	args = append(args, "--format", "json")
	out, err := f.execute(t, project, "", false, args...)
	if err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out)
	}
	return out
}
func (f pmInstallJourney) namespace(project string) string {
	if project == "" {
		return f.cache
	}
	return filepath.Join(project, ".uap")
}
func (f pmInstallJourney) native(project, target string) string {
	if project == "" {
		project = f.home
	}
	return filepath.Join(project, "."+target)
}
func (f pmInstallJourney) state(t *testing.T, project string) domain.StateFileV2 {
	t.Helper()
	scope := domain.ScopeUser
	if project != "" {
		scope = domain.ScopeProject
	}
	app, err := f.app.ForScope(scope, project)
	if err != nil {
		t.Fatal(err)
	}
	state, err := app.StateStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	return state
}
func (f pmInstallJourney) binding(t *testing.T, project, target string) (domain.Installation, domain.ClientBinding) {
	t.Helper()
	state := f.state(t, project)
	for _, installation := range state.Installations {
		for _, binding := range installation.Clients {
			if binding.ClientID == target {
				return installation, binding
			}
		}
	}
	t.Fatalf("missing %s binding: %+v", target, state)
	return domain.Installation{}, domain.ClientBinding{}
}
func (f pmInstallJourney) pair(t *testing.T, project string) [2]string {
	t.Helper()
	root := project
	if root == "" {
		root = f.cache
	}
	pair := [2]string{projectJourneyRead(t, filepath.Join(root, "uap.toml")), projectJourneyRead(t, filepath.Join(root, "uap.lock"))}
	for _, body := range pair {
		for _, private := range []string{f.home, f.cache, f.a, f.b} {
			if strings.Contains(body, private) || strings.Contains(body, filepath.ToSlash(private)) || strings.Contains(body, strings.ReplaceAll(private, `\`, `\\`)) {
				t.Fatalf("agreement leaked machine path %q", private)
			}
		}
	}
	return pair
}
func (f pmInstallJourney) assertPin(t *testing.T, project string, targets ...string) environment.LockedPackage {
	t.Helper()
	var lock environment.Lock
	pair := f.pair(t, project)
	if err := json.Unmarshal([]byte(pair[1]), &lock); err != nil {
		t.Fatal(err)
	}
	if len(lock.Packages) != 1 {
		t.Fatalf("pins=%+v", lock.Packages)
	}
	pin := lock.Packages[0]
	if pin.Alias != "rich" || pin.Version != "1.0.0" || pin.Source.Archive == nil || pin.Source.Archive.URL != f.origin.url() || pin.Source.Archive.SHA256 != f.origin.digest() || pin.Source.Archive.Path != "" || !reflect.DeepEqual(pin.Targets, targets) {
		t.Fatalf("unexpected independent ZIP identity/targets: %+v want %v", pin, targets)
	}
	state := f.state(t, project)
	if len(state.Installations) != 1 || len(state.EnvironmentReceipts) != len(targets) {
		t.Fatalf("pin installation/ownership count: %+v", state)
	}
	installation := state.Installations[0]
	if installation.Source.TreeDigest != pin.TreeDigest || installation.Source.ResolvedRevision != "sha256:"+pin.Source.Archive.SHA256 || len(installation.Clients) != len(targets) {
		t.Fatalf("installed identity diverges from independent archive pin: %+v", installation)
	}
	for _, target := range targets {
		_, binding := f.binding(t, project, target)
		if binding.PackageRevision == nil || binding.PackageRevision.TreeDigest != pin.TreeDigest || binding.PackageRevision.ResolvedRevision != "sha256:"+pin.Source.Archive.SHA256 {
			t.Fatalf("binding diverges from independent archive pin: %+v", binding)
		}
		found := false
		for _, receipt := range state.EnvironmentReceipts {
			if receipt.ClientID == target && receipt.Alias == "rich" && receipt.InstallationID == installation.InstallationID && receipt.ClientBindingID == binding.ClientBindingID && receipt.SourceKey == pin.SourceKey && receipt.PackageDigest == pin.TreeDigest {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing exact %s pin ownership receipt: %+v", target, state.EnvironmentReceipts)
		}
	}
	return pin
}

func (f pmInstallJourney) assertInstalled(t *testing.T, project, target, revision string, replacement bool) (domain.Installation, domain.ClientBinding) {
	t.Helper()
	installation, binding := f.binding(t, project, target)
	activation := domain.ActivationPrepared
	scope := domain.ScopeProject
	if project == "" {
		scope = domain.ScopeUser
		if target == "gemini" {
			activation = domain.ActivationActive
		}
	}
	if binding.Scope != string(scope) || binding.NativeProfileRoot != f.native(project, target) || binding.Activation != activation || binding.Verification != domain.VerificationInstalled {
		t.Fatalf("scope/readiness: %+v", binding)
	}
	if err := pathpolicy.RequireContainedChild(f.namespace(project), binding.TargetLocator); err != nil {
		t.Fatal(err)
	}
	second, remote := "audit", "docs"
	if replacement {
		second, remote = "release", "release"
	}
	for _, name := range []string{"docs", second} {
		for _, root := range []string{binding.TargetLocator, f.native(project, target)} {
			if got := projectJourneyRead(t, filepath.Join(root, "skills", name, "SKILL.md")); got != pmInstallSkill(name, revision) {
				t.Fatalf("wrong %s skill bytes in %s: %q", name, root, got)
			}
		}
	}
	if got := projectJourneyRead(t, filepath.Join(binding.TargetLocator, "marker.txt")); got != "independent package payload "+revision+"\n" {
		t.Fatalf("wrong package marker: %q", got)
	}
	if got := projectJourneyRead(t, filepath.Join(f.native(project, target), "skills", "foreign", "SKILL.md")); got != "foreign skill must survive\n" {
		t.Fatal("foreign skill changed")
	}
	name := "settings.json"
	if target == "cursor" {
		name = "mcp.json"
	}
	var config struct {
		Servers    map[string]map[string]any `json:"mcpServers"`
		Preference string                    `json:"foreignPreference"`
	}
	if err := json.Unmarshal([]byte(projectJourneyRead(t, filepath.Join(f.native(project, target), name))), &config); err != nil {
		t.Fatal(err)
	}
	if len(config.Servers) != 3 || config.Preference != "keep" {
		t.Fatalf("unexpected native configuration: %+v", config)
	}
	urlKey := "httpUrl"
	if target == "cursor" {
		urlKey = "url"
	}
	if config.Servers["foreign"][urlKey] != "https://foreign.test/mcp" || config.Servers[remote][urlKey] != "https://managed.test/"+revision {
		t.Fatalf("native remote/foreign URL mismatch: %+v", config.Servers)
	}
	if target == "cursor" && config.Servers["foreign"]["disabled"] != true {
		t.Fatal("foreign disabled server was enabled")
	}
	headers, ok := config.Servers[remote]["headers"].(map[string]any)
	if !ok || headers["X-Fixture"] != revision {
		t.Fatalf("native remote headers=%+v", config.Servers[remote])
	}
	worker := config.Servers["worker"]
	if target == "gemini" {
		if worker["command"] != "go" || !reflect.DeepEqual(worker["args"], []any{"version"}) {
			t.Fatalf("native stdio command=%+v", worker)
		}
		env, ok := worker["env"].(map[string]any)
		if !ok || env["FIXTURE"] != revision || env["PLUGIN_ROOT"] != binding.TargetLocator || env["PLUGIN_DATA"] == "" || env["DATA"] != env["PLUGIN_DATA"] {
			t.Fatalf("native stdio environment=%+v", worker)
		}
		if worker["cwd"] != binding.TargetLocator {
			t.Fatalf("native stdio cwd=%+v", worker)
		}
		found := false
		for _, receipt := range installation.DataReceipts {
			if receipt.Locator == env["PLUGIN_DATA"] {
				found = true
			}
		}
		if !found {
			t.Fatal("native stdio data is not scoped persistent data")
		}
	} else {
		command, ok := worker["command"].(string)
		if !ok || command != filepath.Join(binding.TargetLocator, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName) {
			t.Fatalf("Cursor did not use scoped launcher: %+v", worker)
		}
		var data string
		for _, receipt := range installation.DataReceipts {
			data = receipt.Locator
		}
		if !reflect.DeepEqual(worker["args"], []any{"--internal-stdio-v1", binding.TargetLocator, data, binding.TargetLocator, "plugin", "--argv-base64-v1", "Z28=", "dmVyc2lvbg=="}) {
			t.Fatalf("Cursor launcher bindings=%+v", worker)
		}
		env, ok := worker["env"].(map[string]any)
		if !ok || env["FIXTURE"] != revision || env["PLUGIN_ROOT"] != binding.TargetLocator || env["PLUGIN_DATA"] != data || env["DATA"] != data {
			t.Fatalf("Cursor launcher environment=%+v", worker)
		}
	}
	if len(installation.DataReceipts) != 1 {
		t.Fatalf("persistent data receipts=%+v", installation.DataReceipts)
	}
	for _, receipt := range installation.DataReceipts {
		if err := pathpolicy.RequireContainedChild(filepath.Join(f.namespace(project), "plugin-data"), receipt.Locator); err != nil {
			t.Fatal(err)
		}
	}
	return installation, binding
}

func pmInstallEqual(t *testing.T, label string, before, after any) {
	t.Helper()
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("%s changed\nbefore=%#v\nafter=%#v", label, before, after)
	}
}
func (f pmInstallJourney) activeSnapshot(t *testing.T, project string) map[string]map[string]string {
	t.Helper()
	out := map[string]map[string]string{}
	for _, root := range []string{f.namespace(project), f.native(project, "gemini"), f.native(project, "cursor")} {
		if _, err := os.Lstat(root); os.IsNotExist(err) {
			out[root] = nil
		} else {
			out[root] = pmInstallStableTree(t, root)
		}
	}
	if project != "" {
		for _, name := range []string{"uap.toml", "uap.lock"} {
			path := filepath.Join(project, name)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				out[path] = nil
			} else {
				out[path] = securityFixtureTree(t, path)
			}
		}
	}
	return out
}

// Process lock metadata is an observation of each invocation, not installed
// package authority. Immutable acquisition objects are outside active effects.
func pmInstallStableTree(t *testing.T, root string) map[string]string {
	t.Helper()
	entries := securityFixtureTree(t, root)
	for name := range entries {
		if name == "mutation.lock" || name == "environment.lock" || name == "store" || strings.HasPrefix(name, "store"+string(filepath.Separator)) || strings.HasSuffix(name, ".agentplugins.lock") {
			delete(entries, name)
		}
	}
	return entries
}
func (f pmInstallJourney) noActive(t *testing.T, project string) {
	t.Helper()
	for _, name := range []string{"state-v2.json", "managed", "plugin-data"} {
		projectJourneyAbsent(t, filepath.Join(f.namespace(project), name))
	}
	state := f.state(t, project)
	if len(state.Installations) != 0 || len(state.EnvironmentReceipts) != 0 {
		t.Fatalf("preview/refusal published state: %+v", state)
	}
}
