package nativeconfig

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type codexParserTestIO struct {
	body          []byte
	reads, writes int
}

func (files *codexParserTestIO) ReadNoFollow(string) ([]byte, os.FileMode, bool, error) {
	files.reads++
	return files.body, 0600, true, nil
}
func (files *codexParserTestIO) WriteAtomic(_ string, body []byte, _ os.FileMode) error {
	files.writes++
	files.body = bytes.Clone(body)
	return nil
}
func (*codexParserTestIO) RemoveNoFollow(string) error { return nil }

type codexParserTestDocument struct{ entries map[string]json.RawMessage }

func (doc *codexParserTestDocument) MCPEntries() (map[string]json.RawMessage, error) {
	return doc.entries, nil
}
func (*codexParserTestDocument) RenderMCPEntries(entries map[string]json.RawMessage) ([]byte, error) {
	return json.Marshal(entries)
}

func codexParserRequest(t *testing.T) Request {
	t.Helper()
	return Request{Paths: Paths{JSON: filepath.Join(t.TempDir(), "config.toml")}, Codec: CodecCodex,
		Action: ActionAdd, Name: "managed", Server: Server{Type: "remote", URL: "https://inert.test/mcp"}}
}

func TestProjectCodexMissingParserRefusesBeforeIOOrLocks(t *testing.T) {
	files := &codexParserTestIO{body: []byte("original")}
	locks := 0
	kernel := NewWithFileIO(files)
	kernel.acquireLocks = func(Paths, Codec) (func() error, error) { locks++; return func() error { return nil }, nil }
	request := codexParserRequest(t)
	if _, err := kernel.Apply(request); err == nil {
		t.Fatal("missing parser admitted mutation")
	}
	if _, _, err := kernel.Inspect(request.Paths, request.Codec, request.Name, nil); err == nil {
		t.Fatal("missing parser admitted inspection")
	}
	if _, err := DesiredReceipt(request.Paths.JSON, request.Codec, request.Name, request.Server, Placeholders{}); err != nil {
		t.Fatal("pure receipt gained parser requirement", err)
	}
	if files.reads != 0 || files.writes != 0 || locks != 0 {
		t.Fatalf("missing parser effects reads=%d writes=%d locks=%d", files.reads, files.writes, locks)
	}
}

func TestProjectCodexParserCopyPreservesInjectedAuthority(t *testing.T) {
	files := &codexParserTestIO{body: []byte("original")}
	locks, releases := 0, 0
	original := NewWithFileIO(files)
	original.acquireLocks = func(Paths, Codec) (func() error, error) { locks++; return func() error { releases++; return nil }, nil }
	kernel := original.WithCodexDocumentParser(func([]byte) (CodexDocument, error) {
		return &codexParserTestDocument{entries: map[string]json.RawMessage{}}, nil
	})
	if _, err := kernel.Apply(codexParserRequest(t)); err != nil {
		t.Fatal(err)
	}
	if original.codexParser != nil || kernel.files != files || files.reads == 0 || files.writes != 1 || locks != 1 || releases != 1 {
		t.Fatalf("configured copy lost authority reads=%d writes=%d locks=%d releases=%d", files.reads, files.writes, locks, releases)
	}
}

func TestProjectCodexNilDocumentsCannotMutateSnapshotsOrLeakLocks(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "literal", true: "typed"}[typed], func(t *testing.T) {
			files := &codexParserTestIO{body: []byte("original snapshot")}
			locks, releases := 0, 0
			kernel := NewWithFileIO(files)
			kernel.acquireLocks = func(Paths, Codec) (func() error, error) { locks++; return func() error { releases++; return nil }, nil }
			kernel = kernel.WithCodexDocumentParser(func(body []byte) (CodexDocument, error) {
				body[0] = '!'
				if typed {
					var doc *codexParserTestDocument
					return doc, nil
				}
				return nil, nil
			})
			if _, err := kernel.Apply(codexParserRequest(t)); err == nil {
				t.Fatal("nil document admitted")
			}
			if string(files.body) != "original snapshot" || files.writes != 0 || locks != 1 || releases != 1 {
				t.Fatalf("parser aliased snapshot or leaked lock: body=%q writes=%d lock=%d release=%d", files.body, files.writes, locks, releases)
			}
		})
	}
}

func TestProjectCodexParserRawEntriesDoNotAliasNeutralDocument(t *testing.T) {
	raw := json.RawMessage(`{"url":"https://inert.test/mcp","large":9007199254740993}`)
	parser := &codexParserTestDocument{entries: map[string]json.RawMessage{"foreign": raw}}
	kernel := New().WithCodexDocumentParser(func([]byte) (CodexDocument, error) { return parser, nil })
	doc, err := kernel.parseDocument(nil, false, CodecCodex)
	if err != nil {
		t.Fatal(err)
	}
	raw[0] = '!'
	entries, err := doc.codexEntries()
	if err != nil || !bytes.Contains(entries["foreign"], []byte("9007199254740993")) || !json.Valid(entries["foreign"]) {
		t.Fatalf("raw parser alias/number loss: %q %v", entries["foreign"], err)
	}
}
