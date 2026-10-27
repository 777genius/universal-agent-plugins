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
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestClineCycleRepeatedRemovalRetainsActualSupervisorNaturalExit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Only a fixed inert shell primitive runs; no installer, vendor or hosted
	// marker is present. The actual retained descendant supervisor owns it.
	script := `printf '%s\n' 'agentplugins: plugin is not installed for target "cline" in project scope; no target was changed' >&2; exit 1`
	result, err := (process.OS{}).RunWithDescendantExitGrace(ctx, ports.Command{Argv: []string{"/bin/sh", "-c", script}, Env: []string{"PATH=/usr/bin:/bin"}, Dir: t.TempDir(), StdoutLimitBytes: 1 << 20}, time.Second)
	if errors.Is(err, process.ErrOrdinaryContainmentUnsupported) {
		t.Skip("actual local proc containment unavailable; genuine runtime admission remains mandatory")
	}
	if ctx.Err() != nil || !clineCycleAbsentRemoval(result, err) {
		t.Fatal("actual strict runner did not normalize exact natural refusal", err)
	}
}
func TestClineCycleRepeatedRemovalRejectsEveryNonexactOutcome(t *testing.T) {
	result := ports.CommandResult{ExitCode: 1, Stderr: []byte("agentplugins: plugin is not installed for target \"cline\" in project scope; no target was changed\n")}
	if !clineCycleAbsentRemoval(result, nil) {
		t.Fatal("exact controlled refusal fixture rejected")
	}
	for _, wrong := range []ports.CommandResult{{ExitCode: 0, Stderr: result.Stderr}, {ExitCode: 2, Stderr: result.Stderr}, {ExitCode: 1, Stderr: append(result.Stderr, '\n')}, {ExitCode: 1, Stderr: result.Stderr, Stdout: []byte("unexpected")}} {
		if clineCycleAbsentRemoval(wrong, nil) {
			t.Fatal("nonexact repeated-removal outcome admitted")
		}
	}
	if clineCycleAbsentRemoval(result, errors.New("inert cleanup failure")) {
		t.Fatal("cleanup/capture error became ordinary absence")
	}
}

