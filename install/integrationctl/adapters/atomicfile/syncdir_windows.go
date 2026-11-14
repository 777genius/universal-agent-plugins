//go:build windows

package atomicfile

func syncParent(string) error {
	return nil
}
