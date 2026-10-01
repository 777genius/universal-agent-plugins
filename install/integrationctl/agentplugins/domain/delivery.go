package domain

import (
	"encoding/json"
	"fmt"
	"reflect"
)

const DeliveryVSCodeLocalV1 = "vscode-local-v1"

// SelectedDelivery is a frozen selected-mode record. Its zero value preserves
// historical ClientID policy. Accessors return copies, including pointer bools;
// copying a plan or assessment cannot expose mutable constructor inputs.
// It is operational authority and must not appear in public diagnostic JSON.
type SelectedDelivery struct {
	mode  string
	local *LocalDeliveryFacts
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
func (d SelectedDelivery) IsZero() bool { return d.mode == "" && d.local == nil }
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
		Mode  string              `json:"mode"`
		Local *LocalDeliveryFacts `json:"local,omitempty"`
	}{d.mode, d.local})
}

// Decoding retains unknown mode evidence so mutation can refuse without
// reinterpreting it as a historical receipt. No constructor/environment reads.
func (d *SelectedDelivery) UnmarshalJSON(raw []byte) error {
	var wire struct {
		Mode  string              `json:"mode"`
		Local *LocalDeliveryFacts `json:"local,omitempty"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return err
	}
	result := SelectedDelivery{mode: wire.Mode}
	if wire.Local != nil {
		result.local = cloneLocalFacts(*wire.Local)
	}
	if result.IsZero() {
		return fmt.Errorf("selected delivery record has no mode")
	}
	*d = result
	return nil
}
