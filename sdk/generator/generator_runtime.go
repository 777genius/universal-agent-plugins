package generator

import (
	"fmt"
	"go/format"
)

func renderArtifacts(m model) []Artifact {
	artifacts := []Artifact{
		{Path: "sdk/internal/descriptors/gen/registry_gen.go", Content: mustGo(renderRegistry(m))},
		{Path: "sdk/internal/descriptors/gen/resolvers_gen.go", Content: mustGo(renderResolvers(m))},
		{Path: "sdk/internal/descriptors/gen/support_gen.go", Content: mustGo(renderSupportIndex())},
		{Path: "sdk/internal/descriptors/gen/support_gen_claude.go", Content: mustGo(renderSupportBucket(m, "claude"))},
		{Path: "sdk/internal/descriptors/gen/support_gen_gemini.go", Content: mustGo(renderSupportBucket(m, "gemini"))},
		{Path: "sdk/internal/descriptors/gen/support_gen_codex.go", Content: mustGo(renderSupportBucket(m, "codex"))},
		{Path: "sdk/internal/descriptors/gen/completeness_gen_test.go", Content: mustGo(renderCompletenessTest(m))},
		{Path: "cli/internal/scaffold/platforms_gen.go", Content: mustGo(renderScaffoldPlatforms(m))},
		{Path: "cli/internal/validate/rules_gen.go", Content: mustGo(renderValidateRules(m))},
		{Path: "docs/generated/support_matrix.md", Content: []byte(renderSupportMatrix(m))},
		{Path: "docs/generated/target_support_matrix.md", Content: []byte(renderTargetSupportMatrix(m))},
	}
	for _, p := range runtimeProfiles(m) {
		artifacts = append(artifacts, Artifact{
			Path:    fmt.Sprintf("sdk/%s/registrar_gen.go", p.PublicPackage),
			Content: mustGo(renderRegistrar(m, p)),
		})
	}
	return artifacts
}

func mustGo(src string) []byte {
	body, err := format.Source([]byte(src))
	if err != nil {
		panic(err)
	}
	return body
}
