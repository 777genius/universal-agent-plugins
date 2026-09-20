// Package installerui contains neutral terminal selection and confirmation
// primitives. Hosts provide product-specific options and interpret the result.
package installerui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
)

var (
	ErrUnavailable = errors.New("installer ui unavailable")
	ErrCancelled   = errors.New("installer ui cancelled")
)

type Option struct{ ID, Label string }

type SelectRequest struct {
	Title    string
	Options  []Option
	Defaults []string
}

type Selection struct {
	IDs       []string
	Accepted  bool
	Cancelled bool
}

type ConfirmRequest struct {
	Title   string
	Summary []string
	Default bool
}

type Confirmation struct {
	Accepted  bool
	Cancelled bool
}

type Config struct {
	Input  io.Reader
	Output io.Writer
	// ReadLine supplies optional platform-aware cancellation without prefetching.
	ReadLine func(context.Context, io.Reader) (string, error)
	// Style may add trusted formatting to already sanitized labels.
	Style func(string) string
}

type UI struct {
	in       io.Reader
	out      io.Writer
	readLine func(context.Context, io.Reader) (string, error)
	style    func(string) string
}

func New(cfg Config) (*UI, error) {
	if cfg.Input == nil || cfg.Output == nil {
		return nil, ErrUnavailable
	}
	return &UI{in: cfg.Input, out: checkedWriter{cfg.Output}, readLine: cfg.ReadLine, style: cfg.Style}, nil
}

func (u *UI) SelectOne(ctx context.Context, req SelectRequest) (Selection, error) {
	if err := ctx.Err(); err != nil {
		return Selection{}, err
	}
	if err := validate(req, false); err != nil {
		return Selection{}, err
	}
	if err := u.render(req, false); err != nil {
		return Selection{}, err
	}
	line, err := u.read(ctx)
	if err != nil {
		return Selection{}, err
	}
	if isCancel(line) {
		return Selection{Cancelled: true}, nil
	}
	if strings.TrimSpace(line) == "" && len(req.Defaults) != 0 {
		return Selection{IDs: []string{req.Defaults[0]}, Accepted: true}, nil
	}
	ids, err := parse(line, req.Options, false)
	if err != nil {
		return Selection{}, err
	}
	return Selection{IDs: ids, Accepted: true}, nil
}

func (u *UI) SelectMany(ctx context.Context, req SelectRequest) (Selection, error) {
	if err := ctx.Err(); err != nil {
		return Selection{}, err
	}
	if err := validate(req, true); err != nil {
		return Selection{}, err
	}
	if err := u.render(req, true); err != nil {
		return Selection{}, err
	}
	line, err := u.read(ctx)
	if err != nil {
		return Selection{}, err
	}
	if isCancel(line) {
		return Selection{Cancelled: true}, nil
	}
	if strings.TrimSpace(line) == "" {
		return Selection{IDs: unique(req.Defaults), Accepted: true}, nil
	}
	ids, err := parse(line, req.Options, true)
	if err != nil {
		return Selection{}, err
	}
	return Selection{IDs: ids, Accepted: true}, nil
}

func (u *UI) Confirm(ctx context.Context, req ConfirmRequest) (Confirmation, error) {
	if err := ctx.Err(); err != nil {
		return Confirmation{}, err
	}
	if strings.TrimSpace(req.Title) == "" {
		return Confirmation{}, errors.New("confirmation title is required")
	}
	for _, line := range req.Summary {
		if _, err := fmt.Fprintln(u.out, clean(line)); err != nil {
			return Confirmation{}, err
		}
	}
	hint := "y/N"
	if req.Default {
		hint = "Y/n"
	}
	if _, err := fmt.Fprintf(u.out, "%s [%s] ", u.label(req.Title), hint); err != nil {
		return Confirmation{}, err
	}
	line, err := u.read(ctx)
	if err != nil {
		return Confirmation{}, err
	}
	if isCancel(line) {
		return Confirmation{Cancelled: true}, nil
	}
	line = strings.ToLower(strings.TrimSpace(line))
	if line == "" {
		return Confirmation{Accepted: req.Default}, nil
	}
	return Confirmation{Accepted: line == "y" || line == "yes"}, nil
}

