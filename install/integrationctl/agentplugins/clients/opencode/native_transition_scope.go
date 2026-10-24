package opencode

import (
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Project scope is stored binding authority, never a heuristic over package
// paths. Legacy user bindings retain the existing receipt-root fallback.
func transitionBindingLayout(binding domain.ClientBinding) (openCodeNativeLayout, error) {
	root := storedTransitionRoot(binding)
	if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return openCodeNativeLayout{}, fmt.Errorf("opencode stored native root is unavailable")
	}
	layout := userOpenCodeLayout(root)
	if binding.Scope == string(domain.ScopeProject) {
		if _, err := domain.NewTargetContext(domain.ScopeProject, root, ""); err != nil {
			return openCodeNativeLayout{}, err
		}
		layout = projectOpenCodeLayout(root)
	} else if binding.Scope != "" && binding.Scope != string(domain.ScopeUser) {
		return openCodeNativeLayout{}, fmt.Errorf("unsupported opencode binding scope")
	}
	return layout, layout.validate()
}

func recordOpenCodeLayout(record transitionRecord) (openCodeNativeLayout, error) {
	_, binding, err := transitionBinding(record.OldState, record.Identity)
	if err != nil {
		return openCodeNativeLayout{}, err
	}
	return transitionBindingLayout(binding)
}

func (t NativeTransitions) selectedBindingLayout(binding domain.ClientBinding) (openCodeNativeLayout, error) {
	layout, err := transitionBindingLayout(binding)
	if err != nil {
		return layout, err
	}
	if layout.Scope == domain.ScopeProject {
		if t.ProjectRoot == "" || t.ProjectRoot != layout.Root {
			return layout, fmt.Errorf("opencode transition differs from the selected project")
		}
	} else if t.ProjectRoot != "" {
		return layout, fmt.Errorf("opencode user transition is outside the selected project")
	}
	return layout, nil
}

func (t NativeTransitions) validateRecordContext(record transitionRecord) error {
	current, err := t.State.Load()
	if err != nil {
		return err
	}
	_, binding, err := transitionBinding(current, record.Identity)
	if err != nil {
		return err
	}
	selected, err := t.selectedBindingLayout(binding)
	if err != nil {
		return err
	}
	bound, err := recordOpenCodeLayout(record)
	if err != nil {
		return err
	}
	if selected != bound {
		return fmt.Errorf("opencode transition scope or native layout changed")
	}
	return nil
}
