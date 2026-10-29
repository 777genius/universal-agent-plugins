//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"testing"
)

const grokDuplicateReceiptName = "grok-provider.duplicate.json"
const grokDuplicateRequestName = "grok-provider.duplicate-request.json"

func grokProjectDuplicateSHA(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }

type grokProjectDuplicateReceipt struct {
	SchemaVersion   int    `json:"schema_version"`
	Role            string `json:"role"`
	AttemptIndex    int    `json:"attempt_index"`
	Status          string `json:"status"`
	OriginalFailure string `json:"original_failure"`
	ResponseIssued  bool   `json:"response_issued"`
	CapturedBytes   int    `json:"captured_bytes"`
	CapturedSHA256  string `json:"captured_sha256"`
}

type grokProjectDuplicateCapture struct {
	receipt grokProjectDuplicateReceipt
	raw     []byte
	done    chan struct{}
}

// Caller holds p.mu. Reserve both first failure and the sole observation before
// releasing the lock; all later attempts keep their original immediate refusal.
func (p *grokProjectProvider) claimDuplicate() *grokProjectDuplicateCapture {
	if p.ctx.Err() != nil || p.title == nil || p.title.transport == nil || p.title.transport == p ||
		p.key == "" || p.title.transport.key == "" || p.title.transport.key == p.key ||
		p.receipt.Failure != "none" || p.receipt.Requests != 1 || p.receipt.Responses != 1 ||
		len(p.request) == 0 || !bytes.Equal(p.reply, grokProjectProviderResponse()) || p.duplicate != nil {
		return nil
	}
	p.receipt.Failure = "duplicate"
	p.duplicate = &grokProjectDuplicateCapture{done: make(chan struct{})}
	return p.duplicate
}

