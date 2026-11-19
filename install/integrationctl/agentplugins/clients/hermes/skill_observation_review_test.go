package hermes

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestHermesSkillObservationSelectsCWDOverInheritedTerminal(t *testing.T) {
	for _, scope := range []domain.InstallScope{domain.ScopeProject, domain.ScopeUser} {
		t.Run(string(scope), func(t *testing.T) {
			request, env, doc := hermesObservationFixture(t, scope)
			foreign := filepath.Join(t.TempDir(), "another-project")
			if err := os.Mkdir(foreign, 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("TERMINAL_CWD", foreign)
			t.Setenv("HERMES_COMPLETION_RETAIN", "unchanged")
			observe := hermesObservationRunner(t, request, doc, nil)
			env.Runner = hermesSkillRunner(func(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
				count := 0
				for _, item := range command.Env {
					if strings.HasPrefix(item, "TERMINAL_CWD=") {
						count++
						if item != "TERMINAL_CWD="+command.Dir {
							t.Fatal("inherited terminal directory redirected observation")
						}
					}
				}
				if count != 1 || !slices.Contains(command.Env, "HERMES_COMPLETION_RETAIN=unchanged") {
					t.Fatal("child selection changed independent environment values")
				}
				return observe(ctx, command)
			})
			outcome, err := completeHermesSkills(t.Context(), env, request, hermesPreparedSkillOutcome(), hermesCompletedAction)
			if err != nil || outcome.Activation != domain.ActivationActive || os.Getenv("TERMINAL_CWD") != foreign {
				t.Fatal("selected catalog did not complete without changing the caller", outcome, err)
			}
		})
	}
}

func TestHermesSkillObservationRequiresInjectedMetadataAuthority(t *testing.T) {
	for _, missing := range []string{"parser", "snapshots"} {
		t.Run(missing, func(t *testing.T) {
			request, env, _ := hermesObservationFixture(t, domain.ScopeUser)
			if missing == "parser" {
				env.ParseSkill = nil
			} else {
				env.Snapshots = nil
			}
			env.Runner = hermesSkillRunner(func(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
				t.Fatal("missing metadata authority started a native command")
				return legacyports.CommandResult{}, nil
			})
			outcome, err := completeHermesSkills(t.Context(), env, request, hermesPreparedSkillOutcome(), hermesCompletedAction)
			if err != nil || outcome.Activation != domain.ActivationPrepared {
				t.Fatal("missing metadata authority qualified native completion", outcome, err)
			}
		})
	}
}

func TestHermesSkillTextDigestMatchesNativeTextDecoding(t *testing.T) {
	plain := []byte("---\nname: owned\ndescription: Native text.\n---\nInstructions.\n")
	wanted := hermesSkillTextDigest(plain)
	for _, body := range [][]byte{
		append([]byte("\xef\xbb\xbf"), plain...),
		[]byte(strings.ReplaceAll(string(plain), "\n", "\r\n")),
		[]byte(strings.ReplaceAll(string(plain), "\n", "\r")),
	} {
		if hermesSkillTextDigest(body) != wanted {
			t.Fatal("owned text differs from pinned Hermes utf-8-sig/newline decoding")
		}
	}
	if hermesSkillTextDigest(append(plain, 'x')) == wanted {
		t.Fatal("different evaluated text retained the owned digest")
	}
}
