package app

import (
	"context"
	"os/exec"
)

var testCommandContext = exec.CommandContext

type PluginTestOptions struct {
	Root         string
	Platform     string
	Event        string
	Fixture      string
	GoldenDir    string
	UpdateGolden bool
	All          bool
}

type PluginTestCase struct {
	Platform     string               `json:"platform"`
	Event        string               `json:"event"`
	FixturePath  string               `json:"fixture_path"`
	Carrier      string               `json:"carrier"`
	Command      []string             `json:"command,omitempty"`
	Stdout       string               `json:"stdout"`
	Stderr       string               `json:"stderr"`
	ExitCode     int                  `json:"exit_code"`
	GoldenDir    string               `json:"golden_dir,omitempty"`
	GoldenStatus string               `json:"golden_status,omitempty"`
	GoldenFiles  []string             `json:"golden_files,omitempty"`
	Mismatches   []string             `json:"mismatches,omitempty"`
	MismatchInfo []PluginTestMismatch `json:"mismatch_info,omitempty"`
	Failure      string               `json:"failure,omitempty"`
	Passed       bool                 `json:"passed"`
}

type PluginTestMismatch struct {
	Field           string `json:"field"`
	GoldenFile      string `json:"golden_file,omitempty"`
	ExpectedPreview string `json:"expected_preview,omitempty"`
	ActualPreview   string `json:"actual_preview,omitempty"`
}

type PluginTestSummary struct {
	Total               int `json:"total"`
	Passed              int `json:"passed"`
	Failed              int `json:"failed"`
	GoldenMatched       int `json:"golden_matched"`
	GoldenUpdated       int `json:"golden_updated"`
	GoldenNotConfigured int `json:"golden_not_configured"`
	GoldenMismatch      int `json:"golden_mismatch"`
}

type PluginTestResult struct {
	Passed  bool              `json:"passed"`
	Summary PluginTestSummary `json:"summary"`
	Lines   []string          `json:"lines"`
	Cases   []PluginTestCase  `json:"cases"`
}

type runtimeTestSupport struct {
	Platform string
	Event    string
	Carrier  string
}

func (PluginService) Test(ctx context.Context, opts PluginTestOptions) (PluginTestResult, error) {
	return runPluginTests(ctx, opts)
}
