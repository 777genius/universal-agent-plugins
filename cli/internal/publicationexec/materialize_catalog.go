package publicationexec

import (
	"fmt"
	"strings"
)

func MergeCatalogArtifact(target string, existing, generated []byte) ([]byte, error) {
	return mergeCatalogDocument(target, existing, generated, catalogIdentityKeys(target)...)
}

func RemoveCatalogArtifact(target string, existing []byte, pluginName string) ([]byte, bool, error) {
	return removeCatalogArtifact(target, existing, pluginName)
}

type CatalogIssue struct {
	Code    string
	Path    string
	Message string
}

func DiagnoseCatalogArtifact(target string, existing, generated []byte, pluginName string) ([]CatalogIssue, error) {
	return diagnoseCatalogArtifact(target, existing, generated, pluginName)
}

func catalogIdentityKeys(target string) []string {
	switch strings.TrimSpace(target) {
	case "codex-package":
		return []string{"name", "interface"}
	case "claude":
		return []string{"name", "owner"}
	default:
		return nil
	}
}

func validateCatalogTarget(target string) error {
	if len(catalogIdentityKeys(target)) == 0 {
		return fmt.Errorf("local publication materialization supports only %q or %q", "codex-package", "claude")
	}
	return nil
}
