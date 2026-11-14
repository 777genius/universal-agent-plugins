//go:build linux

package grok

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

func (recovery EnablementRecovery) appendDecision(record enablementRecord, digest string) (resultErr error) {
	tail, err := decisionTail(record, digest)
	if err != nil {
		return err
	}
	journal, err := recovery.openRecordJournal(record)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, journal.root.Close()) }()
	name := enablementRecordPrefix + record.OperationID
	file, body, info, err := journal.readFile(name, true)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	if enablementDigest(body) != record.rawDigest {
		return fmt.Errorf("grok native decision record changed before append")
	}
	if err := journal.point("before_decision_write"); err != nil {
		return err
	}
	if err := journal.joinFile(name, file, info); err != nil {
		return err
	}
	remaining := tail[len(record.footer):]
	if count, err := file.WriteAt(remaining, int64(len(body))); err != nil || count != len(remaining) {
		return errors.Join(err, io.ErrShortWrite)
	}
	info, err = journalFileStat(file)
	if err != nil {
		return err
	}
	if err := journal.point("decision_written"); err != nil {
		return err
	}
	if err := journal.joinFile(name, file, info); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := journal.point("decision_synced"); err != nil {
		return err
	}
	want := append(bytes.Clone(body[:record.headerBytes]), tail...)
	readback := make([]byte, len(want))
	if _, err := file.ReadAt(readback, 0); err != nil || !bytes.Equal(readback, want) {
		return errors.Join(err, fmt.Errorf("grok native decision readback differs"))
	}
	return journal.joinFile(name, file, info)
}

// A rollback removes its non-committed footer before clearing the attempt.
// Therefore a crash after the state restore can always clean a plain record.
func (recovery EnablementRecovery) clearDecision(record enablementRecord) (result enablementRecord, resultErr error) {
	if err := record.revalidateParent(); err != nil {
		return record, err
	}
	if len(record.footer) == 0 {
		return record, nil
	}
	journal, err := recovery.openRecordJournal(record)
	if err != nil {
		return record, err
	}
	defer func() { resultErr = errors.Join(resultErr, journal.root.Close()) }()
	name := enablementRecordPrefix + record.OperationID
	file, body, info, err := journal.readFile(name, true)
	if err != nil {
		return record, err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	if enablementDigest(body) != record.rawDigest {
		return record, fmt.Errorf("grok native decision record changed before rollback")
	}
	if err := journal.joinFile(name, file, info); err != nil {
		return record, err
	}
	if err := file.Truncate(int64(record.headerBytes)); err != nil {
		return record, err
	}
	info, err = journalFileStat(file)
	if err != nil {
		return record, err
	}
	if err := journal.point("decision_cleared"); err != nil {
		return record, err
	}
	if err := journal.joinFile(name, file, info); err != nil {
		return record, err
	}
	if err := file.Sync(); err != nil {
		return record, err
	}
	if err := journal.joinFile(name, file, info); err != nil {
		return record, err
	}
	return recovery.decodeRecord(record.OperationID, body[:record.headerBytes])
}
