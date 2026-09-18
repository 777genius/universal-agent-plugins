package nativeconfig

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tailscale/hujson"
)

type document struct {
	ast   hujson.Value
	root  *hujson.Object
	jsonc bool
}

func parseDocument(body []byte, jsonc bool) (*document, error) {
	if !jsonc && !json.Valid(body) {
		return nil, fmt.Errorf("%w: invalid JSON", ErrMalformed)
	}
	ast, err := hujson.Parse(body)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	root, ok := ast.Value.(*hujson.Object)
	if !ok {
		return nil, fmt.Errorf("%w: top level must be an object", ErrMalformed)
	}
	if err := rejectDuplicateKeys(&ast, "$"); err != nil {
		return nil, err
	}
	return &document{ast: ast, root: root, jsonc: jsonc}, nil
}

func newDocument(jsonc bool) (*document, error) { return parseDocument([]byte("{}\n"), jsonc) }

func rejectDuplicateKeys(value *hujson.Value, path string) error {
	switch node := value.Value.(type) {
	case *hujson.Object:
		seen := make(map[string]struct{}, len(node.Members))
		for i := range node.Members {
			name, ok := node.Members[i].Name.Value.(hujson.Literal)
			if !ok {
				return fmt.Errorf("%w: non-string object key at %s", ErrMalformed, path)
			}
			key := name.String()
			if _, exists := seen[key]; exists {
				return fmt.Errorf("%w: duplicate key %q at %s", ErrMalformed, key, path)
			}
			seen[key] = struct{}{}
			if err := rejectDuplicateKeys(&node.Members[i].Value, path+"."+key); err != nil {
				return err
			}
		}
	case *hujson.Array:
		for i := range node.Elements {
			if err := rejectDuplicateKeys(&node.Elements[i], fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}

func objectMember(obj *hujson.Object, key string) (*hujson.ObjectMember, int) {
	for i := range obj.Members {
		if obj.Members[i].Name.Value.(hujson.Literal).String() == key {
			return &obj.Members[i], i
		}
	}
	return nil, -1
}

func collection(doc *document, key string, create bool) (*hujson.Object, error) {
	member, _ := objectMember(doc.root, key)
	if member != nil {
		obj, ok := member.Value.Value.(*hujson.Object)
		if !ok {
			return nil, fmt.Errorf("%w: %s must be an object", ErrMalformed, key)
		}
		return obj, nil
	}
	if !create {
		return nil, nil
	}
	value, err := jsonValue(map[string]any{})
	if err != nil {
		return nil, err
	}
	appendMember(doc.root, key, value)
	member, _ = objectMember(doc.root, key)
	return member.Value.Value.(*hujson.Object), nil
}

func jsonValue(value any) (hujson.Value, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return hujson.Value{}, err
	}
	return hujson.Parse(body)
}

func appendMember(obj *hujson.Object, key string, value hujson.Value) {
	name := hujson.Value{Value: hujson.String(key)}
	name.BeforeExtra = []byte("\n  ")
	obj.Members = append(obj.Members, hujson.ObjectMember{Name: name, Value: value})
	if len(obj.AfterExtra) == 0 {
		obj.AfterExtra = []byte("\n")
	}
}

func setEntry(obj *hujson.Object, name string, value hujson.Value) bool {
	member, _ := objectMember(obj, name)
	if member == nil {
		appendMember(obj, name, value)
		return false
	}
	value.BeforeExtra = member.Value.BeforeExtra
	value.AfterExtra = member.Value.AfterExtra
	member.Value = value
	return true
}

func removeEntry(obj *hujson.Object, name string) bool {
	_, index := objectMember(obj, name)
	if index < 0 {
		return false
	}
	obj.Members = append(obj.Members[:index], obj.Members[index+1:]...)
	return true
}

func (doc *document) render() ([]byte, error) {
	body := doc.ast.Pack()
	if !doc.jsonc && !json.Valid(body) {
		return nil, fmt.Errorf("rendered native config is not strict JSON")
	}
	if _, err := parseDocument(body, doc.jsonc); err != nil {
		return nil, fmt.Errorf("validate rendered native config: %w", err)
	}
	return body, nil
}

func entryCanonical(member *hujson.ObjectMember) ([]byte, error) {
	standard, err := hujson.Standardize(member.Value.Pack())
	if err != nil {
		return nil, err
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(standard))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

func entryDigest(codec Codec, name string, member *hujson.ObjectMember) (string, error) {
	canonical, err := entryCanonical(member)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	_, _ = hash.Write([]byte("agentplugins-native-mcp-v1\x00"))
	_, _ = hash.Write([]byte(codec))
	_, _ = hash.Write([]byte{'\x00'})
	_, _ = hash.Write([]byte(name))
	_, _ = hash.Write([]byte{'\x00'})
	_, _ = hash.Write(canonical)
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func verifyReceipt(receipt *Receipt, path string, codec Codec, name string, member *hujson.ObjectMember) error {
	if receipt == nil || receipt.Version != "1" || receipt.Path != path || receipt.Codec != codec || receipt.Name != name {
		return ErrNotOwned
	}
	digest, err := entryDigest(codec, name, member)
	if err != nil {
		return err
	}
	if !strings.EqualFold(receipt.Digest, digest) {
		return ErrNotOwned
	}
	return nil
}
