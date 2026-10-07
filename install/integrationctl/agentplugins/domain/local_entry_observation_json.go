package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strconv"
	"unicode/utf8"
)

const maxObservationBytes = 1 << 20
const maxObservationDepth = 64

// Decode a wire basis rather than SelectedDelivery: its historical decoder
// intentionally retains unknown-mode evidence and has a different contract.
func decodeLocalEntryObservation(raw []byte) (LocalEntryObservationFacts, error) {
	var wire struct {
		RevisionBasis struct {
			Mode  string              `json:"mode"`
			Local *LocalDeliveryFacts `json:"local"`
		} `json:"revision_basis"`
		Enabled       *bool  `json:"enabled"`
		ReceiptDigest string `json:"receipt_digest"`
	}
	if len(raw) > maxObservationBytes || !utf8.Valid(raw) {
		return LocalEntryObservationFacts{}, fmt.Errorf("local observation exceeds byte budget or contains invalid Unicode")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := observationJSONValue(decoder, raw, 0, ""); err != nil {
		return LocalEntryObservationFacts{}, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return LocalEntryObservationFacts{}, fmt.Errorf("observation contains trailing JSON")
	}
	decoder = json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return LocalEntryObservationFacts{}, err
	}
	if wire.Enabled == nil || wire.RevisionBasis.Mode != DeliveryVSCodeLocalV1 || wire.RevisionBasis.Local == nil {
		return LocalEntryObservationFacts{}, fmt.Errorf("observation requires enabled bool and Local revision basis")
	}
	basis, err := NewLocalDelivery(*wire.RevisionBasis.Local)
	return LocalEntryObservationFacts{RevisionBasis: basis, Enabled: *wire.Enabled, ReceiptDigest: wire.ReceiptDigest}, err
}

func observationJSONValue(d *json.Decoder, raw []byte, depth int, field string) error {
	if depth > maxObservationDepth {
		return fmt.Errorf("observation exceeds depth budget")
	}
	start := d.InputOffset()
	token, err := d.Token()
	if err != nil {
		return err
	}
	if _, ok := token.(string); ok {
		return observationUnicode(raw[start:d.InputOffset()])
	}
	switch token {
	case json.Delim('{'):
		return observationJSONObject(d, raw, depth, field)
	case json.Delim('['):
		for d.More() {
			if err := observationJSONValue(d, raw, depth+1, field); err != nil {
				return err
			}
		}
		_, err = d.Token()
	}
	return err
}

func observationJSONObject(d *json.Decoder, raw []byte, depth int, field string) error {
	seen := map[string]bool{}
	for d.More() {
		start := d.InputOffset()
		token, err := d.Token()
		if err != nil {
			return err
		}
		key, ok := token.(string)
		if !observationKnownField(field, key) {
			return fmt.Errorf("unknown observation field %q", key)
		}
		if !ok || seen[key] {
			return fmt.Errorf("duplicate or invalid observation key %q", key)
		}
		seen[key] = true
		if err := observationUnicode(raw[start:d.InputOffset()]); err != nil {
			return err
		}
		if err := observationJSONValue(d, raw, depth+1, key); err != nil {
			return err
		}
	}
	_, err := d.Token()
	return err
}

// encoding/json replaces unpaired UTF-16 escapes. Receipt identity must retain
// exact Unicode, so reject those escapes before typed decoding can replace them.
func observationUnicode(raw []byte) error {
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' {
			continue
		}
		i++
		if i >= len(raw) || raw[i] != 'u' {
			continue
		}
		value, err := observationEscape(raw, i)
		if err != nil {
			return err
		}
		i += 4
		if value >= 0xdc00 && value <= 0xdfff {
			return fmt.Errorf("unpaired observation Unicode surrogate")
		}
		if value < 0xd800 || value > 0xdbff {
			continue
		}
		if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
			return fmt.Errorf("unpaired observation Unicode surrogate")
		}
		low, err := observationEscape(raw, i+2)
		if err != nil || low < 0xdc00 || low > 0xdfff {
			return fmt.Errorf("invalid observation Unicode surrogate pair")
		}
		i += 6
	}
	return nil
}

func observationEscape(raw []byte, u int) (uint64, error) {
	if u+4 >= len(raw) {
		return 0, fmt.Errorf("incomplete observation Unicode escape")
	}
	return strconv.ParseUint(string(raw[u+1:u+5]), 16, 16)
}

func observationKnownField(parent, key string) bool {
	var fields []string
	switch parent {
	case "":
		fields = []string{"revision_basis", "enabled", "receipt_digest"}
	case "revision_basis":
		fields = []string{"mode", "local"}
	case "local":
		fields = []string{"profile_root", "settings_path", "profile_identity", "settings_identity", "qualified_tuple", "native_stop", "mcp_servers", "skills", "canonical_digest", "projection_digest", "registration"}
	case "qualified_tuple":
		fields = []string{"vscode_version", "copilot_version", "target_os", "target_shell", "qualification_id"}
	case "registration":
		fields = []string{"object_id", "selector", "desired_value", "previous_value"}
	}
	return slices.Contains(fields, key)
}
