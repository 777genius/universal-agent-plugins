//go:build !linux

package grok

import (
	"context"
	"fmt"
	"io"
)

type grokACPTail struct{}

func captureGrokACPTail(io.Reader) (*grokACPTail, error) {
	return nil, fmt.Errorf("grok ACP complete-stream verification is unqualified on this platform")
}

func (*grokACPTail) close() error { return nil }

func (*grokACPTail) drain(context.Context, []byte) error {
	return fmt.Errorf("grok ACP complete-stream verification is unqualified on this platform")
}
