//go:build !linux

package grok

import (
	"fmt"
	"os"
	"strings"
)

func (recovery EnablementRecovery) pendingNames() ([]string, error) {
	entries, err := os.ReadDir(recovery.OperationsDir)
	if os.IsNotExist(err) || recovery.OperationsDir == "" {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), enablementRecordPrefix) || strings.HasPrefix(entry.Name(), enablementStagePrefix) {
			return nil, fmt.Errorf("grok native operation recovery is not qualified on this platform")
		}
	}
	return nil, nil
}
