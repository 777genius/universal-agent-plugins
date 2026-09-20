package main

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
)

type publicationDoctorJSONReport struct {
	Format                string                                 `json:"format"`
	SchemaVersion         int                                    `json:"schema_version"`
	RequestedTarget       string                                 `json:"requested_target,omitempty"`
	Ready                 bool                                   `json:"ready"`
	Status                string                                 `json:"status"`
	WarningCount          int                                    `json:"warning_count"`
	Warnings              []string                               `json:"warnings"`
	IssueCount            int                                    `json:"issue_count"`
	Issues                []publicationIssue                     `json:"issues"`
	NextSteps             []string                               `json:"next_steps"`
	MissingPackageTargets []string                               `json:"missing_package_targets,omitempty"`
	LocalRoot             *app.PluginPublicationVerifyRootResult `json:"local_root,omitempty"`
	Publication           publicationmodel.Model                 `json:"publication"`
}

type publicationJSONReport struct {
	Format          string                 `json:"format"`
	SchemaVersion   int                    `json:"schema_version"`
	RequestedTarget string                 `json:"requested_target,omitempty"`
	WarningCount    int                    `json:"warning_count"`
	Warnings        []string               `json:"warnings"`
	Publication     publicationmodel.Model `json:"publication"`
}
