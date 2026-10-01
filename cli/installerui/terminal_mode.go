package installerui

import (
	"fmt"
	"os"
	"runtime"

	"golang.org/x/term"
)

func terminalMode(cfg TerminalConfig) (TerminalMode, error) {
	mode := cfg.Mode
	if mode == "" {
		mode = ModeAuto
	}
	if mode != ModeAuto && mode != ModePlain && mode != ModeRich {
		return "", fmt.Errorf("%w: unknown mode %q", ErrInvalidRequest, mode)
	}
	if cfg.Input == nil || cfg.Output == nil {
		return "", ErrUnavailable
	}
	for _, f := range []*os.File{cfg.Input, cfg.Output} {
		if _, err := f.Stat(); err != nil {
			return "", fmt.Errorf("prompt descriptor: %w", err)
		}
		if !term.IsTerminal(int(f.Fd())) {
			return "", ErrUnavailable
		}
	}
	switch runtime.GOOS {
	case "linux", "darwin", "windows", "freebsd", "openbsd", "netbsd", "dragonfly":
	default:
		return "", ErrUnavailable
	}
	if err := validatePromptInput(cfg.Input); err != nil {
		return "", err
	}
	width, height, err := term.GetSize(int(cfg.Output.Fd()))
	rich := (runtime.GOOS == "linux" || runtime.GOOS == "darwin") && os.Getenv("TERM") != "" && os.Getenv("TERM") != "dumb" && err == nil && width >= 40 && height >= 10
	if mode == ModeRich && !rich {
		return "", ErrUnavailable
	}
	if mode == ModePlain || !rich {
		return ModePlain, nil
	}
	return ModeRich, nil
}
