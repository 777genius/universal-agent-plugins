package clientdetect

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/codex"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/cursor"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func codexDetector(t *testing.T, home string) Detector {
	t.Helper()
	d := NewOS(home)
	var err error
	d.Registry, err = clients.NewRegistry(codex.New())
	if err != nil {
		t.Fatal(err)
	}
	d.LookPath = func(string) (string, error) { return "", errors.New("absent") }
	return d
}

func TestCodexHomeSelection(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home, project := filepath.Join(base, "home"), filepath.Join(base, "project")
	for _, p := range []string{home, project, filepath.Join(home, ".codex"), filepath.Join(project, "profile")} {
		if err := os.MkdirAll(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(project)
	for _, tc := range []struct {
		name, value, want string
		absent            bool
	}{
		{name: "absent", absent: true, want: filepath.Join(home, ".codex")},
		{name: "empty", want: filepath.Join(home, ".codex")},
		{name: "absolute", value: filepath.Join(project, "profile"), want: filepath.Join(project, "profile")},
		{name: "relative", value: "./profile", want: filepath.Join(project, "profile")},
		{name: "missing nested", value: "new/nested", want: filepath.Join(project, "new", "nested")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CODEX_HOME", tc.value)
			if tc.absent {
				if err := os.Unsetenv("CODEX_HOME"); err != nil {
					t.Fatal(err)
				}
			}
			d := codexDetector(t, home)
			if !tc.absent && d.Environment["CODEX_HOME"] != tc.value {
				t.Fatal("snapshot lost CODEX_HOME")
			}
			t.Setenv("CODEX_HOME", "changed-after-snapshot")
			t.Chdir(home)
			got, err := d.Detect(context.Background())
			if err != nil || len(got) != 1 || got[0].ConfigRoot != tc.want {
				t.Fatalf("detection = %+v, %v; want %s", got, err, tc.want)
			}
		})
	}
}

func TestCodexHomeSymlinksAndInvalidPaths(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(base, "target")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(base, "file")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{" ", "\t", "bad\nroot", "bad\x00root", file, filepath.Join(file, "child"), string(filepath.Separator)} {
		d := codexDetector(t, base)
		d.Environment["CODEX_HOME"] = bad
		assertInvalidCodexDetection(t, d)
	}
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(target, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	for _, suffix := range []string{"", "missing/nested"} {
		d := codexDetector(t, base)
		d.Environment["CODEX_HOME"] = filepath.Join(alias, suffix)
		got, err := d.Detect(context.Background())
		if err != nil || got[0].ConfigRoot != filepath.Join(target, suffix) {
			t.Fatalf("alias: %+v %v", got, err)
		}
	}
	for _, name := range []string{"dangling", "loop"} {
		link := filepath.Join(base, name)
		dest := filepath.Join(base, "absent")
		if name == "loop" {
			dest = link
		}
		if err := os.Symlink(dest, link); err != nil {
			t.Fatal(err)
		}
		d := codexDetector(t, base)
		d.Environment["CODEX_HOME"] = link
		assertInvalidCodexDetection(t, d)
	}
	d := codexDetector(t, base)
	d.Environment["CODEX_HOME"] = target
	d.Lstat = func(string) (os.FileInfo, error) { return nil, os.ErrPermission }
	assertInvalidCodexDetection(t, d)
}

func assertInvalidCodexDetection(t *testing.T, detector Detector) {
	t.Helper()
	got, err := detector.Detect(context.Background())
	if err != nil || len(got) != 1 || got[0].Status != domain.DetectionNotDetected || got[0].DetectionError == nil || got[0].ConfigRoot != "" || got[0].ExecutablePath != "" {
		t.Fatalf("invalid Codex profile gained authority or interrupted detection: %+v, %v", got, err)
	}
}

func TestCodexVersionProcessUsesFrozenProfile(t *testing.T) {
	executable := copyTestExecutable(t, "codex-profile-version-probe")
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home, project, profile := filepath.Join(base, "home"), filepath.Join(base, "project"), filepath.Join(base, "project", "profile")
	for _, p := range []string{filepath.Join(home, ".codex"), profile} {
		if err := os.MkdirAll(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	sentinel := filepath.Join(home, ".codex", "keep")
	if err := os.WriteFile(sentinel, []byte("default profile"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("CODEX_HOME", "profile")
	t.Chdir(project)
	d := codexDetector(t, home)
	d.LookPath = func(string) (string, error) { return executable, nil }
	t.Chdir(home)
	t.Setenv("CODEX_HOME", "wrong-after-snapshot")
	if _, err := d.Detect(context.Background()); err != nil {
		t.Fatal(err)
	}
	observation := filepath.Join(profile, "version-observation.json")
	if _, err := os.Stat(observation); !os.IsNotExist(err) {
		t.Fatal("read-only detection executed version")
	}
	got, err := d.DetectTargetsWithVersionProbe(context.Background(), []domain.ClientID{domain.ClientCodex})
	if err != nil || got[0].ConfigRoot != profile || got[0].Version != "1.2.3" {
		t.Fatalf("version detection: %+v %v", got, err)
	}
	body, err := os.ReadFile(observation)
	if err != nil {
		t.Fatal(err)
	}
	var child struct {
		CWD string
		Env []string
	}
	if err := json.Unmarshal(body, &child); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, item := range child.Env {
		if strings.HasPrefix(item, "CODEX_HOME=") {
			count++
			if item != "CODEX_HOME="+profile {
				t.Fatal(item)
			}
		}
		if strings.HasPrefix(item, "HOME=") || strings.HasPrefix(item, "USERPROFILE=") {
			t.Fatalf("ambient profile leaked: %s", item)
		}
	}
	if count != 1 || child.CWD == project || child.CWD == home {
		t.Fatalf("child: %+v", child)
	}
	if body, err := os.ReadFile(sentinel); err != nil || string(body) != "default profile" {
		t.Fatal("default sentinel changed")
	}
	entries, err := os.ReadDir(filepath.Dir(sentinel))
	if err != nil || len(entries) != 1 {
		t.Fatal("default profile mutated")
	}
	want := []string{"CODEX_HOME=" + profile}
	var seen []string
	d.ProbeVersionWithEnvironment = func(_ context.Context, _ string, env []string) (string, error) { seen = env; return "", nil }
	if _, err := d.DetectWithVersionProbe(context.Background()); err != nil || !reflect.DeepEqual(seen, want) {
		t.Fatalf("probe env %v, %v", seen, err)
	}
}

// Installing a test/host probe must never accidentally fall through to an OS
// process. Profile-aware clients need the new callback to carry their root.
func TestLegacyVersionProbeNeverFallsThroughToOS(t *testing.T) {
	for _, adapter := range []clients.Adapter{codex.New(), cursor.New()} {
		t.Run(string(adapter.ID()), func(t *testing.T) {
			root := t.TempDir()
			d := NewOS(root)
			d.Environment = map[string]string{}
			var err error
			d.Registry, err = clients.NewRegistry(adapter)
			if err != nil {
				t.Fatal(err)
			}
			d.LookPath = func(string) (string, error) { return filepath.Join(root, "client"), nil }
			legacyCalls := 0
			d.ProbeVersion = func(context.Context, string) (string, error) {
				legacyCalls++
				return "1.2.3", nil
			}
			d.ProbeVersionWithEnvironment = func(context.Context, string, []string) (string, error) {
				t.Fatal("injected legacy probe fell through to the OS probe")
				return "", nil
			}
			got, err := d.DetectWithVersionProbe(context.Background())
			if err != nil || len(got) != 1 {
				t.Fatalf("detect: %+v %v", got, err)
			}
			wantCalls, wantVersion := 1, "1.2.3"
			if adapter.ID() == domain.ClientCodex {
				wantCalls, wantVersion = 0, ""
			}
			if legacyCalls != wantCalls || got[0].Version != wantVersion {
				t.Fatalf("legacy calls=%d; version=%q", legacyCalls, got[0].Version)
			}
		})
	}
}

func TestWindowsCodexHomeRejectsDriveRelativeAndRootedPaths(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows path semantics")
	}
	for _, root := range []string{`C:profile`, `\profile`, `/profile`, `C:\`} {
		d := codexDetector(t, t.TempDir())
		d.Environment["CODEX_HOME"] = root
		if got, err := d.Detect(context.Background()); err == nil {
			t.Fatalf("ambiguous Windows root %q accepted: %+v", root, got)
		}
	}
}
