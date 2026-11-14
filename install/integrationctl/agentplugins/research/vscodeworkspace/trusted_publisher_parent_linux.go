//go:build linux

package main

import (
	"fmt"
	"os"
	"strconv"

	"golang.org/x/sys/unix"
)

// The fixed sudo -> unshare(no fork) -> bootstrap -> sealed exec chain retains
// this immediate parent. Unknown ancestry is refusal, never an ancestry search.
type publisherParent struct {
	file                 *os.File
	info                 os.FileInfo
	pid                  int
	uid                  uint32
	start, executableSHA string
	namespace            uint64
}

func holdPublisherParent(original, current uint64, caller int) (*publisherParent, error) {
	if original == 0 || current == 0 || original == current || caller <= 0 || caller > 1<<30 {
		return nil, fmt.Errorf("publisher parent namespace differs")
	}
	for _, path := range []string{"/usr/bin/sudo", "/usr/bin/unshare"} {
		if err := trustedSystem(path); err != nil {
			return nil, err
		}
	}
	pid := os.Getppid()
	root := "/proc/" + strconv.Itoa(pid)
	info, err := anchoredInfo(root)
	if err != nil {
		return nil, err
	}
	uid, err := statUID(info)
	if err != nil || uid > 1<<30 || (uid != 0 && uid != uint32(caller)) || pid <= 1 || !info.IsDir() {
		return nil, fmt.Errorf("publisher actual sudo parent owner refused")
	}
	fd, err := unix.Open(root, unix.O_PATH|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	p := &publisherParent{file: os.NewFile(uintptr(fd), root), info: info, pid: pid, uid: uid, namespace: original}
	p.start, err = publisherProcessStart(root, p.uid)
	if err == nil {
		p.executableSHA, err = hashFile("/usr/bin/sudo", 128<<20)
	}
	if err == nil {
		err = p.verify()
	}
	if err != nil {
		_ = p.file.Close()
		return nil, err
	}
	return p, nil
}
func (p *publisherParent) verify() error {
	if p == nil || os.Getppid() != p.pid {
		return fmt.Errorf("publisher sudo parent lifetime changed")
	}
	root := "/proc/" + strconv.Itoa(p.pid)
	held, err := p.file.Stat()
	linked, le := anchoredInfo(root)
	if err != nil || le != nil || !scannerNodeIdentity(p.info, held) || !scannerNodeIdentity(p.info, linked) {
		return fmt.Errorf("publisher held parent identity changed")
	}
	start, err := publisherProcessStart(root, p.uid)
	if err != nil || start != p.start {
		return fmt.Errorf("publisher parent start identity changed")
	}
	if err = trustedSystem("/usr/bin/sudo"); err != nil {
		return err
	}
	original, err := os.Stat("/usr/bin/sudo")
	actual, ae := os.Stat(root + "/exe")
	namespace, ne := os.Stat(root + "/ns/mnt")
	if err != nil || ae != nil || ne != nil || !os.SameFile(original, actual) {
		return fmt.Errorf("actual publisher parent is not fixed sudo")
	}
	identity, err := policyStat(namespace)
	digest, de := hashFile("/usr/bin/sudo", 128<<20)
	if err != nil || de != nil || identity.Inode != p.namespace || digest != p.executableSHA {
		return fmt.Errorf("actual sudo parent namespace/executable changed")
	}
	after, err := publisherProcessStart(root, p.uid)
	if err != nil || after != p.start {
		return fmt.Errorf("publisher parent changed during admission")
	}
	return nil
}
