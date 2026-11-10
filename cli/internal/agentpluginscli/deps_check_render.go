package agentpluginscli

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

func finishDependencyCheck(cmd *cobra.Command, opts *options, report dependencyCheckReport) error {
	var renderErr error
	if opts.format == "json" {
		renderErr = renderJSON(cmd.OutOrStdout(), report)
	} else {
		renderErr = renderDependencyCheck(cmd.OutOrStdout(), report)
	}
	if report.Outcome != dependencyCheckValid {
		return errors.Join(errDependencyCheckFailed, renderErr)
	}
	return renderErr
}

func renderDependencyCheck(writer io.Writer, report dependencyCheckReport) error {
	if _, err := fmt.Fprintf(writer, "Local dependency integrity (%s): %s\n", report.Scope, report.Outcome); err != nil {
		return err
	}
	for _, check := range report.Checks {
		if err := renderDependencyCheckResult(writer, check); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(writer, "Checks cover the selected local agreement and cached package bytes. They do not establish publisher trust, current revocation status, native activation or authentication. No package sources were fetched.")
	return err
}

func renderDependencyCheckResult(writer io.Writer, check dependencyCheckResult) error {
	name := check.Stage
	if check.Alias != "" {
		name += "/" + check.Alias
	}
	if _, err := fmt.Fprintf(writer, "  %s: %s", name, check.Outcome); err != nil {
		return err
	}
	if check.Code != "" {
		if _, err := fmt.Fprintf(writer, " (%s)", check.Code); err != nil {
			return err
		}
	}
	if check.Path != "" {
		if _, err := fmt.Fprintf(writer, " %q", check.Path); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(writer); err != nil {
		return err
	}
	if check.Message != "" {
		_, err := fmt.Fprintln(writer, "    "+check.Message)
		return err
	}
	return nil
}
