//go:build (!linux && !darwin && !windows) || (linux && !amd64 && !arm64) || (darwin && !arm64) || (windows && !amd64 && !arm64)

package directoryidentity

import "os"

func platformScheme() string                               { return "unsupported" }
func openDirectory(_ *os.File, _ string) (*os.File, error) { return nil, ErrUnsupported }
func directoryFacts(_ *os.File) (string, string, error)    { return "", "", ErrUnsupported }
