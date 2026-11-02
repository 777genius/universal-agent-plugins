//go:build windows

package securityscan

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestWindowsAtomicPublicationSecurityCache(t *testing.T) {
	cache := FileCache{Root: filepath.Join(t.TempDir(), "assessments")}
	key := strings.Repeat("a", 64)
	requirement := DefaultRequirement()
	subject := domain.SecuritySubject{TreeDigest: testTreeDigest, ManifestDigest: testManifestDigest}
	first := domain.SecurityAssessment{
		SchemaVersion: 1, Subject: subject, Scanner: requirement.Scanner, Policy: requirement.Policy,
		Outcome: domain.SecurityNoBlockingFindings, ScannedFiles: 1, ReportDigest: testTreeDigest,
		Evidence: domain.SecurityEvidenceLocalScan,
	}
	if err := first.Validate(requirement, subject); err != nil {
		t.Fatal(err)
	}
	if err := cache.Store(key, first); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cache.Root, key+".json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(cache.Root)
	if err != nil {
		t.Fatal(err)
	}
	reader, openErr := root.Open(filepath.Base(path))
	closeErr := root.Close()
	if openErr != nil || closeErr != nil {
		if reader != nil {
			_ = reader.Close()
		}
		t.Fatalf("open controlled cache reader: %v %v", openErr, closeErr)
	}
	t.Cleanup(func() {
		if err := reader.Close(); err != nil {
			t.Error(err)
		}
	})
	second := first
	second.ScannedFiles = 2
	if err := cache.Store(key, second); err != nil {
		t.Fatal(err)
	}
	old, err := io.ReadAll(reader)
	if err != nil || !bytes.Equal(old, before) {
		t.Fatal("cache publication changed bytes behind the retained reader")
	}
	loaded, ok := cache.Load(key)
	if !ok || !reflect.DeepEqual(loaded, second) {
		t.Fatal("fresh cache reader did not observe the complete new assessment")
	}
	after, err := os.ReadFile(path)
	want, marshalErr := json.Marshal(second)
	if err != nil || marshalErr != nil || !bytes.Equal(after, want) {
		t.Fatal("cache target is not the complete encoded assessment")
	}
	entries, err := os.ReadDir(cache.Root)
	if err != nil || len(entries) != 1 || entries[0].Name() != key+".json" {
		t.Fatal("cache publication leaked a temporary file or removed its target")
	}
}
