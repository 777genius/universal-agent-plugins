// Command project-driver is a disposable native-E2E host. It invokes UAP's
// public installer facade, not a separate Pi package manager.
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "os"

    "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
    "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/pi"
    "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
    "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func main() {
    if len(os.Args) != 8 {
        fmt.Fprintln(os.Stderr, "usage: project-driver STATE USER_PROFILE PROJECT SOURCE OPERATION INSTALLATION_ID OUTPUT")
        os.Exit(2)
    }
    registry, err := clients.NewRegistry(pi.NewProject())
    if err != nil { fail(err) }
    helper, err := os.Executable()
    if err != nil { fail(err) }
    engine, err := installer.New(installer.Config{
        ProjectRoot: os.Args[3], StateRoot: os.Args[1],
        Registry: registry, HelperExecutable: helper, TrustedLocalPackages: true,
    })
    if err != nil { fail(err) }
    request := installer.Request{
        Scope: domain.ScopeProject, ProjectRoot: os.Args[3],
        Operation: installer.Operation(os.Args[5]), ClientID: "pi",
        ClientConfigRoot: os.Args[2], PackageRoot: os.Args[4],
        InstallationID: os.Args[6],
    }
    prepared, err := engine.Prepare(context.Background(), request)
    if err != nil { fail(err) }
    result, applyErr := engine.Apply(context.Background(), prepared, installer.Decision{Confirmed: true})
    closeErr := prepared.Close()
    if applyErr != nil { fail(applyErr) }
    if closeErr != nil { fail(closeErr) }
    if result.Outcome != installer.OutcomeCompleted && result.Outcome != installer.OutcomeUnchanged {
        fail(fmt.Errorf("incomplete UAP Pi project operation: %s", result.Outcome))
    }
    body, err := json.Marshal(result)
    if err != nil { fail(err) }
    if err := os.WriteFile(os.Args[7], append(body, '\n'), 0600); err != nil { fail(err) }
}
func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
