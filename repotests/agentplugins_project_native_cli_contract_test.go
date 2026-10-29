//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
)

func TestProjectNativeCLIAttemptRetainsRealHostedBoundary(t *testing.T) {
	if !projectNativeCLIHosted("linux", "amd64", "true", "github-hosted", "1") {
		t.Fatal("genuine hosted candidate rejected")
	}
	for _, input := range [][5]string{{"linux", "amd64", "", "", "1"}, {"linux", "amd64", "true", "self-hosted", "1"}, {"linux", "amd64", "false", "github-hosted", "1"}, {"linux", "arm64", "true", "github-hosted", "1"}, {"darwin", "amd64", "true", "github-hosted", "1"}, {"linux", "amd64", "true", "github-hosted", "0"}} {
		if projectNativeCLIHosted(input[0], input[1], input[2], input[3], input[4]) {
			t.Fatalf("inert unsafe boundary admitted: %v", input)
		}
	}
}
func TestProjectNativeCLIAttemptToolRestrictionsCannotBecomeApproval(t *testing.T) {
	for _, client := range []string{"qoder", "grok", "kiro", "windsurf"} {
		spec, err := projectNativeCLISpecification(client)
		if err != nil || spec.client != client || len(spec.archiveSHA) != 64 || len(spec.executableSHA) != 64 {
			t.Fatal("finite client identity missing")
		}
		argv, err := projectNativeCLIArguments(client, "/owned/native", "/owned/receipt/.agentplugins-qoder-native", "owned prompt")
		if err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(argv, "\x00")
		for _, forbidden := range []string{"--always-approve", "--trust-all-tools", "--trust-tools", "--dangerously-skip-permissions", "bypass_permissions", "bypassPermissions", "--respect-workspace-trust\x00false", "--permission-mode\x00auto"} {
			if strings.Contains(joined, forbidden) {
				t.Fatalf("%s gained approval: %v", client, argv)
			}
		}
		switch client {
		case "qoder":
			if !strings.Contains(joined, "--tools\x00\x00--plugin-dir\x00/owned/receipt/.agentplugins-qoder-native") {
				t.Fatal("Qoder measured empty tool restriction absent")
			}
		case "grok":
			if strings.Contains(joined, "--tools") || !strings.Contains(joined, "--deny\x00\x00--no-subagents") || !strings.HasSuffix(joined, "--max-turns\x001\x00--model\x00uap-grok-fixture") {
				t.Fatal("Grok empty allowlist bug or missing unconditional deny")
			}
		case "kiro":
			if !reflect.DeepEqual(argv, []string{"/owned/native", "chat", "--v2", "--no-interactive", "owned prompt"}) {
				t.Fatal("Kiro gained unreviewed mode")
			}
		case "windsurf":
			if !strings.Contains(joined, "--respect-workspace-trust\x00true") {
				t.Fatal("native trust wall omitted")
			}
		}
	}
	if _, err := projectNativeCLISpecification("cursor"); err == nil {
		t.Fatal("403-blocked Cursor silently entered runtime table")
	}
	if _, err := projectNativeCLIArguments("unknown", "/native", "", "prompt"); err == nil {
		t.Fatal("unknown client argv accepted")
	}
	for _, view := range []string{"", "relative/.agentplugins-qoder-native", "/owned/whole-package", "/owned/../receipt/.agentplugins-qoder-native"} {
		if _, err := projectNativeCLIArguments("qoder", "/native", view, "prompt"); err == nil {
			t.Fatal("nonreceipt native view accepted", view)
		}
	}
}
func TestProjectNativeCLIAttemptOriginalUpdaterAndDenySettingsAreFrozen(t *testing.T) {
	_, qoder := projectNativeCLIUpdater("qoder")
	if !bytes.Equal(qoder, []byte("{\"general\":{\"enableAutoUpdate\":false}}\n")) {
		t.Fatal("Qoder updater suppression changed")
	}
	_, devin := projectNativeCLIUpdater("windsurf")
	value, err := projectNativeRuntimeJSON(devin)
	if err != nil {
		t.Fatal(err)
	}
	object := value.(map[string]any)
	permissions := object["permissions"].(map[string]any)
	if object["auto_update"] != false || !reflect.DeepEqual(permissions["allow"], []any{}) || !reflect.DeepEqual(permissions["ask"], []any{}) || !reflect.DeepEqual(permissions["deny"], []any{"read", "edit", "write", "grep", "glob", "exec", "fetch", "web_fetch", "mcp__*"}) {
		t.Fatal("Devin deny-only profile became an approval")
	}
}
func TestProjectNativeCLIClosedQoderViewRefusesExtraNativeSurfacesAndAliases(t *testing.T) {
	for _, kind := range []string{"valid", "mcp", "empty-dir", "alias", "hardlink", "wrong-manifest", "wrong-skill"} {
		t.Run(kind, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), ".agentplugins-qoder-native")
			name := "uap-qoder-attempt"
			skill := []byte("inert selected skill\n")
			manifest, _ := json.Marshal(map[string]any{"name": name, "version": "1.0.0", "description": "Disposable native project startup observation", "skills": "./skills/"})
			projectNativeRuntimeWrite(t, filepath.Join(root, ".qoder-plugin", "plugin.json"), manifest, 0600)
			path := filepath.Join(root, "skills", name, "SKILL.md")
			projectNativeRuntimeWrite(t, path, skill, 0600)
			switch kind {
			case "mcp":
				projectNativeRuntimeWrite(t, filepath.Join(root, ".mcp.json"), []byte("{}"), 0600)
			case "empty-dir":
				if err := os.Mkdir(filepath.Join(root, "hooks"), 0700); err != nil {
					t.Fatal(err)
				}
			case "alias":
				if err := os.Rename(path, path+".saved"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(path+".saved", path); err != nil {
					t.Fatal(err)
				}
			case "hardlink":
				if err := os.Link(path, filepath.Join(t.TempDir(), "other")); err != nil {
					t.Fatal(err)
				}
			case "wrong-manifest":
				if err := os.WriteFile(filepath.Join(root, ".qoder-plugin", "plugin.json"), []byte(`{"name":"foreign","version":"1.0.0","skills":"./skills/"}`), 0600); err != nil {
					t.Fatal(err)
				}
			case "wrong-skill":
				if err := os.WriteFile(path, []byte("foreign skill"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			err := projectNativeCLIClosedView(root, name, skill)
			if kind == "valid" && err != nil {
				t.Fatal(err)
			}
			if kind != "valid" && err == nil {
				t.Fatal("unsafe closed view admitted")
			}
		})
	}
}

func TestProjectNativeCLICommandLabelsCannotAuthorizeDifferentArgvOrScope(t *testing.T) {
	spec, _ := projectNativeCLISpecification("qoder")
	f := &projectNativeCLIAttempt{spec: spec, base: &nativeFixture{Project: "/owned/A", Package: "/owned/source"}, client: "/owned/qodercli", installer: "/owned/agentplugins", git: "/usr/bin/git", outside: "/owned/outside", other: "/owned/B", view: "/owned/A/receipt/.agentplugins-qoder-native", name: "uap-qoder-attempt"}
	good := []string{f.client, "--version"}
	if err := f.selectedCommand("version", f.outside, good, 30*time.Second); err != nil {
		t.Fatal(err)
	}
	for _, change := range []struct {
		label, cwd string
		argv       []string
		timeout    time.Duration
	}{{"version", f.base.Project, good, 30 * time.Second}, {"version", f.outside, []string{f.client, "login"}, 30 * time.Second}, {"version", f.outside, good, 60 * time.Second}, {"native-other", f.outside, good, 30 * time.Second}} {
		if f.selectedCommand(change.label, change.cwd, change.argv, change.timeout) == nil {
			t.Fatal("different native command/scope admitted")
		}
	}
	catalog := []string{f.client, "plugins", "list", "--json", "--plugin-dir", f.view}
	if err := f.selectedCommand("native-catalog", f.base.Project, catalog, 30*time.Second); err != nil {
		t.Fatal(err)
	}
	for _, change := range []struct {
		client, cwd string
		argv        []string
		timeout     time.Duration
	}{
		{"grok", f.base.Project, catalog, 30 * time.Second},
		{"qoder", f.other, catalog, 30 * time.Second},
		{"qoder", f.base.Project, append(append([]string{}, catalog...), "--available"), 30 * time.Second},
		{"qoder", f.base.Project, []string{f.client, "plugins", "install", f.view}, 30 * time.Second},
		{"qoder", f.base.Project, catalog, 60 * time.Second},
	} {
		f.spec.client = change.client
		if f.selectedCommand("native-catalog", change.cwd, change.argv, change.timeout) == nil {
			t.Fatal("unreviewed catalog argv/client/cwd/deadline admitted")
		}
	}
}

func TestProjectNativeCLIQoderCatalogRemainsOpaqueStrictWholeJSON(t *testing.T) {
	for _, body := range []string{`{}`, `[]`, `{"plugins":[],"unknownSchema":true}`} {
		if _, err := projectNativeCLIQoderCatalogJSON([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	for _, body := range []string{`null`, `true`, `"plugins"`, `0`, `{"plugins":[],"plugins":[]}`, `{"value":NaN}`, `[] trailing`, "\xff"} {
		if _, err := projectNativeCLIQoderCatalogJSON([]byte(body)); err == nil {
			t.Fatal("unsafe/scalar catalog accepted", body)
		}
	}
}

func TestProjectNativeCLIQoderCatalogCannotMutateOriginalProfileCache(t *testing.T) {
	for _, kind := range []string{"bytes", "mode", "empty-directory", "new-cache", "alias", "hardlink"} {
		t.Run(kind, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), ".qoder")
			path := filepath.Join(root, "settings.json")
			projectNativeRuntimeWrite(t, path, []byte("original updater configuration\n"), 0600)
			before, err := projectNativeCLIQoderProfile(root)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "bytes":
				err = os.WriteFile(path, []byte("changed"), 0600)
			case "mode":
				err = os.Chmod(path, 0640)
			case "empty-directory":
				err = os.Mkdir(filepath.Join(root, "empty"), 0700)
			case "new-cache":
				err = os.WriteFile(filepath.Join(root, "plugins-cache.json"), []byte("{}"), 0600)
			case "alias":
				err = os.Symlink(path, filepath.Join(root, "alias"))
			case "hardlink":
				err = os.Link(path, filepath.Join(t.TempDir(), "hardlink"))
			}
			if err != nil {
				t.Fatal(err)
			}
			after, err := projectNativeCLIQoderProfile(root)
			if err == nil && reflect.DeepEqual(before, after) {
				t.Fatal("catalog profile/cache change omitted", kind)
			}
		})
	}
}

