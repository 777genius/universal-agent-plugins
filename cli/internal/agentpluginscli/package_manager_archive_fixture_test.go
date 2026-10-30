//go:build uappm_e2e

package agentpluginscli

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// These fixtures exercise production acquisition and installation. Only security
// assessment and optional native client responses are inert host fixtures. No Git
// repository, native agent, MCP process or public endpoint is needed for ZIPs.
type pmArchiveOrigin struct {
	server   *httptest.Server
	mu       sync.Mutex
	payloads map[string][]byte
	requests map[string]int
	blocked  string
	started  chan struct{}
	release  <-chan struct{}
}

type pmArchiveGraph map[string][]string
type pmArchiveTargets map[string][]string

func pmArchiveNames[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func newPMArchiveOrigin(t *testing.T, graph pmArchiveGraph) *pmArchiveOrigin {
	t.Helper()
	origin := &pmArchiveOrigin{payloads: map[string][]byte{}, requests: map[string]int{}}
	origin.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin.mu.Lock()
		origin.requests[r.Method+" "+r.URL.Path]++
		body, found := origin.payloads[r.URL.Path]
		blocked, started, release := origin.blocked == r.URL.Path, origin.started, origin.release
		origin.mu.Unlock()
		if blocked {
			select {
			case started <- struct{}{}:
			default:
			}
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		if !found || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(body)
	}))
	t.Cleanup(origin.server.Close)
	for _, name := range pmArchiveNames(graph) {
		manifest := "schema_version = 1\n"
		for _, child := range graph[name] {
			if _, ok := graph[child]; !ok {
				t.Fatalf("fixture edge %s -> unknown %s", name, child)
			}
			manifest += fmt.Sprintf("[dependencies.%s]\narchive = %q\n", child, origin.url(child))
		}
		files := map[string]string{
			"plugin.json":                  fmt.Sprintf(`{"$schema":%q,"name":%q,"version":"1.0.0","description":"Disposable package-manager graph"}`, domain.PluginSchemaV1, name),
			"uap.toml":                     manifest,
			"marker.txt":                   pmArchiveMarker(name),
			"skills/" + name + "/SKILL.md": pmArchiveSkill(name),
			"mcp.json":                     fmt.Sprintf(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{%q:{"type":"streamable-http","url":%q}}}`, name, "https://example.test/pm/"+name),
		}
		var body bytes.Buffer
		writer := zip.NewWriter(&body)
		for _, path := range pmArchiveNames(files) {
			file, err := writer.Create("publisher/" + path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := file.Write([]byte(files[path])); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		origin.payloads["/"+name+".zip"] = body.Bytes()
	}
	return origin
}

func pmArchiveMarker(name string) string {
	return "independent ZIP payload for " + name + " version 1.0.0\n"
}
func pmArchiveSkill(name string) string {
	return fmt.Sprintf("---\nname: %s\ndescription: Disposable graph marker\n---\n%s", name, pmArchiveMarker(name))
}
func (o *pmArchiveOrigin) url(name string) string { return o.server.URL + "/" + name + ".zip" }
func (o *pmArchiveOrigin) counts() map[string]int {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := map[string]int{}
	for key, value := range o.requests {
		out[key] = value
	}
	return out
}

func (o *pmArchiveOrigin) sidecar(t *testing.T, name, sidecar string) {
	t.Helper()
	o.mu.Lock()
	defer o.mu.Unlock()
	body := o.payloads["/"+name+".zip"]
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	var updated bytes.Buffer
	writer := zip.NewWriter(&updated)
	for _, member := range reader.File {
		input, err := member.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, readErr := io.ReadAll(input)
		closeErr := input.Close()
		if readErr != nil || closeErr != nil {
			t.Fatalf("read fixture ZIP: %v %v", readErr, closeErr)
		}
		if member.Name == "publisher/uap.toml" {
			content = []byte(sidecar)
		}
		output, err := writer.Create(member.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := output.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	o.payloads["/"+name+".zip"] = updated.Bytes()
}

type pmArchiveEnvironment struct {
	cliFixture
	clients map[string]domain.DetectedClient
}

func newPMArchiveEnvironment(t *testing.T, origin *pmArchiveOrigin, mixed bool) pmArchiveEnvironment {
	t.Helper()
	gemini := fixtureClient(t, domain.ClientGemini)
	gemini.ConfigRoot = filepath.Join(filepath.Dir(gemini.ConfigRoot), "profile space é", "gemini")
	if err := os.MkdirAll(gemini.ConfigRoot, 0700); err != nil {
		t.Fatal(err)
	}
	dependencyWrite(t, filepath.Join(gemini.ConfigRoot, "settings.json"), `{"mcpServers":{"foreign.test":{"httpUrl":"https://foreign.test/mcp"}},"sentinel":"preserve foreign settings"}`+"\n")
	clients := []domain.DetectedClient{gemini}
	runner := &dependencyFixtureRunner{id: domain.ClientCodex}
	if mixed {
		codex := fixtureClient(t, domain.ClientCodex)
		codex.ExecutablePath = filepath.Join(t.TempDir(), "codex-fixture-never-executed")
		clients = append(clients, codex)
	}
	fixture, _ := newDependencyHarness(t, "", clients, installer.Config{Runner: runner})
	fixture.app.SourceAcquirer = sourceacquisition.Acquirer{TempRoot: t.TempDir(), HTTPClient: origin.server.Client()}
	out := pmArchiveEnvironment{cliFixture: fixture, clients: map[string]domain.DetectedClient{}}
	for _, client := range clients {
		out.clients[string(client.ClientID)] = client
	}
	return out
}

func (f pmArchiveEnvironment) executePM(ctx context.Context, t *testing.T, input string, terminal bool, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	var stdout, stderr bytes.Buffer
	app := f.app
	app.Input, app.Output, app.ErrorOutput, app.Terminal = strings.NewReader(input), &stdout, &stderr, terminal
	command := NewRoot(app)
	command.SetArgs(args)
	err := command.ExecuteContext(ctx)
	if stderr.Len() != 0 {
		t.Logf("CLI stderr: %s", stderr.String())
	}
	return stdout.String(), err
}

func (f pmArchiveEnvironment) run(t *testing.T, args ...string) string {
	t.Helper()
	output, err := f.executePM(t.Context(), t, "", false, args...)
	if err != nil {
		t.Fatalf("%v: %v\n%s", args, err, output)
	}
	return output
}

func (f pmArchiveEnvironment) manifest(t *testing.T, origin *pmArchiveOrigin, roots []string, targets map[string][]string) {
	t.Helper()
	body := "schema_version = 1\n"
	for _, root := range roots {
		body += fmt.Sprintf("[dependencies.%s]\narchive = %q\n", root, origin.url(root))
	}
	body += "[targets]\n"
	for _, target := range pmArchiveNames(targets) {
		quoted := make([]string, 0, len(targets[target]))
		for _, root := range targets[target] {
			quoted = append(quoted, fmt.Sprintf("%q", root))
		}
		body += target + " = [" + strings.Join(quoted, ", ") + "]\n"
	}
	mp, _ := f.app.dependencyPaths()
	dependencyWrite(t, mp, body)
}

func (f pmArchiveEnvironment) lock(t *testing.T) environment.Lock {
	t.Helper()
	return freshRestoreReadLock(t, f.cliFixture)
}

func pmArchiveAll(graph pmArchiveGraph, targets ...string) pmArchiveTargets {
	out := pmArchiveTargets{}
	for name := range graph {
		out[name] = append([]string(nil), targets...)
	}
	return out
}

// Expected nodes/targets are supplied by each scenario, never derived from the
// production lock. Parents come from the independent fixture edge declaration.
func (f pmArchiveEnvironment) assertGraph(t *testing.T, origin *pmArchiveOrigin, graph pmArchiveGraph, expected pmArchiveTargets) environment.Lock {
	t.Helper()
	lock := f.lock(t)
	if len(lock.Packages) != len(expected) {
		t.Fatalf("pins=%d want=%d: %+v", len(lock.Packages), len(expected), lock.Packages)
	}
	seen := map[string]bool{}
	for _, pin := range lock.Packages {
		targets, ok := expected[pin.Alias]
		if !ok || seen[pin.Alias] {
			t.Fatalf("unexpected/duplicate pin: %+v", pin)
		}
		seen[pin.Alias] = true
		origin.mu.Lock()
		digest := sha256.Sum256(origin.payloads["/"+pin.Alias+".zip"])
		origin.mu.Unlock()
		if pin.Version != "1.0.0" || pin.Source.Archive == nil || pin.Source.Archive.URL != origin.url(pin.Alias) || pin.Source.Archive.SHA256 != hex.EncodeToString(digest[:]) || pin.Source.Archive.RequestedSHA256 != "" || pin.Source.Archive.Path != "" || !slices.Equal(pin.Targets, targets) {
			t.Fatalf("pin identity/targets: %+v want targets=%v SHA=%x", pin, targets, digest)
		}
		wantParents := []environment.RequiredBy{}
		for _, parent := range pmArchiveNames(expected) {
			if slices.Contains(graph[parent], pin.Alias) {
				wantParents = append(wantParents, environment.RequiredBy{Alias: parent, Selector: environment.Selector{Kind: environment.SelectorArchive}})
			}
		}
		if !slices.Equal(pin.RequiredBy, wantParents) {
			t.Fatalf("%s parents=%+v want=%+v", pin.Alias, pin.RequiredBy, wantParents)
		}
	}
	return lock
}

func (f pmArchiveEnvironment) assertInstalled(t *testing.T, expected pmArchiveTargets) map[string]installer.InspectedBinding {
	t.Helper()
	view, err := f.app.Installer.Inspect(t.Context())
	if err != nil || view.Recovery.Required {
		t.Fatalf("inspection: %+v %v", view, err)
	}
	bindings := map[string]installer.InspectedBinding{}
	settings := dependencyRead(t, filepath.Join(f.clients["gemini"].ConfigRoot, "settings.json"))
	var native struct {
		Servers  map[string]map[string]any `json:"mcpServers"`
		Sentinel string                    `json:"sentinel"`
	}
	if err := json.Unmarshal([]byte(settings), &native); err != nil {
		t.Fatal(err)
	}
	if native.Sentinel != "preserve foreign settings" || native.Servers["foreign.test"]["httpUrl"] != "https://foreign.test/mcp" {
		t.Fatal("installation/removal changed foreign Gemini settings")
	}
	wantNative := 1
	for name, targets := range expected {
		if slices.Contains(targets, "gemini") {
			wantNative++
			wantServer := map[string]any{"httpUrl": "https://example.test/pm/" + name}
			if !reflect.DeepEqual(native.Servers[name], wantServer) {
				t.Fatalf("Gemini server %s=%#v want=%#v", name, native.Servers[name], wantServer)
			}
			if body := dependencyRead(t, filepath.Join(f.clients["gemini"].ConfigRoot, "skills", name, "SKILL.md")); body != pmArchiveSkill(name) {
				t.Fatalf("wrong native Gemini skill bytes for %s: %q", name, body)
			}
		}
	}
	if len(native.Servers) != wantNative {
		t.Fatalf("Gemini server count=%d want=%d; stale or missing native entries", len(native.Servers), wantNative)
	}
	lock := f.lock(t)
	liveInstallations := 0
	wantBindings := 0
	for _, targets := range expected {
		wantBindings += len(targets)
	}
	for _, installation := range view.Installations {
		if len(installation.Bindings) != 0 {
			liveInstallations++
		}
		for _, binding := range installation.Bindings {
			if binding.Owner == nil {
				t.Fatalf("unowned graph binding: %+v", binding)
			}
			name, target := binding.Owner.Alias, binding.ClientID
			key := name + "/" + target
			if !slices.Contains(expected[name], target) || bindings[key].BindingID != "" || binding.Materialization != "materialized" || binding.ProjectionStatus != "" || binding.Activation != "active" || binding.Verification != "installation_verified" {
				t.Fatalf("unexpected/unhealthy binding: %+v", binding)
			}
			bindings[key] = binding
			pin, ok := lock.Package(name)
			if !ok || binding.TreeDigest != pin.TreeDigest || binding.Revision != "sha256:"+pin.Source.Archive.SHA256 || binding.Owner.SourceKey != pin.SourceKey || binding.Owner.PackageDigest != pin.TreeDigest {
				t.Fatalf("installed identity differs from checked pin: %+v", binding)
			}
			if body := dependencyRead(t, filepath.Join(binding.TargetPath, "marker.txt")); body != pmArchiveMarker(name) {
				t.Fatalf("wrong installed bytes for %s: %q", key, body)
			}
			if body := dependencyRead(t, filepath.Join(binding.TargetPath, "skills", name, "SKILL.md")); body != pmArchiveSkill(name) {
				t.Fatalf("wrong installed skill bytes for %s: %q", key, body)
			}
		}
	}
	if len(bindings) != wantBindings || len(view.EnvironmentReceipts) != wantBindings {
		t.Fatalf("bindings=%d receipts=%d want=%d", len(bindings), len(view.EnvironmentReceipts), wantBindings)
	}
	if liveInstallations != len(expected) {
		t.Fatalf("live installations=%d want=%d; shared source duplicated", liveInstallations, len(expected))
	}
	return bindings
}

func (f pmArchiveEnvironment) activeFiles(t *testing.T) map[string]freshRestoreStamp {
	t.Helper()
	roots := make([]string, 0, 2+len(f.clients))
	roots = append(roots, f.store.Path, f.app.ManagedRoot)
	for _, client := range f.clients {
		roots = append(roots, client.ConfigRoot)
	}
	return freshRestoreFiles(t, roots...)
}

func pmArchiveAgreement(t *testing.T, from, to pmArchiveEnvironment) {
	t.Helper()
	fromManifest, fromLock := from.app.dependencyPaths()
	toManifest, toLock := to.app.dependencyPaths()
	dependencyWrite(t, toManifest, dependencyRead(t, fromManifest))
	dependencyWrite(t, toLock, dependencyRead(t, fromLock))
}

func pmArchiveAssertEqual(t *testing.T, label string, before, after any) {
	t.Helper()
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("%s changed\nbefore: %#v\nafter: %#v", label, before, after)
	}
}

func pmArchiveAssertNoActive(t *testing.T, f pmArchiveEnvironment) {
	t.Helper()
	state, err := f.store.Load()
	if err != nil || len(state.Installations) != 0 || len(state.EnvironmentReceipts) != 0 {
		t.Fatalf("refusal/preview has active state: %+v %v", state, err)
	}
	for _, path := range []string{f.app.ManagedRoot, f.store.Path, f.app.EnvironmentStoreRoot} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("unexpected durable path %s: %v", path, err)
		}
	}
}

func pmArchiveWriteJSON(t *testing.T, path string, value any) {
	t.Helper()
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	dependencyWrite(t, path, string(body)+"\n")
}
