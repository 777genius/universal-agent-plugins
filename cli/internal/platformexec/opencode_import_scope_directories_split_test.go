package platformexec

import "testing"

func TestOpenCodeSkillDirectoryImportKeepsOnlySkillMarkdown(t *testing.T) {
	t.Parallel()
	spec := openCodeSkillDirectoryImport(opencodeScopeConfig{})
	if spec.keep == nil {
		t.Fatal("expected keep filter")
	}
	if !spec.keep("demo/SKILL.md") {
		t.Fatal("expected SKILL.md to be kept")
	}
	if spec.keep("demo/README.md") {
		t.Fatal("expected README.md to be filtered out")
	}
}
