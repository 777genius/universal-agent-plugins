package hermes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func newProjectSessionFixture(t *testing.T, names ...string) (clients.SessionRequest, domain.NativeObjectOwnership) {
	t.Helper()
	project, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	project = filepath.Join(append([]string{project}, names...)...)
	if err := os.MkdirAll(filepath.Join(project, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
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
	req := clients.SessionRequest{ProjectRoot: project, Inspect: true, Client: domain.DetectedClient{ClientID: domain.ClientHermes, ConfigRoot: user, ExecutablePath: sessionBundleFixture(t, t.TempDir()), Status: domain.DetectionDetected, Surfaces: []domain.ClientSurface{{ID: "hermes_cli", Detected: true}}}, Bindings: []domain.ClientBinding{{ClientID: string(domain.ClientHermes), Scope: string(domain.ScopeProject), NativeProfileRoot: filepath.Join(project, ".hermes"), NativeObjects: []domain.NativeObjectOwnership{owner, {Kind: projectSessionMCPKind, ObjectID: projectSessionMCPKind + ":owned", LogicalName: "owned", Path: config, ManagedDigest: receipt.Digest, ProtectionClass: "managed"}}}}}
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
	if !reflect.DeepEqual(plan.Environment, map[string]string{"HERMES_HOME": projectSessionRoot(req.ProjectRoot)}) || !reflect.DeepEqual(plan.Arguments, []string{"config", "get", "mcp_servers", "--json"}) {
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
	for _, name := range []string{"active_profile", "profiles", "history", "auth.json", ".env", ".op.env", "plugins", "profile.yaml"} {
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

func TestProjectSessionBootstrapRetainsHistoricalBackupAcrossOwnedUpdate(t *testing.T) {
	req, owner := newProjectSessionFixture(t)
	backup := sessionBootstrapFixture(t, req.ProjectRoot)
	original, err := os.ReadFile(backup)
	if err != nil {
		t.Fatal(err)
	}
	before, err := prepareProjectSession(t.Context(), req, owner)
	if err != nil {
		t.Fatal("measured bootstrap refused", err)
	}
	object := &req.Bindings[0].NativeObjects[1]
	previous := nativeconfig.Receipt{Version: "1", Path: object.Path, Codec: nativeconfig.CodecHermes, Name: object.LogicalName, Digest: object.ManagedDigest}
	receipt, err := nativeconfig.New().Apply(nativeconfig.Request{Paths: nativeconfig.Paths{JSON: object.Path}, Codec: nativeconfig.CodecHermes,
		Action: nativeconfig.ActionUpdate, Name: object.LogicalName, Owned: &previous, Server: nativeconfig.Server{Type: "remote", URL: "http://127.0.0.1:2345/mcp"}})
	if err != nil {
		t.Fatal(err)
	}
	object.ManagedDigest = receipt.Digest
	after, err := prepareProjectSession(t.Context(), req, owner)
	if err != nil || before.ConfigDigest == after.ConfigDigest {
		t.Fatal("updated primary with historical backup refused", err)
	}
	if err := verifyProjectSessionSource(t.Context(), after, owner); err != nil {
		t.Fatal(err)
	}
	retained, err := os.ReadFile(backup)
	if err != nil || !reflect.DeepEqual(original, retained) {
		t.Fatal("historical backup was rewritten", err)
	}
}

func TestProjectSessionBootstrapRefusesUnknownUnsafeAndExcessMembers(t *testing.T) {
	cases := []string{"soul", "nested", "backup-reason", "backup-time", "backup-fraction", "backup-comma", "backup-count", "log-size", "backup-size", "hardlink"}
	if runtime.GOOS != "windows" {
		cases = append(cases, "log-mode", "directory-mode")
	}
	for _, change := range cases {
		t.Run(change, func(t *testing.T) {
			req, owner := newProjectSessionFixture(t)
			backup := sessionBootstrapFixture(t, req.ProjectRoot)
			root := projectSessionRoot(req.ProjectRoot)
			path, body, mode := filepath.Join(root, "SOUL.md"), []byte(strings.Repeat("x", 667)), os.FileMode(0o600)
			switch change {
			case "nested":
				path = filepath.Join(root, "skills", "foreign")
			case "backup-reason":
				path = filepath.Join(filepath.Dir(backup), "config.yaml.corrupt.20261113-120000")
			case "backup-time":
				path = filepath.Join(filepath.Dir(backup), "config.yaml.good.20261399-120000")
			case "backup-fraction":
				path = backup + ".1"
			case "backup-comma":
				path = backup + ",1"
			case "log-size":
				path, body, mode = filepath.Join(root, "logs", "agent.log"), make([]byte, (1<<20)+1), 0o644
			case "backup-size":
				path, body = backup, make([]byte, (1<<20)+1)
			case "backup-count":
				for index := 1; index <= 5; index++ {
					if err := os.WriteFile(filepath.Join(filepath.Dir(backup), fmt.Sprintf("config.yaml.good.20261113-12000%d", index)), nil, 0o600); err != nil {
						t.Fatal(err)
					}
				}
			case "hardlink":
				if err := os.Link(filepath.Join(root, "logs", "agent.log"), filepath.Join(t.TempDir(), "outside-log")); err != nil {
					t.Fatal(err)
				}
			case "log-mode", "directory-mode":
				path = filepath.Join(root, "logs", "agent.log")
				if change == "directory-mode" {
					path = filepath.Join(root, "backups")
				}
				if err := os.Chmod(path, 0o777); err != nil {
					t.Fatal(err)
				}
			}
			if change != "backup-count" && change != "hardlink" && !strings.HasSuffix(change, "-mode") {
				if err := os.WriteFile(path, body, mode); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := prepareProjectSession(t.Context(), req, owner); err == nil {
				t.Fatal("unsafe bootstrap member was admitted")
			}
		})
	}
}

func TestProjectSessionStructurallyValidatesSkillsOnlyBindingAlongsideMCP(t *testing.T) {
	req, owner := newProjectSessionFixture(t)
	skill := domain.NativeObjectOwnership{Kind: projectSkillKind, ObjectID: projectSkillKind + ":docs", LogicalName: "docs",
		Path: filepath.Join(req.ProjectRoot, ".hermes", "skills", "docs"), SourceRelative: "skills/docs", ManagedDigest: "owned-skill", ProtectionClass: "managed"}
	binding := domain.ClientBinding{ClientID: string(domain.ClientHermes), Scope: string(domain.ScopeProject),
		NativeProfileRoot: filepath.Join(req.ProjectRoot, ".hermes"), NativeObjects: []domain.NativeObjectOwnership{skill}}
	req.Bindings = append(req.Bindings, binding)
	if _, err := prepareProjectSession(t.Context(), req, owner); err != nil {
		t.Fatal("Skills-only binding incorrectly required an MCP root", err)
	}
	req.Bindings[1].NativeObjects[0].Path = filepath.Join(req.ProjectRoot, ".hermes", "skills", "foreign")
	if _, err := prepareProjectSession(t.Context(), req, owner); err == nil {
		t.Fatal("foreign Skills ownership was not structurally rejected")
	}
}
