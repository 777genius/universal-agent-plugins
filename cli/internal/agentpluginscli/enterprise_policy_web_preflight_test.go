package agentpluginscli

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/enterprisepolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Source admission must precede HTTPS publisher catalog reads. This also
// covers the --all discovery route when collection selection is enabled.
func TestEnterprisePolicyDeniesHTTPSPublisherBeforeCatalogRead(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		http.NotFound(w, nil)
	}))
	defer server.Close()

	f := newCLIFixture(t, nil)
	policyPath := filepath.Join(f.root, "deny-web.yaml")
	dependencyWrite(t, policyPath, "schema_version: 2\nsources:\n  allowed_kinds: []\n")
	setEnterpriseCommandFiles(&f, enterprisepolicy.Path{Layer: "machine", Path: policyPath})
	f.app.SourceAcquirer = sourceacquisition.Acquirer{TempRoot: t.TempDir(), HTTPClient: server.Client()}

	for _, selection := range [][]string{{"--skill", "review"}, {"--all"}} {
		t.Run(selection[0], func(t *testing.T) {
			args := append([]string{"skills", "add", server.URL}, selection...)
			args = append(args, "--target", "codex", "--no-ui", "--yes", "--format", "json")
			_, _, err := f.execute(false, args...)
			var denied *domain.InstallPolicyError
			if !errors.As(err, &denied) {
				t.Fatalf("expected typed source policy refusal before publisher read, got %v", err)
			}
			if got := requests.Load(); got != 0 {
				t.Fatalf("enterprise-denied publisher received %d HTTP requests", got)
			}
		})
	}
}
