package agentpluginscli

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func TestSingleAgentAddFailureIsRecordedLoggedAndRetried(t *testing.T) {
	cmd, _, errOut := retryTestCommand(t)
	app := App{Terminal: true, InstallLogDir: filepath.Join(t.TempDir(), "logs")}
	var asked []string
	app.Prompter = fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
		asked = append(asked, "retry")
		return prompt.ConfirmationResult{Accepted: true}, nil
	}}
	opts := &options{format: "human"}
	boom := installer.ErrTargetFactsUnavailable
	app.recordSingleAdd(cmd, opts, domain.ClientCodex, "demo", boom)
	if opts.installResult == nil || len(opts.installResult.failedAgents()) != 1 || opts.installResult.LogFile == "" {
		t.Fatalf("failure not recorded: %+v", opts.installResult)
	}
	if !strings.Contains(errOut.String(), "Technical details: "+opts.installResult.LogFile) {
		t.Fatalf("log path not shown: %s", errOut.String())
	}
	var rerun string
	err := retryFailedAgents(cmd, app, opts, boom, func(target string, _ []string) error { rerun = target; return nil })
	if err != nil || rerun != "codex" || len(asked) != 1 {
		t.Fatalf("rerun=%q asked=%v err=%v", rerun, asked, err)
	}
	// Success records nothing.
	clean := &options{format: "human"}
	app.recordSingleAdd(cmd, clean, domain.ClientCodex, "demo", nil)
	if clean.installResult != nil {
		t.Fatal("a successful add must not create a failure record")
	}
}

func TestSingleTargetAddHandsItsResultBackToTheCaller(t *testing.T) {
	outer := &options{target: "codex,claude", firstTreeDigest: "sha256:reviewed", retryAttempt: 1}
	want := &installOutcome{LogFile: "x"}
	handled, err := runSingleTargetAdd(App{}, outer, loadedPackage{}, domain.ClientCodex, true, func(selected *options) error {
		if selected == outer || selected.target != "codex" || selected.firstTreeDigest != outer.firstTreeDigest || selected.retryAttempt != outer.retryAttempt {
			t.Fatalf("single-target dispatch lost or aliased its reviewed options: %+v", selected)
		}
		selected.installResult = want
		return errors.New("failed")
	})
	if !handled || err == nil || outer.installResult != want || outer.target != "codex,claude" {
		t.Fatalf("the retry step must see the one-agent result: handled=%v err=%v result=%v", handled, err, outer.installResult)
	}
}

func TestConvergeRetryRerunsEverythingOutstandingAndRespectsTheGate(t *testing.T) {
	var asked []string
	app := App{Terminal: true, Prompter: fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
		asked = append(asked, "retry")
		return prompt.ConfirmationResult{Accepted: true}, nil
	}}}
	opts := &options{format: "human", installResult: &installOutcome{Agents: reportByAgent([]agentItem{
		failedItem("demo", "codex", errors.New("native command failed")),
		{Alias: "demo", ClientID: "claude", Status: "completed"},
	})}}
	cmd, _, _ := retryTestCommand(t)
	reruns := 0
	if err := retryConverge(cmd, app, opts, errors.New("failed"), func() error { reruns++; return nil }); err != nil || reruns != 1 || len(asked) != 1 || opts.retryAttempt != 1 {
		t.Fatalf("reruns=%d asked=%v attempt=%d err=%v", reruns, asked, opts.retryAttempt, err)
	}
	boom := errors.New("failed")
	opts.yes = true
	if err := retryConverge(cmd, app, opts, boom, func() error { t.Fatal("--yes must never prompt"); return nil }); !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
}

func TestRetryInstallsExactlyThePackageOfTheFirstAttempt(t *testing.T) {
	first := loadedPackage{}
	first.envelope.TreeDigest = "sha256:first"
	opts := &options{}
	if err := pinRetryPackage(opts, first); err != nil || opts.firstTreeDigest != "sha256:first" {
		t.Fatalf("first attempt must pin its package: %v %q", err, opts.firstTreeDigest)
	}
	retry := *opts
	if err := pinRetryPackage(&retry, first); err != nil {
		t.Fatalf("an unchanged package may be retried: %v", err)
	}
	changed := loadedPackage{}
	changed.envelope.TreeDigest = "sha256:edited"
	if err := pinRetryPackage(&retry, changed); err == nil {
		t.Fatal("a retry must refuse a package that changed since the first attempt")
	}
	if err := pinRetryPackage(&options{}, loadedPackage{}); err != nil {
		t.Fatalf("a package without a digest cannot be pinned: %v", err)
	}
}

