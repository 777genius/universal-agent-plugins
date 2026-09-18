package kiro

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

func writeACPRecord(writer io.Writer, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	body = append(body, '\n')
	written, err := writer.Write(body)
	if err == nil && written != len(body) {
		return io.ErrShortWrite
	}
	return err
}

func readBoundedACPLine(reader *bufio.Reader, total *int) ([]byte, error) {
	line, err := reader.ReadSlice('\n')
	*total += len(line)
	if errors.Is(err, bufio.ErrBufferFull) || len(line) > kiroACPMaxLine || *total > kiroACPMaxBytes {
		return nil, fmt.Errorf("ACP output exceeded its byte bound")
	}
	if err != nil {
		// ReadSlice may return both bytes and a terminal/deadline error. Those
		// Bytes accompanying EOF or another terminal read error are queued
		// protocol evidence. Preserve them as a partial-record failure.
		if len(line) > 0 {
			return nil, fmt.Errorf("%w: ACP output ended with a non-delimited %d-byte record", ErrACPPartialExit, len(line))
		}
		if errors.Is(err, io.EOF) {
			return nil, errKiroACPEarlyExit
		}
		return nil, fmt.Errorf("read ACP output: %w", err)
	}
	if len(line) == 1 {
		return nil, fmt.Errorf("ACP output contained an empty record")
	}
	return line[:len(line)-1], nil
}

func decodeACPMessage(line []byte) (acpMessage, error) {
	// encoding/json accepts invalid UTF-8 by replacing it with U+FFFD. ACP
	// evidence must remain byte-exact: replacement could turn a malformed
	// status, identity, object key, or extension value into trusted evidence.
	if !utf8.Valid(line) {
		return acpMessage{}, fmt.Errorf("malformed ACP JSON: record is not valid UTF-8")
	}
	if err := validateJSONSurrogateEscapes(line); err != nil {
		return acpMessage{}, fmt.Errorf("malformed ACP JSON: %w", err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(line)))
	decoder.UseNumber()
	value, err := shared.DecodeUniqueJSONValue(decoder)
	if err != nil {
		return acpMessage{}, fmt.Errorf("malformed ACP JSON: %w", err)
	}
	if _, err := decoder.Token(); err == nil || !errors.Is(err, io.EOF) {
		return acpMessage{}, fmt.Errorf("ACP record contains multiple JSON values")
	}
	document, ok := value.(map[string]any)
	if !ok || document["jsonrpc"] != "2.0" {
		return acpMessage{}, fmt.Errorf("ACP record has an invalid JSON-RPC envelope")
	}
	_, hasMethod := document["method"]
	_, hasID := document["id"]
	_, hasResult := document["result"]
	_, hasParams := document["params"]
	errorValue, hasError := document["error"]
	if hasMethod {
		if hasID || hasResult || hasError {
			return acpMessage{}, fmt.Errorf("ACP record mixes JSON-RPC request and response members")
		}
	} else if !hasID || hasParams || hasResult == hasError || hasError && errorValue == nil {
		return acpMessage{}, fmt.Errorf("ACP response does not have an exclusive result or error member")
	}
	return acpMessage{document: document}, nil
}

func validateJSONSurrogateEscapes(document []byte) error {
	inString := false
	for index := 0; index < len(document); index++ {
		switch document[index] {
		case '"':
			inString = !inString
		case '\\':
			if !inString || index+1 >= len(document) {
				continue
			}
			index++
			if document[index] != 'u' || index+4 >= len(document) {
				continue
			}
			value, ok := parseJSONHexQuad(document[index+1 : index+5])
			if !ok {
				continue // encoding/json reports the malformed escape itself
			}
			index += 4
			consumed, err := consumeJSONSurrogatePair(document, index, value)
			if err != nil {
				return err
			}
			index += consumed
		}
	}
	return nil
}

func consumeJSONSurrogatePair(document []byte, index int, value uint16) (int, error) {
	switch {
	case value >= 0xd800 && value <= 0xdbff:
		if index+6 >= len(document) || document[index+1] != '\\' || document[index+2] != 'u' {
			return 0, fmt.Errorf("unpaired high UTF-16 surrogate escape")
		}
		low, valid := parseJSONHexQuad(document[index+3 : index+7])
		if !valid || low < 0xdc00 || low > 0xdfff {
			return 0, fmt.Errorf("unpaired high UTF-16 surrogate escape")
		}
		return 6, nil
	case value >= 0xdc00 && value <= 0xdfff:
		return 0, fmt.Errorf("unpaired low UTF-16 surrogate escape")
	}
	return 0, nil
}

func parseJSONHexQuad(value []byte) (uint16, bool) {
	if len(value) != 4 {
		return 0, false
	}
	var result uint16
	for _, digit := range value {
		result <<= 4
		switch {
		case digit >= '0' && digit <= '9':
			result |= uint16(digit - '0')
		case digit >= 'a' && digit <= 'f':
			result |= uint16(digit-'a') + 10
		case digit >= 'A' && digit <= 'F':
			result |= uint16(digit-'A') + 10
		default:
			return 0, false
		}
	}
	return result, true
}

func validateInitializeResponse(message acpMessage) error {
	if !acpIDEquals(message.document["id"], 0) {
		return fmt.Errorf("initialize response has the wrong id")
	}
	result, ok := message.document["result"].(map[string]any)
	_, hasError := message.document["error"]
	if !ok || hasError || !acpIDEquals(result["protocolVersion"], 1) {
		return fmt.Errorf("initialize response did not select ACP protocol v1")
	}
	return nil
}

func acpIDEquals(value any, expected int) bool {
	number, ok := value.(json.Number)
	return ok && number.String() == fmt.Sprintf("%d", expected)
}
