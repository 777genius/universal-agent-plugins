package hermes

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func newProjectSessionFixture(t *testing.T) (clients.SessionRequest, domain.NativeObjectOwnership) {
	t.Helper()
	project := t.TempDir()
	if err := os.Mkdir(filepath.Join(project, ".hermes"), 0o700); err != nil {
		t.Fatal(err)
	}
	owner, err := createProjectSessionRoot(context.Background(), project)
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(projectSessionRoot(project), "config.yaml")
	receipt, err := nativeconfig.New().Apply(nativeconfig.Request{Paths: nativeconfig.Paths{JSON: config}, Codec: nativeconfig.CodecHermes, Action: nativeconfig.ActionAdd, Name: "owned", Server: nativeconfig.Server{Type: "remote", URL: "http://127.0.0.1:1234/mcp"}})
	if err != nil {
		t.Fatal(err)
	}
	user := t.TempDir()
	req := clients.SessionRequest{ProjectRoot: project, Inspect: true, Client: domain.DetectedClient{ClientID: domain.ClientHermes, ConfigRoot: user, ExecutablePath: filepath.Join(t.TempDir(), "hermes")}, Bindings: []domain.ClientBinding{{ClientID: string(domain.ClientHermes), Scope: string(domain.ScopeProject), NativeProfileRoot: filepath.Join(project, ".hermes"), NativeObjects: []domain.NativeObjectOwnership{owner, {Kind: projectSessionMCPKind, ObjectID: projectSessionMCPKind + ":owned", LogicalName: "owned", Path: config, ManagedDigest: receipt.Digest, ProtectionClass: "managed"}}}}}
	return req, owner
}

