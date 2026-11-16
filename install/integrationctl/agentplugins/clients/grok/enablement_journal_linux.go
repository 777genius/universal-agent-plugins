//go:build linux

package grok

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
)

// The private journal is one extensionless file in the existing operation
// namespace. A stage is never native-effect authority. Publication is a single
// no-replace rename of a completely written, synced file in a held directory.
type enablementJournal struct {
	root   *os.File
	path   string
	hook   func(string) error
	parent directoryidentity.Authority
}

func (recovery EnablementRecovery) openJournal() (*enablementJournal, error) {
	root, _, err := openProjectPath(recovery.OperationsDir, true, nil)
	if err != nil {
		return nil, err
	}
	var info unix.Stat_t
	if err := unix.Fstat(int(root.Fd()), &info); err != nil || info.Mode&07777 != 0700 || int64(info.Uid) != int64(os.Geteuid()) {
		_ = root.Close()
		return nil, errors.Join(err, fmt.Errorf("grok operation namespace is not private and owned"))
	}
	return &enablementJournal{root: root, path: recovery.OperationsDir, hook: recovery.journalPoint}, nil
}

func (recovery EnablementRecovery) openRecordJournal(record enablementRecord) (*enablementJournal, error) {
	if err := recovery.validateRecord(record); err != nil {
		return nil, err
	}
	if err := record.revalidateParent(); err != nil {
		return nil, err
	}
	journal, err := recovery.openJournal()
	if err == nil {
		journal.parent = record.UserParentAuthority
	}
	return journal, err
}

func (journal *enablementJournal) point(name string) error {
	if journal.hook != nil {
		return journal.hook(name)
	}
	return nil
}

func (journal *enablementJournal) joinRoot() (resultErr error) {
	named, info, err := openProjectPath(journal.path, true, nil)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, named.Close()) }()
	held, err := journal.root.Stat()
	if err != nil || !os.SameFile(info, held) || info.Mode().Perm() != 0700 {
		return errors.Join(err, fmt.Errorf("grok operation namespace changed"))
	}
	if !journal.parent.IsZero() {
		return directoryidentity.Revalidate(context.Background(), journal.parent)
	}
	return nil
}

func journalFileStat(file *os.File) (unix.Stat_t, error) {
	var info unix.Stat_t
	if err := unix.Fstat(int(file.Fd()), &info); err != nil {
		return info, err
	}
	if info.Mode&unix.S_IFMT != unix.S_IFREG || info.Mode&07777 != 0600 ||
		int64(info.Uid) != int64(os.Geteuid()) || info.Nlink != 1 || info.Size < 0 || info.Size > enablementRecordLimit {
		return info, fmt.Errorf("grok operation record is not a bounded private ordinary file")
	}
	return info, nil
}

func (journal *enablementJournal) joinFile(name string, file *os.File, before unix.Stat_t) error {
	if err := journal.joinRoot(); err != nil {
		return err
	}
	held, err := journalFileStat(file)
	if err != nil {
		return err
	}
	var named unix.Stat_t
	if err := unix.Fstatat(int(journal.root.Fd()), name, &named, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return err
	}
	if !sameJournalFile(before, held) || !sameJournalFile(held, named) {
		return fmt.Errorf("grok operation record changed")
	}
	return nil
}

func sameJournalFile(left, right unix.Stat_t) bool {
	return left.Dev == right.Dev && left.Ino == right.Ino && left.Mode == right.Mode &&
		left.Uid == right.Uid && left.Gid == right.Gid && left.Nlink == right.Nlink &&
		left.Size == right.Size && left.Mtim == right.Mtim && left.Ctim == right.Ctim
}

func (journal *enablementJournal) openFile(name string, writable bool) (*os.File, unix.Stat_t, error) {
	flags := unix.O_RDONLY
	if writable {
		flags = unix.O_RDWR
	}
	fd, err := unix.Openat(int(journal.root.Fd()), name, flags|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, unix.Stat_t{}, err
	}
	file := os.NewFile(uintptr(fd), name)
	info, err := journalFileStat(file)
	if err == nil {
		err = journal.joinFile(name, file, info)
	}
	if err != nil {
		_ = file.Close()
		return nil, info, err
	}
	return file, info, nil
}

func (journal *enablementJournal) readFile(name string, writable bool) (*os.File, []byte, unix.Stat_t, error) {
	file, info, err := journal.openFile(name, writable)
	if err != nil {
		return nil, nil, info, err
	}
	body, err := io.ReadAll(io.LimitReader(file, enablementRecordLimit+1))
	if err == nil {
		err = journal.point("record_read")
	}
	if err == nil {
		err = journal.joinFile(name, file, info)
	}
	if err != nil || int64(len(body)) != info.Size {
		_ = file.Close()
		return nil, nil, info, errors.Join(err, fmt.Errorf("grok operation record read is not stable"))
	}
	return file, body, info, nil
}

func (recovery EnablementRecovery) readRecordBytes(operationID string) (result []byte, resultErr error) {
	if _, err := recovery.recordRoot(operationID); err != nil {
		return nil, err
	}
	journal, err := recovery.openJournal()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, journal.root.Close()) }()
	file, body, _, err := journal.readFile(enablementRecordPrefix+operationID, false)
	if err != nil {
		return nil, err
	}
	return body, file.Close()
}

