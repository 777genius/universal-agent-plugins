package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

const DeliveryVSCodeLocalV1 = "vscode-local-v1"
const DeliveryCursorUserStopV1 = "cursor-user-stop-v1"

// SelectedDelivery is a frozen selected-mode record. Its zero value preserves
// historical ClientID policy. Accessors return copies, including pointer bools;
// copying a plan or assessment cannot expose mutable constructor inputs.
// It is operational authority and must not appear in public diagnostic JSON.
type SelectedDelivery struct {
	mode   string
	local  *LocalDeliveryFacts
	cursor CursorDeliveryFacts
}

// CursorDeliveryFacts is a value packet for one fixed user Stop route. The
// planned receipt and raw basis belong to this attempt, never to acknowledged
// ownership. No foreign document or mutable authority is retained.
type CursorDeliveryFacts struct {
	ProfileRoot       string            `json:"profile_root"`
	HooksPath         string            `json:"hooks_path"`
	ProfileIdentity   string            `json:"profile_identity"`
	CursorVersion     string            `json:"cursor_version"`
	TargetOS          string            `json:"target_os"`
	TargetArch        string            `json:"target_arch"`
	QualificationID   string            `json:"qualification_id"`
	Executable        string            `json:"executable"`
	Selector          string            `json:"selector"`
	Shell             string            `json:"shell"`
	ObjectID          string            `json:"object_id"`
	EntryDigest       string            `json:"entry_digest"`
	CanonicalDigest   string            `json:"canonical_digest"`
	ProjectionDigest  string            `json:"projection_digest,omitempty"`
	PlannedReceipt    CursorHookReceipt `json:"planned_receipt"`
	OriginalExists    bool              `json:"original_exists"`
	OriginalRawDigest string            `json:"original_raw_digest"`
}

func NewCursorDelivery(facts CursorDeliveryFacts) (SelectedDelivery, error) {
	result := SelectedDelivery{mode: DeliveryCursorUserStopV1, cursor: facts}
	return result, result.Validate()
}
func (d SelectedDelivery) CursorFacts() (CursorDeliveryFacts, bool) {
	return d.cursor, d.mode == DeliveryCursorUserStopV1
}
func (d SelectedDelivery) CanonicalDigest() string {
	if f, ok := d.CursorFacts(); ok {
		return f.CanonicalDigest
	}
	f, _ := d.LocalFacts()
	return f.CanonicalDigest
}
func (d SelectedDelivery) ProjectionDigest() string {
	if f, ok := d.CursorFacts(); ok {
		return f.ProjectionDigest
	}
	f, _ := d.LocalFacts()
	return f.ProjectionDigest
}
func (d SelectedDelivery) ProfileRoot() string {
	if f, ok := d.CursorFacts(); ok {
		return f.ProfileRoot
	}
	f, _ := d.LocalFacts()
	return f.ProfileRoot
}

// LocalDeliveryFacts describes one qualified physical profile and projection.
// These are adapter-established facts, not discovery from HOME or current env.
type LocalDeliveryFacts struct {
	ProfileRoot      string              `json:"profile_root"`
	SettingsPath     string              `json:"settings_path"`
	ProfileIdentity  string              `json:"profile_identity"`
	SettingsIdentity string              `json:"settings_identity"`
	Tuple            LocalQualifiedTuple `json:"qualified_tuple"`
	NativeStop       bool                `json:"native_stop"`
	MCPServers       []string            `json:"mcp_servers,omitempty"`
	Skills           []string            `json:"skills,omitempty"`
	CanonicalDigest  string              `json:"canonical_digest"`
	ProjectionDigest string              `json:"projection_digest,omitempty"`
	Registration     OwnedProfileEntry   `json:"registration"`
}

type LocalQualifiedTuple struct {
	VSCodeVersion   string `json:"vscode_version"`
	CopilotVersion  string `json:"copilot_version"`
	TargetOS        string `json:"target_os"`
	TargetShell     string `json:"target_shell,omitempty"`
	QualificationID string `json:"qualification_id"`
}

// OwnedProfileEntry stores only the owned selector and bool authority, never
// foreign settings or a whole-document preimage. Nil PreviousValue means absent;
// an explicit false remains false across JSON and recovery.
type OwnedProfileEntry struct {
	ObjectID      string `json:"object_id"`
	Selector      string `json:"selector"`
	DesiredValue  *bool  `json:"desired_value"`
	PreviousValue *bool  `json:"previous_value,omitempty"`
}

