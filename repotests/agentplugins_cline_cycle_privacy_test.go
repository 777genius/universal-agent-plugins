//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

type clineCyclePrivacyBudget struct{ text, nodes int }

func clineCyclePrivateCredential(raw []byte) bool {
	marker := []byte("uap-cline-private-")
	if len(raw) > 1<<20 || !utf8.Valid(raw) || bytes.Contains(raw, marker) {
		return true
	}
	budget := clineCyclePrivacyBudget{text: 4 * len(raw), nodes: 16384}
	return clineCycleCredentialJSON(raw, marker, 0, &budget)
}
func clineCycleCredentialJSON(raw, marker []byte, depth int, budget *clineCyclePrivacyBudget) bool {
	if depth > 64 || len(raw) > budget.text {
		return true
	}
	budget.text -= len(raw)
	value, err := projectNativeRuntimeJSON(raw)
	if err == nil {
		return clineCycleCredentialValue(value, marker, depth, budget)
	}
	if clineCyclePrivacyParseBound(err) {
		return true
	}
	if clineCycleNonfiniteRefusal(raw, err, budget) {
		return true
	}
	if !bytes.Contains(raw, []byte{'\n'}) {
		return false
	}
	for len(raw) != 0 {
		line, rest, _ := bytes.Cut(raw, []byte{'\n'})
		if len(line) > budget.text {
			return true
		}
		budget.text -= len(line)
		value, err := projectNativeRuntimeJSON(line)
		if clineCyclePrivacyParseBound(err) || clineCycleNonfiniteRefusal(line, err, budget) || (err == nil && clineCycleCredentialValue(value, marker, depth, budget)) {
			return true
		}
		raw = rest
	}
	return false
}
func clineCyclePrivacyParseBound(err error) bool {
	if err == nil {
		return false
	}
	switch err.Error() {
	case "JSON structure limit", "JSON key", "duplicate JSON key":
		return true
	}
	return false
}
func clineCycleNonfiniteRefusal(raw []byte, err error, budget *clineCyclePrivacyBudget) bool {
	if err == nil || err.Error() != "nonfinite JSON number" {
		return false
	}
	return clineCycleCompleteNumericJSON(raw, budget)
}
func clineCycleCompleteNumericJSON(raw []byte, budget *clineCyclePrivacyBudget) bool {
	if len(raw) > budget.text {
		return true
	}
	budget.text -= len(raw)
	d := json.NewDecoder(bytes.NewReader(raw))
	var value json.RawMessage
	if err := d.Decode(&value); err != nil {
		return clineCycleSyntaxResource(err)
	}
	err := d.Decode(&value)
	return errors.Is(err, io.EOF) || clineCycleSyntaxResource(err)
}
func clineCycleSyntaxResource(err error) bool {
	if err == nil || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return false
	}
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		return strings.Contains(syntax.Error(), "exceeded max depth")
	}
	return true
}
func clineCycleCredentialValue(value any, marker []byte, depth int, budget *clineCyclePrivacyBudget) bool {
	budget.nodes--
	if depth > 64 || budget.nodes < 0 {
		return true
	}
	switch node := value.(type) {
	case string:
		if bytes.Contains([]byte(node), marker) {
			return true
		}
		return clineCycleCredentialJSON([]byte(node), marker, depth+1, budget)
	case map[string]any:
		for key, item := range node {
			if clineCycleCredentialValue(key, marker, depth+1, budget) || clineCycleCredentialValue(item, marker, depth+1, budget) {
				return true
			}
		}
	case []any:
		for _, item := range node {
			if clineCycleCredentialValue(item, marker, depth+1, budget) {
				return true
			}
		}
	}
	return false
}
