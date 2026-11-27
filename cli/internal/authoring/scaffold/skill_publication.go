package scaffold

import (
	"context"
	"errors"
	"os"
)

// skillPublication retains the existing source CAS, stage identities and exact
// generated tree while a Windows parent reader temporarily delays publication.
// Every native attempt checks this same authority; no failed open is retried.
type skillPublication struct {
	ctx                              context.Context
	precondition                     func() error
	source, parent, owned, payload   *os.Root
	reservation, stage               string
	reserved, stageInfo, payloadInfo os.FileInfo
	files                            []File
}

func (p skillPublication) check() error {
	if err := verifyTree(p.ctx, p.payload, p.files); err != nil {
		return err
	}
	if err := p.precondition(); err != nil {
		return err
	}
	now, err := p.source.Lstat(p.reservation)
	if err != nil || !os.SameFile(p.reserved, now) {
		return errors.New("skill reservation ownership changed")
	}
	now, err = p.parent.Lstat(p.stage)
	if err != nil || !os.SameFile(p.stageInfo, now) {
		return errors.New("stage ownership changed")
	}
	now, err = p.owned.Lstat("payload")
	if err != nil || !now.IsDir() || now.Mode()&os.ModeSymlink != 0 || !os.SameFile(p.payloadInfo, now) {
		return errors.New("payload ownership changed")
	}
	return p.ctx.Err()
}
