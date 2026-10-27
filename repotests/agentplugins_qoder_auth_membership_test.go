//go:build linux

package pluginkitairepo_test

import (
	"errors"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// Only names are enumerated. Unknown entries are never opened or read.
func qoderMetadataAuthNames(directory *os.File) error {
	names, err := directory.Readdirnames(3)
	if (err != nil && !errors.Is(err, io.EOF)) || len(names) != 2 {
		return qoderMetadataRefused()
	}
	if !((names[0] == "machine_id" && names[1] == ".credential-transaction") ||
		(names[1] == "machine_id" && names[0] == ".credential-transaction")) {
		return qoderMetadataRefused()
	}
	extra, err := directory.Readdirnames(1)
	if len(extra) != 0 || !errors.Is(err, io.EOF) {
		return qoderMetadataRefused()
	}
	return nil
}

func qoderMetadataAuthLinked(profile, auth *os.File, expected os.FileInfo) bool {
	held, err := auth.Stat()
	var linked unix.Stat_t
	return err == nil && projectNativeRuntimeStableStat(expected.Sys(), held.Sys()) &&
		unix.Fstatat(int(profile.Fd()), ".auth", &linked, unix.AT_SYMLINK_NOFOLLOW) == nil &&
		qoderMetadataLinked(expected, &linked)
}

func qoderMetadataAuthMembers(profile, auth *os.File, expected os.FileInfo) (err error) {
	before, err := auth.Stat()
	if err != nil || !qoderMetadataSame(expected, before, true) || before.Mode() != os.ModeDir|0700 || !qoderMetadataAuthLinked(profile, auth, before) {
		return qoderMetadataRefused()
	}
	// The held auth handle is O_PATH. Open a fresh readable descriptor relative
	// to it so every check starts at offset zero without resolving a pathname.
	fd, err := unix.Openat(int(auth.Fd()), ".", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return qoderMetadataRefused()
	}
	directory := os.NewFile(uintptr(fd), "qoder-held-auth-membership")
	defer func() { err = errors.Join(err, directory.Close()) }()
	opened, err := directory.Stat()
	if err != nil || !projectNativeRuntimeStableStat(before.Sys(), opened.Sys()) {
		return qoderMetadataRefused()
	}
	if err := qoderMetadataAuthNames(directory); err != nil {
		return err
	}
	after, err := directory.Stat()
	if err != nil || !projectNativeRuntimeStableStat(before.Sys(), after.Sys()) || !qoderMetadataAuthLinked(profile, auth, before) {
		return qoderMetadataRefused()
	}
	return nil
}
