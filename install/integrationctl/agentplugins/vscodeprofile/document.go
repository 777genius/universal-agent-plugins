package vscodeprofile

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/tailscale/hujson"
)

// Selective adaptation of nativeconfig's private AST helpers; the MCP codec is
// intentionally not used for this distinct native setting. No DTO roundtrip.
type document struct {
	ast       hujson.Value
	root      *hujson.Object
	locations *hujson.Object
}

func parseSettings(body []byte, pluginRoot string) (*document, error) {
	if err := preflight(body); err != nil {
		return nil, err
	}
	if len(body) == 0 {
		body = []byte("{}")
	}
	ast, err := hujson.Parse(body)
	if err != nil {
		return nil, fmt.Errorf("malformed JSONC")
	}
	root, ok := ast.Value.(*hujson.Object)
	if !ok {
		return nil, fmt.Errorf("settings must be an object")
	}
	d := &document{ast: ast, root: root}
	found := false
	for _, m := range root.Members {
		// Unknown keys/values remain opaque, including duplicate foreign keys.
		if m.Name.Value.(hujson.Literal).String() != selector {
			continue
		}
		if found {
			return nil, fmt.Errorf("duplicate pluginLocations selector")
		}
		found = true
		d.locations, ok = m.Value.Value.(*hujson.Object)
		if !ok {
			return nil, fmt.Errorf("pluginLocations must be an object")
		}
	}
	if err := validateLocations(d.locations, pluginRoot); err != nil {
		return nil, err
	}
	return d, nil
}

func validateLocations(obj *hujson.Object, pluginRoot string) error {
	if obj == nil {
		return nil
	}
	seen := map[string]bool{}
	for _, m := range obj.Members {
		literal := m.Name.Value.(hujson.Literal)
		if !losslessKey(literal) {
			return fmt.Errorf("ambiguous plugin location Unicode")
		}
		key := literal.String()
		if seen[key] {
			return fmt.Errorf("duplicate plugin location")
		}
		seen[key] = true
		if key != pluginRoot && aliasesRoot(key, pluginRoot) {
			return fmt.Errorf("plugin location alias collision")
		}
	}
	return nil
}

// Go decoding replaces lone UTF-16 surrogates; native JS retains them. Refuse
// that ambiguity only in the edited shape, preserving opaque foreign literals.
func losslessKey(s hujson.Literal) bool {
	for i := 1; i < len(s)-1; i++ {
		if s[i] != '\\' {
			continue
		}
		i++
		if s[i] != 'u' {
			continue
		}
		u, _ := strconv.ParseUint(string(s[i+1:i+5]), 16, 16)
		i += 4
		if !utf16.IsSurrogate(rune(u)) {
			continue
		}
		if u > 0xdbff || i+6 >= len(s) || s[i+1] != '\\' || s[i+2] != 'u' {
			return false
		}
		low, _ := strconv.ParseUint(string(s[i+3:i+7]), 16, 16)
		if low < 0xdc00 || low > 0xdfff {
			return false
		}
		i += 6
	}
	return true
}

func aliasesRoot(key, root string) bool {
	key = nativeTrim(key)
	if !strings.HasPrefix(root, "/") {
		// Windows resolves either separator. Normalize only collision comparison;
		// explicit identities, receipts and map lookup retain their exact keys.
		key = strings.ReplaceAll(key, "/", "\\")
		// fileURI also resolves a leading-separator drive URI path to the
		// drive root. Strip that one separator only for foreign comparison;
		// absolutePath still validates the drive letter and rooted suffix.
		if len(key) >= 4 && key[0] == '\\' && key[2] == ':' && key[3] == '\\' {
			key = key[1:]
		}
	}
	normalized, ok := absolutePath(key)
	if !ok {
		return false
	}
	if strings.HasPrefix(root, "/") {
		return normalized == root
	}
	return strings.EqualFold(normalized, root)
}

func member(obj *hujson.Object, key string) (*hujson.ObjectMember, int) {
	if obj != nil {
		for i := range obj.Members {
			if obj.Members[i].Name.Value.(hujson.Literal).String() == key {
				return &obj.Members[i], i
			}
		}
	}
	return nil, -1
}

func (d *document) current(root string) (bool, bool, error) {
	m, _ := member(d.locations, root)
	if m == nil {
		return false, false, nil
	}
	v, ok := m.Value.Value.(hujson.Literal)
	if !ok || (v.Kind() != 't' && v.Kind() != 'f') {
		return false, true, fmt.Errorf("owned location is not boolean")
	}
	return v.Kind() == 't', true, nil
}

func appendMember(obj *hujson.Object, key string, v hujson.Value) {
	obj.Members = append(obj.Members, hujson.ObjectMember{Name: hujson.Value{Value: hujson.String(key)}, Value: v})
}

func (d *document) set(root string, enabled bool) {
	if d.locations == nil {
		d.locations = &hujson.Object{}
		appendMember(d.root, selector, hujson.Value{Value: d.locations})
	}
	m, _ := member(d.locations, root)
	literal := hujson.Bool(enabled)
	if m != nil {
		m.Value.Value = literal
		return
	}
	appendMember(d.locations, root, hujson.Value{Value: literal})
}

func (d *document) remove(root string) {
	_, i := member(d.locations, root)
	m := d.locations.Members[i]
	// Keep all trivia, including comments adjacent to the removed selector.
	extra := append(hujson.Extra{}, m.Name.BeforeExtra...)
	extra = append(extra, m.Name.AfterExtra...)
	extra = append(extra, m.Value.BeforeExtra...)
	extra = append(extra, m.Value.AfterExtra...)
	d.locations.Members = append(d.locations.Members[:i], d.locations.Members[i+1:]...)
	if i < len(d.locations.Members) {
		next := &d.locations.Members[i].Name
		next.BeforeExtra = append(extra, next.BeforeExtra...)
	} else {
		d.locations.AfterExtra = append(extra, d.locations.AfterExtra...)
	}
}
