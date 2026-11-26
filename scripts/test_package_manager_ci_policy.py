"""Guard the fast-draft / full-ready package-manager CI contract.

Structural checks only; they do not substitute for GitHub event evaluation.
Dependency-free so drafts can run without setting up Go or accounts.
"""

from pathlib import Path
import importlib.util
import json
from tempfile import TemporaryDirectory
import textwrap
from unittest.mock import Mock, patch
import re
import os
import shutil
import subprocess
import unittest

WORKFLOW = Path(__file__).resolve().parents[1] / ".github/workflows/package-manager-e2e.yml"
NATIVE_WORKFLOW = WORKFLOW.with_name("agentplugins-native-clients.yml")
RESILIENCE_WORKFLOW = WORKFLOW.with_name("package-manager-resilience.yml")


class PackageManagerCIPolicyTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.body = WORKFLOW.read_text(encoding="utf-8")

    def test_ready_pr_and_main_are_always_full(self):
        self.assertRegex(self.body, r"(?m)^  pull_request:\s*$")
        self.assertIn("types: [opened, synchronize, reopened, ready_for_review]", self.body)
        self.assertRegex(self.body, r"(?m)^  push:\n    branches: \[main\]")
        self.assertRegex(self.body, r"(?m)^  workflow_dispatch:\s*$")
        self.assertIn("github.event_name != 'pull_request' || github.event.pull_request.draft == false", self.body)
        # Ordinary PRs stay on the four already-qualified source-build OSs.
        # Weekly/manual deep E2E expands to both genuine ARM64 hosted runners.
        self.assertIn("os: ${{ fromJSON(", self.body)
        self.assertIn('["ubuntu-24.04","windows-2025","macos-15","macos-15-intel"]', self.body)
        self.assertIn('["ubuntu-24.04","ubuntu-24.04-arm","windows-2025","windows-11-arm","macos-15","macos-15-intel"]', self.body)
        self.assertIn("schedule:", self.body)
        self.assertIn("fail-fast: false", self.body)

    def test_release_qualification_reuses_workflows_and_fail_closed_profiles(self):
        root = WORKFLOW.parent
        version = (root / "agentplugins-published-version.yml").read_text(encoding="utf-8")
        installer = (root / "agentplugins-native-install.yml").read_text(encoding="utf-8")
        self.assertIn("workflow_call:", version)
        # GitHub Release may be public before npm environment approval.
        # For unpinned PR/weekly smoke, only npm latest may select a target,
        # and it still requires the corresponding public GitHub release.
        # Ordinary source-PR smoke may test historical npm latest; trusted
        # publication must additionally prove newest GitHub/npm parity.
        self.assertIn('releases/latest" --jq', version)
        self.assertIn('latest_parity_required=true', version)
        self.assertIn('"${GITHUB_EVENT_NAME}" == "workflow_run" || -z "$requested"', version)
        self.assertIn('latest_public_tag', version)
        self.assertIn('tag="agentplugins-v$current"', version)
        self.assertIn('[[ "$current" =~ ^[0-9]+\\.[0-9]+\\.[0-9]+$ ]]', version)
        self.assertIn("npm view universal-agent-plugins dist-tags.latest", version)
        self.assertIn("npm view \"universal-agent-plugins@$version\" version", version)
        for target in ("darwin-amd64", "darwin-arm64", "linux-amd64",
                       "linux-arm64", "windows-amd64", "windows-arm64"):
            self.assertIn("target: " + target, installer)
        self.assertIn("needs: release-version", installer)
        self.assertIn("schedule:", installer)
        for workflow in ("agentplugins-published-windows-audit.yml",
                         "agentplugins-published-macos-audit.yml"):
            body = (root / workflow).read_text(encoding="utf-8")
            with self.subTest(workflow=workflow):
                self.assertIn("uses: ./.github/workflows/agentplugins-published-version.yml", body)
                self.assertIn("Agentplugins NPM Publish", body)
                self.assertIn("schedule:", body)
                self.assertIn("AUDIT_LIMIT:", body)
                self.assertIn("target: [codex, claude, opencode, gemini, copilot, cline, kimi, qoder, pi]", body)
                self.assertIn("exclude:", body)
                self.assertIn("- target: pi", body)
                self.assertIn("require_trusted_publish: true", body)
                self.assertIn("kind: plugin", body)
                self.assertIn("kind: [plugin, skill]", body)
                self.assertIn("scripts/live-install-audit.py", body)
                self.assertIn("AUDIT_VENDOR_OUTCOME:", body)
                self.assertIn("if: always()", body)
                self.assertNotIn("continue-on-error: true", body)
        for workflow in ("agentplugins-published-catalog-audit.yml",
                         "agentplugins-published-other-clients-audit.yml"):
            body = (root / workflow).read_text(encoding="utf-8")
            with self.subTest(workflow=workflow):
                self.assertIn("uses: ./.github/workflows/agentplugins-published-version.yml", body)
                self.assertIn("Agentplugins NPM Publish", body)
                self.assertIn("schedule:", body)
                self.assertIn("scripts/live-install-audit.py", body)
                self.assertNotIn("universal-agent-plugins@0.1.101", body)

    def test_deep_macos_docker_hub_has_genuine_cli_prerequisite(self):
        macos = (WORKFLOW.parent / "agentplugins-published-macos-audit.yml").read_text(
            encoding="utf-8")
        # Published audits no longer run on PRs. The full plugin lane retains
        # the real Docker CLI prerequisite; it cannot be skipped for publishing.
        self.assertIn("if: ${{ matrix.kind == 'plugin' }}", macos)
        self.assertIn("brew install docker", macos)
        self.assertIn("command -v docker", macos)
        self.assertIn("docker --version", macos)
        self.assertNotIn("continue-on-error: true", macos)


    def test_published_audits_only_run_after_qualified_publish_or_explicit_window(self):
        root = WORKFLOW.parent
        for workflow in (
            "agentplugins-published-windows-audit.yml",
            "agentplugins-published-macos-audit.yml",
            "agentplugins-published-catalog-audit.yml",
            "agentplugins-published-other-clients-audit.yml",
        ):
            body = (root / workflow).read_text(encoding="utf-8")
            with self.subTest(workflow=workflow):
                events = body.split("\npermissions:", 1)[0]
                self.assertNotRegex(events, r"(?m)^  (pull_request|push):")
                self.assertIn("  workflow_run:", events)
                self.assertIn("    workflows: ['Agentplugins NPM Publish']", events)
                self.assertIn("    types: [completed]", events)
                self.assertIn("  schedule:", events)
                self.assertIn("  workflow_dispatch:", events)
                self.assertIn("require_trusted_publish: true", body)
                self.assertIn("github.event.workflow_run.conclusion == 'success'", body)
                self.assertIn("permissions:\n  contents: read\n  actions: read", body)
                self.assertIn("cancel-in-progress: false", body)
                self.assertNotIn("continue-on-error: true", body)

    def test_trusted_publish_resolver_rejects_staging_only_runs(self):
        root = WORKFLOW.parent
        version = (root / "agentplugins-published-version.yml").read_text(encoding="utf-8")
        native = (root / "agentplugins-native-install.yml").read_text(encoding="utf-8")
        self.assertIn("require_trusted_publish:", version)
        self.assertIn("PUBLISH_RUN_ID:", version)
        self.assertIn("PUBLISH_HEAD_REF:", version)
        self.assertIn("Verify public npm provenance and lifecycle", version)
        self.assertIn("Publish npm package with trusted provenance", version)
        self.assertIn('requested="${PUBLISH_HEAD_REF#agentplugins-v}"', version)
        self.assertIn("actions: read", version)
        # The native installer remains a resolver caller, without requiring
        # the publisher's run identity on manual or PR executions.
        self.assertIn("actions: read", native)
        self.assertIn("uses: ./.github/workflows/agentplugins-published-version.yml", native)
        self.assertIn("require_trusted_publish: ${{ github.event_name == 'workflow_run' }}", native)

    def test_trusted_publish_latest_parity_negative_fixtures(self):
        version = (WORKFLOW.parent / "agentplugins-published-version.yml").read_text(encoding="utf-8")
        # Test the exact Bash condition used by the trusted-publisher lane.
        predicate = ('[[ "$published" == "$version" && "$current" == "$version" '
                     '&& "$latest_public_tag" == "$tag" ]]')
        self.assertIn(predicate, version)
        bash = shutil.which("bash")
        if not bash:
            self.skipTest("Bash unavailable; structural release assertions still run")
        cases = [
            ("matching .112", "0.1.112", "0.1.112", "agentplugins-v0.1.112", "0.1.112", True),
            ("GitHub .111 npm latest .110", "0.1.111", "0.1.110", "agentplugins-v0.1.111", "0.1.111", False),
            ("newer GitHub latest", "0.1.111", "0.1.111", "agentplugins-v0.1.112", "0.1.111", False),
            ("wrong package", "0.1.110", "0.1.111", "agentplugins-v0.1.111", "0.1.111", False),
        ]
        for name, published, current, latest, exact, expected in cases:
            with self.subTest(case=name):
                env = dict(os.environ, published=published, current=current,
                           latest_public_tag=latest, version=exact,
                           tag="agentplugins-v" + exact)
                result = subprocess.run([bash, "-c", predicate], env=env,
                                        capture_output=True, timeout=5, check=False)
                self.assertEqual(result.returncode == 0, expected, name)

        # This is the exact event-mode guard from the resolver, not a
        # Python reimplementation; published audits pass require=true.
        mode_guard = ('[[ "${REQUIRE_TRUSTED_PUBLISH}" == "true" && '
                      '( "${GITHUB_EVENT_NAME}" == "workflow_run" || -z "$requested" ) ]]')
        self.assertIn(mode_guard, version)
        modes = [
            ("published on schedule", "true", "schedule", "", True),
            ("published manual unpinned", "true", "workflow_dispatch", "", True),
            ("trusted publish event", "true", "workflow_run", "0.1.112", True),
            ("manual historical explicit", "true", "workflow_dispatch", "0.1.110", False),
            ("native source PR smoke", "false", "pull_request", "", False),
            ("native installer weekly smoke", "false", "schedule", "", False),
        ]
        for name, require, event_name, requested, expected in modes:
            with self.subTest(mode=name):
                env = dict(os.environ, REQUIRE_TRUSTED_PUBLISH=require,
                           GITHUB_EVENT_NAME=event_name, requested=requested)
                result = subprocess.run([bash, "-c", mode_guard], env=env,
                                        capture_output=True, timeout=5, check=False)
                self.assertEqual(result.returncode == 0, expected, name)

    def test_firebase_current_source_proof_stays_automatic_and_independent(self):
        body = (WORKFLOW.parent / "agentplugins-windows-firebase-capture-probe.yml").read_text()
        self.assertRegex(body, r"(?m)^  pull_request:\s*$")
        self.assertRegex(body, r"(?m)^  workflow_dispatch:\s*$")
        # An event-dependent timeout does not disable automatic execution.
        # The source job itself must remain unconditional on every event.
        header = body.split("    steps:", 1)[0].split("jobs:", 1)[1]
        self.assertNotRegex(header, r"(?m)^    (?:if|'if'|\"if\")\s*:",
                            "Firebase source job must stay unconditional")
        events = body.split("  workflow_dispatch:", 1)[0]
        for path in ("scripts/firebase_runtime_failure.py",
                     "scripts/test_firebase_runtime_failure.py"):
            self.assertIn("      - '" + path + "'", events)
        steps = dict(re.findall(r"(?ms)^      - name: ([^\n]+)\n(.*?)(?=^      - |\Z)", body))
        native = next(step for step in steps.values()
                      if "npm.cmd install --global '@anthropic-ai/claude-code@2.1.263'" in step)
        self.assertIn("id: native-bootstrap", native)
        self.assertNotIn("        if:", native)
        self.assertIn("npm.cmd install --global '@anthropic-ai/claude-code@2.1.263'", native)
        self.assertIn("claude --version", native)
        self.assertNotIn("universal-agent-plugins", native)
        build = steps["Build exact current-source CLI for Windows"]
        self.assertIn("if: ${{ !cancelled() }}", build)
        self.assertIn("go build -trimpath -ldflags='-X main.version=0.1.115'", build)
        source = steps["Genuine current-source Firebase add, repeat, native attribution and remove"]
        source_version = re.search(r"main\.version=([0-9.]+)", build).group(1)
        self.assertIn("assert cli_version == 'agentplugins " + source_version + "'", source)
        self.assertIn("if: ${{ !cancelled() && steps.source-build.outcome == 'success' }}", source)
        self.assertIn("AUDIT_VENDOR_OUTCOME: ${{ steps.native-bootstrap.outcome }}", source)
        self.assertIn("assert actual_head == os.environ['GITHUB_SHA'] and clean_worktree", source)
        self.assertIn("'source_tree': actual_tree", source)
        self.assertIn("'source_cli_sha256': cli_sha256", source)
        self.assertIn("Current-source Windows Firebase native lifecycle unqualified", source)
        self.assertNotIn("workflow_dispatch", source)
        artifact = steps["Preserve current-source Firebase evidence on failure"]
        self.assertIn("if: always()", artifact)
        self.assertIn("source-firebase-claude.json", artifact)

    def test_firebase_automatic_policy_rejects_actual_job_conditions(self):
        body = (WORKFLOW.parent / "agentplugins-windows-firebase-capture-probe.yml").read_text()
        for key, condition in (("if", "${{ github.event_name == 'workflow_dispatch' }}"),
                               ("'if'", "false"),
                               ('"if"', "${{ needs.legacy.result == 'success' }}")):
            with self.subTest(key=key, condition=condition):
                gated = body.replace("    runs-on: windows-2025",
                                     f"    {key}: {condition}\n    runs-on: windows-2025", 1)
                self.assertNotEqual(gated, body)
                with patch.object(Path, "read_text", return_value=gated), \
                     self.assertRaisesRegex(AssertionError, "Firebase source job must stay unconditional"):
                    self.test_firebase_current_source_proof_stays_automatic_and_independent()

    def test_firebase_legacy_control_requires_manual_execution_and_native_proof(self):
        body = (WORKFLOW.parent / "agentplugins-windows-firebase-capture-probe.yml").read_text()
        steps = dict(re.findall(r"(?ms)^      - name: ([^\n]+)\n(.*?)(?=^      - |\Z)", body))
        install = next(step for step in steps.values()
                       if "npm.cmd install --global universal-agent-plugins@0.1.101" in step)
        legacy = steps["Genuine pinned Firebase add, repeat, native attribution and remove"]
        for step in (install, legacy):
            self.assertIn("if: ${{ github.event_name == 'workflow_dispatch' }}", step)
            self.assertNotIn("continue-on-error", step)
        self.assertIn("npm.cmd install --global universal-agent-plugins@0.1.101", install)
        self.assertIn("$v -ne 'agentplugins 0.1.101'", install)
        self.assertIn("'published_cli': 'agentplugins 0.1.101'", legacy)
        for assertion in ("row.get('status') != 'passed'", "row.get('repeat_verified') is not True",
                          "row.get('removal_verified') is not True", "row.get('sentinels_preserved') is not True",
                          "row.get('native_residuals') != []"):
            self.assertIn(assertion, legacy)
        self.assertIn("raise SystemExit('Published Windows Firebase native lifecycle unqualified')", legacy)
        artifact = steps["Preserve exact-source evidence on failure"]
        self.assertIn("if: ${{ always() && github.event_name == 'workflow_dispatch' }}", artifact)
        self.assertIn("published-firebase-claude.json", artifact)

    def test_firebase_source_program_refuses_failed_bootstrap_before_native_activity(self):
        body = (WORKFLOW.parent / "agentplugins-windows-firebase-capture-probe.yml").read_text()
        step = body.split("      - name: Genuine current-source Firebase add, repeat, native attribution and remove\n", 1)[1]
        program = textwrap.dedent(step.split("          @'\n", 1)[1].split("          '@ | python -", 1)[0])
        original_spec = importlib.util.spec_from_file_location
        native_case = Mock(side_effect=AssertionError("native case reached after failed bootstrap"))

        def bind_case(*args, **kwargs):
            spec = original_spec(*args, **kwargs)
            if args[0] == "published_firebase":
                execute = spec.loader.exec_module

                def load(audit):
                    execute(audit)
                    audit.do_case = native_case

                spec.loader.exec_module = load
            return spec

        with TemporaryDirectory() as temporary:
            (Path(temporary) / "agentplugins-source.exe").touch()
            with (patch.dict("os.environ", {"AUDIT_VENDOR_OUTCOME": "failure", "RUNNER_TEMP": temporary}),
                  patch("importlib.util.spec_from_file_location", side_effect=bind_case),
                  patch("subprocess.check_output", side_effect=AssertionError("native observation reached")) as observation):
                with self.assertRaisesRegex(SystemExit, "Vendor bootstrap was not verified"):
                    exec(compile(program, "<firebase-source-workflow>", "exec"), {})
                observation.assert_not_called()
        native_case.assert_not_called()

    def test_draft_only_runs_disposable_harness_preflight(self):
        self.assertIn("github.event_name == 'pull_request' && github.event.pull_request.draft", self.body)
        self.assertIn("name: draft evidence preflight", self.body)
        self.assertIn("test_package_manager_e2e.py", self.body)
        self.assertIn("test_package_manager_ci_policy.py", self.body)
        self.assertRegex(self.body, r"(?m)^    timeout-minutes: 5\s*$")
        self.assertNotIn("continue-on-error: true", self.body)

    def test_required_source_and_test_paths_are_watched(self):
        for path in (
            "'cli/**'", "'install/integrationctl/**'", "'sdk/**'", "'go.work*'",
            "'**/go.mod'", "'**/go.sum'", "'scripts/package-manager-e2e.py'",
            "'scripts/package_manager_binary.py'", "'scripts/test_package_manager_e2e.py'",
            "'scripts/test_package_manager_ci_policy.py'",
            "'.github/workflows/agentplugins-native-clients.yml'",
            "'.github/workflows/package-manager-resilience.yml'",
            "'.github/workflows/package-manager-e2e.yml'",
        ):
            event_paths = re.findall(r"(?m)^      - " + re.escape(path) + r"$", self.body)
            self.assertEqual(len(event_paths), 2, path)
        # Replace superseded PR runs; preserve every non-PR run independently.
        concurrency = re.search(r"(?m)^concurrency:\n((?:  .+\n)+)", self.body)
        self.assertIsNotNone(concurrency)
        self.assertEqual(concurrency[1],
            "  group: package-manager-e2e-${{ github.event_name }}-${{ "
            "github.event_name == 'pull_request' && (github.event.pull_request.number || github.ref) "
            "|| github.run_id }}\n"
            "  cancel-in-progress: ${{ github.event_name == 'pull_request' }}\n")

    def test_each_preflight_rejection_is_an_independent_fail_closed_step(self):
        # One native command per Actions step: Windows PowerShell must never
        # return the second test's exit code after the first test fails.
        for test in ("test_package_manager_e2e.py", "test_package_manager_ci_policy.py"):
            run = "        run: python -B -m unittest discover -s scripts -p " + test
            self.assertEqual(self.body.count(run + "\n"), 2, test)

    def test_native_matrix_not_lost_when_draft_becomes_ready(self):
        native = NATIVE_WORKFLOW.read_text(encoding="utf-8")
        self.assertIn("types: [opened, synchronize, reopened, ready_for_review]", native)
        self.assertIn("github.event.pull_request.draft == false", native)
        # The PR classifier may narrow a client's cells, but an unavailable
        # classification or a shared source change must still admit all three.
        self.assertIn(
            "client: ${{ fromJSON(needs.changes.result == 'success' && needs.changes.outputs.native_clients || "
            '\'["codex","claude","opencode"]\') }}',
            native,
        )
        self.assertIn("opencode-v2-project:", native)
        self.assertIn("target: linux-amd64", native)
        self.assertIn("client: opencode", native)
        self.assertIn("--project-components", native)
        self.assertIn("schedule:", native)
        self.assertIn("workflow_dispatch:", native)

    def test_resilience_runs_at_open_or_ready_and_targets_only_risk_changes(self):
        resilience = RESILIENCE_WORKFLOW.read_text(encoding="utf-8")
        self.assertIn("types: [opened, synchronize, reopened, ready_for_review]", resilience)
        self.assertIn("github.event.pull_request.draft == false", resilience)
        self.assertIn("schedule:", resilience)
        self.assertIn("workflow_dispatch:", resilience)
        self.assertIn("-fuzz '^FuzzVersionSelectionOrderAndPinReplay$'", resilience)
        self.assertIn("go test -race", resilience)
        for path in ("environment/resolver_reliability_test.go", "processlock/reliability_test.go"):
            self.assertIn(path, resilience)


