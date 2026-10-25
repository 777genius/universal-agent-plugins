//go:build linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type sequenceEvents struct {
	mu    sync.Mutex
	items []string
}

func (e *sequenceEvents) add(s string) { e.mu.Lock(); e.items = append(e.items, s); e.mu.Unlock() }
func (e *sequenceEvents) copy() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.items...)
}

type sequenceRunner struct {
	events      *sequenceEvents
	outerReader io.ReadCloser
}

func (r *sequenceRunner) RunDuplexWithPlannedShutdown(ctx context.Context, command ports.Command, exchange func(io.Writer, io.Reader) error) error {
	var argv []string
	if err := json.Unmarshal([]byte(command.Argv[len(command.Argv)-1]), &argv); err != nil {
		return err
	}
	label := argv[0]
	r.events.add("start:" + label)
	var reader io.Reader = strings.NewReader(testProof() + "inert\n")
	if label == "Xvfb" && r.outerReader != nil {
		reader = r.outerReader
	}
	err := exchange(io.Discard, reader)
	if label == "Xvfb" && r.outerReader != nil {
		_ = r.outerReader.Close()
	}
	r.events.add("reap:" + label)
	return err
}
func runInertSequence(t *testing.T, fail string, overflow bool) ([]string, []childRecord, error) {
	t.Helper()
	events := &sequenceEvents{}
	work, cancelWork := context.WithCancel(t.Context())
	defer cancelWork()
	c := &controller{request: sealedRequest{Input: inputRequest{UID: 1001, GID: 1001}}, parentNS: 1, work: work, files: map[string][]byte{}}
	c.cancelWork = func() { events.add("work-cancel"); cancelWork() }
	runner := &sequenceRunner{events: events}
	c.runner = runner
	innerStarted := make(chan struct{})
	writerDone := make(chan struct{})
	if overflow {
		reader, writer := io.Pipe()
		runner.outerReader = reader
		go func() {
			defer close(writerDone)
			defer func() { _ = writer.Close() }()
			_, _ = writer.Write([]byte(testProof()))
			<-innerStarted
			_, _ = writer.Write([]byte(strings.Repeat("x", streamLimit+1)))
		}()
	}
	outerCtx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	_, err := runDisplayTable(work, func(exchange func(context.Context, isolationProof) error) (childRecord, error) {
		return c.strict(outerCtx, "Xvfb", []string{"Xvfb"}, t.TempDir(), map[string]string{}, exchange)
	}, func(context.Context, isolationProof) error { events.add("ready"); return nil }, func(label string) error {
		_, err := c.strict(work, label, []string{label}, t.TempDir(), map[string]string{}, func(ctx context.Context, _ isolationProof) error {
			if overflow {
				close(innerStarted)
				<-ctx.Done()
				return ctx.Err()
			}
			if label == fail {
				return errors.New("inert probe failure")
			}
			return nil
		})
		return err
	})
	if overflow {
		select {
		case <-writerDone:
		case <-time.After(time.Second):
			t.Fatal("fake producer did not join")
		}
	}
	return events.copy(), c.children, err
}
func TestFixedFiveCallNestingJoinsEveryInnerBeforeDisplay(t *testing.T) {
	events, records, err := runInertSequence(t, "", false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"start:Xvfb", "ready", "start:A", "reap:A", "start:B", "reap:B", "start:A-restart", "reap:A-restart", "start:A-after-removal", "reap:A-after-removal", "reap:Xvfb"}
	if !reflect.DeepEqual(events, want) || len(records) != 5 {
		t.Fatalf("nested order %v records=%d", events, len(records))
	}
	for _, record := range records {
		if !record.TreeReaped || !record.PlannedStop {
			t.Fatal("inert strict boundary lost", record)
		}
	}
}
func TestFailedGUIStopsTableBeforeAnotherProbeAndDisplayReturnsLast(t *testing.T) {
	events, _, err := runInertSequence(t, "B", false)
	if err == nil {
		t.Fatal("failed exchange became success")
	}
	want := []string{"start:Xvfb", "ready", "start:A", "reap:A", "start:B", "reap:B", "reap:Xvfb"}
	if !reflect.DeepEqual(events, want) {
		t.Fatal(events)
	}
}
func TestOuterOverflowCancelsWorkThenJoinsInnerBeforeDisplay(t *testing.T) {
	events, records, err := runInertSequence(t, "", true)
	if err == nil {
		t.Fatal("overflow became success")
	}
	want := []string{"start:Xvfb", "ready", "start:A", "work-cancel", "reap:A", "reap:Xvfb"}
	if !reflect.DeepEqual(events, want) {
		t.Fatal(events)
	}
	if len(records) != 2 || records[1].PlannedStop || !records[1].MergedOverflow {
		t.Fatal(records)
	}
}

type endingErrorReader struct{ cause error }

func (r endingErrorReader) Read([]byte) (int, error) { return 0, r.cause }
func TestUnexpectedMergedReadErrorCannotBecomePlannedSuccess(t *testing.T) {
	sentinel := errors.New("inert read failure")
	_, cancel := context.WithCancel(t.Context())
	defer cancel()
	m := &mergedCapture{ready: make(chan struct{}), done: make(chan struct{}), cancel: cancel}
	m.drain(io.MultiReader(strings.NewReader(testProof()), endingErrorReader{sentinel}), 1001, 1001, 1)
	if !errors.Is(m.err, sentinel) {
		t.Fatal("read error discarded", m.err)
	}
}

type unjoinedReader struct {
	release <-chan struct{}
	joined  chan<- struct{}
}

func (r unjoinedReader) Read([]byte) (int, error) {
	<-r.release
	close(r.joined)
	return 0, io.EOF
}

type unjoinedRunner struct{ reader io.Reader }

func (r unjoinedRunner) RunDuplexWithPlannedShutdown(_ context.Context, _ ports.Command, exchange func(io.Writer, io.Reader) error) error {
	return exchange(io.Discard, r.reader)
}
func TestUnjoinedDrainCannotBecomePlannedSuccess(t *testing.T) {
	release, joined := make(chan struct{}), make(chan struct{})
	work, cancel := context.WithCancel(t.Context())
	defer cancel()
	reader := io.MultiReader(strings.NewReader(testProof()), unjoinedReader{release, joined})
	c := &controller{request: sealedRequest{Input: inputRequest{UID: 1001, GID: 1001}}, parentNS: 1, work: work, files: map[string][]byte{}, runner: unjoinedRunner{reader}}
	record, err := c.strict(work, "A", []string{"inert"}, t.TempDir(), map[string]string{}, func(context.Context, isolationProof) error { return nil })
	close(release)
	select {
	case <-joined:
	case <-time.After(time.Second):
		t.Fatal("inert reader failed to join after release")
	}
	if err == nil || record.PlannedStop || record.TreeReaped || !strings.Contains(record.Error, "merged drain did not join") {
		t.Fatalf("unjoined drain became success: %+v %v", record, err)
	}
}
func TestOnlyExplicitPlannedCloseMayDiscardClosedPipe(t *testing.T) {
	for _, planned := range []bool{false, true} {
		_, cancel := context.WithCancel(t.Context())
		m := &mergedCapture{ready: make(chan struct{}), done: make(chan struct{}), cancel: cancel, planned: planned}
		m.drain(io.MultiReader(strings.NewReader(testProof()), endingErrorReader{os.ErrClosed}), 1001, 1001, 1)
		cancel()
		if (m.err == nil) != planned {
			t.Fatalf("closed pipe planned=%t error=%v", planned, m.err)
		}
	}
}
