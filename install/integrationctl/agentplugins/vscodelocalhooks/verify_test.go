package vscodelocalhooks_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	hooks "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/vscodelocalhooks"
)

const authored = `{"hooks":{"Stop":[{"type":"command","linux":"'/TEST/runtime' 'local-stop'","timeout":5}]}}`

func TestVerifyAuthoredSemanticContract(t *testing.T) {
	// Independent input: reordered fields, exponent seconds, escaped apostrophes.
	body := []byte(" \n" + `{"hooks":{"Stop":[{"timeout":5e0,"linux":"\u0027/TEST/runtime\u0027 \u0027local-stop\u0027","type":"command"}]}}`)
	before := bytes.Clone(body)
	if err := hooks.VerifyOwned(body, linux(), []hooks.Spec{stop()}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body, before) {
		t.Fatal("verification modified caller file")
	}
	// A legitimate supplementary rune may be represented as a surrogate pair.
	spec := stop()
	spec.Args = []string{"😀", "�"}
	body = []byte(`{"hooks":{"Stop":[{"type":"command","linux":"'/TEST/runtime' '\ud83d\ude00' '\ufffd'","timeout":5.0}]}}`)
	if err := hooks.VerifyOwned(body, linux(), []hooks.Spec{spec}); err != nil {
		t.Fatal(err)
	}
}

// Red: unknown/duplicate native fields or executable drift is adopted as owned.
func TestVerifyRefusesForeignAndDrift(t *testing.T) {
	cases := []string{
		strings.Replace(authored, `{"hooks":`, `{"version":1,"hooks":`, 1),
		strings.Replace(authored, `"Stop":`, `"agentStop":`, 1),
		strings.Replace(authored, `"timeout":5`, `"timeout":5000`, 1),
		strings.Replace(authored, `"timeout":5`, `"timeoutSec":5`, 1),
		strings.Replace(authored, `"type":"command"`, `"type":"command","cwd":"/foreign"`, 1),
		strings.Replace(authored, `"type":"command"`, `"foreign":"command"`, 1),
		strings.Replace(authored, `"type":"command"`, `"type":"command","env":{}`, 1),
		strings.Replace(authored, `"type":"command"`, `"type":"command","command":"other"`, 1),
		strings.Replace(authored, `"type":"command"`, `"type":"command","windows":"other"`, 1),
		strings.Replace(authored, `/TEST/runtime`, `/TEST/drifted`, 1),
		strings.Replace(authored, `"timeout":5`, `"timeout":"5"`, 1),
		strings.Replace(authored, `"timeout":5`, `"timeout":null`, 1),
		strings.Replace(authored, `"Stop":[`, `"SubagentStop":[],"Stop":[`, 1),
		strings.Replace(authored, `"type":"command"`, `"type":"command","type":"command"`, 1),
		`{"hooks":{"Stop":[{"linux":"'/TEST/runtime' 'local-stop'","lin\u0075x":"'/TEST/runtime' 'local-stop'","timeout":5}]}}`,
		`{"hooks":{},"hooks":{"Stop":[{"type":"command","linux":"'/TEST/runtime' 'local-stop'","timeout":5}]}}`,
		`{"hooks":{"Stop":[],"\u0053top":[{"type":"command","linux":"'/TEST/runtime' 'local-stop'","timeout":5}]}}`,
		`{"hooks":{"Stop":[{"matcher":"*","hooks":[{"type":"command","linux":"'/TEST/runtime' 'local-stop'","timeout":5}]}]}}`,
		`{"hooks":{"Stop":[{"type":"command","linux":"'/TEST/runtime' 'local-stop'","timeout":5},{"type":"command","linux":"other","timeout":5}]}}`,
		`{"hooks":null}`, `{"hooks":{"Stop":[]}}`, `[]`,
	}
	for i, body := range cases {
		if err := hooks.VerifyOwned([]byte(body), linux(), []hooks.Spec{stop()}); !errors.Is(err, hooks.ErrNotOwned) {
			t.Fatalf("case %d accepted or wrong error: %v", i, err)
		}
	}
}

func TestVerifyMalformedAndBounded(t *testing.T) {
	for _, body := range [][]byte{
		[]byte(`{"hooks":{/* comment */}}`), []byte(`{"hooks":{},}`),
		[]byte(authored + `{}`), []byte(`{"hooks":`),
		[]byte(strings.Repeat(" ", hooks.MaxDocumentBytes+1)),
		[]byte(strings.Repeat("[", 9) + "0" + strings.Repeat("]", 9)),
		[]byte(`{"\ud800":0}`), []byte(`{"\udc00":0}`),
		[]byte(`{"x":"\ud800a"}`), []byte(`{"x":"\ud800\u0041"}`),
		[]byte(`{"x":"\udc00\ud800"}`), []byte("{\"x\":\"\xff\"}"),
	} {
		if err := hooks.VerifyOwned(body, linux(), []hooks.Spec{stop()}); !errors.Is(err, hooks.ErrInvalid) {
			t.Fatalf("malformed/budget input accepted: %v", err)
		}
	}
	// Invalid expected ownership never becomes a wildcard for foreign bytes.
	if err := hooks.VerifyOwned([]byte(authored), linux(), nil); !errors.Is(err, hooks.ErrInvalid) {
		t.Fatal(err)
	}
	// Refusal must not echo supplied values that might contain private content.
	_, err := hooks.RenderArgv(linux(), "/TEST/runtime", []string{"TEST-private-sentinel\x00"})
	if err == nil || strings.Contains(err.Error(), "TEST-private-sentinel") {
		t.Fatal("invalid literal leaked in error", err)
	}
}
