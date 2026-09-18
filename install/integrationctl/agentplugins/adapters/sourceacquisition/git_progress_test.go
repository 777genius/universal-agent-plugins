package sourceacquisition

import (
	"context"
	"strings"
	"testing"
)

func TestParseGitProgressLineReadsReceivingPercent(t *testing.T) {
	t.Parallel()
	transfer, ok := parseGitProgressLine([]byte("Receiving objects:  40% (2/5), 1.20 KiB | 400.00 KiB/s"))
	if !ok || transfer.Phase != "receiving objects" || transfer.Fraction != 0.4 {
		t.Fatalf("transfer = %+v ok=%v", transfer, ok)
	}
}

func TestParseGitProgressLineReadsCarriageReturnUpdates(t *testing.T) {
	t.Parallel()
	reporter := &recordingTransferReporter{}
	writer := newGitProgressWriter(reporter)
	if _, err := writer.Write([]byte("remote: Counting objects:  10% (1/10)\rReceiving objects: 100% (10/10), done.\n")); err != nil {
		t.Fatal(err)
	}
	if len(reporter.events) != 2 {
		t.Fatalf("events = %+v", reporter.events)
	}
	if reporter.events[0].Phase != "counting objects" || reporter.events[0].Fraction != 0.1 {
		t.Fatalf("first = %+v", reporter.events[0])
	}
	if reporter.events[1].Phase != "receiving objects" || reporter.events[1].Fraction != 1 {
		t.Fatalf("second = %+v", reporter.events[1])
	}
}

func TestParseGitProgressLineIgnoresUnrelatedDiagnostics(t *testing.T) {
	t.Parallel()
	if _, ok := parseGitProgressLine([]byte("fatal: unable to access")); ok {
		t.Fatal("unrelated diagnostic parsed as progress")
	}
}

func TestGitCommandAttachesProgressWriterForFetch(t *testing.T) {
	t.Parallel()
	reporter := &recordingTransferReporter{}
	runner := &captureProgressRunner{}
	acquirer := Acquirer{Runner: runner, Reporter: reporter}
	if _, err := acquirer.gitCommand(context.Background(), t.TempDir())("fetch", "--progress", "origin", "abc"); err != nil {
		t.Fatal(err)
	}
	if runner.progress == nil {
		t.Fatal("fetch did not attach a progress writer")
	}
	if _, err := runner.progress.Write([]byte("Receiving objects:  50% (1/2)\n")); err != nil {
		t.Fatal(err)
	}
	if len(reporter.events) != 1 || reporter.events[0].Fraction != 0.5 || reporter.events[0].Phase != "receiving objects" {
		t.Fatalf("reported = %+v", reporter.events)
	}
}

func TestGitFetchArgsUseProgressOnlyWhenRequested(t *testing.T) {
	t.Parallel()
	quiet := strings.Join(gitFetchArgs("repo", "abc", false), " ")
	live := strings.Join(gitFetchArgs("repo", "abc", true), " ")
	if !strings.Contains(quiet, "--quiet") || strings.Contains(quiet, "--progress") {
		t.Fatalf("quiet fetch = %q", quiet)
	}
	if !strings.Contains(live, "--progress") || strings.Contains(live, "--quiet") {
		t.Fatalf("live fetch = %q", live)
	}
}

type recordingTransferReporter struct {
	events []Transfer
}

func (reporter *recordingTransferReporter) Report(transfer Transfer) {
	reporter.events = append(reporter.events, transfer)
}

type captureProgressRunner struct {
	progress interface{ Write([]byte) (int, error) }
}

func (runner *captureProgressRunner) Run(_ context.Context, command Command) ([]byte, error) {
	runner.progress = command.Progress
	return nil, nil
}
