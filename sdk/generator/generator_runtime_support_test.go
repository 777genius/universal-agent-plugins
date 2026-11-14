package generator

import (
	"strings"
	"testing"
)

func TestRenderSupportIndexIncludesAllPlatforms(t *testing.T) {
	t.Parallel()

	body := renderSupportIndex()
	for _, want := range []string{
		"claudeSupportEntries",
		"geminiSupportEntries",
		"codexSupportEntries",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("renderSupportIndex() missing %q:\n%s", want, body)
		}
	}
}

func TestSupportBucketFuncName(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"claude": "claudeSupportEntries",
		"gemini": "geminiSupportEntries",
		"codex":  "codexSupportEntries",
	}
	for platform, want := range cases {
		if got := supportBucketFuncName(platform); got != want {
			t.Fatalf("supportBucketFuncName(%q) = %q, want %q", platform, got, want)
		}
	}
}

func TestRenderSupportBucket_UsesPlatformSpecificFunctionName(t *testing.T) {
	t.Parallel()

	m, err := loadModel()
	if err != nil {
		t.Fatal(err)
	}
	body := renderSupportBucket(m, "claude")
	if !strings.Contains(body, "func claudeSupportEntries() []runtime.SupportEntry") {
		t.Fatalf("renderSupportBucket() missing function name:\n%s", body)
	}
	if !strings.Contains(body, `Event: "Stop"`) {
		t.Fatalf("renderSupportBucket() missing expected Claude event:\n%s", body)
	}
}
