//go:build !linux

package grok

import "fmt"

func (recovery EnablementRecovery) publishRecord(string, []byte) error {
	return fmt.Errorf("grok project enablement journal is not qualified on this platform")
}

func (recovery EnablementRecovery) readRecordBytes(string) ([]byte, error) {
	return nil, fmt.Errorf("grok project enablement journal is not qualified on this platform")
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
