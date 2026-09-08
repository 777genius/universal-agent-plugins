package terminalprompts

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/promptio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/charmbracelet/colorprofile"
	"github.com/muesli/cancelreader"
)

type HuhPrompter struct {
	Input   io.Reader
	Output  io.Writer
	NoColor bool
}

func (p HuhPrompter) SelectTargets(ctx context.Context, r prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
	if p.Input == nil || p.Output == nil {
		return prompt.TargetSelectionResult{}, prompt.ErrPromptUnavailable
	}
	if err := ctx.Err(); err != nil {
		return prompt.TargetSelectionResult{}, err
	}
	if err := prompt.ValidateRequest(r); err != nil {
		return prompt.TargetSelectionResult{}, err
	}
	ids := append([]domain.ClientID(nil), r.DefaultIDs...)
	choices := make([]huh.Option[domain.ClientID], 0, len(r.Choices))
	for _, c := range r.Choices {
		choices = append(choices, huh.NewOption(prompt.SafeText(c.Label)+" ("+prompt.SafeText(string(c.ID))+")", c.ID))
	}
	for _, label := range r.SkippedLabels {
		if err := promptio.WriteText(p.Output, "Skipped installed clients that this package cannot install together: "+prompt.SafeText(label)+"\n"); err != nil {
			return prompt.TargetSelectionResult{}, err
		}
	}
	field := huh.NewMultiSelect[domain.ClientID]().Title("Choose targets (all selected by default)").Options(choices...).Height(len(choices) + 2).Value(&ids).Filterable(false).Validate(func(v []domain.ClientID) error { _, err := prompt.ValidateSelection(r, v); return err })
	if err := p.run(ctx, huh.NewForm(huh.NewGroup(field)), func() bool { _, err := prompt.ValidateSelection(r, ids); return err == nil }); err != nil {
		return prompt.TargetSelectionResult{}, err
	}
	return prompt.ValidateSelection(r, ids)
}
func (p HuhPrompter) Confirm(ctx context.Context, r prompt.ConfirmationRequest) (prompt.ConfirmationResult, error) {
	if p.Input == nil || p.Output == nil {
		return prompt.ConfirmationResult{}, prompt.ErrPromptUnavailable
	}
	if err := ctx.Err(); err != nil {
		return prompt.ConfirmationResult{}, err
	}
	if err := promptio.WriteText(p.Output, fmt.Sprintf("%s (No by default; arrows/Space choose, Enter submits)\n", prompt.SafeText(r.Title))); err != nil {
		return prompt.ConfirmationResult{}, err
	}
	accepted := false
	for _, s := range r.Summary {
		if err := promptio.WriteText(p.Output, prompt.SafeText(s)+"\n"); err != nil {
			return prompt.ConfirmationResult{}, err
		}
	}
	field := huh.NewConfirm().Title(prompt.SafeText(r.Title)).Affirmative("Yes").Negative("No").Value(&accepted)
	if err := p.run(ctx, huh.NewForm(huh.NewGroup(field))); err != nil {
		return prompt.ConfirmationResult{}, err
	}
	return prompt.ConfirmationResult{Accepted: accepted}, nil
}
func promptKeyMap() *huh.KeyMap {
	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(key.WithKeys("ctrl+c", "esc", "ctrl+d"), key.WithHelp("esc/ctrl+c/ctrl+d", "cancel"))
	// Consent requires Enter, even when a printable key is held or pasted.
	km.Confirm.Toggle = key.NewBinding(key.WithKeys("left", "right", "space"), key.WithHelp("←/→/space", "choose"))
	km.Confirm.Next = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "submit"))
	km.MultiSelect.Next = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "submit"))
	km.Confirm.Accept = key.NewBinding(key.WithDisabled())
	km.Confirm.Reject = key.NewBinding(key.WithDisabled())
	return km
}
func (p HuhPrompter) run(ctx context.Context, form *huh.Form, canSubmit ...func() bool) (err error) {
	if err = ctx.Err(); err != nil {
		return err
	}
	if p.Input == nil || p.Output == nil {
		return prompt.ErrPromptUnavailable
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	writer := &formWriter{Writer: p.Output, cancel: cancel}
	var output io.Writer = writer
	if f, ok := p.Output.(*os.File); ok {
		output = &formFileWriter{formWriter: writer, file: f}
	}
	var owned cancelreader.CancelReader
	var source io.Reader = p.Input
	if f, ok := p.Input.(*os.File); ok {
		owned, err = cancelreader.NewReader(f)
		if err != nil {
			return fmt.Errorf("prepare terminal input: %w", err)
		}
		source = owned
	}
	handoff := newSubmissionReader(runCtx, source)
	defer handoff.finish()
	reader := &formReader{reader: handoff, cancel: cancel}
	var finishOnce sync.Once
	finishInput := func() {
		finishOnce.Do(func() {
			handoff.finish()
			if owned != nil {
				owned.Cancel()
			}
		})
	}
	callbackDone := make(chan struct{})
	stopCancel := context.AfterFunc(runCtx, func() { defer close(callbackDone); finishInput() })
	defer func() {
		if !stopCancel() {
			<-callbackDone
		}
		reader.stop(finishInput)
		if owned != nil {
			if closeErr := owned.Close(); err == nil && closeErr != nil {
				err = fmt.Errorf("close terminal input: %w", closeErr)
			}
		}
	}()
	var input io.Reader = reader
	if f, ok := p.Input.(*os.File); ok {
		input = &formFileReader{formReader: reader, file: f}
	}
	options := []tea.ProgramOption{tea.WithInput(input), tea.WithOutput(output), tea.WithoutSignalHandler(), tea.WithFilter(func(_ tea.Model, msg tea.Msg) tea.Msg {
		switch m := msg.(type) {
		case tea.QuitMsg, tea.InterruptMsg:
			finishInput()
		case tea.KeyPressMsg:
			if m.String() == "enter" && len(canSubmit) > 0 && !canSubmit[0]() {
				handoff.reject()
			}
		}
		return msg
	})}
	if p.NoColor {
		options = append(options, tea.WithColorProfile(colorprofile.Ascii))
	}
	form.WithAccessible(false).WithInput(input).WithOutput(output).WithKeyMap(promptKeyMap()).WithProgramOptions(options...).WithViewHook(func(v tea.View) tea.View { v.ReportFocus = false; return v })
	// Huh assumes a nonnil returned model even on initialization failure. Bubble
	// Tea retains its own panic cleanup; normalize a remaining adapter panic.
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("prompt initialization failed: %v", recovered)
		}
	}()
	err = form.RunWithContext(runCtx)
	if outputErr := writer.Err(); outputErr != nil {
		return fmt.Errorf("write prompt: %w", outputErr)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if inputErr := reader.Err(); inputErr != nil {
		if errors.Is(inputErr, io.EOF) {
			return prompt.ErrPromptInputClosed
		}
		return fmt.Errorf("read prompt: %w", inputErr)
	}
	if errors.Is(err, huh.ErrUserAborted) {
		return prompt.ErrPromptCanceled
	}
	if err != nil {
		return fmt.Errorf("terminal prompt: %w", err)
	}
	return nil
}

