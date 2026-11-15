// Command hermes_install_runner reuses the production process supervisor for
// pinned, disposable native-install acceptance commands. It never plans effects.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type request struct {
	Argv           []string `json:"argv"`
	Directory      string   `json:"directory"`
	TimeoutSeconds int      `json:"timeout_seconds"`
	Limit          int      `json:"limit"`
}

type response struct {
	Result legacyports.CommandResult `json:"result"`
	Error  string                    `json:"error,omitempty"`
}

func main() {
	var req request
	decoder := json.NewDecoder(os.Stdin)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		fail(err)
	}
	if len(req.Argv) == 0 || req.TimeoutSeconds < 1 || req.TimeoutSeconds > 9000 || req.Limit < 1 || req.Limit > 16<<20 {
		fail(fmt.Errorf("invalid bounded native acceptance request"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(req.TimeoutSeconds)*time.Second)
	defer cancel()
	result, err := (processadapter.OS{}).Run(ctx, legacyports.Command{Argv: req.Argv, Dir: req.Directory, Env: os.Environ(), StdoutLimitBytes: req.Limit})
	out := response{Result: result}
	if err != nil {
		out.Error = err.Error()
	}
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		fail(err)
	}
	if err != nil || result.ExitCode != 0 || result.NoChildStarted || !result.CleanupComplete {
		os.Exit(1)
	}
}

func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
