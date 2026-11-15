package transaction

import (
	"crypto/sha256"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// ComparableStateDigest uses the persisted comparison used by durable state
// decisions without normalizing lifecycle values or receipt content.
func ComparableStateDigest(state domain.StateFileV2) (string, error) {
	body, err := marshalComparableState(state)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("sha256:%x", sha256.Sum256(body)), nil
}
