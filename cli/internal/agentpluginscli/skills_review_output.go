package agentpluginscli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
)

// A CLI dry run is a review, not a session-bound HTTP confirmation. Do not emit
// a fictitious plan ID or zero expiry timestamp that could suggest otherwise.
func renderSkillsDryRun(cmd *cobra.Command, review skillspicker.Plan) error {
	output := struct {
		Status      string                    `json:"status"`
		DryRun      bool                      `json:"dryRun"`
		Actions     []skillspicker.Action     `json:"actions"`
		Conflicts   []skillspicker.Diagnostic `json:"conflicts"`
		Warnings    []skillspicker.Diagnostic `json:"warnings"`
		Confirmable bool                      `json:"confirmable"`
	}{Status: "planned", DryRun: true, Actions: review.Actions, Conflicts: review.Conflicts, Warnings: review.Warnings, Confirmable: review.Confirmable}
	return json.NewEncoder(cmd.OutOrStdout()).Encode(output)
}
func renderHumanSkillsReview(cmd *cobra.Command, review skillspicker.Plan) error {
	for _, action := range review.Actions {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s %s → %s (%s)\n", action.Action, prompt.SafeText(action.Alias), prompt.SafeText(action.TargetId), prompt.SafeText(action.DestinationPath)); err != nil {
			return err
		}
	}
	for _, warning := range review.Warnings {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Warning: %s\n", prompt.SafeText(warning.Message)); err != nil {
			return err
		}
	}
	return nil
}