func NewLocalDelivery(facts LocalDeliveryFacts) (SelectedDelivery, error) {
	result := SelectedDelivery{mode: DeliveryVSCodeLocalV1, local: cloneLocalFacts(facts)}
	if err := result.Validate(); err != nil {
		return SelectedDelivery{}, err
	}
	return result, nil
}

func (d SelectedDelivery) Mode() string { return d.mode }
func (d SelectedDelivery) IsZero() bool {
	return d.mode == "" && d.local == nil && d.cursor == (CursorDeliveryFacts{})
}
func (d SelectedDelivery) LocalFacts() (LocalDeliveryFacts, bool) {
	if d.mode != DeliveryVSCodeLocalV1 || d.local == nil {
		return LocalDeliveryFacts{}, false
	}
	return *cloneLocalFacts(*d.local), true
}

func cloneLocalFacts(facts LocalDeliveryFacts) *LocalDeliveryFacts {
	facts.MCPServers = append([]string(nil), facts.MCPServers...)
	facts.Skills = append([]string(nil), facts.Skills...)
	facts.Registration.DesiredValue = cloneBool(facts.Registration.DesiredValue)
	facts.Registration.PreviousValue = cloneBool(facts.Registration.PreviousValue)
	return &facts
}
func cloneBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

// WithProjectionDigest seals the result of the existing stager, separately from
// canonical package identity. It does not change the original selected record.
func (d SelectedDelivery) WithProjectionDigest(digest string) (SelectedDelivery, error) {
	if d.IsZero() {
		return d, nil
	}
	if err := d.Validate(); err != nil {
		return SelectedDelivery{}, err
	}
	if !deliveryDigest(digest) {
		return SelectedDelivery{}, fmt.Errorf("selected delivery projection digest is invalid")
	}
	if facts, ok := d.CursorFacts(); ok {
		facts.ProjectionDigest = digest
		return NewCursorDelivery(facts)
	}
	facts, _ := d.LocalFacts()
	facts.ProjectionDigest = digest
	return NewLocalDelivery(facts)
}

// SameSelection excludes revision digests: update/refresh can change projected
// bytes, but cannot silently change profile, shell, components or authority.
func (d SelectedDelivery) SameSelection(other SelectedDelivery) bool {
	if d.mode != other.mode {
		return false
	}
	if d.IsZero() {
		return other.IsZero()
	}
	if a, ok := d.CursorFacts(); ok {
		b, otherOK := other.CursorFacts()
		a.CanonicalDigest, a.ProjectionDigest, a.OriginalRawDigest = "", "", ""
		b.CanonicalDigest, b.ProjectionDigest, b.OriginalRawDigest = "", "", ""
		a.OriginalExists, b.OriginalExists = false, false
		// Only the remainder of the actual plan changes with the per-attempt basis.
		a.PlannedReceipt.RemainderDigest, b.PlannedReceipt.RemainderDigest = "", ""
		return otherOK && a == b
	}
	a, ok := d.LocalFacts()
	b, otherOK := other.LocalFacts()
	if !ok || !otherOK {
		return false
	}
	a.CanonicalDigest, a.ProjectionDigest = "", ""
	b.CanonicalDigest, b.ProjectionDigest = "", ""
	return reflect.DeepEqual(a, b)
}

// SameProfile fences immutable physical entry authority across an explicitly
// reviewed projection refresh. A fresh qualified tuple/component selection is
// allowed there; ordinary add/update/repair require SameSelection.
func (d SelectedDelivery) SameProfile(other SelectedDelivery) bool {
	if d.mode != other.mode {
		return false
	}
	if d.IsZero() {
		return other.IsZero()
	}
	if _, ok := d.CursorFacts(); ok {
		return d.SameSelection(other)
	}
	a, ok := d.LocalFacts()
	b, otherOK := other.LocalFacts()
	if !ok || !otherOK {
		return false
	}
	a.Tuple, b.Tuple = LocalQualifiedTuple{}, LocalQualifiedTuple{}
	a.NativeStop, b.NativeStop = false, false
	a.MCPServers, b.MCPServers = nil, nil
	a.Skills, b.Skills = nil, nil
	a.CanonicalDigest, a.ProjectionDigest = "", ""
	b.CanonicalDigest, b.ProjectionDigest = "", ""
	return reflect.DeepEqual(a, b)
}

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
