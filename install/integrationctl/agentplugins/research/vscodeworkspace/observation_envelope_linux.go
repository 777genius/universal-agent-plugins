//go:build linux

package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
)

func validateObservationEnvelope(m map[string]any) error {
	if m["collected"] == true {
		if _, present := m["stage_error"]; present {
			return fmt.Errorf("successful observer envelope contains stage_error")
		}
	}
	allowed := map[string]bool{}
	for _, key := range []string{"schema_version", "collected", "stage_error", "version", "app_root", "app_host", "observer", "folder", "trusted", "policy", "mcp", "skills", "native_project_qualified", "native_mcp_initialization_qualified", "native_runtime_qualified"} {
		allowed[key] = true
	}
	for key := range m {
		if !allowed[key] {
			return fmt.Errorf("unknown observer result field")
		}
	}
	if m["schema_version"] != json.Number("1") || m["native_project_qualified"] != false || m["native_mcp_initialization_qualified"] != false || m["native_runtime_qualified"] != false {
		return fmt.Errorf("observer schema/qualification differs")
	}
	return nil
}

func validateObservationContext(m map[string]any, base, folder string) error {
	if m["version"] != publisherVersion || m["app_root"] != filepath.Join(base, "publisher", publisherPrefix, "resources", "app") || m["app_host"] != "desktop" || m["trusted"] != false {
		return fmt.Errorf("publisher/context differs")
	}
	if err := metadataURI(m["folder"]); err != nil {
		return err
	}
	selected, ok := m["folder"].(map[string]any)
	if !ok || selected["scheme"] != "file" || selected["authority"] != "" || selected["path"] != folder || selected["value"] != "file://"+folder {
		return fmt.Errorf("selected native folder differs")
	}
	return nil
}

func validateObservationPolicy(m map[string]any) error {
	policy, policyErr := objectKeys(m["policy"], []string{"skills_enabled", "mcp_access", "mcp_autostart"}, nil)
	ok := policyErr == nil
	if !ok || policy["mcp_autostart"] != "never" {
		return fmt.Errorf("actual restrictive policy missing")
	}
	if _, ok = policy["skills_enabled"].(bool); !ok {
		return fmt.Errorf("unknown skills policy type")
	}
	if value := policy["mcp_access"]; value != "all" && value != "none" && value != "registry" {
		return fmt.Errorf("unknown MCP policy type")
	}
	return nil
}

func validateObserverIdentity(m map[string]any) error {
	observer, observerErr := objectKeys(m["observer"], []string{"id", "mode", "proposals"}, nil)
	ok := observerErr == nil
	if !ok || observer["id"] != "uap-research.workspace-observer" || observer["mode"] != json.Number("3") || !reflect.DeepEqual(observer["proposals"], []any{"mcpServerDefinitions", "chatPromptFiles"}) {
		return fmt.Errorf("observer identity/proposals differ")
	}
	return nil
}
