"""Inert tests execute actual parser, capture fence and artifacts; no vendor."""
import base64
import copy
from contextlib import contextmanager
import json
import os
from pathlib import Path
import selectors
import subprocess
import sys
import tempfile
import time
import unittest
from unittest import mock
import zipfile
import grok_acp_inventory_admission as case
import grok_acp_bootstrap as bootstrap_case

SQLITE_COMPANIONS = ('sessions/session_search.sqlite-wal', 'sessions/session_search.sqlite-shm')


def help_rows():
    bodies = (b'grok 1.0.70 (f50904eaee6c)\n', b'Commands:\n  agent Agent\n',
              b'Commands:\n  stdio Agent\n --no-leader\n', b'Usage: grok agent stdio')
    return [dict(label=label, argv=['/sealed/grok'], exit=0, error=None, merged_base64=base64.b64encode(body).decode(),
                 merged_bytes=len(body), merged_sha256=case.digest(body)) for label, body in zip(case.LABELS, bodies)]


def plugin_row(root, scope, name):
    path = str(root / 'home/.grok/plugins' / name if scope == 'user' else root / scope / '.grok/plugins' / name)
    native_scope = 'user' if scope == 'user' else 'project'
    row = dict(id=native_scope + '/' + case.digest(path.encode())[:8] + '/' + name,
               root=path, name=name, scope=native_scope, trusted=scope == 'user', enabled=False)
    if scope != 'user' and name == 'uap-acp-collision':
        row['conflict'] = 'Name collision: shadowing "uap-acp-collision" from ' + str(root / 'home/.grok/plugins/uap-acp-collision')
    return row



