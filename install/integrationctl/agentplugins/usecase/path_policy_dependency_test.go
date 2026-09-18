package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// TestMutatingEntryPointsRequireAPathPolicy pins the fail-fast decision: the
// path policy has no default. A service wired without one must refuse before it
// reaches state, staging or a client, instead of silently running with whatever
// containment rules a fallback happened to carry.
func TestMutatingEntryPointsRequireAPathPolicy(t *testing.T) {
	t.Parallel()
	complete, _, client := serviceFixture(t)
	incomplete := complete
	incomplete.Paths = nil
	ctx := context.Background()
	target := AddInput{Client: client, Scope: domain.ScopeUser}

	entryPoints := map[string]func(Service) error{
		"Add": func(service Service) error {
			_, err := service.Add(ctx, target)
			return err
		},
		"Repair": func(service Service) error {
			_, err := service.Repair(ctx, target)
			return err
		},
		"Remove": func(service Service) error {
			_, err := service.Remove(ctx, RemoveInput{Selector: "demo"})
			return err
		},
		"AddGroup": func(service Service) error {
			_, err := service.AddGroup(ctx, GroupInput{Targets: []AddInput{target}})
			return err
		},
		"RemoveGroup": func(service Service) error {
			_, err := service.RemoveGroup(ctx, RemoveGroupInput{
				Selector: "demo", Targets: []RemoveInput{{Selector: "demo"}},
			})
			return err
		},
	}
	const incompleteDependencies = "dependencies are incomplete"
	for name, run := range entryPoints {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := run(incomplete)
			if err == nil || !strings.Contains(err.Error(), incompleteDependencies) {
				t.Fatalf("%s accepted a service without a path policy: %v", name, err)
			}
			// Control: the same inputs fail later for their own reasons once the
			// policy is wired, so the assertion above is about Paths only.
			if err := run(complete); err != nil && strings.Contains(err.Error(), incompleteDependencies) {
				t.Fatalf("%s reports incomplete dependencies with a fully wired service: %v", name, err)
			}
		})
	}
}