func TestInstallRetryPreservesPromptFailuresAndDecline(t *testing.T) {
	for _, mode := range []string{"add", "converge"} {
		for _, tc := range []struct {
			name        string
			cause       error
			accepted    bool
			factory     bool
			unavailable bool
			original    bool
		}{
			{name: "closed input", cause: prompt.ErrPromptInputClosed},
			{name: "error wins over acceptance", cause: io.ErrUnexpectedEOF, accepted: true},
			{name: "factory error", cause: io.ErrShortWrite, factory: true},
			{name: "unavailable", cause: prompt.ErrPromptUnavailable, unavailable: true},
			{name: "declined", original: true},
			{name: "canceled", cause: fmt.Errorf("selection: %w", prompt.ErrPromptCanceled), accepted: true, original: true},
			{name: "factory canceled", cause: prompt.ErrPromptCanceled, factory: true, original: true},
		} {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				boom := errors.New("native installation failed")
				asked, reruns := 0, 0
				app := App{Terminal: true, Prompter: fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
					asked++
					return prompt.ConfirmationResult{Accepted: tc.accepted}, tc.cause
				}}}
				if tc.factory || tc.unavailable {
					app.Prompter = nil
				}
				if tc.factory {
					app.PromptFactory = func(io.Reader, io.Writer, io.Writer, bool, bool) (prompt.Prompter, io.Writer, error) {
						asked++
						return nil, nil, tc.cause
					}
				}
				report := &installOutcome{Agents: reportByAgent([]agentItem{failedItem("demo", "codex", boom)})}
				opts := &options{format: "human", installResult: report}
				cmd, _, errOut := retryTestCommand(t)
				var err error
				if mode == "add" {
					err = retryFailedAgents(cmd, app, opts, boom, func(string, []string) error { reruns++; return nil })
				} else {
					err = retryConverge(cmd, app, opts, boom, func() error { reruns++; return nil })
				}
				if !errors.Is(err, boom) || (!tc.original && !errors.Is(err, tc.cause)) || (tc.original && reflect.ValueOf(err) != reflect.ValueOf(boom)) {
					t.Fatalf("retry lost or changed a cause: got %v; original %v; prompt %v", err, boom, tc.cause)
				}
				wantAsked := 1
				if tc.unavailable {
					wantAsked = 0
				}
				if asked != wantAsked || reruns != 0 || opts.retryAttempt != 0 || opts.installResult != report || strings.Contains(errOut.String(), "Retrying") {
					t.Fatalf("failed/declined prompt changed the attempt: asked=%d reruns=%d attempt=%d stderr=%q", asked, reruns, opts.retryAttempt, errOut.String())
				}
			})
		}
	}
}

func TestInstallRetryAnnouncementFailurePreservesInstallationCause(t *testing.T) {
	for _, mode := range []string{"add", "converge"} {
		t.Run(mode, func(t *testing.T) {
			boom := errors.New("native installation failed")
			asked, reruns := 0, 0
			app := App{Terminal: true, Prompter: fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
				asked++
				return prompt.ConfirmationResult{Accepted: true}, nil
			}}}
			opts := &options{format: "human", installResult: &installOutcome{Agents: reportByAgent([]agentItem{failedItem("demo", "codex", boom)})}}
			cmd, _, _ := retryTestCommand(t)
			cmd.SetErr(targetNoticeWriter{err: io.ErrShortWrite})
			var err error
			if mode == "add" {
				err = retryFailedAgents(cmd, app, opts, boom, func(string, []string) error { reruns++; return nil })
			} else {
				err = retryConverge(cmd, app, opts, boom, func() error { reruns++; return nil })
			}
			if !errors.Is(err, boom) || !errors.Is(err, io.ErrShortWrite) || asked != 1 || reruns != 0 {
				t.Fatalf("announcement failure lost a cause or retried: err=%v asked=%d reruns=%d", err, asked, reruns)
			}
		})
	}
}

func TestInstallYesFlagDoesNotPromptToRetry(t *testing.T) {
	client := fixtureClient(t, domain.ClientCodex)
	client.ExecutablePath = filepath.Join(t.TempDir(), "codex")
	runner := &dependencyClaudeListing{root: client.ConfigRoot, failNextCodexAdd: true}
	f, source := newDependencyHarness(t, "", []domain.DetectedClient{client}, installer.Config{Runner: runner})
	pin := seedDependencyCheckPackage(t, f, "demo", "")
	writeDependencyCheckAgreement(t, f, []environment.LockedPackage{pin}, "demo")
	asked := 0
	f.app.Prompter = fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
		asked++
		return prompt.ConfirmationResult{}, nil
	}}
	out, stderr, err := f.execute(true, "install", "--scope", "user", "--locked", "--yes", "--format", "human")
	if err == nil || runner.failNextCodexAdd || !strings.Contains(out, "Needs attention") {
		t.Fatalf("did not reach the intended failed install: err=%v pendingFailure=%v\n%s\n%s", err, runner.failNextCodexAdd, out, stderr)
	}
	if asked != 0 || strings.Contains(stderr, "Retrying") || source.downloads != 0 || source.lookups != 0 {
		t.Fatalf("--yes retried or reacquired the locked package: prompts=%d downloads=%d lookups=%d stderr=%q", asked, source.downloads, source.lookups, stderr)
	}
}