class AdmissionContracts(unittest.TestCase):
    def request(self, root):
        return dict(schema_version=1, source_commit='a' * 40, source_tree='b' * 40, uid=1001, gid=1001,
                    nonce='c' * 32, scope='user', input_root=str(root), parent_network_namespace=1, start_monotonic_ns=time.monotonic_ns())

    def test_entrypoint_calls_required_target_host_guard_before_any_output(self):
        import project_native_research as shared

        class GuardStop(Exception):
            pass

        calls = []
        def guard(target):  # Required argument matches the actual shared authority.
            calls.append(target)
            raise GuardStop('actual-host-guard-boundary')

        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / 'not-created'
            with (mock.patch.object(shared.native, 'require_hosted', guard),
                  mock.patch.object(shared, 'acquire', side_effect=AssertionError('acquisition preceded host guard')),
                  mock.patch.object(Path, 'mkdir', side_effect=AssertionError('output preceded host guard')),
                  self.assertRaisesRegex(GuardStop, 'actual-host-guard-boundary')):
                case.run(output)
            self.assertEqual(calls, ['linux-amd64'])
            self.assertFalse(output.exists())

    def test_json_bounds_unique_finite_and_closed_fields(self):
        for body in (b'{"x":1,"x":2}', b'{"x":1e999}', b'{"x":NaN}', b'{}{}', b'[' * 18 + b'0' + b']' * 18):
            with self.subTest(body=body[:30]), self.assertRaises(ValueError): case.document(body)
        self.assertEqual(case.document(b'{"finite":1e2}'), {'finite': 100.0})
        with self.assertRaises(ValueError): case.closed({'extra': 1}, 'only')

    def test_two_frame_exchange_with_exact_empty_notification(self):
        state, sent = case.rpc_state(), []
        req = {'nonce': 'a' * 32, 'scope': 'a'}
        notification = {'jsonrpc': '2.0', 'method': '_x.ai/mcp/servers_updated', 'params': {'mcpServers': []}}
        self.assertFalse(case.rpc_frame(notification, state, req, sent.append))
        self.assertFalse(case.rpc_frame({'jsonrpc': '2.0', 'id': 1, 'result': {'protocolVersion': 1, '_meta': {'agentVersion': '1.0.70'}}}, state, req, sent.append))
        self.assertEqual(sent, [{'jsonrpc': '2.0', 'id': 2, 'method': '_x.ai/plugins/list', 'params': {'sessionId': 'uap-admission-' + req['nonce'] + '-a'}}])
        self.assertTrue(case.rpc_frame({'jsonrpc': '2.0', 'id': 2, 'result': {'result': {'plugins': []}}}, state, req, sent.append))
        with self.assertRaises(ValueError): case.rpc_frame(notification, state, req, sent.append)

    def test_listing_requires_measured_double_result_envelope(self):
        req = {'nonce': 'a' * 32, 'scope': 'user'}
        initialize = {'jsonrpc': '2.0', 'id': 1, 'result': {'protocolVersion': 1, '_meta': {'agentVersion': '1.0.70'}}}
        for result in ({'plugins': []}, {'result': {'result': {'plugins': []}}},
                       {'result': {'plugins': []}, 'extra': None},
                       {'result': {'plugins': [], 'extra': None}},
                       {'result': {'plugins': None}}, {'result': {'plugins': [{}] * 129}}):
            with self.subTest(result=result):
                state, sent = case.rpc_state(), []
                case.rpc_frame(initialize, state, req, sent.append)
                with self.assertRaises(ValueError):
                    case.rpc_frame({'jsonrpc': '2.0', 'id': 2, 'result': result}, state, req, sent.append)
                self.assertIsNone(state['plugins'])
                self.assertEqual(len(sent), 1)

    def test_requests_unknown_notifications_ids_and_versions_refuse(self):
        for value in ({'jsonrpc': '2.0', 'id': 1, 'method': 'fs/read_text_file', 'params': {}},
                      {'jsonrpc': '2.0', 'method': '_unknown', 'params': {}},
                      {'jsonrpc': '2.0', 'method': '_x.ai/mcp/servers_updated', 'params': {'mcpServers': ['foreign']}},
                      {'jsonrpc': '2.0', 'id': True, 'result': {}},
                      {'jsonrpc': '2.0', 'id': 1, 'result': {'protocolVersion': '1', '_meta': {'agentVersion': '1.0.70'}}},
                      {'jsonrpc': '2.0', 'id': 1, 'result': {'protocolVersion': 1, '_meta': {'agentVersion': '1.0.71'}}}):
            with self.subTest(value=value), self.assertRaises(ValueError):
                case.rpc_frame(value, case.rpc_state(), {'nonce': 'a' * 32, 'scope': 'a'}, lambda _: self.fail('unexpected request'))

    def test_actual_pipe_preserves_nested_listing_for_exact_inventory(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            rows = [plugin_row(root, 'user', name)
                    for name in ('uap-acp-collision', 'uap-acp-foreign-user')]
            frames = [{'jsonrpc': '2.0', 'id': 1, 'result': {'protocolVersion': 1, '_meta': {'agentVersion': '1.0.70'}}},
                      {'jsonrpc': '2.0', 'id': 2, 'result': {'result': {'plugins': rows}}}]
            child = """import json, sys
frames = json.loads(sys.argv[1])
for method, frame in zip(('initialize', '_x.ai/plugins/list'), frames):
    assert json.loads(sys.stdin.readline())['method'] == method
    print(json.dumps(frame), flush=True)
sys.stdin.read()
"""
            state = case.rpc_state()
            record = case.command([sys.executable, '-I', '-c', child, json.dumps(frames)], 'acp', root,
                                  {'PATH': os.defpath}, time.monotonic() + 3, case.LIMIT, state, self.request(root))
            self.assertIsNone(record['error'])
            self.assertEqual(state['plugins'], frames[1])
            captured = base64.b64decode(record['merged_base64'], validate=True)
            self.assertEqual([json.loads(line) for line in captured.splitlines()], frames)
            self.assertEqual(len(captured), record['merged_bytes'])
            self.assertEqual(case.digest(captured), record['merged_sha256'])
            self.assertEqual(case.inventory_rows({'rpc': state}, root, 'user'),
                             {row['root']: row['id'] for row in rows})

    def test_help_requires_exact_native_pin_and_advertised_syntax(self):
        rows = help_rows()
        case.help_admitted(rows)
        for index, body in ((0, b'grok 1.0.71\n'), (2, b'Commands:\n  stdio Agent\n')):
            changed = copy.deepcopy(rows)
            changed[index]['merged_base64'] = base64.b64encode(body).decode()
            with self.assertRaises(ValueError): case.help_admitted(changed)

    def test_full_id_requires_exact_roots_scope_and_no_sibling(self):
        root = Path('/tmp/uap-fixture')
        rows = [plugin_row(root, 'user', name)
                for name in ('uap-acp-collision', 'uap-acp-foreign-user')]
        result = {'rpc': {'plugins': {'result': {'result': {'plugins': rows}}}}}
        case.inventory_rows(result, root, 'user')
        for mutate in ('missing', 'duplicate', 'scope', 'sibling', 'name-only', 'wrong-hash', 'outside'):
            value = copy.deepcopy(result)
            current = value['rpc']['plugins']['result']['result']['plugins']
            if mutate == 'missing': current.pop()
            elif mutate == 'duplicate': current.append(current[0])
            elif mutate == 'scope': current[0]['scope'] = 'project'
            elif mutate == 'name-only': current[0]['id'] = current[0]['name']
            elif mutate == 'wrong-hash': current[0]['id'] = 'user/00000000/' + current[0]['name']
            elif mutate == 'outside': current[0]['root'] = '/outside/plugin'
            else: current[0]['root'] = str(root / 'b/.grok/plugins/foreign')
            with self.assertRaises(ValueError): case.inventory_rows(value, root, 'user')

    def test_project_precedence_has_exact_winner_and_preserved_unique_sources(self):
        root = Path('/tmp/uap-fixture')
        for scope in ('a', 'b'):
            rows = [plugin_row(root, 'user', 'uap-acp-foreign-user'),
                    plugin_row(root, scope, 'uap-acp-collision'),
                    plugin_row(root, scope, 'uap-acp-foreign-' + scope)]
            result = {'rpc': {'plugins': {'result': {'result': {'plugins': rows}}}}}
            self.assertEqual(case.inventory_rows(result, root, scope), {row['root']: row['id'] for row in rows})
            for mutation in ('wrong-winner', 'duplicate', 'sibling', 'lost-unique', 'both-winners', 'wrong-warning'):
                value = copy.deepcopy(result)
                current = value['rpc']['plugins']['result']['result']['plugins']
                if mutation == 'wrong-winner': current[1] = plugin_row(root, 'user', 'uap-acp-collision')
                elif mutation == 'duplicate': current[2] = copy.deepcopy(current[1])
                elif mutation == 'sibling': current[1] = plugin_row(root, 'b' if scope == 'a' else 'a', 'uap-acp-collision')
                elif mutation == 'lost-unique': current.pop(0)
                elif mutation == 'both-winners': current.append(plugin_row(root, 'user', 'uap-acp-collision'))
                else: current[1]['conflict'] = 'different source'
                with self.subTest(scope=scope, mutation=mutation), self.assertRaises(ValueError):
                    case.inventory_rows(value, root, scope)


    def test_bounded_reader_rejects_same_bytes_replacement_and_alias(self):
        with tempfile.TemporaryDirectory() as directory:
            root, real_open = Path(directory), os.open
            target = root / 'ordinary'
            target.write_bytes(b'held')
            self.assertEqual(case.ordinary(target), b'held')
            (root / 'alias').symlink_to(target)
            with self.assertRaises(ValueError): case.ordinary(root / 'alias')
            def replaced(path, flags):
                target.rename(root / 'old')
                target.write_bytes(b'held')
                return real_open(path, flags)
            with mock.patch.object(case.os, 'open', replaced), self.assertRaises(ValueError): case.ordinary(target)

    def test_actual_pipe_drain_preserves_late_bytes_and_refuses_flood_or_no_eof(self):
        for mode in ('late', 'open', 'flood'):
            read_fd, write_fd = os.pipe()
            os.set_blocking(read_fd, False)
            with os.fdopen(read_fd, 'rb') as pipe:
                try:
                    os.write(write_fd, b'late-bytes')
                    if mode != 'open': os.close(write_fd); write_fd = None
                    held = bytearray()
                    if mode == 'late':
                        case.drain_stopped(pipe, held, time.monotonic() + .1, 100)
                        self.assertEqual(held, b'late-bytes')
                    else:
                        with self.assertRaises(ValueError): case.drain_stopped(pipe, held, time.monotonic() + .01, 2 if mode == 'flood' else 100)
                finally:
                    if write_fd is not None: os.close(write_fd)

    def test_incomplete_each_command_never_emits_collector_json(self):
        for failed_index in range(5):
            with self.subTest(command=failed_index), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                request = root / 'request.json'
                request.write_text(json.dumps(self.request(root)))
                calls = []
                def incomplete(*args):
                    index = len(calls)
                    calls.append(args)
                    if index == failed_index: raise OSError('inert incomplete pipe/wait')
                    return help_rows()[index]
                output = mock.Mock()
                with mock.patch.object(case, 'isolation', return_value={}), mock.patch.object(case, 'command', incomplete), mock.patch.object(case.sys, 'stdout', output):
                    with self.assertRaises(OSError): case.collect(request)
                output.buffer.write.assert_not_called()
                self.assertEqual(len(calls), failed_index + 1)

    def test_actual_collector_closes_output_descriptors_before_parent_shutdown(self):
        child = """import pathlib, runpy, sys
case = runpy.run_path(sys.argv[1], run_name='inert_collector_eof')
case['collect'].__globals__['collector_result'] = lambda _: {'fixture': True}
case['collect'](pathlib.Path(sys.argv[2]))
"""
        with tempfile.TemporaryDirectory() as directory:
            request = Path(directory) / 'request.json'
            request.write_text(json.dumps(self.request(Path(directory))))
            process = subprocess.Popen([sys.executable, '-I', '-c', child, str(Path(case.__file__).resolve()), str(request)],
                                       stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            streams = {'stdout': bytearray(), 'stderr': bytearray()}
            try:
                deadline = time.monotonic() + 3
                with selectors.DefaultSelector() as selector:
                    for name, pipe in (('stdout', process.stdout), ('stderr', process.stderr)):
                        selector.register(pipe, selectors.EVENT_READ, name)
                    while selector.get_map() and time.monotonic() < deadline:
                        for key, _ in selector.select(min(.05, max(0, deadline - time.monotonic()))):
                            body = os.read(key.fd, 1025)
                            streams[key.data].extend(body)
                            self.assertLessEqual(len(streams[key.data]), 1024)
                            if not body: selector.unregister(key.fileobj)
                    self.assertFalse(selector.get_map(), 'collector kept an output descriptor open')
                self.assertEqual(streams, {'stdout': b'{"fixture":true}\n', 'stderr': b''})
                self.assertIsNone(process.poll(), 'collector exited before its strict parent shutdown')
            finally:
                process.kill()
                process.wait(timeout=3)
                for pipe in (process.stdin, process.stdout, process.stderr): pipe.close()

    def test_actual_drain_failure_bypasses_semantic_negative_handler(self):
        read_fd, write_fd = os.pipe()
        os.set_blocking(read_fd, False)
        try:
            with os.fdopen(read_fd, 'rb') as pipe, tempfile.TemporaryDirectory() as directory:
                req = self.request(Path(directory))
                def incomplete(*_): case.drain_stopped(pipe, bytearray(), time.monotonic() + .005, 10)
                with mock.patch.object(case, 'isolation', return_value={}), mock.patch.object(case, 'command', incomplete), self.assertRaisesRegex(ValueError, 'drain-deadline'):
                    case.collector_result(req)
        finally: os.close(write_fd)

    def test_fully_drained_negative_is_retained_without_extra_native_command(self):
        row = dict(help_rows()[0], error='process-refused')
        with mock.patch.object(case, 'isolation', return_value={}), mock.patch.object(case, 'command', return_value=row) as command:
            value = case.collector_result(self.request(Path('/tmp/fixture')))
        self.assertFalse(value['semantic_complete'])
        self.assertEqual(value['reason'], 'help-refused')
        self.assertEqual(value['commands'], [row])
        command.assert_called_once()

    def test_unknown_write_and_trust_store_never_renew_original_baseline(self):
        with native_bootstrap_fixture() as fixture:
            root, capture, ledger = fixture['root'], fixture['capture'], fixture['bootstrap']
            self.assertTrue(capture()['accepted'])
            original = copy.deepcopy(ledger.created)
            trust = root / 'home/.grok/trusted_folders.toml'
            trust.write_text('unadmitted')
            self.assertFalse(capture()['accepted'])
            trust.unlink()
            (root / 'unknown').mkdir()
            self.assertFalse(capture()['accepted'])
            self.assertEqual(ledger.created, original)

    def bootstrap_ledger(self, root, budget, payloads=None):
        from devin_structured_inventory_admission import metadata_inventory
        from client_automation_native_probe import snapshot
        with mock.patch.object(case.subprocess, 'run'): roots = case.fixtures(root, 'a' * 32)
        baseline = metadata_inventory(root, budget)
        profiles = {name: snapshot(path) for name, path in roots.items()}
        for name, body in (payloads or {'config.toml': b'unreviewed native config\x00'}).items():
            target = root / 'home/.grok' / name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(body)
        # Preserve the original negative capture schema for historical-reader
        # contracts; new live capture additionally requires full admission.
        return dict(accepted=False, before=baseline, after=metadata_inventory(root, budget),
                    profiles_before=profiles, profiles_after={name: snapshot(path) for name, path in roots.items()},
                    native_bootstrap=[])

    def test_bootstrap_capture_cannot_read_after_new_metadata_refusal(self):
        with native_bootstrap_fixture() as fixture:
            (fixture['root'] / 'home/.grok/trusted_folders.toml').write_text('unadmitted')
            ledger = fixture['capture']()
            self.assertFalse(ledger['accepted'])
            with mock.patch.object(case, 'ordinary') as read, self.assertRaisesRegex(ValueError, 'bootstrap-profile-snapshot'):
                case.capture_bootstrap_files(fixture['root'], ledger, fixture['budget'])
            read.assert_not_called()

    def test_bootstrap_capture_still_joins_admitted_originals(self):
        with native_bootstrap_fixture() as fixture:
            ledger = fixture['capture']()
            self.assertTrue(ledger['accepted'])
            capture = case.capture_bootstrap_files(fixture['root'], ledger, fixture['budget'])
            self.assertEqual(set(capture['files']), set(case.BOOTSTRAP_FILES))
            case.validate_bootstrap_capture(ledger, capture)

    def test_bootstrap_capture_joins_bytes_and_stays_unqualified(self):
        from project_native_research import Budget
        with tempfile.TemporaryDirectory() as directory, Budget(10) as budget:
            root = Path(directory).resolve()
            ledger = self.bootstrap_ledger(root, budget)
            original = copy.deepcopy(ledger)
            capture = case.capture_bootstrap_files(root, ledger, budget)
            selected = 'home/.grok/config.toml'
            self.assertEqual(base64.b64decode(capture['files'][selected]['body_base64']), b'unreviewed native config\x00')
            self.assertEqual(sum(row is None for row in capture['files'].values()), 9)
            self.assertEqual(ledger, original)
            self.assertFalse(ledger['accepted'])
            for change in ('extra', 'hash', 'bytes', 'identity', 'null', 'missing', 'boolean', 'base64'):
                altered = copy.deepcopy(capture)
                row = altered['files'][selected]
                if change == 'extra': altered['files']['home/.grok/trusted_folders.toml'] = None
                elif change == 'hash': row['sha256'] = '0' * 64
                elif change == 'bytes': row['bytes'] += 1
                elif change == 'identity': row['identity'][1] += 1
                elif change == 'null': altered['files'][selected] = None
                elif change == 'missing': del altered['files'][selected]
                elif change == 'boolean': row['identity'][3] = True
                else: row['body_base64'] = '!'
                with self.subTest(change=change), self.assertRaises(ValueError):
                    case.validate_bootstrap_capture(ledger, altered)
            packet, members = self.readout_fixture()
            ledger['bootstrap_capture'] = capture
            members['user.ledger.json'] = json.dumps(ledger).encode()
            readout = case.publication_readout(self.readout_zip(members), packet, time.monotonic() + 5)
            self.assertFalse(readout['collection_complete'])
            self.assertFalse(readout['scopes'][0]['ledger_accepted'])
            self.assertTrue(all(value is False for value in json.loads(members['admission.json'])['qualification'].values()))
            capture['files'][selected]['sha256'] = '0' * 64
            members['user.ledger.json'] = json.dumps(ledger).encode()
            self.assertEqual(case.publication_readout(self.readout_zip(members), packet, time.monotonic() + 5)['reason'], 'readout-refused')

    def test_bootstrap_capture_refuses_replacement_and_final_metadata_changes(self):
        from project_native_research import Budget
        for replacement in (True, False):
            with tempfile.TemporaryDirectory() as directory, Budget(10) as budget:
                root = Path(directory).resolve()
                ledger = self.bootstrap_ledger(root, budget)
                original_read = case.ordinary
                def changed(path, limit, **kwargs):
                    body = original_read(path, limit, **kwargs)
                    new = root / 'unadmitted-new-file'
                    new.write_bytes(body)
                    if replacement: new.replace(path)
                    return body
                with mock.patch.object(case, 'ordinary', changed), self.assertRaises(ValueError):
                    case.capture_bootstrap_files(root, ledger, budget)
                self.assertFalse(ledger['accepted'])
                self.assertNotIn('bootstrap_capture', ledger)

    def test_bootstrap_capture_refuses_gap_replacement_before_open(self):
        from project_native_research import Budget
        with tempfile.TemporaryDirectory() as directory, Budget(10) as budget:
            root = Path(directory).resolve()
            ledger = self.bootstrap_ledger(root, budget)
            original_read = case.ordinary
            def replaced(path, limit, **kwargs):
                replacement = root / 'replacement'
                replacement.write_bytes(b'foreign candidate bytes')
                replacement.replace(path)
                return original_read(path, limit, **kwargs)
            with mock.patch.object(case, 'ordinary', replaced), mock.patch.object(case.os, 'open', wraps=case.os.open) as opened:
                with self.assertRaisesRegex(ValueError, 'original-identity'):
                    case.capture_bootstrap_files(root, ledger, budget)
                opened.assert_not_called()

    def test_bootstrap_capture_keeps_exact_aggregate_and_leaf_bounds(self):
        from project_native_research import Budget
        for mode in ('aggregate', 'file', 'hardlink', 'symlink'):
            with tempfile.TemporaryDirectory() as directory, Budget(10) as budget:
                root = Path(directory).resolve()
                bodies = {'README.md': b'x' * (case.BOOTSTRAP_LIMIT // 2), 'config.toml': b'y' * (case.BOOTSTRAP_LIMIT // 2 + 1)}
                if mode != 'aggregate': bodies = {'config.toml': b'x' * (case.BOOTSTRAP_LIMIT + 1 if mode == 'file' else 1)}
                ledger = self.bootstrap_ledger(root, budget, bodies)
                target = root / 'home/.grok/config.toml'
                if mode == 'hardlink': os.link(target, root / 'alias')
                if mode == 'symlink': target.rename(root / 'alias'); target.symlink_to(root / 'alias')
                with self.subTest(mode=mode), self.assertRaises(ValueError):
                    case.capture_bootstrap_files(root, ledger, budget)

    def test_root_receipt_requires_both_cleanup_proofs_and_held_system_hashes(self):
        req = {key: value for key, value in self.request(Path('/tmp/fixture')).items() if key in ('schema_version', 'source_commit', 'source_tree', 'nonce', 'scope')}
        result = dict(req, semantic_complete=False, reason='help-refused', isolation={}, commands=[], rpc=case.rpc_state())
        receipt = dict(req, cleanup_complete=True, sealed_cleanup_complete=True, result=result,
                       system_executables={name: {'path': '/usr/bin/' + ('python3.12' if name == 'python3' else name), 'sha256': 'a' * 64}
                                           for name in ('unshare', 'setpriv', 'env', 'python3')})
        case.validate_receipt(receipt, req)
        for field in ('cleanup_complete', 'sealed_cleanup_complete'):
            value = copy.deepcopy(receipt)
            value[field] = False
            with self.assertRaises(ValueError): case.validate_receipt(value, req)
        value = copy.deepcopy(receipt)
        value['system_executables']['python3']['path'] = '/tmp/python3'
        with self.assertRaises(ValueError): case.validate_receipt(value, req)
        value = copy.deepcopy(receipt)
        value['result']['commands'] = [dict(help_rows()[0], merged_sha256='0' * 64)]
        with self.assertRaises(ValueError): case.validate_receipt(value, req)

    def test_closed_single_attempt_staging_never_admits_late_failure(self):
        import native_client_project_artifacts as artifacts
        with tempfile.TemporaryDirectory() as directory:
            parent = Path(directory)
            root = parent / 'output'
            root.mkdir()
            with self.assertRaises(ValueError): case.publish(root, {'unknown-private': b'raw'})
            def fail(path, _): path.write_bytes(b'partial'); raise OSError('inert stage failure')
            with mock.patch.object(artifacts, 'write_zip', fail), self.assertRaises(OSError): case.publish(root, {'admission.json': b'{}'})
            self.assertFalse((root / 'upload').exists())
            clean = parent / 'new-output'
            clean.mkdir()
            real_rename = case.rename_pending
            def late(descriptor): real_rename(descriptor); raise OSError('inert after rename')
            with mock.patch.object(case, 'rename_pending', late), self.assertRaises(OSError): case.publish(clean, {'admission.json': b'{}'})
            with self.assertRaises(ValueError): case.admit_publication(clean, None, 'a' * 40, 'b' * 40, parent / 'admitted')
            self.assertFalse((parent / 'admitted').exists())

    def test_original_output_replacement_is_not_adopted_or_quarantined(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'output'
            root.mkdir()
            fd = os.open(root, os.O_RDONLY | os.O_DIRECTORY)
            expected = case.directory_id(os.fstat(fd))
            try:
                root.rename(root.with_name('original'))
                root.mkdir()
                sentinel = root / 'foreign'
                sentinel.write_bytes(b'retained')
                with self.assertRaises(ValueError): case.publish(root, {'admission.json': b'{}'}, fd, expected)
                self.assertEqual(sentinel.read_bytes(), b'retained')
                self.assertEqual(set(x.name for x in root.iterdir()), {'foreign'})
            finally: os.close(fd)

    def test_parent_publication_admission_binds_actual_zip_and_source(self):
        with tempfile.TemporaryDirectory() as directory:
            parent = Path(directory)
            root = parent / 'output'
            root.mkdir()
            proof = case.publish(root, {'admission.json': json.dumps({'qualification': case.FALSE}).encode()})
            packet = dict(schema_version=1, source_commit='a' * 40, source_tree='b' * 40, nonce='c' * 32, publication=proof)
            admitted = case.admit_publication(root, packet, 'a' * 40, 'b' * 40, parent / 'admitted')
            with zipfile.ZipFile(admitted) as archive:
                self.assertEqual(archive.namelist(), ['admission.json'])
                self.assertTrue(all(x is False for x in json.loads(archive.read('admission.json'))['qualification'].values()))
            for change in ('source', 'tree', 'hash', 'unknown'):
                value = copy.deepcopy(packet)
                if change == 'source': value['source_commit'] = 'd' * 40
                elif change == 'tree': value['source_tree'] = 'd' * 40
                elif change == 'hash': value['publication']['sha256'] = '0' * 64
                else: value['private'] = 'unknown'
                with self.assertRaises(ValueError): case.admit_publication(root, value, 'a' * 40, 'b' * 40, parent / ('refused-' + change))
    def test_publication_nested_identity_requires_closed_integer_fields(self):
        with tempfile.TemporaryDirectory() as directory:
            parent = Path(directory)
            root = parent / 'output'
            root.mkdir()
            proof = case.publish(root, {'admission.json': b'{}'})
            packet = dict(schema_version=1, source_commit='a' * 40, source_tree='b' * 40, nonce='c' * 32, publication=proof)
            for name in ('output', 'upload', 'archive'):
                for mutation in ('boolean', 'extra'):
                    value = copy.deepcopy(packet)
                    if mutation == 'boolean': value['publication'][name]['links' if name == 'archive' else 'device'] = True
                    else: value['publication'][name]['unknown'] = 1
                    destination = parent / (name + mutation)
                    with self.assertRaises(ValueError): case.admit_publication(root, value, 'a' * 40, 'b' * 40, destination)
                    self.assertFalse(destination.exists())
            for digest in (True, '0' * 63, 'G' * 64):
                value = copy.deepcopy(packet)
                value['publication']['sha256'] = digest
                with self.assertRaises(ValueError): case.admit_publication(root, value, 'a' * 40, 'b' * 40, parent / 'bad-digest')
            self.assertEqual(set(x.name for x in root.iterdir()), {'upload'})

    def test_same_byte_archive_or_upload_replacement_after_read_refuses_without_foreign_delete(self):
        for mutation in ('archive', 'upload'):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as directory:
                parent = Path(directory)
                root, destination = parent / 'output', parent / 'admitted'
                root.mkdir()
                proof = case.publish(root, {'admission.json': b'{}'})
                packet = dict(schema_version=1, source_commit='a' * 40, source_tree='b' * 40, nonce='c' * 32, publication=proof)
                original = (root / 'upload/research.zip').read_bytes()
                real_mkdir = Path.mkdir
                def replaced(path, *args, **kwargs):
                    if path == destination:
                        if mutation == 'upload':
                            (root / 'upload').rename(root / 'old-upload')
                            real_mkdir(root / 'upload')
                        else:
                            (root / 'upload/research.zip').rename(root / 'upload/original.zip')
                        (root / 'upload/research.zip').write_bytes(original)
                        (root / 'upload/foreign').write_bytes(b'retained')
                    return real_mkdir(path, *args, **kwargs)
                with mock.patch.object(Path, 'mkdir', replaced), self.assertRaises(ValueError):
                    case.admit_publication(root, packet, 'a' * 40, 'b' * 40, destination)
                self.assertEqual((root / 'upload/foreign').read_bytes(), b'retained')
                self.assertEqual((root / 'upload/research.zip').read_bytes(), original)



    def readout_fixture(self):
        packet = dict(source_commit='a' * 40, source_tree='b' * 40, nonce='c' * 32)
        body = b'inert-stream'
        row = dict(label='version', argv=['/PRIVATE/NONPRINTABLE'], exit=1, error=None,
                   merged_file='user-version.merged.log', merged_bytes=len(body), merged_sha256=case.digest(body))
        result = dict(packet, schema_version=1, scope='user', semantic_complete=False,
                      reason='help-refused', commands=[row])
        receipt = dict(packet, schema_version=1, scope='user', cleanup_complete=True,
                       sealed_cleanup_complete=True, result=result)
        summary = dict(packet, schema_version=1, collection_complete=False, cleanup_complete=True,
                       qualification=dict(case.FALSE), scopes=[receipt], error='admission-refused')
        ledger = dict(accepted=False, before={'PRIVATE/PATH': [1]}, after={'PRIVATE/PATH': [2], 'OTHER/SECRET': [3]})
        members = {'admission.json': json.dumps(summary).encode(), 'user.ledger.json': json.dumps(ledger).encode(),
                   'user-version.merged.log': body}
        return packet, members

    def readout_zip(self, members):
        import io
        stream = io.BytesIO()
        with zipfile.ZipFile(stream, 'w') as archive:
            for name, body in members.items(): archive.writestr(name, body)
        return stream.getvalue()

    def test_readout_projects_only_finite_joined_facts_and_keeps_negative(self):
        packet, members = self.readout_fixture()
        value = case.publication_readout(self.readout_zip(members), packet, time.monotonic() + 5)
        self.assertEqual(value, dict(schema_version=1, reason='admission-refused', collection_complete=False, cleanup_complete=True,
                                    scopes=[dict(scope='user', reason='help-refused', semantic_complete=False, ledger_accepted=False,
                                                 commands=1, failed_commands=1, added_entries=1, removed_entries=0, changed_entries=1)]))
        encoded = json.dumps(value)
        for private in ('PRIVATE', 'SECRET', 'NONPRINTABLE', 'inert-stream', 'merged_file', 'source_commit'):
            self.assertNotIn(private, encoded)

    def test_readout_malformed_unknown_types_sources_and_streams_are_fixed_refusal(self):
        packet, original = self.readout_fixture()
        for mutation in ('json', 'reason', 'bool', 'source', 'scope', 'cleanup', 'qualification', 'stream', 'complete', 'extra'):
            with self.subTest(mutation=mutation):
                members = copy.deepcopy(original)
                summary = json.loads(members['admission.json'])
                if mutation == 'json': members['admission.json'] = b'{"secret":"RAW","secret":1}'
                elif mutation == 'reason': summary['scopes'][0]['result']['reason'] = 'RAW SECRET'
                elif mutation == 'bool': summary['scopes'][0]['result']['semantic_complete'] = 0
                elif mutation == 'source': summary['source_tree'] = 'd' * 40
                elif mutation == 'scope': summary['scopes'][0]['scope'] = '/PRIVATE/PATH'
                elif mutation == 'cleanup': summary['cleanup_complete'] = False
                elif mutation == 'qualification': summary['qualification']['native_mcp_call'] = 0
                elif mutation == 'stream': members['user-version.merged.log'] = b'changed'
                elif mutation == 'complete': summary['collection_complete'] = True
                else: members['RAW-SECRET'] = b'raw bytes'
                if mutation != 'json': members['admission.json'] = json.dumps(summary).encode()
                self.assertEqual(case.publication_readout(self.readout_zip(members), packet, time.monotonic() + 5),
                                 dict(schema_version=1, reason='readout-refused', scopes=[]))

    def test_readout_budget_member_bound_and_crc_never_leak_or_raise(self):
        packet, members = self.readout_fixture()
        raw = self.readout_zip(members)
        for candidate, deadline in ((raw, time.monotonic() - 1),
                                    (raw.replace(b'inert-stream', b'other-stream'), time.monotonic() + 5),
                                    (self.readout_zip(dict(members, **{'user-acp.merged.log': b'x' * (case.LIMIT + 1)})), time.monotonic() + 5)):
            self.assertEqual(case.publication_readout(candidate, packet, deadline),
                             dict(schema_version=1, reason='readout-refused', scopes=[]))

    def test_readout_appends_only_after_original_publication_authority(self):
        packet, members = self.readout_fixture()
        with tempfile.TemporaryDirectory() as temporary:
            parent = Path(temporary); root = parent / 'output'; root.mkdir()
            proof = case.publish(root, members)
            frame = dict(packet, schema_version=1, publication=proof)
            diagnostic = []
            bad = copy.deepcopy(frame); bad['publication']['sha256'] = '0' * 64
            with mock.patch.object(case, 'publication_readout') as readout, self.assertRaises(ValueError):
                case.admit_publication(root, bad, packet['source_commit'], packet['source_tree'], parent / 'bad',
                                       diagnostics=diagnostic, diagnostic_deadline=time.monotonic() + 5)
            self.assertEqual(diagnostic, []); readout.assert_not_called()
            for label, offset in (('fresh', 5), ('expired', -1)):
                diagnostic = []
                actual = case.admit_publication(root, frame, packet['source_commit'], packet['source_tree'], parent / label,
                                                diagnostics=diagnostic, diagnostic_deadline=time.monotonic() + offset)
                self.assertEqual(actual.read_bytes(), (root / 'upload/research.zip').read_bytes())
                self.assertEqual(len(diagnostic), 1)
                self.assertEqual(diagnostic[0]['reason'], 'admission-refused' if offset > 0 else 'readout-refused')

    def test_readout_never_appends_or_touches_foreign_replaced_root(self):
        packet, members = self.readout_fixture()
        with tempfile.TemporaryDirectory() as temporary:
            parent = Path(temporary); root = parent / 'output'; root.mkdir()
            proof = case.publish(root, members)
            frame = dict(packet, schema_version=1, publication=proof)
            root.rename(parent / 'original'); root.mkdir()
            sentinel = root / 'foreign'; sentinel.write_bytes(b'retained')
            diagnostics = []
            with mock.patch.object(case, 'publication_readout') as readout, self.assertRaises(FileNotFoundError):
                case.admit_publication(root, frame, packet['source_commit'], packet['source_tree'], parent / 'admitted',
                                       diagnostics=diagnostics, diagnostic_deadline=time.monotonic() + 5)
            readout.assert_not_called()
            self.assertEqual(diagnostics, [])
            self.assertEqual(sentinel.read_bytes(), b'retained')
            self.assertFalse((parent / 'admitted').exists())

    def test_readout_rejects_unbounded_central_metadata_before_zip_allocation(self):
        import struct
        packet, members = self.readout_fixture()
        raw = self.readout_zip(members)
        for offset, format, value in ((8, '<H', 65535), (12, '<L', 65536), (16, '<L', 0), (20, '<H', 1)):
            changed = bytearray(raw)
            struct.pack_into(format, changed, len(changed) - 22 + offset, value)
            with mock.patch.object(zipfile, 'ZipFile') as parser:
                self.assertEqual(case.publication_readout(bytes(changed), packet, time.monotonic() + 5),
                                 dict(schema_version=1, reason='readout-refused', scopes=[]))
                parser.assert_not_called()


@contextmanager
def native_bootstrap_fixture(*, companions=True):
    """Real owned files and authored pins; no vendor or recorded-current adoption."""
    import project_native_research as shared
    from devin_structured_inventory_admission import metadata_inventory
    from client_automation_native_probe import snapshot
    with tempfile.TemporaryDirectory() as directory, shared.Budget(10) as budget:
        root = Path(directory).resolve()
        with mock.patch.object(case.subprocess, 'run'):
            roots = case.fixtures(root, 'a' * 32)
        (root / 'request.json').write_bytes(b'{}')
        before = metadata_inventory(root, budget)
        profiles = {name: snapshot(path) for name, path in roots.items()}
        native = roots['user_profile']
        for name, mode in bootstrap_case.DIRECTORIES.items():
            path = native / name; path.mkdir(parents=True, exist_ok=True); path.chmod(mode)
        pins = {}
        for name in bootstrap_case.PINNED:
            body = b'' if name.endswith('.lock') else ('PRIVATE_NATIVE_' + name).encode()
            path = native / name; path.write_bytes(body); path.chmod(0o644)
            pins[name] = (len(body), case.digest(body), 0o644)
        uuid = b'01234567-89ab-4cde-8fab-0123456789ab'
        (native / 'agent_id').write_bytes(uuid); (native / 'agent_id').chmod(0o600)
        for name in bootstrap_case.MUTABLE:
            if not companions and name in SQLITE_COMPANIONS:
                continue
            path = native / name
            path.write_bytes(b'x' * (32768 if name.endswith('-shm') else 4096))
            path.chmod(0o644)
        ledger = bootstrap_case.NativeBootstrap()
        with mock.patch.object(bootstrap_case, 'PINNED', pins):
            yield dict(root=root, roots=roots, native=native, before=before, profiles=profiles,
                       budget=budget, bootstrap=ledger, uuid=uuid,
                       capture=lambda: case.post_ledger(root, before, roots, profiles, budget, ledger))


class NativeBootstrapContracts(unittest.TestCase):
    def test_exact_initial_memberships_capture_absent_companions_without_renewal(self):
        for companions in (False, True):
            with self.subTest(companions=companions), native_bootstrap_fixture(companions=companions) as f:
                ledger = f['capture']()
                self.assertTrue(ledger['accepted'])
                names = set(bootstrap_case.NAMES) - (set() if companions else set(SQLITE_COMPANIONS))
                self.assertEqual(set(ledger['native_bootstrap']), names)
                held = copy.deepcopy(f['bootstrap'].created)
                self.assertEqual(set(held), names)
                capture = case.capture_bootstrap_files(f['root'], ledger, f['budget'])
                self.assertEqual(set(capture['files']), set(case.BOOTSTRAP_FILES))
                for name in SQLITE_COMPANIONS:
                    self.assertEqual(capture['files']['home/.grok/' + name] is None, not companions)
                case.validate_bootstrap_capture(ledger, capture)
                self.assertTrue(f['capture']()['accepted'])
                self.assertEqual(f['bootstrap'].created, held)

    def test_partial_companions_journal_and_created_membership_changes_refuse_before_reads(self):
        import client_automation_native_probe as snapshot_case
        kinds = ('first-wal', 'first-shm', 'first-journal', 'created-add', 'created-remove', 'created-inode')
        for kind in kinds:
            with self.subTest(kind=kind), native_bootstrap_fixture(companions=kind == 'created-remove') as f:
                if kind.startswith('created-'):
                    self.assertTrue(f['capture']()['accepted'])
                held, uuid_hash = copy.deepcopy(f['bootstrap'].created), f['bootstrap'].uuid_hash
                if kind == 'created-remove':
                    for name in SQLITE_COMPANIONS: (f['native'] / name).unlink()
                elif kind == 'created-inode':
                    path = f['native'] / 'sessions/session_search.sqlite'
                    replacement = path.with_name('.replacement')
                    replacement.write_bytes(path.read_bytes()); replacement.chmod(0o644)
                    replacement.replace(path)
                else:
                    names = SQLITE_COMPANIONS if kind == 'created-add' else (
                        'sessions/session_search.sqlite-' + kind.removeprefix('first-'),)
                    for name in names:
                        path = f['native'] / name
                        path.write_bytes(b'x' * (32768 if name.endswith('-shm') else 4096))
                        path.chmod(0o644)
                with mock.patch.object(snapshot_case, 'snapshot') as reader:
                    ledger = f['capture']()
                reader.assert_not_called()
                self.assertFalse(ledger['accepted'])
                self.assertEqual(ledger['profiles_after'], {})
                self.assertEqual(f['bootstrap'].created, held)
                self.assertEqual(f['bootstrap'].uuid_hash, uuid_hash)

    def test_uuid_gap_replacement_refuses_before_open(self):
        with native_bootstrap_fixture() as f:
            target, ordinary, opened_counts = f['native'] / 'agent_id', case.ordinary, []
            def replace_before_read(path, limit, **kwargs):
                self.assertEqual(path, target)
                replacement = target.with_name('.replacement')
                replacement.write_bytes(b'11234567-89ab-4cde-8fab-0123456789ab')
                replacement.chmod(0o600); replacement.replace(target)
                with mock.patch.object(case.os, 'open', wraps=case.os.open) as opened:
                    try:
                        return ordinary(path, limit, **kwargs)
                    finally:
                        opened_counts.append(opened.call_count)
            with mock.patch.object(case, 'ordinary', replace_before_read):
                ledger = f['capture']()
            self.assertEqual(opened_counts, [0])
            self.assertFalse(ledger['accepted'])
            self.assertEqual(ledger['profiles_after'], {})
            self.assertIsNone(f['bootstrap'].created)
            self.assertIsNone(f['bootstrap'].uuid_hash)

    def test_shadowed_user_source_replacement_refuses_before_profile_reads(self):
        import client_automation_native_probe as snapshot_case
        with native_bootstrap_fixture() as fixture:
            self.assertTrue(fixture['capture']()['accepted'])
            root = fixture['root']
            rows = [plugin_row(root, 'user', 'uap-acp-foreign-user'),
                    plugin_row(root, 'a', 'uap-acp-collision'), plugin_row(root, 'a', 'uap-acp-foreign-a')]
            case.inventory_rows({'rpc': {'plugins': {'result': {'result': {'plugins': rows}}}}}, root, 'a')
            target = fixture['native'] / 'plugins/uap-acp-collision/skills/inert/SKILL.md'
            original = target.read_bytes()
            with target.open('rb'):
                target.unlink()
                target.write_bytes(original)
                target.chmod(0o600)
                with mock.patch.object(snapshot_case, 'snapshot') as reader:
                    self.assertFalse(fixture['capture']()['accepted'])
                reader.assert_not_called()
                self.assertEqual(target.read_bytes(), original)

    def test_real_first_creation_repeat_uuid_chmod_and_bounded_cache_changes(self):
        with native_bootstrap_fixture() as f:
            original = copy.deepcopy(f['before'])
            first = f['capture']()
            self.assertTrue(first['accepted'])
            self.assertEqual(set(first['native_bootstrap']), set(bootstrap_case.NAMES))
            held = copy.deepcopy(f['bootstrap'].created)
            (f['root'] / 'request.json').write_bytes(b'{"scope":"a"}')
            (f['native'] / 'agent_id').chmod(0o600)
            with (f['native'] / 'logs/unified.jsonl').open('ab') as output:
                output.write(b'PRIVATE_LOG')
            repeated = f['capture']()
            self.assertTrue(repeated['accepted'])
            self.assertEqual(f['bootstrap'].created, held)
            self.assertEqual(f['before'], original)
            serialized = json.dumps(repeated)
            self.assertNotIn('PRIVATE_NATIVE', serialized)
            self.assertNotIn('PRIVATE_LOG', serialized)
            self.assertNotIn(f['uuid'].decode(), serialized)

    def test_unreviewed_members_modes_missing_pins_and_bounds_refuse_before_reads(self):
        import client_automation_native_probe as snapshot_case
        for kind in ('unknown', 'missing', 'mode', 'size', 'cache', 'shm'):
            with self.subTest(kind=kind), native_bootstrap_fixture() as f:
                native = f['native']
                if kind == 'unknown': (native / 'unexpected-token.json').write_bytes(b'PRIVATE')
                elif kind == 'missing': (native / 'README.md').unlink()
                elif kind == 'mode': (native / 'README.md').chmod(0o666)
                elif kind == 'size': (native / 'README.md').write_bytes(b'')
                elif kind == 'cache': (native / 'logs/unified.jsonl').write_bytes(b'x' * ((128 << 10) + 1))
                else: (native / 'sessions/session_search.sqlite-shm').write_bytes(b'x')
                with mock.patch.object(snapshot_case, 'snapshot') as reader:
                    result = f['capture']()
                reader.assert_not_called()
                self.assertFalse(result['accepted'])
                self.assertEqual(result['profiles_after'], {})
                self.assertIsNone(f['bootstrap'].created)

    def test_wrong_pinned_bytes_and_uuid_grammar_never_become_creation_authority(self):
        for name in ('README.md', 'config.toml', 'agent_id'):
            with self.subTest(name=name), native_bootstrap_fixture() as f:
                path = f['native'] / name
                path.write_bytes(b'!' * path.stat().st_size)
                result = f['capture']()
                self.assertFalse(result['accepted'])
                self.assertEqual(result['profiles_after'], {})
                self.assertIsNone(f['bootstrap'].created)

    def test_created_inode_same_bytes_original_project_and_uuid_content_replacement_refuse(self):
        for kind in ('cache-inode', 'pin-inode', 'directory-inode', 'project', 'uuid'):
            with self.subTest(kind=kind), native_bootstrap_fixture() as f:
                self.assertTrue(f['capture']()['accepted'])
                held = copy.deepcopy(f['bootstrap'].created)
                if kind in ('cache-inode', 'pin-inode'):
                    path = f['native'] / ('logs/unified.jsonl' if kind == 'cache-inode' else 'README.md')
                    replacement = path.with_name('.replacement')
                    replacement.write_bytes(path.read_bytes()); replacement.chmod(0o644)
                    replacement.replace(path)
                elif kind == 'directory-inode':
                    path = f['native'] / 'installed-plugins'
                    child = (path / 'registry.lock').read_bytes()
                    (path / 'registry.lock').unlink(); path.rmdir(); path.mkdir(mode=0o755)
                    (path / 'registry.lock').write_bytes(child)
                elif kind == 'project':
                    (f['roots']['b'] / 'foreign').write_bytes(b'changed')
                else:
                    (f['native'] / 'agent_id').write_bytes(b'11234567-89ab-4cde-8fab-0123456789ab')
                result = f['capture']()
                self.assertFalse(result['accepted'])
                self.assertEqual(result['profiles_after'], {})
                self.assertEqual(f['bootstrap'].created, held)

    def test_real_aliases_refuse_without_new_content_read(self):
        import client_automation_native_probe as snapshot_case
        for hardlink in (False, True):
            with self.subTest(hardlink=hardlink), native_bootstrap_fixture() as f:
                if hardlink:
                    os.link(f['native'] / 'README.md', f['root'] / 'external-alias')
                else:
                    path = f['native'] / 'README.md'; path.unlink(); path.symlink_to(f['root'] / 'request.json')
                with mock.patch.object(snapshot_case, 'snapshot') as reader, self.assertRaises(ValueError):
                    f['capture']()
                reader.assert_not_called()

    def test_owner_volume_link_and_control_authority_are_checked_without_body_reads(self):
        from devin_structured_inventory_admission import metadata_inventory
        for field, index in (('device', 0), ('owner', 4), ('group', 5), ('links', 3)):
            with self.subTest(field=field), native_bootstrap_fixture() as f:
                after = metadata_inventory(f['root'], f['budget'])
                key = 'home/.grok/README.md'; changed = list(after[key]); changed[index] += 1
                after[key] = tuple(changed)
                with self.assertRaises(ValueError): f['bootstrap'].metadata(f['before'], after)
        with native_bootstrap_fixture() as f:
            after = metadata_inventory(f['root'], f['budget'])
            value = list(after['request.json']); value[1] += 1; after['request.json'] = tuple(value)
            with self.assertRaises(ValueError): f['bootstrap'].metadata(f['before'], after)

    def test_late_snapshot_or_metadata_mismatch_cannot_commit_a_partial_baseline(self):
        import client_automation_native_probe as snapshot_case
        import devin_structured_inventory_admission as metadata_case
        for kind in ('snapshot', 'metadata'):
            with self.subTest(kind=kind), native_bootstrap_fixture() as f:
                if kind == 'snapshot':
                    real = snapshot_case.snapshot
                    def changed(path):
                        value = real(path)
                        if path == f['native']: value['agent_id']['inode'] += 1
                        return value
                    patch = mock.patch.object(snapshot_case, 'snapshot', changed)
                else:
                    real_metadata, calls = metadata_case.metadata_inventory, []
                    def changed_metadata(root, budget):
                        value = real_metadata(root, budget); calls.append(True)
                        if len(calls) == 2:
                            row = list(value['.']); row[1] += 1; value['.'] = tuple(row)
                        return value
                    patch = mock.patch.object(metadata_case, 'metadata_inventory', changed_metadata)
                with patch: result = f['capture']()
                self.assertFalse(result['accepted'])
                self.assertEqual(result['profiles_after'], {})
                self.assertIsNone(f['bootstrap'].created)


if __name__ == '__main__':
    unittest.main()
