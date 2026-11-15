package agentpluginscli

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// A CLI dry run is a review, not a session-bound HTTP confirmation. Do not emit
// a fictitious plan ID or zero expiry timestamp that could suggest otherwise.
func renderSkillsDryRun(cmd *cobra.Command, review skillspicker.Plan) error {
	output := struct {
		SchemaVersion int                       `json:"schema_version"`
		Command       string                    `json:"command"`
		Result        string                    `json:"result"`
		Status        string                    `json:"status"`
		DryRun        bool                      `json:"dryRun"`
		Actions       []skillspicker.Action     `json:"actions"`
		Conflicts     []skillspicker.Diagnostic `json:"conflicts"`
		Warnings      []skillspicker.Diagnostic `json:"warnings"`
		Confirmable   bool                      `json:"confirmable"`
	}{SchemaVersion: outputSchemaVersion, Command: "skills add", Result: outputResultSuccess, Status: "planned", DryRun: true, Actions: review.Actions, Conflicts: review.Conflicts, Warnings: review.Warnings, Confirmable: review.Confirmable}
	return json.NewEncoder(cmd.OutOrStdout()).Encode(output)
}
func renderHumanSkillsReview(cmd *cobra.Command, review skillspicker.Plan) error {
	for _, action := range review.Actions {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), skillsActionDetail(action)); err != nil {
			return err
		}
	}
	for _, warning := range review.Warnings {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Warning: %s\n", prompt.SafeText(warning.Message)); err != nil {
			return err
		}
	}
	for _, conflict := range review.Conflicts {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Conflict: %s\n", prompt.SafeText(conflict.Message)); err != nil {
			return err
		}
	}
	return nil
}

// Keep review authority separate from its compact presentation. Exact action /
// destination mappings remain available before the user authorizes application.
func skillsReviewConfirmation(review skillspicker.Plan) prompt.ConfirmationRequest {
	type group struct {
		action, alias string
		targets       []string
	}
	groups := []group{}
	indices, aliases, agents, counts := map[string]int{}, map[string]bool{}, map[string]bool{}, map[string]int{}
	order := []string{}
	req := prompt.ConfirmationRequest{Title: "Install these Skills?", Default: true}
	for _, action := range review.Actions {
		kind := string(action.Action)
		key := kind + "\x00" + action.Alias
		index, found := indices[key]
		if !found {
			index = len(groups)
			indices[key] = index
			groups = append(groups, group{action: kind, alias: action.Alias})
		}
		groups[index].targets = append(groups[index].targets, prompt.SafeText(reviewClientName(domain.ClientID(action.TargetId))))
		aliases[action.Alias], agents[action.TargetId] = true, true
		if counts[kind] == 0 {
			order = append(order, kind)
		}
		counts[kind]++
		req.Details = append(req.Details, skillsActionDetail(action))
	}
	req.Summary = append(req.Summary, fmt.Sprintf("%d Skills · %d agents · %d bindings", len(aliases), len(agents), len(review.Actions)))
	changes := make([]string, 0, len(order))
	for _, kind := range order {
		changes = append(changes, fmt.Sprintf("%d %s", counts[kind], prompt.SafeText(kind)))
	}
	req.Summary = append(req.Summary, strings.Join(changes, " · "), "")
	for _, group := range groups {
		req.Summary = append(req.Summary, prompt.SafeText(group.action)+" "+prompt.SafeText(group.alias), "  → "+strings.Join(group.targets, ", "))
	}
	for _, warning := range review.Warnings {
		req.Summary = append(req.Summary, "Warning: "+prompt.SafeText(warning.Message))
	}
	for _, conflict := range review.Conflicts {
		req.Summary = append(req.Summary, "Conflict: "+prompt.SafeText(conflict.Message))
	}
	return req
}

func skillsActionDetail(action skillspicker.Action) string {
	// Paths are authority: sanitize control sequences without shortening them.
	path := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return -1
		}
		return r
	}, action.DestinationPath)
	return fmt.Sprintf("%s %s → %s (%s)", prompt.SafeText(string(action.Action)), prompt.SafeText(action.Alias), prompt.SafeText(reviewClientName(domain.ClientID(action.TargetId))), path)
}
