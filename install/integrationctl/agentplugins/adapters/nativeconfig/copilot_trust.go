package nativeconfig

import (
	"bytes"
	"fmt"
	"path/filepath"

	"github.com/tailscale/hujson"
)

// CopilotTrustProject appends only the exact selected project to Copilot's
// trustedFolders. It preserves JSONC comments and unrelated values. The caller
// authorizes the project/profile and owns the exact-file write transaction.
// A zero-length body represents an absent configuration, not an empty file.
func CopilotTrustProject(body []byte, root string) ([]byte, bool, error) {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || filepath.Dir(root) == root {
		return nil, false, fmt.Errorf("copilot trust requires a clean absolute project below the filesystem root")
	}
	source := body
	if len(source) == 0 {
		source = []byte("{}\n")
	}
	doc, err := parseDocument(source, true)
	if err != nil {
		return nil, false, err
	}
	member, _ := objectMember(doc.root, "trustedFolders")
	if member == nil {
		value, valueErr := jsonValue([]string{root})
		if valueErr != nil {
			return nil, false, valueErr
		}
		appendMember(doc.root, "trustedFolders", value)
	} else {
		folders, ok := member.Value.Value.(*hujson.Array)
		if !ok {
			return nil, false, fmt.Errorf("%w: copilot trustedFolders must be an array of strings", ErrMalformed)
		}
		trusted := false
		for _, element := range folders.Elements {
			literal, ok := element.Value.(hujson.Literal)
			if !ok || literal.Kind() != '"' {
				return nil, false, fmt.Errorf("%w: copilot trustedFolders must contain only strings", ErrMalformed)
			}
			trusted = trusted || literal.String() == root
		}
		if trusted {
			return bytes.Clone(body), true, nil
		}
		folders.Elements = append(folders.Elements, hujson.Value{Value: hujson.String(root)})
	}
	next, err := doc.render()
	return next, false, err
}
