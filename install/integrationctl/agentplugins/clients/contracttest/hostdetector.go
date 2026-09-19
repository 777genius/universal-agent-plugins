package contracttest

import (
	"fmt"
	"io/fs"
	"os"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
)

// RunHostDetector asserts the detection half of the contract on each supported
// operating system, against a host whose probes answer "nothing is installed":
//
//   - surface ids are non-empty and unique, and an undetected surface carries no
//     evidence;
//   - every selection surface is one of the reported surfaces;
//   - repeated observations of the same host are identical, in content, in
//     surface order and in the number of probe calls;
//   - nothing is reported as present, because the host says nothing is.
//
// The last one is how far this harness gets towards "the adapter observes the
// machine only through clients.Host". It catches the case that matters - an
// adapter that stats a real path or resolves a real binary behind the host's
// back reports evidence the host never gave it - but it is a necessary
// condition, not a proof: an ambient read whose result does not reach the
// Detection is invisible here.
//
// The properties under test are structural. What each client reports for a real
// installation is frozen by the detector's golden files instead.
func RunHostDetector(t *testing.T, adapter clients.Adapter) {
	t.Helper()
	RunAdapter(t, adapter)
	for _, violation := range hostDetectorViolations(adapter) {
		t.Error(violation)
	}
}

func hostDetectorViolations(adapter clients.Adapter) []string {
	detector, ok := adapter.(clients.HostDetector)
	if !ok {
		return []string{"adapter does not implement clients.HostDetector"}
	}
	violations := []string{}
	for _, goos := range []string{"darwin", "linux", "windows"} {
		violations = append(violations, platformViolations(detector, goos)...)
	}
	return violations
}

// detectionRuns is how often one host is observed. A single repeat catches a
// deterministic mistake but only coin-flips on a map-backed one, and a flaky
// adapter is exactly the kind this harness exists to stop from reaching the
// golden files.
const detectionRuns = 8

func platformViolations(detector clients.HostDetector, goos string) []string {
	probes := &countingProbes{goos: goos}
	first := detector.DetectSurfaces(clients.NewHost(probes.hostProbes()))
	observed := probes.counts

	violations := []string{}
	for run := 2; run <= detectionRuns; run++ {
		probes.counts = probeCounts{}
		repeated := detector.DetectSurfaces(clients.NewHost(probes.hostProbes()))
		if !reflect.DeepEqual(first, repeated) {
			violations = append(violations, fmt.Sprintf("on %s run %d observed a different detection than run 1; surface order and content must be deterministic", goos, run))
			break
		}
		if observed != probes.counts {
			violations = append(violations, fmt.Sprintf("on %s run %d probed the host %+v times against %+v in run 1; detection must not depend on call history", goos, run, probes.counts, observed))
			break
		}
	}
	violations = append(violations, emptyHostViolations(first, goos)...)
	for _, violation := range detectionViolations(first) {
		violations = append(violations, "on "+goos+": "+violation)
	}
	return violations
}

// emptyHostViolations holds the adapter to the host it was given. Every probe
// answered "does not exist", so anything the adapter reports as present was
// observed somewhere the harness cannot reproduce - normally the real machine.
func emptyHostViolations(detection clients.Detection, goos string) []string {
	violations := []string{}
	if detection.ExecutablePath != "" {
		violations = append(violations, fmt.Sprintf("on %s the adapter reported the executable %q although the host resolves no binary; it may observe only through clients.Host", goos, detection.ExecutablePath))
	}
	for _, surface := range detection.Surfaces {
		if surface.Detected {
			violations = append(violations, fmt.Sprintf("on %s surface %q is detected although the host reports nothing installed; it may observe only through clients.Host", goos, surface.ID))
		}
	}
	return violations
}

func detectionViolations(detection clients.Detection) []string {
	violations := []string{}
	seen := make(map[string]struct{}, len(detection.Surfaces))
	for _, surface := range detection.Surfaces {
		if surface.ID == "" {
			violations = append(violations, "a surface has an empty id")
			continue
		}
		if _, duplicate := seen[surface.ID]; duplicate {
			violations = append(violations, fmt.Sprintf("surface %q is reported twice; a surface id is what the plan and the CLI key evidence by", surface.ID))
		}
		seen[surface.ID] = struct{}{}
		if !surface.Detected && surface.Evidence != "" {
			violations = append(violations, fmt.Sprintf("surface %q is not detected but still carries the evidence %q", surface.ID, surface.Evidence))
		}
	}
	for _, id := range detection.SelectionSurfaceIDs {
		if _, reported := seen[id]; !reported {
			violations = append(violations, fmt.Sprintf("selection surface %q is not among the reported surfaces, so it can never select the client", id))
		}
	}
	return violations
}

// probeCounts records how often an adapter asked the host a question. It is a
// determinism signal, not a sandbox: a read that bypasses the host leaves it
// untouched, which is what emptyHostViolations is for.
type probeCounts struct {
	LookPath int
	Lstat    int
	ReadDir  int
}

type countingProbes struct {
	goos   string
	counts probeCounts
}

func (p *countingProbes) hostProbes() clients.HostProbes {
	return clients.HostProbes{
		HomeDir:               "/contracttest/home",
		GOOS:                  p.goos,
		Environment:           map[string]string{},
		SystemApplicationsDir: "/contracttest/Applications",
		WindowsProgramFiles:   []string{`C:\Program Files`},
		LinuxApplicationDirs:  []string{"/contracttest/share/applications"},
		LookPath: func(string) (string, error) {
			p.counts.LookPath++
			return "", os.ErrNotExist
		},
		Lstat: func(string) (fs.FileInfo, error) {
			p.counts.Lstat++
			return nil, os.ErrNotExist
		},
		ReadDir: func(string) ([]os.DirEntry, error) {
			p.counts.ReadDir++
			return nil, os.ErrNotExist
		},
	}
}
