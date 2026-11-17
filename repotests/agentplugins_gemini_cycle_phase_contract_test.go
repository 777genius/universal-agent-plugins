//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
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
func TestGeminiCycleBindingRefusalReportsBothObservedStatusDifferences(t *testing.T) {
	c := geminiCycleInert(t)
	geminiCycleStageActual(t, c, "a")
	r := c.recipes["a"]
	i := r.Installation
	b := i.Clients[r.Binding.ClientBindingID]
	b.Activation, b.Authentication = domain.ActivationActive, domain.AuthenticationNotRequired
	i.Clients = map[string]domain.ClientBinding{b.ClientBindingID: b}
	_, err := geminiCycleBindingIdentity(r, i)
	if err == nil || err.Error() != "Gemini exact binding/materialization/revision authority (activation_match=false authentication_match=false)" {
		t.Fatalf("both committed status mismatches must remain a bounded refusal: %v", err)
	}
}

func TestGeminiCycleBindingRefusalRetainsBoundedStateTrustAndUnassignedEvents(t *testing.T) {
	c := geminiCycleInert(t)
	raw := geminiCycleStageActual(t, c, "a")
	r := c.recipes["a"]
	c.stage = "add-a"
	events := []byte("{\"method\":\"initialize\",\"nonce\":\"inert-unassigned\"}\n")
	if err := os.WriteFile(c.f.base.Events, events, 0600); err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(c.f.evidence)
	c.bindingRefusalEvidence(r, raw)
	diagnostic := c.f.evidence["cycle_binding_refusal"].(map[string]any)
	trust := filepath.Join(c.f.base.Home, ".gemini", "trustedFolders.json")
	trustBody, err := os.ReadFile(trust)
	if err != nil || diagnostic["observation_only"] != true || diagnostic["stage"] != "add-a" || diagnostic["scope"] != "a" || diagnostic["trust_original_sha256"] != geminiCycleSHA(trustBody) || diagnostic["probe_accepted_sha256"] != geminiCycleSHA(nil) {
		t.Fatal("failed observation lost its original authority or stage")
	}
	for name, want := range map[string][]byte{"state": raw, "trust": trustBody, "probe_events": events} {
		part := diagnostic[name].(map[string]any)
		stream, ok := part["stream"].(geminiCycleBytes)
		if part["captured"] != true || !ok || stream.Bytes != len(want) || stream.SHA256 != geminiCycleSHA(want) || !bytes.Equal(stream.Body, want) {
			t.Fatal("failed observation changed or omitted exact bounded bytes", name)
		}
	}
	delete(c.f.evidence, "cycle_binding_refusal")
	after, _ := json.Marshal(c.f.evidence)
	if !bytes.Equal(before, after) || len(c.eventsAccepted) != 0 || c.observed {
		t.Fatal("diagnostic advanced an accepted event, baseline or completion")
	}
	for _, rejected := range [][]byte{bytes.Repeat([]byte{'x'}, (64<<10)+1), []byte(c.f.secret), []byte(`{"secret":"uap-gemini-private-key"}`)} {
		if !reflect.DeepEqual(c.bindingRefusalBytes(rejected, nil), map[string]any{"captured": false}) {
			t.Fatal("private or oversized diagnostic body retained")
		}
	}
	if !reflect.DeepEqual(c.bindingRefusalBytes(raw, errors.New("private path")), map[string]any{"captured": false}) {
		t.Fatal("failed read exposed diagnostics or an error string")
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
