package usecase

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func TestDoctorManualStepsDedupesRepeatedAuthGuidance(t *testing.T) {
	t.Parallel()
	steps := doctorManualSteps("demo", domain.TargetInstallation{
		State: domain.InstallAuthPending,
		EnvironmentRestrictions: []domain.EnvironmentRestrictionCode{
			domain.RestrictionNativeAuthRequired,
			domain.RestrictionSourceAuthRequired,
		},
	})
	count := 0
	for _, step := range steps {
		if step == "complete the missing authentication step and rerun repair" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("steps = %+v", steps)
	}
}
