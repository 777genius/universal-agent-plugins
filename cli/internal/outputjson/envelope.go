// Package outputjson writes the shared one-document CLI envelope.
package outputjson

import (
	"encoding/json"
	"io"
)

const SchemaVersion = 1
const Success = "success"
const Failure = "failure"

type Envelope struct {
	SchemaVersion int    `json:"schema_version"`
	Command       string `json:"command"`
	Result        string `json:"result"`
	Data          any    `json:"data"`
}

func Write(w io.Writer, command, result string, data any) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(Envelope{SchemaVersion, command, result, data})
}
