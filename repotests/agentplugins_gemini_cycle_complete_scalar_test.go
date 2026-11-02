//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"unicode/utf8"
)

// Match complete nested JSON values, keys and JSONL lines already visited by
// the original privacy scanner. Its budgets remain shared across each member;
// incomplete ordinary text containing a literal backslash-u stays ordinary.
func geminiCompleteScalars(body []byte) bool {
	if len(body) > 1<<20 || !utf8.Valid(body) {
		return false
	}
	budget := geminiCyclePrivacyBudget{text: 4 * len(body), nodes: 16384}
	return budget.completeScalarJSON(body, 0)
}

func (b *geminiCyclePrivacyBudget) completeScalarJSON(body []byte, depth int) bool {
	if depth > 64 || len(body) > b.text {
		return false
	}
	b.text -= len(body)
	value, err := projectNativeRuntimeJSON(body)
	if err == nil {
		return geminiCompleteUnicodeEscapes(body) && b.completeScalarValue(value, depth)
	}
	if geminiCyclePrivacyParseBound(err) || geminiCycleNonfiniteRefusal(body, err, b) {
		return false
	}
	if !bytes.ContainsRune(body, '\n') {
		return true
	}
	for len(body) != 0 {
		line, rest, _ := bytes.Cut(body, []byte{'\n'})
		if len(line) > b.text {
			return false
		}
		b.text -= len(line)
		value, err := projectNativeRuntimeJSON(line)
		if geminiCyclePrivacyParseBound(err) || geminiCycleNonfiniteRefusal(line, err, b) || err == nil && (!geminiCompleteUnicodeEscapes(line) || !b.completeScalarValue(value, depth)) {
			return false
		}
		body = rest
	}
	return true
}

func (b *geminiCyclePrivacyBudget) completeScalarValue(value any, depth int) bool {
	b.nodes--
	if depth > 64 || b.nodes < 0 {
		return false
	}
	switch node := value.(type) {
	case string:
		return b.completeScalarJSON([]byte(node), depth+1)
	case map[string]any:
		for key, item := range node {
			if !b.completeScalarValue(key, depth+1) || !b.completeScalarValue(item, depth+1) {
				return false
			}
		}
	case []any:
		for _, item := range node {
			if !b.completeScalarValue(item, depth+1) {
				return false
			}
		}
	}
	return true
}
