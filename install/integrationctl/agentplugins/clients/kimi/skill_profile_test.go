package kimi

import (
	"context"
	"errors"
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

func TestKimiAuthenticatedFirstTokenMustSurviveCompletedExchange(t *testing.T) {
	for _, change := range []string{"removed", "replaced"} {
		t.Run(change, func(t *testing.T) {
			f := newKimiCompletionFixture(t, "2.5.2")
			f.afterExchange = func() {
				path := filepath.Join(f.request.Client.ConfigRoot, "server.token")
				if change == "removed" {
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
				} else {
					write(t, path, strings.Repeat("B", 43))
				}
			}
			out, err := New().Activate(t.Context(), f.env, f.request)
			if err == nil || out.Activation != domain.ActivationPrepared || out.NativeEffect != domain.NativeEffectUncertain || f.queries != 1 {
				t.Fatal("authenticated first token changed after closed exchange without refusal", out, err)
			}
			if len(f.calls) == 0 || f.calls[len(f.calls)-1] != "POST /api/v1/shutdown" {
				t.Fatal("token test did not complete ordinary authenticated shutdown", f.calls)
			}
		})
	}
}

func TestKimiFinalValidSkillParseCannotHideCancellationOrProfileChange(t *testing.T) {
	for _, change := range []string{"cancel", "profile"} {
		t.Run(change, func(t *testing.T) {
			f := newKimiCompletionFixture(t, "2.5.2")
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			parse := f.env.ParseSkill
			finalParses := 0
			f.env.ParseSkill = func(ctx context.Context, name string, body []byte) (domain.Skill, error) {
				metadata, err := parse(ctx, name, body)
				if len(f.calls) > 0 && f.calls[len(f.calls)-1] == "POST /api/v1/shutdown" {
					finalParses++
					if change == "cancel" {
						cancel()
					} else {
						write(t, filepath.Join(f.request.Client.ConfigRoot, "config.toml"), "telemetry = true\n")
					}
				}
				return metadata, err
			}
			out, err := New().Activate(ctx, f.env, f.request)
			if err == nil || out.Activation != domain.ActivationPrepared || finalParses != 1 || f.queries != 1 {
				t.Fatal("final valid parse escaped cancellation/profile refusal", out, err, finalParses)
			}
			if change == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatal("final parse cancellation was not propagated", err)
			}
			if change == "profile" && out.NativeEffect != domain.NativeEffectUncertain {
				t.Fatal("last-parse profile mutation lost uncertainty", out, err)
			}
		})
	}
}