func TestProjectSessionRootRequiresFreshOrExactReceipt(t *testing.T) {
	req, owner := newProjectSessionFixture(t)
	if err := verifyProjectSessionRoot(context.Background(), req.ProjectRoot, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := createProjectSessionRoot(context.Background(), req.ProjectRoot); err == nil {
		t.Fatal("adopted existing root")
	}
	wrong := owner
	wrong.ManagedDigest = "foreign"
	if err := verifyProjectSessionRoot(context.Background(), req.ProjectRoot, wrong); err == nil {
		t.Fatal("adopted foreign receipt")
	}
	marker := filepath.Join(projectSessionRoot(req.ProjectRoot), ".uap-owner")
	if err := os.WriteFile(marker, []byte("foreign"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyProjectSessionRoot(context.Background(), req.ProjectRoot, owner); err == nil {
		t.Fatal("adopted foreign marker")
	}
}

func TestProjectSessionSourceUsesOneChildOverrideAndExactReceipts(t *testing.T) {
	req, owner := newProjectSessionFixture(t)
	userConfig := filepath.Join(req.Client.ConfigRoot, "config.yaml")
	const foreign = "model: private\nmcp_servers:\n  foreign:\n    command: foreign\n"
	if err := os.WriteFile(userConfig, []byte(foreign), 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := prepareProjectSession(context.Background(), req, owner)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plan.Environment, map[string]string{"HERMES_HOME": projectSessionRoot(req.ProjectRoot)}) || !reflect.DeepEqual(plan.Arguments, []string{"mcp", "list"}) {
		t.Fatal("unexpected selected process", plan)
	}
	child := plan.ChildEnvironment([]string{"HOME=/original-os-home", "HERMES_HOME=/foreign", "KEEP=unchanged"})
	if !reflect.DeepEqual(child, []string{"HOME=/original-os-home", "KEEP=unchanged", "HERMES_HOME=" + projectSessionRoot(req.ProjectRoot)}) {
		t.Fatal("changed ordinary HOME", child)
	}
	if err := verifyProjectSessionSource(context.Background(), plan, owner); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(userConfig)
	if err != nil || string(body) != foreign {
		t.Fatal("changed original User")
	}
	config := filepath.Join(projectSessionRoot(req.ProjectRoot), "config.yaml")
	body, err = os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte(strings.ReplaceAll(string(body), "1234", "2345")), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareProjectSession(context.Background(), req, owner); err == nil {
		t.Fatal("accepted altered server with same name")
	}
	if err := verifyProjectSessionSource(context.Background(), plan, owner); err == nil {
		t.Fatal("accepted source change after planning")
	}
}

func TestProjectSessionRejectsUnknownRootEntriesWithoutRemovingThem(t *testing.T) {
	for _, name := range []string{"active_profile", "profiles", "history", "auth.json"} {
		t.Run(name, func(t *testing.T) {
			req, owner := newProjectSessionFixture(t)
			path := filepath.Join(projectSessionRoot(req.ProjectRoot), name)
			if err := os.WriteFile(path, []byte("private foreign value"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := prepareProjectSession(context.Background(), req, owner); err == nil {
				t.Fatal("accepted unknown root entry")
			}
			b, err := os.ReadFile(path)
			if err != nil || string(b) != "private foreign value" {
				t.Fatal("removed unknown entry")
			}
		})
	}
}

func TestProjectSessionRefusesAliasesAndCancellation(t *testing.T) {
	req, owner := newProjectSessionFixture(t)
	root := projectSessionRoot(req.ProjectRoot)
	moved := root + "-original"
	if err := os.Rename(root, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(moved, root); err != nil {
		t.Skip("symlinks unavailable")
	}
	if _, err := prepareProjectSession(context.Background(), req, owner); err == nil {
		t.Fatal("accepted root alias")
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := prepareProjectSession(canceled, req, owner); err == nil {
		t.Fatal("ignored pre-cancellation")
	}
}

func TestProjectSessionConfigRejectsForeignSettingsAndAmbiguousYAML(t *testing.T) {
	for _, body := range []string{
		"mcp_servers: {}\n", "model: private\nmcp_servers: {owned: {url: x}}\n",
		"mcp_servers: {owned: {url: x}, foreign: {url: y}}\n",
		"mcp_servers: {owned: {url: x}, owned: {url: x}}\n",
		"mcp_servers: {owned: {url: x, url: y}}\n",
		"mcp_servers: {owned: &server {url: x}}\n",
		"mcp_servers: {owned: {url: x}}\n---\nmcp_servers: {}\n",
	} {
		if err := exactProjectSessionConfig([]byte(body), []string{"owned"}); err == nil {
			t.Fatalf("accepted ambiguous source %q", body)
		}
	}
}

func TestProjectSessionSnapshotNeverWrites(t *testing.T) {
	snapshot := sessionConfigSnapshot{path: "/owned/config.yaml", body: []byte("mcp_servers: {}")}
	if _, _, _, err := snapshot.ReadNoFollow("/foreign"); err == nil {
		t.Fatal("read foreign path")
	}
	if err := snapshot.WriteAtomic(snapshot.path, nil, 0o600); err == nil {
		t.Fatal("snapshot wrote")
	}
	if err := snapshot.RemoveNoFollow(snapshot.path); err == nil {
		t.Fatal("snapshot removed")
	}
}

func TestProjectSessionRefusesHardlinkedAndOversizedConfig(t *testing.T) {
	req, owner := newProjectSessionFixture(t)
	config := filepath.Join(projectSessionRoot(req.ProjectRoot), "config.yaml")
	other := filepath.Join(t.TempDir(), "outside-config")
	if err := os.Link(config, other); err != nil {
		t.Skip("hardlinks unavailable")
	}
	if _, err := prepareProjectSession(context.Background(), req, owner); err == nil {
		t.Fatal("accepted external hardlink")
	}
	if err := os.Remove(other); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, make([]byte, (1<<20)+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareProjectSession(context.Background(), req, owner); err == nil {
		t.Fatal("accepted oversized source")
	}
}

func TestProjectSessionRejectsForeignBindingBeforePlanning(t *testing.T) {
	for _, field := range []string{"user-root", "kind", "digest", "duplicate", "attempt"} {
		t.Run(field, func(t *testing.T) {
			req, owner := newProjectSessionFixture(t)
			binding := &req.Bindings[0]
			switch field {
			case "user-root":
				binding.NativeProfileRoot = t.TempDir()
			case "kind":
				binding.NativeObjects[1].Kind = userMCPKind
			case "digest":
				binding.NativeObjects[1].ManagedDigest = "sha256:foreign"
			case "duplicate":
				binding.NativeObjects = append(binding.NativeObjects, binding.NativeObjects[1])
			case "attempt":
				binding.NativeActivationAttempt = "unacknowledged"
			}
			if _, err := prepareProjectSession(context.Background(), req, owner); err == nil {
				t.Fatal("accepted foreign authority")
			}
		})
	}
}

func TestProjectSessionFreshChildSwapCannotWriteForeignMarker(t *testing.T) {
	project := t.TempDir()
	parentPath := filepath.Join(project, ".hermes")
	if err := os.Mkdir(parentPath, 0o700); err != nil {
		t.Fatal(err)
	}
	parent, err := os.OpenRoot(parentPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := parent.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := parent.Mkdir("uap-session", 0o700); err != nil {
		t.Fatal(err)
	}
	original, err := parent.Lstat("uap-session")
	if err != nil {
		t.Fatal(err)
	}
	if err := parent.Mkdir("foreign", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(parentPath, "uap-session"), filepath.Join(parentPath, "moved-owned")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("foreign", filepath.Join(parentPath, "uap-session")); err != nil {
		t.Skip("symlinks unavailable")
	}
	if err := writeSessionRootMarker(parent, original, project); err == nil {
		t.Fatal("accepted swapped child")
	}
	for _, name := range []string{"foreign", "moved-owned"} {
		if _, err := os.Lstat(filepath.Join(parentPath, name, ".uap-owner")); !os.IsNotExist(err) {
			t.Fatalf("marker written into %s: %v", name, err)
		}
	}
}
