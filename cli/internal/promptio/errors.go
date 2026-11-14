package promptio

import "errors"

// ErrUnavailable means no usable visible prompt terminal is available.
var ErrUnavailable = errors.New("prompt input/output unavailable")
