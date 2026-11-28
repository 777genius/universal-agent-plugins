package agentpluginscli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

func TestEnterprisePolicyBrowserGitPinsAtSourceAndFinalGate(t *testing.T) {
	commit := strings.Repeat("a", 40)
	for _, tc := range []struct {
		name, raw string
		pinned    bool
	}{
		{"github root", "https://github.com/acme/demo/tree/" + commit, true},
		{"github package", "https://github.com/acme/demo/tree/" + commit + "/plugins/demo", true},
		{"gitlab package", "https://gitlab.com/acme/team/demo/-/tree/" + commit + "/plugins/demo", true},
		{"self hosted gitlab", "https://git.corp.test/acme/demo/-/tree/" + commit + "/plugins/demo", true},
		{"explicit commit", "https://github.com/acme/demo.git#" + commit + "//plugins/demo", true},
		{"branch", "https://github.com/acme/demo/tree/main/plugins/demo", false},
		{"commit only in package path", "https://github.com/acme/demo/tree/main/" + commit, false},
		{"short commit", "https://github.com/acme/demo/tree/" + commit[:12] + "/plugins/demo", false},
		{"uppercase commit", "https://github.com/acme/demo/tree/" + strings.ToUpper(commit) + "/plugins/demo", false},
		{"commit-like tag", "https://github.com/acme/demo/tree/" + commit + "-tag/plugins/demo", false},
		{"explicit ref", "https://github.com/acme/demo.git#refs/heads/" + commit + "//plugins/demo", false},
		{"default", "https://github.com/acme/demo.git", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := sourcePolicyApp(t, "schema_version: 2\nintegrity:\n  require_pinned_git: true\n")
			spec, err := sourcespec.Parse(tc.raw)
			if err != nil || spec.Git == nil {
				t.Fatalf("parse requested Git input: %+v %v", spec, err)
			}
			ctx := context.Background()
			assertEnterpriseGitPinDecision(t, app.checkEnterpriseGit(ctx, *spec.Git), tc.pinned)
			loaded := loadedPackage{origin: domain.OriginModeDirect, envelope: domain.PackageEnvelope{Source: domain.SourceIdentity{
				SourceKind: "git", SourceURL: spec.Git.URL, Repository: spec.Git.Repository,
				RequestedSource: spec.Requested, ResolvedRevision: commit,
			}}}
			request := enterprisePolicyRequest(loaded, domain.ScopeUser, []domain.ClientID{domain.ClientCodex})
			_, err = app.checkEnterprisePolicy(ctx, request)
			assertEnterpriseGitPinDecision(t, err, tc.pinned)
			if request.GitPinned == nil || *request.GitPinned != tc.pinned {
				t.Fatalf("final request lost original pin intent: %+v", request)
			}
			if !tc.pinned {
				// An unavailable acquirer must never be consulted for moving refs.
				_, err := app.loadDirectGit(ctx, *spec.Git)
				assertEnterpriseGitPinDecision(t, err, false)
				return
			}
			runner := &policyPinGitRunner{}
			resolved, err := (sourceacquisition.Acquirer{Runner: runner}).ResolveGitSource(ctx, *spec.Git)
			if err != nil || runner.calls != 0 || resolved.Git.Commit != commit || resolved.Resolution.RequestedRef != commit {
				t.Fatalf("policy pin disagrees with offline resolver: %+v %v calls=%d", resolved, err, runner.calls)
			}
		})
	}
}

func assertEnterpriseGitPinDecision(t *testing.T, err error, pinned bool) {
	t.Helper()
	if pinned {
		if err != nil {
			t.Fatalf("explicit requested commit refused: %v", err)
		}
		return
	}
	var denied *domain.InstallPolicyError
	if !errors.As(err, &denied) || len(denied.Violations) != 1 || denied.Violations[0].Code != "integrity.require_pinned_git" {
		t.Fatalf("moving Git intent escaped requested-pin refusal: %v", err)
	}
}

type policyPinGitRunner struct{ calls int }

func (runner *policyPinGitRunner) Run(context.Context, sourceacquisition.Command) ([]byte, error) {
	runner.calls++
	return nil, errors.New("exact requested commit must not need Git discovery")
}

func TestEnterprisePolicyGitPinSelectorPrecedence(t *testing.T) {
	commit, browserCommit := strings.Repeat("a", 40), strings.Repeat("b", 40)
	for _, tc := range []struct {
		name, commit, ref, browser, want string
	}{
		{"commit before ref and browser", commit, "refs/heads/main", browserCommit + "/plugin", commit},
		{"ref before browser", "", "refs/heads/main", browserCommit + "/plugin", ""},
		{"browser commit", "", "", browserCommit + "/plugin", browserCommit},
		{"empty browser", "", "", "", ""},
		{"browser branch", "", "", "main/plugin", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := sourcespec.Git{Commit: tc.commit, Ref: tc.ref, Browser: &sourcespec.BrowserTree{Suffix: tc.browser}}
			if got := requestedPolicyGitCommit(source); got != tc.want {
				t.Fatalf("requested commit %q, want %q", got, tc.want)
			}
		})
	}
}
