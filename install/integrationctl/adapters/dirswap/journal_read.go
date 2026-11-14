package dirswap

// Decode the original journal before validating its recorded publication scope.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
)

func (manager Manager) Load(operationID string) (Receipt, error) {
	if err := pathpolicy.ValidateLeafID(operationID); err != nil {
		return Receipt{}, fmt.Errorf("unsafe directory swap operation id: %w", err)
	}
	// Keep symlink resolution beneath the captured journal directory, and
	// validate the file that will actually be read rather than a path probe.
	file, err := os.OpenInRoot(manager.JournalDir, operationID+".json")
	if err != nil {
		return Receipt{}, err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return Receipt{}, err
	}
	if !info.Mode().IsRegular() {
		return Receipt{}, fmt.Errorf("directory swap journal %q must be a regular file", manager.journalPath(operationID))
	}
	body, err := io.ReadAll(file)
	if err != nil {
		return Receipt{}, err
	}
	var receipt Receipt
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&receipt); err != nil {
		return Receipt{}, fmt.Errorf("decode directory swap receipt %q: %w", manager.journalPath(operationID), err)
	}
	if receipt.OperationID != operationID {
		return Receipt{}, fmt.Errorf("directory swap journal %q contains a different operation id", manager.journalPath(operationID))
	}
	var validationErr error
	if receipt.SchemaVersion == 3 {
		if receipt.PublicationParentAuthority != nil {
			return Receipt{}, fmt.Errorf("legacy journal cannot claim publication ancestry")
		}
		validationErr = validateLegacyReceipt(receipt)
	} else {
		validationErr = manager.validateReceipt(receipt)
	}
	if err := validationErr; err != nil {
		return Receipt{}, manager.recoveryError(receipt, err)
	}
	return receipt, nil
}

func clonePublicationParent(parent *directoryidentity.Authority) *directoryidentity.Authority {
	if parent == nil {
		return nil
	}
	cloned := *parent
	return &cloned
}

func (r Receipt) validatePublicationParent() error {
	if r.PublicationParentAuthority == nil {
		return nil
	}
	if r.SchemaVersion < 4 || r.SchemaVersion > 7 || r.Operation != OperationSwap || r.HadActive || r.BackupIdentity != "" || r.BackupDigest != "" {
		return fmt.Errorf("publication parent requires an original fresh swap")
	}
	return validatePublicationParent(context.Background(), r.PublicationParentAuthority, r.OwnedBase, r.ActivePath, r.StagingPath)
}

func validatePublicationParent(ctx context.Context, parent *directoryidentity.Authority, base, active, staging string) error {
	if parent == nil {
		return nil
	}
	if parent.IsZero() || parent.Facts().CanonicalRoot != base {
		return fmt.Errorf("original publication parent is missing or misplaced")
	}
	for _, path := range []string{active, staging} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || filepath.Dir(path) != base {
			return fmt.Errorf("publication path differs from original parent")
		}
	}
	return directoryidentity.Revalidate(ctx, *parent)
}
