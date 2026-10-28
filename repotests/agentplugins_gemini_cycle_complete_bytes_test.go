//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const geminiCycleCompleteMode = "gemini-complete-cycle"

var geminiCompleteProofOrder = []string{"source-user-U-one", "source-a-A-one", "source-b-B-one", "add-user", "add-a", "add-b", "repeat-add-a", "unchanged-update-a", "intact-repair-a", "before-source-revision-a", "source-a-A-two", "update-a", "before-missing-a", "missing-repair-a", "remove-a", "repeat-remove-a", "remove-b", "remove-user"}
var geminiCompleteObservations = []string{"listing:baseline-a", "listing:baseline-b", "listing:baseline-user", "native:baseline-a", "listing:installed-user", "native:installed-user", "listing:installed-a", "native:installed-a", "listing:installed-b", "native:installed-b", "listing:repeat-add-a", "listing:unchanged-update-a", "listing:intact-repair-a", "native:restarted-a", "listing:updated-a", "native:updated-a", "listing:repaired-a", "native:repaired-a", "listing:removed-a", "native:removed-a", "listing:removed-b", "native:removed-b", "listing:removed-user", "native:removed-user"}

var geminiCompleteTimestamp = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(?:\.[0-9]{1,9})?(?:Z|[+-](?:[01][0-9]|2[0-3]):[0-5][0-9])$`)

type geminiCompleteRow struct {
	Stage string
	Value map[string]any
}

func geminiCompleteRefusal() error {
	return fmt.Errorf("Gemini complete raw semantic admission refused")
}

func geminiCompleteInteger(value any, low, high int64) bool {
	number, ok := value.(json.Number)
	if !ok {
		return false
	}
	integer, err := number.Int64()
	return err == nil && integer >= low && integer <= high
}

func geminiCompleteIntegerIs(value any, expected int64) bool {
	return geminiCompleteInteger(value, expected, expected)
}

func geminiCompleteText(value any, limit int, nonempty bool) bool {
	text, ok := value.(string)
	return ok && len(text) <= limit && utf8.ValidString(text) && !strings.ContainsRune(text, 0) && (!nonempty || text != "")
}

func geminiCompleteNormalizeIntegers(value any) any {
	switch item := value.(type) {
	case json.Number:
		if integer, err := item.Int64(); err == nil {
			return json.Number(strconv.FormatInt(integer, 10))
		}
	case map[string]any:
		result := map[string]any{}
		for key, child := range item {
			result[key] = geminiCompleteNormalizeIntegers(child)
		}
		return result
	case []any:
		result := make([]any, len(item))
		for index, child := range item {
			result[index] = geminiCompleteNormalizeIntegers(child)
		}
		return result
	}
	return value
}

func geminiCompleteObject(value any, required, optional string) (map[string]any, error) {
	row, ok := value.(map[string]any)
	if !ok || projectNativeRuntimeKeys(row, required, optional) != nil {
		return nil, geminiCompleteRefusal()
	}
	return row, nil
}

func geminiCompleteJSON(body []byte) (map[string]any, error) {
	if len(body) == 0 || len(body) > 1<<20 || !geminiCompleteScalars(body) || geminiCyclePrivateCredential(body) {
		return nil, geminiCompleteRefusal()
	}
	return geminiCompleteJSONObject(body)
}

// Whole-member traversal precedes row parsing, so JSONL rows share the original
// privacy budget instead of each acquiring a new text/node allowance.
func geminiCompleteJSONObject(body []byte) (map[string]any, error) {
	value, err := projectNativeRuntimeJSON(body)
	row, ok := value.(map[string]any)
	if err != nil || !ok || !geminiCompleteUnicodeEscapes(body) {
		return nil, geminiCompleteRefusal()
	}
	return row, nil
}

// encoding/json otherwise replaces unpaired UTF16 escapes lossily. The new
// success reader rejects those original bytes, matching Python UTF8 admission.
func geminiCompleteUnicodeEscapes(body []byte) bool {
	inString := false
	for index := 0; index < len(body); index++ {
		if body[index] == '"' {
			inString = !inString
			continue
		}
		if !inString || body[index] != '\\' {
			continue
		}
		index++
		if index >= len(body) {
			return false
		}
		if body[index] != 'u' {
			continue
		}
		if index+4 >= len(body) {
			return false
		}
		code, err := strconv.ParseUint(string(body[index+1:index+5]), 16, 16)
		if err != nil {
			return false
		}
		index += 4
		if code >= 0xdc00 && code <= 0xdfff {
			return false
		}
		if code < 0xd800 || code > 0xdbff {
			continue
		}
		if index+6 >= len(body) || body[index+1] != '\\' || body[index+2] != 'u' {
			return false
		}
		low, err := strconv.ParseUint(string(body[index+3:index+7]), 16, 16)
		if err != nil || low < 0xdc00 || low > 0xdfff {
			return false
		}
		index += 6
	}
	return !inString
}

func geminiCompleteJSONLines(body []byte) error {
	if !geminiCompleteScalars(body) {
		return geminiCompleteRefusal()
	}
	if len(body) == 0 {
		return nil
	}
	if body[len(body)-1] != '\n' {
		return geminiCompleteRefusal()
	}
	for _, line := range bytes.Split(body[:len(body)-1], []byte{'\n'}) {
		row, err := geminiCompleteJSONObject(line)
		if err != nil {
			return err
		}
		if _, err := geminiCompleteTime(row["timestamp"]); err != nil {
			return err
		}
	}
	return nil
}

func geminiCompleteTyped(value any, target any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return geminiCompleteRefusal()
	}
	return nil
}

func geminiCompleteStream(value any) ([]byte, error) {
	row, err := geminiCompleteObject(value, "bytes sha256 body", "")
	text, ok := row["body"].(string)
	if err != nil || !ok || len(text) > (1<<20)*4/3+4 {
		return nil, geminiCompleteRefusal()
	}
	body, err := base64.StdEncoding.Strict().DecodeString(text)
	if err != nil || base64.StdEncoding.EncodeToString(body) != text || len(body) > 1<<20 || !geminiCompleteIntegerIs(row["bytes"], int64(len(body))) || row["sha256"] != geminiCycleSHA(body) || !geminiCompleteScalars(body) || geminiCyclePrivateCredential(body) {
		return nil, geminiCompleteRefusal()
	}
	return body, nil
}

func geminiCompleteRows(body []byte, order []string, repeat bool) ([]geminiCompleteRow, error) {
	if len(body) > 1<<20 || len(body) > 0 && body[len(body)-1] != '\n' || !geminiCompleteScalars(body) || geminiCyclePrivateCredential(body) {
		return nil, geminiCompleteRefusal()
	}
	var result []geminiCompleteRow
	previous := -1
	for _, line := range bytes.Split(bytes.TrimSuffix(body, []byte{'\n'}), []byte{'\n'}) {
		if len(body) == 0 {
			break
		}
		row, err := geminiCompleteJSONObject(line)
		if err != nil || projectNativeRuntimeKeys(row, "stage value", "") != nil || len(result) >= 128 {
			return nil, geminiCompleteRefusal()
		}
		stage, ok := row["stage"].(string)
		value, vok := row["value"].(map[string]any)
		index := -1
		for at, candidate := range order {
			if stage == candidate {
				index = at
			}
		}
		if !ok || !vok || index < 0 || index < previous || index > previous+1 || !repeat && index == previous {
			return nil, geminiCompleteRefusal()
		}
		previous = index
		result = append(result, geminiCompleteRow{stage, value})
	}
	return result, nil
}

func geminiCompleteStrings(value any, want []string) bool {
	body, err := json.Marshal(value)
	var got []string
	return err == nil && json.Unmarshal(body, &got) == nil && reflect.DeepEqual(got, want)
}

func geminiCompleteTime(value any) (time.Time, error) {
	text, ok := value.(string)
	if !ok || len(text) > 64 || !geminiCompleteTimestamp.MatchString(text) {
		return time.Time{}, geminiCompleteRefusal()
	}
	stamp, err := time.Parse(time.RFC3339Nano, text)
	if err != nil || stamp.Year() < 1 {
		return time.Time{}, geminiCompleteRefusal()
	}
	return stamp, nil
}

func geminiCompleteScopePath(paths map[string]string, scope string) string {
	if scope == "user" {
		return paths["outside"]
	}
	return paths[scope]
}

func geminiCompleteProofStage(stage string) string {
	stages := map[string]string{"installed-user": "add-user", "installed-a": "add-a", "installed-b": "add-b", "restarted-a": "intact-repair-a", "updated-a": "update-a", "repaired-a": "missing-repair-a", "removed-a": "remove-a", "removed-b": "remove-b", "removed-user": "remove-user"}
	if strings.HasPrefix(stage, "baseline-") {
		return ""
	}
	if proof, ok := stages[stage]; ok {
		return proof
	}
	return stage
}
