//go:build windows

package atomicfile

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"golang.org/x/sys/windows"
)

func TestAdmittedWindowsReaderPreservesDeleteSharing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metadata.json")
	if err := Write(path, []byte("before"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := openRegularNoFollow(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	if err := Write(path, []byte("after"), 0o600); err != nil {
		t.Fatalf("admitted reader blocked atomic publication: %v", err)
	}
	body, err := io.ReadAll(file)
	if err != nil || string(body) != "before" {
		t.Fatalf("reader lost held file: %q, %v", body, err)
	}
	if body, err := os.ReadFile(path); err != nil || string(body) != "after" {
		t.Fatalf("replacement missing: %q, %v", body, err)
	}
}

func TestAdmittedWindowsReaderUnicodeLongPath(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	parent := filepath.Join(strings.Repeat("long-parent-", 10), strings.Repeat("second-parent-", 9), "unicode-é-插件")
	if err := os.MkdirAll(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "状態.json")
	absolute, err := filepath.Abs(path)
	if err != nil || len(absolute) <= 260 {
		t.Fatal("fixture did not establish a long absolute path")
	}
	if err := Write(path, []byte("before"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{path, absolute} {
		if body, err := ReadBoundedRegularFile(name, 16); err != nil || string(body) != "before" {
			t.Fatalf("bounded long-path read: %q, %v", body, err)
		}
	}
	file, err := openRegularNoFollow(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	if err := Write(path, []byte("after"), 0o600); err != nil {
		t.Fatalf("long-path reader blocked atomic publication: %v", err)
	}
	if body, err := io.ReadAll(file); err != nil || string(body) != "before" {
		t.Fatalf("long-path reader lost held bytes: %q, %v", body, err)
	}
}

func TestWindowsRegularReadPathPreservesNamespaces(t *testing.T) {
	tail := strings.Repeat(`directory\`, 35) + "metadata.json"
	for _, tc := range []struct{ input, want string }{
		{`C:\` + tail, `\\?\C:\` + tail},
		{`\\server\share\` + tail, `\\?\UNC\server\share\` + tail},
		{`\\?\C:\` + tail, `\\?\C:\` + tail},
		{`\\?\UNC\server\share\` + tail, `\\?\UNC\server\share\` + tail},
		{`\??\C:\` + tail, `\??\C:\` + tail},
		{`\\.\C:\` + tail, `\\.\C:\` + tail},
	} {
		got, err := windowsRegularReadPath(tc.input)
		if err != nil || got != tc.want {
			t.Errorf("path %q: got %q, %v; want %q", tc.input, got, err, tc.want)
		}
	}
}

func TestWindowsRegularReadDedupAndUnsafeAttributes(t *testing.T) {
	const dedup = 0x80000013
	for _, tc := range []struct {
		name       string
		attributes uint32
		tag        uint32
		want       error
	}{
		{"regular", windows.FILE_ATTRIBUTE_NORMAL, 0, nil},
		{"dedup", windows.FILE_ATTRIBUTE_REPARSE_POINT, dedup, nil},
		{"symlink", windows.FILE_ATTRIBUTE_REPARSE_POINT, windows.IO_REPARSE_TAG_SYMLINK, ErrUnsafeFile},
		{"junction", windows.FILE_ATTRIBUTE_REPARSE_POINT, windows.IO_REPARSE_TAG_MOUNT_POINT, ErrUnsafeFile},
		{"unknown-reparse", windows.FILE_ATTRIBUTE_REPARSE_POINT, 0x80000000, ErrUnsafeFile},
		{"directory", windows.FILE_ATTRIBUTE_DIRECTORY, 0, ErrUnsafeFile},
		{"device", windows.FILE_ATTRIBUTE_DEVICE, 0, ErrUnsafeFile},
		{"offline-regular", windows.FILE_ATTRIBUTE_OFFLINE, 0, windows.ERROR_FILE_OFFLINE},
		{"offline-dedup", windows.FILE_ATTRIBUTE_REPARSE_POINT | windows.FILE_ATTRIBUTE_OFFLINE, dedup, windows.ERROR_FILE_OFFLINE},
		{"recall-on-open", windows.FILE_ATTRIBUTE_RECALL_ON_OPEN, 0, windows.ERROR_FILE_OFFLINE},
		{"recall-on-read", windows.FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS, 0, windows.ERROR_FILE_OFFLINE},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := admitWindowsRegularRead("metadata.json", tc.attributes, tc.tag)
			if !errors.Is(err, tc.want) {
				t.Fatalf("admission=%v want=%v", err, tc.want)
			}
			if tc.want == windows.ERROR_FILE_OFFLINE {
				var pathError *os.PathError
				if !errors.As(err, &pathError) || pathError.Path != "metadata.json" || errors.Is(err, ErrUnsafeFile) {
					t.Fatalf("availability failure lost operational identity: %v", err)
				}
			}
		})
	}
}

type windowsReadFileInfo struct {
	os.FileInfo
	mode       os.FileMode
	attributes uint32
}

func (info windowsReadFileInfo) Mode() os.FileMode { return info.mode }
func (info windowsReadFileInfo) Sys() any {
	return &syscall.Win32FileAttributeData{FileAttributes: info.attributes}
}

func TestWindowsRegularReadAvailabilityBeforeTypeAdmission(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metadata.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	base, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name       string
		mode       os.FileMode
		attributes uint32
		want       error
	}{
		{"regular", 0o600, windows.FILE_ATTRIBUTE_NORMAL, nil},
		{"offline-regular", 0o600, windows.FILE_ATTRIBUTE_OFFLINE, windows.ERROR_FILE_OFFLINE},
		{"offline-placeholder", os.ModeIrregular, windows.FILE_ATTRIBUTE_REPARSE_POINT | windows.FILE_ATTRIBUTE_OFFLINE, windows.ERROR_FILE_OFFLINE},
		{"recall-on-open-placeholder", os.ModeIrregular, windows.FILE_ATTRIBUTE_REPARSE_POINT | windows.FILE_ATTRIBUTE_RECALL_ON_OPEN, windows.ERROR_FILE_OFFLINE},
		{"recall-on-read-placeholder", os.ModeIrregular, windows.FILE_ATTRIBUTE_REPARSE_POINT | windows.FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS, windows.ERROR_FILE_OFFLINE},
		{"available-irregular", os.ModeIrregular, windows.FILE_ATTRIBUTE_REPARSE_POINT, ErrUnsafeFile},
		{"symlink", os.ModeSymlink, windows.FILE_ATTRIBUTE_REPARSE_POINT, ErrUnsafeFile},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// This is the shared pre-open admission, using the FileInfo shape Go
			// returns for cloud reparse points. The handle helper alone misses it.
			info := windowsReadFileInfo{FileInfo: base, mode: tc.mode, attributes: tc.attributes}
			err := admitRegularRead(info, 16)
			if !errors.Is(err, tc.want) {
				t.Fatalf("pre-open admission=%v want=%v", err, tc.want)
			}
			if tc.want == windows.ERROR_FILE_OFFLINE {
				var pathError *os.PathError
				if !errors.As(err, &pathError) || errors.Is(err, ErrUnsafeFile) {
					t.Fatalf("unavailable placeholder became repairable corruption: %v", err)
				}
			}
		})
	}
}