func (recovery EnablementRecovery) publishRecord(record enablementRecord, body []byte) (resultErr error) {
	operationID := record.OperationID
	if _, err := recovery.recordRoot(operationID); err != nil {
		return err
	}
	journal, err := recovery.openRecordJournal(record)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, journal.root.Close()) }()
	if err := journal.joinRoot(); err != nil {
		return err
	}
	stageName := enablementStagePrefix + operationID
	fd, err := unix.Openat(int(journal.root.Fd()), stageName, unix.O_RDWR|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_CREAT|unix.O_EXCL, 0600)
	if err != nil {
		return err
	}
	file, published := os.NewFile(uintptr(fd), stageName), false
	defer func() {
		if !published {
			resultErr = errors.Join(resultErr, journal.discardStage(stageName, file))
		}
		resultErr = errors.Join(resultErr, file.Close())
	}()
	info, err := journal.writeStage(file, body)
	if err != nil {
		return err
	}
	if err := journal.joinFile(stageName, file, info); err != nil {
		return err
	}
	finalName := enablementRecordPrefix + operationID
	if err := unix.Renameat2(int(journal.root.Fd()), stageName, int(journal.root.Fd()), finalName, unix.RENAME_NOREPLACE); err != nil {
		return err
	}
	published = true
	info, err = journalFileStat(file) // rename legitimately changes ctime
	if err != nil {
		return err
	}
	if err := journal.point("record_published"); err != nil {
		return err
	}
	if err := journal.joinFile(finalName, file, info); err != nil {
		return err
	}
	return errors.Join(journal.root.Sync(), journal.point("publication_synced"))
}

func (journal *enablementJournal) writeStage(file *os.File, body []byte) (unix.Stat_t, error) {
	var empty unix.Stat_t
	if err := journal.point("stage_created"); err != nil {
		return empty, err
	}
	if err := journal.joinRoot(); err != nil {
		return empty, err
	}
	if count, err := file.Write(body); err != nil || count != len(body) {
		return empty, errors.Join(err, io.ErrShortWrite)
	}
	if err := journal.point("record_written"); err != nil {
		return empty, err
	}
	if err := journal.joinRoot(); err != nil {
		return empty, err
	}
	if err := file.Sync(); err != nil {
		return empty, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return empty, err
	}
	current, err := io.ReadAll(io.LimitReader(file, enablementRecordLimit+1))
	if err != nil || !bytes.Equal(current, body) {
		return empty, errors.Join(err, fmt.Errorf("grok operation stage changed during write"))
	}
	info, err := journalFileStat(file)
	if err != nil {
		return empty, err
	}
	return info, journal.point("record_synced")
}

func (journal *enablementJournal) discardStage(name string, file *os.File) error {
	info, err := journalFileStat(file)
	if err != nil {
		return err
	}
	return journal.unlinkFile(name, file, info)
}

func (journal *enablementJournal) unlinkFile(name string, file *os.File, info unix.Stat_t) error {
	if err := journal.joinFile(name, file, info); err != nil {
		return err
	}
	if err := unix.Unlinkat(int(journal.root.Fd()), name, 0); err != nil {
		return err
	}
	return journal.root.Sync()
}

func (recovery EnablementRecovery) removeRecord(operationID, digest string) (resultErr error) {
	if _, err := recovery.recordRoot(operationID); err != nil {
		return err
	}
	journal, err := recovery.openJournal()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, journal.root.Close()) }()
	name := enablementRecordPrefix + operationID
	file, body, info, err := journal.readFile(name, false)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	if enablementDigest(body) != digest {
		return fmt.Errorf("grok operation record differs from settled authority")
	}
	record, err := recovery.decodeRecord(operationID, body)
	if err != nil {
		return err
	}
	journal.parent = record.UserParentAuthority
	if err := journal.point("before_record_remove"); err != nil {
		return err
	}
	if err := journal.joinFile(name, file, info); err != nil {
		return err
	}
	if err := unix.Unlinkat(int(journal.root.Fd()), name, 0); err != nil {
		return err
	}
	if err := journal.point("record_removed"); err != nil {
		return err
	}
	return journal.root.Sync()
}

func (recovery EnablementRecovery) recordNames() (result []string, resultErr error) {
	journal, err := recovery.openJournal()
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, journal.root.Close()) }()
	names, err := journal.root.Readdirnames(4097)
	if err != nil && !errors.Is(err, io.EOF) || len(names) > 4096 {
		return nil, errors.Join(err, fmt.Errorf("grok operation inventory exceeds its bound"))
	}
	if err := journal.joinRoot(); err != nil {
		return nil, err
	}
	for _, name := range names {
		if !strings.HasPrefix(name, enablementStagePrefix) {
			continue
		}
		if err := pathpolicy.ValidateLeafID(strings.TrimPrefix(name, enablementStagePrefix)); err != nil {
			return nil, err
		}
		file, body, info, err := journal.readFile(name, false)
		if err != nil {
			return nil, err
		}
		if len(body) != 0 {
			var record enablementRecord
			record, err = recovery.decodeRecord(strings.TrimPrefix(name, enablementStagePrefix), body)
			journal.parent = record.UserParentAuthority
		} else {
			// An empty unpublished stage has no native-effect authority. Its
			// cleanup remains bound only to the held operation namespace.
			journal.parent = directoryidentity.Authority{}
		}
		if err == nil {
			err = journal.unlinkFile(name, file, info)
		}
		err = errors.Join(err, file.Close())
		if err != nil {
			return nil, err
		}
	}
	return names, nil
}
