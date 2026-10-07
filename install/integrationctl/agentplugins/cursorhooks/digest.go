package cursorhooks

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"hash"
	"sort"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/tailscale/hujson"
)

// Canonical structural hashing sorts object keys, retains array order and
// exact number tokens, and identifies JSON strings by native UTF-16 units.
// It never rounds opaque numbers or merges lone surrogates with U+FFFD.
// Unknown fields participate fully. Whitespace and string escape spelling
// alone do not break ownership; number lexeme changes deliberately do.
func valueDigest(domain string, v hujson.Value) string {
	h := sha256.New()
	frame(h, []byte("agentplugins-cursor-hooks-v1:"+domain))
	hashValue(h, v)
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func frame(h hash.Hash, b []byte) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(b)))
	_, _ = h.Write(size[:])
	_, _ = h.Write(b)
}

func hashValue(h hash.Hash, v hujson.Value) {
	switch n := v.Value.(type) {
	case *hujson.Object:
		frame(h, []byte("object"))
		members := append([]hujson.ObjectMember(nil), n.Members...)
		sort.Slice(members, func(i, j int) bool {
			return stringIdentity(members[i].Name.Value.(hujson.Literal)) <
				stringIdentity(members[j].Name.Value.(hujson.Literal))
		})
		for _, m := range members {
			frame(h, []byte(stringIdentity(m.Name.Value.(hujson.Literal))))
			hashValue(h, m.Value)
		}
		frame(h, []byte("end-object"))
	case *hujson.Array:
		frame(h, []byte("array"))
		for _, element := range n.Elements {
			hashValue(h, element)
		}
		frame(h, []byte("end-array"))
	case hujson.Literal:
		frame(h, []byte{byte(n.Kind())})
		if n.Kind() == '"' {
			frame(h, []byte(stringIdentity(n)))
		} else {
			frame(h, n)
		}
	}
}

func remainderDigest(d *document, owned *location) string {
	clone := d.ast.Clone()
	root := clone.Value.(*hujson.Object)
	if m := member(root, "hooks"); m != nil {
		hooks := m.Value.Value.(*hujson.Object)
		if stop := member(hooks, "stop"); stop != nil {
			arr := stop.Value.Value.(*hujson.Array)
			if owned != nil {
				arr.Elements = append(arr.Elements[:owned.index], arr.Elements[owned.index+1:]...)
			}
			// An absent managed container and its empty skeleton express the
			// same remainder. No other event or empty foreign object is pruned.
			if len(arr.Elements) == 0 {
				deleteMember(hooks, "stop")
			}
		}
		if len(hooks.Members) == 0 {
			deleteMember(root, "hooks")
		}
	}
	return valueDigest("remainder", clone)
}

// Adapted from the private Gemini/nativeconfig AST boundary: JSON.parse
// retains UTF-16 surrogates while Go unmarshal replaces unpaired units. Inputs
// here have already passed strict JSON and UTF-8 validation.
func stringIdentity(s hujson.Literal) string {
	units := make([]byte, 0, len(s))
	add := func(u uint16) { units = binary.BigEndian.AppendUint16(units, u) }
	for i := 1; i < len(s)-1; {
		r, size := utf8.DecodeRune(s[i:])
		i += size
		if r == '\\' {
			r = rune(s[i])
			i++
			if r == 'u' {
				var u uint16
				for _, c := range s[i : i+4] {
					u = u<<4 | hexDigit(c)
				}
				i += 4
				add(u)
				continue
			}
			switch r {
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
		if r > 0xffff {
			for _, u := range utf16.Encode([]rune{r}) {
				add(u)
			}
		} else {
			add(uint16(r))
		}
	}
	return string(units)
}

func hexDigit(c byte) uint16 {
	if c <= '9' {
		return uint16(c - '0')
	}
	if c <= 'F' {
		return uint16(c - 'A' + 10)
	}
	return uint16(c - 'a' + 10)
}
