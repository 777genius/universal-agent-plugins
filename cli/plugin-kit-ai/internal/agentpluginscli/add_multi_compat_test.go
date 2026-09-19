package agentpluginscli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type failingLoadStore struct {
	err error
}

func (store failingLoadStore) Load() (domain.StateFileV2, error) {
	return domain.StateFileV2{}, store.err
}

func (store failingLoadStore) Save(domain.StateFileV2) error {
	return nil
}

func TestAddGroupCompatibilityChecksSurfacesStateLoadError(t *testing.T) {
	t.Parallel()
	_, err := addGroupCompatibilityChecks(
		context.Background(),
		App{StateStore: failingLoadStore{err: errors.New("state load failed")}},
		&options{},
		loadedPackage{},
		nil,
		nil,
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "state load failed") {
		t.Fatalf("load error = %v", err)
	}
}