type formWriter struct {
	io.Writer
	mu     sync.Mutex
	err    error
	cancel context.CancelFunc
}

func (w *formWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	// Renderer writes run on Bubble Tea's goroutine, outside run's recovery.
	// Convert an injected writer panic into the usual cancellation path, while
	// allowing subsequent writes to restore the terminal. Never expose its value.
	defer func() {
		if recover() != nil {
			n, err = 0, errors.New("terminal output writer panicked")
		}
		if err != nil && w.err == nil {
			w.err = err
			w.cancel()
		}
	}()
	n, err = w.Writer.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	return n, err
}
func (w *formWriter) Err() error { w.mu.Lock(); defer w.mu.Unlock(); return w.err }

type formFileWriter struct {
	*formWriter
	file *os.File
}

func (w *formFileWriter) Fd() uintptr                { return w.file.Fd() }
func (w *formFileWriter) Read(p []byte) (int, error) { return w.file.Read(p) }
func (w *formFileWriter) Close() error               { return nil } // inherited output remains caller-owned

type formReader struct {
	reader  io.Reader
	mu      sync.Mutex
	err     error
	cancel  context.CancelFunc
	active  sync.WaitGroup
	stopped bool
}

func (r *formReader) Read(p []byte) (int, error) {
	r.mu.Lock()
	if r.stopped {
		r.mu.Unlock()
		return 0, cancelreader.ErrCanceled
	}
	r.active.Add(1)
	r.mu.Unlock()
	defer r.active.Done()
	n, err := r.reader.Read(p)
	if err == nil && n == 0 {
		err = io.ErrNoProgress
	}
	if err != nil && !errors.Is(err, cancelreader.ErrCanceled) {
		r.mu.Lock()
		if r.err == nil {
			r.err = err
		}
		r.mu.Unlock()
		r.cancel()
	}
	return n, err
}
func (r *formReader) Err() error { r.mu.Lock(); defer r.mu.Unlock(); return r.err }

type formFileReader struct {
	*formReader
	file *os.File
}

func (r *formFileReader) Fd() uintptr                 { return r.file.Fd() }
func (r *formFileReader) Write(p []byte) (int, error) { return r.file.Write(p) }
func (r *formFileReader) Close() error                { return nil }

// Omitting Name deliberately keeps the outer library on its fallback reader;
// our inner cancel reader owns wakeups, and stop joins every in-flight read.
func (r *formReader) stop(cancelRead func()) {
	r.mu.Lock()
	r.stopped = true
	r.mu.Unlock()
	cancelRead()
	r.active.Wait()
}
