package agentpluginscli

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestEnterprisePolicyLocalCollectionRefusesBeforeDiscovery(t *testing.T) {
	j, root := localCollectionJourney(t)
	j.cli.app = j.cli.app.withEnterprisePolicyGate(sourcePolicyApp(t, "schema_version: 2\nsources:\n  allowed_kinds: [web]\n").InstallationPolicy)
	before := savedJourneyTree(t, j.cli.root)
	_, _, err := j.cli.execute(false, "skills", "add", filepath.Join(root, "missing"), "--all", "--target", "codex,claude", "--yes", "--format", "json")
	var denied *domain.InstallPolicyError
	if !errors.As(err, &denied) {
		t.Fatalf("denied local collection reached filesystem discovery: %v", err)
	}
	j.run(t, "skills", "list", root, "--format", "json")
	if j.runner.calls != 0 || !reflect.DeepEqual(before, savedJourneyTree(t, j.cli.root)) {
		t.Fatal("read-only listing changed durable state")
	}
}

func TestEnterprisePolicyGitAllRefusesBeforeDiscovery(t *testing.T) {
	for _, input := range []string{"acme/skills", "https://github.com/acme/skills/tree/main", "https://github.com/acme/skills/blob/main/review/SKILL.md", "https://raw.githubusercontent.com/acme/skills/main/review/SKILL.md", "ssh://git@git.example/skills.git"} {
		t.Run(input, func(t *testing.T) {
			j := newSavedJourney(t, t.TempDir())
			runner := &policyPinGitRunner{}
			j.cli.app.SourceAcquirer = sourceacquisition.Acquirer{Runner: runner}
			j.cli.app = j.cli.app.withEnterprisePolicyGate(sourcePolicyApp(t, "schema_version: 2\nsources:\n  allowed_kinds: [local]\n").InstallationPolicy)
			before := savedJourneyTree(t, j.cli.root)
			_, _, err := j.cli.execute(false, "skills", "add", input, "--all", "--target", "codex", "--yes", "--format", "json")
			var denied *domain.InstallPolicyError
			if !errors.As(err, &denied) || runner.calls != 0 || j.runner.calls != 0 || !reflect.DeepEqual(before, savedJourneyTree(t, j.cli.root)) {
				t.Fatalf("denied Git reached resolution/open or mutated state: git=%d native=%d err=%v", runner.calls, j.runner.calls, err)
			}
		})
	}
}

func TestEnterprisePolicyPickerRefusesBeforeDiscovery(t *testing.T) {
	j := newSavedJourney(t, t.TempDir())
	runner := &policyPinGitRunner{}
	j.cli.app.SourceAcquirer = sourceacquisition.Acquirer{Runner: runner}
	j.cli.app = j.cli.app.withEnterprisePolicyGate(sourcePolicyApp(t, "schema_version: 2\nsources:\n  allowed_kinds: [local]\n").InstallationPolicy)
	before := savedJourneyTree(t, j.cli.root)
	picker, err := j.cli.app.StartSkillsPicker(t.Context(), SkillsPickerRequest{Input: "acme/skills"})
	if picker != nil {
		picker.Close()
	}
	var denied *domain.InstallPolicyError
	if !errors.As(err, &denied) || runner.calls != 0 || !reflect.DeepEqual(before, savedJourneyTree(t, j.cli.root)) {
		t.Fatalf("denied picker reached discovery or changed state: calls=%d err=%v", runner.calls, err)
	}
}

func TestEnterprisePolicySkillsDiscoveryKeepsRequestedPins(t *testing.T) {
	commit := strings.Repeat("a", 40)
	app := sourcePolicyApp(t, "schema_version: 2\nintegrity:\n  require_pinned_git: true\n")
	for _, tc := range []struct {
		input, ref string
		pinned     bool
	}{
		{"acme/skills@" + commit, "", true},
		{"https://github.com/acme/skills/tree/" + commit + "/review", "", true},
		{"https://raw.githubusercontent.com/acme/skills/" + commit + "/review/SKILL.md", "", true},
		{"acme/skills", "", false},
		{"acme/skills@main", "", false},
		{"https://github.com/acme/skills/tree/main/review", "", false},
		{"https://github.com/acme/skills/tree/" + commit + "/review", "refs/heads/" + commit, false},
	} {
		t.Run(tc.input+tc.ref, func(t *testing.T) {
			source, err := skillsource.RequestedGitIntent(tc.input, tc.ref)
			if err != nil {
				t.Fatal(err)
			}
			assertEnterpriseGitPinDecision(t, app.checkEnterpriseGit(t.Context(), source), tc.pinned)
		})
	}
}

