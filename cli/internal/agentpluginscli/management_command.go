package agentpluginscli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/managementhttp"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/managementstate"
)

func newManagementCommand(app App) *cobra.Command {
	command := &cobra.Command{Use: "manage", Short: "Embed the versioned UAP management API"}
	command.AddCommand(&cobra.Command{Use: "discover", Short: "Observe private user-profile facts without executing clients", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if cmd.Flags().NFlag() != 0 {
			return errors.New("management discovery accepts no target, scope or lifecycle options")
		}
		return runManagementDiscovery(cmd.Context(), app, cmd.OutOrStdout())
	}})
	command.AddCommand(&cobra.Command{Use: "serve", Short: "Serve a private captured management context", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return runManagementServer(cmd.Context(), app, cmd.InOrStdin(), cmd.OutOrStdout())
	}})
	return command
}
func readManagementStartup(reader *bufio.Reader) (management.Startup, error) {
	line, err := reader.ReadSlice('\n')
	if err != nil || len(line) > 64*1024 {
		return management.Startup{}, errors.New("startup requires one JSON line of at most 64 KiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.DisallowUnknownFields()
	var cfg management.Startup
	if err = decoder.Decode(&cfg); err != nil {
		return cfg, errors.New("invalid management startup configuration")
	}
	if err = decoder.Decode(&struct{}{}); err != io.EOF {
		return cfg, errors.New("startup must contain one JSON document")
	}
	return cfg, nil
}
func runManagementServer(ctx context.Context, app App, input io.Reader, output io.Writer) error {
	reader := bufio.NewReaderSize(input, 64*1024+1)
	cfg, err := readManagementStartup(reader)
	if err != nil {
		return err
	}
	if app.ManagementFactory == nil {
		return errors.New("management composition is unavailable")
	}
	configured, err := app.ManagementFactory(cfg)
	if err != nil {
		return err
	}
	backend, err := NewManagementBackend(configured, cfg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	application, err := composeManagementApplication(ctx, backend, configured.Version)
	if err != nil {
		backend.Close()
		return err
	}
	server, err := managementhttp.Start(ctx, application)
	if err != nil {
		application.Close()
		return err
	}
	defer server.Close()
	if err = json.NewEncoder(output).Encode(server.Bootstrap()); err != nil {
		return err
	}
	go func() { _, _ = io.Copy(io.Discard, reader); cancel() }()
	select {
	case <-server.Done():
	case <-ctx.Done():
	}
	return nil
}

// The caller owns backend cleanup until successful application construction.
// A successful application is handed to the HTTP host or closed by its caller.
func composeManagementApplication(ctx context.Context, backend *ManagementBackend, version string) (*management.Application, error) {
	store, err := managementstate.NewStore(backend.config.StateRoot, backend.EnvironmentID())
	if err != nil {
		return nil, err
	}
	lock, err := managementstate.NewLock(backend.config.StateRoot)
	if err != nil {
		return nil, err
	}
	return management.NewApplication(ctx, management.Options{Backend: backend, Store: store, Lock: lock,
		ContextID: backend.ContextID(), EnvironmentID: backend.EnvironmentID(),
		IntentRoot: backend.config.DependenciesRoot, EngineVersion: version})
}
