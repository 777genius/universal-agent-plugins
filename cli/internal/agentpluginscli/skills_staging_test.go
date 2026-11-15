package agentpluginscli

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type stagedSkillSource struct {
	skillsource.Acquirer
	acquisitions int
	offline      bool
}

func (source *stagedSkillSource) AcquireGitHubSkill(ctx context.Context, repository, commit, path string) (domain.PackageSnapshot, error) {
	source.acquisitions++
	if source.offline {
		return domain.PackageSnapshot{}, errors.New("source unavailable after review")
	}
	return source.Acquirer.AcquireGitHubSkill(ctx, repository, commit, path)
}

func (source *stagedSkillSource) AcquireGitHubSkillVerified(ctx context.Context, repository, commit, path, digest string) (domain.PackageSnapshot, error) {
	source.acquisitions++
	if source.offline {
		return domain.PackageSnapshot{}, errors.New("source unavailable after review")
	}
	return source.Acquirer.AcquireGitHubSkillVerified(ctx, repository, commit, path, digest)
}

func TestSkillsApplyRetainsReviewedBytesAndRechecksSecurity(t *testing.T) {
	for _, refuse := range []bool{false, true} {
		name := "offline upstream"
		if refuse {
			name = "fresh security refusal"
		}
		t.Run(name, func(t *testing.T) {
			journey, _, sha := skillJourney(t)
			source := &stagedSkillSource{Acquirer: skillsource.Acquirer{Acquirer: journey.source.Acquirer}}
			scanner := &dependencyStagingScanner{}
			journey.cli.app.SourceAcquirer = source
			journey.cli.app.SecurityEvaluator = scanner
			plan, err := journey.cli.app.PrepareSkills(t.Context(), SkillsBatchRequest{
				Selections: []SkillsSelection{{Repository: "acme/tools", Commit: sha, Path: "skills/pdf", Name: "pdf"}}, TargetIDs: []string{"codex"},
			})
			if err != nil {
				t.Fatal(err)
			}
			defer plan.Close()
			roots := []string{}
			for _, snapshot := range plan.preview.dependencySnapshots.snapshots {
				roots = append(roots, snapshot.Root)
			}
			if len(roots) == 0 || source.acquisitions != 1 {
				t.Fatalf("review did not retain one acquisition: roots=%v acquisitions=%d", roots, source.acquisitions)
			}
			source.offline, scanner.fail = true, refuse
			before := scanner.calls
			result, err := plan.Apply(t.Context())
			if refuse {
				if err == nil || !strings.Contains(err.Error(), "fresh scanner refusal") {
					t.Fatalf("reviewed bytes bypassed fresh security refusal: %+v %v", result, err)
				}
			} else if err != nil || result.Status != "succeeded" {
				t.Fatalf("apply redownloaded bytes after review: %+v %v", result, err)
			}
			if source.acquisitions != 1 || scanner.calls <= before {
				t.Fatalf("acquisition or security recheck changed: acquisitions=%d scans=%d before=%d", source.acquisitions, scanner.calls, before)
			}
			plan.Close()
			for _, root := range roots {
				if _, err := os.Stat(root); !os.IsNotExist(err) {
					t.Fatalf("plan Close retained staged bytes: %s: %v", root, err)
				}
			}
		})
	}
}
