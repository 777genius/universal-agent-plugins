package conformance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
)

func decodeJSONObject(body []byte) (map[string]json.RawMessage, map[string]any, error) {
	if err := rejectDuplicateTopLevelObjectKeys(body); err != nil {
		return nil, nil, err
	}
	var raw map[string]json.RawMessage
	if err := decodeJSONUnchecked(body, &raw); err != nil {
		return nil, nil, err
	}
	if raw == nil {
		return nil, nil, fmt.Errorf("document must be a JSON object")
	}
	var decoded map[string]any
	if err := decodeJSONUnchecked(body, &decoded); err != nil {
		return nil, nil, err
	}
	return raw, decoded, nil
}

func decodeJSON(body []byte, target any) error {
	if err := rejectDuplicateJSONKeys(body); err != nil {
		return err
	}
	return decodeJSONUnchecked(body, target)
}

func decodeRawJSONObject(body []byte, target any) error {
	if err := rejectDuplicateTopLevelObjectKeys(body); err != nil {
		return err
	}
	return decodeJSONUnchecked(body, target)
}

func decodeJSONUnchecked(body []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("document contains multiple JSON values")
		}
		return err
	}
	return nil
}

func rejectDuplicateTopLevelObjectKeys(body []byte) error {
	_, err := scanJSON(context.Background(), body, Limits{}, duplicateRoot)
	return err
}
func rejectDuplicateJSONKeys(body []byte) error {
	_, err := scanJSON(context.Background(), body, Limits{}, duplicateAll)
	return err
}
