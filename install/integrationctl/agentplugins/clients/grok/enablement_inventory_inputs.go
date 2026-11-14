package grok

import (
	"errors"
	"os"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

var errEnablementInventoryInputs = errors.New("grok readonly inventory input closure is unsupported or changed")

// This operation-local lease borrows the already locked profile. It owns only
// the additional source handles, never the config transaction or its flock.
type enablementInventoryInputs struct {
	inputs    legacyports.ReadonlyDuplexInputs
	profile   *enablementProfile
	parents   []directoryidentity.Authority
	pins      []enablementInventoryPin
	files     []*os.File
	remaining int
	closed    bool
}

type enablementInventoryPin struct {
	path   string
	file   *os.File
	info   os.FileInfo
	parent directoryidentity.Authority
	digest [32]byte
}

func (lease *enablementInventoryInputs) close() error {
	if lease == nil || lease.closed {
		return nil
	}
	lease.closed = true
	var result error
	for _, file := range lease.files {
		result = errors.Join(result, file.Close())
	}
	lease.files = nil
	return result
}