func TestEnterprisePolicyGitListingRemainsReadOnly(t *testing.T) {
	j, _, _ := skillsFullFlowFixture(t)
	j.cli.app = j.cli.app.withEnterprisePolicyGate(sourcePolicyApp(t, "schema_version: 2\nsources:\n  allowed_kinds: [local]\n").InstallationPolicy)
	before := savedJourneyTree(t, j.cli.root)
	j.run(t, "skills", "list", "acme/skills", "--format", "json")
	if j.runner.calls != 0 || !reflect.DeepEqual(before, savedJourneyTree(t, j.cli.root)) {
		t.Fatal("read-only Git listing changed durable/native state")
	}
}

func TestEnterprisePolicyCollectionEnvelopeRefusesWholeBatch(t *testing.T) {
	j, root := localCollectionJourney(t)
	policy := sourcePolicyApp(t, "schema_version: 2\ncomponents:\n  denied: [skills]\n").InstallationPolicy
	var firstAdmitted, secondDenied bool
	j.cli.app = j.cli.app.withEnterprisePolicyGate(enterpriseHTTPSPolicyFunc(func(ctx context.Context, request domain.InstallPolicyRequest) (string, error) {
		if request.Envelope != nil {
			switch request.Envelope.Manifest.Name {
			case "review":
				firstAdmitted = true
			case "testing":
				secondDenied = true
				return policy.Check(ctx, request)
			}
		}
		return "collection-policy", nil
	}))
	before, source := savedJourneyTree(t, j.cli.root), savedJourneyTree(t, root)
	_, _, err := j.cli.execute(false, "skills", "add", root, "--all", "--target", "codex,claude", "--yes", "--format", "json")
	var denied *domain.InstallPolicyError
	if !errors.As(err, &denied) || !firstAdmitted || !secondDenied || j.runner.calls != 0 || !reflect.DeepEqual(before, savedJourneyTree(t, j.cli.root)) || !reflect.DeepEqual(source, savedJourneyTree(t, root)) {
		t.Fatalf("denied collection changed durable/native/source state: %v", err)
	}
}

// Explicit client restrictions are known before the Git/Local Skills catalog is
// fetched. No native/source operation may occur when policy denies the target.
func TestEnterprisePolicySkillsExplicitTargetRefusesBeforeDiscovery(t *testing.T) {
	for _, sourceKind := range []string{"git", "local"} {
		t.Run(sourceKind, func(t *testing.T) {
			j := newSavedJourney(t, t.TempDir())
			gate := sourcePolicyApp(t, "schema_version: 2\ninstall:\n  allowed_targets: [claude]\n").InstallationPolicy
			j.cli.app = j.cli.app.withEnterprisePolicyGate(gate)
			input := "acme/skills"
			var gitRunner *policyPinGitRunner
			if sourceKind == "git" {
				gitRunner = &policyPinGitRunner{}
				j.cli.app.SourceAcquirer = sourceacquisition.Acquirer{Runner: gitRunner}
			} else {
				input = filepath.Join(t.TempDir(), "source-not-present")
			}
			before := savedJourneyTree(t, j.cli.root)
			_, _, err := j.cli.execute(false, "skills", "add", input, "--all", "--target", "codex", "--yes", "--format", "json")
			var denied *domain.InstallPolicyError
			if !errors.As(err, &denied) || j.runner.calls != 0 || !reflect.DeepEqual(before, savedJourneyTree(t, j.cli.root)) {
				t.Fatalf("source discovery preceded denied target or mutated state: %v", err)
			}
			if gitRunner != nil && gitRunner.calls != 0 {
				t.Fatalf("Git ref lookup started for disallowed target: %d", gitRunner.calls)
			}
		})
	}
}
