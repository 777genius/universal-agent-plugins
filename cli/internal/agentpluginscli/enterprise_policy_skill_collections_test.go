package agentpluginscli

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/enterprisepolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestEnterprisePolicyWebsiteAllRefusesBeforeDiscoveryAndKeepsListing(t *testing.T) {
	for _, tc := range []struct {
		name, policy string
		invalid      bool
	}{
		{name: "source kind", policy: "sources:\n  allowed_kinds: []\n"},
		{name: "publisher host", policy: "sources:\n  deny_hosts: [127.0.0.1]\n"},
		{name: "scope", policy: "install:\n  allowed_scopes: [project]\n"},
		{name: "target", policy: "install:\n  allowed_targets: [claude]\n"},
		{name: "invalid policy", policy: "unknown_rule: true\n", invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			publisher := &allWebsitePublisher{}
			var requests atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				publisher.serve(w, r)
			}))
			defer server.Close()
			journey := newSavedJourney(t, t.TempDir())
			fixture := journey.cli
			policy := filepath.Join(fixture.root, "collection-policy.yaml")
			dependencyWrite(t, policy, "schema_version: 2\n"+tc.policy)
			setEnterpriseCommandFiles(&fixture, enterprisepolicy.Path{Layer: "machine", Path: policy})
			fixture.app.SourceAcquirer = sourceacquisition.Acquirer{TempRoot: t.TempDir(), HTTPClient: server.Client()}
			before := savedJourneyTree(t, fixture.root)
			_, _, err := fixture.execute(false, "skills", "add", server.URL, "--all", "--target", "codex",
				"--no-ui", "--yes", "--format", "json")
			var denied *domain.InstallPolicyError
			var invalid *enterprisepolicy.ConfigError
			if tc.invalid && !errors.As(err, &invalid) || !tc.invalid && !errors.As(err, &denied) {
				t.Fatalf("expected typed policy refusal before publisher discovery, got %v", err)
			}
			if got := requests.Load(); got != 0 {
				t.Fatalf("denied --all contacted publisher %d times", got)
			}
			if after := savedJourneyTree(t, fixture.root); journey.runner.calls != 0 || !reflect.DeepEqual(before, after) {
				t.Fatal("denied collection changed durable state: " + savedJourneyDifferences(before, after))
			}
			out, _, err := fixture.execute(false, "skills", "list", server.URL, "--format", "json")
			if err != nil {
				t.Fatalf("installation policy blocked read-only discovery: %v", err)
			}
			var report struct {
				Data skillsListReport `json:"data"`
			}
			if err := json.Unmarshal([]byte(out), &report); err != nil {
				t.Fatal(err)
			}
			if requests.Load() != 1 || !report.Data.Complete || len(report.Data.Candidates) != 2 {
				t.Fatalf("listing did not observe the real bounded catalog: requests=%d report=%+v", requests.Load(), report.Data)
			}
			if after := savedJourneyTree(t, fixture.root); journey.runner.calls != 0 || !reflect.DeepEqual(before, after) {
				t.Fatal("read-only collection listing changed durable state: " + savedJourneyDifferences(before, after))
			}
		})
	}
}
