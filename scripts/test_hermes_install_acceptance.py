"""Inert regressions for the builder-only native acceptance boundary."""
import base64
import hashlib
import io
from contextlib import ExitStack
from contextlib import redirect_stdout
import json
import os
import runpy
from pathlib import Path
import subprocess
import sys
import tempfile
import types
import unittest
from unittest.mock import patch

import hermes_install_acceptance as acceptance


class BuilderCleanupAcceptance(unittest.TestCase):
    def test_only_explicit_exact_builder_may_continue_with_original_failure(self):
        original_error = 'original forced cleanup diagnostic'
        base = {'result': {'ExitCode': 0, 'CleanupComplete': True, 'NoChildStarted': False,
                           'Stdout': base64.b64encode(b'original stdout').decode(),
                           'Stderr': base64.b64encode(b'original stderr').decode()},
                'error': original_error, 'forced_cleanup_only': True}
        cases = [('native default', False, True, 1, {}, False),
                 ('reviewed builder', True, True, 1, {}, True),
                 ('different command', True, False, 1, {}, False),
                 ('owner status mismatch', True, True, 0, {}, False),
                 ('joined or unknown cause', True, True, 1, {'forced_cleanup_only': False}, False),
                 ('unconfirmed cleanup', True, True, 1, {'CleanupComplete': False}, False),
                 ('failed native leader', True, True, 1, {'ExitCode': 9}, False),
                 ('unstarted native child', True, True, 1, {'NoChildStarted': True}, False)]
        for label, opt_in, exact, owner_code, changes, allowed in cases:
            with self.subTest(label=label), tempfile.TemporaryDirectory() as directory, ExitStack() as stack:
                packet = json.loads(json.dumps(base))
                for key, value in changes.items():
                    (packet['result'] if key in packet['result'] else packet)[key] = value
                owner = subprocess.CompletedProcess(['supervisor'], owner_code, json.dumps(packet).encode(), b'owner stderr')
                stack.enter_context(patch.dict(os.environ, {'RUNNER_TEMP': directory, 'HERMES_INSTALL_RUNNER': 'supervisor'}))
                stack.enter_context(patch.object(acceptance.subprocess, 'run', return_value=owner))
                stack.enter_context(patch.object(acceptance, 'reviewed_windows_builder', return_value=exact))
                if allowed:
                    record, stdout, stderr = acceptance.supervised(['exact builder'], Path(directory), os.environ,
                                                                  completed_builder_cleanup=opt_in)
                    self.assertEqual(record['error'], original_error)
                    self.assertTrue(record['builder_preparation_continued_after_forced_cleanup'])
                    self.assertEqual((stdout, stderr), (b'original stdout', b'original stderr'))
                else:
                    with self.assertRaises(RuntimeError):
                        acceptance.supervised(['command'], Path(directory), os.environ, completed_builder_cleanup=opt_in)
                evidence = list(Path(directory).glob('hermes-project-supervision-*'))
                self.assertEqual(len(evidence), 1)
                self.assertEqual((evidence[0] / 'owner.stdout').read_bytes(), owner.stdout)
                self.assertEqual((evidence[0] / 'native.stderr').read_bytes(), b'original stderr')
                self.assertEqual(json.loads((evidence[0] / 'record.json').read_text())['error'], original_error)

    def test_failed_command_reports_bounded_ascii_tails_and_preserves_raw_streams(self):
        stdout = b'prefix-not-in-tail' + b'x' * 5000 + '\u2713\n'.encode('utf-8')
        stderr = b'prefix-not-in-tail' + b'y' * 5000 + b'\xff\x1b[31merror\n'
        packet = {'result': {'ExitCode': 7, 'CleanupComplete': True, 'NoChildStarted': False,
                             'Stdout': base64.b64encode(stdout).decode(),
                             'Stderr': base64.b64encode(stderr).decode()}, 'error': None}
        owner = subprocess.CompletedProcess(['supervisor'], 0, json.dumps(packet).encode(), b'')
        with tempfile.TemporaryDirectory() as directory, ExitStack() as stack:
            stack.enter_context(patch.dict(os.environ, {'RUNNER_TEMP': directory, 'HERMES_INSTALL_RUNNER': 'supervisor'}))
            stack.enter_context(patch.object(acceptance.subprocess, 'run', return_value=owner))
            with self.assertRaises(RuntimeError) as failure:
                acceptance.supervised(['failed command'], Path(directory), os.environ)
            message = str(failure.exception)
            self.assertTrue(message.isascii())
            self.assertNotIn('prefix-not-in-tail', message)
            self.assertLess(len(message), 60 << 10)
            detail = json.loads(message.split(': ', 1)[1])
            self.assertEqual(detail['record']['exit_code'], 7)
            self.assertTrue(detail['record']['cleanup_complete'])
            for name, body in (('stdout', stdout), ('stderr', stderr)):
                self.assertEqual(detail['output_tails'][name], body[-4096:].decode('utf-8', errors='backslashreplace'))
                self.assertEqual(detail['record']['streams'][name]['bytes'], len(body))
            evidence = list(Path(directory).glob('hermes-project-supervision-*'))
            self.assertEqual(len(evidence), 1)
            self.assertEqual((evidence[0] / 'native.stdout').read_bytes(), stdout)
            self.assertEqual((evidence[0] / 'native.stderr').read_bytes(), stderr)

    def test_reviewed_builder_requires_exact_pinned_argument_vector(self):
        with tempfile.TemporaryDirectory() as directory, patch.dict(os.environ, {'RUNNER_TEMP': directory}):
            root = Path(directory).resolve()
            source = root / 'hermes-agent'
            argv = [str(root / 'python.exe'), '-S', '-B', str(source / 'scripts/bundles/native_build.py'),
                    '--source', str(source), '--work', str(root / 'hermes-project-work'),
                    '--cache', str(root / 'hermes-project-cache'), '--out', str(root / 'hermes-project-payload'),
                    '--commit', '2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19']
            self.assertEqual(acceptance.reviewed_windows_builder(argv), os.name == 'nt')
            for index in range(len(argv)):
                changed = list(argv)
                changed[index] = 'unreviewed'
                self.assertFalse(acceptance.reviewed_windows_builder(changed))
            self.assertFalse(acceptance.reviewed_windows_builder(argv + ['extra']))


