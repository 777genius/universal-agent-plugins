package shared

import (
	"errors"
	"math"
)

// ErrCombinedCapacityOverflow is returned instead of allocating a slice whose
// requested capacity cannot be represented. Native mutations size their buffers
// from two persisted object lists, so an overflow has to fail the operation
// before any filesystem effect, not panic halfway through it.
var ErrCombinedCapacityOverflow = errors.New("combined capacity exceeds platform limits")

// CombinedCapacityFunc is the test seam for the capacity check.
type CombinedCapacityFunc func(left, right int) (int, error)

// CheckedCombinedCapacity adds two lengths and rejects a negative input or an
// int overflow.
func CheckedCombinedCapacity(left, right int) (int, error) {
	if left < 0 || right < 0 {
		return 0, ErrCombinedCapacityOverflow
	}
	if left > math.MaxInt-right {
		return 0, ErrCombinedCapacityOverflow
	}
	return left + right, nil
}
