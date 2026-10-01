package installerui

import (
	"context"
	"fmt"
	"io"
	"strings"
)

func (t *Terminal) selectPlain(ctx context.Context, req MultiSelectRequest) (Selection, error) {
	u, err := t.PlainUI()
	if err != nil {
		return Selection{}, err
	}
	for range 3 {
		if err := u.render(req.SelectRequest, true); err != nil {
			return Selection{}, err
		}
		if _, err := fmt.Fprint(u.out, "Use all/none, or q to cancel.\n> "); err != nil {
			return Selection{}, err
		}
		line, err := u.read(ctx)
		if err != nil {
			return Selection{}, err
		}
		if terminalCancel(line) {
			return Selection{Cancelled: true}, nil //nolint:misspell // Preserve the existing public cancellation API.
		}
		ids, err := terminalPlainIDs(line, req)
		if err == nil {
			result, e := terminalSelection(req, ids)
			if e == nil {
				return result, nil
			}
		}
		if _, e := fmt.Fprintln(u.out, "Invalid selection; choose a subset of the shown options."); e != nil {
			return Selection{}, e
		}
	}
	return Selection{}, ErrInvalidSelection
}

func terminalPlainIDs(line string, req MultiSelectRequest) ([]string, error) {
	line = strings.TrimSpace(line)
	switch strings.ToLower(line) {
	case "":
		return append([]string(nil), req.Defaults...), nil
	case "all":
		ids := make([]string, len(req.Options))
		for i, option := range req.Options {
			ids[i] = option.ID
		}
		return ids, nil
	case "none":
		return nil, nil
	}
	ids, err := parse(line, req.Options, true) // One comma parser, legacy grammar unchanged.
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidSelection, err)
	}
	return ids, nil
}

func terminalCancel(line string) bool {
	if isCancel(line) {
		return true
	}
	return strings.ContainsAny(line, "\x1b\x03\x04")
}

func (t *Terminal) confirmPlain(ctx context.Context, req ConfirmRequest, queued []byte) (Confirmation, error) {
	u, err := t.PlainUI()
	if err != nil {
		return Confirmation{}, err
	}
	if len(queued) > 0 {
		// The snapshot may decline/cancel, never accept or read the next owner's suffix.
		answer := "cancel\n"
		if queued[0] == '\r' {
			answer = "n\n"
		}
		u.in = strings.NewReader(answer)
	}
	// Add terminal control gestures without altering the legacy UI.Confirm grammar.
	read := u.readLine
	u.readLine = func(ctx context.Context, input io.Reader) (string, error) {
		line, err := read(ctx, input)
		if err == nil && terminalCancel(line) {
			line = "cancel"
		}
		return line, err
	}
	return u.Confirm(ctx, req)
}
