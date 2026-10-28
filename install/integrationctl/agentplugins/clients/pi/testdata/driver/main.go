// Command driver is a disposable integration-test host for the public common
// installer. It is not a shipped installer or an alternate lifecycle engine.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/pi"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func main() {
	if len(os.Args) != 7 {
		fmt.Fprintln(os.Stderr, "usage: driver STATE PROFILE SOURCE OPERATION INSTALLATION_ID OUTPUT")
		os.Exit(2)
	}
	registry, err := clients.NewRegistry(pi.New())
	if err != nil {
		fail(err)
	}
	helper, err := os.Executable()
	if err != nil {
		fail(err)
	}
	engine, err := installer.New(installer.Config{StateRoot: os.Args[1], Registry: registry, HelperExecutable: helper, TrustedLocalPackages: true})
	if err != nil {
		fail(err)
	}
	prepared, err := engine.Prepare(context.Background(), installer.Request{Operation: installer.Operation(os.Args[4]), ClientID: "pi", ClientConfigRoot: os.Args[2], PackageRoot: os.Args[3], InstallationID: os.Args[5]})
	if err != nil {
		fail(err)
	}
	defer prepared.Close()
	result, err := engine.Apply(context.Background(), prepared, installer.Decision{Confirmed: true})
	if err != nil {
		fail(err)
	}
	body, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fail(err)
	}
	if err := os.WriteFile(os.Args[6], append(body, '\n'), 0600); err != nil {
		fail(err)
	}
}
func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