func (u *UI) read(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if u.readLine != nil {
		return u.readLine(ctx, u.in)
	}
	var line strings.Builder
	var b [1]byte
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, err := u.in.Read(b[:])
		if e := ctx.Err(); e != nil {
			return "", e
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		if n > 0 {
			if b[0] == '\n' {
				return strings.TrimSpace(line.String()), nil
			}
			if line.Len() >= 4096 {
				return "", errors.New("prompt answer exceeds 4096 bytes")
			}
			line.WriteByte(b[0])
		}
		if err != nil {
			return "", err
		}
		if n == 0 {
			return "", io.ErrNoProgress
		}
	}
}

func (u *UI) render(req SelectRequest, many bool) error {
	if _, err := fmt.Fprintln(u.out, u.label(req.Title)); err != nil {
		return err
	}
	for i, option := range req.Options {
		mark := " "
		if has(req.Defaults, option.ID) {
			mark = "*"
		}
		if _, err := fmt.Fprintf(u.out, "  %d. [%s] %s\n", i+1, mark, clean(option.Label)); err != nil {
			return err
		}
	}
	if many {
		_, err := fmt.Fprint(u.out, u.label("Choose one or more by number or id, comma-separated [Enter keeps defaults]: "))
		return err
	}
	_, err := fmt.Fprint(u.out, u.label("Choose one by number or id [Enter keeps the default]: "))
	return err
}

func validate(req SelectRequest, many bool) error {
	if strings.TrimSpace(req.Title) == "" || len(req.Options) == 0 {
		return errors.New("selection title and options are required")
	}
	seen := make(map[string]struct{}, len(req.Options))
	for _, option := range req.Options {
		if strings.TrimSpace(option.ID) == "" || strings.TrimSpace(option.Label) == "" {
			return errors.New("selection options need non-empty id and label")
		}
		if _, ok := seen[option.ID]; ok {
			return fmt.Errorf("duplicate selection id %q", option.ID)
		}
		seen[option.ID] = struct{}{}
	}
	if !many && len(req.Defaults) > 1 {
		return errors.New("single selection accepts at most one default")
	}
	for _, id := range req.Defaults {
		if _, ok := seen[id]; !ok {
			return fmt.Errorf("default selection %q is not an option", id)
		}
	}
	return nil
}

func parse(line string, options []Option, many bool) ([]string, error) {
	byID := make(map[string]struct{}, len(options))
	for _, option := range options {
		byID[option.ID] = struct{}{}
	}
	ids := make([]string, 0, 1)
	seen := make(map[string]struct{})
	for _, raw := range strings.Split(line, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return nil, errors.New("empty selection")
		}
		id := raw
		if n, err := strconv.Atoi(raw); err == nil {
			if n < 1 || n > len(options) {
				return nil, fmt.Errorf("selection number %d is out of range", n)
			}
			id = options[n-1].ID
		}
		if _, ok := byID[id]; !ok {
			return nil, fmt.Errorf("unknown selection %q", raw)
		}
		if _, ok := seen[id]; ok {
			return nil, fmt.Errorf("duplicate selection %q", id)
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if !many && len(ids) != 1 {
		return nil, errors.New("single selection accepts one value")
	}
	return ids, nil
}

func isCancel(line string) bool {
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "q", "quit", "cancel", "c":
		return true
	default:
		return false
	}
}

func has(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func unique(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func clean(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return -1
		}
		return r
	}, value)
}

func (u *UI) label(text string) string {
	text = clean(text)
	if u.style != nil {
		return u.style(text)
	}
	return text
}

type checkedWriter struct{ io.Writer }

func (w checkedWriter) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	return n, err
}
