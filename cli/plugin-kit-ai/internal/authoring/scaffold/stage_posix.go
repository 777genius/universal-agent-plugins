//go:build !windows

package scaffold

import "os"

func makePrivateStage(parent *os.Root, name string) error {
	return parent.Mkdir(name, 0700)
}
