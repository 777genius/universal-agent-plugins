package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// These fixtures run through the same rules, file discovery and import parser
// as TestLayerBoundaries. They do not need a compilable future client or a second
// registry: importing an implementation must be rejected before registration.
func TestLayerBoundariesRejectClientRegressions(t *testing.T) {
	t.Parallel()
	const root = modulePath + "/" + agentplugins + "/clients"
	const managementDir = "cli/internal/agentpluginscli/"
	const managementRoot = modulePath + "/" + managementDir
	type boundaryCase struct {
		name, file, imported, build string
		forbidden                   bool
	}
	fixtures := []boundaryCase{
		{"future provider", agentplugins + "/providers/new.go", root + "/futureagent", "", true},
		{"future planner nested package", agentplugins + "/planner/new.go", root + "/futureagent/internal/runtime", "", true},
		{"Windows detection", agentplugins + "/adapters/clientdetect/probe_windows.go", root + "/futureagent", "//go:build windows\n\n", true},
		{"custom build tag", agentplugins + "/providers/probe.go", root + "/futureagent", "//go:build architecture_fixture\n\n", true},
		{"peer adapter", agentplugins + "/clients/futureagent/adapter.go", root + "/anotheragent", "", true},
		{"peer external test", agentplugins + "/clients/futureagent/adapter_test.go", root + "/anotheragent", "", true},
		{"tagged peer test", agentplugins + "/clients/futureagent/adapter_windows_test.go", root + "/anotheragent", "//go:build windows && architecture_fixture\n\n", true},
		{"shared helper", agentplugins + "/clients/shared/helper.go", root + "/futureagent", "", true},
		{"root contract", agentplugins + "/clients/contract.go", root + "/futureagent", "", true},
		{"contract helper", agentplugins + "/clients/contracttest/helper.go", root + "/futureagent", "", true},
		{"internal example", agentplugins + "/clients/internal/exampleclient/adapter.go", root + "/futureagent", "", true},
		{"nested shared helper", agentplugins + "/clients/shared/internal/helper.go", root + "/futureagent", "", true},
		{"shared name prefix", agentplugins + "/planner/plan.go", root + "/sharedfuture", "", true},
		{"contract helper name prefix", agentplugins + "/planner/plan.go", root + "/contracttestfuture", "", true},
		{"generic assembled registry", agentplugins + "/providers/new.go", root + "/all", "", true},
		{"peer assembled registry", agentplugins + "/clients/futureagent/new.go", root + "/all", "", true},
		{"installer assembled registry", agentplugins + "/installer/new.go", root + "/all", "", true},
		{"tagged installer registry", agentplugins + "/installer/new_windows.go", root + "/all/internal", "//go:build windows\n\n", true},
		{"CLI assembled registry", "cli/internal/other/new.go", root + "/all", "", true},
		{"consumer test peer", agentplugins + "/clients/gemini/consumer_test.go", root + "/futureagent", "", true},
		{"consumer test registry", agentplugins + "/clients/gemini/consumer_test.go", root + "/all", "", true},
		{"consumer test nested implementation", agentplugins + "/clients/gemini/consumer_test.go", root + "/gemini/internal", "", true},
		{"assembly", agentplugins + "/clients/all/registry.go", root + "/futureagent", "", false},
		{"assembly consumer test", agentplugins + "/clients/all/registry_test.go", root + "/futureagent", "", false},
		{"generic root contract", agentplugins + "/providers/new.go", root, "", false},
		{"generic shared helpers", agentplugins + "/planner/new.go", root + "/shared", "", false},
		{"generic nested shared helpers", agentplugins + "/planner/new.go", root + "/shared/internal", "", false},
		{"peer root contract", agentplugins + "/clients/futureagent/new.go", root, "", false},
		{"peer shared helpers", agentplugins + "/clients/futureagent/new.go", root + "/shared", "", false},
		{"peer contract helpers", agentplugins + "/clients/futureagent/new_test.go", root + "/contracttest", "", false},
		{"consumer own package", agentplugins + "/clients/gemini/consumer_test.go", root + "/gemini", "", false},
		{"dispatcher fixture registry", agentplugins + "/providers/fixture_test.go", root + "/all", "", false},
		{"installer fixture registry", agentplugins + "/installer/fixture_test.go", root + "/all", "", false},
		{"ignored test data", agentplugins + "/providers/testdata/fixture.go", root + "/futureagent", "", false},
		{"management HTTP dependency", managementDir + "management/new.go", "net/http", "", true},
		{"management Windows I/O", managementDir + "management/new_windows.go", "os", "//go:build windows\n\n", true},
		{"management tagged subprocess", managementDir + "management/new.go", "os/exec", "//go:build architecture_fixture\n\n", true},
		{"management CLI dependency", managementDir + "management/new.go", strings.TrimSuffix(managementRoot, "/"), "", true},
		{"management Cobra dependency", managementDir + "management/new.go", "github.com/spf13/cobra", "", true},
		{"management transport back edge", managementDir + "management/new.go", managementRoot + "managementhttp", "", true},
		{"management store back edge", managementDir + "management/new.go", managementRoot + "managementstate", "", true},
		{"management concrete lock", managementDir + "management/new.go", modulePath + "/" + agentplugins + "/adapters/processlock", "", true},
		{"management pure standard library", managementDir + "management/new.go", "context", "", false},
		{"HTTP application port", managementDir + "managementhttp/new.go", managementRoot + "management", "", false},
		{"HTTP private guard", managementDir + "managementhttp/new.go", modulePath + "/cli/internal/loopbackapi", "", false},
		{"HTTP transport standard library", managementDir + "managementhttp/new.go", "net/http", "", false},
		{"HTTP store construction", managementDir + "managementhttp/new.go", managementRoot + "managementstate", "", true},
		{"HTTP look-alike contract", managementDir + "managementhttp/new.go", managementRoot + "managementfuture", "", true},
		{"HTTP direct files", managementDir + "managementhttp/new.go", "os", "", true},
		{"state application port", managementDir + "managementstate/new.go", managementRoot + "management", "", false},
		{"state atomic persistence", managementDir + "managementstate/new.go", modulePath + "/install/integrationctl/adapters/atomicfile", "", false},
		{"state concrete lock", managementDir + "managementstate/new.go", modulePath + "/" + agentplugins + "/adapters/processlock", "", false},
		{"state filesystem", managementDir + "managementstate/new.go", "os", "", false},
		{"state HTTP dependency", managementDir + "managementstate/new.go", "net/http", "", true},
		{"state process execution", managementDir + "managementstate/new.go", "os/exec", "", true},
		{"state transport back edge", managementDir + "managementstate/new.go", managementRoot + "managementhttp", "", true},
		{"HTTP test may compose store", managementDir + "managementhttp/new_test.go", managementRoot + "managementstate", "", false},
	}
	// Preserve coverage of every current adapter as well as unborn names above.
	identities := domain.SupportedClientIDs()
	tests := make([]boundaryCase, 0, len(fixtures)+len(identities))
	tests = append(tests, fixtures...)
	for _, id := range identities {
		tests = append(tests, boundaryCase{"current client " + string(id), agentplugins + "/providers/new.go", root + "/" + string(id), "", true})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fixture := t.TempDir()
			writeBoundarySource(t, fixture, tt.file, tt.build+"package fixture\nimport _ "+strconv.Quote(tt.imported)+"\n")
			var violations []string
			for _, rule := range layerBoundaries() {
				if !strings.HasPrefix(tt.file, rule.pkg+"/") {
					continue
				}
				// Even test-only fixtures leave a production file to scan when
				// the real dispatcher rule intentionally excludes test sources.
				writeBoundarySource(t, fixture, rule.pkg+"/baseline.go", "package fixture\n")
				violations = append(violations, boundaryViolations(t, fixture, rule)...)
			}
			if got := len(violations) > 0; got != tt.forbidden {
				t.Fatalf("forbidden = %v, want %v; violations: %v", got, tt.forbidden, violations)
			}
			for _, violation := range violations {
				if !strings.Contains(violation, tt.file+" imports "+tt.imported) {
					t.Fatalf("diagnostic does not identify forbidden source and import: %s", violation)
				}
			}
		})
	}
}

func writeBoundarySource(t *testing.T, root, relative, body string) {
	t.Helper()
	file := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