class HeavyNativeSelectionPolicyTests(unittest.TestCase):
    """Exercise the deployed classifier and admission expressions without GitHub."""

    full_native = ["codex", "claude", "opencode"]
    full_remaining = ["cursor", "kiro", "windsurf", "grok", "chatgpt"]
    full_live = ["codex", "claude", "opencode", "gemini", "copilot", "cline", "kimi", "qoder", "pi"]

    @classmethod
    def setUpClass(cls):
        cls.workflows = {
            name: (WORKFLOW.parent / name).read_text()
            for name in ("agentplugins-native-clients.yml", "live-install-audit.yml",
                         "live-install-remaining-clients.yml", "pr-changes.yml")
        }

    def classify(self, files, *, event="pull_request", changed=None, fail=False):
        bash = shutil.which("bash")
        if not bash:
            self.skipTest("classifier runs on Linux with Bash")
        body = self.workflows["pr-changes.yml"]
        script = textwrap.dedent(body.split("        run: |\n", 1)[1])
        with TemporaryDirectory() as temporary:
            root = Path(temporary)
            listing, output = root / "files", root / "output"
            listing.write_text("\n".join(files), encoding="utf-8", newline="\n")
            gh = root / "gh"
            gh.write_text('#!/bin/sh\n[ "$FAKE_GH_FAIL" = 1 ] && exit 1\ncat "$FAKE_GH_FILES"\n',
                          encoding="utf-8", newline="\n")
            gh.chmod(0o755)
            env = {**os.environ, "PATH": str(root) + os.pathsep + os.environ["PATH"],
                   "GITHUB_EVENT_NAME": event, "GITHUB_OUTPUT": str(output),
                   "GITHUB_REPOSITORY": "owner/repo", "PR_NUMBER": "7",
                   "CHANGED_FILES": str(len(files) if changed is None else changed),
                   "FAKE_GH_FILES": str(listing), "FAKE_GH_FAIL": "1" if fail else "0"}
            # Keep the script out of Windows Bash launcher argument limits.
            # Binary stdin also preserves LF instead of translating it to CRLF.
            result = subprocess.run([bash, "-s"], input=script.encode("utf-8"), env=env,
                                    capture_output=True, timeout=5, check=False)
            self.assertEqual(result.returncode, 0, result.stderr.decode("utf-8", errors="replace"))
            pairs = [line.split("=", 1) for line in output.read_text().splitlines()]
            self.assertEqual(len(pairs), len(dict(pairs)), "duplicate classifier output")
            return {key: json.loads(value) for key, value in pairs if key.endswith("_clients")}

    def job(self, workflow, name):
        body = self.workflows[workflow]
        return re.search(r"(?ms)^  " + re.escape(name) + r":\n(.*?)(?=^  [\w-]+:|\Z)", body)[1]

    def expression(self, expression, *, result="success", selection="[]", pilot="success",
                   cancelled=False, event="pull_request", action="synchronize", draft=False,
                   label="", labels=(), sdk_only=False):
        # Only the expression vocabulary used by these four admission gates.
        # Values replace complete context atoms before the restricted evaluation.
        values = {"needs.changes.result": result, "needs.native-pilot.result": pilot,
                  "github.event_name": event, "github.event.action": action,
                  "github.event.pull_request.draft": draft, "github.event.label.name": label,
                  "github.event.pull_request.labels.*.name": list(labels),
                  "inputs.sdk_linux_only": sdk_only}
        for name in ("native_clients", "remaining_clients", "live_clients"):
            values["needs.changes.outputs." + name] = selection
        expression = expression.strip().removeprefix("${{").removesuffix("}}").strip()
        for key in sorted(values, key=len, reverse=True):
            expression = expression.replace(key, repr(values[key]))
        expression = expression.replace("!cancelled()", repr(not cancelled))
        expression = re.sub(r"\bfalse\b", "False", expression)
        expression = re.sub(r"\btrue\b", "True", expression)
        expression = expression.replace("!inputs", "not inputs")
        expression = expression.replace("!True", "not True").replace("!False", "not False")
        expression = expression.replace("&&", " and ").replace("||", " or ")
        return eval(expression, {"__builtins__": {}, "fromJSON": json.loads,
                                 "contains": lambda items, item: item in items}, {})

    def test_classifier_survives_windows_launch_transport(self):
        run, write_text = subprocess.run, Path.write_text

        def windows_write_text(path, data, encoding=None, errors=None, newline=None):
            # Model Windows defaults while honoring an explicit LF newline.
            return write_text(path, data, encoding=encoding, errors=errors,
                              newline="\r\n" if newline is None else newline)

        def windows_transport(args, **kwargs):
            # Model the launcher argument limit and Windows text stdin newline
            # conversion while still executing the real classifier in Bash.
            if isinstance(kwargs.get("input"), str):
                kwargs["input"] = kwargs["input"].replace("\n", "\r\n")
            return run([arg[:8192] for arg in args], **kwargs)

        with patch.object(subprocess, "run", side_effect=windows_transport), \
                patch.object(Path, "write_text", windows_write_text):
            self.assertEqual(self.classify(["scripts/project_native_cursor_acquisition.py", "docs/archive.md"]), {
                "native_clients": [], "remaining_clients": ["cursor"], "live_clients": []})

    def test_expanded_isolated_inputs_preserve_scoped_selection(self):
        cases = [
            (["scripts/project_native_cursor_acquisition.py"], [], ["cursor"], []),
            (["scripts/claude_darwin_metadata.py"], [], ["kiro"], []),
            (["scripts/test_claude_darwin_metadata.py"], [], ["kiro"], []),
            (["scripts/devin_structured_inventory_admission.py"], [], ["windsurf"], []),
            (["scripts/test_devin_structured_inventory_admission.py"], [], ["windsurf"], []),
            (["scripts/codex_plugin_retention_audit.py"], [], ["chatgpt"], ["codex"]),
            (["scripts/test_codex_plugin_retention_audit.py"], [], ["chatgpt"], ["codex"]),
            (["scripts/native_client_research.py"], self.full_native, self.full_remaining, self.full_live),
            (["scripts/project_native_research.py"], [], self.full_remaining, []),
            (["scripts/native_client_draft.py"], self.full_native, self.full_remaining, []),
            (["cli/internal/agentpluginscli/install_report.go"], self.full_native, self.full_remaining, []),
            (["cli/cmd/agentplugins/compose.go"], self.full_native, self.full_remaining, []),
            (["install/integrationctl/agentplugins/installer/types.go"], self.full_native, self.full_remaining, []),
            (["scripts/windows_owned_job.py"], [], [], self.full_live),
            (["scripts/test_live_install_audit_components.py"], [], [], self.full_live),
            (["scripts/devin_project_auto_install_e2e.py"], [], ["windsurf"], []),
            (["scripts/prepare-codex-linux-sandbox.sh"], self.full_native, ["chatgpt"], ["codex"]),
            (["scripts/qoder_prepared_skill_audit.py"], [], [], ["qoder"]),
            (["install/integrationctl/agentplugins/clients/claude/config.go"], ["claude"], self.full_remaining, []),
            (["scripts/project_native_cursor_acquisition.py", "docs/archive.md"], [], ["cursor"], []),
            ([".github/workflows/pr-changes.yml"], self.full_native, self.full_remaining, self.full_live),
        ]
        for files, native, remaining, live in cases:
            with self.subTest(files=files):
                self.assertEqual(self.classify(files), {"native_clients": native,
                    "remaining_clients": remaining, "live_clients": live})

    def test_classifier_unsafe_or_unavailable_inputs_stay_full(self):
        for kwargs in ({"fail": True}, {"changed": 3000}, {"changed": 1},
                       {"event": "workflow_dispatch"}, {"event": "schedule"}):
            with self.subTest(kwargs=kwargs):
                self.assertEqual(self.classify([], **kwargs), {"native_clients": self.full_native,
                    "remaining_clients": self.full_remaining, "live_clients": self.full_live})
        self.assertEqual(self.classify(["docs/unsafe\tname.md"]), {
            "native_clients": self.full_native, "remaining_clients": self.full_remaining,
            "live_clients": self.full_live})

    def test_classifier_dependency_is_watched_by_all_three_callers(self):
        for workflow, body in self.workflows.items():
            if workflow == "pr-changes.yml":
                continue
            with self.subTest(workflow=workflow):
                events = body.split("\npermissions:", 1)[0]
                self.assertIn("      - '.github/workflows/pr-changes.yml'\n", events)

    def test_matrix_admission_skips_empty_but_failure_uses_full_even_with_partial_outputs(self):
        for workflow, job, full in (
            ("agentplugins-native-clients.yml", "native-client", self.full_native),
            ("live-install-audit.yml", "real-native-cli-matrix", self.full_live),
            ("live-install-remaining-clients.yml", "remaining-client-install", self.full_remaining),
        ):
            body = self.job(workflow, job)
            condition = re.search(r"(?m)^    if: (.+)$", body)[1]
            matrix = re.search(r"(?m)^        (?:client|target): (\$\{\{.+\}\})$", body)[1]
            self.assertIn("!cancelled()", condition)  # overrides implicit success() after needs failure
            for result, selection, admitted, expected in (
                ("success", "[]", False, []),
                ("success", json.dumps(full[:1]), True, full[:1]),
                ("failure", "", True, full),
                ("failure", "[]", True, full),
                ("failure", json.dumps(full[:1]), True, full),
            ):
                with self.subTest(workflow=workflow, result=result, selection=selection):
                    self.assertEqual(self.expression(condition, result=result, selection=selection), admitted)
                    self.assertEqual(self.expression(matrix, result=result, selection=selection), expected)
                    self.assertFalse(self.expression(condition, result=result, selection=selection, cancelled=True))
            if job == "real-native-cli-matrix":
                for pilot in ("failure", "cancelled", "skipped"):
                    self.assertFalse(self.expression(condition, result="failure", pilot=pilot))
            with self.assertRaises(json.JSONDecodeError):
                self.expression(matrix, result="success", selection="{malformed")

    def test_live_pilot_and_historical_smoke_preserve_real_prerequisite_and_event_policy(self):
        body = self.job("live-install-audit.yml", "native-pilot")
        condition = re.search(r"(?m)^    if: (.+)$", body)[1]
        self.assertIn("!cancelled()", condition)
        for result in ("success", "failure"):
            self.assertTrue(self.expression(condition, result=result, selection="[]"))
            self.assertFalse(self.expression(condition, result=result, cancelled=True))
            self.assertFalse(self.expression(condition, result=result, draft=True))
            self.assertFalse(self.expression(condition, result=result, action="labeled", label="unrelated"))
            self.assertTrue(self.expression(condition, result=result, draft=True, action="labeled",
                                            label="ci:full", labels=("ci:full",)))
        smoke = self.job("live-install-audit.yml", "published-cli-smoke")
        self.assertIn("    needs: native-pilot\n", smoke)
        self.assertNotRegex(smoke, r"(?m)^    if:")
        remaining = self.job("live-install-remaining-clients.yml", "remaining-client-install")
        condition = re.search(r"(?m)^    if: (.+)$", remaining)[1]
        for kwargs in ({"draft": True}, {"action": "labeled", "label": "unrelated"}):
            self.assertFalse(self.expression(condition, result="failure", **kwargs))
        self.assertTrue(self.expression(condition, result="failure", draft=True, action="labeled",
                                        label="ci:full", labels=("ci:full",)))

    def test_opencode_v2_admission_preserves_selection_and_failure_fallback(self):
        body = self.job("agentplugins-native-clients.yml", "opencode-v2-project")
        condition = re.search(r"(?m)^    if: (.+)$", body)[1]
        for result, selection, admitted in (
            ("success", "[]", False), ("success", '["claude"]', False),
            ("success", '["opencode"]', True), ("failure", '["claude"]', True),
        ):
            with self.subTest(result=result, selection=selection):
                self.assertEqual(self.expression(condition, result=result, selection=selection), admitted)
                self.assertFalse(self.expression(condition, result=result, selection=selection, cancelled=True))
                self.assertFalse(self.expression(condition, result=result, selection=selection,
                                                 event="workflow_dispatch", sdk_only=True))


if __name__ == "__main__":
    unittest.main()
