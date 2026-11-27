package skillsource

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func writeLocalSkill(t *testing.T, root, name string) {
	t.Helper()
	directory := filepath.Join(root, name)
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + name + "\ndescription: A bounded local test Skill\n---\nDocumentation only.\n"
	if err := os.WriteFile(filepath.Join(directory, "SKILL.md"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestLocalScanDeterministicAndSelection(t *testing.T) {
	root := physicalLocalTemp(t)
	writeLocalSkill(t, root, "review")
	writeLocalSkill(t, root, "testing")
	first, err := ScanLocal(t.Context(), root, ScanPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := ScanLocal(t.Context(), root, ScanPolicy{})
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatalf("unstable catalog: %v", err)
	}
	if !first.Complete || first.Commit != "" || first.Repository != root || len(first.Candidates) != 2 {
		t.Fatalf("catalog: %+v", first)
	}
	for _, candidate := range first.Candidates {
		if candidate.Status != "available" {
			t.Fatalf("candidate: %+v", candidate)
		}
		selected, err := LocalSkillPath(root, candidate.Path)
		if err != nil || selected != filepath.Join(root, candidate.Path) {
			t.Fatalf("selected: %s %v", selected, err)
		}
	}
	for _, relative := range []string{"../escape", "/absolute", "review/../testing", "review\\testing"} {
		if _, err := LocalSkillPath(root, relative); err == nil {
			t.Fatalf("accepted unsafe path %q", relative)
		}
	}
}

func TestLocalScanRootDocumentPrecedence(t *testing.T) {
	root := physicalLocalTemp(t)
	writeLocalSkill(t, root, "nested")
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("invalid metadata"), 0600); err != nil {
		t.Fatal(err)
	}
	catalog, err := ScanLocal(t.Context(), root, ScanPolicy{})
	if err != nil || len(catalog.Candidates) != 1 || catalog.Candidates[0].Path != "" || catalog.Candidates[0].Status == "available" {
		t.Fatalf("root fallback: %+v %v", catalog, err)
	}
}

func TestLocalScanBudgetsAndCancellation(t *testing.T) {
	root := physicalLocalTemp(t)
	writeLocalSkill(t, root, "first")
	writeLocalSkill(t, root, "second")
	for _, policy := range []ScanPolicy{{MaxEntries: 1}, {MaxCandidates: 1}, {MaxDocumentBytes: 1}, {MaxAggregateBytes: 1}, {MaxMetadataBytes: 1}} {
		catalog, err := ScanLocal(t.Context(), root, policy)
		if err != nil || catalog.Complete {
			t.Fatalf("unbounded catalog %+v: %+v %v", policy, catalog, err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := ScanLocal(ctx, root, ScanPolicy{}); err == nil {
		t.Fatal("ignored cancellation")
	}
}

func TestLocalScanRejectsObservedLinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink privilege is host dependent")
	}
	outside := physicalLocalTemp(t)
	writeLocalSkill(t, outside, "secret")
	root := physicalLocalTemp(t)
	if err := os.Symlink(filepath.Join(outside, "secret"), filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	catalog, err := ScanLocal(t.Context(), root, ScanPolicy{})
	if err != nil || catalog.Complete || len(catalog.Candidates) != 0 {
		t.Fatalf("followed directory link: %+v %v", catalog, err)
	}
	if _, err := LocalSkillPath(root, "linked"); err == nil {
		t.Fatal("selected directory link")
	}
	if err := os.Symlink(filepath.Join(outside, "secret", "SKILL.md"), filepath.Join(root, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	catalog, err = ScanLocal(t.Context(), root, ScanPolicy{})
	if err != nil || len(catalog.Candidates) != 1 || catalog.Candidates[0].Status != "invalid" {
		t.Fatalf("followed root document: %+v %v", catalog, err)
	}
	parent := physicalLocalTemp(t)
	alias := filepath.Join(parent, "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	if _, err := ScanLocal(t.Context(), alias, ScanPolicy{}); err == nil {
		t.Fatal("accepted linked root")
	}
	if _, err := ScanLocal(t.Context(), filepath.Join(alias, "linked"), ScanPolicy{}); err == nil {
		t.Fatal("accepted linked ancestor")
	}
}

func TestLocalScanNeverReadsResources(t *testing.T) {
	root := physicalLocalTemp(t)
	writeLocalSkill(t, root, "review")
	if err := os.WriteFile(filepath.Join(root, "review", "resource.bin"), make([]byte, 4096), 0000); err != nil {
		t.Fatal(err)
	}
	catalog, err := ScanLocal(t.Context(), root, ScanPolicy{MaxAggregateBytes: 1024})
	if err != nil || !catalog.Complete || len(catalog.Candidates) != 1 || catalog.Candidates[0].Status != "available" || catalog.Bytes >= 1024 {
		t.Fatalf("resource read: %+v %v", catalog, err)
	}
}

func physicalLocalTemp(t *testing.T) string {
	t.Helper()
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return directory
}

func TestLocalScanSharedEntryBudget(t *testing.T) {
	root := physicalLocalTemp(t)
	for i := 0; i < 140; i++ {
		writeLocalSkill(t, root, fmt.Sprintf("skill-%03d", i))
	}
	opened, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	for _, limit := range []int{1, 3, 17, 128, 129} {
		scanner := localScanner{root: opened, policy: ScanPolicy{MaxEntries: limit}.bounded(), catalog: Catalog{Complete: true}}
		if err := scanner.walk(t.Context(), ".", 0); err != nil {
			t.Fatal(err)
		}
		if scanner.entries > limit || scanner.catalog.Complete {
			t.Fatalf("limit %d exceeded: entries=%d complete=%t", limit, scanner.entries, scanner.catalog.Complete)
		}
	}
	scanner := localScanner{root: opened, policy: ScanPolicy{MaxMetadataBytes: 150}.bounded(), catalog: Catalog{Complete: true}}
	if err := scanner.walk(t.Context(), ".", 0); err != nil {
		t.Fatal(err)
	}
	if scanner.metadata > 150 || scanner.catalog.Complete {
		t.Fatalf("metadata exceeded: %d", scanner.metadata)
	}
}
