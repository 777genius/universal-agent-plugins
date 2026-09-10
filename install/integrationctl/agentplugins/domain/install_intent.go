package domain

import (
	"encoding/json"
	"fmt"
)

// InstallIntent is persisted per binding. Empty historical state means automatic.
type InstallIntent string

const (
	InstallIntentAutomatic InstallIntent = ""
	InstallIntentPrepare   InstallIntent = "prepare"
)

func (intent InstallIntent) Validate(client ClientID) error {
	switch intent {
	case InstallIntentAutomatic:
		return nil
	case InstallIntentPrepare:
		if client == ClientKiro {
			return nil
		}
	}
	return fmt.Errorf("unsupported install intent %q for client %s", intent, client)
}

// InstallPreference retains user intent after owned artifacts are removed. It
// carries no ownership or runtime verification evidence.
type InstallPreference struct {
	ClientID      ClientID      `json:"client_id"`
	Scope         InstallScope  `json:"scope"`
	InstallIntent InstallIntent `json:"install_intent"`
}

func (intent *InstallIntent) UnmarshalJSON(data []byte) error {
	var value *string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if value == nil {
		return fmt.Errorf("install intent must be a string")
	}
	switch InstallIntent(*value) {
	case InstallIntentAutomatic, InstallIntentPrepare:
		*intent = InstallIntent(*value)
		return nil
	}
	return fmt.Errorf("unsupported install intent %q", *value)
}
