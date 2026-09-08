package commands_test

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/commands"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/report"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoringcli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
)

func TestReadinessComposition(t *testing.T) {
	root, scratch := t.TempDir(), t.TempDir()
	write(t, root, "plugin.json", plugin(`,"extensions":{"dev.example":{"secret":"`+marker+`"}}`))
	write(t, root, "mcp.json", mcp(`{"good":{"type":"stdio","command":"node","env":{"SECRET":"`+marker+`"}},"bad":{"type":"future-transport","url":"`+marker+`"}}`))
	app := commands.App{Projects: project.Service{Scratch: scratch}, Revision: "readiness-test"}
	p, err := app.Projects.Read(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	want, err := planner.Compatibility(*p.Facts.Package, domain.SupportedClientIDs())
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, id := range domain.SupportedClientIDs() {
		ids = append(ids, string(id))
	}
	before := tree(t, root)
	for _, name := range []string{"compat", "inspect"} {
		args := []string{name, root, "--target=" + strings.Join(ids, ","), "--format=json"}
		a, ac, ab := execute(t, app, args, false)
		b, bc, bb := execute(t, app, args, true)
		if ac != 1 || ac != bc || !bytes.Equal(ab, bb) || !reflect.DeepEqual(a.Clients, want) || !reflect.DeepEqual(a, b) {
			t.Fatalf("%s parity/planner mismatch: %d %d", name, ac, bc)
		}
		if a.Compatibility.Status != report.Fail || a.Runtime.Status != report.NotEvaluated || a.Conformance.Status != report.Fail {
			t.Fatal("conflated assessments", a)
		}
	}
	for _, mounted := range []bool{false, true} {
		r, code, _ := execute(t, app, []string{"doctor", root, "--format=json"}, mounted)
		if code != 1 || r.Toolchain.Status == report.Pass || len(r.DoctorChecks) == 0 {
			t.Fatal("fake doctor readiness", r)
		}
		r, code, _ = execute(t, app, []string{"capabilities", "--format=json"}, mounted)
		if code != 0 || r.Capabilities == nil || r.Readiness.Status != report.NotEvaluated || r.Runtime.Status != report.NotEvaluated {
			t.Fatal("capabilities is not package readiness", r)
		}
	}
	if !reflect.DeepEqual(before, tree(t, root)) || len(tree(t, scratch)) != 0 {
		t.Fatal("read changed source or retained scratch")
	}
}

func TestReadinessArgumentIsolation(t *testing.T) {
	app := commands.App{Projects: project.Service{Scratch: "/must-not-be-opened"}}
	for _, args := range [][]string{
		{"compat", "/must-not-be-opened"}, {"compat", "/must-not-be-opened", "--target=codex,codex"},
		{"compat", "/must-not-be-opened", "--target=" + marker}, {"inspect", "/must-not-be-opened", "--target="},
		{"doctor", "/must-not-be-opened", "--target=codex"}, {"capabilities", "extra"},
		{"capabilities", "--release-policy"}, {"capabilities", "--target=codex"},
		{"compat", "/must-not-be-opened", "--target=codex", "--dry-run=false"},
		{"doctor", "/must-not-be-opened", "--runtime"},
	} {
		args = append(args, "--format=json")
		_, ac, a := execute(t, app, args, false)
		r, bc, b := execute(t, app, args, true)
		if ac != 2 || bc != 2 || !bytes.Equal(a, b) || r.Error.Code != "arguments_invalid" {
			t.Fatalf("argument mismatch %v: %d %d %s", args, ac, bc, b)
		}
	}
	for _, flag := range []string{"--scope=user", "--accept-security-risk=false", "--security-details=false"} {
		for _, name := range []string{"compat", "doctor", "capabilities"} {
			args := []string{name}
			if name != "capabilities" {
				args = append(args, "/must-not-be-opened")
			}
			if name == "compat" {
				args = append(args, "--target=codex")
			}
			args = append(args, flag, "--format=json")
			r, code, _ := execute(t, app, args, true)
			if code != 2 || r.Error.Code != "arguments_invalid" {
				t.Fatal("installer flag reached read", r)
			}
		}
	}
}

func TestReadinessUnknownSchemaAndFreshFactories(t *testing.T) {
	root := t.TempDir()
	write(t, root, "plugin.json", `{"$schema":"https://invalid.example/future","name":"demo"}`)
	write(t, root, "package.json", `{"secret":"`+marker+`"}`)
	app := commands.App{Projects: project.Service{Scratch: t.TempDir()}}
	for _, name := range []string{"compat", "inspect", "doctor"} {
		args := []string{name, root, "--format=json"}
		if name != "doctor" {
			args = append(args, "--target=codex")
		}
		r, code, _ := execute(t, app, args, false)
		if code != 1 || len(r.Clients) != 0 || len(r.DoctorChecks) != 0 || r.Capabilities != nil || r.Compatibility.Status != report.NotEvaluated {
			t.Fatal("future schema guesses", r)
		}
	}
	for i := 0; i < 4; i++ {
		r, code, _ := execute(t, app, []string{"capabilities", "--format=json"}, i%2 == 0)
		if code != 0 || len(r.Capabilities.Commands) != 8 {
			t.Fatal("factory state leak")
		}
	}
	var out bytes.Buffer
	if err := app.Execute(context.Background(), []string{"doctor", "--help"}, authoringcli.Streams{Out: &out, Err: &out}, authoringcli.NewPluginKitRoot); err != nil || !strings.Contains(out.String(), "no processes or network") {
		t.Fatal("doctor help", err, out.String())
	}
}

func TestReadinessHumanOutput(t *testing.T) {
	root := t.TempDir()
	write(t, root, "plugin.json", plugin(`,"extensions":{"dev.example":{"secret":"`+marker+`"}}`))
	write(t, root, "mcp.json", mcp(`{"`+marker+`":{"type":"stdio","command":"`+marker+`","env":{"SECRET":"`+marker+`"}}}`))
	app := commands.App{Projects: project.Service{Scratch: t.TempDir()}}
	for _, args := range [][]string{{"compat", root, "--target=cursor"}, {"doctor", root}, {"capabilities"}} {
		for _, mount := range []bool{false, true} {
			var out, stderr bytes.Buffer
			builder := commands.RootBuilder(authoringcli.NewPluginKitRoot)
			input := append([]string{}, args...)
			if mount {
				builder = mounted
				input = append([]string{"author"}, input...)
			}
			_ = app.Execute(context.Background(), input, authoringcli.Streams{Out: &out, Err: &stderr}, builder)
			if stderr.Len() != 0 || strings.Contains(out.String(), marker) || strings.Contains(out.String(), root) {
				t.Fatalf("human disclosure: %s", out.String())
			}
			if args[0] == "doctor" && !strings.Contains(out.String(), "no PATH lookup") {
				t.Fatal("doctor lacks actionable uncertainty")
			}
			if args[0] == "capabilities" && !strings.Contains(out.String(), "schema: https://agent-plugins.org/") {
				t.Fatal("human schemas missing")
			}
			if args[0] == "compat" && !strings.Contains(out.String(), "static support only") {
				t.Fatal("human compatibility scope missing")
			}
		}
	}
}
