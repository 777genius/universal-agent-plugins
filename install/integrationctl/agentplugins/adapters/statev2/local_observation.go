package statev2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func validateObservationLinkage(client domain.ClientBinding) error {
	if err := client.ValidateLocalEntryObservation(); err != nil {
		return err
	}
	if err := client.SelectedDelivery.ValidateCursorObjects(client.NativeObjects); err != nil {
		return err
	}
	if _, ok := client.SelectedDelivery.CursorFacts(); ok {
		if err := client.SelectedDelivery.Validate(); err != nil {
			return err
		}
		if client.SelectedDelivery.ValidateClient(domain.ClientID(client.ClientID)) != nil || client.Scope != string(domain.ScopeUser) || client.NativeProfileRoot != client.SelectedDelivery.ProfileRoot() || client.LocalEntryObservation != nil {
			return fmt.Errorf("persisted Cursor binding linkage differs")
		}
	}
	if client.PendingNativeIntent != nil {
		return client.PendingNativeIntent.Validate(client)
	}
	return nil
}

// Only duplicate keys carrying observation authority are rejected here. The
// historical state schema remains otherwise governed by its existing decoder.
// Scan before typed decoding, including earlier values a duplicate would hide.
func rejectShadowedObservations(raw []byte) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if _, err := observationCarrierValue(d, "state"); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("state contains trailing JSON values")
	}
	return nil
}

func observationCarrierValue(d *json.Decoder, scope string) (bool, error) {
	token, err := d.Token()
	if err != nil {
		return false, err
	}
	switch token {
	case json.Delim('{'):
		return observationCarrierObject(d, scope)
	case json.Delim('['):
		childScope := ""
		if scope == "objects" {
			childScope = "owned_object"
		}
		if scope == "installations" {
			childScope = "installation"
		}
		observed := false
		for d.More() {
			found, err := observationCarrierValue(d, childScope)
			if err != nil {
				return false, err
			}
			observed = observed || found
		}
		_, err = d.Token()
		return observed, err
	}
	return false, nil
}

func observationCarrierObject(d *json.Decoder, scope string) (bool, error) {
	seen := map[string]bool{}
	observed := false
	shadowed := false
	for d.More() {
		token, err := d.Token()
		if err != nil {
			return false, err
		}
		key, ok := token.(string)
		if !ok {
			return false, fmt.Errorf("invalid state object key")
		}
		key = observationCarrierKey(scope, key)
		childScope, carrier := observationChildScope(scope, key)
		found, err := observationCarrierValue(d, childScope)
		if err != nil {
			return false, err
		}
		found = found || carrier
		prior, duplicate := seen[key]
		shadowed = shadowed || duplicate
		if duplicate && (prior || found) {
			return false, fmt.Errorf("duplicate observation carrier or ancestor %q", key)
		}
		seen[key] = prior || found
		observed = observed || found
	}
	if scope == "owned_object" && observed && shadowed {
		return false, fmt.Errorf("shadowed Cursor owned object")
	}
	_, err := d.Token()
	return observed, err
}

// Only these persisted binding/intent paths carry authority. A matching name
// inside permissive historical SelectedDelivery evidence is not a carrier.
func observationChildScope(scope, key string) (string, bool) {
	switch scope {
	case "cursor_authority":
		return "cursor_authority", true
	case "owned_object":
		if key == "cursor_receipt" {
			return "cursor_authority", true
		}
	case "selected":
		if key == "cursor" {
			return "cursor_authority", true
		}
		if key == "mode" {
			return "", true
		}

	case "state":
		if key == "installations" {
			return "installations", false
		}
	case "installation":
		if key == "clients" {
			return "clients", false
		}
	case "clients":
		return "binding", false
	case "binding", "intent":
		if key == "selected_delivery" {
			return "selected", false
		}
		if key == "native_objects" {
			return "objects", false
		}
		if key == "previous_cursor_object" {
			return "owned_object", true
		}
		if key == "local_entry_observation" {
			return "", true
		}
		if scope == "binding" && key == "pending_native_intent" {
			return "intent", false
		}
	}
	return "", false
}

// Match only known struct tags with the same Unicode equivalence as the typed
// JSON decoder. Client map IDs remain exact strings.
func observationCarrierKey(scope, key string) string {
	if scope == "clients" {
		return key
	}
	if scope == "cursor_authority" || scope == "owned_object" {
		key = strings.ToLower(key)
		// Fixed Cursor authority tags only: no pairwise scan over
		// arbitrary historical fields, and no change to Local decoding.
		for _, tag := range []string{"profile_root", "hooks_path", "profile_identity", "cursor_version", "target_os", "target_arch", "qualification_id", "executable", "selector", "shell", "object_id", "entry_digest", "canonical_digest", "projection_digest", "planned_receipt", "original_exists", "original_raw_digest", "version", "event", "remainder_digest", "kind", "logical_name", "path", "source_relative", "before_digest", "managed_digest", "protection_class", "user_modified"} {
			if strings.EqualFold(key, tag) {
				key = tag
				break
			}
		}
	}
	for _, tag := range []string{"installations", "clients", "pending_native_intent", "local_entry_observation", "selected_delivery", "cursor", "mode", "native_objects", "cursor_receipt", "previous_cursor_object"} {
		if strings.EqualFold(key, tag) {
			return tag
		}
	}
	return key
}
