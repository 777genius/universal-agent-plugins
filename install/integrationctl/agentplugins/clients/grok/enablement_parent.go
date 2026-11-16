package grok

import (
	"context"
	"errors"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
)

// Capture the existing User directory before either prototype native exchange.
// This authority does not admit or recover auxiliary native startup writes.
func captureEnablementParent(ctx context.Context, root string) (result directoryidentity.Authority, resultErr error) {
	file, _, err := openProjectPath(root, true, nil)
	if err != nil {
		return result, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, file.Close())
		if resultErr != nil {
			result = directoryidentity.Authority{}
		}
	}()
	return directoryidentity.CaptureHeld(ctx, root, file)
}

func (record enablementRecord) validateParent() error {
	if record.UserParentAuthority.IsZero() || record.UserParentAuthority.Facts().CanonicalRoot != record.UserRoot {
		return fmt.Errorf("grok enablement lacks the original canonical User parent authority")
	}
	return nil
}

func (record enablementRecord) revalidateParent() error {
	if err := record.validateParent(); err != nil {
		return err
	}
	return directoryidentity.Revalidate(context.Background(), record.UserParentAuthority)
}

func (record enablementRecord) applyConfig(file *nativeconfig.ExactFile, body []byte) error {
	if err := record.revalidateParent(); err != nil {
		return err
	}
	if err := file.Apply(body); err != nil {
		return err
	}
	return record.revalidateParent()
}

func (record enablementRecord) rollbackConfig(file *nativeconfig.ExactFile) error {
	if err := record.revalidateParent(); err != nil {
		return err
	}
	if err := file.Rollback(); err != nil {
		return err
	}
	return record.revalidateParent()
}
