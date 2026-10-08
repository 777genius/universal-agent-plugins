//go:build !windows

package opencode

import (
	"fmt"
	"os"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
)

func createPrivateTransitionRoot(parent string) (string, error) {
	return os.MkdirTemp(parent, ".agentplugins-native-")
}

func writePrivateTransitionJournal(path string, body []byte) error {
	return atomicfile.Write(path, body, 0600)
}

func validateTransitionPrivacy(_ string, info os.FileInfo, directory bool) error {
	want := os.FileMode(0600)
	if directory {
		want = 0700
	}
	if info.Mode().Perm() != want {
		return fmt.Errorf("transition permissions must be %04o", want)
	}
	return nil
}