func declineInstallReview(t *testing.T, cmd *cobra.Command, app App, opts *options, singleAdd bool) error {
	t.Helper()
	cmd.SetIn(strings.NewReader("n\n"))
	if singleAdd {
		accepted, err := confirmPlannedAdd(cmd.Context(), cmd, app, opts, loadedPackage{}, usecase.AddResult{}, true, false, false)
		if err != nil || accepted {
			t.Fatalf("single-add review was not declined: accepted=%v err=%v", accepted, err)
		}
		return renderDeclinedPlannedAdd(cmd, opts, loadedPackage{}, usecase.AddResult{})
	}
	accepted, err := confirmDependencySync(cmd, app, opts, false)
	if accepted {
		t.Fatal("dependency review was not declined")
	}
	return err
}

func TestInstallRetryDeclinedReviewKeepsOriginalFailure(t *testing.T) {
	for _, singleAdd := range []bool{false, true} {
		t.Run(fmt.Sprintf("single_add_%v", singleAdd), func(t *testing.T) {
			asked, reviewed := 0, 0
			app := App{Terminal: true, Prompter: fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
				asked++
				return prompt.ConfirmationResult{Accepted: true}, nil
			}}}
			boom := errors.New("native installation failed")
			opts := &options{format: "human", installResult: &installOutcome{Agents: reportByAgent([]agentItem{failedItem("demo", "codex", boom)})}}
			cmd, out, _ := retryTestCommand(t)
			rerun := func(next *options) error {
				reviewed++
				next.installResult = nil
				return declineInstallReview(t, cmd, app, next, singleAdd)
			}
			var err error
			if singleAdd {
				err = retryFailedAgents(cmd, app, opts, boom, func(string, []string) error {
					next := *opts
					next.retryAttempt++
					return rerun(&next)
				})
			} else {
				err = retryConverge(cmd, app, opts, boom, func() error { return rerun(opts) })
			}
			if !errors.Is(err, boom) || !errors.Is(err, installer.ErrCancelled) || asked != 1 || reviewed != 1 || !strings.Contains(out.String(), "No changes made.") {
				t.Fatalf("abandoned retry hid its failure: err=%v asked=%d reviewed=%d output=%q", err, asked, reviewed, out.String())
			}
		})
	}
}

func TestFirstInstallDeclinedReviewStillSucceeds(t *testing.T) {
	for _, singleAdd := range []bool{false, true} {
		t.Run(fmt.Sprintf("single_add_%v", singleAdd), func(t *testing.T) {
			cmd, out, _ := retryTestCommand(t)
			err := declineInstallReview(t, cmd, App{Terminal: true}, &options{format: "human"}, singleAdd)
			if err != nil || !strings.Contains(out.String(), "No changes made.") {
				t.Fatalf("first-attempt decline changed: err=%v output=%q", err, out.String())
			}
		})
	}
}

func TestConvergeRetryUnclosedProbeNeverPromptsOrRetries(t *testing.T) {
	var offered []string
	app, opts := retryAppAndOptions(t, &offered)
	prompts := 0
	app.Prompter = fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
		prompts++
		return prompt.ConfirmationResult{Accepted: true}, nil
	}}
	cmd, _, _ := retryTestCommand(t)
	cause := errors.New("family closure unconfirmed")
	failure := fmt.Errorf("install failed: %w", errors.Join(errors.New("ordinary client failed"), &domain.UnclosedClientProbe{Operation: "readonly inventory", Err: cause}))
	calls := 0
	err := retryConverge(cmd, app, opts, failure, func() error { calls++; return nil })
	if !domain.IsUnclosedClientProbe(err) || !errors.Is(err, cause) || len(offered) != 0 || prompts != 0 || calls != 0 {
		t.Fatalf("unsafe retry lost cause or prompted: %v offered=%v prompts=%d calls=%d", err, offered, prompts, calls)
	}
}

func TestConvergeUnclosedProbeNeverPromptsOrRetries(t *testing.T) {
	closeErr := errors.New("client process cleanup failed")
	probe := &domain.UnclosedClientProbe{Operation: "readonly inventory", Err: closeErr}
	for name, cause := range map[string]error{
		"bare":    probe,
		"wrapped": fmt.Errorf("install failed: %w", probe),
		"joined":  errors.Join(errors.New("another agent failed"), probe),
	} {
		t.Run(name, func(t *testing.T) {
			var offered []string
			app, opts := retryAppAndOptions(t, &offered)
			cmd, out, errOut := retryTestCommand(t)
			before := opts.installResult
			reruns := 0
			// The ordinary failed agent is retryable, but the run's unclosed
			// process must prevent every new convergence attempt.
			got := retryConverge(cmd, app, opts, cause, func() error { reruns++; return nil })
			var retained *domain.UnclosedClientProbe
			if !errors.Is(got, cause) || !errors.Is(cause, got) || !errors.As(got, &retained) || retained != probe || !errors.Is(got, closeErr) {
				t.Fatalf("convergence lost the cleanup refusal or its cause: %v", got)
			}
			if len(offered) != 0 || reruns != 0 || opts.retryAttempt != 0 || opts.installResult != before || out.Len() != 0 || errOut.Len() != 0 {
				t.Fatalf("unclosed convergence prompted, reran or changed its result: offered=%v reruns=%d attempt=%d", offered, reruns, opts.retryAttempt)
			}
		})
	}
}