// No provider lock is held during body reading or parsing. The existing server
// ReadTimeout and original work/total contexts are the only time origins.
func (p *grokProjectProvider) observeDuplicate(w http.ResponseWriter, r *http.Request, capture *grokProjectDuplicateCapture) {
	status := "body-refused"
	var retained []byte
	defer func() {
		p.mu.Lock()
		if p.ctx.Err() != nil {
			status, retained = "context-refused", nil
		}
		capture.receipt = grokProjectDuplicateReceipt{SchemaVersion: 1, Role: "main-duplicate-diagnostic", AttemptIndex: 2,
			Status: status, OriginalFailure: "duplicate"}
		if status == "captured" {
			capture.raw = bytes.Clone(retained)
			capture.receipt.CapturedBytes = len(retained)
			capture.receipt.CapturedSHA256 = grokProjectDuplicateSHA(retained)
		}
		close(capture.done)
		p.mu.Unlock()
		http.Error(w, "closed Grok provider request refused", http.StatusBadRequest)
		p.failWithRequest("duplicate", r)
	}()
	if p.ctx.Err() != nil {
		status = "context-refused"
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
	if err != nil || len(raw) == 0 || len(raw) > 1<<20 {
		return
	}
	status = grokProjectDuplicateObject(raw, p.key, p.title.transport.key)
	if status == "captured" {
		retained = raw
	}
}

func grokProjectDuplicateObject(raw []byte, keys ...string) string {
	value, err := projectNativeRuntimeJSON(raw)
	object, ok := value.(map[string]any)
	if err != nil || !ok {
		return "json-refused"
	}
	decoded, err := json.Marshal(object)
	if err != nil || grokProjectPrivateCredential(raw) || grokProjectPrivateCredential(decoded) {
		return "privacy-refused"
	}
	for _, key := range keys {
		if key == "" || bytes.Contains(raw, []byte(key)) || bytes.Contains(decoded, []byte(key)) {
			return "privacy-refused"
		}
	}
	budget := grokProjectPrivacyBudget{text: 4 * len(raw), nodes: 16384}
	if !budget.scalarJSON(raw, 0) {
		return "json-refused"
	}
	return "captured"
}

// encoding/json replaces lone UTF16 escapes. This new diagnostic retains the
// original bytes, so refuse such scalars instead of normalizing its raw input.
// Whole strict JSON parsing always precedes this lexical string preflight.
func grokProjectDuplicateScalars(raw []byte) bool {
	quoted := false
	for index := 0; index < len(raw); index++ {
		if raw[index] == '"' {
			quoted = !quoted
			continue
		}
		if !quoted || raw[index] != '\\' {
			continue
		}
		index++
		if index >= len(raw) {
			return false
		}
		if raw[index] != 'u' {
			continue
		}
		if index+4 >= len(raw) {
			return false
		}
		unit, err := strconv.ParseUint(string(raw[index+1:index+5]), 16, 16)
		if err != nil {
			return false
		}
		index += 4
		if unit >= 0xdc00 && unit <= 0xdfff {
			return false
		}
		if unit < 0xd800 || unit > 0xdbff {
			continue
		}
		if index+6 >= len(raw) || raw[index+1] != '\\' || raw[index+2] != 'u' {
			return false
		}
		low, err := strconv.ParseUint(string(raw[index+3:index+7]), 16, 16)
		if err != nil || low < 0xdc00 || low > 0xdfff {
			return false
		}
		index += 6
	}
	return !quoted
}

// Match only complete nested JSON values/lines already traversed by the
// existing privacy scanner, with its unchanged shared text/node/depth bounds.
// Ordinary backslash-u text that is not complete JSON stays ordinary data.
func (b *grokProjectPrivacyBudget) scalarJSON(body []byte, depth int) bool {
	if depth > 64 || len(body) > b.text {
		return false
	}
	b.text -= len(body)
	value, err := projectNativeRuntimeJSON(body)
	if err == nil {
		return grokProjectDuplicateScalars(body) && b.scalarValue(value, depth)
	}
	if b.parseRefusal(body, err) {
		return false
	}
	if !bytes.ContainsRune(body, '\n') {
		return true
	}
	for len(body) != 0 {
		line, rest, _ := bytes.Cut(body, []byte{'\n'})
		if len(line) > b.text {
			return false
		}
		b.text -= len(line)
		value, err := projectNativeRuntimeJSON(line)
		if b.parseRefusal(line, err) || err == nil && (!grokProjectDuplicateScalars(line) || !b.scalarValue(value, depth)) {
			return false
		}
		body = rest
	}
	return true
}

func (b *grokProjectPrivacyBudget) scalarValue(value any, depth int) bool {
	b.nodes--
	if depth > 64 || b.nodes < 0 {
		return false
	}
	switch node := value.(type) {
	case string:
		return b.scalarJSON([]byte(node), depth+1)
	case map[string]any:
		for key, item := range node {
			if !b.scalarValue(key, depth+1) || !b.scalarValue(item, depth+1) {
				return false
			}
		}
	case []any:
		for _, item := range node {
			if !b.scalarValue(item, depth+1) {
				return false
			}
		}
	}
	return true
}

// Runs after the original finish unlocks. Snapshot only while holding p.mu;
// never wait, parse, scan privacy or persist evidence under that lock.
func (p *grokProjectProvider) finishDuplicate(t *testing.T, f *projectNativeCLIAttempt) {
	t.Helper()
	p.mu.Lock()
	if p.duplicate == nil {
		p.mu.Unlock()
		return
	}
	select {
	case <-p.duplicate.done:
	default:
		p.mu.Unlock()
		t.Error("Grok duplicate observation did not settle before export")
		return
	}
	receipt, body := p.duplicate.receipt, bytes.Clone(p.duplicate.raw)
	p.mu.Unlock()
	raw, err := json.Marshal(receipt)
	if err != nil || len(raw) > 2048 {
		t.Error("Grok duplicate finite receipt refused")
		return
	}
	summary := make(map[string]any, len(f.evidence)+1)
	for key, value := range f.evidence {
		summary[key] = value
	}
	summary["test_failed"] = t.Failed()
	result, err := json.Marshal(summary)
	if err != nil {
		t.Error("Grok duplicate failed boundary refused")
		return
	}
	candidate := make(map[string][]byte, len(f.files)+3)
	for name, body := range f.files {
		candidate[name] = body
	}
	candidate["runtime-attempt.json"], candidate[grokDuplicateReceiptName] = result, raw
	if receipt.Status == "captured" {
		candidate[grokDuplicateRequestName] = body
	}
	if err := grokProjectDuplicateMembers(candidate, f.spec.client); err != nil {
		t.Error(fmt.Errorf("Grok duplicate artifact refused: %w", err))
		return
	}
	f.keep(t, grokDuplicateReceiptName, raw)
	if receipt.Status == "captured" {
		f.keep(t, grokDuplicateRequestName, body)
	}
}