class HermesTrustReportCompletionAcceptance(unittest.TestCase):
    def test_completed_guidance_requires_exact_public_state_and_native_roots(self):
        lifecycle = runpy.run_path(str(Path(__file__).parent / "e2e" / "hermes-native-lifecycle.py"))["Lifecycle"]
        mutations = ("", "grouped", "unverified", "unknown-auth", "blocked",
                     "activation-attested", "authentication-attested", "missing-disclosure",
                     "not-ready", "empty-expected",
                     "missing-expected", "missing-selected", "lost-prior", "extra-root",
                     "invalid-native", "native-read-failed")
        for mutation in mutations:
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as directory:
                root = Path(directory).resolve()
                prior, selected = root / "prior-project", root / "selected-project"
                activation = {"activation": "active", "authentication": "not_required",
                              "policy": "allowed", "verification": "installation_verified",
                              "user_actions": ["Hermes now trusts this project's Git repository root"]}
                data = {"readiness": "ready", "result": {"activation": activation}}
                expected, roots = {prior, selected}, [str(prior), str(selected)]
                if mutation == "unverified":
                    activation["verification"] = "package_validated"
                elif mutation == "unknown-auth":
                    activation["authentication"] = "not_checked"
                elif mutation == "blocked":
                    activation["policy"] = "blocked"
                elif mutation in ("activation-attested", "authentication-attested"):
                    activation[mutation.replace("-", "_")] = True
                elif mutation == "missing-disclosure":
                    activation["user_actions"] = []
                elif mutation == "not-ready":
                    del data["readiness"]
                elif mutation == "empty-expected":
                    expected = set()
                elif mutation == "missing-expected":
                    expected = None
                elif mutation == "missing-selected":
                    roots = [str(prior)]
                elif mutation == "lost-prior":
                    roots = [str(selected)]
                elif mutation == "extra-root":
                    roots.append(str(root / "foreign"))
                elif mutation == "invalid-native":
                    roots = {"unknown": True}
                document = {"data": {"targets": [{"output": data}]} if mutation == "grouped" else data}
                output, native = root / "project-add.json", root / "native-roots.json"
                output.write_text(json.dumps(document), encoding="utf-8")
                native.write_text(json.dumps(roots), encoding="utf-8")
                suite = lifecycle.__new__(lifecycle)
                suite.root, suite.hermes = root, ["selected-hermes"]
                # Inert unit output only. The actual fixture still invokes its
                # existing genuine config collector, never an attestation flag.
                with patch.object(suite, "capture", return_value=native) as capture:
                    if mutation == "native-read-failed":
                        capture.side_effect = RuntimeError("native readback failed")
                    if mutation in ("", "grouped"):
                        suite.require_trust_report(output, granted=True, expected_roots=expected)
                    else:
                        with self.assertRaises(RuntimeError):
                            suite.require_trust_report(output, granted=True, expected_roots=expected)
                    if capture.called:
                        self.assertEqual(capture.call_count, 1)
                        self.assertEqual(capture.call_args.args[0],
                                         ["selected-hermes", "config", "get", "--json",
                                          "skills.trusted_project_dirs"])
                    if mutation in ("", "grouped", "missing-selected", "lost-prior",
                                    "extra-root", "invalid-native", "native-read-failed"):
                        self.assertTrue(capture.called)

    def test_existing_granted_and_refused_guidance_keeps_its_boundary(self):
        lifecycle = runpy.run_path(str(Path(__file__).parent / "e2e" / "hermes-native-lifecycle.py"))["Lifecycle"]
        granted = "Hermes now trusts this project's Git repository root"
        refused = "UAP did not change Hermes project trust: maintenance; run `hermes skills trust` inside it"
        for grant, actions, accepted in ((True, [granted], True),
                                         (False, [refused], True),
                                         (False, [], False)):
            with self.subTest(granted=grant, actions=actions), tempfile.TemporaryDirectory() as directory:
                root = Path(directory).resolve()
                output = root / "project-add.json"
                output.write_text(json.dumps({"data": {"readiness": "ready", "result": {"activation": {
                    "activation": "prepared", "authentication": "not_required", "policy": "allowed",
                    "verification": "installation_verified", "user_actions": actions}}}}), encoding="utf-8")
                suite = lifecycle.__new__(lifecycle)
                suite.root, suite.hermes = root, ["selected-hermes"]
                with patch.object(suite, "capture") as capture:
                    if accepted:
                        suite.require_trust_report(output, granted=grant, reason="maintenance")
                    else:
                        with self.assertRaises(RuntimeError):
                            suite.require_trust_report(output, granted=grant, reason="maintenance")
                    capture.assert_not_called()


