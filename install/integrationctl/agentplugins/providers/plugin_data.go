package providers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const dataOwnershipMarker = ".agentplugins-data-owner.json"

type PluginDataManager struct {
	Base string
	Now  func() time.Time
	// RunNPM is a test seam; production uses the bounded npm ci runner.
	RunNPM func(context.Context, string, string, bool) error
}

func (manager PluginDataManager) EnsureData(ctx context.Context, installationID, physicalBackend, scope string) (domain.DataReceipt, bool, error) {
	if err := ctx.Err(); err != nil {
		return domain.DataReceipt{}, false, err
	}
	if strings.TrimSpace(manager.Base) == "" {
		return domain.DataReceipt{}, false, fmt.Errorf("plugin data base is required")
	}
	receiptID := dataReceiptID(installationID, physicalBackend, scope)
	locator := filepath.Join(manager.Base, physicalBackend)
	if err := pathpolicy.RequireContainedChild(manager.Base, locator); err != nil {
		return domain.DataReceipt{}, false, err
	}
	now := time.Now().UTC()
	if manager.Now != nil {
		now = manager.Now().UTC()
	}
	receipt := domain.DataReceipt{DataReceiptID: receiptID, PhysicalBackend: physicalBackend, Scope: scope,
		Locator: locator, OwnershipDigest: dataOwnershipDigest(receiptID, locator), State: domain.DataReceiptOwned,
		CreatedAt: now.Format(time.RFC3339Nano), UpdatedAt: now.Format(time.RFC3339Nano)}
	if _, err := os.Lstat(locator); err == nil {
		existing, err := manager.readOwned(locator)
		if err != nil {
			return domain.DataReceipt{}, false, err
		}
		if existing.DataReceiptID != receipt.DataReceiptID || existing.OwnershipDigest != receipt.OwnershipDigest || existing.PhysicalBackend != physicalBackend || existing.Scope != scope {
			return domain.DataReceipt{}, false, fmt.Errorf("PLUGIN_DATA ownership marker does not match requested physical backend")
		}
		return existing, false, nil
	} else if !os.IsNotExist(err) {
		return domain.DataReceipt{}, false, err
	}
	if err := os.MkdirAll(locator, 0o700); err != nil {
		return domain.DataReceipt{}, false, fmt.Errorf("create PLUGIN_DATA: %w", err)
	}
	body, err := json.Marshal(receipt)
	if err != nil {
		return domain.DataReceipt{}, false, err
	}
	if err := atomicfile.Write(filepath.Join(locator, dataOwnershipMarker), append(body, '\n'), 0o600); err != nil {
		_ = os.Remove(locator)
		return domain.DataReceipt{}, false, err
	}
	return receipt, true, nil
}

func (manager PluginDataManager) ValidateData(ctx context.Context, receipt domain.DataReceipt) error {
	return manager.ValidateDataAt(ctx, receipt, receipt.Locator)
}

// ValidateDataAt reads a moved physical directory while keeping the original
// logical locator in both the marker and the caller receipt. The ownership
// digest binds that locator, not the temporary backup name.
func (manager PluginDataManager) ValidateDataAt(ctx context.Context, receipt domain.DataReceipt, physicalPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := pathpolicy.RequireContainedChild(manager.Base, receipt.Locator); err != nil {
		return fmt.Errorf("unsafe PLUGIN_DATA receipt: %w", err)
	}
	if err := pathpolicy.RequireContainedChild(manager.Base, physicalPath); err != nil {
		return fmt.Errorf("unsafe PLUGIN_DATA physical path: %w", err)
	}
	existing, err := manager.readOwnedAt(physicalPath, receipt.Locator)
	if err != nil {
		return err
	}
	if existing.DataReceiptID != receipt.DataReceiptID || existing.OwnershipDigest != receipt.OwnershipDigest ||
		existing.PhysicalBackend != receipt.PhysicalBackend || existing.Scope != receipt.Scope {
		return fmt.Errorf("PLUGIN_DATA ownership receipt is stale")
	}
	return nil
}

func (manager PluginDataManager) PurgeData(ctx context.Context, receipt domain.DataReceipt) error {
	swap := dirswap.Manager{JournalDir: filepath.Join(manager.Base, ".agentplugins-data-operations")}
	r, err := swap.Apply(ctx, dirswap.Input{
		OperationID: "data-purge-" + rand.Text(), ClientBindingID: receipt.DataReceiptID, Sequence: 1,
		OwnedBase: manager.Base, ActivePath: receipt.Locator, Remove: true,
		VerifyActive: func(ctx context.Context, path string) error { return manager.ValidateDataAt(ctx, receipt, path) },
	})
	if err != nil {
		if r.OperationID != "" {
			if rollbackErr := swap.Rollback(context.Background(), r); rollbackErr != nil {
				return fmt.Errorf("%w; rollback failed: %w", err, rollbackErr)
			}
		}
		return err
	}
	if err := swap.Commit(ctx, r); err != nil {
		return err
	}
	// Remove only an empty journal directory; other in-flight purges retain it.
	_ = os.Remove(swap.JournalDir)
	return nil
}

func (manager PluginDataManager) readOwned(locator string) (domain.DataReceipt, error) {
	return manager.readOwnedAt(locator, locator)
}

func (manager PluginDataManager) readOwnedAt(physicalPath, logicalLocator string) (domain.DataReceipt, error) {
	info, err := os.Lstat(physicalPath)
	if err != nil {
		return domain.DataReceipt{}, fmt.Errorf("inspect PLUGIN_DATA: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return domain.DataReceipt{}, fmt.Errorf("PLUGIN_DATA locator is not an owned directory")
	}
	markerPath := filepath.Join(physicalPath, dataOwnershipMarker)
	if err := pathpolicy.RequireContainedChild(physicalPath, markerPath); err != nil {
		return domain.DataReceipt{}, err
	}
	marker, err := os.Lstat(markerPath)
	if err != nil {
		return domain.DataReceipt{}, err
	}
	if !marker.Mode().IsRegular() {
		return domain.DataReceipt{}, fmt.Errorf("PLUGIN_DATA ownership marker must be a regular file")
	}
	body, err := os.ReadFile(markerPath)
	if err != nil {
		return domain.DataReceipt{}, fmt.Errorf("read PLUGIN_DATA ownership marker: %w", err)
	}
	var receipt domain.DataReceipt
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&receipt); err != nil {
		return domain.DataReceipt{}, fmt.Errorf("decode PLUGIN_DATA ownership marker: %w", err)
	}
	if receipt.State != domain.DataReceiptOwned || receipt.Locator != logicalLocator || receipt.OwnershipDigest != dataOwnershipDigest(receipt.DataReceiptID, logicalLocator) {
		return domain.DataReceipt{}, fmt.Errorf("invalid PLUGIN_DATA ownership marker")
	}
	return receipt, nil
}

func dataReceiptID(installationID, physicalBackend, scope string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{installationID, physicalBackend, scope}, "\x00")))
	return "data_" + hex.EncodeToString(sum[:12])
}

func dataOwnershipDigest(receiptID, locator string) string {
	sum := sha256.Sum256([]byte("agentplugins-plugin-data-v1\x00" + receiptID + "\x00" + filepath.Clean(locator)))
	return "sha256:" + hex.EncodeToString(sum[:])
}
