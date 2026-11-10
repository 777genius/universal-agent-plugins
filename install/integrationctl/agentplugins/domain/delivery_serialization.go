package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

func (d SelectedDelivery) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Mode   string              `json:"mode"`
		Local  *LocalDeliveryFacts `json:"local,omitempty"`
		Cursor CursorDeliveryFacts `json:"cursor,omitzero"`
	}{d.mode, d.local, d.cursor})
}

// Decoding retains unknown mode evidence so mutation can refuse without
// reinterpreting it as a historical receipt. No constructor/environment reads.
func (d *SelectedDelivery) UnmarshalJSON(raw []byte) error {
	var wire struct {
		Mode   string              `json:"mode"`
		Local  *LocalDeliveryFacts `json:"local,omitempty"`
		Cursor CursorDeliveryFacts `json:"cursor,omitzero"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return err
	}
	result := SelectedDelivery{mode: wire.Mode, cursor: wire.Cursor}
	if wire.Local != nil {
		result.local = cloneLocalFacts(*wire.Local)
	}
	if result.IsZero() {
		return fmt.Errorf("selected delivery record has no mode")
	}
	*d = result
	return nil
}

// New Cursor carriers are closed. Historical Local decoding stays permissive.
func decodeCursorValue(raw []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return fmt.Errorf("cursor carrier must be an object")
	}
	seen := map[string]bool{}
	for d.More() {
		token, err := d.Token()
		if err != nil {
			return err
		}
		key := token.(string)
		// Only the two fixed Cursor wire structs reach this decoder. Match
		// their bounded tag set with encoding/json's Unicode equivalence.
		for _, tag := range []string{"profile_root", "hooks_path", "profile_identity", "cursor_version", "target_os", "target_arch", "qualification_id", "executable", "selector", "shell", "object_id", "entry_digest", "canonical_digest", "projection_digest", "planned_receipt", "original_exists", "original_raw_digest", "version", "event", "remainder_digest"} {
			if strings.EqualFold(key, tag) {
				key = tag
				break
			}
		}
		if seen[key] {
			return fmt.Errorf("duplicate Cursor authority field %q", key)
		}
		seen[key] = true
		var value json.RawMessage
		if err := d.Decode(&value); err != nil {
			return err
		}
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	return d.Decode(target)
}
func (f *CursorDeliveryFacts) UnmarshalJSON(raw []byte) error {
	type wire CursorDeliveryFacts
	var decoded wire
	if err := decodeCursorValue(raw, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	if _, exists := fields["original_exists"]; !exists || string(fields["original_exists"]) == "null" {
		return fmt.Errorf("cursor original existence must be explicit")
	}
	*f = CursorDeliveryFacts(decoded)
	return nil
}
func (r *CursorHookReceipt) UnmarshalJSON(raw []byte) error {
	type wire CursorHookReceipt
	var decoded wire
	if err := decodeCursorValue(raw, &decoded); err != nil {
		return err
	}
	*r = CursorHookReceipt(decoded)
	return nil
}
