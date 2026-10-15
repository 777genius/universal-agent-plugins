package agentpluginscli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

type dependencyActionDTO struct {
	Alias          string `json:"alias"`
	ClientID       string `json:"client_id"`
	InstallationID string `json:"installation_id,omitempty"`
	BindingID      string `json:"binding_id,omitempty"`
	Action         string `json:"action"`
	Reason         string `json:"reason,omitempty"`
	PackageDigest  string `json:"package_digest,omitempty"`
	Revision       string `json:"revision,omitempty"`
}

func dependencyActionOutput(actions []dependencyAction) []dependencyActionDTO {
	out := make([]dependencyActionDTO, 0, len(actions))
	for _, action := range actions {
		item := dependencyActionDTO{Alias: action.Alias, ClientID: action.ClientID, InstallationID: action.InstallationID, BindingID: action.BindingID, Action: string(action.Action), Reason: action.Reason}
		if action.Package != nil {
			item.PackageDigest = action.Package.TreeDigest
			item.Revision = lockedCommit(*action.Package)
		}
		out = append(out, item)
	}
	return out
}

type dependencyResultDTO struct {
	InstallationID string                    `json:"installation_id,omitempty"`
	ClientID       string                    `json:"client_id,omitempty"`
	Operation      string                    `json:"operation"`
	Outcome        string                    `json:"outcome"`
	Reason         string                    `json:"reason,omitempty"`
	ManualActions  []string                  `json:"manual_actions,omitempty"`
	NextActions    []dependencyNextActionDTO `json:"next_actions,omitempty"`
}
type dependencyNextActionDTO struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason,omitempty"`
}

func renderDependencyResults(cmd *cobra.Command, opts *options, actions []dependencyAction, results []installer.Result, applyErr error) error {
	out := make([]dependencyResultDTO, 0, len(results))
	for _, result := range results {
		item := dependencyResultDTO{InstallationID: result.InstallationID, ClientID: result.Client.ClientID, Operation: string(result.Operation), Outcome: string(result.Outcome), Reason: result.Reason, ManualActions: result.ManualActions}
		fact := dependencyStatusFact{ClientID: result.Client.ClientID}
		switch result.Client.Authentication {
		case "auth_pending":
			fact.Facts = []string{"auth_pending"}
		case "failed":
			fact.Facts = []string{"auth_failed"}
		}
		item.NextActions = dependencyNextActions(fact, result.NextActions...)
		out = append(out, item)
	}
	outcome := "completed"
	if applyErr != nil {
		outcome = "partial"
	}
	if opts.format == "json" {
		return renderJSON(cmd.OutOrStdout(), map[string]any{"outcome": outcome, "actions": dependencyActionOutput(actions), "results": out})
	}
	for _, result := range out {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s %s/%s: %s %s\n", result.Operation, result.InstallationID, result.ClientID, result.Outcome, result.Reason); err != nil {
			return err
		}
		for _, next := range result.NextActions {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  Next: %s %s\n", next.Kind, next.Reason); err != nil {
				return err
			}
		}
		for _, manual := range result.ManualActions {
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), "  Manual:", manual); err != nil {
				return err
			}
		}
	}
	return nil
}
