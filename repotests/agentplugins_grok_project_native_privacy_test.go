//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

type grokProjectPrivacyBudget struct{ text, nodes int }

func grokProjectPrivateCredential(body []byte) bool {
	if len(body) > 1<<20 || !utf8.Valid(body) || bytes.Contains(body, []byte("uap-grok-private-")) {
		return true
	}
	budget := grokProjectPrivacyBudget{text: 4 * len(body), nodes: 16384}
	return budget.inspectJSON(body, 0)
}

func (b *grokProjectPrivacyBudget) inspectJSON(body []byte, depth int) bool {
	if depth > 64 || len(body) > b.text {
		return true
	}
	b.text -= len(body)
	value, err := projectNativeRuntimeJSON(body)
	if err == nil {
		return b.inspectValue(value, depth)
	}
	if b.parseRefusal(body, err) {
		return true
	}
	if !bytes.Contains(body, []byte{'\n'}) {
		return false
	}
	for len(body) != 0 {
		line, rest, _ := bytes.Cut(body, []byte{'\n'})
		if len(line) > b.text {
			return true
		}
		b.text -= len(line)
		value, err := projectNativeRuntimeJSON(line)
		if b.parseRefusal(line, err) || (err == nil && b.inspectValue(value, depth)) {
			return true
		}
		body = rest
	}
	return false
}

func (b *grokProjectPrivacyBudget) parseRefusal(body []byte, err error) bool {
	if err == nil {
		return false
	}
	switch err.Error() {
	case "JSON structure limit", "JSON key", "duplicate JSON key":
		return true
	case "nonfinite JSON number":
		return b.nonfiniteRefused(body)
	}
	return false
}

func (b *grokProjectPrivacyBudget) nonfiniteRefused(body []byte) bool {
	if len(body) > b.text {
		return true
	}
	b.text -= len(body)
	decoder := json.NewDecoder(bytes.NewReader(body))
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return grokProjectPrivacySyntaxResource(err)
	}
	err := decoder.Decode(&raw)
	return errors.Is(err, io.EOF) || grokProjectPrivacySyntaxResource(err)
}

func grokProjectPrivacySyntaxResource(err error) bool {
	if err == nil || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return false
	}
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		return strings.Contains(syntax.Error(), "exceeded max depth")
	}
	return true
}

func (b *grokProjectPrivacyBudget) inspectValue(value any, depth int) bool {
	b.nodes--
	if depth > 64 || b.nodes < 0 {
		return true
	}
	switch node := value.(type) {
	case string:
		if strings.Contains(node, "uap-grok-private-") {
			return true
		}
		return b.inspectJSON([]byte(node), depth+1)
	case map[string]any:
		for key, item := range node {
			if b.inspectValue(key, depth+1) || b.inspectValue(item, depth+1) {
				return true
			}
		}
	case []any:
		for _, item := range node {
			if b.inspectValue(item, depth+1) {
				return true
			}
		}
	}
	return false
}

