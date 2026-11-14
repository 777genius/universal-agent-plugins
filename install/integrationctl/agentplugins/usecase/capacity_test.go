package usecase

import (
	"errors"
	"math"
	"testing"
)

func TestCheckedCombinedCapacity(t *testing.T) {
	tests := []struct {
		name    string
		left    int
		right   int
		want    int
		wantErr bool
	}{
		{name: "ordinary", left: 2, right: 3, want: 5},
		{name: "exact maximum", left: math.MaxInt - 1, right: 1, want: math.MaxInt},
		{name: "overflow", left: math.MaxInt, right: 1, wantErr: true},
		{name: "negative left", left: -1, right: 0, wantErr: true},
		{name: "negative right", left: 0, right: -1, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := checkedCombinedCapacity(test.left, test.right)
			if test.wantErr {
				if !errors.Is(err, errCombinedCapacityOverflow) {
					t.Fatalf("error = %v, want %v", err, errCombinedCapacityOverflow)
				}
				return
			}
			if err != nil {
				t.Fatalf("checked capacity: %v", err)
			}
			if got != test.want {
				t.Fatalf("capacity = %d, want %d", got, test.want)
			}
		})
	}
}
