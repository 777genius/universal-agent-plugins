//go:build linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestCLIObservationClosedPhasesAndSettledExchange(t *testing.T) {
	for _, phase := range []string{"details-help", "list-json", "details-full-id", "details-name", "inspect-project"} {
		r := inertRequest()
		r.Mode, r.Phase = cliMode, phase
		if e := validateResearchMode(r); e != nil {
			t.Fatal(e)
		}
		var value map[string]any
		if e := json.Unmarshal(inertCatalogResult(t, inertRequest(), []string{phase}), &value); e != nil {
			t.Fatal(e)
		}
		value["mode"], value["phase"], value["semantic_complete"], value["reason"] = cliMode, phase, true, "complete"
		row := value["commands"].([]any)[0].(map[string]any)
		row["argv"] = append([]string{"/tmp/uap-grok-acp-sealed-inert/grok"}, cliArguments(r)...)
		for _, fault := range []string{"none", "cleanup", "phase", "rpc", "extra-command", "missing-command", "mutator", "overflow", "null-size", "null-complete", "provider", "lock"} {
			body, e := json.Marshal(value)
			if e != nil {
				t.Fatal(e)
			}
			changes := map[string][2]string{"phase": {`"phase":"` + phase + `"`, `"phase":"enable"`}, "rpc": {`"plugins":null`, `"plugins":{}`},
				"mutator": {`"argv":[`, `"argv":["enable",`}, "overflow": {`"merged_bytes":0`, `"merged_bytes":16385`},
				"null-size": {`"merged_bytes":0`, `"merged_bytes":null`}, "provider": {`"rpc":`, `"provider":{},"rpc":`},
				"null-complete": {`"semantic_complete":true`, `"semantic_complete":null`},
				"lock":          {`"rpc":`, `"lock_held":false,"rpc":`}}
			if pair, ok := changes[fault]; ok {
				changed := bytes.Replace(body, []byte(pair[0]), []byte(pair[1]), 1)
				if bytes.Equal(changed, body) {
					t.Fatal("mutation missed", fault)
				}
				body = changed
			}
			if fault == "null-complete" {
				body = bytes.Replace(body, []byte(`"reason":"complete"`), []byte(`"reason":"protocol-refused"`), 1)
			}
			if fault == "extra-command" {
				body = bytes.Replace(body, []byte(`"commands":[`), []byte(`"commands":[{},`), 1)
			}
			if fault == "missing-command" {
				copyValue := make(map[string]any)
				for key, item := range value {
					copyValue[key] = item
				}
				copyValue["commands"] = []any{}
				body, e = json.Marshal(copyValue)
				if e != nil {
					t.Fatal(e)
				}
			}
			body = append(body, '\n')
			raw, e := supervisedCapture(context.Background(), ports.Command{}, r, 1, func(_ context.Context, _ ports.Command, exchange func(io.Writer, io.Reader) error) error {
				if err := exchange(io.Discard, bytes.NewReader(body)); err != nil {
					return err
				}
				if fault == "cleanup" {
					return errors.New("family not settled")
				}
				return nil
			})
			if fault == "none" && (e != nil || !bytes.Equal(raw, body)) || fault != "none" && (e == nil || raw != nil) {
				t.Fatal(phase, fault, e)
			}
		}
		r.Scope = "b"
		if validateResearchMode(r) == nil {
			t.Fatal("sibling CLI scope admitted")
		}
		r.Scope, r.Phase = "a", "enable"
		if validateResearchMode(r) == nil {
			t.Fatal("unknown command admitted")
		}
	}
}
