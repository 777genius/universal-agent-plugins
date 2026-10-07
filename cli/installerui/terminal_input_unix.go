//go:build !windows

package installerui

import "os"

func validatePromptInput(*os.File) error { return nil }
