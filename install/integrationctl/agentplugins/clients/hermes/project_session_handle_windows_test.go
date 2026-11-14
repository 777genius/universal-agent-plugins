package hermes

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestProjectSessionWindowsOwnerDecoderStaysWithinReturnedBuffer(t *testing.T) {
	const header = strconv.IntSize / 8
	for _, change := range []string{"none", "null", "prefix", "end", "truncated", "revision", "count", "wide-authority"} {
		t.Run(change, func(t *testing.T) {
			body := make([]byte, header+16)
			var pinned runtime.Pinner
			pinned.Pin(&body[0])
			defer pinned.Unpin()
			base := uint64(reflect.ValueOf(body).Pointer())
			selected := base + header
			sid := body[header:]
			sid[0], sid[1], sid[7] = 1, 2, 5
			binary.LittleEndian.PutUint32(sid[8:], 32)
			binary.LittleEndian.PutUint32(sid[12:], 544)
			switch change {
			case "null":
				selected = 0
			case "prefix":
				selected--
			case "end":
				selected = base + uint64(len(body)) - 7
			case "truncated":
				body = body[:len(body)-1]
			case "revision":
				sid[0] = 2
			case "count":
				sid[1] = 16
			case "wide-authority":
				copy(sid[2:8], []byte{1, 2, 3, 4, 5, 6})
			}
			if header == 8 {
				binary.LittleEndian.PutUint64(body[:header], selected)
			} else {
				binary.LittleEndian.PutUint32(body[:header], uint32(selected))
			}
			owner, err := sessionTokenOwnerSID(body)
			valid := change == "none" || change == "wide-authority"
			if valid != (err == nil) {
				t.Fatalf("invalid decoder outcome: %v", err)
			}
			if valid && !owner.IsValid() {
				t.Fatal("decoder returned an invalid owner")
			}
			if valid {
				var authority [6]byte
				copy(authority[:], sid[2:8])
				assertExact := func() {
					t.Helper()
					if !owner.IsValid() || owner.Len() != 16 || owner.IdentifierAuthority().Value != authority ||
						owner.SubAuthorityCount() != 2 || owner.SubAuthority(0) != 32 || owner.SubAuthority(1) != 544 {
						t.Fatal("decoder changed the exact binary owner")
					}
				}
				assertExact()
				sid[0], sid[7] = 2, 0
				binary.LittleEndian.PutUint32(sid[8:], 99)
				assertExact()
			}
		})
	}
}

func TestProjectSessionWindowsTokenOwnerDiffersFromTokenUser(t *testing.T) {
	user, err := windows.StringToSid("S-1-5-21-1-2-3-1001")
	if err != nil {
		t.Fatal(err)
	}
	owner, err := windows.StringToSid("S-1-5-32-544")
	if err != nil {
		t.Fatal(err)
	}
	queries := 0
	selected, err := sessionTokenDefaultOwner(0, func(_ windows.Token, class uint32, output *byte, capacity uint32, size *uint32) error {
		queries++
		if class != windows.TokenOwner {
			t.Fatalf("queried token class %d instead of the default object owner", class)
		}
		header := int(unsafe.Sizeof(uintptr(0)))
		*size = uint32(header + owner.Len())
		if output == nil {
			return windows.ERROR_INSUFFICIENT_BUFFER
		}
		if capacity < *size {
			t.Fatal("token owner query buffer is too small")
		}
		body := unsafe.Slice(output, capacity)
		copy(body[header:], unsafe.Slice((*byte)(unsafe.Pointer(owner)), owner.Len()))
		*(**windows.SID)(unsafe.Pointer(output)) = (*windows.SID)(unsafe.Pointer(&body[header]))
		return nil
	})
	if err != nil || queries != 2 || !selected.Equals(owner) || selected.Equals(user) {
		t.Fatalf("selected TokenUser or another group instead of exact TokenOwner: %v", err)
	}
}

