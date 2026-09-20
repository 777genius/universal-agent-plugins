package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
)

func renderInspectTargetsSection(out *strings.Builder, report pluginmanifest.Inspection) {
	for _, target := range report.Targets {
		_, _ = fmt.Fprintf(out, "- %s: class=%s production=%s runtime=%s native=%s managed=%s\n",
			target.Target,
			target.TargetClass,
			target.ProductionClass,
			target.RuntimeContract,
			strings.Join(target.TargetNativeKinds, ","),
			strings.Join(target.ManagedArtifacts, ","),
		)
		if docs := inspectTargetDocs(target); len(docs) > 0 {
			_, _ = fmt.Fprintf(out, "  docs=%s\n", strings.Join(docs, ","))
		}
		if len(target.UnsupportedKinds) > 0 {
			_, _ = fmt.Fprintf(out, "  unsupported=%s\n", strings.Join(target.UnsupportedKinds, ","))
		}
		if len(target.NativeSurfaces) > 0 {
			var tiers []string
			for _, surface := range target.NativeSurfaces {
				tiers = append(tiers, surface.Kind+"="+surface.Tier)
			}
			_, _ = fmt.Fprintf(out, "  surfaces=%s\n", strings.Join(tiers, ","))
		}
		for _, advice := range inspectTargetAdvice(report, target) {
			_, _ = fmt.Fprintf(out, "  %s\n", advice)
		}
	}
}

func inspectTargetDocs(target pluginmanifest.InspectTarget) []string {
	if len(target.NativeDocPaths) == 0 {
		return nil
	}
	var docs []string
	for _, kind := range target.TargetNativeKinds {
		if path := strings.TrimSpace(target.NativeDocPaths[kind]); path != "" {
			docs = append(docs, kind+"="+path)
		}
	}
	var remainingKinds []string
	for kind := range target.NativeDocPaths {
		remainingKinds = append(remainingKinds, kind)
	}
	slices.Sort(remainingKinds)
	for _, kind := range remainingKinds {
		path := target.NativeDocPaths[kind]
		if strings.TrimSpace(path) == "" || containsInspectDoc(docs, kind) {
			continue
		}
		docs = append(docs, kind+"="+path)
	}
	return docs
}

func containsInspectDoc(items []string, kind string) bool {
	prefix := kind + "="
	for _, item := range items {
		if strings.HasPrefix(item, prefix) {
			return true
		}
	}
	return false
}

func inspectTargetAdvice(report pluginmanifest.Inspection, target pluginmanifest.InspectTarget) []string {
	if target.Target != "gemini" {
		return nil
	}
	if report.Launcher == nil {
		return []string{
			"next=generate --check + validate --strict keep the packaging lane honest; add --runtime go when you want the Gemini production-ready 9-hook runtime",
		}
	}
	return []string{
		"next=go mod tidy; go test ./...; plugin-kit-ai generate --check .; plugin-kit-ai validate . --platform gemini --strict; gemini extensions link .",
		"runtime_gate=make test-gemini-runtime",
		"live_runtime_gate=make test-gemini-runtime-live",
	}
}