func grokProjectPrivacyJSON(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestGrokProjectNativeProviderPrivacyInspectsNestedSemanticStrings(t *testing.T) {
	for _, seed := range []string{`{"key":"\u0075ap-grok-private-inert"}`, `{"\u0075ap-grok-private-inert":"ordinary"}`, `["\u0075ap-grok-private-inert"]`} {
		nested := []byte(seed)
		for range 8 {
			nested = grokProjectPrivacyJSON(t, map[string]string{"content": string(nested)})
			for _, raw := range [][]byte{nested, bytes.Join([][]byte{[]byte(`{"first":true}`), nested, {}}, []byte{'\n'})} {
				before := bytes.Clone(raw)
				if bytes.Contains(raw, []byte("uap-grok-private-")) || !grokProjectPrivateCredential(raw) || !bytes.Equal(raw, before) {
					t.Fatal("nested encoded credential escaped or changed input")
				}
			}
		}
	}
	benign := grokProjectPrivacyJSON(t, map[string]string{"content": "{\"key\":\"ordinary\"}\n{\"tail\":true}"})
	if grokProjectPrivateCredential(benign) || grokProjectPrivateCredential([]byte("ordinary non-JSON diagnostic")) {
		t.Fatal("bounded ordinary text was misclassified")
	}
}

func TestGrokProjectNativeProviderPrivacyBudgetsSpanWholeMember(t *testing.T) {
	for _, raw := range [][]byte{{0xff}, []byte(`{"same":1,"same":2}`), []byte(strings.Repeat("[", 33) + "0" + strings.Repeat("]", 33)),
		[]byte("[" + strings.Repeat("0,", 17000) + "0]"), grokProjectPrivacyJSON(t, map[string]bool{strings.Repeat("k", 4097): true}),
		[]byte(`{"n":` + strings.Repeat("9", 5000) + `,"key":"\u0075ap-grok-private-inert"}`)} {
		if !grokProjectPrivateCredential(raw) {
			t.Fatal("ambiguous syntax or privacy resource limit escaped refusal")
		}
	}
	nested := grokProjectPrivacyJSON(t, strings.Repeat("x", 4096))
	for range 2 {
		nested = grokProjectPrivacyJSON(t, map[string]string{"content": string(nested)})
	}
	if grokProjectPrivateCredential(nested) || !grokProjectPrivateCredential(bytes.Join([][]byte{nested, nested, {}}, []byte{'\n'})) {
		t.Fatal("JSONL renewed the shared text budget or rejected bounded nested text")
	}
	for _, keys := range [][2]string{{"first", "last"}, {"last", "first"}} {
		if !grokProjectPrivateCredential(grokProjectPrivacyJSON(t, map[string]string{keys[0]: string(nested), keys[1]: string(nested)})) {
			t.Fatal("siblings renewed the shared text budget")
		}
	}
	if grokProjectPrivateCredential(bytes.Repeat([]byte{' '}, 1<<20)) {
		t.Fatal("bounded ordinary whitespace was refused")
	}
	budget := grokProjectPrivacyBudget{text: 100, nodes: 16384}
	if !budget.inspectJSON([]byte(`"ordinary"`), 65) || budget.text != 100 {
		t.Fatal("depth exhaustion parsed additional text")
	}
	budget.nodes = 0
	if !budget.inspectValue(nil, 0) {
		t.Fatal("shared node limit was ignored")
	}
}

func TestGrokProjectNativeProviderPrivacyRequiresCompleteNumericJSON(t *testing.T) {
	text := "1e999" + strings.Repeat("a", 59)
	for _, raw := range [][]byte{
		[]byte(text), []byte("-1e999ordinary"), []byte("1e999 []"), []byte("1e999 1e999"), []byte("NaN"), []byte("Infinity"),
		grokProjectPrivacyJSON(t, map[string]string{"metadata": text}), grokProjectPrivacyJSON(t, map[string]string{text: "ordinary"}),
		[]byte(text + "\n{\"event\":\"ordinary\"}\n"),
	} {
		original := bytes.Clone(raw)
		if grokProjectPrivateCredential(raw) || !bytes.Equal(raw, original) {
			t.Fatal("ordinary non-JSON numeric prefix was refused or rewritten")
		}
	}
	for _, raw := range [][]byte{[]byte("1e999"), []byte(`{"n":1e999}`), []byte(`[1e999]`), grokProjectPrivacyJSON(t, "1e999"),
		[]byte(text + "\n{\"opaque\":\"\\u0075ap-grok-private-inert\"}\n"),
		[]byte("[1e999," + strings.Repeat("[", 10050) + strings.Repeat("]", 10050) + "]")} {
		if !grokProjectPrivateCredential(raw) {
			t.Fatal("complete nonfinite value, later JSONL credential or syntax resource failure escaped refusal")
		}
	}
	budget := grokProjectPrivacyBudget{text: len(text), nodes: 16384}
	if budget.nonfiniteRefused([]byte(text)) || budget.text != 0 {
		t.Fatal("complete-syntax attempt did not debit the shared budget")
	}
	budget.text = len(text) - 1
	if !budget.nonfiniteRefused([]byte(text)) {
		t.Fatal("numeric syntax attempt exceeded the shared budget")
	}
}

func TestGrokProjectPrivateNestedCaptureRefusesBeforePersistence(t *testing.T) {
	if os.Getenv("UAP_GROK_PRIVATE_CAPTURE_CHILD") == "1" {
		f := &projectNativeCLIAttempt{grok: &grokProjectProvider{key: "uap-grok-private-inert"}, files: map[string][]byte{}}
		defer func() {
			if len(f.files) != 0 {
				t.Error("PRIVATE_CAPTURE_WAS_STORED")
				return
			}
			if _, err := fmt.Fprint(os.Stdout, "EMPTY_CAPTURE_VERIFIED\n"); err != nil {
				t.Error("inert verification output failed")
			}
		}()
		body := grokProjectPrivacyJSON(t, map[string]string{"content": `{"key":"\u0075ap-grok-private-inert"}`})
		f.keep(t, "probe-events.jsonl", body)
		t.Fatal("private capture unexpectedly returned")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	result := captureProjectNative(context.Background(), executable, t.TempDir(), []string{"UAP_GROK_PRIVATE_CAPTURE_CHILD=1"},
		[]string{"-test.run=^TestGrokProjectPrivateNestedCaptureRefusesBeforePersistence$"}, 2048, 5*time.Second)
	if result.Exit != 1 || result.Deadline || result.StdoutLimit || result.StderrLimit ||
		!bytes.Contains(result.Stdout, []byte("EMPTY_CAPTURE_VERIFIED")) || !bytes.Contains(result.Stdout, []byte("unredacted Grok fixture credential refused")) ||
		bytes.Contains(result.Stdout, []byte("PRIVATE_CAPTURE_WAS_STORED")) || bytes.Contains(result.Stdout, []byte("uap-grok-private-")) || len(result.Stderr) != 0 {
		t.Fatalf("actual inert capture refusal was not proved: exit=%d, error=%s", result.Exit, result.ErrorKind)
	}
}
