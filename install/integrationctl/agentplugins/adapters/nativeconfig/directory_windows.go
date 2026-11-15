package nativeconfig

import "os"

func openRootNativeLock(root *os.Root, name string) (*os.File, error) {
	if info, err := root.Lstat(name); err == nil && !info.Mode().IsRegular() {
		return nil, ErrConcurrentChange
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	file, err := root.OpenFile(name, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	held, statErr := file.Stat()
	named, nameErr := root.Lstat(name)
	if statErr != nil || nameErr != nil || !named.Mode().IsRegular() || !os.SameFile(held, named) {
		_ = file.Close()
		return nil, ErrConcurrentChange
	}
	return file, nil
}

// Match the existing Windows atomic writer's directory durability contract.
func syncNativeDirectory(*os.Root) error { return nil }
