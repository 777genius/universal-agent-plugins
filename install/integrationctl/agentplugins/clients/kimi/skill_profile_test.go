package kimi

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestKimiFirstTokenMayOnlyHardenSameProfileDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX directory-mode transitions are unavailable on Windows; ordinary completion tests still exercise its native token policy")
	}
	for _, scenario := range []struct {
		name                 string
		before, after        os.FileMode
		existingToken, ready bool
	}{{"fresh-native-hardening", 0755, 0700, false, true}, {"permission-widening", 0700, 0755, false, false}, {"existing-token-unexpected-change", 0755, 0700, true, false}} {
		t.Run(scenario.name, func(t *testing.T) {
			f := newKimiCompletionFixture(t, "2.5.2")
			profile := f.request.Client.ConfigRoot
			if scenario.existingToken {
				write(t, filepath.Join(profile, "server.token"), strings.Repeat("A", 43))
			}
			if err := os.Chmod(profile, scenario.before); err != nil {
				t.Fatal(err)
			}
			f.change = func(request *http.Request, _ map[string]any) {
				if strings.HasSuffix(request.URL.Path, "/meta") {
					if err := os.Chmod(profile, scenario.after); err != nil {
						t.Fatal(err)
					}
				}
			}
			out, err := New().Activate(t.Context(), f.env, f.request)
			if f.queries != 1 || (out.Activation == domain.ActivationActive) != scenario.ready {
				t.Fatal("profile mode transition violated native first-token policy", out, err)
			}
			if scenario.ready && err != nil {
				t.Fatal(err)
			}
			if !scenario.ready && (err == nil || out.NativeEffect != domain.NativeEffectUncertain) {
				t.Fatal("unexplained profile mutation lost uncertainty", out, err)
			}
		})
	}
}
