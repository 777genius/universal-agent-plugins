package agentpluginscli

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// preparedOnlySuffix marks a client whose files are written but whose runtime
// connection the CLI cannot verify. Planning appends it to the display name;
// the picker presents it as a gray note and a short hint instead.
const preparedOnlySuffix = " (prepare only; manual verification)"

const (
	preparedOnlyNote = "prepare only"
	preparedOnlyInfo = "verify manually"
)

// targetChoiceFor turns a detected client into a picker choice, moving the
// prepared-only marker out of the label into its annotation fields.
func targetChoiceFor(client domain.DetectedClient, annotations ...map[domain.ClientID]prompt.TargetChoice) prompt.TargetChoice {
	choice := prompt.TargetChoice{ID: client.ClientID, Label: prompt.SafeText(client.DisplayName)}
	if label, ok := strings.CutSuffix(client.DisplayName, preparedOnlySuffix); ok {
		choice.Label, choice.Note, choice.Info = prompt.SafeText(label), preparedOnlyNote, preparedOnlyInfo
	}
	if len(annotations) > 0 {
		annotation := annotations[0][client.ClientID]
		choice.Note = joinTargetNotes(choice.Note, annotation.Note)
		choice.Info = joinTargetNotes(choice.Info, annotation.Info)
	}
	return choice
}

func joinTargetNotes(left, right string) string {
	if left != "" && right != "" {
		return left + "; " + right
	}
	return left + right
}