func TestClineCycleCurrentStateAcceptsActualWriterAndRefusesSchemaConfusion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := (statev2.Store{Path: path}).Save(domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := clineCycleState(raw); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{
		bytes.Replace(raw, []byte(`"schema_version": 5`), []byte(`"schema_version": 2`), 1),
		[]byte(`{"schema_version":5,"installations":[],"unknown":true}`),
		[]byte(`{"schema_version":5,"schema_version":5,"installations":[]}`),
		[]byte(`{"schema_version":"5","installations":[]}`), []byte(`{"schema_version":null,"installations":[]}`),
		[]byte(`{"schema_version":6,"installations":[]}`), []byte(`{"installations":[]}`),
	} {
		if _, err := clineCycleState(bad); err == nil {
			t.Fatal("unsupported/ambiguous Cline raw state accepted")
		}
	}
}
func clineCycleEmptyTranscript(t *testing.T) (*clineProjectAttempt, []map[string]any) {
	t.Helper()
	frames, _ := clineMeasuredTranscript(t)
	frames = append(frames[:2:2], frames[7:]...)
	f := clineMeasuredAttempt()
	f.provider.empty = true
	f.provider.issued = false
	return f, frames
}
func clineCycleFrames(t *testing.T, frames []map[string]any) []byte {
	t.Helper()
	var raw bytes.Buffer
	for _, frame := range frames {
		body, err := json.Marshal(frame)
		if err != nil {
			t.Fatal(err)
		}
		raw.Write(body)
		raw.WriteByte('\n')
	}
	return raw.Bytes()
}
func TestClineCycleEmptyModeRemovesAuthorityWithoutClaimingMeasuredGrammar(t *testing.T) {
	f, frames := clineCycleEmptyTranscript(t)
	var output, transcript bytes.Buffer
	if err := f.acp(&output, bytes.NewReader(clineCycleFrames(t, frames)), &transcript); err != nil {
		t.Fatal(err)
	}
	if f.provider.approved || f.nativeFacts != nil || bytes.Contains(output.Bytes(), []byte("allow_once")) || bytes.Contains(output.Bytes(), []byte("request_permission")) {
		t.Fatal("empty candidate granted authority")
	}
	for _, kind := range []string{"tool", "permission", "missing-info", "missing-sentinel", "repeated-info", "foreign-session"} {
		f, frames := clineCycleEmptyTranscript(t)
		original, _ := clineMeasuredTranscript(t)
		switch kind {
		case "tool":
			frames = append(frames[:2:2], append([]map[string]any{original[2]}, frames[2:]...)...)
		case "permission":
			frames = append(frames[:2:2], append([]map[string]any{original[3]}, frames[2:]...)...)
		case "missing-info":
			frames = append(frames[:3:3], frames[4:]...)
		case "missing-sentinel":
			frames[2]["params"].(map[string]any)["update"].(map[string]any)["content"].(map[string]any)["text"] = "foreign"
		case "repeated-info":
			frames = append(frames[:4:4], append([]map[string]any{frames[3]}, frames[4:]...)...)
		case "foreign-session":
			frames[2]["params"].(map[string]any)["sessionId"] = "foreign"
		}
		var out, captured bytes.Buffer
		if err := f.acp(&out, bytes.NewReader(clineCycleFrames(t, frames)), &captured); err == nil {
			t.Fatal("empty mode accepted tool/permission/unknown closure", kind)
		}
		if bytes.Contains(out.Bytes(), []byte(`"optionId":"allow_once"`)) {
			t.Fatal("empty mode approved injected permission")
		}
	}
}
func TestClineCycleEmptyProviderRejectsOwnedOrForeignMCPAndHistory(t *testing.T) {
	p := clineContractProvider()
	p.empty = true
	p.issued = false
	good := clineContractRequest(p)
	good["tools"].([]any)[0].(map[string]any)["function"].(map[string]any)["name"] = "read_file"
	raw, _ := json.Marshal(good)
	if _, err := p.validateRequest(raw, 0); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{clineAttemptTool, "foreign__tool"} {
		m := clineContractClone(t, good)
		m["tools"].([]any)[0].(map[string]any)["function"].(map[string]any)["name"] = name
		raw, _ := json.Marshal(m)
		if _, err := p.validateRequest(raw, 0); err == nil {
			t.Fatal("empty provider admitted MCP declaration")
		}
	}
}
func TestClineCycleProviderCannotArmAcrossUnclosedOrUnregisteredStage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := &clineCycleProvider{cancel: cancel, key: "inert", completed: []string{}}
	r := &clineCycleRecipe{Nonce: "inert-nonce", Scope: "a"}
	if _, err := p.arm("installed-a", r); err == nil {
		t.Fatal("provider skipped empty stages")
	}
	q, err := p.arm("empty-a", r)
	if err != nil || !q.empty {
		t.Fatal(err)
	}
	if _, err := p.arm("empty-b", r); err == nil {
		t.Fatal("prior phase overwritten")
	}
	p.current = nil
	p.completed = append(p.completed, "empty-a")
	r.Scope = "b"
	p.busy = 1
	if _, err := p.arm("empty-b", r); err == nil {
		t.Fatal("previous handler retained next-stage authority")
	}
	p.busy = 0
	p.failure = true
	if _, err := p.arm("empty-b", r); err == nil {
		t.Fatal("failed provider rearmed")
	}
	p.failure = false
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://127.0.0.1/", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	p.serve(response, request)
	if response.Code != 400 || !p.failure || ctx.Err() == nil {
		t.Fatal("idle request did not fail closed")
	}
}
func TestClineCyclePrivacyInspectsJSONLBeforeBase64(t *testing.T) {
	encoded := []byte(`{"opaque":"\u0075ap-cline-private-0123456789abcdef"}`)
	for _, raw := range [][]byte{encoded, append([]byte("{\"first\":true}\n"), encoded...), append(append([]byte{}, encoded...), []byte("\n{\"last\":true}\n")...)} {
		if bytes.Contains(raw, []byte("uap-cline-private-")) || !clineCyclePrivateCredential(raw) {
			t.Fatal("encoded private JSONL escaped bounded canary")
		}
	}
	if clineCyclePrivateCredential([]byte("{\"first\":true}\n{\"last\":true}\n")) {
		t.Fatal("ordinary JSONL falsely refused")
	}
	nested := encoded
	for range 8 {
		body, err := json.Marshal(map[string]string{"content": string(nested)})
		if err != nil {
			t.Fatal(err)
		}
		nested = body
		if bytes.Contains(nested, []byte("uap-cline-private-")) || !clineCyclePrivateCredential(nested) {
			t.Fatal("nested semantic JSON string escaped private canary")
		}
		if !clineCyclePrivateCredential(append([]byte("{\"first\":true}\n"), nested...)) {
			t.Fatal("nested semantic JSONL escaped private canary")
		}
	}
	benign, err := json.Marshal(map[string]string{"content": "{\"key\":\"ordinary\"}\n{\"tail\":true}"})
	if err != nil || clineCyclePrivateCredential(benign) {
		t.Fatal("ordinary nested JSON/JSONL falsely refused")
	}
	_, measured := clineMeasuredTranscript(t)
	if clineCyclePrivateCredential(measured) {
		t.Fatal("actual preserved native nested JSONL falsely refused")
	}
	for _, raw := range [][]byte{{255}, []byte("{\"same\":1,\"same\":2}"), bytes.Repeat([]byte{' '}, (1<<20)+1)} {
		if !clineCyclePrivateCredential(raw) {
			t.Fatal("ambiguous/invalid/bounded privacy refusal lost")
		}
	}
	f := &clineProjectAttempt{key: "uap-cline-private-inert"}
	c := &clineProjectCycle{f: f}
	row := c.stream(t, []byte("ordinary "+f.key))
	if clineCyclePrivateCredential(row.Body) || row.Bytes != len(row.Body) || row.SHA256 != clineAttemptSHA(row.Body) {
		t.Fatal("pre-encoding redaction receipt differs")
	}
}
func TestClineCycleSourceRecipeDoesNotAdoptReceiptRevisionOrScope(t *testing.T) {
	r := &clineCycleRecipe{Source: "/inert/A/source", Skill: "owned-a", Scope: "a", Digest: "sha256:source", Authored: map[string][]byte{"plugin.json": []byte("manifest")}}
	i := domain.Installation{InstallationID: "owned", DeclaredName: clineCycleName, Source: domain.SourceBinding{SourceBindingID: "src_" + clineAttemptSHA([]byte(r.Source + "\x00\x00"))[:24], RequestedSource: r.Source, CanonicalSource: r.Source, TreeDigest: r.Digest}, Package: domain.PackageBinding{LoaderKind: domain.LoaderKindAgentPlugins, FormatID: domain.FormatIDAgentPluginsV1, SchemaURI: domain.PluginSchemaV1, DeclaredName: clineCycleName, Version: "1.0.0", ManifestDigest: "sha256:" + clineAttemptSHA(r.Authored["plugin.json"]), Inventory: domain.ComponentInventory{MCPPresent: true, MCPEnabled: true, MCPServers: []string{"uapclinea"}, Skills: []string{r.Skill}}}}
	if err := clineCycleSource(i, r); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"source", "revision", "name", "skill", "manifest"} {
		body, _ := json.Marshal(i)
		var wrong domain.Installation
		if err := json.Unmarshal(body, &wrong); err != nil {
			t.Fatal(err)
		}
		switch kind {
		case "source":
			wrong.Source.CanonicalSource = "/inert/B/source"
		case "revision":
			wrong.Source.TreeDigest = "sha256:foreign"
		case "name":
			wrong.Package.DeclaredName = "foreign"
		case "skill":
			wrong.Package.Inventory.Skills = []string{"foreign-b"}
		case "manifest":
			wrong.Package.ManifestDigest = "sha256:foreign"
		}
		if clineCycleSource(wrong, r) == nil || !reflect.DeepEqual(i.Source.TreeDigest, r.Digest) {
			t.Fatal("receipt altered independent source expectation", kind)
		}
	}
}
