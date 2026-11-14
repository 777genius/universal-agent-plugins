package cursorhooks

import (
	"encoding/json"
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/tailscale/hujson"
)

type document struct {
	ast   hujson.Value
	root  *hujson.Object
	hooks *hujson.Object
}

// Selective adaptation of nativeconfig's private hujson AST operations. It
// exposes no general config framework, MCP codec or second parser engine.
func parseDocument(body []byte) (*document, error) {
	if len(body) == 0 {
		body = []byte(`{"version":1,"hooks":{}}`)
	}
	if err := preflight(body); err != nil {
		return nil, err
	}
	if !utf8.Valid(body) || !json.Valid(body) {
		return nil, fmt.Errorf("document requires strict UTF-8 JSON")
	}
	ast, err := hujson.Parse(body)
	if err != nil {
		return nil, fmt.Errorf("invalid JSON AST")
	}
	if err := uniqueKeys(&ast); err != nil {
		return nil, err
	}
	root, ok := ast.Value.(*hujson.Object)
	if !ok {
		return nil, fmt.Errorf("document must be an object")
	}
	version := member(root, "version")
	if version == nil || !nativeVersionOne(version.Value) {
		return nil, fmt.Errorf("document requires version 1")
	}
	d := &document{ast: ast, root: root}
	if m := member(root, "hooks"); m != nil {
		d.hooks, ok = m.Value.Value.(*hujson.Object)
		if !ok {
			return nil, fmt.Errorf("hooks must be an object")
		}
		for _, event := range d.hooks.Members {
			if _, ok := event.Value.Value.(*hujson.Array); !ok {
				return nil, fmt.Errorf("hook events must be flat arrays")
			}
		}
	}
	return d, nil
}

func nativeVersionOne(v hujson.Value) bool {
	n, ok := v.Value.(hujson.Literal)
	if !ok || n.Kind() != '0' {
		return false
	}
	// Only the version selector uses native JSON's binary64 number semantics.
	// Foreign numbers and all digest number tokens remain exact and opaque.
	version, err := strconv.ParseFloat(string(n), 64)
	return err == nil && version == 1
}

func uniqueKeys(ast *hujson.Value) error {
	for v := range ast.All() {
		if obj, ok := v.Value.(*hujson.Object); ok {
			seen := make(map[string]bool, len(obj.Members))
			for _, m := range obj.Members {
				key := stringIdentity(m.Name.Value.(hujson.Literal))
				if seen[key] {
					return fmt.Errorf("duplicate object key")
				}
				seen[key] = true
			}
		}
	}
	return nil
}

func member(obj *hujson.Object, key string) *hujson.ObjectMember {
	if obj == nil {
		return nil
	}
	id := stringIdentity(hujson.String(key))
	for i := range obj.Members {
		if stringIdentity(obj.Members[i].Name.Value.(hujson.Literal)) == id {
			return &obj.Members[i]
		}
	}
	return nil
}

func addMember(obj *hujson.Object, key string, v hujson.Value) {
	obj.Members = append(obj.Members, hujson.ObjectMember{
		Name: hujson.Value{Value: hujson.String(key)}, Value: v,
	})
}

func deleteMember(obj *hujson.Object, key string) {
	m := member(obj, key)
	for i := range obj.Members {
		if &obj.Members[i] == m {
			obj.Members = append(obj.Members[:i], obj.Members[i+1:]...)
			return
		}
	}
}

func (d *document) stopArray() *hujson.Array {
	m := member(d.hooks, "stop")
	if m == nil {
		return nil
	}
	return m.Value.Value.(*hujson.Array)
}

func (d *document) appendStop(entry hujson.Value) {
	if d.hooks == nil {
		d.hooks = &hujson.Object{}
		addMember(d.root, "hooks", hujson.Value{Value: d.hooks})
	}
	arr := d.stopArray()
	if arr == nil {
		arr = &hujson.Array{}
		addMember(d.hooks, "stop", hujson.Value{Value: arr})
	}
	arr.Elements = append(arr.Elements, entry)
}

func (d *document) render() ([]byte, error) {
	body := d.ast.Pack()
	if _, err := parseDocument(body); err != nil {
		return nil, fmt.Errorf("planned document exceeds bounds or grammar")
	}
	return body, nil
}
