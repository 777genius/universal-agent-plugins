package providers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
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
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := pathpolicy.RequireContainedChild(manager.Base, receipt.Locator); err != nil {
		return fmt.Errorf("unsafe PLUGIN_DATA receipt: %w", err)
	}
	existing, err := manager.readOwned(receipt.Locator)
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
	if err := manager.ValidateData(ctx, receipt); err != nil {
		return err
	}
	return os.RemoveAll(receipt.Locator)
}

func (manager PluginDataManager) readOwned(locator string) (domain.DataReceipt, error) {
	info, err := os.Lstat(locator)
	if err != nil {
		return domain.DataReceipt{}, fmt.Errorf("inspect PLUGIN_DATA: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return domain.DataReceipt{}, fmt.Errorf("PLUGIN_DATA locator is not an owned directory")
	}
	body, err := os.ReadFile(filepath.Join(locator, dataOwnershipMarker))
	if err != nil {
		return domain.DataReceipt{}, fmt.Errorf("read PLUGIN_DATA ownership marker: %w", err)
	}
	var receipt domain.DataReceipt
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&receipt); err != nil {
		return domain.DataReceipt{}, fmt.Errorf("decode PLUGIN_DATA ownership marker: %w", err)
	}
	if receipt.State != domain.DataReceiptOwned || receipt.OwnershipDigest != dataOwnershipDigest(receipt.DataReceiptID, locator) {
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
