package usecase

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

type removalDecisionControl struct {
	store           transaction.StateStore
	mode            string
	prepared        bool
	settled         int
	packagePath     string
	before, desired domain.StateFileV2
}

func (control *removalDecisionControl) PrepareNativeRemovalDecision(_ context.Context, installationID, bindingID string, before, desired domain.StateFileV2) (bool, error) {
	control.prepared = true
	current, err := control.store.Load()
	if err != nil || !reflect.DeepEqual(current, before) {
		return true, errors.Join(err, errors.New("callback predecessor differs from a fresh durable load"))
	}
	if err := pendingAndClearedRemovalDecision(before, desired, installationID, bindingID); err != nil {
		return true, err
	}
	frozen, err := freezeNativeRemovalAttempt(before, desired, "")
	if err != nil {
		return true, err
	}
	control.before, control.desired = frozen.before, frozen.attempted
	for _, installation := range before.Installations {
		if installation.InstallationID == installationID {
			control.packagePath = installation.Clients[bindingID].TargetLocator
		}
	}
	if _, err := os.Stat(control.packagePath); err != nil {
		return true, fmt.Errorf("managed package disappeared before native decision: %w", err)
	}
	switch control.mode {
	case "callback-error":
		return true, errors.New("inert decision publication failure")
	case "mutate-before":
		before.Installations[0].Clients[bindingID] = domain.ClientBinding{}
	case "mutate-desired":
		desired.Installations[0].Clients[bindingID] = domain.ClientBinding{}
	case "foreign-state":
		current, err := control.store.Load()
		if err != nil {
			return true, err
		}
		current.Installations[0].UpdatedAt = "foreign after callback"
		if err := control.store.Save(current); err != nil {
			return true, err
		}
	}
	return control.mode != "unrelated", nil
}

func pendingAndClearedRemovalDecision(before, desired domain.StateFileV2, installationID, bindingID string) error {
	for index, installation := range before.Installations {
		if installation.InstallationID != installationID {
			continue
		}
		if installation.Clients[bindingID].NativeActivationAttempt == "" || desired.Installations[index].Clients[bindingID].NativeActivationAttempt != "" {
			return errors.New("native callback did not receive pending predecessor and cleared target")
		}
		return nil
	}
	return errors.New("native callback selected installation disappeared")
}

func (control *removalDecisionControl) Recover(context.Context) error {
	if !control.prepared {
		return nil
	}
	control.settled++
	state, err := control.store.Load()
	if err != nil || !reflect.DeepEqual(state, control.desired) {
		return errors.Join(err, errors.New("native recovery did not receive exact committed state"))
	}
	if _, err := os.Stat(control.packagePath); err != nil {
		return fmt.Errorf("core deleted package before native recovery: %w", err)
	}
	if control.mode == "recovery-error" {
		return errors.New("inert journal cleanup failure")
	}
	return nil
}

func TestNativeRemovalDecisionFreezesStateAndOrdersPackageDeletion(t *testing.T) {
	for _, grouped := range []bool{false, true} {
		for _, mode := range []string{"success", "unrelated", "callback-error", "mutate-before", "mutate-desired", "foreign-state", "recovery-error"} {
			t.Run(fmt.Sprintf("grouped-%t/%s", grouped, mode), func(t *testing.T) {
				service, _, id, client := removeNoEffectFixture(t)
				control := &removalDecisionControl{store: service.StateStore, mode: mode}
				service.Kernel.NativeRecovery = control
				err := removeNoEffectCall(t, service, id, client, grouped)
				success := mode == "success" || mode == "unrelated"
				if !control.prepared || success && err != nil || !success && err == nil {
					t.Fatal("wrong removal decision result", control.prepared, err)
				}
				_, packageErr := os.Stat(control.packagePath)
				if success && !errors.Is(packageErr, os.ErrNotExist) || !success && packageErr != nil {
					t.Fatal("package deletion was not ordered after durable native decision", packageErr)
				}
				if mode == "unrelated" && control.settled != 0 || mode == "success" && control.settled != 1 {
					t.Fatal("recovery was not restricted to the handled decision", control.settled)
				}
				if !success && mode != "foreign-state" {
					state, loadErr := service.StateStore.Load()
					want := control.before
					if mode == "recovery-error" {
						want = control.desired
					}
					if loadErr != nil || !reflect.DeepEqual(state, want) {
						t.Fatal("callback changed the core's frozen durable decision", loadErr)
					}
				}
			})
		}
	}
}
