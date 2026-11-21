package agentpluginscli

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// unmanagedBindingError is the user-facing form of environment.ForeignBindingError.
// It keeps the typed cause reachable through errors.As.
type unmanagedBindingError struct {
	cause   *environment.ForeignBindingError
	message string
}

func (e unmanagedBindingError) Error() string { return e.message }
func (e unmanagedBindingError) Unwrap() error { return e.cause }

// explainForeignBinding names the clients that already hold the dependency
// outside the saved list and the exact command that removes each installation.
// Other errors pass through unchanged; the environment's decision is not altered.
func explainForeignBinding(app App, err error) error {
	var foreign *environment.ForeignBindingError
	if err == nil || !errors.As(err, &foreign) || len(foreign.Bindings) == 0 {
		return err
	}
	byInstallation := map[string][]string{}
	var order []string
	for _, binding := range foreign.Bindings {
		if _, ok := byInstallation[binding.InstallationID]; !ok {
			order = append(order, binding.InstallationID)
		}
		byInstallation[binding.InstallationID] = append(byInstallation[binding.InstallationID], binding.ClientID)
	}
	scope := ""
	if app.scope() == domain.ScopeProject {
		scope = " --scope project"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s is already installed outside your dependency list", foreign.Alias)
	var commands []string
	for _, id := range order {
		clients := byInstallation[id]
		sort.Strings(clients)
		names, targets := make([]string, len(clients)), make([]string, len(clients))
		for i, client := range clients {
			names[i], targets[i] = domain.ClientDisplayName(domain.ClientID(client)), publicCLITargetName(domain.ClientID(client))
		}
		fmt.Fprintf(&b, "\n  %s (installation %s)", strings.Join(names, ", "), id)
		commands = append(commands, fmt.Sprintf("  agentplugins remove %s --target %s%s", id, strings.Join(targets, ","), scope))
	}
	b.WriteString("\nRemove it first (the command lists any step to finish in the client), then run this command again; plugin data is kept:\n")
	b.WriteString(strings.Join(commands, "\n"))
	return unmanagedBindingError{cause: foreign, message: b.String()}
}
