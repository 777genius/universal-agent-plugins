package agentpluginscli

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// An optional setup repair must qualify before version preflight. An ineligible
// unchanged binding keeps its original no-op; existing lifecycle actions are
// untouched. Existing clients without a prepared completer keep their policy.
func reviewedSavedAddActions(cmd *cobra.Command, app App, opts *options, targets []string, actions []dependencyAction) ([]dependencyAction, error) {
	canReview := opts.yes || strings.TrimSpace(opts.target) != "" || (app.Terminal && opts.format == "human")
	if cmd.Name() != "add" || !canReview || app.scope() != domain.ScopeProject ||
		len(targets) != 1 || !domain.ClientTraitsFor(domain.ClientID(targets[0])).SavedProjectRepairOnAdd {
		return actions, nil
	}
	for i, action := range actions {
		if action.Action != environment.ActionUnchanged || action.ClientID != targets[0] ||
			action.Authority == nil || action.Authority.ReceiptID == "" || action.Package == nil {
			continue
		}
		eligible, err := savedAddCompletionEligible(cmd.Context(), app, action)
		if err != nil {
			return nil, &dependencyPreflightError{Alias: action.Alias, ClientID: action.ClientID, Cause: err}
		}
		if eligible {
			actions[i] = managementNativeRepair(action)
			actions[i].Reason = "Verify and finish selected project setup if missing"
		}
	}
	return actions, nil
}

func savedAddCompletionEligible(ctx context.Context, app App, action dependencyAction) (bool, error) {
	if _, complete := clients.As[clients.PreparedSetupCompleter](app.ClientRegistry, domain.ClientID(action.ClientID)); !complete {
		return true, nil
	}
	pin := *action.Package
	if pin.ExecutableFiles == nil {
		var err error
		pin, err = hydrateRecordedDependencyInventory(ctx, app, pin)
		if err != nil {
			return false, err
		}
	}
	repair := managementNativeRepair(action)
	repair.Package = &pin
	recorded, err := (&dependencyInstaller{app: app}).recordedDependencyRelease(repair)
	if err != nil {
		return false, err
	}
	loaded, err := materializeLockedPackage(ctx, app, pin, false, recorded, domain.DirectoryRepair)
	if err != nil {
		return false, err
	}
	eligible := pureSkillsEnvelope(loaded.envelope)
	if loaded.cleanup != nil {
		err = loaded.cleanup()
	}
	return eligible, err
}
