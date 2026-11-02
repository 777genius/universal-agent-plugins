package agentpluginscli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

type dependencyTransferPublicationError struct {
	cause   error
	partial bool
}

func (err *dependencyTransferPublicationError) Error() string { return err.cause.Error() }
func (err *dependencyTransferPublicationError) Unwrap() error { return err.cause }

func (transfer *dependencyTransfer) publish(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := transfer.check(); err != nil {
		return err
	}
	destination := transfer.destination
	if destination.directoryInfo == nil {
		if err := os.MkdirAll(destination.directory, 0o700); err != nil {
			return err
		}
		var err error
		destination.directoryInfo, err = os.Stat(destination.directory)
		if err != nil {
			return err
		}
	}
	attempted := [2]bool{}
	for i := range destination.files {
		if bytes.Equal(transfer.source.files[i].body, destination.files[i].body) && destination.files[i].info != nil {
			continue
		}
		if err := transfer.checkPublicationStep(ctx, destination); err != nil {
			return transfer.rollback(attempted, err)
		}
		attempted[i] = true
		if err := publishEnvironmentFile(destination.files[i].path, transfer.source.files[i].body); err != nil {
			return transfer.rollback(attempted, err)
		}
		current, err := readDependencyTransferFile(destination.files[i].path)
		if err != nil {
			return transfer.rollback(attempted, err)
		}
		if !bytes.Equal(current.body, transfer.source.files[i].body) {
			return transfer.rollback(attempted, installer.ErrPlanChanged)
		}
		destination.files[i] = current
	}
	transfer.output.AgreementSaved = true
	return nil
}

func (transfer dependencyTransfer) checkPublicationStep(ctx context.Context, expected dependencyTransferPair) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := transfer.source.check(); err != nil {
		return err
	}
	return expected.check()
}

func (transfer dependencyTransfer) rollback(attempted [2]bool, cause error) error {
	failures := []error{}
	for i := len(attempted) - 1; i >= 0; i-- {
		if !attempted[i] {
			continue
		}
		if err := restoreDependencyTransferFile(transfer.destination.files[i], transfer.source.files[i].body); err != nil {
			failures = append(failures, err)
		}
	}
	if len(failures) == 0 {
		return &dependencyTransferPublicationError{cause: fmt.Errorf("agreement publication failed; previous agreement restored: %w", cause)}
	}
	detail := errors.Join(failures...)
	return &dependencyTransferPublicationError{partial: true, cause: fmt.Errorf("partial agreement publication; rollback incomplete (%w); remaining files: %s; retry the same validated source %s with explicit confirmation: %w", detail, transfer.remainingFileIdentities(), transfer.source.directory, cause)}
}

func restoreDependencyTransferFile(before dependencyTransferFile, candidate []byte) error {
	current, err := readDependencyTransferFile(before.path)
	if err != nil {
		return fmt.Errorf("%s: %w", before.path, err)
	}
	if sameDependencyTransferPreimage(before, current) {
		return nil
	}
	if current.info == nil || !bytes.Equal(current.body, candidate) {
		return fmt.Errorf("%s changed externally; retained current file", before.path)
	}
	expectedMode := os.FileMode(0o600)
	if runtime.GOOS == "windows" {
		expectedMode = 0o666
	}
	if before.info != nil {
		expectedMode = before.info.Mode().Perm()
	}
	if current.info.Mode().Perm() != expectedMode {
		return fmt.Errorf("%s mode changed externally; retained current file", before.path)
	}
	if before.info == nil {
		err = os.Remove(before.path)
	} else {
		err = publishEnvironmentFile(before.path, before.body)
	}
	// Atomic replacement can succeed before a parent-directory sync reports failure.
	restored, readErr := readDependencyTransferFile(before.path)
	if readErr == nil && sameDependencyTransferPreimage(before, restored) {
		return nil
	}
	return fmt.Errorf("%s not restored: %w", before.path, errors.Join(err, readErr, errors.New("preimage mismatch")))
}

func sameDependencyTransferPreimage(before, current dependencyTransferFile) bool {
	if before.info == nil || current.info == nil {
		return before.info == nil && current.info == nil
	}
	return before.info.Mode().Perm() == current.info.Mode().Perm() && bytes.Equal(before.body, current.body)
}

func (transfer dependencyTransfer) remainingFileIdentities() string {
	states := make([]string, 0, len(transfer.destination.files))
	for _, expected := range transfer.destination.files {
		current, err := readDependencyTransferFile(expected.path)
		if err != nil {
			states = append(states, fmt.Sprintf("%s unreadable: %v", expected.path, err))
			continue
		}
		identity := dependencyTransferFileIdentity(current)
		states = append(states, fmt.Sprintf("%s exists=%t bytes=%d sha256=%s", expected.path, identity.Exists, identity.Bytes, identity.SHA256))
	}
	return strings.Join(states, "; ")
}
