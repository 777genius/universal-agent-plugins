package usecase

import (
	"context"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreviouslySkippedMCPLifecycle(t *testing.T) {
	for _, op := range []string{"add_again", "repair", "update", "group_add_again", "group_repair", "group_update"} {
		t.Run(op, func(t *testing.T) {
			service, _, client := serviceFixture(t)
			input := clinePackageInput(t, client, "1.0.0", "sha256:xhigh-partial", "sha256:xhigh-partial-manifest", "agentplugins-definitely-unavailable-runtime")
			input.Confirmed = true
			installed, err := service.Add(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			input.InstallationID = installed.InstallationID
			input.OperationID = "partial-followup"
			if op == "group_repair" {
				service.NativeObserver = providers.NativeIdentityObserver{Stager: service.Stager}
				if err := os.RemoveAll(installed.Plan.ActivePath); err != nil {
					t.Fatal(err)
				}
			} else if strings.HasSuffix(op, "repair") {
				if err := os.WriteFile(filepath.Join(installed.Plan.ActivePath, "review-damage"), []byte("damage"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if strings.HasSuffix(op, "update") {
				input = clinePackageInput(t, client, "2.0.0", "sha256:xhigh-partial-v2", "sha256:xhigh-partial-manifest-v2", "agentplugins-definitely-unavailable-runtime")
				input.Confirmed = true
				input.OperationID = "partial-update"
				input.InstallationID = installed.InstallationID
			}
			var result AddResult
			if strings.HasPrefix(op, "group_") {
				args := GroupInput{Targets: []AddInput{input}, Confirmed: true, OperationGroupID: "partial-next"}
				var g GroupResult
				switch op {
				case "group_repair":
					g, err = service.RepairGroup(context.Background(), args)
				case "group_update":
					g, err = service.UpdateGroup(context.Background(), args)
				default:
					g, err = service.AddGroup(context.Background(), args)
				}
				if len(g.Targets) > 0 {
					result = g.Targets[0]
				}
			} else {
				switch op {
				case "repair":
					result, err = service.Repair(context.Background(), input)
				case "update":
					result, err = service.Update(context.Background(), input)
				default:
					result, err = service.Add(context.Background(), input)
				}
			}
			if err != nil {
				t.Fatalf("previously skipped MCP still absent; healthy skill lifecycle refused: mutated=%v noChange=%v err=%v", result.Mutated, result.NoChange, err)
			}
		})
	}
}
