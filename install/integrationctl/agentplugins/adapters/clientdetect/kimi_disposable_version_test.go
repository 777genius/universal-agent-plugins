package clientdetect

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/kimi"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// The upstream Kimi 2.3.2 --version bootstraps cache workers. Prove both
// success and failure keep those writes outside the selected user profile.
func TestKimiTargetedVersionProbeUsesDisposableProfile(t *testing.T) {
	for _, test := range []struct {
		name string
		fail bool
	}{{"success", false}, {"probe-failed", true}} {
		t.Run(test.name, func(t *testing.T) {
			base := t.TempDir()
			home := filepath.Join(base, "home")
			selected := filepath.Join(home, ".kimi-code")
			if err := os.MkdirAll(selected, 0700); err != nil {
				t.Fatal(err)
			}
			canary := filepath.Join(selected, "foreign-config.toml")
			if err := os.WriteFile(canary, []byte("untouched\n"), 0600); err != nil {
				t.Fatal(err)
			}
			executable := filepath.Join(base, "kimi")
			if runtime.GOOS == "windows" {
				executable += ".exe"
			}
			if err := os.WriteFile(executable, []byte("inert fixture"), 0700); err != nil {
				t.Fatal(err)
			}
			registry, err := clients.NewRegistry(kimi.New())
			if err != nil {
				t.Fatal(err)
			}
			detector := NewOS(home)
			detector.Registry = registry
			detector.LookPath = func(binary string) (string, error) {
				if binary == "kimi" {
					return executable, nil
				}
				return "", os.ErrNotExist
			}
			calls, scratch := 0, ""
			detector.ProbeVersionWithEnvironment = func(_ context.Context, command string, environment []string) (string, error) {
				calls++
				if command != executable {
					t.Fatal("version probe used a different executable")
				}
				values := map[string]string{}
				for _, entry := range environment {
					key, value, ok := strings.Cut(entry, "=")
					if ok {
						values[key] = value
					}
				}
				scratch = values["KIMI_CODE_HOME"]
				cache := values["KIMI_CODE_CACHE_DIR"]
				if scratch == "" || scratch == selected || cache != filepath.Join(scratch, "cache") || values["KIMI_CODE_NO_AUTO_UPDATE"] != "1" {
					t.Fatalf("version environment leaked selected profile: %+v", environment)
				}
				if err := os.MkdirAll(cache, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(cache, "native-bootstrap"), []byte("owned by disposable probe"), 0600); err != nil {
					t.Fatal(err)
				}
				if test.fail {
					return "", errors.New("native bootstrap failed")
				}
				return "kimi-code 2.3.2", nil
			}
			got, err := detector.DetectTargetsWithVersionProbe(context.Background(), []domain.ClientID{domain.ClientKimi})
			if err != nil || len(got) != 1 || calls != 1 {
				t.Fatalf("targeted probe result=%+v calls=%d err=%v", got, calls, err)
			}
			version := "2.3.2"
			if test.fail {
				version = ""
			}
			if got[0].ConfigRoot != selected || got[0].ExecutablePath != executable || got[0].Version != version {
				t.Fatalf("lost original host identity/version: %+v", got[0])
			}
			if _, err := os.Lstat(scratch); !os.IsNotExist(err) {
				t.Fatalf("disposable native cache survived probe: %v", err)
			}
			if body, err := os.ReadFile(canary); err != nil || string(body) != "untouched\n" {
				t.Fatal("version probe modified original profile")
			}
			if _, err := os.Lstat(filepath.Join(selected, "cache")); !os.IsNotExist(err) {
				t.Fatal("version probe initialized original user's cache")
			}
		})
	}
}
