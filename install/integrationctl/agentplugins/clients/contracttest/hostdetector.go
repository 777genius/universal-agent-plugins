package contracttest

import (
	"fmt"
	"io/fs"
	"os"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
)

// RunHostDetector asserts the detection half of the contract: surface ids are
// unique, the observation is deterministic, the selection list names surfaces
// that were actually reported, and the only way the adapter touches the machine
// is through the host probes.
//
// It runs on each supported operating system against a host whose probes answer
// "nothing is installed", because the properties under test are structural.
// What each client reports for a real installation is frozen by the detector's
// golden files instead.
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

func platformViolations(detector clients.HostDetector, goos string) []string {
	probes := &countingProbes{goos: goos}
	first := detector.DetectSurfaces(clients.NewHost(probes.hostProbes()))
	observed := probes.counts

	probes.counts = probeCounts{}
	second := detector.DetectSurfaces(clients.NewHost(probes.hostProbes()))

	violations := []string{}
	if !reflect.DeepEqual(first, second) {
		violations = append(violations, fmt.Sprintf("on %s two probes of the same host produced different detections; surface order and content must be deterministic", goos))
	}
	if observed != probes.counts {
		violations = append(violations, fmt.Sprintf("on %s the adapter probed the host %+v times and then %+v times; detection must not depend on call history", goos, observed, probes.counts))
	}
	for _, violation := range detectionViolations(first) {
		violations = append(violations, "on "+goos+": "+violation)
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

// probeCounts records how often an adapter reached for the machine. Anything an
// adapter observes has to go through one of these three, so a client that read
// a file on its own would leave the counters untouched and be caught by the
// determinism check instead.
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
