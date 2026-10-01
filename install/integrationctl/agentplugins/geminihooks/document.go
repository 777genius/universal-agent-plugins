package geminihooks

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/tailscale/hujson"
)

// Selective adaptation of nativeconfig/document.go's private AST primitives.
// Sharing them publicly would expose the MCP codec's broader internals.
func parseSettings(body []byte) (hujson.Value, *hujson.Object, error) {
	if len(body) > MaxSettingsBytes {
		return hujson.Value{}, nil, fmt.Errorf("settings exceed size bound")
	}
	if len(body) == 0 {
		body = []byte("{}\n")
	}
	if err := preflightSettings(body); err != nil {
		return hujson.Value{}, nil, err
	}
	ast, err := hujson.Parse(body)
	if err != nil {
		return ast, nil, err
	}
	root, ok := ast.Value.(*hujson.Object)
	if !ok {
		return ast, nil, fmt.Errorf("settings must be an object")
	}
	// hujson accepts trailing commas, but native JSON.parse(stripJsonComments)
	// does not. A non-nil last AfterExtra is hujson's trailing-comma marker.
	for v := range ast.All() {
		switch n := v.Value.(type) {
		case *hujson.Object:
			if len(n.Members) > 0 && n.Members[len(n.Members)-1].Value.AfterExtra != nil {
				return ast, nil, fmt.Errorf("trailing comma")
			}
			seen := map[string]bool{}
			for _, m := range n.Members {
				key := nativeString(m.Name.Value.(hujson.Literal))
				if seen[key] {
					return ast, nil, fmt.Errorf("duplicate object key %s", m.Name.Value)
				}
				seen[key] = true
			}
		case *hujson.Array:
			if len(n.Elements) > 0 && n.Elements[len(n.Elements)-1].AfterExtra != nil {
				return ast, nil, fmt.Errorf("trailing comma")
			}
		}
	}
	return ast, root, nil
}

func member(obj *hujson.Object, key string) *hujson.ObjectMember {
	identity := nativeKey(key)
	for i := range obj.Members {
		if nativeString(obj.Members[i].Name.Value.(hujson.Literal)) == identity {
			return &obj.Members[i]
		}
	}
	return nil
}

func addMember(obj *hujson.Object, key string, v hujson.Value) {
	obj.Members = append(obj.Members, hujson.ObjectMember{Name: hujson.Value{Value: hujson.String(key)}, Value: v})
}

func hooksObject(root *hujson.Object, create bool) (*hujson.Object, error) {
	m := member(root, "hooks")
	if m == nil {
		if !create {
			return nil, nil
		}
		addMember(root, "hooks", hujson.Value{Value: &hujson.Object{}})
		m = member(root, "hooks")
	}
	obj, ok := m.Value.Value.(*hujson.Object)
	if !ok {
		return nil, fmt.Errorf("hooks must be an object")
	}
	return obj, nil
}

func groupDigest(event string, group hujson.Value) (string, error) {
	// encoding/json replaces lone UTF-16 surrogates (and invalid UTF-8) with
	// U+FFFD. Only owned groups need canonicalization; reject any lossy key or
	// value there. Foreign literals remain in the AST without Go re-encoding.
	for v := range group.All() {
		if s, ok := v.Value.(hujson.Literal); ok && s.Kind() == '"' {
			if !utf8.Valid(s) || nativeString(s) != nativeKey(s.String()) {
				return "", fmt.Errorf("owned group contains a lossy JSON string")
			}
		}
	}
	group = group.Clone()
	group.Standardize()
	var value any
	decoder := json.NewDecoder(bytes.NewReader(group.Pack()))
	decoder.UseNumber()
	// Input was parsed and duplicate-checked; canonical JSON sorts object keys.
	if err := decoder.Decode(&value); err != nil {
		return "", fmt.Errorf("decode owned group: %w", err)
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("canonicalize owned group: %w", err)
	}
	h := sha256.New()
	_, _ = h.Write([]byte("agentplugins-gemini-hooks-v1\x00" + event + "\x00"))
	_, _ = h.Write(canonical)
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func stringMember(obj *hujson.Object, key string) string {
	m := member(obj, key)
	if m != nil {
		if s, ok := m.Value.Value.(hujson.Literal); ok && s.Kind() == '"' {
			return nativeString(s)
		}
	}
	return ""
}

func nativeKey(s string) string { return nativeString(hujson.String(s)) }

// nativeString maps a hujson-validated JSON string to UTF-16 code-unit bytes.
// JSON.parse retains lone surrogates; Go strings cannot represent them as
// Unicode scalars. These private identities preserve that distinction and make
// paired escapes equal to literal Unicode, without changing any document bytes.
func nativeString(s hujson.Literal) string {
	units := make([]byte, 0, len(s))
	appendUnit := func(u uint16) { units = binary.BigEndian.AppendUint16(units, u) }
	for i := 1; i < len(s)-1; {
		r, size := utf8.DecodeRune(s[i:])
		i += size
		if r == '\\' {
			r = rune(s[i])
			i++
			switch r {
			case 'u':
				var u uint16
				for _, c := range s[i : i+4] {
					u <<= 4
					switch {
					case c >= '0' && c <= '9':
						u |= uint16(c - '0')
					case c >= 'a' && c <= 'f':
						u |= uint16(c - 'a' + 10)
					default:
						u |= uint16(c - 'A' + 10)
					}
				}
				i += 4
				appendUnit(u)
				continue
			case 'b':
				r = '\b'
			case 'f':
				r = '\f'
			case 'n':
				r = '\n'
			case 'r':
				r = '\r'
			case 't':
				r = '\t'
			}
		}
		// DecodeRune returns Unicode scalars; EncodeRune returns two 16-bit
		// surrogates. Masks make those bounds explicit without altering units.
		if r > 0xffff {
			hi, lo := utf16.EncodeRune(r)
			appendUnit(uint16(hi & 0xffff))
			appendUnit(uint16(lo & 0xffff))
		} else {
			appendUnit(uint16(r & 0xffff))
		}
	}
	return string(units)
}
