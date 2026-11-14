package main

import (
	"errors"

	"github.com/777genius/plugin-kit-ai/cli/internal/exitx"
)

func publicationDoctorIssueErr(ready bool) error {
	if ready {
		return nil
	}
	return exitx.Wrap(errors.New("publication doctor found issues"), 1)
}
