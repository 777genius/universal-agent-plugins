package nativeconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirectoryKernelKeepsExistingCodecAndWriterLifecycle(t *testing.T) {
	directory := t.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	}()
	kernel, err := New().WithDirectory(root, directory, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "config.yaml")
	req := Request{Paths: Paths{JSON: path}, Codec: CodecHermes, Name: "owned", Action: ActionAdd, Server: Server{Type: "remote", URL: "https://example.test/one"}}
	receipt, err := kernel.Apply(req)
	if err != nil {
		t.Fatal(err)
	}
	req.Action, req.Owned, req.Server.URL = ActionUpdate, &receipt, "https://example.test/two"
	receipt, err = kernel.Apply(req)
	if err != nil {
		t.Fatal(err)
	}
	req.Action, req.Owned = ActionRemove, &receipt
	if _, err := kernel.Apply(req); err != nil {
		t.Fatal(err)
	}
	if present, _, err := kernel.Inspect(req.Paths, req.Codec, req.Name, nil); err != nil || present {
		t.Fatal("retained codec did not remove its exact owned entry", err)
	}
	if _, err := NewWithLockAcquirer(nil).WithDirectory(root, directory, nil); err == nil {
		t.Fatal("missing original authority accepted")
	}
	if _, err := NewWithFileIO(osFiles{}).WithDirectory(root, directory, func() error { return nil }); err == nil {
		t.Fatal("custom file authority bypassed")
	}
}
