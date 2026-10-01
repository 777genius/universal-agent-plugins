package installerui

import (
	"context"
	"errors"
	"fmt"
	"io"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/charmbracelet/colorprofile"
)

func (p terminalRenderer) run(ctx context.Context, form *huh.Form, configs ...formInput) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if p.Input == nil || p.Output == nil {
		return ErrUnavailable
	}
	var config formInput
	if len(configs) > 0 {
		config = configs[0]
	}
	restore, err := p.snapshot(ctx)
	if err != nil {
		return err
	}
	defer func() { err = cleanupError(err, restore()) }()
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	session, err := p.formSession(runCtx, cancel, config)
	if err != nil {
		return err
	}
	defer func() { err = cleanupError(err, session.close()) }()
	p.configureForm(form, session, config)
	// Huh assumes a nonnil returned model on initialization failure. Tea retains
	// its own cleanup; never expose an arbitrary recovered payload.
	defer func() {
		if recover() != nil {
			err = errors.Join(err, errors.New("prompt initialization failed"))
		}
	}()
	return session.result(ctx, form.RunWithContext(runCtx))
}

func (s *formSession) result(ctx context.Context, runErr error) error {
	var failures []error
	if err := s.writer.Err(); err != nil {
		failures = append(failures, fmt.Errorf("write prompt: %w", err))
	}
	if err := ctx.Err(); err != nil {
		failures = append(failures, terminalError(err))
	}
	if s.resizeErr != nil {
		failures = append(failures, fmt.Errorf("query terminal size: %w", s.resizeErr))
	}
	if err := s.reader.Err(); err != nil {
		failures = append(failures, fmt.Errorf("read prompt: %w", err))
	}
	if len(failures) != 0 {
		return errors.Join(failures...)
	}
	if errors.Is(runErr, huh.ErrUserAborted) {
		return formCancelled{}
	}
	if runErr != nil {
		return fmt.Errorf("terminal prompt: %w", runErr)
	}
	return nil
}

func (s *formSession) filter(output io.Writer, config formInput) func(tea.Model, tea.Msg) tea.Msg {
	return func(_ tea.Model, msg tea.Msg) tea.Msg {
		filtered, err := formWindowSize(output, msg)
		if size, ok := filtered.(tea.WindowSizeMsg); ok && config.resize != nil && err == nil && promptOutputTerminal(output) {
			err = config.resize(size.Width, size.Height)
		}
		if err != nil {
			s.resizeErr = err
			s.cancel()
			return nil
		}
		switch m := filtered.(type) {
		case tea.QuitMsg, tea.InterruptMsg:
			s.finishInput()
		case tea.KeyPressMsg:
			if config.onKey != nil {
				config.onKey(m)
			}
			if m.String() == "enter" && config.canSubmit != nil && !config.canSubmit() {
				s.handoff.reject()
			}
		}
		return filtered
	}
}

func cleanupError(err, cleanup error) error {
	if cleanup == nil {
		return err
	}
	return errors.Join(err, cleanup)
}

func (p terminalRenderer) configureForm(form *huh.Form, session *formSession, config formInput) {
	options := []tea.ProgramOption{tea.WithInput(session.input), tea.WithOutput(session.output), tea.WithoutSignalHandler(), tea.WithFilter(session.filter(p.Output, config))}
	profile := colorprofile.ANSI
	if p.NoColor {
		profile = colorprofile.Ascii
	}
	options = append(options, tea.WithColorProfile(profile))
	form.WithTheme(huh.ThemeFunc(semanticHuhTheme)).WithAccessible(false).WithInput(session.input).WithOutput(session.output).WithKeyMap(promptKeyMap()).WithProgramOptions(options...).WithViewHook(func(v tea.View) tea.View { v.ReportFocus = false; return v })
}

// A user gesture may become a clean cancellation only while it is the sole
// outcome. Joining any cleanup failure changes the concrete error type.
type formCancelled struct{}

func (formCancelled) Error() string { return ErrCancelled.Error() }
func (formCancelled) Unwrap() error { return ErrCancelled }
