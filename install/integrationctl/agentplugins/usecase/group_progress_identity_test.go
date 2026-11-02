package usecase

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type progressIdentityObserver struct {
	service Service
	order   *[]string
}

func (observer progressIdentityObserver) ObserveNativeIdentity(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	*observer.order = append(*observer.order, "observe:"+string(client.ClientID))
	return observer.service.filesystemIdentityObservation(ctx, plan, managed)
}

func TestGroupedConfiguringCallbacksPrecedeFinalIdentityObservation(t *testing.T) {
	for _, inject := range []bool{false, true} {
		name := "unchanged_callback"
		if inject {
			name = "last_callback_changes_first_target"
		}
		t.Run(name, func(t *testing.T) {
			service, store, cursor := serviceFixture(t)
			kiro := domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".kiro")}
			first := addInput(t, cursor, "https://example.com/progress-identity")
			second := addInput(t, kiro, "https://example.com/progress-identity")
			activator := &failNthGroupActivator{}
			service.Activator = activator
			order := []string{}
			service.NativeObserver = progressIdentityObserver{service: service, order: &order}
			paths := map[domain.ClientID]string{}
			group := GroupInput{Targets: []AddInput{first, second}, OperationGroupID: "progress-identity", Confirmed: true}
			group.Progress = func(event GroupProgressEvent) {
				if event.Phase != GroupProgressConfiguring {
					return
				}
				order = append(order, "callback:"+string(event.ClientID))
				paths[event.ClientID] = event.Result.Plan.ActivePath
				if inject && event.ClientID == domain.ClientKiro {
					if err := os.MkdirAll(paths[domain.ClientCursor], 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(paths[domain.ClientCursor], "FOREIGN"), []byte("preserve foreign identity"), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			result, err := service.AddGroup(t.Context(), group)
			state, loadErr := store.Load()
			if loadErr != nil {
				t.Fatal(loadErr)
			}
			wantOrder := []string{"observe:cursor", "observe:kiro", "callback:cursor", "callback:kiro", "observe:cursor"}
			if !inject {
				wantOrder = append(wantOrder, "observe:kiro")
				if err != nil || !result.Mutated || len(result.Receipts) != 2 || activator.calls != 2 || len(state.Installations) != 1 {
					t.Fatalf("ordinary callbacks changed group success: result=%+v err=%v activations=%d state=%+v", result, err, activator.calls, state)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), "native identity changed before group commit") || result.Mutated || len(result.Receipts) != 0 || activator.calls != 0 || len(state.Installations) != 0 {
					t.Fatalf("callback ownership change escaped precommit refusal: result=%+v err=%v activations=%d state=%+v", result, err, activator.calls, state)
				}
				body, readErr := os.ReadFile(filepath.Join(paths[domain.ClientCursor], "FOREIGN"))
				if readErr != nil || string(body) != "preserve foreign identity" {
					t.Fatalf("foreign object changed: %q %v", body, readErr)
				}
				if _, statErr := os.Lstat(paths[domain.ClientKiro]); !os.IsNotExist(statErr) {
					t.Fatalf("sibling target was published despite precommit refusal: %v", statErr)
				}
			}
			if !reflect.DeepEqual(order, wantOrder) {
				t.Fatalf("identity observation/callback order=%v want=%v", order, wantOrder)
			}
		})
	}
}
