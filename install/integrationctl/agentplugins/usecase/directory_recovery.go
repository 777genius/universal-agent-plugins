package usecase

import (
	"context"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// An unresolved directory journal owns its recovery copies. Do not discard
// staged bytes or newly prepared data while its commit/rollback is ambiguous.
func (service Service) directoryRecoveryPending(operationID string) bool {
	if service.Paths == nil {
		return true
	}
	if err := service.Paths.ValidateLeafID(operationID); err != nil {
		return true
	}
	_, err := os.Lstat(filepath.Join(service.Kernel.Directory.JournalDir, operationID+".json"))
	return !os.IsNotExist(err)
}

func (service Service) discardSettledDelivery(operationID string, delivery domain.StagedDelivery) {
	if !service.directoryRecoveryPending(operationID) {
		_ = service.Stager.Discard(context.Background(), delivery)
	}
}
