package vscodelocalhooks

import (
	"encoding/json"
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/tailscale/hujson"
)

// VerifyOwned checks an entire already-owned authored hook file against the
// trusted target/specs. Whitespace, key order and equivalent JSON escapes are
// immaterial. Extra fields/events/commands, duplicates and any changed command
// fail. It does not establish filesystem ownership, adopt foreign hooks, plan
// edits or verify a native client. A later projector must supply package/digest
// ownership before using this check; unknown native data is never discarded.
func VerifyOwned(body []byte, target Target, specs []Spec) error {
	expected, err := Render(target, specs)
	if err != nil {
		return err
	}
	if err := checkJSON(body); err != nil {
		return err
	}
	actualAST, err := hujson.Parse(body)
	if err != nil {
		return fmt.Errorf("%w: JSON parse", ErrInvalid)
	}
	expectedAST, err := hujson.Parse(expected)
	if err != nil {
		return err
	}
	if !sameAuthoredValue(actualAST.Value, expectedAST.Value) {
		return ErrNotOwned
	}
	return nil
}

// Comparison follows the tiny expected schema, rather than interpreting or
// normalizing arbitrary native settings. Duplicate keys cannot match it.
func sameAuthoredValue(actual, expected hujson.ValueTrimmed) bool {
	switch want := expected.(type) {
	case *hujson.Object:
		got, ok := actual.(*hujson.Object)
		if !ok || len(got.Members) != len(want.Members) {
			return false
		}
		members := make(map[string]hujson.ValueTrimmed, len(got.Members))
		for _, member := range got.Members {
			key := member.Name.Value.(hujson.Literal).String()
			if _, duplicate := members[key]; duplicate {
				return false
			}
			members[key] = member.Value.Value
		}
		for _, member := range want.Members {
			key := member.Name.Value.(hujson.Literal).String()
			if !sameAuthoredValue(members[key], member.Value.Value) {
				return false
			}
		}
		return true
	case *hujson.Array:
		got, ok := actual.(*hujson.Array)
		if !ok || len(got.Elements) != len(want.Elements) {
			return false
		}
		for i := range want.Elements {
			if !sameAuthoredValue(got.Elements[i].Value, want.Elements[i].Value) {
				return false
			}
		}
		return true
	case hujson.Literal:
		got, ok := actual.(hujson.Literal)
		if !ok {
			return false
		}
		var gotValue, wantValue any
		if json.Unmarshal(got, &gotValue) != nil || json.Unmarshal(want, &wantValue) != nil {
			return false
		}
		return gotValue == wantValue
	}
	return false
}

// Bound parsing before allocating an AST. JSON validity is supplied by the
// standard library; this pass adds UTF-8, surrogate and depth refusal. It is
// private to this complete authored artifact, not another settings parser.
func checkJSON(body []byte) error {
	if len(body) > MaxDocumentBytes || !utf8.Valid(body) || !json.Valid(body) {
		return fmt.Errorf("%w: document byte limit, UTF-8 or strict JSON", ErrInvalid)
	}
	depth, inString := 0, false
	for i := 0; i < len(body); i++ {
		switch {
		case body[i] == '"':
			inString = !inString
		case inString && body[i] == '\\':
			i++
			if body[i] == 'u' {
				end, ok := unicodeEscapeEnd(body, i)
				if !ok {
					return fmt.Errorf("%w: unpaired Unicode surrogate", ErrInvalid)
				}
				i = end
			}
		case !inString && (body[i] == '{' || body[i] == '['):
			depth++
			if depth > 8 {
				return fmt.Errorf("%w: document depth exceeds eight", ErrInvalid)
			}
		case !inString && (body[i] == '}' || body[i] == ']'):
			depth--
		}
	}
	return nil
}

// Input is valid JSON, so the first escape always has four valid hex digits.
func unicodeEscapeEnd(body []byte, index int) (int, bool) {
	value, _ := strconv.ParseUint(string(body[index+1:index+5]), 16, 16)
	end := index + 4
	if value < 0xd800 || value > 0xdfff {
		return end, true
	}
	if value > 0xdbff || end+6 >= len(body) || body[end+1] != '\\' || body[end+2] != 'u' {
		return end, false
	}
	low, err := strconv.ParseUint(string(body[end+3:end+7]), 16, 16)
	return end + 6, err == nil && low >= 0xdc00 && low <= 0xdfff
}
