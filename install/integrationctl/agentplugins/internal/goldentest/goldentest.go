// Package goldentest pins the observable behavior of the install core before
// it is refactored. It renders every exported field, including the ones the
// production json tags hide, because a refactor must not change internal
// operational values either.
package goldentest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// UpdateEnv names the environment variable that rewrites golden files instead
// of comparing against them: UPDATE_GOLDEN=1 go test ./...
const UpdateEnv = "UPDATE_GOLDEN"

// Replacement rewrites a volatile substring, typically a temporary directory,
// into a stable token so the golden file is reproducible across machines.
type Replacement struct {
	From string
	To   string
}

// Golden compares rendered values against files under Dir.
type Golden struct {
	// Dir defaults to testdata/golden relative to the calling package.
	Dir string
	// Replace is applied to every string in order; list longer prefixes first.
	Replace []Replacement
}

// Assert compares value against Dir/name.json, or rewrites it when UPDATE_GOLDEN
// is set. It fails with the full rendered value so a diff is readable.
func (golden Golden) Assert(t *testing.T, name string, value any) {
	t.Helper()
	rendered, err := json.MarshalIndent(golden.render(reflect.ValueOf(value)), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	rendered = append(rendered, '\n')
	path := filepath.Join(golden.dir(), name+".json")
	if os.Getenv(UpdateEnv) != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, rendered, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v; rerun with %s=1 to record the baseline", err, UpdateEnv)
	}
	if string(expected) != string(rendered) {
		t.Fatalf("%s changed behavior.\n--- recorded ---\n%s\n--- current ---\n%s", path, expected, rendered)
	}
}

func (golden Golden) dir() string {
	if strings.TrimSpace(golden.Dir) != "" {
		return golden.Dir
	}
	return filepath.Join("testdata", "golden")
}

func (golden Golden) render(value reflect.Value) any {
	switch value.Kind() {
	case reflect.Invalid:
		return nil
	case reflect.Pointer, reflect.Interface:
		if value.IsNil() {
			return nil
		}
		return golden.render(value.Elem())
	case reflect.Struct:
		return golden.renderStruct(value)
	case reflect.Slice, reflect.Array:
		return golden.renderSequence(value)
	case reflect.Map:
		return golden.renderMap(value)
	case reflect.String:
		return golden.text(value.String())
	case reflect.Func, reflect.Chan, reflect.UnsafePointer:
		return fmt.Sprintf("<%s>", value.Kind())
	default:
		return value.Interface()
	}
}

func (golden Golden) renderStruct(value reflect.Value) any {
	if stamp, ok := value.Interface().(time.Time); ok {
		return stamp.UTC().Format(time.RFC3339Nano)
	}
	fields := make(map[string]any, value.NumField())
	for index := range value.NumField() {
		field := value.Type().Field(index)
		if !field.IsExported() {
			continue
		}
		fields[field.Name] = golden.render(value.Field(index))
	}
	return fields
}

func (golden Golden) renderSequence(value reflect.Value) any {
	if value.Kind() == reflect.Slice && value.IsNil() {
		return nil
	}
	// Raw JSON and other byte slices read as text, not as a list of numbers.
	if value.Type().Elem().Kind() == reflect.Uint8 {
		return golden.text(string(value.Bytes()))
	}
	items := make([]any, value.Len())
	for index := range value.Len() {
		items[index] = golden.render(value.Index(index))
	}
	return items
}

func (golden Golden) renderMap(value reflect.Value) any {
	if value.IsNil() {
		return nil
	}
	// json.Marshal sorts string keys, so the rendered file is stable.
	entries := make(map[string]any, value.Len())
	for _, key := range value.MapKeys() {
		entries[golden.text(fmt.Sprint(key.Interface()))] = golden.render(value.MapIndex(key))
	}
	return entries
}

// text stabilizes machine-specific paths: volatile roots become tokens and
// Windows separators become slashes so one golden file serves every platform.
func (golden Golden) text(value string) string {
	for _, replacement := range golden.Replace {
		value = strings.ReplaceAll(value, replacement.From, replacement.To)
	}
	if filepath.Separator != '/' {
		value = strings.ReplaceAll(value, string(filepath.Separator), "/")
	}
	return value
}
