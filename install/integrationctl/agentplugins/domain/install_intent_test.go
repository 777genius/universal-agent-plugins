package domain

import (
	"encoding/json"
	"testing"
)

func TestInstallIntentRejectsMalformedJSON(t *testing.T) {
	for _, raw := range []string{`null`, `true`, `42`, `{}`, `"automatic"`, `" prepare"`, `"PREPARE"`} {
		var binding ClientBinding
		if err := json.Unmarshal([]byte(`{"install_intent":`+raw+`}`), &binding); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	for _, raw := range []string{`{}`, `{"install_intent":""}`, `{"install_intent":"prepare"}`} {
		var binding ClientBinding
		if err := json.Unmarshal([]byte(raw), &binding); err != nil {
			t.Fatal(err)
		}
	}
}
