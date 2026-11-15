package shared

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestComponentCompensationRetainsRecoveryAfterAuthorityLoss(t *testing.T) {
	mutation := componentNext(t, componentInstalled(t))
	valid := true
	refused := errors.New("original physical parent changed")
	mutation.Revalidate = func() error {
		if !valid {
			return refused
		}
		return nil
	}
	mutation.Kernel = nativeconfig.NewWithLockAcquirer(func(nativeconfig.Paths, nativeconfig.Codec) (func() error, error) {
		componentAssertBody(t, filepath.Join(mutation.Layout.SkillsRoot, "docs", "SKILL.md"), "inert version two\n")
		valid = false
		return nil, fmt.Errorf("native mutation refused after parent replacement")
	})
	err := mutation.Apply(t.Context())
	if !errors.Is(err, refused) {
		t.Fatal("mutation ignored the retained parent", err)
	}
	componentAssertBody(t, filepath.Join(mutation.Layout.SkillsRoot, "docs", "SKILL.md"), "inert version two\n")
	recovery := componentRecovery(t, mutation)
	componentAssertBody(t, filepath.Join(recovery, "old-docs", "SKILL.md"), "inert version one\n")
	var observed *NativeEffectError
	if errors.As(err, &observed) && observed.Effect == domain.NativeEffectUnchanged {
		t.Fatal("uncertain compensation claimed unchanged")
	}
	if _, err := os.Stat(recovery); err != nil {
		t.Fatal("uncertain recovery was deleted", err)
	}
}

func TestCommittedCleanupCannotOverrideUncertainAuthority(t *testing.T) {
	err := &NativeEffectError{Effect: domain.NativeEffectUncertain, Err: &nativeconfig.CommittedCleanupError{Err: errors.New("original authority changed before lock release")}}
	if CommittedNativeCleanup(&domain.ActivationOutcome{}, err) || CommittedNativeDeactivationCleanup(&domain.DeactivationOutcome{}, err) {
		t.Fatal("cleanup classification replaced uncertain authority with a committed effect")
	}
}
