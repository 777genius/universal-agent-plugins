// Package terminaltheme owns human presentation colors; writers are never rewritten.
package terminaltheme

import (
	"fmt"
	"image/color"
	"io"
	"os"

	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

type Policy struct {
	Mode     string
	Explicit bool
}

func (p *Policy) Set(value string) error {
	if value != "auto" && value != "never" && value != "always" {
		return fmt.Errorf("--color must be auto, never, or always")
	}
	if p.Explicit && p.Mode != value {
		return fmt.Errorf("contradictory explicit color flags")
	}
	p.Mode, p.Explicit = value, true
	return nil
}
func (p *Policy) String() string {
	if p.Mode == "" {
		return "auto"
	}
	return p.Mode
}
func (*Policy) Type() string { return "auto|never|always" }

type NoColorFlag struct{ Policy *Policy }

func (f NoColorFlag) Set(v string) error {
	if v != "true" {
		return fmt.Errorf("--no-color only accepts true; use --color=auto to enable detection")
	}
	return f.Policy.Set("never")
}
func (f NoColorFlag) String() string {
	if f.Policy.Explicit && f.Policy.Mode == "never" {
		return "true"
	}
	return "false"
}
func (f NoColorFlag) Type() string { return "bool" }
func (p Policy) Enabled(tty bool, terminal, noColor, format string) bool {
	if format == "json" || p.Mode == "never" {
		return false
	}
	if !p.Explicit && noColor != "" {
		return false
	}
	if p.Mode == "always" {
		return true
	}
	return tty && terminal != "dumb"
}
func IsTerminal(w io.Writer) bool {
	f, ok := Unwrap(w).(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

type writer struct {
	io.Writer
	policy *Policy
	format *string
}

func Wrap(w io.Writer, p *Policy, format *string) io.Writer { return &writer{w, p, format} }
func Unwrap(w io.Writer) io.Writer {
	if v, ok := w.(*writer); ok {
		return v.Writer
	}
	return w
}
func For(w io.Writer) Theme {
	if v, ok := w.(interface{ SemanticTheme() Theme }); ok {
		return v.SemanticTheme()
	}
	return Theme{}
}
func (w *writer) SemanticTheme() Theme {
	return Theme{Enabled: w.policy.Enabled(IsTerminal(w.Writer), os.Getenv("TERM"), os.Getenv("NO_COLOR"), *w.format)}
}

type Role int

const (
	Success Role = iota
	Warning
	Error
	Label
	Muted
)

type Theme struct{ Enabled bool }

func (t Theme) Text(role Role, text string) string {
	if !t.Enabled || text == "" {
		return text
	}
	return ansi.NewStyle().ForegroundColor(Color(role)).Styled(text)
}

// Color shares the semantic palette with the existing Huh style adapter.
func Color(role Role) color.Color {
	return ansi.BasicColor([]uint8{2, 3, 1, 6, 8}[role])
}
