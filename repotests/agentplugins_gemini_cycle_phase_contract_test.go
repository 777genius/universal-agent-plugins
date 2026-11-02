//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestGeminiCycleHeldProviderNoCallStageIssuesOnlySentinelAndCannotRearmAcrossAuthority(t *testing.T) {
	c := geminiCycleInert(t)
	c.startProvider(t)
	p := c.owner
	t.Cleanup(func() { _ = p.server.Close() })
	q, err := p.arm("baseline-a", "a")
	if err != nil || q.operation != "none" || len(q.tools) != 0 {
		t.Fatal("exact empty initial stage", err)
	}
	if _, err := p.arm("installed-user", "user"); err == nil {
		t.Fatal("unclosed stage could rearm")
	}
	_, _, files := geminiCycleMeasured(t)
	value := geminiCycleClone(t, files["provider-request-1.json"])
	value["tools"] = []any{map[string]any{"functionDeclarations": []any{}}}
	raw := geminiCycleEncoded(t, value)
	url := "http://" + p.listener.Addr().String() + "/v1beta/models/" + geminiProjectModel + ":streamGenerateContent?alt=sse"
	request, err := http.NewRequestWithContext(c.f.ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("x-goog-api-key", c.f.secret)
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	p.serve(response, request)
	if response.Code != 200 || q.issued || q.returned || len(q.requests) != 1 || len(q.responses) != 1 || !strings.Contains(response.Body.String(), q.sentinel) || strings.Contains(response.Body.String(), "functionCall") {
		t.Fatal("empty stage issued authority or omitted sentinel")
	}
	p.complete(t, "baseline-a")
	if _, err := p.arm("installed-b", "b"); err == nil {
		t.Fatal("skipped registered stage accepted")
	}
	if _, err := p.arm("installed-user", "user"); err == nil {
		t.Fatal("uninstalled selected caller accepted")
	}
	geminiCycleStageActual(t, c, "user")
	q, err = p.arm("installed-user", "user")
	if err != nil || q.operation != "write" || q.id == q.wireID || q.id != q.tool+"__"+q.wireID {
		t.Fatal("exact owned next stage native/wire identity", err)
	}
	if _, err := p.arm("installed-user", "user"); err == nil {
		t.Fatal("repeated active stage accepted")
	}
}
func TestGeminiCycleExactCurrentBindingIdentityRejectsRootsIDsIntentAndExtraAuthority(t *testing.T) {
	c := geminiCycleInert(t)
	geminiCycleStageActual(t, c, "user")
	r := c.recipes["user"]
	for _, kind := range []string{"map-key", "client", "scope", "profile", "active", "physical", "binding-id", "revision", "intent", "native-attempt", "affected", "extra-client"} {
		t.Run(kind, func(t *testing.T) {
			raw, _ := json.Marshal(r.Installation)
			var i domain.Installation
			if json.Unmarshal(raw, &i) != nil {
				t.Fatal("actual installation clone")
			}
			b := i.Clients[r.Binding.ClientBindingID]
			switch kind {
			case "map-key":
				i.Clients = map[string]domain.ClientBinding{"foreign": b}
			case "client":
				b.ClientID = "other"
			case "scope":
				b.Scope = "project"
			case "profile":
				b.NativeProfileRoot = c.recipes["a"].Profile
			case "active":
				b.TargetLocator = c.recipes["a"].Source
			case "physical":
				b.PhysicalArtifact = "foreign"
			case "binding-id":
				b.ClientBindingID = "client_foreign"
			case "revision":
				b.PackageRevision.TreeDigest = "sha256:" + strings.Repeat("0", 64)
			case "intent":
				b.InstallIntent = domain.InstallIntentPrepare
			case "native-attempt":
				b.NativeActivationAttempt = "foreign"
			case "affected":
				b.AffectedSurfaces = []string{"gemini", "cline"}
			case "extra-client":
				i.Clients["foreign"] = b
			}
			if kind != "map-key" {
				i.Clients[r.Binding.ClientBindingID] = b
			}
			if _, err := geminiCycleBindingIdentity(r, i); err == nil {
				t.Fatal("foreign binding identity/authority accepted")
			}
		})
	}
}
func TestGeminiCycleRepeatedRemovalPreservesActualSupervisorNaturalExit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result, err := (process.OS{}).RunWithDescendantExitGrace(ctx, ports.Command{Argv: []string{"/bin/sh", "-c", `printf '%s\n' 'agentplugins: plugin is not installed for target "gemini" in project scope; no target was changed' >&2; exit 1`}, Env: []string{"PATH=/usr/bin:/bin"}, StdoutLimitBytes: 4096}, time.Second)
	if errors.Is(err, process.ErrOrdinaryContainmentUnsupported) {
		t.Skip("local inert proc prerequisite unavailable; genuine hosted containment remains mandatory")
	}
	if ctx.Err() != nil || !geminiCycleAbsentRemoval(result, err) {
		t.Fatal("natural supervised exit1 no longer matches exact refusal", err)
	}

}
func TestGeminiCycleRepeatedRemovalPureExactAndHostileOutcomesAreUnconditional(t *testing.T) {
	result := ports.CommandResult{ExitCode: 1, Stderr: []byte("agentplugins: plugin is not installed for target \"gemini\" in project scope; no target was changed\n")}
	if !geminiCycleAbsentRemoval(result, nil) {
		t.Fatal("exact pure removal refusal fixture")
	}
	for _, kind := range []string{"exit", "stdout", "stderr", "error"} {
		changed := result
		var failure error
		switch kind {
		case "exit":
			changed.ExitCode = 0
		case "stdout":
			changed.Stdout = []byte("extra")
		case "stderr":
			changed.Stderr = append([]byte("prefix"), changed.Stderr...)
		case "error":
			failure = context.DeadlineExceeded
		}
		if geminiCycleAbsentRemoval(changed, failure) {
			t.Fatal("nonexact repeated removal admitted", kind)
		}
	}
}
