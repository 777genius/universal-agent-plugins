//go:build !darwin || !arm64

package packageview

import (
	"context"
	"os"
)

func (s *source) verifyBindings() error  { return nil }
func sameIdentity(a, b os.FileInfo) bool { return os.SameFile(a, b) }

func (s *source) sourceHooks(*captureHooks) {}

func openSourceContext(_ context.Context, name string) (*source, error) { return openSource(name) }

func (s *source) phaseContext(context.Context) {}
