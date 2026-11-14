//go:build !linux

package grok

import "fmt"

func lockNativeEnablement(string) (func() error, error) {
	return nil, fmt.Errorf("grok explicit plugin enablement is qualified only on Linux")
}
