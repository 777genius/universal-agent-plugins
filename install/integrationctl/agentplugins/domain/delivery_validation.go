package domain

import (
	"encoding/hex"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"
)

func (d SelectedDelivery) Validate() error {
	if d.IsZero() {
		return nil
	}
	if d.mode == DeliveryCursorUserStopV1 {
		return d.validateCursor()
	}
	if d.mode != DeliveryVSCodeLocalV1 || d.local == nil || d.cursor != (CursorDeliveryFacts{}) {
		return fmt.Errorf("unknown or incomplete selected delivery mode %q", d.mode)
	}
	facts := d.local
	if err := validateLocalProfile(facts); err != nil {
		return err
	}
	if facts.NativeStop && !deliveryText(facts.Tuple.TargetShell) {
		return fmt.Errorf("selected native Stop requires a qualified target shell")
	}
	if facts.Registration.PreviousValue != nil && !*facts.Registration.PreviousValue && facts.Registration.DesiredValue != nil && *facts.Registration.DesiredValue {
		return fmt.Errorf("selected delivery cannot re-enable an explicit false registration")
	}
	if facts.Registration.DesiredValue == nil || !deliveryDigest(facts.CanonicalDigest) {
		return fmt.Errorf("selected delivery authority or canonical digest is incomplete")
	}
	if facts.ProjectionDigest != "" && !deliveryDigest(facts.ProjectionDigest) {
		return fmt.Errorf("selected delivery projection digest is invalid")
	}
	if !deliveryNames(facts.MCPServers) || !deliveryNames(facts.Skills) {
		return fmt.Errorf("selected delivery components must be unique sorted names")
	}
	return nil
}

func validateLocalProfile(facts *LocalDeliveryFacts) error {
	for _, path := range []string{facts.ProfileRoot, facts.SettingsPath, facts.Registration.Selector} {
		if !deliveryText(path) || !filepath.IsAbs(path) || filepath.Clean(path) != path {
			return fmt.Errorf("selected delivery requires clean absolute physical paths")
		}
	}
	if filepath.Dir(facts.SettingsPath) != facts.ProfileRoot {
		return fmt.Errorf("selected delivery settings are outside the bound profile root")
	}
	for _, value := range []string{facts.ProfileIdentity, facts.SettingsIdentity, facts.Registration.ObjectID, facts.Tuple.VSCodeVersion, facts.Tuple.CopilotVersion, facts.Tuple.TargetOS, facts.Tuple.QualificationID} {
		if !deliveryText(value) {
			return fmt.Errorf("selected delivery profile/qualified tuple identity is incomplete")
		}
	}
	return nil
}

func deliveryText(value string) bool {
	return value != "" && len(value) <= 4096 && utf8.ValidString(value) && !strings.ContainsAny(value, "\x00\r\n")
}
func deliveryDigest(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	_, err := hex.DecodeString(value[7:])
	return err == nil && strings.ToLower(value) == value
}
func deliveryNames(names []string) bool {
	if !slices.IsSorted(names) {
		return false
	}
	for i, name := range names {
		if !deliveryText(name) || (i > 0 && names[i-1] == name) {
			return false
		}
	}
	return true
}

// ValidatePlan binds selected facts to actual package/projection inputs. Native
// shell and filesystem qualification belongs to the selected adapter in U4b.
func (d SelectedDelivery) ValidatePlan(plan DeliveryPlan, canonicalDigest string) error {
	if err := d.Validate(); err != nil {
		return err
	}
	if d.IsZero() {
		return nil
	}
	if f, ok := d.CursorFacts(); ok {
		if plan.ClientID != ClientCursor || plan.Scope != ScopeUser || f.CanonicalDigest != canonicalDigest || f.ProfileRoot != plan.NativeRegistryRoot {
			return fmt.Errorf("selected Cursor plan differs from fixed client/scope/package")
		}
		if authority := plan.ProfileAuthority(); authority != nil && authority.Facts().CanonicalRoot != f.ProfileRoot {
			return fmt.Errorf("cursor packet differs from frozen physical root")
		}
		return nil
	}
	facts := d.local
	if !supportsLocalDelivery(plan.ClientID) {
		return fmt.Errorf("selected Local delivery belongs to a different client")
	}
	if facts.CanonicalDigest != canonicalDigest || facts.Registration.Selector != plan.ActivePath {
		return fmt.Errorf("selected delivery differs from the canonical package or owned location")
	}
	if !slices.Equal(facts.MCPServers, SelectedMCPNames(plan)) {
		return fmt.Errorf("selected delivery MCP components differ from plan")
	}
	skills := []string{}
	for _, component := range plan.Components {
		if component.Kind == ComponentSkill && component.Support != SupportUnsupported {
			skills = append(skills, component.Name)
		}
	}
	slices.Sort(skills)
	if !slices.Equal(skills, facts.Skills) {
		return fmt.Errorf("selected delivery skill components differ from plan")
	}
	return nil
}

