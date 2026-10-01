package installerui

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/777genius/plugin-kit-ai/cli/internal/terminalreader"
	"github.com/muesli/cancelreader"
)

type formSession struct {
	writer       *formWriter
	reader       *formReader
	handoff      *submissionReader
	owned        cancelreader.CancelReader
	input        io.Reader
	output       io.Writer
	cancel       context.CancelFunc
	finishInput  func()
	stopCancel   func() bool
	callbackDone chan struct{}
	resizeErr    error
}

func (p terminalRenderer) formSession(ctx context.Context, cancel context.CancelFunc, config formInput) (*formSession, error) {
	s := &formSession{writer: &formWriter{Writer: p.Output, cancel: cancel}, cancel: cancel, callbackDone: make(chan struct{})}
	s.output = s.writer
	if f, ok := p.Output.(*os.File); ok {
		s.output = &formFileWriter{formWriter: s.writer, file: f}
	}
	source := p.Input
	if f, ok := p.Input.(*os.File); ok {
		var err error
		s.owned, err = terminalreader.New(f)
		if err != nil {
			return nil, fmt.Errorf("prepare terminal input: %w", err)
		}
		source = s.owned
	}
	if len(config.queued) > 0 {
		source = io.MultiReader(bytes.NewReader(config.queued), source)
	}
	s.handoff = newSubmissionReader(ctx, source)
	s.reader = &formReader{reader: s.handoff, cancel: cancel}
	s.input = s.reader
	if f, ok := p.Input.(*os.File); ok {
		s.input = &formFileReader{formReader: s.reader, file: f}
	}
	s.startCancellation(ctx)
	return s, nil
}

func (s *formSession) close() error {
	if !s.stopCancel() {
		<-s.callbackDone
	}
	s.reader.stop(s.finishInput)
	if s.owned != nil {
		if err := s.owned.Close(); err != nil {
			return fmt.Errorf("close terminal input: %w", err)
		}
	}
	return nil
}

func (s *formSession) startCancellation(ctx context.Context) {
	var finishOnce sync.Once
	s.finishInput = func() {
		finishOnce.Do(func() {
			s.handoff.finish()
			if s.owned != nil {
				s.owned.Cancel()
			}
		})
	}
	s.stopCancel = context.AfterFunc(ctx, func() { defer close(s.callbackDone); s.finishInput() })
}