func TestProjectNativeCLIQoderProfileRefusalReportsOnlyBoundedEntryMetadata(t *testing.T) {
	for _, kind := range []string{"symlink", "hardlink", "long-control-symlink"} {
		t.Run(kind, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), ".qoder")
			target := filepath.Join(t.TempDir(), "outside-target")
			body := []byte("foreign target contents must not enter diagnostics\n")
			projectNativeRuntimeWrite(t, target, body, 0600)
			rel := "entry"
			if kind == "long-control-symlink" {
				rel = filepath.Join(strings.Repeat("a", 100)+"\n\"", strings.Repeat("b", 100), strings.Repeat("c", 100), "entry")
			}
			path := filepath.Join(root, rel)
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			link := os.Symlink
			if kind == "hardlink" {
				link = os.Link
			}
			if err := link(target, path); err != nil {
				t.Fatal(err)
			}
			info, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			targetInfo, err := os.Lstat(target)
			if err != nil {
				t.Fatal(err)
			}
			_, err = projectNativeCLIQoderProfile(root)
			projectNativeCLIQoderRefusalMetadata(t, root, target, rel, info, err)
			after, err := os.Lstat(path)
			if err != nil || !projectNativeRuntimeStableStat(info.Sys(), after.Sys()) {
				t.Fatal("refusal changed the original entry")
			}
			targetAfter, err := os.Lstat(target)
			if err != nil || !projectNativeRuntimeStableStat(targetInfo.Sys(), targetAfter.Sys()) {
				t.Fatal("refusal changed foreign target identity")
			}
			got, err := os.ReadFile(target)
			if err != nil || !bytes.Equal(got, body) {
				t.Fatal("refusal changed foreign target contents")
			}
		})
	}
}

