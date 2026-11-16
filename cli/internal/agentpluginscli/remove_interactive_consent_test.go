package agentpluginscli

import (
	"os"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// An interactive remove without --target once removed the only installed
// target immediately: the inferred target was treated as an explicit command.
// Only an explicit --target or --yes is consent; an inferred one needs a review.
func TestInteractiveRemoveWithInferredTargetRequiresConfirmation(t *testing.T) {
	t.Parallel()
	for _, clients := range [][]domain.ClientID{{domain.ClientCursor}, {domain.ClientCursor, domain.ClientGemini}} {
		t.Run(string(clients[len(clients)-1])+"-"+string(rune('0'+len(clients))), func(t *testing.T) {
			t.Parallel()
			detected := make([]domain.DetectedClient, 0, len(clients))
			names := make([]string, 0, len(clients))
			for _, client := range clients {
				detected = append(detected, fixtureClient(t, client))
				names = append(names, string(client))
				if err := os.MkdirAll(detected[len(detected)-1].ConfigRoot, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			fixture := newCLIFixture(t, detected)
			if _, _, err := fixture.execute(false, "add", writeCLIPlugin(t), "--target", strings.Join(names, ",")); err != nil {
				t.Fatal(err)
			}
			bound := func() int {
				t.Helper()
				state, err := fixture.store.Load()
				if err != nil || len(state.Installations) != 1 {
					t.Fatalf("state = %+v, %v", state, err)
				}
				return len(state.Installations[0].Clients)
			}
			// Multiple targets first answer the target selection with its default.
			selection := ""
			if len(clients) > 1 {
				selection = "\n"
			}
			for _, answer := range []string{"\n", "n\n"} {
				stdout, _, err := fixture.executeInput(true, selection+answer, "remove", "demo")
				if err != nil {
					t.Fatalf("declined interactive remove: %v\n%s", err, stdout)
				}
				if bound() != len(clients) || !strings.Contains(stdout, "No changes made.") || strings.Contains(stdout, removalIncompleteNotice) {
					t.Fatalf("interactive remove changed targets without confirmation (%q):\n%s", answer, stdout)
				}
			}
			stdout, _, err := fixture.executeInput(true, selection+"y\n", "remove", "demo")
			if err != nil {
				t.Fatalf("confirmed interactive remove: %v\n%s", err, stdout)
			}
			if bound() != 0 {
				t.Fatalf("confirmed interactive remove kept targets:\n%s", stdout)
			}
		})
	}
}
