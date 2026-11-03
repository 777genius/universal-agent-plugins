package agentpluginscli

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateUpdateResultCount(t *testing.T) {
	engineErr := errors.New("engine returned a partial update")
	for _, tc := range []struct {
		name          string
		got, selected int
		engineErr     error
		wantMismatch  bool
	}{
		{"complete", 2, 2, nil, false},
		{"empty selected", 0, 0, nil, false},
		{"short success", 1, 2, nil, true},
		{"empty success", 0, 2, nil, true},
		{"extra success", 3, 2, nil, true},
		{"short error", 1, 2, engineErr, false},
		{"empty error", 0, 2, engineErr, false},
		{"complete error", 2, 2, engineErr, false},
		{"extra error", 3, 2, engineErr, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateUpdateResultCount(tc.got, tc.selected, tc.engineErr)
			if tc.wantMismatch {
				if err == nil || !strings.Contains(err.Error(), "update engine returned") {
					t.Fatalf("malformed group result accepted: got %d of %d: %v", tc.got, tc.selected, err)
				}
				if tc.engineErr != nil && !errors.Is(err, tc.engineErr) {
					t.Fatalf("original error lost: %v", err)
				}
			} else if err != nil {
				t.Fatalf("valid group result rejected: %v", err)
			}
		})
	}
}
