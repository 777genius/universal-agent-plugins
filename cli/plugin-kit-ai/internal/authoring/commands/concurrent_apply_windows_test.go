//go:build windows && amd64

package commands_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/report"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/scaffold"
)

// Keep raw errors in test diagnostics: the command's public failure report
// intentionally hides the filesystem primitive that refused publication.
func TestConcurrentApplyWithSharedProjectService(t *testing.T) {
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	scratch, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(parent, "demo")
	service := project.Service{Scratch: scratch}
	plan, err := scaffold.BuildPlan(scaffold.Options{Template: scaffold.Skill, Name: "demo", Description: "A fixture."})
	if err != nil {
		t.Fatal(err)
	}
	type outcome struct {
		result    scaffold.Result
		err       error
		validated bool
	}
	ready := make(chan struct{}, 2)
	release := make(chan struct{})
	results := make(chan outcome, 2)
	for range 2 {
		go func() {
			validated := false
			// Also arrive on early errors so a failing validator cannot strand
			// its peer at the barrier and hide the original diagnostic.
			defer func() {
				if !validated {
					ready <- struct{}{}
				}
			}()
			result, err := scaffold.Apply(context.Background(), plan, scaffold.ApplyOptions{
				Destination: dest,
				Validate: func(ctx context.Context, stage string) error {
					p, err := service.Read(ctx, stage)
					if err != nil {
						return err
					}
					if !report.Build("validate", baseline, p, false).Successful() {
						return fmt.Errorf("generated package validation failed")
					}
					// Read has returned and closed its leases before either
					// contender is permitted to attempt publication.
					validated = true
					ready <- struct{}{}
					<-release
					return nil
				},
			})
			results <- outcome{result, err, validated}
		}()
	}
	<-ready
	<-ready
	close(release)
	winners := 0
	for range 2 {
		got := <-results
		t.Logf("shared-service Apply: validated=%t result=%+v raw_error=%T %v", got.validated, got.result, got.err, got.err)
		if !got.validated {
			t.Errorf("Apply did not complete shared validation: %v", got.err)
		}
		if got.err == nil && got.result.Committed {
			winners++
		} else if got.result.Committed || !errors.Is(got.err, os.ErrExist) {
			t.Errorf("loser must report destination existence: result=%+v raw_error=%T %v", got.result, got.err, got.err)
		}
	}
	if winners != 1 {
		t.Fatalf("exclusive shared-service Apply winners: %d", winners)
	}
	p, err := service.Read(context.Background(), dest)
	if err != nil || !report.Build("validate", baseline, p, false).Successful() {
		t.Fatalf("winning tree invalid: %v", err)
	}
	if entries, err := os.ReadDir(parent); err != nil || len(entries) != 1 || entries[0].Name() != "demo" {
		t.Errorf("publication staging remains: %v %v", entries, err)
	}
	if entries, err := os.ReadDir(scratch); err != nil || len(entries) != 0 {
		t.Errorf("shared validation scratch remains: %v %v", entries, err)
	}
}
