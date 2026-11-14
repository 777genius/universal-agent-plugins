package gemini

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/authoredpath"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func (a Adapter) syncManagedLocalSource(ctx context.Context, manifest domain.IntegrationManifest, sourceRoot string) (string, error) {
	root := filepath.Clean(sourceRoot)
	managedRoot := a.managedSourceRoot(manifest.IntegrationID)
	if err := a.validateManagedSourceRoot(manifest.IntegrationID, managedRoot); err != nil {
		return "", domain.NewError(domain.ErrMutationApply, "validate Gemini managed source root", err)
	}
	tmpRoot, err := a.prepareManagedSourceTempRoot(managedRoot, manifest.IntegrationID)
	if err != nil {
		return "", err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(tmpRoot)
		}
	}()
	if err := a.materializeManagedSource(ctx, manifest, root, tmpRoot); err != nil {
		return "", err
	}
	if err := replaceGeminiPath(tmpRoot, managedRoot, "replace Gemini managed source root", "activate Gemini managed source root"); err != nil {
		return "", err
	}
	cleanup = false
	return managedRoot, nil
}

func (a Adapter) prepareManagedSourceTempRoot(managedRoot, integrationID string) (string, error) {
	parent := filepath.Dir(managedRoot)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", domain.NewError(domain.ErrMutationApply, "prepare Gemini managed materialization root", err)
	}
	tmpRoot, err := os.MkdirTemp(parent, integrationID+".tmp-*")
	if err != nil {
		return "", domain.NewError(domain.ErrMutationApply, "create Gemini materialization temp root", err)
	}
	return tmpRoot, nil
}

func (a Adapter) materializeManagedSource(ctx context.Context, manifest domain.IntegrationManifest, root, tmpRoot string) error {
	if authoredpath.HasManifest(root) {
		return a.materializeAuthoredGeminiSource(ctx, manifest, root, tmpRoot)
	}
	if fileExists(filepath.Join(root, "gemini-extension.json")) {
		return a.copyNativeGeminiPackage(root, tmpRoot)
	}
	return a.materializeAuthoredGeminiSource(ctx, manifest, root, tmpRoot)
}

func (a Adapter) activateManagedLocalInstall(managedRoot, name string) error {
	if strings.TrimSpace(managedRoot) == "" {
		return domain.NewError(domain.ErrMutationApply, "Gemini local projection requires managed source root", nil)
	}
	extensionDir := a.extensionDir(name)
	if err := a.validateExtensionDir(name, extensionDir); err != nil {
		return domain.NewError(domain.ErrMutationApply, "validate Gemini extension dir", err)
	}
	tmpRoot, err := a.prepareExtensionTempRoot(extensionDir, name)
	if err != nil {
		return err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(tmpRoot)
		}
	}()
	if err := a.populateManagedLocalInstall(tmpRoot, extensionDir, managedRoot); err != nil {
		return err
	}
	if err := replaceGeminiPath(tmpRoot, extensionDir, "replace Gemini extension dir", "activate Gemini extension dir"); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func (a Adapter) prepareExtensionTempRoot(extensionDir, name string) (string, error) {
	parent := filepath.Dir(extensionDir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", domain.NewError(domain.ErrMutationApply, "prepare Gemini extension dir parent", err)
	}
	tmpRoot, err := os.MkdirTemp(parent, name+".tmp-*")
	if err != nil {
		return "", domain.NewError(domain.ErrMutationApply, "create Gemini extension temp root", err)
	}
	return tmpRoot, nil
}

func (a Adapter) populateManagedLocalInstall(tmpRoot, extensionDir, managedRoot string) error {
	if err := copyDirIfExists(managedRoot, tmpRoot); err != nil {
		return domain.NewError(domain.ErrMutationApply, "copy Gemini managed source into extension dir", err)
	}
	if err := writeGeminiInstallMetadata(filepath.Join(tmpRoot, ".gemini-extension-install.json"), managedRoot); err != nil {
		return err
	}
	existingEnv := filepath.Join(extensionDir, ".env")
	if fileExists(existingEnv) {
		if err := copyFile(existingEnv, filepath.Join(tmpRoot, ".env")); err != nil {
			return domain.NewError(domain.ErrMutationApply, "preserve Gemini local .env", err)
		}
	}
	return nil
}

func replaceGeminiPath(tmpRoot, activePath, removeMessage, activateMessage string) error {
	if err := os.RemoveAll(activePath); err != nil && !os.IsNotExist(err) {
		return domain.NewError(domain.ErrMutationApply, removeMessage, err)
	}
	if err := os.Rename(tmpRoot, activePath); err != nil {
		return domain.NewError(domain.ErrMutationApply, activateMessage, err)
	}
	return nil
}

func writeGeminiInstallMetadata(path string, source string) error {
	doc := map[string]any{
		"source": strings.TrimSpace(source),
		"type":   "link",
	}
	body, err := marshalJSON(doc)
	if err != nil {
		return domain.NewError(domain.ErrMutationApply, "marshal Gemini install metadata", err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return domain.NewError(domain.ErrMutationApply, "write Gemini install metadata", err)
	}
	return nil
}
