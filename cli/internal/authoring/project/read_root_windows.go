package project

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packageview"
)

// Adapt only ordinary relative authoring roots to the native reader contract.
// Source components must reach the handle walker in order, including reparse/..
// and missing/..; Abs, Join and Clean would erase the evidence it must inspect.
func readRoot(name string) (string, error) {
	if name == "" || filepath.VolumeName(name) != "" || strings.HasPrefix(name, `/`) || strings.HasPrefix(name, `\`) || strings.Contains(name, ":") {
		return name, nil // leave namespaces and drive-relative spellings to the gate
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", &packageview.Error{Code: "root_unreadable"}
	}
	return prefixReadRoot(cwd, name), nil
}

func prefixReadRoot(cwd, name string) string {
	// Getwd at a drive root already ends in a separator. Do not introduce an
	// empty leading component into the native walk of the drive's remainder.
	if !strings.HasSuffix(cwd, `\`) && !strings.HasSuffix(cwd, `/`) {
		cwd += `\`
	}
	return cwd + name
}
