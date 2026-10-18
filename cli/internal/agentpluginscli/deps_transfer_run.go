package agentpluginscli

import (
	"bufio"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
)

func runDependencyTransfer(cmd *cobra.Command, app App, opts *options, transfer *dependencyTransfer, yes bool) error {
	if opts.dryRun {
		return renderDependencyTransfer(cmd, opts, transfer.output)
	}
	if !transfer.output.Changed {
		transfer.output.Outcome = "unchanged"
		return renderDependencyTransfer(cmd, opts, transfer.output)
	}
	if transfer.output.Operation == "import" {
		if err := requireDependencyTransferReady(cmd, app); err != nil {
			return err
		}
	}
	confirmed, err := confirmDependencyTransfer(cmd, app, opts, transfer, yes)
	if err != nil || !confirmed {
		return err
	}
	release, err := lockDependencyTransfer(cmd, app, transfer)
	if err != nil {
		return err
	}
	defer release()
	if transfer.output.Operation == "import" {
		if err := requireDependencyTransferReady(cmd, app); err != nil {
			return err
		}
	}
	if err := transfer.publish(cmd.Context()); err != nil {
		return renderDependencyTransferFailure(cmd, opts, transfer, err)
	}
	transfer.output.Outcome = "completed"
	return renderDependencyTransfer(cmd, opts, transfer.output)
}

func requireDependencyTransferReady(cmd *cobra.Command, app App) error {
	if err := requireDependencyMutationReady(app); err != nil {
		return err
	}
	if app.Installer != nil {
		return requireDependencyRecoveryClear(cmd, app)
	}
	return nil
}

func lockDependencyTransfer(cmd *cobra.Command, app App, transfer *dependencyTransfer) (func(), error) {
	if err := cmd.Context().Err(); err != nil {
		return nil, err
	}
	if transfer.output.Operation != "import" {
		return func() {}, nil
	}
	unlock, err := processlock.Lock{Path: app.EnvironmentLockPath}.Acquire(cmd.Context())
	if err != nil {
		return nil, err
	}
	if err := transfer.observeImportLockDirectory(); err != nil {
		_ = unlock()
		return nil, err
	}
	return func() { _ = unlock() }, nil
}

func confirmDependencyTransfer(cmd *cobra.Command, app App, opts *options, transfer *dependencyTransfer, yes bool) (bool, error) {
	if opts.format == "human" {
		if err := renderDependencyTransfer(cmd, opts, transfer.output); err != nil {
			return false, err
		}
		if transfer.output.InstallRequested {
			if err := renderDependencyConfirmation(cmd, opts, transfer.prepared); err != nil {
				return false, err
			}
		}
	}
	if yes {
		return true, cmd.Context().Err()
	}
	if !app.Terminal || opts.format != "human" {
		return false, errors.New("dependency agreement transfer requires --yes or explicit interactive confirmation")
	}
	confirmed, err := promptYesNo(cmd.Context(), bufio.NewReader(cmd.InOrStdin()), cmd.OutOrStdout(), cmd.ErrOrStderr(), "Apply this dependency agreement transfer and any requested locked installation? [y/N]")
	if err != nil {
		return false, err
	}
	if !confirmed {
		transfer.output.Outcome = "canceled"
		_, err = fmt.Fprintln(cmd.OutOrStdout(), "No changes made.")
	}
	return confirmed, err
}

func renderDependencyTransferFailure(cmd *cobra.Command, opts *options, transfer *dependencyTransfer, err error) error {
	var publication *dependencyTransferPublicationError
	if errors.As(err, &publication) && publication.partial {
		transfer.output.Outcome = "partial"
		if renderErr := renderDependencyTransfer(cmd, opts, transfer.output); renderErr != nil {
			return errors.Join(err, renderErr)
		}
	}
	return err
}
