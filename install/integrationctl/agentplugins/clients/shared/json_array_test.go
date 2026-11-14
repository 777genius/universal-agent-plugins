package shared

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeUniqueJSONValuePreservesEmptyArrays(t *testing.T) {
	for _, original := range []string{
		`[]`,
		`{"argv":[]}`,
		`{"nested":[[],{"servers":[]}]}`,
		`{"argv":null}`,
	} {
		t.Run(original, func(t *testing.T) {
			decoder := json.NewDecoder(strings.NewReader(original))
			decoder.UseNumber()
			value, err := DecodeUniqueJSONValue(decoder)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if string(encoded) != original {
				t.Fatalf("decoded JSON changed array/null identity: got %s, want %s", encoded, original)
			}
		})
	}
}
