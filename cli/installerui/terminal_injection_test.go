package installerui

import (
	"context"
	"io"
)

// Private injected form seam; public Terminal accepts only actual terminal files.
type testRich struct {
	Input   io.Reader
	Output  io.Writer
	NoColor bool
}
type testPlain struct {
	Input  io.Reader
	Output io.Writer
}
type testPrompter interface {
	SelectMany(context.Context, MultiSelectRequest) (Selection, error)
	Confirm(context.Context, ConfirmRequest) (Confirmation, error)
}

func (p testRich) terminal() *Terminal {
	return &Terminal{mode: ModeRich, renderer: terminalRenderer(p)}
}
func (p testRich) SelectMany(ctx context.Context, r MultiSelectRequest) (Selection, error) {
	return p.terminal().SelectMany(ctx, r)
}
func (p testRich) Confirm(ctx context.Context, r ConfirmRequest) (Confirmation, error) {
	return p.terminal().Confirm(ctx, r)
}
func (p testPlain) terminal() *Terminal {
	return &Terminal{mode: ModePlain, renderer: terminalRenderer{Input: p.Input, Output: p.Output, NoColor: true}}
}
func (p testPlain) SelectMany(ctx context.Context, r MultiSelectRequest) (Selection, error) {
	return p.terminal().SelectMany(ctx, r)
}
func (p testPlain) Confirm(ctx context.Context, r ConfirmRequest) (Confirmation, error) {
	return p.terminal().Confirm(ctx, r)
}

type broken struct{}

func (broken) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
