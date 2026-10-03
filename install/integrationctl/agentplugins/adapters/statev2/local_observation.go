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
	for d.More() {
		token, err := d.Token()
		if err != nil {
			return false, err
		}
		key, ok := token.(string)
		if !ok {
			return false, fmt.Errorf("invalid state object key")
		}
		if scope != "clients" {
			key = observationCarrierKey(key)
		}
		childScope, carrier := observationChildScope(scope, key)
		found, err := observationCarrierValue(d, childScope)
		if err != nil {
			return false, err
		}
		found = found || carrier
		prior, duplicate := seen[key]
		if duplicate && (prior || found) {
			return false, fmt.Errorf("duplicate observation carrier or ancestor %q", key)
		}
		seen[key] = prior || found
		observed = observed || found
	}
	_, err := d.Token()
	return observed, err
}

// Only these persisted binding/intent paths carry authority. A matching name
// inside permissive historical SelectedDelivery evidence is not a carrier.
func observationChildScope(scope, key string) (string, bool) {
	switch scope {
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
// JSON decoder. Caller excludes clients, whose map IDs remain exact strings.
func observationCarrierKey(key string) string {
	for _, tag := range []string{"installations", "clients", "pending_native_intent", "local_entry_observation"} {
		if strings.EqualFold(key, tag) {
			return tag
		}
	}
	return key
}
