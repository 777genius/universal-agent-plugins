"""Inert regressions for the builder-only native acceptance boundary."""
import base64
from contextlib import ExitStack, contextmanager, redirect_stdout
import copy
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import time
import tempfile
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


class ProjectHomePreservationAcceptance(unittest.TestCase):
    """Real fixture files and fixed parser children; no Hermes or native process."""

    parser_run = staticmethod(subprocess.run)
    inert_yaml = b"import json\ndef safe_load(value):\n    return json.loads(value)\n"

    @staticmethod
    def load_project():
        path = Path(__file__).parent / 'e2e/hermes-project-install.py'
        spec = importlib.util.spec_from_file_location('hermes_project_install_control', path)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        return module

    @staticmethod
    def initial_config():
        return {'theme': 'retained-user', '_config_version': 52, 'mcp_servers': {},
                'unrelated': {'enabled': True, 'nested': [1, 'kept'],
                              'note': 'theme: retained-user'}}

    @staticmethod
    def write_config(path, document):
        temporary = path.with_name('config.fixture-temporary')
        temporary.write_text(json.dumps(document, sort_keys=True) + '\n', encoding='utf-8')
        temporary.chmod(0o600)
        os.replace(temporary, path)

    def payload(self, root):
        payload = root / 'payload'
        for relative in ('bin', 'hermes-agent', 'packages', 'tools'):
            (payload / relative).mkdir(parents=True)
        launcher = payload / 'bin' / ('hermes.exe' if os.name == 'nt' else 'hermes')
        launcher.write_bytes(b'inert launcher; never executed\n')
        python = payload / 'tools/python'
        python.write_bytes(b'inert declared Python; substituted only in this control\n')
        (payload / 'hermes-agent/hermes_yaml.py').write_bytes(self.inert_yaml)
        manifest = json.dumps({'runtime': {'repoDir': 'hermes-agent', 'sitePackages': 'packages',
                                          'storePython': 'tools/python'}}).encode()
        (payload / 'manifest.json').write_bytes(manifest)
        payload.with_name('payload.prepared.json').write_text('{}\n')
        return payload, launcher, {'manifest_sha256': hashlib.sha256(manifest).hexdigest()}

    def parser_patch(self, stack, module, payload):
        digest = hashlib.sha1(b'blob ' + str(len(self.inert_yaml)).encode() + b'\0' + self.inert_yaml).hexdigest()
        stack.enter_context(patch.object(module, 'YAML_POLICY_GIT_BLOB', digest, create=True))

        def run(argv, **options):
            self.assertEqual(argv[:5], [str(payload / 'tools/python'), '-I', '-S', '-B', '-c'])
            self.assertEqual(argv[5], module.CONFIG_PARSER)
            self.assertEqual(argv[6:], [str(payload / 'hermes-agent'), str(payload / 'packages')])
            self.assertEqual(options['cwd'], payload)
            self.assertGreater(options['timeout'], 0)
            self.assertLessEqual(options['timeout'], 5)
            self.assertLessEqual(len(options['input']), 1 << 20)
            return self.parser_run([sys.executable, *argv[1:]], **options)

        stack.enter_context(patch.object(subprocess, 'run', side_effect=run))

    @contextmanager
    def fixture(self):
        module = self.load_project()
        with tempfile.TemporaryDirectory() as directory, ExitStack() as stack:
            root = Path(directory).resolve()
            home, evidence = root / 'home', root / 'evidence'
            profile = home / 'selected'
            profile.mkdir(parents=True, mode=0o700)
            evidence.mkdir(mode=0o700)
            config = profile / 'config.yaml'
            self.write_config(config, self.initial_config())
            (profile / 'auth-sentinel').write_bytes(b'original inert auth sentinel\n')
            prior = profile / 'backups/config'
            prior.mkdir(parents=True)
            (prior / 'user-owned-backup').write_bytes(b'prior User bytes; retain intact\n')
            payload, _, provenance = self.payload(root)
            self.parser_patch(stack, module, payload)
            tick = [time.time()]
            stack.enter_context(patch.object(time, 'time', side_effect=lambda: tick[0]))
            guard = module.ProjectHomePreservation(home, profile, evidence, payload, provenance,
                                                   dict(os.environ), time.monotonic() + 20, stack)
            project = root / 'project'
            project.mkdir()
            original = copy.deepcopy(guard.original)
            yield module, guard, project, tick
            self.assertEqual(guard.original, original, 'the original HOME map must never be rebaselined')
            descriptors = [descriptor for _, descriptor, _ in guard.directories]
        for descriptor in descriptors:
            with self.assertRaises(OSError):
                os.fstat(descriptor)

    def trust(self, guard, project):
        document = json.loads(guard.before_body)
        document.setdefault('skills', {})['trusted_project_dirs'] = [str(project)]
        self.write_config(guard.config, document)

    @staticmethod
    def backup(guard, tick):
        stamp = time.strftime('%Y%m%d-%H%M%S', time.localtime(tick[0]))
        path = guard.backups / ('config.yaml.good.' + stamp)
        # Match the pinned writer's occupied-second and newest-identical rules.
        existing = sorted(guard.backups.glob('config.yaml.good.*'), reverse=True)
        if (existing and existing[0].read_bytes() == guard.config.read_bytes()) or path.exists():
            return None
        shutil.copy2(guard.config, path)
        return path

    def actual_main(self, attack):
        module = self.load_project()
        with tempfile.TemporaryDirectory() as directory, ExitStack() as stack:
            root = Path(directory).resolve()
            payload, launcher, provenance = self.payload(root)
            uap = root / 'uap-inert'
            uap.write_bytes(b'inert CLI; never executed\n')
            stack.enter_context(patch.dict(os.environ, {'RUNNER_TEMP': str(root),
                'HERMES_PROJECT_BUNDLE': str(launcher), 'UAP_BIN': str(uap),
                'EXPECTED_HEAD': 'a' * 40, 'EXPECTED_TREE': 'b' * 40}))
            self.parser_patch(stack, module, payload)
            stack.enter_context(patch.object(module, 'payload_contract', return_value=provenance))
            events = []
            inventory = {'inventory-only': {'url': 'http://127.0.0.1:1/mcp'}}

            def transport(argv, cwd, env, timeout):
                argv = list(map(str, argv))
                events.append(argv)
                profile = Path(env['HERMES_HOME'])
                project = Path(env['HOME']).parent / 'project-a'
                config = profile / 'config.yaml'
                output = b'{}'
                if argv[0] == 'git':
                    Path(argv[-1], '.git').mkdir()
                elif argv[0] == str(launcher):
                    if argv[1:] == ['--version']:
                        output = b'Hermes Agent vunknown (2026.9.24)\n'
                    elif argv[1:4] == ['config', 'get', 'mcp_servers']:
                        output = json.dumps(inventory).encode()
                    elif argv[1:4] == ['config', 'get', 'skills.trusted_project_dirs']:
                        output = json.dumps([str(project)]).encode()
                    else:
                        self.fail('unexpected inert Hermes command')
                else:
                    self.assertEqual(argv[0], str(uap))
                    command = argv[1]
                    user = argv[argv.index('--scope') + 1] == 'user'
                    installed = (profile if user else project / '.hermes') / 'skills/hermes-project-install/SKILL.md'
                    if command in ('add', 'update', 'repair'):
                        installed.parent.mkdir(parents=True, exist_ok=True)
                        installed.write_bytes(b'owned inert Skill\n')
                    elif command == 'remove':
                        installed.unlink()
                        installed.parent.rmdir()
                    elif command == 'run':
                        output = json.dumps(inventory).encode()
                    else:
                        self.fail('unexpected inert CLI command')
                    if user and command == 'remove':
                        self.write_config(config, self.initial_config())
                        prior = profile / 'backups/config'
                        prior.mkdir(parents=True)
                        (prior / 'prior-user').write_bytes(b'preserved User backup\n')
                    if not user and command == 'add' and not (project / '.hermes/uap-session/.uap-owner').exists():
                        backup = profile / 'backups/config' / ('config.yaml.good.' + time.strftime('%Y%m%d-%H%M%S'))
                        shutil.copy2(config, backup)
                        document = json.loads(config.read_text())
                        document['skills'] = {'trusted_project_dirs': [str(project)]}
                        self.write_config(config, document)
                        marker = project / '.hermes/uap-session/.uap-owner'
                        marker.parent.mkdir(parents=True)
                        marker.write_bytes(b'owned session\n')
                        sentinel = profile / 'auth-sentinel'
                        if attack == 'body':
                            sentinel.write_bytes(b'unexpected inert auth mutation\n')
                        elif attack == 'replacement':
                            replacement = profile / 'auth-replacement'
                            shutil.copy2(sentinel, replacement)
                            os.replace(replacement, sentinel)
                return {'exit_code': 0, 'cleanup_complete': True, 'no_child_started': False,
                        'error': None, 'argv': argv}, output, b''

            stack.enter_context(patch.object(module, 'supervised', side_effect=transport))
            try:
                with redirect_stdout(io.StringIO()):
                    module.main()
            except RuntimeError as error:
                self.assertIn('HOME', str(error))
                return False, events
            return True, events

    def test_actual_main_retains_auth_sentinel_through_first_trust_write(self):
        accepted, events = self.actual_main(None)
        self.assertTrue(accepted, 'the exact declared trust effect must still complete')
        self.assertTrue(any(argv[1:2] == ['run'] for argv in events))
        self.assertTrue(any(argv[1:2] == ['remove'] and 'project' in argv for argv in events))
        for attack in ('body', 'replacement'):
            with self.subTest(attack=attack):
                accepted, events = self.actual_main(attack)
                self.assertTrue(any('skills.trusted_project_dirs' in argv for argv in events))
                self.assertFalse(accepted, 'first add must not rebaseline an altered original auth sentinel')
                self.assertFalse(any(argv[1:2] == ['update'] and 'project' in argv for argv in events))

    def test_captured_config_parser_preserves_all_unrelated_typed_values(self):
        variants = ('nested', 'bool-to-int', 'version', 'extra-trust', 'unknown-key', 'nested-type')
        for variant in variants:
            with self.subTest(variant=variant), self.fixture() as (module, guard, project, tick):
                document = json.loads(guard.before_body)
                document['skills'] = {'trusted_project_dirs': [str(project)]}
                if variant == 'nested':
                    document['unrelated']['nested'][1] = 'changed'
                elif variant == 'bool-to-int':
                    document['unrelated']['enabled'] = 1
                elif variant == 'version':
                    document['_config_version'] = 53
                elif variant == 'extra-trust':
                    document['skills']['trusted_project_dirs'].append(str(project.parent))
                elif variant == 'unknown-key':
                    document['new'] = 'unexpected'
                else:
                    document['unrelated']['nested'] = {'1': 'new mapping'}
                self.write_config(guard.config, document)
                with self.assertRaisesRegex(RuntimeError, 'beyond trust and schema 52'):
                    guard.authorize_trust(project)
                self.assertIsNone(guard.final)
        with self.fixture() as (module, guard, project, tick):
            self.trust(guard, project)
            (guard.payload / 'hermes-agent/hermes_yaml.py').write_bytes(self.inert_yaml + b'# replacement\n')
            with self.assertRaisesRegex(RuntimeError, 'pinned YAML policy changed'):
                guard.authorize_trust(project)
            self.assertIsNone(guard.final)

    def test_only_exact_source_consistent_backups_are_projected(self):
        with self.fixture() as (module, guard, project, tick):
            first = self.backup(guard, tick)
            self.assertIsNotNone(first)
            self.trust(guard, project)
            self.assertIsNone(self.backup(guard, tick), 'an occupied second cannot be overwritten')
            guard.authorize_trust(project)
            tick[0] += 1
            second = self.backup(guard, tick)
            self.assertIsNotNone(second)
            self.assertEqual(guard.checked(module.snapshot(guard.home)), guard.original)
            tick[0] += 1
            self.assertIsNone(self.backup(guard, tick), 'an unchanged newest copy must be skipped')
            self.assertEqual(guard.checked(module.snapshot(guard.home)), guard.original)
            self.assertEqual((guard.archive / 'user-owned-backup').read_bytes(), b'prior User bytes; retain intact\n')
        variants = ['body', 'mtime', 'mode', 'name', 'unicode-name', 'stale-name', 'future-name',
                    'hardlink', 'duplicate', 'third', 'deleted', 'replaced', 'foreign', 'archive']
        if os.name != 'nt':
            variants.append('symlink')
        for variant in variants:
            with self.subTest(variant=variant), self.fixture() as (module, guard, project, tick):
                first = self.backup(guard, tick)
                self.trust(guard, project)
                guard.authorize_trust(project)
                tick[0] += 1
                second = self.backup(guard, tick)
                if variant == 'body':
                    second.write_bytes(b'unknown backup content')
                elif variant == 'mtime':
                    info = second.stat()
                    os.utime(second, ns=(info.st_atime_ns, info.st_mtime_ns + 1_000_000_000))
                elif variant == 'mode':
                    second.chmod(0o400)
                elif variant == 'name':
                    second.rename(second.with_name('config.yaml.good.foreign'))
                elif variant == 'unicode-name':
                    second.rename(second.with_name(second.name.replace('2', '\u0662', 1)))
                elif variant in ('stale-name', 'future-name'):
                    offset = -60 if variant == 'stale-name' else 60
                    second.rename(second.with_name('config.yaml.good.' + time.strftime(
                        '%Y%m%d-%H%M%S', time.localtime(tick[0] + offset))))
                elif variant == 'symlink':
                    second.unlink()
                    second.symlink_to(guard.config)
                elif variant == 'hardlink':
                    os.link(second, guard.evidence / 'foreign-alias')
                elif variant == 'duplicate':
                    shutil.copy2(first, second)
                elif variant == 'third':
                    shutil.copy2(first, second.with_name('config.yaml.good.' + time.strftime('%Y%m%d-%H%M%S', time.localtime(tick[0] + 1))))
                    tick[0] += 1
                elif variant == 'deleted':
                    first.unlink()
                elif variant == 'replaced':
                    replacement = guard.evidence / 'replacement'
                    shutil.copy2(first, replacement)
                    os.replace(replacement, first)
                elif variant == 'foreign':
                    (guard.backups / 'foreign-sentinel').write_bytes(b'changed foreign member')
                else:
                    (guard.archive / 'user-owned-backup').write_bytes(b'changed archived member')
                with self.assertRaises((RuntimeError, OSError)):
                    guard.checked(module.snapshot(guard.home))
                if variant == 'mode':
                    second.chmod(0o600)

    def test_original_directory_and_unselected_members_remain_authoritative(self):
        for variant in ('new-member', 'config-replaced', 'backup-dir-replaced', 'profile-replaced', 'home-replaced'):
            with self.subTest(variant=variant), self.fixture() as (module, guard, project, tick):
                self.trust(guard, project)
                guard.authorize_trust(project)
                if variant == 'new-member':
                    (guard.home / 'foreign-new').write_bytes(b'not declared')
                elif variant == 'config-replaced':
                    replacement = guard.evidence / 'config-replacement'
                    shutil.copy2(guard.config, replacement)
                    os.replace(replacement, guard.config)
                else:
                    path = {'backup-dir-replaced': guard.backups,
                            'profile-replaced': guard.profile, 'home-replaced': guard.home}[variant]
                    original = guard.evidence / 'moved-original'
                    path.rename(original)
                    shutil.copytree(original, path)
                with self.assertRaises((RuntimeError, OSError)):
                    guard.checked(module.snapshot(guard.home))


if __name__ == '__main__':
    unittest.main()
