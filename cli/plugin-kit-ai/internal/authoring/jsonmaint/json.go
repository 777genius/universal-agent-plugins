// Package jsonmaint provides bounded, lossless-value JSON normalization and
// digest-bound replacement for one standard package document.
package jsonmaint

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const MaxDocumentBytes = 4 << 20

type Error struct{ Code string }

func (e *Error) Error() string { return "JSON maintenance: " + e.Code }
func fail(code string) error   { return &Error{Code: code} }

type Plan struct {
	Document, BeforeSHA256, AfterSHA256 string
	before, after                       []byte
}

func (p Plan) Changed() bool { return !bytes.Equal(p.before, p.after) }

// Build creates an immutable plan from bytes already captured by packageview.
func Build(document string, source []byte) (Plan, error) {
	if document != "plugin.json" && document != "mcp.json" {
		return Plan{}, fail("document_invalid")
	}
	if len(source) == 0 {
		return Plan{}, fail("document_missing")
	}
	if len(source) > MaxDocumentBytes {
		return Plan{}, fail("document_oversize")
	}
	value, err := Decode(source)
	if err != nil {
		return Plan{}, err
	}
	after, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return Plan{}, fail("json_malformed")
	}
	after = append(after, '\n')
	return Plan{Document: document, BeforeSHA256: digest(source), AfterSHA256: digest(after), before: append([]byte(nil), source...), after: after}, nil
}

func digest(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// Decode rejects duplicate object keys at every depth and retains json.Number
// spellings so canonical formatting does not coerce integer or decimal values.
func Decode(body []byte) (any, error) {
	if len(body) == 0 || len(body) > MaxDocumentBytes {
		return nil, fail("json_malformed")
	}
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	v, err := decodeValue(d)
	if err != nil {
		var duplicate *duplicateError
		if errors.As(err, &duplicate) {
			return nil, fail("json_duplicate_key")
		}
		return nil, fail("json_malformed")
	}
	if _, err = d.Token(); err != io.EOF {
		return nil, fail("json_malformed")
	}
	return v, nil
}

type duplicateError struct{}

func (*duplicateError) Error() string { return "duplicate JSON key" }

func decodeValue(d *json.Decoder) (any, error) {
	t, err := d.Token()
	if err != nil {
		return nil, err
	}
	delim, compound := t.(json.Delim)
	if !compound {
		return t, nil
	}
	switch delim {
	case '{':
		object := map[string]any{}
		for d.More() {
			keyToken, err := d.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, fmt.Errorf("object key is not a string")
			}
			if _, exists := object[key]; exists {
				return nil, &duplicateError{}
			}
			value, err := decodeValue(d)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		end, err := d.Token()
		if err != nil || end != json.Delim('}') {
			return nil, fmt.Errorf("unterminated object")
		}
		return object, nil
	case '[':
		array := []any{}
		for d.More() {
			value, err := decodeValue(d)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		end, err := d.Token()
		if err != nil || end != json.Delim(']') {
			return nil, fmt.Errorf("unterminated array")
		}
		return array, nil
	default:
		return nil, fmt.Errorf("unexpected delimiter")
	}
}

type ApplyOptions struct {
	Root     string
	Validate func(context.Context) error
}

// Apply replaces one selected document. The atomic exchange keeps the old
// inode recoverable until full-package validation succeeds.
func Apply(ctx context.Context, plan Plan, opts ApplyOptions) (committed bool, err error) {
	if ctx == nil || opts.Validate == nil || !filepath.IsAbs(opts.Root) || filepath.Clean(opts.Root) != opts.Root {
		return false, fail("arguments_invalid")
	}
	if !plan.Changed() {
		return false, nil
	}
	root, err := os.OpenRoot(opts.Root)
	if err != nil {
		return false, fail("root_unreadable")
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	parent, err := root.Open(".")
	if err != nil {
		return false, fail("root_unreadable")
	}
	defer func() { err = errors.Join(err, parent.Close()) }()
	originalInfo, original, err := readCurrent(root, plan.Document)
	if err != nil || !bytes.Equal(original, plan.before) {
		return false, fail("source_changed")
	}
	if err = ctx.Err(); err != nil {
		return false, err
	}
	temp := ".authoring-json-" + randomName()
	f, err := root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return false, fail("temporary_create_failed")
	}
	tempInfo, statErr := f.Stat()
	if statErr == nil {
		_, err = f.Write(plan.after)
	} else {
		err = statErr
	}
	if err == nil {
		err = f.Chmod(originalInfo.Mode().Perm())
	}
	if err == nil {
		err = f.Sync()
	}
	err = errors.Join(err, f.Close())
	if err != nil {
		if now, e := root.Lstat(temp); e == nil && tempInfo != nil && os.SameFile(now, tempInfo) {
			_ = root.Remove(temp)
		}
		return false, fail("temporary_write_failed")
	}
	owned := true
	cleanupPath := temp
	cleanupInfo := tempInfo
	defer func() {
		if owned {
			if now, e := root.Lstat(cleanupPath); e == nil && os.SameFile(now, cleanupInfo) {
				err = errors.Join(err, root.Remove(cleanupPath), parent.Sync())
			}
		}
	}()
	currentInfo, current, readErr := readCurrent(root, plan.Document)
	if readErr != nil || !os.SameFile(currentInfo, originalInfo) || !bytes.Equal(current, plan.before) {
		return false, fail("source_changed")
	}
	if err = ctx.Err(); err != nil {
		return false, err
	}
	oldPath, replaceErr := replaceOwned(parent, opts.Root, temp, plan.Document)
	if replaceErr != nil {
		return false, fail("atomic_replace_unavailable")
	}
	committed = true
	cleanupPath = oldPath
	cleanupInfo = originalInfo
	rollback := func(cause error) error {
		now, _, check := readCurrent(root, plan.Document)
		if check != nil || !os.SameFile(now, tempInfo) {
			owned = false
			return errors.Join(cause, fail("rollback_refused"))
		}
		discardPath, swapErr := restoreOwned(parent, opts.Root, cleanupPath, plan.Document)
		if swapErr != nil {
			owned = false
			return errors.Join(cause, fail("rollback_failed"))
		}
		committed = false
		cleanupPath = discardPath
		cleanupInfo = tempInfo
		return cause
	}
	oldInfo, old, readErr := readCurrent(root, cleanupPath)
	if readErr != nil || !os.SameFile(oldInfo, originalInfo) || !bytes.Equal(old, plan.before) {
		return committed, rollback(fail("source_changed"))
	}
	if err = parent.Sync(); err != nil {
		return committed, rollback(fail("sync_failed"))
	}
	if err = opts.Validate(ctx); err != nil {
		return committed, rollback(err)
	}
	return true, nil
}

func readCurrent(root *os.Root, name string) (os.FileInfo, []byte, error) {
	info, err := root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 0 || info.Size() > MaxDocumentBytes {
		return nil, nil, fail("document_unavailable")
	}
	f, err := openNoFollow(root, name)
	if err != nil {
		return nil, nil, fail("document_unavailable")
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) || !opened.Mode().IsRegular() {
		return nil, nil, fail("source_changed")
	}
	b, err := io.ReadAll(io.LimitReader(f, MaxDocumentBytes+1))
	if err != nil || len(b) > MaxDocumentBytes {
		return nil, nil, fail("document_oversize")
	}
	after, err := f.Stat()
	if err != nil || !os.SameFile(info, after) || after.Size() != int64(len(b)) {
		return nil, nil, fail("source_changed")
	}
	return info, b, nil
}

func randomName() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable")
	}
	return hex.EncodeToString(b)
}
