package usecase

import (
	"errors"
	"math"
)

var errCombinedCapacityOverflow = errors.New("combined capacity exceeds platform limits")

func checkedCombinedCapacity(left, right int) (int, error) {
	if left < 0 || right < 0 {
		return 0, errCombinedCapacityOverflow
	}
	if left > math.MaxInt-right {
		return 0, errCombinedCapacityOverflow
	}
	return left + right, nil
}
