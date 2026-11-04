"""Prevent the live acceptance harness from touching ambient client profiles."""
import importlib.util
import os
from pathlib import Path
from tempfile import TemporaryDirectory
import unittest
from unittest.mock import patch


def load_audit():
    file = Path(__file__).resolve().parent / "live-install-audit.py"
    spec = importlib.util.spec_from_file_location("live_install_audit", file)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class IsolatedClientEnvironmentTest(unittest.TestCase):
    def test_rebinds_all_client_overrides_and_strips_git_injection(self):
        audit = load_audit()
        hostile = {key: "/outside/actual-user-profile" for key in audit.PROFILE_ENV_KEYS}
        hostile.update({
            "HOME": "/outside/actual-user-home",
            "USERPROFILE": "/outside/actual-user-home",
            "XDG_CONFIG_HOME": "/outside/config",
            "XDG_DATA_HOME": "/outside/data",
            "APPDATA": "/outside/real/windows-roaming",
            "LOCALAPPDATA": "/outside/real/windows-local",
            "GIT_CONFIG_COUNT": "1",
            "GIT_CONFIG_KEY_0": "core.hooksPath",
            "GIT_CONFIG_VALUE_0": "/outside/hooks",
            "GIT_SSH_COMMAND": "/outside/host-ssh",
            "SSH_ASKPASS": "/outside/host-askpass",
        })
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve() / "case"
            home, data = case / "home", case / "uap-data"
            home.mkdir(parents=True)
            data.mkdir()
            with patch.dict(os.environ, hostile):
                env = audit.isolated_client_env(home, case, data)
            for key in audit.PROFILE_ENV_KEYS:
                if key in env:
                    self.assertTrue(Path(env[key]).resolve().is_relative_to(home.resolve()), (key, env[key]))
                else:
                    self.assertNotIn(key, env)
            for key in ("HOME", "USERPROFILE", "XDG_CONFIG_HOME", "XDG_DATA_HOME",
                        "XDG_CACHE_HOME", "XDG_STATE_HOME", "GIT_CONFIG_GLOBAL",
                        "APPDATA", "LOCALAPPDATA", "AGENTPLUGINS_HOME", "TMPDIR"):
                self.assertTrue(Path(env[key]).resolve().is_relative_to(case.resolve()), (key, env[key]))
            self.assertEqual(env["GEMINI_CLI_HOME"], str(home))
            for key in ("GIT_CONFIG_COUNT", "GIT_CONFIG_KEY_0", "GIT_CONFIG_VALUE_0",
                        "GIT_SSH_COMMAND", "SSH_ASKPASS"):
                self.assertNotIn(key, env)

    def test_drops_unrelated_credentials_but_retains_public_runtime_plumbing(self):
        audit = load_audit()
        trusted = {
            "PATH": "/usr/local/bin:/usr/bin:/bin",
            "HTTPS_PROXY": "http://proxy.invalid:3128",
            "LANG": "C.UTF-8",
        }
        canaries = {
            "OPENAI_API_KEY": "never-forward-openai-canary",
            "ANTHROPIC_API_KEY": "never-forward-anthropic-canary",
            "GH_TOKEN": "never-forward-gh-canary",
            "GITHUB_TOKEN": "never-forward-github-canary",
            "ACTIONS_RUNTIME_TOKEN": "never-forward-actions-canary",
            "AWS_SECRET_ACCESS_KEY": "never-forward-aws-canary",
            "NPM_TOKEN": "never-forward-npm-canary",
            "SSH_AUTH_SOCK": "/outside/ssh-agent.sock",
            "GIT_CONFIG_COUNT": "1",
            "GIT_CONFIG_KEY_0": "core.hooksPath",
            "GIT_CONFIG_VALUE_0": "/outside/hooks",
            "CODEX_HOME": "/outside/real-codex-profile",
        }
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve() / "case"
            home, data = case / "home", case / "uap-data"
            home.mkdir(parents=True)
            data.mkdir()
            with patch.dict(os.environ, {**trusted, **canaries}, clear=True):
                env = audit.isolated_client_env(home, case, data)
        for key, value in trusted.items():
            self.assertEqual(env[key], value)
        for key in canaries:
            if key != "CODEX_HOME":
                self.assertNotIn(key, env)
        self.assertTrue(Path(env["CODEX_HOME"]).is_relative_to(home))
        self.assertEqual(env["CI"], "true")

    def test_version_evidence_is_short_and_never_claims_failed_probe(self):
        audit = load_audit()
        with patch.object(audit, "call", return_value={
            "stdout_tail": "codex-cli 0.153.4\n" + ("A" * 500),
            "stderr_tail": "", "exit_code": 0, "timeout": False,
        }) as run:
            reported = audit.version_evidence(["codex", "--version"], {"HOME": "/isolated"}, "/tmp")
            self.assertEqual(reported["status"], "reported")
            self.assertLessEqual(len(reported["value"]), 200)
            self.assertEqual(run.call_args.args[1], {"HOME": "/isolated"})
        for exit_code, stdout, timeout, status in [
            (2, "version 1", False, "probe-failed"),
            (0, "", False, "empty-output"),
            (None, "", True, "timeout"),
        ]:
            with patch.object(audit, "call", return_value={
                "stdout_tail": stdout, "stderr_tail": "",
                "exit_code": exit_code, "timeout": timeout,
            }):
                self.assertEqual(audit.version_evidence(["client", "--version"], {}, "/tmp")["status"], status)

    def test_refuses_case_data_outside_disposable_root(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            parent = Path(temporary).resolve()
            case, home, outside = parent / "case", parent / "case/home", parent / "outside"
            home.mkdir(parents=True)
            outside.mkdir()
            with self.assertRaisesRegex(ValueError, "disposable case"):
                audit.isolated_client_env(home, case, outside)

    def test_success_diagnostics_and_timeout_are_never_source_refusals(self):
        audit = load_audit()
        for exit_code in (0, None):
            self.assertFalse(audit.is_cli_refusal({
                "exit_code": exit_code,
                "stdout_tail": '{"result":"success","component_support":"unsupported"}',
                "stderr_tail": "target_unavailable",
            }))
        self.assertTrue(audit.is_cli_refusal({
            "exit_code": 1,
            "stderr_tail": "release does not support complete target set",
        }))

    def test_every_native_plugin_client_preserves_foreign_profile_file(self):
        audit = load_audit()
        refused = {"exit_code": 1, "stdout_tail": "", "stderr_tail": "test refusal",
                   "timeout": False, "_json": None}
        with TemporaryDirectory() as temporary:
            with patch.object(audit, "call", return_value=refused):
                for target, roots in audit.NATIVE_ROOTS.items():
                    if not roots:
                        continue  # ChatGPT is remote/manual, no fabricated native profile
                    with self.subTest(target=target):
                        runroot = Path(temporary).resolve() / target
                        evidence = audit.do_case(
                            "/unused/audit-cli", target, "plugin", "atlassian", 0,
                            runroot, {}
                        )
                        foreign = runroot / "plugin-00/home" / roots[0] / "foreign-neighbor.txt"
                        self.assertTrue(foreign.is_file())
                        self.assertEqual(foreign.read_text(),
                                         "unmanaged native neighbor must survive\n")
                        self.assertTrue(evidence["sentinels_preserved"])


    def test_relocated_skill_payload_never_qualifies_as_removed(self):
        # BUG-027: dropping SKILL.md and moving a payload used to evade
        # both the identity scan and the known installation path check.
        import shutil
        audit = load_audit()
        item = {"name": "demo-skill", "source": "https://github.com/example/repo",
                "skill_md_git_blob_sha1": "a" * 40}
        installed = [{"name": "demo-skill", "installation_id": "owned-install",
                      "clients": [{"client_id": "claude"}]}]
        def success(rows=None):
            return {"exit_code": 0, "stdout_tail": "", "stderr_tail": "",
                    "timeout": False,
                    "_json": {"data": {"installations": rows or []}}}

        for relocate in (True, False):
            with self.subTest(relocate=relocate), TemporaryDirectory() as tmp:
                state = {"added": 0, "removed": False}
                project = Path(tmp) / "skill-00/project"

                def fake_call(argv, env, cwd, timeout=105):
                    operation = argv[1]
                    skill_dir = project / ".claude/skills/demo-skill"
                    if operation == "skills":
                        if state["added"] == 0:
                            (skill_dir / "scripts").mkdir(parents=True)
                            (skill_dir / "SKILL.md").write_text("name: demo-skill\n")
                            (skill_dir / "scripts/payload.sh").write_text("echo payload\n")
                            (project / "uap.toml").write_text("demo-skill")
                            (project / "uap.lock").write_text("demo-skill")
                        state["added"] += 1
                    elif operation == "remove":
                        if relocate:
                            moved = skill_dir.with_name("other")
                            skill_dir.rename(moved)
                            (moved / "SKILL.md").unlink()
                        else:
                            shutil.rmtree(skill_dir)
                        (project / "uap.toml").unlink()
                        (project / "uap.lock").unlink()
                        state["removed"] = True
                    elif operation == "list":
                        return success([] if state["removed"] else installed)
                    else:
                        raise AssertionError(argv)
                    return success()

                with patch.object(audit, "call", side_effect=fake_call), \
                     patch.object(audit, "matching_installations", return_value=installed), \
                     patch.object(audit, "locked_skill_is_pinned", return_value=True), \
                     patch.object(audit, "matches_pinned_skill_bytes", return_value=True):
                    evidence = audit.do_case(
                        "/unused/agentplugins", "claude", "skill", item, 0,
                        Path(tmp), {})

                self.assertTrue(evidence["sentinels_preserved"])
                self.assertIsNone(evidence["post_removal_skill"])
                self.assertEqual(evidence["post_removal_leftovers"], [])
                if relocate:
                    self.assertIn("skills/other/scripts/payload.sh",
                                  evidence["post_skill_tree_delta"])
                    self.assertFalse(evidence["removal_verified"])
                    self.assertEqual(evidence["status"], "cleanup-failed")
                else:
                    self.assertEqual(evidence["post_skill_tree_delta"], [])
                    self.assertTrue(evidence["removal_verified"])
                    self.assertEqual(evidence["status"], "passed")

    def test_codex_remove_attests_only_pinned_absolute_executable(self):
        # BUG-028: an untrusted package must not shadow codex on PATH.
        audit = load_audit()
        with TemporaryDirectory() as tmp:
            case = Path(tmp).resolve() / "case"
            home = case / "home"
            conf = home / ".codex/config.toml"
            conf.parent.mkdir(parents=True)
            market_dir = case / "managed-marketplace"
            market_dir.mkdir(parents=True)
            market = "agentplugins-012345abcdef"
            conf.write_text(
                '[plugins."demo@' + market + '"]\nenabled = true\n'
                '[marketplaces.' + market + ']\nsource_type = "local"\n'
                'source = "' + str(market_dir) + '"\n')
            commands = []
            def fake_call(argv, env, cwd, timeout=40):
                commands.append(argv)
                if len(commands) == 2:
                    conf.write_text("")
                return {"exit_code": 0, "stdout_tail": "", "stderr_tail": "",
                        "timeout": False, "_json": None}
            pinned = case / "trusted-codex"
            pinned.write_text("test executable")
            with patch.dict(audit.AGENT_BINARIES, {"codex": str(pinned)}, clear=True), \
                 patch.object(audit, "call", side_effect=fake_call):
                result = audit.uninstall_native_codex_plugin(
                    "demo", home, case, {"HOME": str(home)}, case)
            self.assertTrue(result["completed"])
            self.assertEqual(len(commands), 2)
            self.assertTrue(all(cmd[0] == str(pinned) for cmd in commands))
            with patch.dict(audit.AGENT_BINARIES, {}, clear=True), \
                 patch.object(audit, "call") as forbidden:
                refused = audit.uninstall_native_codex_plugin(
                    "demo", home, case, {}, case)
                forbidden.assert_not_called()
            self.assertFalse(refused["completed"])
            self.assertIn("pinned", refused["reason"])

    def test_chatgpt_prepared_requires_registration_and_verified_cleanup(self):
        # BUG-013: a zero-exit CLI can still leave no managed installation.
        audit = load_audit()
        source = "github:example/chatgpt-demo@" + ("a" * 40)
        identity = "chatgpt-demo"
        matched = [{"name": identity, "installation_id": "isolated-owned-install",
                    "clients": [{"client_id": "chatgpt",
                                 "verification": "installation_verified"}]}]

        def response(exit_code, installations=None):
            return {"exit_code": exit_code, "stdout_tail": "", "stderr_tail": "",
                    "timeout": False,
                    "_json": {"data": {"installations": installations or []}}}

        def run_case(outputs, matches):
            with TemporaryDirectory() as temporary:
                with patch.object(audit, "call", side_effect=outputs), \
                     patch.object(audit, "matching_installations", return_value=matches):
                    return audit.do_case(
                        "/unused/agentplugins", "chatgpt", "plugin", source, 0,
                        Path(temporary).resolve(), {source: identity})

        empty = run_case([response(0), response(0)], [])
        self.assertEqual(empty["status"], "incomplete-native-proof")
        self.assertFalse(empty["listed_identity"])
        self.assertIsNone(empty["cleanup"])

        # Initial add/list, repeat add/list, then failed remove.
        failed_remove = run_case(
            [response(0), response(0), response(0), response(0), response(1)], matched)
        self.assertEqual(failed_remove["status"], "cleanup-failed")
        self.assertFalse(failed_remove["removal_verified"])

        # Full lifecycle may qualify only with an exact, successful repeat.
        failed_repeat = run_case(
            [response(0), response(0), response(1), response(0), response(0), response(0)],
            matched)
        self.assertEqual(failed_repeat["status"], "repeat-add-unqualified")
        self.assertFalse(failed_repeat["repeat_verified"])
        self.assertTrue(failed_repeat["removal_verified"])

        prepared = run_case(
            [response(0), response(0), response(0), response(0), response(0), response(0)],
            matched)
        self.assertEqual(prepared["status"], "manual-setup-prepared")
        self.assertTrue(prepared["repeat_verified"])
        self.assertTrue(prepared["repeat_native_unchanged"])
        self.assertTrue(prepared["repeat_lock_unchanged"])
        self.assertEqual(prepared["repeat_installation_ids"], ["isolated-owned-install"])
        self.assertTrue(prepared["listed_identity"])
        self.assertTrue(prepared["removal_verified"])
        self.assertTrue(prepared["sentinels_preserved"])

    def test_both_native_matrices_bind_real_bootstrap_outcomes(self):
        # BUG-014: present binary + failed native --version must never produce PASS.
        # This static guard joins the GitHub step outcome to the harness refusal.
        workflows = (
            (".github/workflows/live-install-audit.yml", "native-install"),
            (".github/workflows/live-install-audit.yml", "native-install-pilot"),
            (".github/workflows/live-install-audit.yml", "published-install"),
            (".github/workflows/live-install-remaining-clients.yml", "vendor-install"),
        )
        for relative_path, step_id in workflows:
            with self.subTest(workflow=relative_path):
                workflow = (Path(__file__).resolve().parents[1] / relative_path).read_text()
                self.assertIn(f"id: {step_id}", workflow)
                self.assertIn(
                    "AUDIT_VENDOR_OUTCOME: ${{ steps." + step_id + ".outcome }}",
                    workflow,
                )

    def test_reports_distinct_pr_head_and_checked_merge_sha(self):
        # BUG-015: pull_request GITHUB_SHA is a merge checkout, not PR head.
        import json
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            with patch.dict(os.environ, {
                "GITHUB_SHA": "a" * 40,
                "AUDIT_PR_HEAD_SHA": "b" * 40,
                "AUDIT_VENDOR_OUTCOME": "failure",
            }), patch("sys.argv", [
                "live-install-audit.py", "--cli", "/unused/agentplugins",
                "--target", "codex", "--kind", "skill", "--limit", "1",
                "--report-dir", temporary,
            ]):
                self.assertEqual(audit.main(), 2)
            report = json.loads((Path(temporary) / "codex-skill.json").read_text())
        self.assertEqual(report["github_sha"], "a" * 40)
        self.assertEqual(report["pr_head_sha"], "b" * 40)
        self.assertEqual(report["results"][0]["status"], "client-unavailable")

    def test_both_matrix_workflows_export_pr_source_head(self):
        for relative_path in (
            ".github/workflows/live-install-audit.yml",
            ".github/workflows/live-install-remaining-clients.yml",
        ):
            with self.subTest(workflow=relative_path):
                workflow = (Path(__file__).resolve().parents[1] / relative_path).read_text()
                self.assertIn("AUDIT_PR_HEAD_SHA: ${{ github.event.pull_request.head.sha || '' }}", workflow)

    def test_failed_vendor_bootstrap_cannot_inherit_unrelated_host_binary(self):
        audit = load_audit()
        with patch.dict(os.environ, {"AUDIT_VENDOR_OUTCOME": "failure"}):
            self.assertFalse(audit.vendor_bootstrap_attested())
        with patch.dict(os.environ, {"AUDIT_VENDOR_OUTCOME": "success"}):
            self.assertTrue(audit.vendor_bootstrap_attested())
        with patch.dict(os.environ, {}, clear=True):
            self.assertFalse(audit.vendor_bootstrap_attested())
        with patch.dict(os.environ, {"AUDIT_VENDOR_OUTCOME": ""}):
            self.assertFalse(audit.vendor_bootstrap_attested())

    def test_vendor_bootstrap_requires_reviewed_bytes_and_preserves_runner_toolchain(self):
        workflow = (Path(__file__).resolve().parents[1] /
                    ".github/workflows/live-install-remaining-clients.yml").read_text()
        self.assertIn("sha256_file -c -", workflow)
        self.assertIn("observed-untrusted.sha256", workflow)
        self.assertIn("AUDIT_VENDOR_OUTCOME", workflow)
        self.assertIn("shasum -a 256", workflow)
        self.assertIn("verification-blocked.txt", workflow)
        self.assertIn("dpkg-deb --extract", workflow)
        self.assertNotIn("sudo DEBIAN_FRONTEND", workflow)
        self.assertNotIn('export PATH="$HOME/.local/bin:$PATH"', workflow)
        self.assertIn(":/usr/bin:/sbin:/bin:", workflow)

    def test_failed_add_inspects_pending_registration_without_false_matches(self):
        audit = load_audit()
        pending = {"data": {"installations": [
            {"name": "atlassian", "clients": [
                {"client_id": "grok", "verification": "pending"}]},
            {"name": "other", "clients": [{"client_id": "codex"}]},
        ]}}
        self.assertTrue(audit.has_target_registration(pending, "atlassian", "grok"))
        self.assertFalse(audit.has_target_registration(pending, "atlassian", "codex"))
        self.assertFalse(audit.has_target_registration(pending, "other", "grok"))
        for malformed in (None, [], {}, {"data": {"installations": "bad"}},
                          {"data": {"installations": [None, "bad"]}}):
            self.assertFalse(audit.has_target_registration(malformed, "atlassian", "grok"))

    def test_signed_release_observation_only_exposes_public_allowlisted_fields(self):
        # BUG-025: diagnose revision drift without logging credentials.
        audit = load_audit()
        response = {"data": {"installations": [
            {"name": "atlassian", "source": "777genius/universal-agent-plugins/plugins/atlassian",
             "api_key": "secret-from-account",
             "directory": {"recorded_distribution": "777genius/atlassian",
                           "recorded_revision": "a" * 40,
                           "recorded_release_sequence": 1,
                           "private_config": "secret"},
             "clients": [
                 {"client_id": "codex",
                  "package_revision": {
                      "resolved_revision": "b" * 40, "release_sequence": 1,
                      "tree_digest": "sha256:" + "c" * 64,
                      "manifest_digest": "sha256:" + "d" * 64,
                      "credential": "secret"}},
                 {"client_id": "claude",
                  "package_revision": {"credential": "other-secret"}},
             ]}
        ]}}
        observed = audit.signed_release_public_observation(
            response, "atlassian", "codex")
        self.assertEqual(len(observed), 1)
        self.assertEqual(observed[0]["source"], "777genius/universal-agent-plugins/plugins/atlassian")
        self.assertEqual(observed[0]["resolved_revision"], "b" * 40)
        self.assertNotIn("secret", repr(observed))
        response["data"]["installations"][0]["source"] = (
            "https://alice:secret@host.invalid/private")
        observed = audit.signed_release_public_observation(
            response, "atlassian", "codex")
        self.assertIsNone(observed[0]["source"])
        self.assertEqual(audit.signed_release_public_observation(
            response, "atlassian", "grok"), [])


    def test_signed_release_requires_same_verified_target_and_immutable_metadata(self):
        audit = load_audit()
        directory = {
            "product_id": "atlassian",
            "recorded_distribution": "777genius/atlassian",
            "recorded_revision": "a" * 40,
            "recorded_release_sequence": 1,
        }
        revision = {
            "distribution_id": "777genius/atlassian",
            "resolved_revision": "a" * 40,
            "release_sequence": 1,
            "tree_digest": "sha256:" + "b" * 64,
            "manifest_digest": "sha256:" + "c" * 64,
        }
        verified = {"client_id": "codex", "verification": "installation_verified",
                    "package_revision": revision}
        node = {"name": "atlassian", "installation_id": "valid-id",
                "source": "777genius/universal-agent-plugins/plugins/atlassian", "directory": directory,
                "clients": [verified]}
        pins = {"atlassian": {
            "distribution": "777genius/atlassian",
            "repository": "777genius/universal-agent-plugins",
            "path": "plugins/atlassian", "revision": "a" * 40,
            "sequence": 1, "tree_digest": "sha256:" + "b" * 64,
            "manifest_digest": "sha256:" + "c" * 64,
        }}
        def matching():
            return audit.matching_installations(
                {"data": {"installations": [node]}}, "atlassian", "codex",
                "plugin", "atlassian", pins,
            )
        self.assertEqual(len(matching()), 1)
        for key, value in (("resolved_revision", "d" * 40),
                           ("distribution_id", "wrong/package"),
                           ("release_sequence", 2),
                           ("tree_digest", "sha256:short"),
                           ("manifest_digest", None)):
            with self.subTest(key=key):
                broken = dict(revision, **{key: value})
                node["clients"] = [dict(verified, package_revision=broken)]
                self.assertEqual(matching(), [])
        node["clients"] = [
            dict(verified, package_revision={}),
            {"client_id": "codex", "verification": "pending",
             "package_revision": revision},
        ]
        self.assertEqual(matching(), [])
        node["clients"] = [dict(verified, client_id="grok")]
        self.assertEqual(matching(), [])

    def test_signed_release_rejects_forged_source_even_when_self_consistent(self):
        audit = load_audit()
        pin = {
            "distribution": "777genius/atlassian",
            "repository": "777genius/universal-agent-plugins",
            "path": "plugins/atlassian", "revision": "a" * 40,
            "sequence": 1, "tree_digest": "sha256:" + "b" * 64,
            "manifest_digest": "sha256:" + "c" * 64,
        }
        directory = {"product_id": "atlassian", "recorded_distribution": pin["distribution"],
                     "recorded_revision": pin["revision"], "recorded_release_sequence": 1}
        revision = {"distribution_id": pin["distribution"], "resolved_revision": pin["revision"],
                    "release_sequence": 1, "tree_digest": pin["tree_digest"],
                    "manifest_digest": pin["manifest_digest"]}
        record = {"name": "atlassian", "installation_id": "proof",
                  "source": "777genius/universal-agent-plugins/plugins/atlassian", "directory": directory,
                  "clients": [{"client_id": "codex", "verification": "installation_verified",
                               "package_revision": revision}]}
        def qualify():
            return audit.matching_installations(
                {"data": {"installations": [record]}}, "atlassian", "codex",
                "plugin", "atlassian", {"atlassian": pin})
        self.assertEqual(len(qualify()), 1)
        for fake in ("evil/atlassian-fork", pin["distribution"], None):
            record["source"] = fake
            self.assertEqual(qualify(), [])
        record["source"] = "777genius/universal-agent-plugins/plugins/atlassian"
        original = record["directory"]
        record["directory"] = dict(original, recorded_revision="d" * 40)
        record["clients"][0]["package_revision"] = dict(
            revision, resolved_revision="d" * 40)
        self.assertEqual(qualify(), [])
        record["directory"] = original
        record["clients"][0]["package_revision"] = dict(
            revision, tree_digest="sha256:" + "d" * 64)
        self.assertEqual(qualify(), [])
        self.assertEqual(audit.matching_installations(
            {"data": {"installations": [record]}}, "atlassian", "codex",
            "plugin", "atlassian"), [])

    def test_frozen_inventory_requires_pins_and_null_revision_is_unqualified(self):
        import json
        audit = load_audit()
        doc = json.loads(audit.FIXTURES.read_text())
        audit.validate_fixtures(doc)
        self.assertEqual(len(doc["signed_release_pins"]), 21)
        missing = json.loads(json.dumps(doc))
        del missing["signed_release_pins"]["atlassian"]
        with self.assertRaisesRegex(ValueError, "Missing frozen Registry"):
            audit.validate_fixtures(missing)
        null_pin = doc["signed_release_pins"]["cloudflare"]
        self.assertIsNone(null_pin["revision"])
        node = {"name": "cloudflare", "source": null_pin["distribution"],
                "installation_id": "unqualified",
                "directory": {"product_id": "cloudflare",
                              "recorded_distribution": null_pin["distribution"],
                              "recorded_revision": "a" * 40,
                              "recorded_release_sequence": null_pin["sequence"]},
                "clients": [{"client_id": "claude",
                             "verification": "installation_verified",
                             "package_revision": {
                                 "distribution_id": null_pin["distribution"],
                                 "resolved_revision": "a" * 40,
                                 "release_sequence": null_pin["sequence"],
                                 "tree_digest": null_pin["tree_digest"],
                                 "manifest_digest": null_pin["manifest_digest"]}}]}
        self.assertEqual(audit.matching_installations(
            {"data": {"installations": [node]}}, "cloudflare", "claude",
            "plugin", "cloudflare", doc["signed_release_pins"]), [])


    def test_direct_git_revision_must_belong_to_verified_client(self):
        audit = load_audit()
        sha = "a" * 40
        item = "github:obra/the-elements-of-style@" + sha
        node = {"name": "elements-of-style", "source": "obra/the-elements-of-style",
                "installation_id": "valid-id", "clients": [
                    {"client_id": "kiro", "verification": "installation_verified",
                     "package_revision": {}},
                    {"client_id": "kiro", "verification": "pending",
                     "package_revision": {"resolved_revision": sha}},
                ]}
        response = {"data": {"installations": [node]}}
        self.assertEqual(audit.matching_installations(
            response, "elements-of-style", "kiro", "plugin", item), [])

    def test_real_signed_release_target_refusal_is_not_an_installer_bug(self):
        audit = load_audit()
        reproduced = (
            'agentplugins: no eligible directory release for "marketing-kit": '
            '777genius/marketing-kit: release does not support complete target set; missing grok'
        )
        self.assertTrue(audit.source_target_incompatibility(reproduced))
        self.assertFalse(audit.source_target_incompatibility(
            "agentplugins: unexpected failure while writing native plugin configuration"))
        self.assertFalse(audit.source_target_incompatibility(
            "agentplugins: lintai security scan failed: exit status 2"))



    def test_native_snapshot_keeps_symlinks_and_unreadable_paths(self):
        # BUG-017: untrusted native entries must not disappear from evidence.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            home = Path(temporary)
            root = home / ".codex"
            root.mkdir()
            owned = root / "managed.yaml"
            owned.write_text("name: managed\n")
            outside = home / "unrelated.txt"
            outside.write_text("foreign\n")
            (root / "unknown-link").symlink_to(outside)
            (root / "linked-dir").symlink_to(home, target_is_directory=True)
            initial = audit.snapshot([root])
            self.assertEqual(initial[".codex/unknown-link"], "symlink")
            self.assertEqual(initial[".codex/linked-dir"], "symlink")
            self.assertIn(".codex/managed.yaml", initial)
            self.assertNotIn(".codex/linked-dir/unrelated.txt", initial)
            with patch.object(Path, "open", side_effect=OSError("unreadable")):
                invalid = audit.snapshot([root])
            self.assertEqual(invalid[".codex/managed.yaml"], "unreadable")
            self.assertIn("unknown-link: symlink", " ".join(
                audit.native_identity_residuals(
                    [root], "managed", [".codex/unknown-link"], "codex")))
            self.assertIn("managed.yaml", " ".join(
                audit.native_identity_residuals(
                    [root], "managed", [".codex/managed.yaml"], "codex")))
            (root / "unknown-link").unlink()
            self.assertFalse(audit.native_identity_residuals(
                [root], "managed", [".codex/unknown-link"], "codex"))

    def test_unreadable_walk_marker_cannot_disappear_from_cleanup_evidence(self):
        # BUG-017: os.walk onerror creates a synthetic path; lstat cannot find
        # that marker, but a failed enumeration must not be counted as clean.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary) / ".codex"
            root.mkdir()
            reported = audit.native_identity_residuals(
                [root], "atlassian",
                [".codex/restricted-dir/.walk-error"], "codex")
            self.assertEqual(reported, [
                ".codex/restricted-dir/.walk-error: unreadable-walk"])

    def test_foreign_sentinel_symlink_cannot_fake_preservation(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            home = Path(temporary)
            foreign = home / "foreign.txt"
            matching = home / "matching.txt"
            foreign.write_text("keep\n")
            matching.write_text("keep\n")
            self.assertTrue(audit.preserved_regular_text(foreign, "keep\n", home))
            foreign.unlink()
            foreign.symlink_to(matching)
            self.assertFalse(audit.preserved_regular_text(foreign, "keep\n", home))
            foreign.unlink()
            self.assertFalse(audit.preserved_regular_text(foreign, "keep\n", home))

    def test_foreign_sentinel_invalid_utf8_is_tampered_not_harness_error(self):
        # BUG-026: corrupt bytes must not abort the remaining lifecycle proof.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            foreign = case / "foreign.txt"
            foreign.write_bytes(b"\xff\xfe")
            self.assertFalse(audit.preserved_regular_text(foreign, "untouched\n", case))

    def test_copilot_settings_depth_limit_is_reported_as_truncated(self):
        # BUG-026: a sole depth-limited leaf must not be called fully parsed.
        import json
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            home = case / "home"
            settings = home / ".copilot/settings.json"
            settings.parent.mkdir(parents=True)
            settings.write_text(json.dumps({"plugins": {"atlassian": "enabled"}}),
                                encoding="utf-8")
            shallow = audit.copilot_settings_sanitized_shape(home, case, "atlassian")
            self.assertEqual(shallow["status"], "parsed")
            self.assertEqual(shallow["match_counts"]["exact_key"], 1)
            nested = {"atlassian": "enabled"}
            for _ in range(32):
                nested = {"nested": nested}
            settings.write_text(json.dumps({"plugins": nested}), encoding="utf-8")
            deep = audit.copilot_settings_sanitized_shape(home, case, "atlassian")
            self.assertEqual(deep["status"], "truncated")
            self.assertEqual(deep["match_counts"]["exact_key"], 0)

    def test_project_skill_intermediate_symlink_cannot_fake_cleanup(self):
        # BUG-023: project/.claude can be swapped even when SKILL.md is regular.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            case = root / "case"
            project = case / "project"
            foreign = root / "foreign"
            project.mkdir(parents=True)
            foreign_skill_root = foreign / "skills"
            (foreign_skill_root / "foreign-neighbor").mkdir(parents=True)
            (foreign_skill_root / "demo-skill").mkdir()
            marker = foreign_skill_root / "foreign-neighbor/SKILL.md"
            marker.write_text("unmanaged identical sentinel\n")
            (foreign_skill_root / "demo-skill/SKILL.md").write_text(
                "---\nname: demo-skill\n---\n")
            (project / ".claude").symlink_to(foreign, target_is_directory=True)
            spoofed = project / ".claude/skills/foreign-neighbor/SKILL.md"
            self.assertTrue(spoofed.is_file())
            self.assertFalse(audit.preserved_regular_text(
                spoofed, "unmanaged identical sentinel\n", case))
            self.assertFalse(audit.path_inside_case(spoofed, case))
            self.assertIsNone(audit.skill_file_present(project, "claude", "demo-skill"))
            leftovers = audit.skill_removal_leftovers(
                project, "claude", "demo-skill", ".claude/skills/demo-skill/SKILL.md")
            self.assertIn(".claude/skills: unsafe ancestor", leftovers)
            self.assertTrue(any("unsafe ancestor" in entry for entry in leftovers))
            # No helper is permitted to mutate the foreign user's Skill.
            self.assertEqual(marker.read_text(), "unmanaged identical sentinel\n")

    def test_skill_lock_requires_two_complete_sha256_digests(self):
        import json
        audit = load_audit()
        item = {"name": "standalone-skill", "folder": "standalone-skill",
                "repo": "upstream/skills", "ref": "a" * 40}
        package = {"alias": item["name"], "source": {"git": {
            "kind": "skill", "skill": item["name"],
            "repository": item["repo"], "commit": item["ref"],
            "resolved_skill_path": "skills/" + item["folder"]}},
            "targets": ["codex"],
            "tree_digest": "sha256:" + "b" * 64,
            "manifest_digest": "sha256:" + "c" * 64}
        with TemporaryDirectory() as temporary:
            project = Path(temporary)
            def qualified():
                (project / "uap.lock").write_text(
                    json.dumps({"packages": [package]}))
                return audit.locked_skill_is_pinned(project, item, "codex")
            self.assertTrue(qualified())
            for malformed in ("sha256:", "sha256:bad", "sha256:" + "g" * 64,
                              "sha256:" + "A" * 64, "sha256:" + "0" * 65):
                for field in ("tree_digest", "manifest_digest"):
                    with self.subTest(digest=malformed, field=field):
                        original = package[field]
                        package[field] = malformed
                        self.assertFalse(qualified())
                        package[field] = original

    def test_direct_git_repository_identity_must_be_exact(self):
        audit = load_audit()
        sha = "a" * 40
        item = "github:obra/the-elements-of-style@" + sha
        node = {"name": "elements-of-style",
                "source": "obra/the-elements-of-style",
                "installation_id": "verified",
                "clients": [{"client_id": "cline",
                             "verification": "installation_verified",
                             "package_revision": {"resolved_revision": sha}}]}
        def qualify():
            return audit.matching_installations(
                {"data": {"installations": [node]}},
                "elements-of-style", "cline", "plugin", item)
        self.assertEqual(len(qualify()), 1)
        for valid in ("github:obra/the-elements-of-style",
                      "https://github.com/obra/the-elements-of-style"):
            node["source"] = valid
            self.assertEqual(len(qualify()), 1)
        for spoof in ("mirror/obra/the-elements-of-style-fork",
                      "obra/the-elements-of-style-other",
                      "evil/obra/the-elements-of-style",
                      "obra/the-elements-of-style?unverified=true"):
            with self.subTest(source=spoof):
                node["source"] = spoof
                self.assertEqual(qualify(), [])

    def test_proxy_url_userinfo_is_not_inherited_by_child(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            case = Path(temporary) / "case"
            home, data = case / "home", case / "uap-data"
            home.mkdir(parents=True)
            data.mkdir()
            with patch.dict(os.environ, {
                "HTTPS_PROXY": "https://alice:secret@proxy.invalid:8443",
                "http_proxy": "http://user:pw@proxy.invalid:3128",
                "ALL_PROXY": "socks5://proxy.invalid:1080",
                "NO_PROXY": "localhost",
                "OPENAI_API_KEY": "never-pass",
            }, clear=True):
                env = audit.isolated_client_env(home, case, data)
            self.assertEqual(env["HTTPS_PROXY"], "https://proxy.invalid:8443")
            self.assertEqual(env["http_proxy"], "http://proxy.invalid:3128")
            self.assertEqual(env["ALL_PROXY"], "socks5://proxy.invalid:1080")
            self.assertEqual(env["NO_PROXY"], "localhost")
            self.assertNotIn("OPENAI_API_KEY", env)


    def test_qoder_passive_cache_does_not_hide_active_residue_or_symlinks(self):
        # BUG-018: observed Qoder cache and run manifests outlive native remove.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary) / ".qoder"
            cache = root / "plugins/cache/local/atlassian/0.1.0/plugin.json"
            log = root / "logs/runs/run-1/manifest.json"
            active = root / "plugins/installed_plugins_v2.json"
            settings = root / "settings.json"
            for path in (cache, log, active, settings):
                path.parent.mkdir(parents=True, exist_ok=True)
            cache.write_text('{"name":"atlassian"}')
            log.write_text('{"plugin":"atlassian"}')
            active.write_text('{"plugin":"atlassian"}')
            settings.write_text('{"mcpServers":{"atlassian":{}}}')
            observed = [
                ".qoder/plugins/cache/local/atlassian/0.1.0/plugin.json",
                ".qoder/logs/runs/run-1/manifest.json",
                ".qoder/plugins/installed_plugins_v2.json",
                ".qoder/settings.json",
            ]
            self.assertEqual(
                audit.native_identity_residuals([root], "atlassian", observed, "qoder"),
                [".qoder/plugins/installed_plugins_v2.json", ".qoder/settings.json"])
            active.write_text("{}")
            settings.write_text("{}")
            self.assertEqual(audit.native_identity_residuals(
                [root], "atlassian", observed, "qoder"), [])
            cache.unlink()
            cache.symlink_to(settings)
            self.assertEqual(audit.native_identity_residuals(
                [root], "atlassian", observed, "qoder"),
                [".qoder/plugins/cache/local/atlassian/0.1.0/plugin.json: symlink"])
            self.assertFalse(audit.passive_native_path("cline", "plugins/cache/local/demo"))
            self.assertFalse(audit.passive_native_path("qoder", "settings.json"))


    def test_post_remove_native_rescan_detects_late_registration(self):
        # BUG-020: a native setting written by remove after the add snapshot
        # must not be mistaken for a successful full lifecycle.
        audit = load_audit()
        identity = "atlassian"
        source = "github:example/native-plugin@" + ("a" * 40)
        matched = [{"name": identity, "installation_id": "owned-once",
                    "clients": [{"client_id": "grok", "verification": "installation_verified"}]}]

        def verify_case(late_registration):
            with TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                native = root / "plugin-00/home/.grok"
                managed = native / "managed-plugin.json"
                late = native / "new-settings.json"
                state = {"installed": False, "add_count": 0}

                def fake_call(argv, env, cwd, timeout=105):
                    operation = argv[1]
                    if operation == "add":
                        state["add_count"] += 1
                        if state["add_count"] == 1:
                            managed.write_text('{"name":"atlassian"}')
                        state["installed"] = True
                    elif operation == "remove":
                        managed.unlink()
                        if late_registration:
                            late.write_text('{"nativePlugin":"atlassian"}')
                        state["installed"] = False
                    installations = matched if state["installed"] else []
                    return {"exit_code": 0, "stdout_tail": "", "stderr_tail": "",
                            "timeout": False,
                            "_json": {"data": {"installations": installations}}}

                with patch.object(audit, "call", side_effect=fake_call), \
                     patch.object(audit, "matching_installations", return_value=matched):
                    evidence = audit.do_case("/unused/agentplugins", "grok", "plugin",
                                             source, 0, root, {source: identity})
                self.assertTrue(evidence["listed_identity"])
                self.assertTrue(evidence["repeat_verified"])
                self.assertTrue(evidence["sentinels_preserved"])
                self.assertTrue(evidence["post_list_schema_valid"])
                self.assertEqual(evidence["cleanup"]["exit_code"], 0)
                self.assertIn(".grok/managed-plugin.json", evidence["native_identity_files"])
                return evidence

        clean = verify_case(False)
        self.assertEqual(clean["status"], "passed")
        self.assertTrue(clean["removal_verified"])
        self.assertEqual(clean["native_residuals"], [])

        delayed = verify_case(True)
        self.assertEqual(delayed["status"], "cleanup-failed")
        self.assertFalse(delayed["removal_verified"])
        self.assertEqual(delayed["native_residuals"], [".grok/new-settings.json"])



    def test_post_remove_reminted_identity_under_new_id_fails_cleanup(self):
        # BUG-022: disjoint old IDs do not prove this package/target is gone.
        audit = load_audit()
        source = "github:example/native-plugin@" + ("a" * 40)
        old = {"name": "atlassian", "installation_id": "owned-old",
               "clients": [{"client_id": "grok",
                            "verification": "installation_verified"}]}
        reminted = {"name": "atlassian", "installation_id": "owned-new",
                    "clients": [{"client_id": "grok",
                                 "verification": "installation_verified"}]}
        with TemporaryDirectory() as temporary:
            runroot = Path(temporary).resolve()
            native = runroot / "plugin-00/home/.grok/managed-plugin.json"
            state = {"removed": False}

            def fake_call(argv, env, cwd, timeout=105):
                command = argv[1]
                if command == "add" and not native.exists():
                    native.write_text('{"name":"atlassian"}')
                elif command == "remove":
                    native.unlink()
                    state["removed"] = True
                entries = [reminted] if state["removed"] else [old]
                return {"exit_code": 0, "stdout_tail": "", "stderr_tail": "",
                        "timeout": False,
                        "_json": {"data": {"installations": entries}}}

            with patch.object(audit, "call", side_effect=fake_call):
                with patch.object(audit, "matching_installations", return_value=[old]):
                    result = audit.do_case(
                        "/unused/agentplugins", "grok", "plugin", source, 0,
                        runroot, {source: "atlassian"})
        self.assertTrue(result["listed_identity"])
        self.assertTrue(result["repeat_verified"])
        self.assertTrue(result["sentinels_preserved"])
        self.assertEqual(result["cleanup"]["exit_code"], 0)
        self.assertTrue(result["post_list_schema_valid"])
        self.assertEqual(result["native_residuals"], [])
        self.assertEqual(result["matched_installation_ids"], ["owned-old"])
        self.assertTrue(result["post_remove_identity_registered"])
        self.assertFalse(result["removal_verified"])
        self.assertEqual(result["status"], "cleanup-failed")


    def test_codex_native_registration_requires_exact_local_marketplace(self):
        # The actual Codex CLI owns [plugins."name@agentplugins-<hash>"].
        # A source cache, arbitrary URL or unrelated marketplace is not proof.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            case = Path(temporary)
            root = case / "home/.codex"
            root.mkdir(parents=True)
            managed = case / "uap-data/managed"
            managed.mkdir(parents=True)
            config = root / "config.toml"
            key = "agentplugins-" + "a" * 12
            config.write_text(
                f'[plugins."atlassian@{key}"]\nenabled = true\n'
                f'[marketplaces.{key}]\nsource_type = "local"\n'
                f'source = "{managed}"\n')
            changes = list(audit.snapshot([root]).items())
            self.assertEqual(
                audit.attributable_native_changes([root], changes, "atlassian", "codex"),
                [".codex/config.toml"])
            self.assertEqual(
                audit.attributable_native_changes([root], changes, "github", "codex"),
                [])
            self.assertFalse(audit.codex_owned_native_config(
                config, "atlassian-wrong", case))
            escaped = Path(temporary).parent / "other-account"
            config.write_text(config.read_text().replace(str(managed), str(escaped)))
            self.assertFalse(audit.codex_owned_native_config(config, "atlassian", case))
            config.write_text(
                config.read_text().replace(str(escaped), str(managed))
                .replace(f"agentplugins-{'a' * 12}", "random-market"))
            self.assertFalse(audit.codex_owned_native_config(config, "atlassian", case))

    def test_large_native_file_rewrite_changes_snapshot(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary) / ".grok"
            root.mkdir()
            native = root / "managed.json"
            native.write_bytes(b"A" * (5 * 1024 * 1024))
            before = audit.snapshot([root])
            native.write_bytes(b"B" * (5 * 1024 * 1024))
            after = audit.snapshot([root])
            self.assertNotEqual(before, after)
            self.assertEqual(len(before[".grok/managed.json"]), 64)
            self.assertNotIn("too-large", after.values())

    def test_native_attribution_excludes_cache_and_unrelated_url(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary) / ".qoder"
            cache = root / "plugins/cache/local/github/app.json"
            url = root / "settings.json"
            native = root / "plugins/installed_plugins_v2.json"
            for f in (cache, url, native):
                f.parent.mkdir(parents=True, exist_ok=True)
            cache.write_text('{"name":"github"}')
            url.write_text('{"repository":"https://github.com/other/repo"}')
            native.write_text('{"plugins":{"github":{"enabled":true}}}')
            files = audit.snapshot([root])
            changes = list(files.items())
            credited = audit.attributable_native_changes([root], changes, "github", "qoder")
            self.assertEqual(credited, [".qoder/plugins/installed_plugins_v2.json"])
            self.assertFalse(audit.exact_native_identity(
                'source = "https://github.com/example/package"', "github"))
            self.assertTrue(audit.exact_native_identity(
                'name = "github"', "github"))

    def test_native_and_temp_symlink_escape_rejected_before_untrusted_writes(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            case = root / "case"
            home = case / "home"
            data = case / "uap-data"
            foreign = root / "foreign"
            foreign.mkdir()
            home.mkdir(parents=True)
            data.mkdir()
            (home / ".codex").symlink_to(foreign, target_is_directory=True)
            with self.assertRaisesRegex(ValueError, "native profile"):
                audit.isolated_client_env(home, case, data)
            (home / ".codex").unlink()
            (case / "tmp").rmdir()
            (case / "tmp").symlink_to(foreign, target_is_directory=True)
            with self.assertRaisesRegex(ValueError, "temp root"):
                audit.isolated_client_env(home, case, data)

            future = root / "run" / "plugin-00"
            future.parent.mkdir()
            future.symlink_to(foreign, target_is_directory=True)
            with self.assertRaises(FileExistsError):
                audit.do_case("/unused/audit-cli", "codex", "plugin",
                              "atlassian", 0, future.parent, {})
            self.assertEqual(list(foreign.iterdir()), [])

    def test_skill_removal_rejects_surviving_links_and_native_registration(self):
        audit = load_audit()
        identity = "demo-skill"
        item = {"name": identity, "folder": identity, "repo": "example/skills",
                "ref": "a" * 40, "source": "https://github.com/example/skills",
                "skill_md_git_blob_sha1": "b" * 40}
        match = [{"name": identity, "installation_id": "skill-1",
                  "clients": [{"client_id": "claude",
                               "verification": "installation_verified"}]}]

        def check(residue):
            with TemporaryDirectory() as temporary:
                runroot = Path(temporary)
                project = runroot / "skill-00/project"
                native = runroot / "skill-00/home/.claude"
                skill = project / ".claude/skills/demo-skill/SKILL.md"
                state = {"installed": False}
                def fake_call(argv, env, cwd, timeout=105):
                    if argv[1] == "skills":
                        skill.parent.mkdir(parents=True, exist_ok=True)
                        skill.write_text("---\nname: demo-skill\n---\n")
                        (project / "uap.toml").write_text("demo-skill\n")
                        (project / "uap.lock").write_text('{"name":"demo-skill"}')
                        state["installed"] = True
                    elif argv[1] == "remove":
                        skill.unlink()
                        skill.parent.rmdir()
                        (project / "uap.lock").unlink()
                        state["installed"] = False
                        if residue == "symlink":
                            skill.parent.mkdir()
                            skill.symlink_to(project / "unmanaged")
                        elif residue == "native":
                            (native / "agent-registry.json").write_text(
                                '{"skills":{"demo-skill":{}}}')
                    return {"exit_code": 0, "stdout_tail": "", "stderr_tail": "",
                            "timeout": False,
                            "_json": {"data": {"installations":
                                               match if state["installed"] else []}}}
                with patch.object(audit, "call", side_effect=fake_call), \
                     patch.object(audit, "matching_installations", return_value=match), \
                     patch.object(audit, "locked_skill_is_pinned", return_value=True), \
                     patch.object(audit, "matches_pinned_skill_bytes", return_value=True):
                    evidence = audit.do_case("/unused/agentplugins", "claude", "skill",
                                             item, 0, runroot, {})
                self.assertTrue(evidence["installed"] if "installed" in evidence
                                else evidence["listed_identity"])
                self.assertTrue(evidence["repeat_verified"])
                return evidence
        clean = check(None)
        self.assertTrue(clean["removal_verified"])
        self.assertEqual(clean["status"], "passed")
        linked = check("symlink")
        self.assertFalse(linked["removal_verified"])
        self.assertEqual(linked["status"], "cleanup-failed")
        self.assertTrue(linked["post_removal_leftovers"])
        native = check("native")
        self.assertFalse(native["removal_verified"])
        self.assertEqual(native["status"], "cleanup-failed")
        self.assertTrue(native["native_residuals"])


    def test_copilot_settings_probe_redacts_values_and_keeps_refusals(self):
        # BUG-019: distinguish a qualified native key from incidental text,
        # never include native settings contents in retained E2E artifacts.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve() / "plugin-00"
            home = case / "home"
            settings = home / ".copilot/settings.json"
            settings.parent.mkdir(parents=True)
            self.assertEqual(
                audit.copilot_settings_sanitized_shape(home, case, "atlassian"),
                {"status": "absent"})
            settings.write_text(
                '{"plugins":{"atlassian@agentplugins-aaaaaaaaaaaa":true},'
                '"unmanaged":{"secret":"confidential-password-123",'
                '"url":"https://example.invalid/atlassian"}}')
            proof = audit.copilot_settings_sanitized_shape(home, case, "atlassian")
            self.assertEqual(proof["status"], "parsed")
            self.assertEqual(proof["allowed_sections"], ["plugins"])
            self.assertEqual(proof["match_counts"]["qualified_key"], 1)
            self.assertEqual(proof["match_counts"]["reference_value"], 1)
            self.assertEqual(proof["match_counts"]["exact_key"], 0)
            self.assertNotIn("confidential-password-123", str(proof))
            self.assertNotIn("example.invalid", str(proof))
            self.assertNotIn("unmanaged", str(proof))
            settings.write_text('{"plugins":{}}')
            clean = audit.copilot_settings_sanitized_shape(home, case, "atlassian")
            self.assertEqual(clean["status"], "parsed")
            self.assertFalse(any(clean["match_counts"].values()))

    def test_copilot_settings_probe_rejects_unsafe_paths_and_bad_json(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            case, outside = root / "case", root / "outside"
            settings = case / "home/.copilot/settings.json"
            settings.parent.mkdir(parents=True)
            outside.write_text('{"plugins":{"atlassian":true}}')
            settings.symlink_to(outside)
            self.assertEqual(audit.copilot_settings_sanitized_shape(
                case / "home", case, "atlassian")["status"], "unsafe-path")
            settings.unlink()
            settings.write_text('{"invalid-json"')
            self.assertEqual(audit.copilot_settings_sanitized_shape(
                case / "home", case, "atlassian")["status"],
                "unreadable-or-invalid-json")
            settings.write_bytes(b"X" * (1024 * 1024 + 1))
            self.assertEqual(audit.copilot_settings_sanitized_shape(
                case / "home", case, "atlassian")["status"],
                "not-regular-or-too-large")



    def test_qoder_foreign_skill_is_never_placed_in_uap_managed_bundle(self):
        audit = load_audit()
        item = {"name": "frontend-design", "source": "unused-fixture",
                "skill_md_git_blob_sha1": "0" * 40}
        refused = {"exit_code": 1, "stdout_tail": "", "stderr_tail": "test refusal",
                   "timeout": False, "_json": {"data": {"installations": []}}}
        with TemporaryDirectory() as temporary:
            runroot = Path(temporary).resolve()
            with patch.object(audit, "call", return_value=refused):
                row = audit.do_case("/unused/agentplugins", "qoder", "skill",
                                    item, 0, runroot, {})
            project = runroot / "skill-00/project"
            foreign = project / ".qoder/skills/foreign-neighbor/SKILL.md"
            managed_foreign = project / ".uap/managed/clients/qoder/foreign-neighbor"
            self.assertTrue(foreign.is_file())
            self.assertEqual(foreign.read_text(encoding="utf-8"),
                             "---\\nname: foreign-neighbor\\ndescription: unrelated\\n---\\n")
            self.assertFalse(managed_foreign.exists())
            self.assertTrue(row["sentinels_preserved"])

    def test_kimi_skill_uses_real_isolated_git_worktree(self):
        audit = load_audit()
        self.assertTrue(audit.GIT_EXECUTABLE and Path(audit.GIT_EXECUTABLE).is_file())
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve() / "case"
            home, project, data = case / "home", case / "project", case / "uap-data"
            for directory in (home, project, data):
                directory.mkdir(parents=True)
            env = audit.isolated_client_env(home, case, data)
            audit.initialize_kimi_project_git_root(project, env, case)
            self.assertTrue((project / ".git").is_dir())
            self.assertFalse((project / ".git").is_symlink())
            # Real Git must recognize this disposable root; an empty marker is not enough.
            checked = audit.call([audit.GIT_EXECUTABLE, "-C", str(project),
                                  "rev-parse", "--show-toplevel"], env, project, 30)
            self.assertEqual(checked["exit_code"], 0)
            self.assertEqual(Path(checked["stdout_tail"].strip()).resolve(), project)
            with self.assertRaisesRegex(RuntimeError, "prerequisite unavailable"):
                audit.initialize_kimi_project_git_root(project, env, case / "outside")

    def test_kimi_git_precondition_is_not_applied_to_other_lanes(self):
        audit = load_audit()
        failed = {"exit_code": 1, "stdout_tail": "", "stderr_tail": "test refusal",
                  "timeout": False, "_json": {"data": {"installations": []}}}
        fixture = {"name": "frontend-design", "source": "unused",
                   "skill_md_git_blob_sha1": "0" * 40}
        with (TemporaryDirectory() as temporary,
              patch.object(audit, "call", return_value=failed),
              patch.object(audit, "initialize_kimi_project_git_root") as initialize):
            root = Path(temporary).resolve()
            audit.do_case("/unused/agentplugins", "kimi", "plugin",
                          "atlassian", 0, root / "kimi-plugin", {})
            audit.do_case("/unused/agentplugins", "qoder", "skill",
                          fixture, 0, root / "qoder-skill", {})
            self.assertEqual(initialize.call_count, 0)
            audit.do_case("/unused/agentplugins", "kimi", "skill",
                          fixture, 0, root / "kimi-skill", {})
            self.assertEqual(initialize.call_count, 1)

    def test_qoder_latest_log_link_only_qualifies_closed_run_diagnostics(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve() / ".qoder"
            run = root / "logs" / "runs" / "session-1"
            run.mkdir(parents=True)
            (run / "qodercli.log").write_text("diagnostic only")
            link = root / "logs/latest"
            observed = [".qoder/logs/latest"]
            link.symlink_to("runs/session-1", target_is_directory=True)
            self.assertTrue(audit.qoder_passive_latest_log_link(root, link))
            self.assertEqual(audit.native_identity_residuals(
                [root], "atlassian", observed, "qoder"), [])
            self.assertEqual(audit.native_identity_residuals(
                [root], "atlassian", observed, "codex"),
                [".qoder/logs/latest: symlink"])
            link.unlink()
            outside = root.parent / "outside"
            outside.mkdir()
            link.symlink_to("../../outside", target_is_directory=True)
            self.assertFalse(audit.qoder_passive_latest_log_link(root, link))
            self.assertEqual(audit.native_identity_residuals(
                [root], "atlassian", observed, "qoder"),
                [".qoder/logs/latest: symlink"])
            link.unlink()
            link.symlink_to("runs/missing", target_is_directory=True)
            self.assertFalse(audit.qoder_passive_latest_log_link(root, link))
            link.unlink()
            (root / "logs/runs/chain").symlink_to(run, target_is_directory=True)
            link.symlink_to("runs/chain", target_is_directory=True)
            self.assertFalse(audit.qoder_passive_latest_log_link(root, link))
            link.unlink()
            other = root / "logs/other"
            other.symlink_to("runs/session-1", target_is_directory=True)
            self.assertEqual(audit.native_identity_residuals(
                [root], "atlassian", [".qoder/logs/other"], "qoder"),
                [".qoder/logs/other: symlink"])

    def test_case_creation_never_writes_through_planted_child_symlinks(self):
        audit = load_audit()
        original_mkdir = Path.mkdir
        with TemporaryDirectory() as temporary:
            outer = Path(temporary).resolve()
            foreign = outer / "foreign"
            foreign.mkdir()
            for poisoned in ("home", "local-parent"):
                with self.subTest(poisoned=poisoned):
                    runroot = outer / poisoned
                    case = runroot / "plugin-00"

                    injected = set()

                    def injected_mkdir(path, *args, **kwargs):
                        result = original_mkdir(path, *args, **kwargs)
                        # Path.mkdir(parents=True) may retry the same path.
                        # Plant a symlink exactly once, before the next guard.
                        if not injected and poisoned == "home" and path == case:
                            injected.add(poisoned)
                            (case / "home").symlink_to(foreign, target_is_directory=True)
                        elif not injected and poisoned == "local-parent" and path == case / "home":
                            injected.add(poisoned)
                            (case / "home/.local").symlink_to(
                                foreign, target_is_directory=True)
                        return result

                    with (patch.object(Path, "mkdir", injected_mkdir),
                          self.assertRaisesRegex(ValueError, "nested audit directory")):
                        audit.do_case("/unused/agentplugins", "codex", "plugin",
                                      "atlassian", 0, runroot, {})
                    self.assertEqual(injected, {poisoned})
                    self.assertEqual(list(foreign.iterdir()), [])

    def test_failed_first_list_cannot_attest_plugin_lifecycle(self):
        # BUG-029: matching JSON from a failed list is not an attestation.
        audit = load_audit()
        rows = [{"installation_id": "owned-id", "name": "atlassian",
                 "clients": [{"client_id": "claude"}]}]
        operations = []

        def fake_call(argv, env, cwd, timeout=105):
            operation = argv[1]
            operations.append(operation)
            first_list_failed = operation == "list" and operations.count("list") == 1
            return {"exit_code": 1 if first_list_failed else 0, "stdout_tail": "",
                    "stderr_tail": "", "timeout": False,
                    "_json": {"data": {"installations": rows}}}

        with TemporaryDirectory() as tmp, \
             patch.object(audit, "call", side_effect=fake_call), \
             patch.object(audit, "matching_installations", return_value=rows):
            result = audit.do_case("/unused/agentplugins", "claude", "plugin",
                                   "atlassian", 0, Path(tmp), {})
        self.assertEqual(result["install"]["exit_code"], 0)
        self.assertEqual(result["list"]["exit_code"], 1)
        self.assertTrue(result["listed_identity"])
        self.assertIsNone(result["repeat_add"])
        self.assertIsNone(result["cleanup"])
        self.assertEqual(operations, ["add", "list"])
        self.assertEqual(result["status"], "incomplete-native-proof")


if __name__ == "__main__":
    unittest.main()
