//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/pelletier/go-toml/v2"
)

type grokConfigObservation struct {
	Status         string                `json:"status"`
	ExpectedSHA256 string                `json:"expected_sha256"`
	ObservedSHA256 string                `json:"observed_sha256"`
	Bytes          *int                  `json:"bytes,omitempty"`
	Comparison     *grokConfigComparison `json:"comparison,omitempty"`
}

type grokConfigComparison struct {
	BeforeFields     int                        `json:"before_fields"`
	AfterFields      int                        `json:"after_fields"`
	SemanticEqual    bool                       `json:"semantic_equal"`
	Known            map[string]grokConfigKnown `json:"known"`
	ChangedCount     int                        `json:"changed_count"`
	OmittedCount     int                        `json:"omitted_count"`
	SemanticEncoding string                     `json:"semantic_encoding"`
	Differences      []grokConfigDifference     `json:"differences"`
}

type grokConfigKnown struct {
	Present bool   `json:"present"`
	Type    string `json:"type"`
	Equal   bool   `json:"equal"`
}

type grokConfigDifference struct {
	KeySHA256    string `json:"key_sha256"`
	BeforeType   string `json:"before_type"`
	AfterType    string `json:"after_type"`
	BeforeSHA256 string `json:"before_sha256,omitempty"`
	AfterSHA256  string `json:"after_sha256,omitempty"`
}

func grokConfigSHA(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func (f *projectNativeCLIAttempt) grokConfigEligible(path, want, got string) bool {
	if f.spec.client != "grok" || f.base == nil || f.grok == nil || f.grok.listener == nil || f.evidence == nil ||
		!f.used["native-attempt"] || path != filepath.Join(f.base.Home, ".grok", "config.toml") || f.protected[path] != want ||
		len(want) != 64 || len(got) != 64 || strings.Trim(want+got, "0123456789abcdef") != "" || want == got {
		return false
	}
	for _, name := range []string{"native-attempt.stdout.log", "native-attempt.stderr.log", "native-attempt.command.json"} {
		if _, exists := f.files[name]; !exists {
			return false
		}
	}
	_, already := f.evidence["grok_config_mutation"]
	return !already
}

func (f *projectNativeCLIAttempt) observeGrokConfigMutation(path, want, got string) {
	if f.grokConfigEligible(path, want, got) {
		f.evidence["grok_config_mutation"] = f.grokReadConfigMutation(path, want, got, grokConfigHeldRead)
	}
}

func grokConfigHeldRead(path string, limit int64) ([]byte, error) {
	return grokConfigStableRead(path, limit, projectNativeRuntimeRead)
}

func grokConfigStableRead(path string, limit int64, read func(string, int64) ([]byte, error)) ([]byte, error) {
	parents, err := grokConfigParentIdentities(path)
	if err != nil {
		return nil, err
	}
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	body, err := read(path, limit)
	if err != nil {
		return nil, err
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) || !projectNativeRuntimeStableStat(before.Sys(), after.Sys()) {
		return nil, fmt.Errorf("Grok config held metadata changed")
	}
	for parent, held := range parents {
		current, err := os.Lstat(parent)
		if err != nil || !grokConfigSameParent(held, current) {
			return nil, fmt.Errorf("Grok config parent identity changed")
		}
	}
	return body, nil
}

func grokConfigSameParent(held, current os.FileInfo) bool {
	a, okA := held.Sys().(*syscall.Stat_t)
	b, okB := current.Sys().(*syscall.Stat_t)
	return okA && okB && current.IsDir() && current.Mode()&os.ModeSymlink == 0 && os.SameFile(held, current) &&
		held.Mode() == current.Mode() && a.Uid == b.Uid && a.Gid == b.Gid
}

func grokConfigParentIdentities(path string) (map[string]os.FileInfo, error) {
	if err := projectNativeRuntimeParents(path); err != nil {
		return nil, err
	}
	parents := map[string]os.FileInfo{}
	for parent := filepath.Dir(path); ; parent = filepath.Dir(parent) {
		info, err := os.Lstat(parent)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("Grok config parent is not ordinary")
		}
		parents[parent] = info
		if filepath.Dir(parent) == parent {
			return parents, nil
		}
	}
}

func (f *projectNativeCLIAttempt) grokReadConfigMutation(path, want, got string, read func(string, int64) ([]byte, error)) grokConfigObservation {
	observation := grokConfigObservation{ExpectedSHA256: want, ObservedSHA256: got, Status: "work-deadline"}
	if !time.Now().Before(f.started.Add(150 * time.Second)) {
		return observation
	}
	body, err := read(path, 64<<10)
	if err != nil || projectNativeRuntimeParents(path) != nil {
		observation.Status = "read-refused"
		return observation
	}
	count := len(body)
	observation.Bytes = &count
	if !time.Now().Before(f.started.Add(150 * time.Second)) {
		return observation
	}
	if len(body) > 64<<10 || grokConfigSHA(body) != got {
		observation.Status = "observed-digest-mismatch"
		return observation
	}
	if !bytes.Equal(f.grok.authoredConfig, f.grok.configuration()) || grokConfigSHA(f.grok.authoredConfig) != want {
		observation.Status = "recipe-mismatch"
		return observation
	}
	observation.Comparison, observation.Status = grokConfigProject(f.grok.authoredConfig, body)
	if !time.Now().Before(f.started.Add(150 * time.Second)) {
		observation.Comparison, observation.Status = nil, "work-deadline"
	}
	return grokConfigBound(observation)
}

