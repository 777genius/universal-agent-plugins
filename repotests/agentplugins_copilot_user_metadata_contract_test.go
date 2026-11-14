//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"
)

func copilotUserMetadataFixture(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), ".copilot")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	projectNativeRuntimeWrite(t, filepath.Join(root, "config.json"), []byte("{\"independent\":true}\n"), 0600)
	return root
}

func copilotUserMetadataHeld(t *testing.T, root string) *copilotUserMetadata {
	t.Helper()
	result, err := copilotUserMetadataCapture(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func copilotUserMetadataDecoded(t *testing.T, body []byte) copilotUserMetadataProjection {
	t.Helper()
	var result copilotUserMetadataProjection
	if len(body) > 2048 || !json.Valid(body) || json.Unmarshal(body, &result) != nil || result.Stage != "add-user" || result.Status != "observed" {
		t.Fatal("closed metadata JSON/byte/status bound")
	}
	return result
}

func TestCopilotUserMetadataDoesNotReadOrExportUnknownPrivateBodies(t *testing.T) {
	root := copilotUserMetadataFixture(t)
	path := filepath.Join(root, "PRIVATE-NAME-user-account")
	projectNativeRuntimeWrite(t, path, []byte("PRIVATE-BODY-credential-like-value\x00\xff"), 0000)
	before := copilotUserMetadataHeld(t, root)
	after := copilotUserMetadataHeld(t, root)
	body := copilotUserMetadataDelta(before, after)
	result := copilotUserMetadataDecoded(t, body)
	if result.BeforeEntries != 3 || result.AfterEntries != 3 || result.ChangedEntries != 0 || result.OmittedCount != 0 || len(result.Entries) != 0 {
		t.Fatal("unchanged ordinary metadata inventory differs")
	}
	for _, private := range []string{"PRIVATE-NAME", "PRIVATE-BODY", root, "credential-like-value", "config.json"} {
		if bytes.Contains(body, []byte(private)) {
			t.Fatal("metadata projection exposed private names, paths or contents")
		}
	}
}

func TestCopilotUserMetadataFullSerializedBoundAndTrueOmissions(t *testing.T) {
	root := copilotUserMetadataFixture(t)
	if err := os.Chtimes(root, time.Unix(10, 0), time.Unix(10, 0)); err != nil {
		t.Fatal(err)
	}
	before := copilotUserMetadataHeld(t, root)
	for i := range 30 {
		projectNativeRuntimeWrite(t, filepath.Join(root, fmt.Sprintf("PRIVATE-NAME-%03d", i)), []byte("PRIVATE-BODY"), 0600)
	}
	after := copilotUserMetadataHeld(t, root)
	body := copilotUserMetadataDelta(before, after)
	result := copilotUserMetadataDecoded(t, body)
	if result.BeforeEntries != 2 || result.AfterEntries != 32 || result.ChangedEntries != 31 || len(result.Entries) == 0 || len(result.Entries) > 16 || result.OmittedCount != 31-len(result.Entries) || result.OmittedCount == 0 {
		t.Fatal("full counts/serialized prefix omissions differ")
	}
	last := ""
	for _, entry := range result.Entries {
		if entry.PathSHA256 <= last || len(entry.PathSHA256) != 64 {
			t.Fatal("metadata records are not a deterministic unique hash prefix")
		}
		last = entry.PathSHA256
	}
	if !bytes.Equal(body, copilotUserMetadataDelta(before, after)) || bytes.Contains(body, []byte("PRIVATE-")) {
		t.Fatal("projection is unstable or private")
	}
	// The limit covers the entire object, including full counts and header.
	more := result
	more.Entries = append(more.Entries, result.Entries[0])
	more.OmittedCount--
	if raw, err := json.Marshal(more); err != nil || len(raw) <= 2048 {
		t.Fatal("chosen prefix did not saturate the complete byte budget")
	}
}

func TestCopilotUserMetadataSameBytesReplacementAndModeAreFactsOnly(t *testing.T) {
	for _, replacement := range []bool{false, true} {
		root := copilotUserMetadataFixture(t)
		path := filepath.Join(root, "target")
		projectNativeRuntimeWrite(t, path, []byte("PRIVATE-BODY"), 0600)
		before := copilotUserMetadataHeld(t, root)
		if replacement {
			if err := os.Rename(path, filepath.Join(filepath.Dir(root), "held-original")); err != nil {
				t.Fatal(err)
			}
			projectNativeRuntimeWrite(t, path, []byte("PRIVATE-BODY"), 0600)
		} else if err := os.Chmod(path, 0640); err != nil {
			t.Fatal(err)
		}
		after := copilotUserMetadataHeld(t, root)
		result := copilotUserMetadataDecoded(t, copilotUserMetadataDelta(before, after))
		found := false
		for _, entry := range result.Entries {
			if entry.PathSHA256 == "34a04005bcaf206eec990bd9637d9fdb6725e0a0c0d4aebf003f17f4c956eb5c" {
				found = true
				if !entry.BeforePresent || !entry.AfterPresent || !entry.OwnerEqual || !entry.GroupEqual || entry.IdentityEqual == replacement || entry.ModeEqual != replacement {
					t.Fatal("replacement/mode metadata equality facts differ")
				}
			}
		}
		if !found {
			t.Fatal("independently fixed target path hash missing")
		}
	}
}

func TestCopilotUserMetadataRefusesAliasSpecialAndHardlinkWithoutReading(t *testing.T) {
	for _, kind := range []string{"symlink", "fifo", "hardlink", "root-alias"} {
		t.Run(kind, func(t *testing.T) {
			root := copilotUserMetadataFixture(t)
			path := filepath.Join(root, "unknown")
			outside := filepath.Join(filepath.Dir(root), "outside-private")
			projectNativeRuntimeWrite(t, outside, []byte("PRIVATE-BODY"), 0600)
			var err error
			switch kind {
			case "symlink":
				err = os.Symlink(outside, path)
			case "fifo":
				err = syscall.Mkfifo(path, 0600)
			case "hardlink":
				err = os.Link(outside, path)
			case "root-alias":
				alias := filepath.Join(t.TempDir(), ".copilot")
				err = os.Symlink(root, alias)
				root = alias
			}
			if err != nil {
				t.Fatal(err)
			}
			if result, err := copilotUserMetadataCapture(context.Background(), root); err == nil || result != nil {
				t.Fatal("alias/special/multiple-link profile accepted")
			}
		})
	}
}

func TestCopilotUserMetadataRealEntryDepthAndOriginalDeadlineBounds(t *testing.T) {
	root := copilotUserMetadataFixture(t)
	for i := range 1022 {
		projectNativeRuntimeWrite(t, filepath.Join(root, fmt.Sprint(i)), nil, 0600)
	}
	if got := copilotUserMetadataHeld(t, root); len(got.nodes) != 1024 {
		t.Fatal("exact root-inclusive entry boundary differs")
	}
	projectNativeRuntimeWrite(t, filepath.Join(root, "one-too-many"), nil, 0600)
	if _, err := copilotUserMetadataCapture(context.Background(), root); err == nil {
		t.Fatal("entry overflow accepted")
	}
	root = copilotUserMetadataFixture(t)
	deep := root
	for range 33 {
		deep = filepath.Join(deep, "d")
		if err := os.Mkdir(deep, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := copilotUserMetadataCapture(context.Background(), root); err == nil {
		t.Fatal("depth overflow accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := copilotUserMetadataAfter(ctx, root, nil); !bytes.Equal(got, copilotUserMetadataRefusal()) {
		t.Fatal("exhausted original context allowed deferred inventory")
	}
}

func TestCopilotUserMetadataParentReplacementAndGoexitPreserveFailure(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "owned", ".copilot")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	before := copilotUserMetadataHeld(t, root)
	if err := os.Rename(filepath.Dir(root), filepath.Join(parent, "held-original")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Dir(root), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(parent, "held-original", ".copilot"), root); err != nil {
		t.Fatal(err)
	}
	after := copilotUserMetadataHeld(t, root)
	if !bytes.Equal(copilotUserMetadataDelta(before, after), copilotUserMetadataRefusal()) {
		t.Fatal("coherent parent replacement accepted")
	}
	stage, originalFailure := "public-user", "original-command-or-guard-failure"
	var body []byte
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { body = copilotUserMetadataAfter(context.Background(), root, after) }()
		runtime.Goexit()
	}()
	<-done
	if !json.Valid(body) || stage != "public-user" || originalFailure != "original-command-or-guard-failure" {
		t.Fatal("deferred read-only observation masked original failure")
	}
}
