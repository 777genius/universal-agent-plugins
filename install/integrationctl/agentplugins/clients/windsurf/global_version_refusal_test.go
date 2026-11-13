package windsurf

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestDevinGlobalMissingVersionRefusesBeforeConfigOrProcessEffects(t *testing.T) {
	for _, verifyOnly := range []bool{false, true} {
		for _, condition := range []string{"unobserved", "wrong-client", "wrong-executable", "relative-root", "legacy-root", "unsupported-version"} {
			t.Run(fmt.Sprintf("%s/verify-only=%t", condition, verifyOnly), func(t *testing.T) {
				client, plan, _ := globalOfflineFixture(t)
				client.Version = ""
				switch condition {
				case "wrong-client":
					client.ClientID = domain.ClientCline
				case "wrong-executable":
					client.ExecutablePath = filepath.Join(filepath.Dir(client.ExecutablePath), "windsurf")
				case "relative-root":
					client.NativeConfigRoot = "devin"
				case "legacy-root":
					client.ConfigRoot = client.NativeConfigRoot
				case "unsupported-version":
					client.Version = "3000.99.99"
				}
				effects := &globalVersionRefusalEffects{}
				env := clients.Env{Runner: effects, NativeConfig: nativeconfig.NewWithFileIO(effects)}
				request := domain.ActivationRequest{Client: client, Plan: plan, VerifyOnly: verifyOnly,
					Delivery: domain.StagedDelivery{ClientID: domain.ClientWindsurf, ActivePath: plan.ActivePath}}
				errorsSeen := []error{New().PreflightActivation(env, request)}
				_, err := New().Activate(context.Background(), env, request)
				errorsSeen = append(errorsSeen, err)
				for _, failure := range errorsSeen {
					var verification *ports.VerificationError
					indeterminate := errors.As(failure, &verification) && verification.Kind == ports.VerificationIndeterminate
					if failure == nil || indeterminate != (condition == "unobserved") {
						t.Fatalf("unexpected refusal classification: %v", failure)
					}
				}
				if len(effects.calls) != 0 {
					t.Fatalf("unqualified version accessed client effects: %v", effects.calls)
				}
			})
		}
	}
}

type globalVersionRefusalEffects struct{ calls []string }

func (effects *globalVersionRefusalEffects) unexpected(operation string) error {
	effects.calls = append(effects.calls, operation)
	return errors.New("unexpected client effect before selected version admission")
}

func (effects *globalVersionRefusalEffects) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	return legacyports.CommandResult{}, effects.unexpected("process")
}

func (effects *globalVersionRefusalEffects) ReadNoFollow(string) ([]byte, os.FileMode, bool, error) {
	return nil, 0, false, effects.unexpected("read")
}

func (effects *globalVersionRefusalEffects) WriteAtomic(string, []byte, os.FileMode) error {
	return effects.unexpected("write")
}

func (effects *globalVersionRefusalEffects) RemoveNoFollow(string) error {
	return effects.unexpected("remove")
}
