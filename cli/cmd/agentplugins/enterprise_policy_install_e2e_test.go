//go:build uappm_e2e

package main

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func enterpriseInstallDeniedBeforeHTTP(t *testing.T) {
	origin := newPMInstallOrigin(t)
	f := newPMInstallJourney(t, origin)
	user := filepath.Join(f.home, ".config", "agentplugins", "policy.yaml")
	projectJourneyWrite(t, user, "schema_version: 2\nsources:\n  deny_hosts: [127.0.0.1]\n")
	projectJourneyWrite(t, filepath.Join(f.a, "uap-policy.yaml"), "schema_version: 2\nsources:\n  allow_hosts: [127.0.0.1]\n")
	before := f.activeSnapshot(t, f.a)
	enterpriseInstallDenied(t, f, f.a, "sources.deny_hosts", user,
		"add", origin.url(), "--alias", "rich", "--target", "gemini", "--yes", "--accept-security-risk")
	if origin.count() != 0 || f.security.calls != 0 {
		t.Fatal("denied source reached HTTPS acquisition or security assessment")
	}
	pmInstallEqual(t, "source policy denial", before, f.activeSnapshot(t, f.a))
	f.noActive(t, f.a)
	projectJourneyAbsent(t, filepath.Join(f.a, "uap.toml"), filepath.Join(f.a, "uap.lock"))
}

func enterpriseInstallTighteningAndRemoval(t *testing.T) {
	for _, scope := range []string{"user", "project"} {
		t.Run(scope, func(t *testing.T) {
			origin := newPMInstallOrigin(t)
			f := newPMInstallJourney(t, origin)
			project := ""
			if scope == "project" {
				project = f.a
				projectJourneyWrite(t, filepath.Join(project, "uap-policy.yaml"), "schema_version: 2\ninstall:\n  allowed_scopes: [project]\n  allowed_targets: [gemini]\n")
			}
			user := filepath.Join(f.home, ".config", "agentplugins", "policy.yaml")
			allowed := "schema_version: 2\nsources:\n  allowed_kinds: [archive]\n  allow_hosts: [127.0.0.1]\n"
			projectJourneyWrite(t, user, allowed)
			f.run(t, project, "add", origin.url(), "--alias", "rich", "--target", "gemini")
			installation, binding := f.assertInstalled(t, project, "gemini", "v1", false)
			f.assertPin(t, project, "gemini")
			projectJourneyWrite(t, user, allowed+"components:\n  denied: [skills]\n")
			enterpriseInstallWarmRefusals(t, f, project, user, installation.InstallationID)
			f.run(t, project, "remove", "rich", "--target", "gemini")
			projectJourneyAbsent(t, binding.TargetLocator)
			enterpriseInstallRemoved(t, f, project)
			projectJourneyWrite(t, user, allowed)
			f.run(t, project, "add", origin.url(), "--alias", "rich", "--target", "gemini")
			f.assertInstalled(t, project, "gemini", "v1", false)
			f.assertPin(t, project, "gemini")
		})
	}
}

func enterpriseInstallWarmRefusals(t *testing.T, f pmInstallJourney, project, policy, installation string) {
	before := f.activeSnapshot(t, project)
	requests := f.origin.count()
	for _, args := range [][]string{
		{"add", f.origin.url(), "--alias", "rich", "--target", "gemini", "--yes"},
		{"install", "--locked", "--yes"},
		{"repair", installation, "--target", "gemini", "--yes"},
	} {
		enterpriseInstallDenied(t, f, project, "components.denied", policy, args...)
		pmInstallEqual(t, "tightened policy refusal for "+args[0], before, f.activeSnapshot(t, project))
	}
	if f.origin.count() != requests {
		t.Fatal("tightened policy did not reject the existing warm package before acquisition")
	}
}

func enterpriseInstallRemoved(t *testing.T, f pmInstallJourney, project string) {
	t.Helper()
	state := f.state(t, project)
	if len(state.EnvironmentReceipts) != 0 {
		t.Fatal("policy-safe removal retained active environment ownership")
	}
	for _, installation := range state.Installations {
		if len(installation.Clients) != 0 {
			t.Fatal("policy-safe removal retained an active client binding")
		}
	}
}

func enterpriseInstallImportPublication(t *testing.T) {
	origin := newPMInstallOrigin(t)
	original := newPMInstallJourney(t, origin)
	original.run(t, original.a, "add", origin.url(), "--alias", "rich", "--target", "gemini")
	pin := original.assertPin(t, original.a, "gemini")
	exchange := projectJourneyTemp(t)
	original.run(t, original.a, "deps", "export", exchange, "--yes")
	pair := original.pair(t, exchange)
	cold := newPMInstallJourney(t, origin)
	user := filepath.Join(cold.home, ".config", "agentplugins", "policy.yaml")
	projectJourneyWrite(t, user, "schema_version: 2\nsources:\n  allowed_kinds: [archive]\n")
	policy := filepath.Join(cold.a, "uap-policy.yaml")
	projectJourneyWrite(t, policy, "schema_version: 2\ncomponents:\n  denied: [mcp]\n")
	before, requests := cold.activeSnapshot(t, cold.a), origin.count()
	enterpriseInstallDenied(t, cold, cold.a, "components.denied", policy, "deps", "import", exchange, "--install", "--yes")
	if origin.count() <= requests {
		t.Fatal("cold import did not inspect the real archived package")
	}
	pmInstallEqual(t, "denied import before agreement publication", before, cold.activeSnapshot(t, cold.a))
	pmInstallEqual(t, "source agreement after denied import", pair, original.pair(t, exchange))
	cold.noActive(t, cold.a)
	projectJourneyAbsent(t, filepath.Join(cold.a, "uap.toml"), filepath.Join(cold.a, "uap.lock"))
	projectJourneyWrite(t, policy, "schema_version: 2\ninstall:\n  allowed_scopes: [project]\n")
	cold.run(t, cold.a, "deps", "import", exchange, "--install", "--yes")
	restored := cold.assertPin(t, cold.a, "gemini")
	cold.assertInstalled(t, cold.a, "gemini", "v1", false)
	if pin.TreeDigest != restored.TreeDigest || pair != cold.pair(t, cold.a) {
		t.Fatal("permitted fresh import changed the portable pin or agreement")
	}
}

func enterpriseInstallDenied(t *testing.T, f pmInstallJourney, project, code, policy string, args ...string) {
	t.Helper()
	out, err := f.execute(t, project, "", false, append(args, "--format", "json")...)
	var denied *domain.InstallPolicyError
	if !errors.As(err, &denied) {
		t.Fatalf("%v did not report installation policy denial: %v\n%s", args, err, out)
	}
	for _, violation := range denied.Violations {
		if violation.Code == code && violation.PolicyPath == policy {
			return
		}
	}
	t.Fatalf("%v did not identify %s from %s: %+v\n%s", args, code, policy, denied.Violations, out)
}
