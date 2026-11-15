package installerui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"

	"github.com/777genius/plugin-kit-ai/cli/internal/promptio"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
)

// TerminalMode selects interaction without changing host selection or consent policy.
type TerminalMode string

const (
	ModeAuto  TerminalMode = "auto"
	ModeRich  TerminalMode = "rich"
	ModePlain TerminalMode = "plain"
)

// TerminalConfig contains borrowed prompt handles, independent of data stdout.
// NoColor is the host's effective decision; the UI never reevaluates NO_COLOR.
type TerminalConfig struct {
	Input   *os.File
	Output  *os.File
	Mode    TerminalMode
	NoColor bool
}

type MultiSelectRequest struct {
	SelectRequest
	MinSelected int
	Filterable  bool
}

var (
	ErrInvalidRequest   = errors.New("invalid terminal request")
	ErrInvalidSelection = errors.New("invalid terminal selection")
	ErrReviewTooLarge   = errors.New("confirmation review exceeds the terminal display budget")
)

// Terminal owns readers only during a call, never the borrowed descriptors.
// All calls, including methods on PlainUI, must be sequential on the same TTY.
// Supply a signal/deadline-aware context to each prompt.
type Terminal struct {
	renderer terminalRenderer
	mode     TerminalMode
}

// Private I/O injection is used by neutral reader/form qualification only.
type terminalRenderer struct {
	Input   io.Reader
	Output  io.Writer
	NoColor bool
}

func NewTerminal(cfg TerminalConfig) (*Terminal, error) {
	mode, err := terminalMode(cfg)
	if err != nil {
		return nil, err
	}
	return &Terminal{renderer: terminalRenderer{Input: cfg.Input, Output: cfg.Output, NoColor: cfg.NoColor}, mode: mode}, nil
}

func (t *Terminal) Mode() TerminalMode { return t.mode }

// PlainUI preserves the legacy line API and grammar for action/unit menus.
// Its cancellable reader joins before return; no reader lives between calls.
// UI.Confirm has legacy semantics WITHOUT a consent snapshot. Always use
// Terminal.Confirm for terminal authorization. Rich mode does not affect this UI.
func (t *Terminal) PlainUI() (*UI, error) {
	theme := terminaltheme.Theme{Enabled: !t.renderer.NoColor}
	return New(Config{Input: t.renderer.Input, Output: t.renderer.Output, ReadLine: promptio.ReadLine,
		Style: func(s string) string { return theme.Text(terminaltheme.Label, s) }})
}

func (t *Terminal) SelectMany(ctx context.Context, req MultiSelectRequest) (Selection, error) {
	if err := terminalContext(ctx); err != nil {
		return Selection{}, err
	}
	req, err := normalizeMulti(req)
	if err != nil {
		return Selection{}, err
	}
	var result Selection
	if t.mode == ModeRich {
		result, err = t.renderer.selectMany(ctx, req)
	} else {
		result, err = t.selectPlain(ctx, req)
	}
	if err != nil {
		return Selection{}, terminalError(err)
	}
	return result, nil
}

func (t *Terminal) Confirm(ctx context.Context, req ConfirmRequest) (Confirmation, error) {
	return t.ConfirmWithDetails(ctx, req, nil)
}

type confirmationReview struct {
	ConfirmRequest
	Details []string
}

// ConfirmWithDetails adds an inspectable full-path view without changing the
// existing ConfirmRequest struct or Confirm method contract.
func (t *Terminal) ConfirmWithDetails(ctx context.Context, req ConfirmRequest, details []string) (result Confirmation, err error) {
	if err := terminalContext(ctx); err != nil {
		return Confirmation{}, err
	}
	review, err := normalizeConfirmationReview(confirmationReview{ConfirmRequest: req, Details: details})
	if err != nil {
		return Confirmation{}, err
	}
	restore, err := t.renderer.snapshot(ctx)
	if err != nil {
		return Confirmation{}, err
	}
	defer func() {
		err = errors.Join(err, restore())
		if err != nil {
			result = Confirmation{}
			err = terminalError(err)
		}
	}()
	queued, err := confirmationInput(ctx, t.renderer.Input)
	if err != nil {
		return Confirmation{}, err
	}
	if t.mode == ModeRich {
		return t.renderer.confirm(ctx, review, queued)
	}
	return t.confirmPlain(ctx, review, queued)
}

func terminalContext(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("%w: context is required", ErrInvalidRequest)
	}
	return ctx.Err()
}

func terminalError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return errors.Join(ErrCancelled, err)
	}
	if errors.Is(err, promptio.ErrUnavailable) {
		return errors.Join(ErrUnavailable, err)
	}
	return err
}

func (p terminalRenderer) snapshot(ctx context.Context) (func() error, error) {
	if f, ok := p.Input.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		snapshot := promptio.SnapshotTerminal
		if ops := promptOps(ctx); ops != nil && ops.snapshot != nil {
			snapshot = ops.snapshot
		}
		restore, err := snapshot(int(f.Fd()))
		if err != nil {
			return nil, fmt.Errorf("snapshot prompt terminal: %w", err)
		}
		return func() error {
			if err := restore(); err != nil {
				return fmt.Errorf("restore prompt terminal: %w", err)
			}
			return nil
		}, nil
	}
	return func() error { return nil }, nil
}