// EffectiveTraits preserves historical traits for an absent mode; a Local
// record supplies only the few lifecycle facts demonstrated by this delivery.
// Callers validate the record before using these facts to authorize effects.
func (d SelectedDelivery) EffectiveTraits(id ClientID) ClientTraits {
	traits := ClientTraitsFor(id)
	if d.IsZero() {
		return traits
	}
	traits.LifecycleKind = LifecycleNativeConfig
	traits.TracksNativeEffects = true
	traits.BindsNativeProfileRoot = true
	traits.SupportsPreparedRecovery = true
	traits.UsesManagedStdioLauncher = d.local != nil && len(d.local.MCPServers) != 0
	return traits
}
func (d SelectedDelivery) SharesBackend(id ClientID) bool {
	return d.IsZero() && len(BackendSiblings(id)) > 0
}

// ValidateClient keeps the fixed Cursor client identity with its selection.
// Historical and Local selections retain their existing caller validations.
func (d SelectedDelivery) ValidateClient(id ClientID) error {
	if _, ok := d.CursorFacts(); ok && id != ClientCursor {
		return fmt.Errorf("cursor selection on another client")
	}
	return nil
}

func (d SelectedDelivery) validateCursor() error {
	f := d.cursor
	if d.local != nil || f.CursorVersion != "2026.09.28-64d2043" || f.TargetOS != "linux" || f.TargetArch != "amd64" || f.Shell != "cursor-linux-user-3.22.12-single-quote" {
		return fmt.Errorf("unknown Cursor selection or qualification tuple")
	}
	if err := validateCursorProfile(f); err != nil {
		return err
	}
	if !deliveryDigest(f.CanonicalDigest) || !deliveryDigest(f.EntryDigest) || !deliveryDigest(f.OriginalRawDigest) || f.ProjectionDigest != "" && !deliveryDigest(f.ProjectionDigest) {
		return fmt.Errorf("cursor revision or attempt basis is incomplete")
	}
	if !f.OriginalExists && f.OriginalRawDigest != "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		return fmt.Errorf("absent Cursor original has nonempty raw digest")
	}
	return d.ValidateCursorReceipt(f.PlannedReceipt)
}

func validateCursorProfile(f CursorDeliveryFacts) error {
	for _, p := range []string{f.ProfileRoot, f.HooksPath, f.Executable, f.Selector} {
		if !deliveryText(p) || !filepath.IsAbs(p) || filepath.Clean(p) != p {
			return fmt.Errorf("cursor requires exact absolute authority paths")
		}
	}
	if f.HooksPath != filepath.Join(f.ProfileRoot, "hooks.json") || !deliveryText(f.ProfileIdentity) || !deliveryText(f.QualificationID) || !deliveryText(f.ObjectID) {
		return fmt.Errorf("cursor profile or object identity is incomplete")
	}
	return nil
}

func (d SelectedDelivery) ValidateCursorReceipt(r CursorHookReceipt) error {
	f, ok := d.CursorFacts()
	if !ok || r.Version != 1 || r.Event != "stop" || r.Executable != f.Executable || r.Selector != f.Selector || r.Shell != f.Shell || r.EntryDigest != f.EntryDigest || !deliveryDigest(r.RemainderDigest) {
		return fmt.Errorf("cursor receipt differs from frozen Stop specification")
	}
	return nil
}

func (d SelectedDelivery) CursorOwnership(r CursorHookReceipt) NativeObjectOwnership {
	f, _ := d.CursorFacts()
	return NativeObjectOwnership{ObjectID: f.ObjectID, Kind: "cursor_user_stop", Path: f.HooksPath, LogicalName: f.Selector, ManagedDigest: f.EntryDigest, ProtectionClass: "owned_selector", CursorReceipt: r}
}

// ValidateCursorObjects rejects zero, foreign and duplicate hook authority.
func (d SelectedDelivery) ValidateCursorObjects(objects []NativeObjectOwnership) error {
	_, selected := d.CursorFacts()
	count := 0
	for _, o := range objects {
		if o.CursorReceipt == (CursorHookReceipt{}) && o.Kind != "cursor_user_stop" {
			continue
		}
		if !selected {
			return fmt.Errorf("cursor receipt on an unselected binding")
		}
		count++
		if count > 1 {
			return fmt.Errorf("duplicate Cursor owned object")
		}
		if err := d.ValidateCursorReceipt(o.CursorReceipt); err != nil {
			return err
		}
		if o != d.CursorOwnership(o.CursorReceipt) {
			return fmt.Errorf("cursor object differs from frozen selection")
		}
	}
	return nil
}
