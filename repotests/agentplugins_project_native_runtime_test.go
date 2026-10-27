//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"syscall"
	"testing"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
)

// Shared held-file mechanics for the source-only native project suites.
// Extracted unchanged from the reviewed Copilot runtime attempt.
func projectNativeRuntimeParents(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("noncanonical path")
	}
	parent := filepath.Dir(path)
	for {
		info, err := os.Lstat(parent)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("parent alias/non-directory")
		}
		next := filepath.Dir(parent)
		if next == parent {
			break
		}
		parent = next
	}
	return nil
}
func projectNativeRuntimeRead(path string, limit int64) ([]byte, error) {
	if err := projectNativeRuntimeParents(path); err != nil {
		return nil, err
	}
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	stat, ok := before.Sys().(*syscall.Stat_t)
	if !ok || stat.Nlink != 1 || stat.Uid != uint32(os.Getuid()) {
		return nil, fmt.Errorf("non-owned/single-link leaf")
	}
	return readProjectNativeSnapshot(path, before, limit)
}
func projectNativeRuntimeStableStat(left, right any) bool {
	a, okA := left.(*syscall.Stat_t)
	b, okB := right.(*syscall.Stat_t)
	return okA && okB && a.Dev == b.Dev && a.Ino == b.Ino && a.Mode == b.Mode && a.Nlink == b.Nlink && a.Uid == b.Uid && a.Gid == b.Gid && a.Size == b.Size && a.Mtim == b.Mtim && a.Ctim == b.Ctim
}
func projectNativeRuntimeDigest(path string) (digest string, err error) {
	if err := projectNativeRuntimeParents(path); err != nil {
		return "", err
	}
	before, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	stat, ok := before.Sys().(*syscall.Stat_t)
	if !ok || !before.Mode().IsRegular() || stat.Nlink != 1 || (stat.Uid != uint32(os.Getuid()) && stat.Uid != 0) || before.Size() > 512<<20 {
		return "", fmt.Errorf("immutable file type/size/ownership")
	}
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return "", err
	}
	file := os.NewFile(uintptr(fd), path)
	defer func() {
		if closeErr := file.Close(); err == nil && closeErr != nil {
			digest, err = "", fmt.Errorf("close immutable file: %w", closeErr)
		}
	}()
	same := func(info os.FileInfo) bool {
		return info != nil && os.SameFile(before, info) && info.Mode() == before.Mode() && info.Size() == before.Size() && info.ModTime().Equal(before.ModTime()) && projectNativeRuntimeStableStat(info.Sys(), before.Sys())
	}
	opened, err := file.Stat()
	if err != nil || !same(opened) {
		return "", fmt.Errorf("immutable held identity changed")
	}
	hash := sha256.New()
	n, err := io.Copy(hash, io.LimitReader(file, (512<<20)+1))
	if err != nil || n != before.Size() {
		return "", fmt.Errorf("immutable read changed")
	}
	after, err := file.Stat()
	if err != nil || !same(after) {
		return "", fmt.Errorf("immutable held identity drift")
	}
	current, err := os.Lstat(path)
	if err != nil || !same(current) {
		return "", fmt.Errorf("immutable path identity drift")
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
func projectNativeRuntimeWrite(t *testing.T, path string, body []byte, mode os.FileMode) {
	t.Helper()
	parent := filepath.Dir(path)
	existing := parent
	for {
		_, err := os.Lstat(existing)
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		existing = filepath.Dir(existing)
	}
	if err := projectNativeRuntimeParents(filepath.Join(existing, "leaf")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(parent, 0700); err != nil {
		t.Fatal(err)
	}
	if err := projectNativeRuntimeParents(path); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.Write(body)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatal("authored file write failed")
	}
}

const projectNativeRuntimeBodyLimit = 1 << 20

// This decoder rejects duplicate keys before typed validation. No request or
// nested tool argument may rely on encoding/json's last-key-wins behavior.
func projectNativeRuntimeJSON(body []byte) (any, error) {
	if len(body) == 0 || len(body) > projectNativeRuntimeBodyLimit || !utf8.Valid(body) {
		return nil, fmt.Errorf("JSON body limit")
	}
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	nodes := 0
	var value func(int) (any, error)
	value = func(depth int) (any, error) {
		nodes++
		if depth > 32 || nodes > 16384 {
			return nil, fmt.Errorf("JSON structure limit")
		}
		token, err := d.Token()
		if err != nil {
			return nil, err
		}
		if delimiter, ok := token.(json.Delim); ok {
			switch delimiter {
			case '{':
				m := map[string]any{}
				for d.More() {
					key, err := d.Token()
					if err != nil {
						return nil, err
					}
					name, ok := key.(string)
					if !ok || len(name) > 4096 {
						return nil, fmt.Errorf("JSON key")
					}
					if _, exists := m[name]; exists {
						return nil, fmt.Errorf("duplicate JSON key")
					}
					v, err := value(depth + 1)
					if err != nil {
						return nil, err
					}
					m[name] = v
				}
				end, err := d.Token()
				if err != nil || end != json.Delim('}') {
					return nil, fmt.Errorf("JSON object end")
				}
				return m, nil
			case '[':
				a := []any{}
				for d.More() {
					v, err := value(depth + 1)
					if err != nil {
						return nil, err
					}
					a = append(a, v)
				}
				end, err := d.Token()
				if err != nil || end != json.Delim(']') {
					return nil, fmt.Errorf("JSON array end")
				}
				return a, nil
			default:
				return nil, fmt.Errorf("unexpected JSON delimiter")
			}
		}
		if number, ok := token.(json.Number); ok {
			n, err := number.Float64()
			if err != nil || math.IsInf(n, 0) || math.IsNaN(n) {
				return nil, fmt.Errorf("nonfinite JSON number")
			}
		}
		return token, nil
	}
	v, err := value(0)
	if err != nil {
		return nil, err
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, fmt.Errorf("JSON trailing data")
	}
	return v, nil
}

func projectNativeRuntimeKeys(object map[string]any, required, optional string) error {
	allowed := map[string]bool{}
	for _, key := range strings.Fields(required + " " + optional) {
		allowed[key] = true
	}
	for _, key := range strings.Fields(required) {
		if _, exists := object[key]; !exists {
			return fmt.Errorf("missing field %s", key)
		}
	}
	for key := range object {
		if !allowed[key] {
			return fmt.Errorf("unknown field %s", key)
		}
	}
	return nil
}
func projectNativeRuntimeText(v any, limit int) bool {
	s, ok := v.(string)
	return ok && len(s) <= limit && !strings.ContainsRune(s, 0)
}
func projectNativeRuntimeNumber(v any, low, high float64, integer bool) bool {
	n, ok := v.(json.Number)
	if !ok {
		return false
	}
	f, err := n.Float64()
	return err == nil && f >= low && f <= high && (!integer || math.Trunc(f) == f)
}
func projectNativeRuntimeSchema(v any) error {
	m, ok := v.(map[string]any)
	if !ok || projectNativeRuntimeKeys(m, "type properties required additionalProperties", "") != nil || m["type"] != "object" || m["additionalProperties"] != false {
		return fmt.Errorf("probe argument schema")
	}
	properties, ok := m["properties"].(map[string]any)
	if !ok || len(properties) != 3 {
		return fmt.Errorf("probe properties")
	}
	for _, name := range []string{"nonce", "operation", "marker"} {
		if !reflect.DeepEqual(properties[name], map[string]any{"type": "string"}) {
			return fmt.Errorf("probe property %s", name)
		}
	}
	if !reflect.DeepEqual(m["required"], []any{"nonce"}) {
		return fmt.Errorf("probe required arguments")
	}
	return nil
}

func TestProjectNativeRuntimeValidationContracts(t *testing.T) {
	t.Run("whole JSON and exact keys", func(t *testing.T) {
		value, err := projectNativeRuntimeJSON([]byte(`{"required":1,"optional":"text"}`))
		if err != nil {
			t.Fatal(err)
		}
		object := value.(map[string]any)
		if err := projectNativeRuntimeKeys(object, "required", "optional"); err != nil {
			t.Fatal(err)
		}
		if projectNativeRuntimeKeys(object, "required missing", "optional") == nil || projectNativeRuntimeKeys(object, "required", "") == nil {
			t.Fatal("missing or unknown JSON field accepted")
		}
		for _, body := range []string{`{"required":1,"required":2}`, `{"nested":{"x":1,"x":2}}`, `{} {}`, `{"number":1e999}`} {
			if _, err := projectNativeRuntimeJSON([]byte(body)); err == nil {
				t.Fatalf("unsafe whole JSON accepted: %s", body)
			}
		}
	})
	t.Run("bounded scalar types", func(t *testing.T) {
		if !projectNativeRuntimeText("abc", 3) || projectNativeRuntimeText("abcd", 3) || !projectNativeRuntimeText("abcd", 4) || projectNativeRuntimeText("a\x00b", 3) || projectNativeRuntimeText(3, 3) {
			t.Fatal("bounded text contract differs")
		}
		if !projectNativeRuntimeNumber(json.Number("2"), 1, 3, true) || !projectNativeRuntimeNumber(json.Number("2.5"), 1, 3, false) {
			t.Fatal("valid exact numeric type rejected")
		}
		for _, value := range []any{2, "2", json.Number("0"), json.Number("4"), json.Number("2.5"), json.Number("NaN"), json.Number("1e999")} {
			if projectNativeRuntimeNumber(value, 1, 3, true) {
				t.Fatalf("invalid bounded integer accepted: %v", value)
			}
		}
	})
	t.Run("canonical probe argument schema", func(t *testing.T) {
		body := []byte(`{"type":"object","properties":{"nonce":{"type":"string"},"operation":{"type":"string"},"marker":{"type":"string"}},"required":["nonce"],"additionalProperties":false}`)
		value, err := projectNativeRuntimeJSON(body)
		if err != nil || projectNativeRuntimeSchema(value) != nil {
			t.Fatalf("canonical probe schema rejected: %v", err)
		}
		for _, kind := range []string{"extra", "additional", "required", "property"} {
			value, err := projectNativeRuntimeJSON(body)
			if err != nil {
				t.Fatal(err)
			}
			object := value.(map[string]any)
			switch kind {
			case "extra":
				object["extra"] = true
			case "additional":
				object["additionalProperties"] = true
			case "required":
				object["required"] = []any{"nonce", "marker"}
			case "property":
				object["properties"].(map[string]any)["nonce"] = map[string]any{"type": "string", "description": "unknown"}
			}
			if projectNativeRuntimeSchema(object) == nil {
				t.Fatalf("noncanonical probe %s schema accepted", kind)
			}
		}
	})
}

// The public non-facade lifecycle records packagesnapshot framing for installed
// managed objects. Source PackageSnapshot.TreeDigest retains its separate v1
// framing. Inputs here are previously bounded, held ordinary-file captures.
func projectNativeRuntimeInstalledDigest(ctx context.Context, captured []packagedigest.CapturedEntry) (string, error) {
	if _, err := packagedigest.DigestCaptured(ctx, captured); err != nil {
		return "", err
	}
	items := append([]packagedigest.CapturedEntry(nil), captured...)
	sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })
	hash := sha256.New()
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		kind := "file"
		switch item.Kind {
		case "directory":
			kind = "dir"
		case "file":
		default:
			return "", fmt.Errorf("installed package capture requires ordinary files/directories")
		}
		_, _ = fmt.Fprintf(hash, "%s\x00%s\x00%t\x00%d\x00", kind, item.Path, item.Executable, len(item.Content))
		for body := item.Content; len(body) > 0; {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			n := min(len(body), 32*1024)
			_, _ = hash.Write(body[:n])
			body = body[n:]
		}
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}
