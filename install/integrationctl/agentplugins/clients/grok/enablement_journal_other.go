//go:build !linux

package grok

import (
	"errors"
	"fmt"
)

func (recovery EnablementRecovery) publishRecord(enablementRecord, []byte) error {
	return fmt.Errorf("grok project enablement journal is not qualified on this platform")
}

func (recovery EnablementRecovery) readRecordBytes(operationID string) ([]byte, error) {
	path, err := recovery.recordRoot(operationID)
	if err != nil {
		return nil, err
	}
	// Prepared-only removal needs the actual absence result. Existing records
	// remain unsupported; this anchored probe never reads a journal body.
	file, _, err := openProjectPath(path, false, nil)
	if err != nil {
		return nil, err
	}
	return nil, errors.Join(fmt.Errorf("grok project enablement journal is not qualified on this platform"), file.Close())
}

func (recovery EnablementRecovery) removeRecord(string, string) error {
	return fmt.Errorf("grok project enablement journal is not qualified on this platform")
}

func (recovery EnablementRecovery) recordNames() ([]string, error) { return nil, nil }

func (recovery EnablementRecovery) appendDecision(enablementRecord, string) error {
	return fmt.Errorf("grok native decision journal is not qualified on this platform")
}

func (recovery EnablementRecovery) clearDecision(record enablementRecord) (enablementRecord, error) {
	return record, fmt.Errorf("grok native decision journal is not qualified on this platform")
}