func grokConfigParse(body []byte) (map[string]any, string) {
	if len(body) > 64<<10 || !utf8.Valid(body) {
		return nil, "toml-syntax"
	}
	value := map[string]any{}
	if err := toml.Unmarshal(body, &value); err != nil {
		return nil, "toml-syntax"
	}
	nodes := 16384
	if !grokConfigValueValid(value, 0, &nodes) {
		return nil, "toml-value"
	}
	canonical, err := json.Marshal(value)
	if err != nil || len(canonical) > 64<<10 {
		return nil, "semantic-bound"
	}
	if grokProjectPrivateCredential(canonical) {
		return nil, "private-content"
	}
	return value, "captured"
}

func grokConfigValueValid(value any, depth int, nodes *int) bool {
	*nodes--
	if depth > 32 || *nodes < 0 {
		return false
	}
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			if len(key) > 4096 || !utf8.ValidString(key) || !grokConfigValueValid(key, depth+1, nodes) || !grokConfigValueValid(item, depth+1, nodes) {
				return false
			}
		}
	case []any:
		for _, item := range typed {
			if !grokConfigValueValid(item, depth+1, nodes) {
				return false
			}
		}
	case string:
		return utf8.ValidString(typed)
	case bool, int64:
		return true
	case float64:
		return !math.IsInf(typed, 0) && !math.IsNaN(typed)
	default:
		return false
	}
	return true
}

func grokConfigType(value any, present bool) string {
	if !present {
		return "absent"
	}
	switch value.(type) {
	case map[string]any:
		return "table"
	case []any:
		return "array"
	case string:
		return "string"
	case bool:
		return "boolean"
	case int64:
		return "integer"
	case float64:
		return "float"
	}
	return "unsupported"
}

func grokConfigKnownFacts(before, after map[string]any) map[string]grokConfigKnown {
	paths := []string{"models.default", "model.uap-grok-fixture.model", "model.uap-grok-fixture.base_url", "model.uap-grok-fixture.env_key",
		"model.uap-grok-fixture.api_backend", "model.uap-grok-fixture.supports_backend_search", "model.uap-grok-fixture.supports_reasoning_effort", "model.uap-grok-fixture.max_retries"}
	facts := make(map[string]grokConfigKnown, len(paths))
	for _, path := range paths {
		expected, expectedPresent := grokConfigLookup(before, path)
		actual, present := grokConfigLookup(after, path)
		facts[path] = grokConfigKnown{Present: present, Type: grokConfigType(actual, present), Equal: expectedPresent == present && reflect.DeepEqual(expected, actual)}
	}
	return facts
}

func grokConfigLookup(value map[string]any, path string) (any, bool) {
	var current any = value
	for _, key := range strings.Split(path, ".") {
		table, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = table[key]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func grokConfigProject(beforeBody, afterBody []byte) (*grokConfigComparison, string) {
	before, status := grokConfigParse(beforeBody)
	if status != "captured" {
		return nil, "recipe-" + status
	}
	after, status := grokConfigParse(afterBody)
	if status != "captured" {
		return nil, status
	}
	differences, ok := grokConfigDifferences(before, after)
	if !ok {
		return nil, "semantic-bound"
	}
	return &grokConfigComparison{BeforeFields: len(before), AfterFields: len(after), SemanticEqual: reflect.DeepEqual(before, after),
		Known: grokConfigKnownFacts(before, after), ChangedCount: len(differences), SemanticEncoding: "standard Go encoding/json.Marshal",
		Differences: differences}, "captured"
}

func grokConfigDifferences(before, after map[string]any) ([]grokConfigDifference, bool) {
	keys := make(map[string]bool, len(before)+len(after))
	for key := range before {
		keys[key] = true
	}
	for key := range after {
		keys[key] = true
	}
	differences := make([]grokConfigDifference, 0, len(keys))
	for key := range keys {
		left, had := before[key]
		right, has := after[key]
		if had == has && reflect.DeepEqual(left, right) {
			continue
		}
		difference := grokConfigDifference{KeySHA256: grokConfigSHA([]byte(key)), BeforeType: grokConfigType(left, had), AfterType: grokConfigType(right, has)}
		var ok bool
		if difference.BeforeSHA256, ok = grokConfigValueSHA(left, had); !ok {
			return nil, false
		}
		if difference.AfterSHA256, ok = grokConfigValueSHA(right, has); !ok {
			return nil, false
		}
		differences = append(differences, difference)
	}
	sort.Slice(differences, func(i, j int) bool { return differences[i].KeySHA256 < differences[j].KeySHA256 })
	return differences, true
}

func grokConfigValueSHA(value any, present bool) (string, bool) {
	if !present {
		return "", true
	}
	body, err := json.Marshal(value)
	if err != nil || len(body) > 64<<10 {
		return "", false
	}
	return grokConfigSHA(body), true
}

func grokConfigBound(observation grokConfigObservation) grokConfigObservation {
	comparison := observation.Comparison
	if comparison != nil && len(comparison.Differences) > 3 {
		comparison.Differences = comparison.Differences[:3]
	}
	for {
		if comparison != nil {
			comparison.OmittedCount = comparison.ChangedCount - len(comparison.Differences)
			if comparison.OmittedCount > 0 {
				observation.Status = "entry-limit"
			}
		}
		body, err := json.Marshal(observation)
		if err == nil && len(body) <= 2<<10 {
			return observation
		}
		if comparison == nil || len(comparison.Differences) == 0 {
			observation.Comparison, observation.Status = nil, "encoded-bound"
			return observation
		}
		comparison.Differences = comparison.Differences[:len(comparison.Differences)-1]
	}
}
