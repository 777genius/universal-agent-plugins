package hermes

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
)

func TestHermesNativeDefaultProfileAcrossPlatforms(t *testing.T) {
	t.Parallel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "home")
	legacy := filepath.Join(home, ".hermes")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(legacy, "config.yaml")
	const foreign = "model: keep-legacy-profile\n"
	if err := os.WriteFile(marker, []byte(foreign), 0o600); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(root, "redirected-appdata")
	explicit := filepath.Join(root, "selected-profile")
	for _, tc := range []struct {
		name string
		goos string
		env  map[string]string
		want string
	}{
		{"linux default ignores Windows environment", "linux", map[string]string{"LOCALAPPDATA": local}, legacy},
		{"macOS default", "darwin", nil, legacy},
		{"Windows local appdata beats existing legacy", "windows", map[string]string{"LOCALAPPDATA": local}, filepath.Join(local, "hermes")},
		{"Windows trims local appdata", "windows", map[string]string{"LOCALAPPDATA": " \t" + local + " \t"}, filepath.Join(local, "hermes")},
		{"Windows missing local appdata", "windows", nil, filepath.Join(home, "AppData", "Local", "hermes")},
		{"Windows empty local appdata", "windows", map[string]string{"LOCALAPPDATA": " \t"}, filepath.Join(home, "AppData", "Local", "hermes")},
		{"Windows literal suffix", "windows", map[string]string{"LOCALAPPDATA": local, "HERMES_DATA_DIR_SUFFIX": "-Canary_1"}, filepath.Join(local, "hermes-Canary_1")},
		{"POSIX literal suffix", "linux", map[string]string{"HERMES_DATA_DIR_SUFFIX": "-Canary_1"}, filepath.Join(home, ".hermes-Canary_1")},
		{"explicit home overrides default and suffix", "windows", map[string]string{"LOCALAPPDATA": local, "HERMES_HOME": explicit, "HERMES_DATA_DIR_SUFFIX": "/ignored"}, explicit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			host := clients.NewHost(clients.HostProbes{
				HomeDir: home, WorkingDir: root, GOOS: tc.goos, Environment: tc.env,
				Lstat: os.Lstat, EvalSymlinks: filepath.EvalSymlinks,
			})
			got := New().DetectSurfaces(host)
			if got.Err != nil || got.ConfigRoot != tc.want {
				t.Fatalf("native profile = %q, error %v; want %q", got.ConfigRoot, got.Err, tc.want)
			}
			if tc.want != legacy {
				if _, err := os.Stat(tc.want); !os.IsNotExist(err) {
					t.Fatalf("read-only detection created a profile: %v", err)
				}
			}
			body, err := os.ReadFile(marker)
			if err != nil || string(body) != foreign {
				t.Fatalf("detection migrated or changed the legacy profile: %q, %v", body, err)
			}
		})
	}
}

func TestHermesInvalidProfileSelectionDoesNotFallBack(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		env  map[string]string
	}{
		{"relative local appdata", map[string]string{"LOCALAPPDATA": "../appdata"}},
		{"relative explicit home", map[string]string{"HERMES_HOME": "../profile"}},
		{"suffix escapes directory", map[string]string{"HERMES_DATA_DIR_SUFFIX": "/../../other"}},
		{"suffix aliases a Windows path", map[string]string{"HERMES_DATA_DIR_SUFFIX": "."}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			probes := 0
			host := clients.NewHost(clients.HostProbes{
				HomeDir: t.TempDir(), GOOS: "windows", Environment: tc.env,
				Lstat: func(string) (os.FileInfo, error) {
					probes++
					return nil, os.ErrNotExist
				},
			})
			got := New().DetectSurfaces(host)
			if got.Err == nil || got.ConfigRoot != "" || probes != 0 {
				t.Fatalf("invalid authority was probed or redirected: %+v; probes %d", got, probes)
			}
		})
	}
}
