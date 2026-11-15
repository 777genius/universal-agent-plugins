package main

import (
	"context"
	"errors"
	"fmt"
	"testing"

	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
)

func TestSoleForcedCleanupPreservesEveryOtherFailure(t *testing.T) {
	marker := processadapter.ErrOrdinaryForcedCleanup
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"exact sentinel", marker, true},
		{"diagnostic wrapper", fmt.Errorf("original diagnostic: %w", errors.Join(marker)), true},
		{"arbitrary same text", errors.New(marker.Error()), false},
		{"joined unrelated failure", errors.Join(marker, errors.New("other failure")), false},
		{"joined cancellation", errors.Join(marker, context.Canceled), false},
		{"joined deadline", errors.Join(marker, context.DeadlineExceeded), false},
		{"repeated causes", errors.Join(marker, marker), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := soleForcedCleanup(tc.err); got != tc.want {
				t.Fatalf("classification=%v, want %v", got, tc.want)
			}
		})
	}
}
