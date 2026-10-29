package clientdetect

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func devinVersionTestHome(t *testing.T) string {
	t.Helper()
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return home
}

func devinVersionEnvironmentForDetector(t *testing.T, detector Detector) ([]string, error) {
	t.Helper()
	profile, ok := clients.As[clients.HostVersionProbeEnvironment](detector.Registry, domain.ClientWindsurf)
	if !ok {
		t.Fatal("selected adapter lacks its original-host version environment capability")
	}
	client := domain.DetectedClient{ClientID: domain.ClientWindsurf, ExecutablePath: filepath.Join(detector.HomeDir, "devin")}
	return profile.VersionProbeEnvironmentForHost(detector.host(), client)
}

func TestDevinVersionEnvironmentPreservesOriginalHostAndLegacyAuthority(t *testing.T) {
	t.Setenv("HOME", "ambient-home-canary")
	t.Setenv("HTTPS_PROXY", "ambient-proxy-canary")
	t.Setenv("DEVIN_API_KEY", "ambient-auth-canary")
	for _, legacy := range []bool{false, true} {
		home := devinVersionTestHome(t)
		executable := filepath.Join(home, "bin", "devin")
		detector := testDetector(home, map[string]string{"devin": executable})
		config := filepath.Join(home, "original-xdg")
		detector.Environment = map[string]string{"XDG_CONFIG_HOME": config, "DEVIN_API_KEY": "held-auth-canary", "NODE_OPTIONS": "held-loader-canary"}
		if legacy {
			if err := os.MkdirAll(filepath.Join(home, ".codeium", "windsurf"), 0700); err != nil {
				t.Fatal(err)
			}
		}
		calls := 0
		detector.ProbeVersionWithEnvironment = func(_ context.Context, path string, env []string) (string, error) {
			calls++
			if path != executable || !reflect.DeepEqual(env, []string{"HOME=" + home, "XDG_CONFIG_HOME=" + config}) {
				t.Fatalf("version probe lost original host authority: %s %v", path, env)
			}
			return "devin 3000.11.205 (6dea39440ae3)\n", nil
		}
		plain, err := detector.Detect(context.Background())
		if err != nil || calls != 0 || clientOf(plain, domain.ClientWindsurf).Version != "" {
			t.Fatal("read-only detection executed a version child")
		}
		observed, err := detector.DetectTargetsWithVersionProbe(context.Background(), []domain.ClientID{domain.ClientWindsurf})
		if err != nil || calls != 1 {
			t.Fatalf("selected Devin probe failed: %v", err)
		}
		before, after := clientOf(plain, domain.ClientWindsurf), clientOf(observed, domain.ClientWindsurf)
		want := before
		want.Version = "3000.11.205"
		if !reflect.DeepEqual(after, want) || before.ExecutablePath != executable || (!legacy && before.ConfigRoot != "") {
			t.Fatalf("Devin version observation changed empty/legacy authority: before=%+v after=%+v", before, after)
		}
		if _, err := os.Lstat(config); !os.IsNotExist(err) {
			t.Fatal("version environment created the absent explicit XDG directory")
		}
	}
}

func TestDevinVersionEnvironmentRequiresEnvironmentCapableProbe(t *testing.T) {
	home := devinVersionTestHome(t)
	detector := testDetector(home, map[string]string{"devin": filepath.Join(home, "devin")})
	calls := 0
	detector.ProbeVersion = func(context.Context, string) (string, error) {
		calls++
		return "3000.11.205", nil
	}
	detector.ProbeVersionWithEnvironment = func(context.Context, string, []string) (string, error) {
		calls++
		return "3000.11.205", nil
	}
	rows, err := detector.DetectTargetsWithVersionProbe(context.Background(), []domain.ClientID{domain.ClientWindsurf})
	if err != nil || calls != 0 || clientOf(rows, domain.ClientWindsurf).Version != "" {
		t.Fatal("legacy callback bypassed original environment or fell back to another child")
	}
}

func TestDevinVersionEnvironmentWindowsKeysAreOriginalAndFinite(t *testing.T) {
	home := devinVersionTestHome(t)
	detector := testDetector(home, nil)
	detector.GOOS = "windows"
	detector.Environment = map[string]string{"APPDATA": filepath.Join(home, "roaming"), "LOCALAPPDATA": filepath.Join(home, "local"), "XDG_CONFIG_HOME": "irrelevant-canary", "DEVIN_API_KEY": "auth-canary"}
	env, err := devinVersionEnvironmentForDetector(t, detector)
	want := []string{"HOME=" + home, "USERPROFILE=" + home, "APPDATA=" + detector.Environment["APPDATA"], "LOCALAPPDATA=" + detector.Environment["LOCALAPPDATA"]}
	if err != nil || !reflect.DeepEqual(env, want) {
		t.Fatalf("Windows original environment differs: %v (%v)", env, err)
	}
	delete(detector.Environment, "APPDATA")
	if _, err := devinVersionEnvironmentForDetector(t, detector); err == nil {
		t.Fatal("missing original Windows APPDATA admitted")
	}
}

func TestDevinVersionEnvironmentRejectsUnsafeOriginalFacts(t *testing.T) {
	for _, kind := range []string{"missing-home", "relative-xdg", "unclean-xdg", "newline-xdg", "missing-parent", "file-xdg", "alias-xdg", "alias-home", "unsupported-platform"} {
		t.Run(kind, func(t *testing.T) {
			home := devinVersionTestHome(t)
			detector := testDetector(home, map[string]string{"devin": filepath.Join(home, "devin")})
			value := filepath.Join(home, "xdg")
			switch kind {
			case "missing-home":
				detector.HomeDir = filepath.Join(home, "missing-home")
			case "relative-xdg":
				value = "relative"
			case "unclean-xdg":
				value += string(filepath.Separator)
			case "newline-xdg":
				value += "\nprivate"
			case "missing-parent":
				value = filepath.Join(home, "missing-parent", "xdg")
			case "file-xdg":
				if err := os.WriteFile(value, []byte("foreign"), 0600); err != nil {
					t.Fatal(err)
				}
			case "alias-xdg", "alias-home":
				if err := os.Symlink(home, value); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
				if kind == "alias-home" {
					detector.HomeDir = value
				}
			case "unsupported-platform":
				detector.GOOS = "unsupported"
			}
			detector.Environment["XDG_CONFIG_HOME"] = value
			calls := 0
			detector.ProbeVersionWithEnvironment = func(context.Context, string, []string) (string, error) {
				calls++
				return "3000.11.205", nil
			}
			rows, err := detector.DetectTargetsWithVersionProbe(context.Background(), []domain.ClientID{domain.ClientWindsurf})
			if err != nil || calls != 0 || clientOf(rows, domain.ClientWindsurf).DetectionError == nil {
				t.Fatal("unsafe original host facts reached a version child")
			}
		})
	}
}

func TestDevinVersionEnvironmentUsesInjectedFilesystemProbes(t *testing.T) {
	home := devinVersionTestHome(t)
	detector := testDetector(home, nil)
	boundary := errors.New("injected original-home observation failure")
	detector.Lstat = func(path string) (os.FileInfo, error) {
		if path == home {
			return nil, boundary
		}
		return os.Lstat(path)
	}
	if _, err := devinVersionEnvironmentForDetector(t, detector); !errors.Is(err, boundary) {
		t.Fatalf("environment validation ignored injected host probes: %v", err)
	}
}
