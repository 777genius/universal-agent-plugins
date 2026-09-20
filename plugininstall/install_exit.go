package plugininstall

import (
	"context"
	"errors"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
)

// ExitCodeFromErr maps domain errors to shell exit codes; unknown returns 1.
func ExitCodeFromErr(err error) int {
	if err == nil {
		return 0
	}
	var de *domain.Error
	if errors.As(err, &de) {
		return int(de.Code)
	}
	if errors.Is(err, context.Canceled) {
		return 1
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return 3
	}
	return 1
}
