package commands

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/report"
	"github.com/777genius/plugin-kit-ai/cli/internal/exitx"
)

// privateReportSummary owns the shared private status line. Human formats
// retain their separate write/flush contracts while using identical fields.
func privateReportSummary(r report.Report) string {
	return fmt.Sprintf("%s: readiness %s; conformance %s; host %s; compatibility %s; toolchain %s; runtime %s\n", r.Command, r.Readiness.Status, r.Conformance.Status, r.HostSafety.Status, r.Compatibility.Status, r.Toolchain.Status, r.Runtime.Status)
}

// writePrivateDetailed keeps the private CLI's line-by-line output behavior.
// It is separate from Execute so Cobra routing and status mapping remain
// independent of human presentation. Do not use raw fields for public output.
func writePrivateDetailed(w io.Writer, r report.Report) error {
	if err := writePrivateDetailedSummary(w, r); err != nil {
		return err
	}
	if err := writePrivateDetailedCapabilities(w, r); err != nil {
		return err
	}
	return writePrivateDetailedClients(w, r)
}

func writePrivateDetailedSummary(w io.Writer, r report.Report) error {
	if _, e := io.WriteString(w, privateReportSummary(r)); e != nil {
		return exitx.Wrap(errors.New("authoring output failed"), 1)
	}
	for _, f := range r.Findings {
		if _, e := fmt.Fprintf(w, "%s: %s (%s)\n", f.Severity, f.Code, f.Layer); e != nil {
			return exitx.Wrap(errors.New("authoring output failed"), 1)
		}
	}
	for _, component := range r.Components {
		if _, e := fmt.Fprintf(w, "%s %s: %s; requirements %s\n", component.Type, component.ID, component.Status, strings.Join(component.Requirements, ", ")); e != nil {
			return exitx.Wrap(errors.New("authoring output failed"), 1)
		}
	}
	return nil
}

func writePrivateDetailedCapabilities(w io.Writer, r report.Report) error {
	if r.Capabilities != nil {
		if _, e := fmt.Fprintf(w, "commands: %s\n", strings.Join(r.Capabilities.Commands, ", ")); e != nil {
			return exitx.Wrap(e, 1)
		}
		for _, s := range r.Capabilities.Schemas {
			if _, e := fmt.Fprintf(w, "schema: %s; %s\n", s.ID, s.Digest); e != nil {
				return exitx.Wrap(e, 1)
			}
		}
		for _, p := range r.Capabilities.Profiles {
			if _, e := fmt.Fprintf(w, "profile: %s; revision %s; %s\n", p.ID, p.Revision, p.Digest); e != nil {
				return exitx.Wrap(e, 1)
			}
		}
		for _, c := range r.Capabilities.Clients {
			if _, e := fmt.Fprintf(w, "client: %s; activation: %s\n", c.ClientID, c.ActivationMode); e != nil {
				return exitx.Wrap(e, 1)
			}
		}
	}
	return nil
}

func writePrivateDetailedClients(w io.Writer, r report.Report) error {
	for _, c := range r.Clients {
		if _, e := fmt.Fprintf(w, "target %s: static support only; %s\n", c.ClientID, strings.Join(c.Limitations, ", ")); e != nil {
			return exitx.Wrap(e, 1)
		}
		for _, v := range c.Components {
			if _, e := fmt.Fprintf(w, "  %s %d: %s\n", v.Kind, v.Index, v.Support); e != nil {
				return exitx.Wrap(e, 1)
			}
		}
	}
	for _, c := range r.DoctorChecks {
		if _, e := fmt.Fprintf(w, "%s: %s; %s\n", c.ID, c.Status, c.Action); e != nil {
			return exitx.Wrap(e, 1)
		}
	}
	if r.Error != nil {
		if _, e := fmt.Fprintln(w, r.Error.Action); e != nil {
			return exitx.Wrap(errors.New("authoring output failed"), 1)
		}
	}
	return nil
}