class HermesSkillCatalogReadAcceptance(unittest.TestCase):
    def test_digest_is_from_the_catalog_read_and_not_a_later_file_version(self):
        source = (Path(__file__).parent.parent / "install/integrationctl/agentplugins/clients/hermes/skill_observation.go").read_text()
        observation = source.split("const hermesSkillObservation = `", 1)[1].split("`", 1)[0]
        original = b"---\nname: owned\ndescription: Owned metadata.\n---\nOriginal.\n"
        substituted = original.replace(b"Original.", b"Substituted.")
        for mode in ("stable", "aba", "cached", "repeated", "oversized", "bom-crlf"):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as directory:
                root = Path(directory).resolve()
                skill = root / "skills" / "owned" / "SKILL.md"
                skill.parent.mkdir(parents=True)
                raw = b"\xef\xbb\xbf" + original.replace(b"\n", b"\r\n") if mode == "bom-crlf" else original
                skill.write_bytes(raw)
                native = types.ModuleType("tools.skills_tool")
                native.__file__ = str(root / "tools" / "skills_tool.py")
                native.get_hermes_home = lambda: root
                reader = lambda path: path.read_text(encoding="utf-8-sig", errors="replace")
                native._read_skill_text = reader

                def catalog():
                    # Inert reader-boundary fixture; this is not native runtime proof.
                    if mode == "aba":
                        skill.write_bytes(substituted)
                    elif mode == "oversized":
                        skill.write_bytes(b"x" * ((1 << 20) + 1))
                    if mode != "cached":
                        native._read_skill_text(skill)
                    if mode == "repeated":
                        native._read_skill_text(skill)
                    skill.write_bytes(raw)
                    return [{"load_name": "owned", "status": "unique", "path": skill, "tier": 1}]

                native._skill_catalog = catalog
                tools_module = types.ModuleType("tools")
                tools_module.skills_tool = native
                utils = types.ModuleType("agent.skill_utils")
                utils.find_project_root = lambda: None
                utils.is_project_root_trusted = lambda path: False
                modules = {"tools": tools_module, "tools.skills_tool": native,
                           "agent": types.ModuleType("agent"), "agent.skill_utils": utils}
                stdout = io.StringIO()
                argv = ["observer", json.dumps({"owned": str(skill)}), str(root)]
                with patch.dict(sys.modules, modules), patch.object(sys, "argv", argv), redirect_stdout(stdout):
                    exec(compile(observation, "hermes-production-observer", "exec"), {})
                rows = json.loads(stdout.getvalue())["skills"]
                self.assertIs(native._read_skill_text, reader)
                self.assertEqual(skill.read_bytes(), raw)
                if mode in ("cached", "repeated", "oversized"):
                    self.assertEqual(rows, [])
                else:
                    observed = substituted if mode == "aba" else original
                    self.assertEqual(rows[0]["digest"], hashlib.sha256(observed).hexdigest())
                    self.assertEqual(rows[0]["digest"] == hashlib.sha256(original).hexdigest(), mode != "aba")


