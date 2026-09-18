package pathpolicy_test

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports/contracttest"
)

func TestPolicySatisfiesThePathPolicyContract(t *testing.T) {
	t.Parallel()
	contracttest.RunPathPolicy(t, pathpolicy.Policy{})
}
