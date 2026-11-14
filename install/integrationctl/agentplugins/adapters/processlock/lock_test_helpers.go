package processlock

import "os"

func writeEmpty(path string) error {
	return os.WriteFile(path, nil, 0o600)
}

func symlink(target, link string) error {
	return os.Symlink(target, link)
}
