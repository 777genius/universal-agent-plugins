//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func grokDuplicateContractProvider(t *testing.T) *grokProjectProvider {
	t.Helper()
	p, _ := grokTitleContractProvider(t)
	first := []byte(`{"model":"uap-grok-fixture","stream":true,"messages":[{"role":"user","content":"independent inert first request"}]}`)
	status, reply := grokProjectContractSend(t, grokProjectContractRequest(t, p, first))
	p.mu.Lock()
	defer p.mu.Unlock()
	if status != http.StatusOK || !bytes.Equal(reply, grokProjectProviderResponse()) || !bytes.Equal(p.request, first) || p.receipt.Requests != 1 || p.receipt.Responses != 1 {
		t.Fatal("first original exchange changed before refused diagnostic")
	}
	return p
}

func grokDuplicateContractEncode(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func grokDuplicateContractFiles(t *testing.T, p *grokProjectProvider) map[string][]byte {
	t.Helper()
	p.stop()
	p.title.transport.stop()
	p.mu.Lock()
	defer p.mu.Unlock()
	files := map[string][]byte{
		"runtime-attempt.json": grokDuplicateContractEncode(t, map[string]any{"client": "grok", "activation": "prepared", "runtime_attempt_complete": false, "test_failed": true,
			"vendor_authentication_qualified": false, "real_model_qualified": false, "qualification": map[string]bool{"project-lifecycle": false, "native-restart-removal": false, "mcp-initialize": false, "mcp-tools-list": false, "mcp-tools-call": false}}),
		"grok-provider.json": grokDuplicateContractEncode(t, p.receipt), "grok-provider.request.json": bytes.Clone(p.request), "grok-provider.response.sse": bytes.Clone(p.reply),
	}
	p.title.transport.mu.Lock()
	files["grok-title-provider.json"] = grokDuplicateContractEncode(t, struct {
		Role string `json:"role"`
		grokProjectProviderReceipt
	}{"session-title", p.title.transport.receipt})
	p.title.transport.mu.Unlock()
	if p.duplicate != nil {
		select {
		case <-p.duplicate.done:
		default:
			t.Fatal("duplicate handler not settled after original stop")
		}
		files[grokDuplicateReceiptName] = grokDuplicateContractEncode(t, p.duplicate.receipt)
		if p.duplicate.receipt.Status == "captured" {
			files[grokDuplicateRequestName] = bytes.Clone(p.duplicate.raw)
		}
	}
	return files
}

func TestGrokProjectDuplicateOpaqueCaptureNeverRespondsOrQualifies(t *testing.T) {
	p := grokDuplicateContractProvider(t)
	second := []byte(`{"unfamiliar_model":"ordinary refused data","messages":null}`)
	status, response := grokProjectContractSend(t, grokProjectContractRequest(t, p, second))
	if status != http.StatusBadRequest || !bytes.Equal(response, []byte("closed Grok provider request refused\n")) || p.ctx.Err() == nil {
		t.Fatal("observed duplicate gained a response or escaped original cancellation")
	}
	p.mu.Lock()
	claimed := p.duplicate
	p.mu.Unlock()
	if third, _ := grokProjectContractSend(t, grokProjectContractRequest(t, p, []byte(`{"third":"refused"}`))); third != http.StatusBadRequest {
		t.Fatal("third request was accepted")
	}
	files := grokDuplicateContractFiles(t, p)
	if err := grokProjectDuplicateMembers(files, "grok"); err != nil {
		t.Fatal("coherent failed diagnostic refused", err)
	}
	if p.duplicate != claimed || !bytes.Equal(files[grokDuplicateRequestName], second) || p.receipt.Failure != "duplicate" || p.receipt.Requests != 1 || p.receipt.Responses != 1 || !bytes.Equal(p.reply, grokProjectProviderResponse()) {
		t.Fatal("diagnostic replaced first exchange, slot or original failure")
	}
}

func TestGrokProjectDuplicateStructureAndPrivacyRefuseBeforeRetention(t *testing.T) {
	for _, kind := range []string{"empty", "oversized", "array", "null", "scalar", "invalid", "duplicate", "trailing", "nonfinite", "depth", "nodes", "key", "utf8", "main-key", "title-key", "escaped-main", "escaped-title", "nested-main", "nested-title"} {
		t.Run(kind, func(t *testing.T) {
			p := grokDuplicateContractProvider(t)
			raw, expected := []byte(`{"inert":true}`), "json-refused"
			switch kind {
			case "empty":
				raw, expected = nil, "body-refused"
			case "oversized":
				raw, expected = bytes.Repeat([]byte{' '}, (1<<20)+1), "body-refused"
			case "array", "null", "scalar":
				raw = []byte(map[string]string{"array": "[]", "null": "null", "scalar": "42"}[kind])
			case "invalid":
				raw = []byte(`{"inert":`)
			case "duplicate":
				raw = []byte(`{"inert":1,"inert":2}`)
			case "trailing":
				raw = []byte(`{} {}`)
			case "nonfinite":
				raw = []byte(`{"number":1e9999}`)
			case "depth":
				raw = []byte(`{"nested":` + strings.Repeat("[", 34) + "0" + strings.Repeat("]", 34) + "}")
			case "nodes":
				raw = []byte(`{"nodes":[` + strings.Repeat("0,", 16384) + "0]}")
			case "key":
				raw = grokDuplicateContractEncode(t, map[string]bool{strings.Repeat("é", 2049): true})
			case "utf8":
				raw = []byte{255}
			default:
				key := p.key
				if strings.Contains(kind, "title") {
					key = p.title.transport.key
				}
				expected = "privacy-refused"
				raw = []byte(`{"opaque":"` + key + `"}`)
				if strings.HasPrefix(kind, "escaped") || strings.HasPrefix(kind, "nested") {
					raw = []byte(`{"\u0075` + key[1:] + `":"ordinary"}`)
				}
				if strings.HasPrefix(kind, "nested") {
					raw = grokDuplicateContractEncode(t, map[string]string{"opaque": string(raw)})
				}
			}
			status, _ := grokProjectContractSend(t, grokProjectContractRequest(t, p, raw))
			files := grokDuplicateContractFiles(t, p)
			if status != http.StatusBadRequest || p.ctx.Err() == nil || p.receipt.Failure != "duplicate" || p.duplicate == nil || p.duplicate.receipt.Status != expected || files[grokDuplicateRequestName] != nil {
				t.Fatal("refused body was retained or changed original failure", expected)
			}
			if err := grokProjectDuplicateMembers(files, "grok"); err != nil {
				t.Fatal("truthful finite refusal failed artifact admission", err)
			}
		})
	}
}

func grokDuplicateContractPartial(t *testing.T, p *grokProjectProvider, duplicate bool) net.Conn {
	t.Helper()
	conn, err := (&net.Dialer{Timeout: time.Second}).DialContext(p.ctx, "tcp4", p.listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if _, err := fmt.Fprintf(conn, "POST /v1/chat/completions HTTP/1.1\r\nHost: %s\r\nAuthorization: Bearer %s\r\nContent-Type: application/json\r\nContent-Length: 1000\r\n\r\n{", p.listener.Addr(), p.key); err != nil {
		t.Fatal(err)
	}
	end := time.Now().Add(time.Second)
	for {
		p.mu.Lock()
		claimed := p.duplicate != nil
		if !duplicate {
			claimed = p.attempted
		}
		p.mu.Unlock()
		if claimed {
			return conn
		}
		if time.Now().After(end) {
			t.Fatal("partial duplicate never reached one-shot decision")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestGrokProjectDuplicateConcurrentRefusalCannotBlockOriginalStop(t *testing.T) {
	p := grokDuplicateContractProvider(t)
	grokDuplicateContractPartial(t, p, true)
	p.mu.Lock()
	claimed := p.duplicate
	p.mu.Unlock()
	status, _ := grokProjectContractSend(t, grokProjectContractRequest(t, p, []byte(`{"third":"ordinary"}`)))
	start := time.Now()
	files := grokDuplicateContractFiles(t, p)
	if status != http.StatusBadRequest || p.duplicate != claimed || p.duplicate.receipt.Status != "context-refused" || files[grokDuplicateRequestName] != nil || p.receipt.Failure != "duplicate" || !p.receipt.ForcedClose || !p.receipt.ShutdownComplete || time.Since(start) > 2500*time.Millisecond {
		t.Fatal("concurrent refusal stalled stop, renewed origin or retained partial bytes")
	}
	if err := grokProjectDuplicateMembers(files, "grok"); err != nil {
		t.Fatal("truthful concurrent/forced failed artifact refused", err)
	}
}

func TestGrokProjectDuplicatePartialReadKeepsOriginalFiveSecondLimit(t *testing.T) {
	p := grokDuplicateContractProvider(t)
	start := time.Now()
	conn := grokDuplicateContractPartial(t, p, true)
	// This is a client-side test observation bound, never a provider deadline.
	if err := conn.SetReadDeadline(start.Add(7 * time.Second)); err != nil {
		t.Fatal(err)
	}
	response, _ := io.ReadAll(io.LimitReader(conn, 2049))
	files := grokDuplicateContractFiles(t, p)
	if time.Since(start) > 6500*time.Millisecond || p.server.ReadTimeout != 5*time.Second || p.duplicate.receipt.Status != "body-refused" || files[grokDuplicateRequestName] != nil || p.ctx.Err() == nil || bytes.Contains(response, []byte("200 OK")) || bytes.Contains(response, []byte("data:")) {
		t.Fatal("partial diagnostic escaped original read/refusal boundary")
	}
	if err := grokProjectDuplicateMembers(files, "grok"); err != nil {
		t.Fatal(err)
	}
}

func TestGrokProjectDuplicateRequiresOriginalEligibility(t *testing.T) {
	for _, kind := range []string{"query", "method", "host", "credential", "duplicate-credential", "content-type", "duplicate-content-type", "late", "earlier", "missing-title", "incomplete-first"} {
		t.Run(kind, func(t *testing.T) {
			var p *grokProjectProvider
			if kind == "incomplete-first" {
				p, _ = grokTitleContractProvider(t)
				grokDuplicateContractPartial(t, p, false)
			} else if kind == "missing-title" {
				p = grokProjectContractProvider(t)
				body := []byte(`{"model":"uap-grok-fixture","stream":true,"messages":[{}]}`)
				if status, _ := grokProjectContractSend(t, grokProjectContractRequest(t, p, body)); status != http.StatusOK {
					t.Fatal("first inert main-only request failed")
				}
			} else {
				p = grokDuplicateContractProvider(t)
			}
			if kind == "late" {
				p.cancel()
			} else if kind == "earlier" {
				p.fail("serve")
			}
			r := grokProjectContractRequest(t, p, []byte(`{"ordinary":"refused"}`))
			grokProjectContractMutateHTTP(r, kind)
			status, _ := grokProjectContractSend(t, r)
			p.stop()
			if status != http.StatusBadRequest || p.ctx.Err() == nil || p.duplicate != nil || kind == "earlier" && p.receipt.Failure != "serve" {
				t.Fatal("ineligible refusal acquired a duplicate capture or replaced first failure")
			}
		})
	}
	main, title := grokTitleContractProvider(t)
	if status, _ := grokProjectContractSend(t, grokProjectContractRequest(t, title.transport, []byte(grokTitleContractGolden))); status != http.StatusOK {
		t.Fatal("first inert title exchange failed")
	}
	status, _ := grokProjectContractSend(t, grokProjectContractRequest(t, title.transport, []byte(`{"second":"title-refused"}`)))
	title.transport.stop()
	main.stop()
	if status != http.StatusBadRequest || title.transport.duplicate != nil || main.duplicate != nil || title.transport.receipt.Failure != "duplicate" {
		t.Fatal("title duplicate acquired main-only diagnostic")
	}
}

func TestGrokProjectDuplicateArtifactJoinsRefuseCoherentForgery(t *testing.T) {
	p := grokDuplicateContractProvider(t)
	grokProjectContractSend(t, grokProjectContractRequest(t, p, []byte(`{"independent":"ordinary refused data"}`)))
	original := grokDuplicateContractFiles(t, p)
	for _, kind := range []string{"client", "missing-receipt", "missing-raw", "missing-title", "missing-first", "unknown-file", "role", "index", "status", "extra", "response", "bytes", "hash", "refused-body", "main-count", "main-failure", "success", "qualification", "private", "nested-private", "invalid-json"} {
		t.Run(kind, func(t *testing.T) {
			files := map[string][]byte{}
			for name, body := range original {
				files[name] = bytes.Clone(body)
			}
			value, err := projectNativeRuntimeJSON(files[grokDuplicateReceiptName])
			if err != nil {
				t.Fatal(err)
			}
			row := value.(map[string]any)
			client := "grok"
			switch kind {
			case "client":
				client = "qoder"
			case "missing-receipt":
				delete(files, grokDuplicateReceiptName)
			case "missing-raw":
				delete(files, grokDuplicateRequestName)
			case "missing-title":
				delete(files, "grok-title-provider.json")
			case "missing-first":
				delete(files, "grok-provider.request.json")
			case "unknown-file":
				files["grok-provider.duplicate-request-2.json"] = []byte(`{}`)
			case "role":
				row["role"] = "session-title"
			case "index":
				row["attempt_index"] = true
			case "status":
				row["status"] = "accepted"
			case "extra":
				row["private_error"] = "not permitted"
			case "response":
				row["response_issued"] = true
			case "bytes":
				row["captured_bytes"] = 1
			case "hash":
				row["captured_sha256"] = strings.Repeat("0", 64)
			case "refused-body":
				row["status"], row["captured_bytes"], row["captured_sha256"] = "privacy-refused", 0, ""
			case "main-count", "main-failure":
				value, _ := projectNativeRuntimeJSON(files["grok-provider.json"])
				parent := value.(map[string]any)
				if kind == "main-count" {
					parent["requests"] = 0
				} else {
					parent["failure"] = "none"
				}
				files["grok-provider.json"] = grokDuplicateContractEncode(t, parent)
			case "success", "qualification":
				value, _ := projectNativeRuntimeJSON(files["runtime-attempt.json"])
				result := value.(map[string]any)
				if kind == "success" {
					result["runtime_attempt_complete"] = true
				} else {
					result["qualification"].(map[string]any)["mcp-tools-call"] = true
				}
				files["runtime-attempt.json"] = grokDuplicateContractEncode(t, result)
			case "private", "nested-private", "invalid-json":
				raw := []byte(`{"opaque":"uap-grok-private-inert"}`)
				if kind == "nested-private" {
					raw = grokDuplicateContractEncode(t, map[string]string{"opaque": `{"\u0075ap-grok-private-inert":"ordinary"}`})
				} else if kind == "invalid-json" {
					raw = []byte(`{"unclosed":`)
				}
				files[grokDuplicateRequestName] = raw
				row["captured_bytes"], row["captured_sha256"] = len(raw), grokProjectDuplicateSHA(raw)
			}
			if kind != "missing-receipt" {
				files[grokDuplicateReceiptName] = grokDuplicateContractEncode(t, row)
			}
			if err := grokProjectDuplicateMembers(files, client); err == nil {
				t.Fatal("incoherent or unsafe duplicate evidence admitted")
			}
		})
	}
}
