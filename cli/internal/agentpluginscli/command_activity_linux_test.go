//go:build linux

package agentpluginscli

import (
	"os"
	"testing"
)

func commandActivityPTY(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	return consentPTY(t)
}
