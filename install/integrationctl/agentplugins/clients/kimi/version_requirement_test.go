package kimi

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestProjectVersionRequirementBelongsToSelectedKimi(t *testing.T) {
	for _, id := range []domain.ClientID{domain.ClientKimi, domain.ClientWindsurf, domain.ClientGemini, ""} {
		if got := New().RequiresProjectVersion(domain.DetectedClient{ClientID: id}); got != (id == domain.ClientKimi) {
			t.Fatalf("client %q project version requirement = %v", id, got)
		}
	}
}
