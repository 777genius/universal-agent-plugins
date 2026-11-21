package agentpluginscli

import (
	"context"
	"strings"
)

func inspectSavedAddRoot(ctx context.Context, app App, opts *options, source string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if app.Installer == nil {
		return nil
	}
	// Preserve recovery and manifest/lock refusals before ownership advice. This
	// snapshot reads intent only; source declarations are verified by saved add.
	snapshot, err := snapshotEnvironmentContext(ctx, app, false)
	if err != nil {
		return err
	}
	key := savedDirectSourceKey(source)
	if match := exactGitPattern.FindStringSubmatch(source); match != nil {
		key = "github:" + strings.ToLower(match[1]) + ":" + match[3]
	}
	if canPreviewSavedAddOwnership(snapshot, opts.alias, key) {
		return checkSavedAddOwnership(ctx, app, key, opts.alias)
	}
	// Unsaved selectors need actual acquired distribution identity. Product-level
	// retained state cannot prove which distribution this operation will select.
	return nil
}

// Existing alias and saved-selector decisions belong to the normal resolver.
// Defer their ownership diagnosis rather than masking a more specific refusal.
func canPreviewSavedAddOwnership(snapshot environmentSnapshot, alias, key string) bool {
	if alias != "" || key == "" {
		return false
	}
	for _, pin := range snapshot.Lock.Packages {
		if pin.SourceKey == key {
			return false
		}
	}
	return true
}