class HermesSkillBootstrapArgvAcceptance(unittest.TestCase):
    def test_actual_bootstrap_preserves_inline_code_arguments_and_child_status(self):
        source = (Path(__file__).parent.parent / "install/integrationctl/agentplugins/clients/hermes/skill_observation.go").read_text()
        bootstrap = source.split("const hermesSkillBootstrap = `", 1)[1].split("`", 1)[0]
        with tempfile.TemporaryDirectory(prefix="hermes argv ") as directory:
            root = Path(directory).resolve() / "source with spaces"
            package = root / "hermes_cli"
            package.mkdir(parents=True)
            (package / "__init__.py").write_text("", encoding="utf-8")
            # Only the installation-bound command factory is inert. Execute
            # the actual production bridge with the host's ordinary argv rules.
            (package / "_launchers.py").write_text(
                "def runtime_command(root, args=(), *, python, code):\n"
                "    return [str(python), '-I', '-c', code, *args]\n", encoding="utf-8")
            selection = json.dumps({"docs": 'C:\\native path\\quote "docs"\\SKILL.md'})
            code = ("import json, sys\n"
                    "print(json.dumps(sys.argv[1:]))\n"
                    "raise SystemExit(int(sys.argv[3]))\n")
            for status in (0, 7):
                with self.subTest(status=status):
                    arguments = [selection, str(root), str(status)]
                    child = subprocess.run(
                        [sys.executable, "-I", "-c", bootstrap, str(root), code, *arguments],
                        capture_output=True, timeout=10, check=False)
                    self.assertEqual(child.returncode, status)
                    self.assertEqual(child.stderr, b"")
                    self.assertEqual(json.loads(child.stdout), arguments)


if __name__ == '__main__':
    unittest.main()
