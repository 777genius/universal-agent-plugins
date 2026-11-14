//go:build linux

package grok

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

func (recovery EnablementRecovery) pendingNames() (result []string, resultErr error) {
	if recovery.OperationsDir == "" {
		return nil, nil
	}
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
	var pending []string
	for _, name := range names {
		if strings.HasPrefix(name, enablementStagePrefix) {
			return nil, fmt.Errorf("grok unpublished native operation requires retained recovery")
		}
		if strings.HasPrefix(name, enablementRecordPrefix) {
			pending = append(pending, name)
		}
	}
	return pending, nil
}
