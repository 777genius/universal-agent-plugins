package agentpluginscli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/spf13/cobra"
)

// A shared security refusal may render its own JSON. Keep that diagnostic inside
// the transfer result rather than emitting a second or unrelated result object.
func runDependencyTransferInstallOutput(cmd *cobra.Command, app App, opts *options, transfer *dependencyTransfer, yes bool) error {
	if opts.format != "json" {
		return runDependencyTransferInstall(cmd, app, opts, transfer, yes)
	}
	writer := cmd.OutOrStdout()
	var captured bytes.Buffer
	cmd.SetOut(&captured)
	err := runDependencyTransferInstall(cmd, app, opts, transfer, yes)
	cmd.SetOut(writer)
	if err == nil || captured.Len() == 0 {
		_, writeErr := writer.Write(captured.Bytes())
		return errors.Join(err, writeErr)
	}
	diagnostics, decodeErr := dependencyTransferDiagnostics(captured.Bytes())
	if decodeErr != nil {
		return errors.Join(err, decodeErr)
	}
	transfer.output.Diagnostics = diagnostics
	transfer.output.Error = err.Error()
	transfer.output.Outcome = "refused"
	if transfer.output.AgreementSaved || len(diagnostics) == 0 {
		transfer.output.Outcome = "partial"
	}
	return errors.Join(err, renderDependencyTransfer(cmd, opts, transfer.output))
}

func dependencyTransferDiagnostics(body []byte) ([]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	diagnostics := []json.RawMessage{}
	for {
		var raw json.RawMessage
		if err := decoder.Decode(&raw); errors.Is(err, io.EOF) {
			return diagnostics, nil
		} else if err != nil {
			return nil, err
		}
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return nil, err
		}
		if _, ok := envelope["agreement_saved"]; !ok {
			diagnostics = append(diagnostics, raw)
		}
	}
}
