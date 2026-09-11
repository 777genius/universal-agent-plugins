package agentpluginscli

import (
	"fmt"
	"io"

	"github.com/777genius/plugin-kit-ai/cli/internal/outputjson"
)

const outputSchemaVersion = 1

const (
	outputResultSuccess = "success"
	outputResultFailure = "failure"
)

type outputResultProvider interface {
	outputResult() string
}

type outputEnvelope = outputjson.Envelope

func writeJSONOutput(writer io.Writer, command string, data any) error {
	result := outputResultSuccess
	if provider, ok := data.(outputResultProvider); ok {
		if provided := provider.outputResult(); provided != "" {
			result = provided
		}
	}
	return writeJSONResult(writer, command, result, data)
}

func writeJSONResult(writer io.Writer, command, result string, data any) error {
	return outputjson.Write(writer, command, result, data)
}

func writeProgress(app App, _ string, message string) {
	if message == "" {
		return
	}
	_, _ = fmt.Fprintln(app.errorOutput(), message)
}
