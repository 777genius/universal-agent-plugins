package statev2

import (
	"encoding/json"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func FuzzStateJSON(f *testing.F) {
	f.Add([]byte(`{"schema_version":4,"installations":[]}`))
	f.Add([]byte(`{"schema_version":999,"installations":[]}`))
	f.Add([]byte(`{"schema_version":4,"installations":[],"unknown":true}`))
	f.Add([]byte(`{"schema_version":4,"installations":[]} trailing`))
	f.Fuzz(func(t *testing.T, body []byte) {
		if len(body) > 1<<20 {
			t.Skip()
		}
		var state domain.StateFileV2
		if err := decodeStrictJSON(body, &state); err != nil {
			return
		}
		if err := Validate(state); err != nil {
			return
		}
		encoded, err := json.Marshal(state)
		if err != nil {
			t.Fatalf("marshal validated state: %v", err)
		}
		var roundTrip domain.StateFileV2
		if err := decodeStrictJSON(encoded, &roundTrip); err != nil {
			t.Fatalf("decode validated round trip: %v", err)
		}
		if err := Validate(roundTrip); err != nil {
			t.Fatalf("validate round trip: %v", err)
		}
	})
}
