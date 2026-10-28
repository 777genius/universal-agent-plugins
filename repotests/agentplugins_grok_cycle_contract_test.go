//go:build linux

package pluginkitairepo_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGrokCycleRawPrivacyDoesNotRequireProvider(t *testing.T) {
	for _, raw := range [][]byte{[]byte("uap-grok-private-inert"), []byte(`{"value":"\u0075ap-grok-private-inert"}`), []byte("{}\n{\"value\":\"\\u0075ap-grok-private-inert\"}\n")} {
		if _, err := grokCycleStream(raw); err == nil {
			t.Fatal("provider-free cycle encoded private raw input")
		}
	}
	value, err := grokCycleStream([]byte("ordinary fixture\n"))
	if err != nil || string(value.Body) != "ordinary fixture\n" || value.Bytes != len(value.Body) || value.SHA256 != clineAttemptSHA(value.Body) {
		t.Fatal("provider-free raw byte receipt differs", err)
	}
}

func TestGrokCycleSnapshotHoldsOriginalIdentityAndRefusesLinks(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "original")
	projectNativeRuntimeWrite(t, path, []byte("original bytes\n"), 0600)
	before, err := grokCycleSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	after, err := grokCycleSnapshot(root)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("ordinary held original changed", err)
	}
	replacement := filepath.Join(t.TempDir(), "replacement")
	projectNativeRuntimeWrite(t, replacement, []byte("original bytes\n"), 0600)
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	after, err = grokCycleSnapshot(root)
	if err != nil || reflect.DeepEqual(before, after) {
		t.Fatal("same bytes adopted a replacement original inode", err)
	}
	link := filepath.Join(root, "alias")
	if err := os.Link(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := grokCycleSnapshot(root); err == nil {
		t.Fatal("hard-linked original was adopted")
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := grokCycleSnapshot(root); err == nil {
		t.Fatal("symlinked original was adopted")
	}
}
