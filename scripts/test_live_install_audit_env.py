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
                "source": "777genius/atlassian", "directory": directory,
                "clients": [verified]}
        def matching():
            return audit.matching_installations(
                {"data": {"installations": [node]}}, "atlassian", "codex",
                "plugin", "atlassian",
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
            self.assertTrue(audit.preserved_regular_text(foreign, "keep\n"))
            foreign.unlink()
            foreign.symlink_to(matching)
            self.assertFalse(audit.preserved_regular_text(foreign, "keep\n"))
            foreign.unlink()
            self.assertFalse(audit.preserved_regular_text(foreign, "keep\n"))

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


if __name__ == "__main__":
    unittest.main()
