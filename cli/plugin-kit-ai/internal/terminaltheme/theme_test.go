package terminaltheme

import "testing"

func TestPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, mode, term, noColor, format string
		explicit, tty, want               bool
	}{
		{name: "auto terminal", tty: true, want: true},
		{name: "redirect", tty: false},
		{name: "dumb", tty: true, term: "dumb"},
		{name: "NO_COLOR nonempty zero", tty: true, noColor: "0"},
		{name: "NO_COLOR whitespace", tty: true, noColor: " "},
		{name: "explicit auto overrides env", mode: "auto", explicit: true, tty: true, noColor: "1", want: true},
		{name: "explicit auto still detects", mode: "auto", explicit: true, noColor: "1"},
		{name: "always forces", mode: "always", explicit: true, term: "dumb", noColor: "1", want: true},
		{name: "never", mode: "never", explicit: true, tty: true},
		{name: "JSON overrides always", mode: "always", explicit: true, tty: true, format: "json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := (Policy{Mode: tc.mode, Explicit: tc.explicit}).Enabled(tc.tty, tc.term, tc.noColor, tc.format); got != tc.want {
				t.Fatalf("enabled=%v", got)
			}
		})
	}
}
func TestRoles(t *testing.T) {
	for role, code := range map[Role]string{Success: "32", Warning: "33", Error: "31", Label: "36", Muted: "90"} {
		if got := (Theme{Enabled: true}).Text(role, "fixture"); got != "\x1b["+code+"mfixture\x1b[m" {
			t.Fatalf("role %d: %q", role, got)
		}
		if got := (Theme{}).Text(role, "fixture"); got != "fixture" {
			t.Fatal(got)
		}
	}
}
