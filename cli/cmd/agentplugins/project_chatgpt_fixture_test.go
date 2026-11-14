package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type projectChatGPTBoundaries struct{ acquisition, detection, security int }

func (spy *projectChatGPTBoundaries) Detect(context.Context) ([]domain.DetectedClient, error) {
	spy.detection++
	return nil, errors.New("unexpected detection after unsupported project request")
}

func (spy *projectChatGPTBoundaries) Evaluate(context.Context, domain.SecurityEvaluationInput) (domain.SecurityAssessment, error) {
	spy.security++
	return domain.SecurityAssessment{}, errors.New("unexpected security evaluation after unsupported project request")
}

func (spy *projectChatGPTBoundaries) rejectAcquisition() (domain.PackageSnapshot, error) {
	spy.acquisition++
	return domain.PackageSnapshot{}, errors.New("unexpected acquisition after unsupported project request")
}

func (spy *projectChatGPTBoundaries) AcquireLocal(context.Context, string) (domain.PackageSnapshot, error) {
	return spy.rejectAcquisition()
}

func (spy *projectChatGPTBoundaries) AcquireGitHub(context.Context, string, string, string) (domain.PackageSnapshot, error) {
	return spy.rejectAcquisition()
}

func (spy *projectChatGPTBoundaries) AcquireGitHubVerified(context.Context, string, string, string, string) (domain.PackageSnapshot, error) {
	return spy.rejectAcquisition()
}

func (spy *projectChatGPTBoundaries) DiscoverGitHubPackages(context.Context, string, string) ([]string, error) {
	spy.acquisition++
	return nil, errors.New("unexpected discovery after unsupported project request")
}

func projectChatGPTApp(app agentpluginscli.App, spy *projectChatGPTBoundaries) agentpluginscli.App {
	app.Detector, app.SourceAcquirer, app.SecurityEvaluator = spy, spy, spy
	app.DirectoryClient, app.DiscoveryClient, app.SecurityIndex = nil, nil, nil
	app.DirectoryClientForDryRun, app.CommitDirectoryPreview = nil, nil
	app.SourceAcquirerForDryRun, app.SecurityEvaluatorForDryRun = nil, nil
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return projectChatGPTApp(selected, spy), nil
	}
	return app
}

func projectChatGPTExecute(app agentpluginscli.App, project string, args ...string) (string, error) {
	var output bytes.Buffer
	app.Input, app.Output, app.ErrorOutput, app.Terminal = strings.NewReader(""), &output, &output, false
	command := agentpluginscli.NewRoot(app)
	command.SetOut(&output)
	command.SetErr(&output)
	args = append(args, "--scope", "project", "--format", "json")
	if project != "" {
		args = append(args, "--project-root", project)
	}
	command.SetArgs(args)
	err := command.Execute()
	return output.String(), err
}

func projectChatGPTWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}
