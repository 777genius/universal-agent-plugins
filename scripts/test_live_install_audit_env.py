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

    def test_refuses_case_data_outside_disposable_root(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            parent = Path(temporary).resolve()
            case, home, outside = parent / "case", parent / "case/home", parent / "outside"
            home.mkdir(parents=True)
            outside.mkdir()
            with self.assertRaisesRegex(ValueError, "disposable case"):
                audit.isolated_client_env(home, case, outside)

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
