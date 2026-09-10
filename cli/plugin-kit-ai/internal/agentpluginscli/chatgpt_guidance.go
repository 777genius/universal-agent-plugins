package agentpluginscli

import (
	"strings"
	"unicode"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Rebuild the actual request, including selected interactive targets. Never
// echo a personal ID; the only added value is a registration placeholder.
func chatGPTRegistrationResumeAction(cmd *cobra.Command, source string, targets []domain.ClientID) string {
	targetOption := strings.Join(clientIDStrings(targets), ",")
	if cmd.Flags().Changed("target") {
		targetOption, _ = cmd.Flags().GetString("target")
	}
	args := []string{"add", quoteResumeArgument(source), "--target", quoteResumeArgument(targetOption), "--prepare"}
	cmd.Flags().Visit(func(flag *pflag.Flag) {
		switch flag.Name {
		case "target", "prepare", "chatgpt-app-id":
			return
		}
		args = append(args, "--"+flag.Name+"="+quoteResumeArgument(flag.Value.String()))
	})
	args = append(args, "--chatgpt-app-id", "<ID>")
	return strings.Replace(domain.ChatGPTRegistrationAction, "add context7 --target chatgpt --prepare --chatgpt-app-id <ID>", strings.Join(args, " "), 1)
}

func clientIDStrings(targets []domain.ClientID) []string {
	values := make([]string, len(targets))
	for i, target := range targets {
		values[i] = string(target)
	}
	return values
}

func quoteResumeArgument(value string) string {
	if value != "" && strings.IndexFunc(value, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune("_./:@,+-=", r)
	}) == -1 {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
