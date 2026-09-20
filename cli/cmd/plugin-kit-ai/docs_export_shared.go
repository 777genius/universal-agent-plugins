package main

import "encoding/json"

func marshalJSON(value any) ([]byte, error) {
	return json.MarshalIndent(value, "", "  ")
}