func projectNativeCLIQoderRefusalMetadata(t *testing.T, root, target, rel string, info os.FileInfo, refusal error) {
	t.Helper()
	identity, ok := info.Sys().(*syscall.Stat_t)
	if refusal == nil || !ok {
		t.Fatal("refusal and actual Linux entry metadata required")
	}
	message := refusal.Error()
	if len(rel) > 256 {
		rel = rel[:256] + "...[truncated]"
	}
	want := fmt.Sprintf("Qoder profile ordinary single-link file required: entry=%q mode=%s nlink=%d size=%d", rel, info.Mode().String(), identity.Nlink, info.Size())
	if message != want || len(message) > 2<<10 || strings.ContainsAny(message, "\r\n") || strings.Contains(message, root) || strings.Contains(message, target) || strings.Contains(message, "foreign target contents") {
		t.Fatalf("bounded entry metadata differs or leaks foreign authority: %q", message)
	}
}

func TestProjectNativeCLIMixedServerRefusesExtraTransportCWDAndUnknownFields(t *testing.T) {
	args := []any{"--internal-stdio-v1", "/owned/package", "/owned/data", "/owned/package", "plugin", "--", "./bin/probe"}
	env := map[string]any{"PLUGIN_ROOT": "/owned/package", "PLUGIN_DATA": "/owned/data"}
	good := map[string]any{"command": "/owned/package/launcher", "args": args, "env": env}
	if err := projectNativeCLIMixedEntry(good, "/owned/package/launcher", args, env); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"cwd", "type", "transport", "disabled", "unknown"} {
		wrong := map[string]any{"command": good["command"], "args": args, "env": env, key: "unqualified"}
		if projectNativeCLIMixedEntry(wrong, "/owned/package/launcher", args, env) == nil {
			t.Fatal("extra native field accepted", key)
		}
	}
}
func TestProjectNativeCLIOwnershipRequiresExactReceiptKindNamePathAndDigest(t *testing.T) {
	good := domain.NativeObjectOwnership{ObjectID: "package:qoder:physical", Kind: "managed_package_directory", LogicalName: "owned", Path: "/owned/package", ManagedDigest: "sha256:recorded"}
	expected := map[string]domain.NativeObjectOwnership{good.Kind: good}
	if err := projectNativeCLIOwnership([]domain.NativeObjectOwnership{good}, expected); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"id", "kind", "name", "path", "digest", "empty-digest"} {
		bad := good
		switch field {
		case "id":
			bad.ObjectID = "package:qoder:foreign"
		case "kind":
			bad.Kind = "foreign"
		case "name":
			bad.LogicalName = "foreign"
		case "path":
			bad.Path = "/other/project/package"
		case "digest":
			bad.ManagedDigest = "sha256:other"
		case "empty-digest":
			bad.ManagedDigest = ""
		}
		if projectNativeCLIOwnership([]domain.NativeObjectOwnership{bad}, expected) == nil {
			t.Fatal("wrong native ownership admitted", field)
		}
	}
	if projectNativeCLIOwnership(nil, expected) == nil || projectNativeCLIOwnership([]domain.NativeObjectOwnership{good, good}, expected) == nil {
		t.Fatal("missing/duplicate ownership admitted")
	}
}
func TestProjectNativeCLIPackageDigestMatchesInstallerFramingFromHeldBytes(t *testing.T) {
	root := t.TempDir()
	projectNativeRuntimeWrite(t, filepath.Join(root, "plugin.json"), []byte(`{"name":"inert"}`), 0600)
	projectNativeRuntimeWrite(t, filepath.Join(root, "bin", "probe"), []byte("inert executable bytes"), 0700)
	got, err := projectNativeCLIPackageDigest(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	want, err := (packagesnapshot.Builder{}).Digest(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("held installed package digest differs from selected lifecycle framing: %s %s", got, want)
	}
	if err := os.Link(filepath.Join(root, "plugin.json"), filepath.Join(t.TempDir(), "alias")); err != nil {
		t.Fatal(err)
	}
	if _, err := projectNativeCLIPackageDigest(context.Background(), root); err == nil {
		t.Fatal("aliased installed package digest accepted")
	}
}
