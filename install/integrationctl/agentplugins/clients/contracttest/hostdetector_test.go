package contracttest

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// detectingAdapter reports whatever the test hands it, so the harness can be
// shown to reject a bad observation and not only to accept a good one.
type detectingAdapter struct {
	exampleAdapter
	detect func(clients.Host) clients.Detection
}

func (a detectingAdapter) DetectSurfaces(host clients.Host) clients.Detection { return a.detect(host) }

func TestRunHostDetectorAcceptsAWellBehavedAdapter(t *testing.T) {
	RunHostDetector(t, detectingAdapter{
		exampleAdapter: exampleAdapter{id: domain.ClientClaude},
		detect: func(host clients.Host) clients.Detection {
			return clients.Detection{
				ExecutablePath: host.LookPath("claude"),
				Surfaces: []domain.ClientSurface{
					host.BinarySurface("claude_cli", "claude"),
					host.DirectorySurface("claude_config", host.HomeDir()+"/.claude"),
				},
				SelectionSurfaceIDs: []string{"claude_cli"},
			}
		},
	})
}

func TestHostDetectorViolationsRejectABrokenObservation(t *testing.T) {
	probeCount := 0
	cases := map[string]clients.Adapter{
		"no detection capability": exampleAdapter{id: domain.ClientClaude},
		"duplicate surface id": detection(func(clients.Host) clients.Detection {
			return clients.Detection{Surfaces: []domain.ClientSurface{{ID: "cli"}, {ID: "cli"}}}
		}),
		"empty surface id": detection(func(clients.Host) clients.Detection {
			return clients.Detection{Surfaces: []domain.ClientSurface{{}}}
		}),
		"evidence without detection": detection(func(clients.Host) clients.Detection {
			return clients.Detection{Surfaces: []domain.ClientSurface{{ID: "cli", Evidence: "executable_on_path"}}}
		}),
		"selection surface nobody reports": detection(func(clients.Host) clients.Detection {
			return clients.Detection{
				Surfaces:            []domain.ClientSurface{{ID: "cli"}},
				SelectionSurfaceIDs: []string{"desktop"},
			}
		}),
		"observation depends on call history": detection(func(clients.Host) clients.Detection {
			probeCount++
			return clients.Detection{Surfaces: []domain.ClientSurface{{ID: "cli", Detected: probeCount > 3}}}
		}),
	}
	for name, adapter := range cases {
		if violations := hostDetectorViolations(adapter); len(violations) == 0 {
			t.Errorf("hostDetectorViolations accepted the %q adapter", name)
		}
	}
}

// TestHostDetectorRejectsAmbientFilesystemAccess proves the counters are load
// bearing: an adapter that reaches past the host probes is reported, because a
// detection that reads the real machine cannot be reproduced in a test.
func TestHostDetectorRejectsAmbientFilesystemAccess(t *testing.T) {
	seen := 0
	adapter := detection(func(host clients.Host) clients.Detection {
		seen++
		surfaces := []domain.ClientSurface{host.BinarySurface("cli", "example")}
		if seen%2 == 0 {
			surfaces = append(surfaces, host.DirectorySurface("config", "/example"))
		}
		return clients.Detection{Surfaces: surfaces}
	})
	if violations := hostDetectorViolations(adapter); len(violations) == 0 {
		t.Fatal("hostDetectorViolations accepted an adapter whose probe count changes between runs")
	}
}

func detection(detect func(clients.Host) clients.Detection) clients.Adapter {
	return detectingAdapter{exampleAdapter: exampleAdapter{id: domain.ClientClaude}, detect: detect}
}
