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
)

func newManagementCommand(app App) *cobra.Command {
	command := &cobra.Command{Use: "manage", Short: "Embed the versioned UAP management API"}
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
	defer backend.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	server, err := management.Start(ctx, management.Options{Backend: backend, ContextID: backend.ContextID(), EnvironmentID: backend.EnvironmentID(), IntentRoot: backend.config.DependenciesRoot, StateRoot: backend.config.StateRoot, EngineVersion: configured.Version})
	if err != nil {
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
