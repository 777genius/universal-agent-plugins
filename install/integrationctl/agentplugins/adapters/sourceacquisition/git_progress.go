package sourceacquisition

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// Transfer is a UI-free download update. Fraction is 0..1 for the current Git phase.
type Transfer struct {
	Phase    string
	Fraction float64
}

// Reporter receives live Git transfer updates. Nil reporters are ignored.
type Reporter interface {
	Report(Transfer)
}

// ReporterBinder is implemented by acquirers that can stream transfer updates.
type ReporterBinder interface {
	BindReporter(Reporter)
}

var gitProgressPattern = regexp.MustCompile(`(?i)(?:remote: )?((?:counting|compressing|receiving) objects|resolving deltas|checking out files):\s+(\d+)%`)

type gitProgressWriter struct {
	mu       sync.Mutex
	reporter Reporter
	pending  []byte
}

func newGitProgressWriter(reporter Reporter) *gitProgressWriter {
	if reporter == nil {
		return nil
	}
	return &gitProgressWriter{reporter: reporter}
}

func (writer *gitProgressWriter) Write(payload []byte) (int, error) {
	if writer == nil || writer.reporter == nil {
		return len(payload), nil
	}
	writer.mu.Lock()
	defer writer.mu.Unlock()
	writer.pending = append(writer.pending, payload...)
	for {
		index := bytes.IndexAny(writer.pending, "\r\n")
		if index < 0 {
			break
		}
		line := writer.pending[:index]
		writer.pending = writer.pending[index+1:]
		if transfer, ok := parseGitProgressLine(line); ok {
			writer.reporter.Report(transfer)
		}
	}
	return len(payload), nil
}

func parseGitProgressLine(line []byte) (Transfer, bool) {
	match := gitProgressPattern.FindSubmatch(bytes.TrimSpace(line))
	if len(match) != 3 {
		return Transfer{}, false
	}
	percent, err := strconv.Atoi(string(match[2]))
	if err != nil || percent < 0 {
		return Transfer{}, false
	}
	if percent > 100 {
		percent = 100
	}
	return Transfer{Phase: strings.ToLower(string(match[1])), Fraction: float64(percent) / 100}, true
}
