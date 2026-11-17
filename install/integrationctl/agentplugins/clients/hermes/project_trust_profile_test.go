package hermes

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestHermesProjectTrustRefusesEnvironmentMismatchBeforeNativeOrLock(t *testing.T) {
	for _, name := range []string{"explicit foreign home", "different default home", "default suffix", "invalid suffix", "relative explicit home", "explicit home is a file"} {
		t.Run(name, func(t *testing.T) {
			f := newHermesTrustFixture(t)
			other := filepath.Join(filepath.Dir(f.home), "other-home")
			if err := os.Mkdir(other, 0o700); err != nil {
				t.Fatal(err)
			}
			f.write(t, filepath.Join(f.profile, "config.yaml"), "theme: selected\n")
			f.write(t, filepath.Join(other, "config.yaml"), "theme: foreign\n")
			switch name {
			case "explicit foreign home":
				f.grant.environ = append(f.grant.environ, "HERMES_HOME="+other)
			case "different default home":
				f.grant.environ = append(f.grant.environ, "HERMES_HOME=", "HOME="+other, "USERPROFILE="+other)
			case "default suffix":
				f.grant.environ = append(f.grant.environ, "HERMES_HOME=", "HERMES_DATA_DIR_SUFFIX=-foreign")
			case "invalid suffix":
				f.grant.environ = append(f.grant.environ, "HERMES_HOME=", "HERMES_DATA_DIR_SUFFIX=/../foreign")
			case "relative explicit home":
				f.grant.environ = append(f.grant.environ, "HERMES_HOME=relative")
			case "explicit home is a file":
				f.grant.environ = append(f.grant.environ, "HERMES_HOME="+filepath.Join(other, "config.yaml"))
			}
			locks := 0
			f.grant.kernel = nativeconfig.NewWithLockAcquirer(func(nativeconfig.Paths, nativeconfig.Codec) (func() error, error) {
				locks++
				return func() error { return nil }, nil
			})
			report, err := f.grant.run(context.Background())
			if err != nil || report.granted || report.attempted || len(f.cli.calls) != 0 || locks != 0 ||
				!strings.Contains(report.reason, "environment selects a different or unavailable profile") {
				t.Fatalf("mismatched authority launched a command or took a lock: report=%+v err=%v calls=%v locks=%d", report, err, f.cli.calls, locks)
			}
			assertHermesTrustFile(t, filepath.Join(f.profile, "config.yaml"), "theme: selected\n")
			assertHermesTrustFile(t, filepath.Join(other, "config.yaml"), "theme: foreign\n")
			if !slices.Equal(f.cli.entries(), []string{"/srv/prior"}) {
				t.Fatalf("refusal changed trust: %v", f.cli.entries())
			}
		})
	}
}

func TestHermesProjectTrustUsesMatchingEnvironmentWithoutRewritingIt(t *testing.T) {
	for _, tc := range []struct {
		name, suffix string
		explicit     bool
		appdata      bool
	}{
		{name: "explicit home", explicit: true},
		{name: "platform default"},
		{name: "redirected appdata", appdata: true},
		{name: "default suffix", suffix: "-Canary_1", appdata: true},
		{name: "explicit home overrides suffix", explicit: true, suffix: "/ignored", appdata: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newHermesTrustFixture(t)
			local, explicit := "", ""
			profile := filepath.Join(f.home, ".hermes"+tc.suffix)
			if tc.appdata {
				local = filepath.Join(filepath.Dir(f.home), "appdata")
			}
			if runtime.GOOS == "windows" {
				base := local
				if base == "" {
					base = filepath.Join(f.home, "AppData", "Local")
				}
				profile = filepath.Join(base, "hermes"+tc.suffix)
			}
			if tc.explicit {
				profile = filepath.Join(filepath.Dir(f.home), "explicit-profile")
				explicit = profile
			}
			if err := os.MkdirAll(profile, 0o700); err != nil {
				t.Fatal(err)
			}
			f.grant.profile, f.cli.configPath = profile, filepath.Join(profile, "config.yaml")
			f.grant.environ = append(f.grant.environ, "HERMES_HOME="+explicit, "LOCALAPPDATA="+local, "HERMES_DATA_DIR_SUFFIX="+tc.suffix)
			report, err := f.grant.run(context.Background())
			if err != nil || !report.granted || !report.attempted || f.cli.trustCalls() != 1 ||
				!slices.Equal(f.cli.entries(), []string{"/srv/prior", f.project}) {
				t.Fatalf("matching authority refused: report=%+v err=%v calls=%v", report, err, f.cli.calls)
			}
			if len(f.cli.environments) != len(f.cli.calls) || len(f.cli.calls) == 0 {
				t.Fatal("missing actual command environments")
			}
			for _, environment := range f.cli.environments {
				if !slices.Equal(environment, f.grant.environ) {
					t.Fatalf("native command environment was rewritten: got=%v want=%v", environment, f.grant.environ)
				}
			}
		})
	}
}

