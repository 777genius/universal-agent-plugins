package journal

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	fsadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/fs"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func TestFileJournalLifecycle(t *testing.T) {
	t.Parallel()
	j := FileJournal{
		FS:      fsadapter.OS{},
		BaseDir: filepath.Join(t.TempDir(), "ops"),
	}
	op := domain.OperationRecord{
		OperationID:   "op1",
		Type:          "add",
		IntegrationID: "demo",
		Status:        "in_progress",
		StartedAt:     "2026-04-09T00:00:00Z",
	}
	if err := j.Start(context.Background(), op); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := j.AppendStep(context.Background(), "op1", domain.JournalStep{Target: "claude", Action: "plan", Status: "ok"}); err != nil {
		t.Fatalf("append: %v", err)
	}
	open, err := j.ListOpen(context.Background())
	if err != nil {
		t.Fatalf("list open: %v", err)
	}
	if len(open) != 1 {
		t.Fatalf("open count = %d, want 1", len(open))
	}
	if err := j.Finish(context.Background(), "op1", "committed"); err != nil {
		t.Fatalf("finish: %v", err)
	}
	open, err = j.ListOpen(context.Background())
	if err != nil {
		t.Fatalf("list open after finish: %v", err)
	}
	if len(open) != 0 {
		t.Fatalf("open count after finish = %d, want 0", len(open))
	}
}

func TestFileJournalRejectsDuplicateAndUnsafeOperationIDs(t *testing.T) {
	t.Parallel()
	j := FileJournal{FS: fsadapter.OS{}, BaseDir: filepath.Join(t.TempDir(), "ops")}
	op := domain.OperationRecord{OperationID: "op1", Status: "in_progress"}
	if err := j.Start(context.Background(), op); err != nil {
		t.Fatal(err)
	}
	if err := j.Start(context.Background(), op); err == nil {
		t.Fatal("duplicate journal start succeeded")
	}
	if err := j.Start(context.Background(), domain.OperationRecord{OperationID: "../escape"}); err == nil {
		t.Fatal("unsafe operation id succeeded")
	}
}

func TestFileJournalFailsClosedOnCorruptOrFutureRecords(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"corrupt.json": `{`,
		"future.json":  `{"schema_version":2,"operation_id":"future","status":"in_progress"}`,
		"unknown.json": `{"schema_version":1,"operation_id":"unknown","status":"in_progress","future":true}`,
	}
	for filename, body := range tests {
		filename, body := filename, body
		t.Run(filename, func(t *testing.T) {
			base := filepath.Join(t.TempDir(), "ops")
			if err := os.MkdirAll(base, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(base, filename), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			j := FileJournal{FS: fsadapter.OS{}, BaseDir: base}
			if _, err := j.ListOpen(context.Background()); err == nil || !strings.Contains(err.Error(), "read operation journal") {
				t.Fatalf("ListOpen error = %v", err)
			}
		})
	}
}
