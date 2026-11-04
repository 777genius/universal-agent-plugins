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



if __name__ == "__main__":
    unittest.main()