func TestHermesProjectActivationKeepsPreparedFilesWhenBareProfileDiffers(t *testing.T) {
	f := newHermesTrustFixture(t)
	request, env := hermesTrustActivation(t, f.project, f.cli, f.profile)
	request.Plan.Authentication = domain.AuthenticationNotChecked
	other := filepath.Join(filepath.Dir(f.home), "bare-hermes")
	if err := os.Mkdir(other, 0o700); err != nil {
		t.Fatal(err)
	}
	f.write(t, filepath.Join(f.profile, "config.yaml"), "theme: selected\n")
	f.write(t, filepath.Join(other, "config.yaml"), "theme: foreign\n")
	t.Setenv("HERMES_HOME", other)
	outcome, err := New().Activate(context.Background(), env, request)
	if err != nil || outcome.Activation != domain.ActivationPrepared || outcome.Verification != domain.VerificationInstalled ||
		outcome.Policy != domain.PolicyAllowed || outcome.Authentication != domain.AuthenticationNotChecked ||
		outcome.ActivationAttested || outcome.AuthenticationAttested || outcome.NativeEffect != domain.NativeEffectCommitted || len(f.cli.calls) != 0 {
		t.Fatalf("profile refusal lost installed files or claimed native activation: outcome=%+v err=%v calls=%v", outcome, err, f.cli.calls)
	}
	if !reflect.DeepEqual(outcome.NativeObjects, request.Delivery.NativeObjects) {
		t.Fatal("profile refusal lost native ownership")
	}
	staged, err := os.ReadFile(filepath.Join(request.Delivery.ActivePath, "skills", "document", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	assertHermesTrustFile(t, filepath.Join(request.Plan.NativeRegistryRoot, "skills", "document", "SKILL.md"), string(staged))
	assertHermesTrustFile(t, filepath.Join(request.Plan.NativeRegistryRoot, "skills", "not-ours", "SKILL.md"), "foreign\n")
	assertHermesTrustFile(t, filepath.Join(f.profile, "config.yaml"), "theme: selected\n")
	assertHermesTrustFile(t, filepath.Join(other, "config.yaml"), "theme: foreign\n")
	if !slices.Equal(f.cli.entries(), []string{"/srv/prior"}) {
		t.Fatal("profile refusal changed native trust")
	}
	message := strings.Join(outcome.UserActions, "\n")
	if !strings.Contains(message, "UAP did not change Hermes project trust: the user's Hermes environment selects a different or unavailable profile") ||
		!strings.Contains(message, "run `hermes skills trust` inside it") || strings.Contains(message, other) ||
		!slices.Contains(outcome.LocalActions, hermesTrustCommandAction(f.project)) {
		t.Fatalf("profile refusal lacks safe manual guidance: %v %v", outcome.UserActions, outcome.LocalActions)
	}
}

func assertHermesTrustFile(t *testing.T, path, want string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil || string(body) != want {
		t.Fatalf("file %q changed: got=%q err=%v want=%q", path, body, err, want)
	}
}
