package validate

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/sdk/platformmeta"
)

func validateTargetExtraDocs(root, target string, tc pluginmanifest.TargetComponents, report *Report) {
	profile, ok := platformmeta.Lookup(target)
	if !ok {
		return
	}
	for _, doc := range profile.NativeDocs {
		if doc.Role != platformmeta.NativeDocRoleExtra {
			continue
		}
		format := pluginmanifest.NativeDocFormatJSON
		if doc.Format == platformmeta.NativeDocTOML {
			format = pluginmanifest.NativeDocFormatTOML
		}
		label := target + " " + filepath.Base(doc.Path)
		validateTargetExtraDoc(root, target, tc.DocPath(doc.Kind), format, label, doc.ManagedKeys, report)
	}
}

func validateTargetExtraDoc(root, target, rel string, format pluginmanifest.NativeDocFormat, label string, managedPaths []string, report *Report) {
	if strings.TrimSpace(rel) == "" {
		return
	}
	doc, err := pluginmanifest.LoadNativeExtraDoc(root, rel, format)
	if err != nil {
		formatName := strings.ToUpper(string(format))
		report.Failures = append(report.Failures, Failure{
			Kind:    FailureManifestInvalid,
			Path:    rel,
			Target:  target,
			Message: fmt.Sprintf("%s %s is invalid %s: %v", label, rel, formatName, err),
		})
		return
	}
	if err := pluginmanifest.ValidateNativeExtraDocConflicts(doc, label, managedPaths); err != nil {
		report.Failures = append(report.Failures, Failure{
			Kind:    FailureManifestInvalid,
			Path:    rel,
			Target:  target,
			Message: err.Error(),
		})
	}
}
