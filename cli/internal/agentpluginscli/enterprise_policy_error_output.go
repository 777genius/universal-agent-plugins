package agentpluginscli

import (
	"io"

	"github.com/spf13/cobra"
)

// Existing command renderers own their single result document. Observe only
// JSON writes to prevent the process fallback from appending another document
// for a policy error discovered during apply. Human and TTY writers are intact.
func configureEnterprisePolicyFailureOutput(command *cobra.Command, opts *options) {
	for _, child := range command.Commands() {
		configureEnterprisePolicyFailureOutput(child, opts)
	}
	if command.RunE == nil {
		return
	}
	run := command.RunE
	command.RunE = func(cmd *cobra.Command, args []string) error {
		if opts.format != "json" {
			return run(cmd, args)
		}
		original := cmd.OutOrStdout()
		output := &enterprisePolicyOutputWriter{writer: original}
		cmd.SetOut(output)
		defer cmd.SetOut(original)
		err := run(cmd, args)
		if _, policy := enterprisePolicyAgentFailure(err); policy && output.attempted {
			return &enterprisePolicyReportedError{cause: err}
		}
		return err
	}
}

type enterprisePolicyOutputWriter struct {
	writer    io.Writer
	attempted bool
}

func (output *enterprisePolicyOutputWriter) Write(body []byte) (int, error) {
	output.attempted = true
	return output.writer.Write(body)
}

type enterprisePolicyReportedError struct{ cause error }

func (*enterprisePolicyReportedError) Error() string {
	return "Installation policy stopped the operation; inspect the reported result."
}

func (err *enterprisePolicyReportedError) Unwrap() error { return err.cause }
