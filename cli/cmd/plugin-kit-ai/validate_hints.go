package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
	"github.com/777genius/plugin-kit-ai/cli/internal/validate"
)

func geminiValidateWarningHints(report validate.Report) []string {
	if !reportTouchesGemini(report) {
		return nil
	}
	seen := map[string]struct{}{}
	var hints []string
	for _, warning := range report.Warnings {
		switch warning.Kind {
		case validate.WarningGeminiDirNameMismatch:
			appendHint(&hints, seen, "rename the extension directory to match plugin.yaml name before running gemini extensions link .")
		case validate.WarningGeminiPolicyIgnored:
			appendHint(&hints, seen, "Gemini extension-tier policies ignore allow/yolo; keep only documented extension policy keys in targets/gemini/policies/*.toml.")
		}
	}
	return hints
}

func geminiValidateFailureHints(report validate.Report) []string {
	if !reportTouchesGemini(report) {
		return nil
	}
	seen := map[string]struct{}{}
	var hints []string
	for _, failure := range report.Failures {
		switch failure.Kind {
		case validate.FailureEntrypointMismatch, validate.FailureGeneratedContractInvalid:
			if strings.Contains(failure.Path, "hooks/hooks.json") || strings.Contains(failure.Message, "hooks/hooks.json") || strings.Contains(strings.ToLower(failure.Message), "generated artifact drift") {
				appendHint(&hints, seen, "rerun plugin-kit-ai generate . to regenerate Gemini hooks/hooks.json from "+pluginmodel.SourceDirName+"/launcher.yaml, then rerun plugin-kit-ai validate . --platform gemini --strict.")
			}
		case validate.FailureLauncherInvalid, validate.FailureRuntimeTargetMissing:
			appendHint(&hints, seen, "for the Gemini Go runtime lane keep "+pluginmodel.SourceDirName+"/launcher.yaml entrypoint, the built binary under bin/, and generated hooks/hooks.json aligned before rerunning validate.")
		}
	}
	if len(hints) > 0 {
		appendHint(&hints, seen, "after validate is green, run make test-gemini-runtime, relink the extension with gemini extensions link ., then use make test-gemini-runtime-live when you need real CLI evidence.")
	}
	return hints
}

func geminiValidateSuccessHints(root string, report validate.Report) []string {
	if !strings.EqualFold(strings.TrimSpace(report.Platform), "gemini") {
		return nil
	}
	launcherPath := filepath.Join(root, pluginmodel.SourceDirName, "launcher.yaml")
	if _, err := os.Stat(launcherPath); err != nil {
		return nil
	}
	return []string{
		"Gemini Go runtime is validate-clean; run make test-gemini-runtime before relinking the extension.",
		"relink the extension with gemini extensions link . before checking the runtime path in a real Gemini CLI session.",
		"use make test-gemini-runtime-live when you need real CLI evidence after the repo-local runtime gate is green.",
	}
}

func reportTouchesGemini(report validate.Report) bool {
	if strings.Contains(strings.ToLower(report.Platform), "gemini") {
		return true
	}
	for _, failure := range report.Failures {
		if strings.EqualFold(strings.TrimSpace(failure.Target), "gemini") || strings.Contains(strings.ToLower(failure.Message), "gemini") {
			return true
		}
	}
	for _, warning := range report.Warnings {
		if strings.Contains(strings.ToLower(warning.Message), "gemini") {
			return true
		}
	}
	return false
}

func appendHint(dst *[]string, seen map[string]struct{}, hint string) {
	if _, ok := seen[hint]; ok {
		return
	}
	seen[hint] = struct{}{}
	*dst = append(*dst, hint)
}