func TestProjectSessionWindowsFreshHeldObjectsUseTokenOwner(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	if err := root.Mkdir("selected", 0o700); err != nil {
		t.Fatal(err)
	}
	directory, err := root.Open("selected")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = directory.Close() }()
	info, err := directory.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if err := validateSessionDirectoryHandle(directory, info); err != nil {
		t.Fatal(err)
	}
	file, err := root.OpenFile(filepath.Join("selected", ".uap-owner"), os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	info, err = file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if err := validateSessionHandle(file, info); err != nil {
		t.Fatal(err)
	}
}

func TestProjectSessionWindowsOwnerDecoderPreservesNumericComponents(t *testing.T) {
	const header = strconv.IntSize / 8
	for _, test := range []struct {
		name      string
		authority [6]byte
		sub       []uint32
	}{
		{"authority-zero", [6]byte{}, []uint32{0}},
		{"authority-decimal-max", [6]byte{0, 0, 255, 255, 255, 255}, []uint32{^uint32(0)}},
		{"authority-hex-min", [6]byte{0, 1, 0, 0, 0, 0}, []uint32{0, ^uint32(0)}},
		{"authority-max", [6]byte{255, 255, 255, 255, 255, 255}, []uint32{1, ^uint32(0)}},
		{"subauthority-count-max", [6]byte{0, 0, 0, 0, 0, 5}, []uint32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, ^uint32(0)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := make([]byte, header+8+4*len(test.sub))
			var pinned runtime.Pinner
			pinned.Pin(&body[0])
			defer pinned.Unpin()
			selected := uint64(reflect.ValueOf(body).Pointer()) + header
			if header == 8 {
				binary.LittleEndian.PutUint64(body[:header], selected)
			} else {
				binary.LittleEndian.PutUint32(body[:header], uint32(selected))
			}
			sid := body[header:]
			sid[0], sid[1] = 1, byte(len(test.sub))
			copy(sid[2:8], test.authority[:])
			for index, value := range test.sub {
				binary.LittleEndian.PutUint32(sid[8+4*index:], value)
			}
			owner, err := sessionTokenOwnerSID(body)
			if err != nil || owner == nil || !owner.IsValid() || owner.Len() != len(sid) {
				t.Fatalf("numeric owner did not round-trip: %v", err)
			}
			if owner.IdentifierAuthority().Value != test.authority || int(owner.SubAuthorityCount()) != len(test.sub) {
				t.Fatal("numeric authority or count changed")
			}
			for index, want := range test.sub {
				if owner.SubAuthority(uint32(index)) != want {
					t.Fatalf("sub-authority %d changed", index)
				}
			}
		})
	}
}

func TestProjectSessionWindowsTokenOwnerRejectsUnreturnedSubauthority(t *testing.T) {
	const header = strconv.IntSize / 8
	queries := 0
	_, err := sessionTokenDefaultOwner(0, func(_ windows.Token, class uint32, output *byte, capacity uint32, size *uint32) error {
		queries++
		if class != windows.TokenOwner {
			t.Fatalf("queried token class %d", class)
		}
		*size = uint32(header + 12)
		if output == nil {
			return windows.ERROR_INSUFFICIENT_BUFFER
		}
		if capacity != *size {
			t.Fatal("unexpected allocated extent")
		}
		body := unsafe.Slice(output, capacity)
		selected := uint64(reflect.ValueOf(body).Pointer()) + header
		if header == 8 {
			binary.LittleEndian.PutUint64(body[:header], selected)
		} else {
			binary.LittleEndian.PutUint32(body[:header], uint32(selected))
		}
		sid := body[header:]
		sid[0], sid[1], sid[7] = 1, 1, 5
		binary.LittleEndian.PutUint32(sid[8:], 32)
		*size = uint32(header + 8)
		return nil
	})
	if err == nil || queries != 2 {
		t.Fatalf("adopted allocated bytes outside returned extent: %v", err)
	}
}
