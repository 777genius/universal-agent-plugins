package runtime

import (
	"encoding/json"
	"fmt"
)

const (
	// MaxPayloadBytes caps stdin and argv JSON payloads accepted by runtime decoders.
	MaxPayloadBytes = 1 << 20
)

func ValidatePayloadSize(body []byte, label string) error {
	if len(body) <= MaxPayloadBytes {
		return nil
	}
	return fmt.Errorf("%s exceeds max payload size of %d bytes", label, MaxPayloadBytes)
}

func DecodeJSONPayload[T any](body []byte, label string) (*T, error) {
	if err := ValidatePayloadSize(body, label); err != nil {
		return nil, err
	}
	var dto T
	if err := json.Unmarshal(body, &dto); err != nil {
		return nil, fmt.Errorf("decode %s: %w", label, err)
	}
	return &dto, nil
}
