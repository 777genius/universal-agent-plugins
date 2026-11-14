"""Inert tests execute actual parser, capture fence and artifacts; no vendor."""
import base64
import copy
from contextlib import contextmanager, redirect_stdout
import io
import json
import os
from pathlib import Path
import re
import selectors
import subprocess
import sys
import tempfile
import time
from types import SimpleNamespace
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


def startup_row(root, scope):
    body = json.dumps(dict(grokVersion='1.0.70', cwd=str(root / scope), mcpServers=[dict(
        name=case.STARTUP_NAME, transport='stdio', target='/usr/bin/python3',
        source=dict(type='configToml', path=str(root / 'home/.grok/config.toml')))])).encode()
    return dict(label='mcp-inventory', argv=['/sealed/grok', 'inspect', '--json'], exit=0, error=None,
                merged_base64=base64.b64encode(body).decode(), merged_bytes=len(body), merged_sha256=case.digest(body))


class FrontendPublicationContracts(unittest.TestCase):
    def preflight_refusal(self):
        import project_native_research as shared
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / 'not-created'
            with (mock.patch.object(shared.native, 'require_hosted', side_effect=ValueError('PRIVATE path/token')),
                  mock.patch.object(shared, 'acquire', side_effect=AssertionError('native acquisition')),
                  redirect_stdout(io.StringIO()) as stream):
                self.assertEqual(case.frontend(output), 1)
            self.assertFalse(output.exists())
            return stream.getvalue().encode()

    def test_preflight_refusal_is_one_closed_frame_without_exception_text(self):
        body = self.preflight_refusal()
        self.assertNotIn(b'PRIVATE', body)
        self.assertEqual(case.document(body), dict(schema_version=1, kind='frontend-publication-refused',
            stage='preflight', error_class='contract', source_identity_checked=False,
            cleanup_complete=False, publication_attempted=False))

    def test_negative_frame_requires_exact_types_enums_exit_and_one_frame(self):
        body = self.preflight_refusal()
        packet = case.document(body)
        for key, value in (('schema_version', True), ('kind', 'PRIVATE'), ('stage', 'PRIVATE'), ('stage', []),
                ('error_class', 'PRIVATE'), ('error_class', []), ('cleanup_complete', 1), ('publication_attempted', 0),
                ('source_identity_checked', None), ('publication', {}), ('extra', 'PRIVATE')):
            changed = dict(packet, **{key: value})
            raw = json.dumps(changed, separators=(',', ':')).encode() + b'\n'
            with self.subTest(key=key), redirect_stdout(io.StringIO()) as stream, self.assertRaises(ValueError):
                case.frontend_packet(raw, 1)
            self.assertEqual(stream.getvalue(), '')
        for key in packet:
            changed = {name: value for name, value in packet.items() if name != key}
            raw = json.dumps(changed, separators=(',', ':')).encode() + b'\n'
            with self.subTest(missing=key), redirect_stdout(io.StringIO()) as stream, self.assertRaises(ValueError):
                case.frontend_packet(raw, 1)
            self.assertEqual(stream.getvalue(), '')
        for raw, exit_code in ((body, 0), (body, True), (body, 2), (body + b'\n', 1),
                (body + b'{}\n', 1), (b'{}\n' + body, 1), (b'', 1)):
            with self.subTest(exit=exit_code, size=len(raw)), redirect_stdout(io.StringIO()) as stream, self.assertRaises(ValueError):
                case.frontend_packet(raw, exit_code)
            self.assertEqual(stream.getvalue(), '')

    def test_actual_workflow_blocks_refusal_before_native_staging_or_admitted_output(self):
        import native_client_research as native
        workflow = (Path(__file__).parents[1] / '.github/workflows/client-automation-native.yml').read_text()
        blocks = re.findall(r"(?ms)^          python3 - <<'PYTHON'\n(.*?)^          PYTHON$", workflow)
        blocks = [block for block in blocks if 'capture.frontend_packet' in block]
        self.assertEqual(len(blocks), 4)
        capture_prefixes = dict(CLI_MODE='GROK_FRONTEND_CAPTURE=',
            SESSION_MODE='GROK_FRONTEND_CAPTURE=', LOCK_MODE='GROK_FRONTEND_CAPTURE=',
            READONLY_MODE='GROK_READONLY_FRONTEND_CAPTURE=')
        modes = [re.search(r'mode=capture\.([A-Z_]+)', block)[1] for block in blocks]
        self.assertCountEqual(modes, capture_prefixes)
        body = self.preflight_refusal()
        with tempfile.TemporaryDirectory() as temporary:
            github_output = Path(temporary) / 'github-output'
            retained = Path(temporary) / 'frontend-failure.zip'
            facts = dict(bytes=123, sha256='c' * 64)
            for block, mode in zip(blocks, modes):
                for error, exit_code in ((None, 1), ('PRIVATE bounded capture refusal', 1), (None, 2), (None, 0)):
                    github_output.write_bytes(b'original output\n')
                    expected = ValueError if error is None and exit_code in (0, 1) else SystemExit
                    with (self.subTest(error=error, exit=exit_code),
                          mock.patch.dict(os.environ, RUNNER_TEMP=temporary, GITHUB_OUTPUT=str(github_output), EXPECTED_COMMIT='a' * 40),
                          mock.patch.object(subprocess, 'check_output', return_value='b' * 40),
                          mock.patch.object(native, 'run_probe', return_value=({'error': error, 'exit': exit_code}, body, b'PRIVATE stderr')) as probe,
                          mock.patch.object(case, 'retain_frontend_failure', return_value=(retained, facts)) as retain,
                          mock.patch.object(case, 'admit_publication', side_effect=AssertionError('negative upload')) as admit,
                          mock.patch.object(tempfile, 'mkdtemp', side_effect=AssertionError('negative staging')),
                          redirect_stdout(io.StringIO()) as stream, self.assertRaises(expected)):
                        exec(compile('\n'.join(line[10:] for line in block.splitlines()), '<workflow>', 'exec'), {})
                    self.assertEqual(probe.call_args.kwargs, dict(timeout=180, limit=1 << 20, memory_only=True))
                    admit.assert_not_called()
                    complete = error is None and exit_code in (0, 1)
                    if complete:
                        self.assertEqual(retain.call_args.args[1:6], ({'error': error, 'exit': exit_code}, body, b'PRIVATE stderr', 'a' * 40, 'b' * 40))
                    else:
                        retain.assert_not_called()
                    self.assertEqual(github_output.read_bytes(), b'original output\n' +
                        (('frontend_failure=' + str(retained) + '\n').encode() if complete else b''))
                    self.assertNotIn('PRIVATE', stream.getvalue())
                    self.assertEqual(stream.getvalue(),
                        (capture_prefixes[mode] + json.dumps(facts, separators=(',', ':')) + '\n' if complete else '') +
                        ('GROK_FRONTEND_REFUSAL=' + body.decode() if error is None and exit_code == 1 else ''))



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

    def test_authored_startup_notification_is_exact_and_does_not_complete_inventory(self):
        root = Path('/tmp/uap-owned')
        expected = case.startup_servers(root)
        req = {'nonce': 'a' * 32, 'scope': 'user'}
        notification = dict(jsonrpc='2.0', method='_x.ai/mcp/servers_updated',
                            params=dict(mcpServers=list(expected)))
        state = case.rpc_state()
        self.assertFalse(case.rpc_frame(notification, state, req, lambda _: None, expected_servers=expected))
        self.assertIsNone(state['plugins'])
        self.assertEqual(state['notifications'], [notification])
        with self.assertRaises(ValueError):
            case.rpc_frame(notification, state, req, lambda _: None, expected_servers=expected)
        with self.assertRaises(ValueError):
            case.rpc_frame(notification, case.rpc_state(), req, lambda _: None)
        for mutation in ('empty', 'duplicate', 'name', 'source', 'type', 'command', 'args', 'extra', 'missing', 'params'):
            changed = copy.deepcopy(notification)
            rows = changed['params']['mcpServers']
            if mutation == 'empty': rows.clear()
            elif mutation == 'duplicate': rows.append(copy.deepcopy(rows[0]))
            elif mutation == 'extra': rows[0]['unreviewed'] = True
            elif mutation == 'missing': del rows[0]['source']
            elif mutation == 'params': changed['params']['unreviewed'] = True
            elif mutation == 'args': rows[0]['args'][2] = '/foreign/marker'
            else: rows[0][mutation] = 'foreign'
            with self.subTest(mutation=mutation), self.assertRaises(ValueError):
                case.rpc_frame(changed, case.rpc_state(), req, lambda _: None, expected_servers=expected)

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
                      {'jsonrpc': '2.0', 'method': '_x.ai/mcp/servers_updated',
                       'params': {'mcpServers': list(case.startup_servers(root))}},
                      {'jsonrpc': '2.0', 'id': 2, 'result': {'result': {'plugins': rows}}}]
            child = """import json, sys
frames = json.loads(sys.argv[1])
for method, selected in zip(('initialize', '_x.ai/plugins/list'), (frames[:1], frames[1:])):
    assert json.loads(sys.stdin.readline())['method'] == method
    for frame in selected:
        print(json.dumps(frame), flush=True)
sys.stdin.read()
"""
            state = case.rpc_state()
            record = case.command([sys.executable, '-I', '-c', child, json.dumps(frames)], 'acp', root,
                                  {'PATH': os.defpath}, time.monotonic() + 3, case.LIMIT, state, self.request(root),
                                  expected_servers=case.startup_servers(root))
            self.assertIsNone(record['error'])
            self.assertEqual(state['plugins'], frames[2])
            self.assertEqual(state['notifications'], frames[1:2])
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
        for failed_index in range(len(case.METADATA_LABELS)):
            with self.subTest(command=failed_index), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                request = root / 'request.json'
                request.write_text(json.dumps(self.request(root)))
                calls = []
                def incomplete(*args, **kwargs):
                    index = len(calls)
                    calls.append(args)
                    self.assertEqual(kwargs, {'expected_servers': case.startup_servers(root)} if index == 5 else {})
                    if index == failed_index: raise OSError('inert incomplete pipe/wait')
                    return help_rows()[index] if index < 4 else startup_row(root, 'user')
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
def native_bootstrap_fixture(*, companions=True, startup=False):
    """Real owned files and authored pins; no vendor or recorded-current adoption."""
    import project_native_research as shared
    from devin_structured_inventory_admission import metadata_inventory
    from client_automation_native_probe import snapshot
    with tempfile.TemporaryDirectory() as directory, shared.Budget(10) as budget:
        root = Path(directory).resolve()
        with mock.patch.object(case.subprocess, 'run'):
            roots = case.fixtures(root, 'a' * 32)
        original_config = case.startup_fixture_config(root) if startup else None
        (root / 'request.json').write_bytes(b'{}')
        before = metadata_inventory(root, budget)
        profiles = {name: snapshot(path) for name, path in roots.items()}
        native = roots['user_profile']
        for name, mode in bootstrap_case.DIRECTORIES.items():
            path = native / name; path.mkdir(parents=True, exist_ok=True); path.chmod(mode)
        pins = {}
        for name in bootstrap_case.PINNED:
            if name in ('config.toml', '.config-init.lock') and startup:
                pins[name] = bootstrap_case.PINNED[name]
                continue
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
        ledger = bootstrap_case.NativeBootstrap(original_config=original_config)
        with mock.patch.object(bootstrap_case, 'PINNED', pins):
            yield dict(root=root, roots=roots, native=native, before=before, profiles=profiles,
                       budget=budget, bootstrap=ledger, uuid=uuid,
                       capture=lambda: case.post_ledger(root, before, roots, profiles, budget, ledger))


class NativeBootstrapContracts(unittest.TestCase):
    def test_startup_tripwire_is_eligible_and_only_authored_config_is_preexisting(self):
        import tomllib
        with native_bootstrap_fixture(startup=True) as f:
            config = tomllib.loads((f['native'] / 'config.toml').read_text())
            selected = config['mcp_servers'][case.STARTUP_NAME]
            self.assertIs(selected['enabled'], True)
            self.assertEqual(selected['command'], '/usr/bin/python3')
            ledger = f['capture']()
            self.assertTrue(ledger['accepted'])
            self.assertEqual(set(ledger['native_bootstrap']), bootstrap_case.NAMES - {'config.toml', '.config-init.lock'})
            self.assertNotIn('home/.grok/.config-init.lock', ledger['before'])
            self.assertNotIn('home/.grok/.config-init.lock', ledger['after'])
            self.assertEqual(ledger['profiles_before']['user_profile']['config.toml'],
                             ledger['profiles_after']['user_profile']['config.toml'])
            self.assertTrue(f['capture']()['accepted'])
            # Positive control executes only our authored script; a native launch
            # uses the identical argv and produces an unadmitted persistent leaf.
            result = subprocess.run([selected['command'], *selected['args']], check=False, timeout=3)
            self.assertEqual(result.returncode, 93)
            self.assertEqual((f['root'] / 'unexpected-startup').read_bytes(), b'unexpected')
            self.assertFalse(f['capture']()['accepted'])

    def test_startup_config_replacement_change_and_missing_other_pin_refuse_before_reads(self):
        import client_automation_native_probe as snapshot_case
        for kind in ('replace', 'change', 'remove-pin', 'mode', 'new-lock', 'prior-lock'):
            with self.subTest(kind=kind), native_bootstrap_fixture(startup=True) as f:
                target = f['native'] / 'config.toml'
                if kind == 'replace':
                    replacement = f['native'] / 'replacement'
                    replacement.write_bytes(target.read_bytes()); replacement.chmod(0o600); replacement.replace(target)
                elif kind == 'change': target.write_bytes(target.read_bytes() + b'# concurrent\n')
                elif kind == 'mode': target.chmod(0o644)
                elif kind in ('new-lock', 'prior-lock'):
                    lock = f['native'] / '.config-init.lock'
                    lock.write_bytes(b''); lock.chmod(0o644)
                    if kind == 'prior-lock':
                        from devin_structured_inventory_admission import metadata_inventory
                        key = 'home/.grok/.config-init.lock'
                        f['before'][key] = metadata_inventory(f['root'], f['budget'])[key]
                else: (f['native'] / '.metadata_version').unlink()
                with mock.patch.object(snapshot_case, 'snapshot') as reader:
                    self.assertFalse(f['capture']()['accepted'])
                reader.assert_not_called()

    def test_startup_inventory_requires_effective_native_source_and_never_treats_absence_as_safe(self):
        root = Path('/tmp/uap-owned')
        for scope in case.SCOPES:
            case.startup_inventory(startup_row(root, scope), root, scope)
        for mutation in ('empty', 'duplicate', 'disabled', 'blocked', 'source', 'target', 'name', 'version', 'cwd', 'problems'):
            row = startup_row(root, 'a')
            report = json.loads(base64.b64decode(row['merged_base64']))
            selected = report['mcpServers'][0]
            if mutation == 'empty': report['mcpServers'] = []
            elif mutation == 'duplicate': report['mcpServers'].append(copy.deepcopy(selected))
            elif mutation == 'disabled': selected['disabled'] = True
            elif mutation == 'blocked': selected['disabledReason'] = 'not allowed'
            elif mutation == 'source': selected['source']['path'] = '/foreign/config.toml'
            elif mutation == 'target': selected['target'] = '/usr/bin/false'
            elif mutation == 'name': selected['name'] = 'foreign'
            elif mutation == 'version': report['grokVersion'] = '1.0.69'
            elif mutation == 'cwd': report['cwd'] = str(root / 'b')
            else: report['mcpConfigProblems'] = [{'server': case.STARTUP_NAME}]
            row['merged_base64'] = base64.b64encode(json.dumps(report).encode()).decode()
            with self.subTest(mutation=mutation), self.assertRaises(ValueError):
                case.startup_inventory(row, root, 'a')

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



class SessionOnlyContracts(unittest.TestCase):
    def config_capture_fixture(self, root):
        import devin_structured_inventory_admission as metadata
        from client_automation_native_probe import snapshot
        (root/'home/.grok').mkdir(parents=True)
        case.session_fixture_config(root,'c'*32)
        (root/'foreign').write_bytes(b'preserve')
        original=snapshot(root/'home/.grok')['config.toml']
        budget=SimpleNamespace(check=lambda:None)
        before=metadata.metadata_inventory(root,budget)
        replacement=root/'replacement'
        replacement.write_bytes(case.session_config_body('c'*32)+b'\n[marketplace]\ndefault_skills_installs_purged = true\n')
        replacement.chmod(0o600);replacement.replace(root/case.SESSION_CONFIG)
        ledger=case.session_metadata_ledger(root,before,budget)
        receipt=dict(cleanup_complete=True,sealed_cleanup_complete=True)
        return metadata,budget,original,ledger,receipt

    def req(self):
        return dict(nonce='c'*32, scope='a', input_root='/tmp/owned', mode=case.SESSION_MODE)

    def state(self):
        return dict(initialize=None, plugins=None, notifications=[], session=None)

    def init(self, state, req, sent):
        return case.session_rpc_frame({'jsonrpc':'2.0','id':1,'result':{'protocolVersion':1,
            '_meta':{'agentVersion':'1.0.70'}}},state,req,sent.append)

    def test_actual_session_sequence_uses_only_returned_id_and_no_prompt(self):
        state, req, sent = self.state(), self.req(), []
        self.assertFalse(self.init(state,req,sent))
        self.assertEqual(sent[-1]['method'],'session/new')
        self.assertEqual(sent[-1]['params'],{'cwd':'/tmp/owned/a','mcpServers':[],'_meta':{'modelId':'uap-local-driver'}})
        reply={'jsonrpc':'2.0','id':2,'result':{'sessionId':'actual-native-id'}}
        self.assertFalse(case.session_rpc_frame(reply,state,req,sent.append))
        self.assertEqual(sent[-1],{'jsonrpc':'2.0','id':3,'method':'_x.ai/plugins/list','params':{'sessionId':'actual-native-id'}})
        self.assertTrue(case.session_rpc_frame({'jsonrpc':'2.0','id':3,'result':{'result':{'plugins':[]}}},state,req,sent.append))
        self.assertEqual(len(sent),2)

    def test_session_reply_and_setup_notifications_remain_finite_and_joined(self):
        for change in ('callback','phase','sid','id','unknown','nested'):
            state, req, sent = self.state(), self.req(), []
            self.init(state,req,sent)
            notice={'jsonrpc':'2.0','method':'_x.ai/session/setup','params':{'method':'session/new','phase':'auth','sessionId':None}}
            case.session_rpc_frame(notice,state,req,sent.append)
            response={'jsonrpc':'2.0','id':2,'result':{'sessionId':'owned'}}
            if change=='callback': notice['method']='session/request_permission'; notice['id']=99
            elif change=='phase': notice['params']['phase']='PRIVATE_TOKEN'
            elif change=='sid': response['result']['sessionId']=True
            elif change=='id': response['id']=4
            elif change=='unknown': response['result']['private']='COOKIE'
            else: response['result']['_meta']={'currentWorkingDirectory':'/foreign'}
            with self.subTest(change=change),self.assertRaises(ValueError):
                case.session_rpc_frame(notice if change in ('callback','phase') else response,state,req,sent.append)
        state,req,sent=self.state(),self.req(),[];self.init(state,req,sent)
        case.session_rpc_frame({'jsonrpc':'2.0','id':2,'result':{'sessionId':'owned'}},state,req,sent.append)
        with self.assertRaises(ValueError):
            case.session_rpc_frame({'jsonrpc':'2.0','method':'_x.ai/session/setup','params':{'method':'session/new','phase':'response_ready','sessionId':'foreign'}},state,req,sent.append)

    def test_measured_agent_build_sequence_preserves_phase_bounds_and_real_id_join(self):
        state,req,sent=self.state(),self.req(),[];self.init(state,req,sent)
        for phase in ('auth','resolve_workspace','folder_trust','plugin_registry','mcp_merge','persistence_init','spawn_session_actor','agent_build'):
            notice={'jsonrpc':'2.0','method':'_x.ai/session/setup','params':{'method':'session/new','phase':phase,
                    'sessionId':'native-owned' if phase in ('persistence_init','spawn_session_actor','agent_build') else None}}
            self.assertFalse(case.session_rpc_frame(notice,state,req,sent.append))
            if phase=='persistence_init':
                case.session_rpc_frame({'jsonrpc':'2.0','method':'_x.ai/mcp/servers_updated','params':{'mcpServers':[]}},state,req,sent.append)
        self.assertIsNone(state['session']);self.assertIsNone(state['plugins'])
        self.assertEqual([row['method'] for row in sent],['session/new'])
        with self.assertRaisesRegex(ValueError,'session-notification-identity'):
            case.session_rpc_frame({'jsonrpc':'2.0','id':2,'result':{'sessionId':'foreign'}},copy.deepcopy(state),req,sent.append)
        with self.assertRaisesRegex(ValueError,'setup-notification'):
            case.session_rpc_frame(dict(notice,params=dict(notice['params'],phase='agent_build_unknown')),state,req,sent.append)
        case.session_rpc_frame(notice,state,req,sent.append)
        with self.assertRaisesRegex(ValueError,'setup-notification'):
            case.session_rpc_frame(notice,state,req,sent.append)
        state['notifications']=[notice]*32
        with self.assertRaisesRegex(ValueError,'notification-bound'):
            case.session_rpc_frame(notice,state,req,sent.append)

    def mcp_initialized_fixture(self):
        state,req,sent=self.state(),self.req(),[];self.init(state,req,sent)
        for phase in ('auth','resolve_workspace','folder_trust','plugin_registry','mcp_merge',
                      'persistence_init','spawn_session_actor','agent_build','model_switch'):
            sid=None if phase in ('auth','resolve_workspace','folder_trust','plugin_registry','mcp_merge') else 'native-owned'
            case.session_rpc_frame({'jsonrpc':'2.0','method':'_x.ai/session/setup',
                'params':{'method':'session/new','phase':phase,'sessionId':sid}},state,req,sent.append)
            if phase=='persistence_init':
                case.session_rpc_frame({'jsonrpc':'2.0','method':'_x.ai/mcp/servers_updated',
                    'params':{'mcpServers':[]}},state,req,sent.append)
        # Original b397 frame12, with only its native session ID anonymized.
        notice={'jsonrpc':'2.0','method':'_x.ai/mcp_initialized',
                'params':{'sessionId':'native-owned','mcpToolCount':0,'elapsedMs':0}}
        return state,req,sent,notice

    def test_measured_mcp_initialized_prefix_keeps_no_tools_and_real_session_join(self):
        state,req,sent,notice=self.mcp_initialized_fixture()
        self.assertEqual(len(state['notifications']),10)
        self.assertFalse(case.session_rpc_frame(notice,state,req,sent.append))
        self.assertEqual(len(state['notifications']),11)
        self.assertIsNone(state['session']);self.assertIsNone(state['plugins'])
        self.assertEqual([row['method'] for row in sent],['session/new'])
        for future in ({'jsonrpc':'2.0','id':2,'result':{'sessionId':'foreign'}},
                       {'jsonrpc':'2.0','method':'_x.ai/session/setup',
                        'params':{'method':'session/new','phase':'response_ready','sessionId':'foreign'}}):
            with self.assertRaisesRegex(ValueError,'session-notification-identity'):
                case.session_rpc_frame(future,copy.deepcopy(state),req,sent.append)
        case.session_rpc_frame({'jsonrpc':'2.0','id':2,'result':{'sessionId':'native-owned'}},state,req,sent.append)
        self.assertTrue(case.session_rpc_frame({'jsonrpc':'2.0','id':3,'result':{'result':{'plugins':[]}}},state,req,sent.append))
        self.assertEqual([row['method'] for row in sent],['session/new','_x.ai/plugins/list'])

    def test_mcp_initialized_payload_is_closed_typed_and_bounded(self):
        changes=[('mcpToolCount',True),('mcpToolCount',1),('mcpToolCount',-1),('mcpToolCount',0.0),
                 ('elapsedMs',False),('elapsedMs',-1),('elapsedMs',1),('elapsedMs',30000),('elapsedMs',30001),('elapsedMs',0.0),
                 ('sessionId',None),('sessionId',''),('sessionId',True),('sessionId','foreign'),
                 ('sessionId','x'*257),('sessionId','bad\n'),('sessionId','nonascii-\u00e9'),('extra',0)]
        for key,value in changes:
            with self.subTest(key=key,value=value):
                state,req,sent,notice=self.mcp_initialized_fixture();notice['params'][key]=value
                with self.assertRaises(ValueError):case.session_rpc_frame(notice,state,req,sent.append)
                self.assertEqual(len(state['notifications']),10)
        for key in ('sessionId','mcpToolCount','elapsedMs'):
            state,req,sent,notice=self.mcp_initialized_fixture();del notice['params'][key]
            with self.subTest(missing=key),self.assertRaises(ValueError):
                case.session_rpc_frame(notice,state,req,sent.append)

    def test_mcp_initialized_joins_actual_response_without_setup_ids(self):
        for notice_first in (False, True):
            state,req,sent=self.state(),self.req(),[];self.init(state,req,sent)
            notice={'jsonrpc':'2.0','method':'_x.ai/mcp_initialized',
                    'params':{'sessionId':'native-owned','mcpToolCount':0,'elapsedMs':0}}
            response={'jsonrpc':'2.0','id':2,'result':{'sessionId':'foreign'}}
            first,second=(notice,response) if notice_first else (response,notice)
            case.session_rpc_frame(first,state,req,sent.append)
            before,outbound=copy.deepcopy(state),copy.deepcopy(sent)
            with self.subTest(notice_first=notice_first),self.assertRaisesRegex(ValueError,'session-notification-identity'):
                case.session_rpc_frame(second,state,req,sent.append)
            self.assertEqual(state,before);self.assertEqual(sent,outbound)

    def test_mcp_initialized_order_duplicate_unknown_and_total_bounds_refuse(self):
        for change in ('before-initialize','duplicate','after-inventory','wrong-returned-id','unknown-method','callback','total-bound'):
            state,req,sent,notice=self.mcp_initialized_fixture()
            if change=='before-initialize':state=self.state()
            elif change=='duplicate':case.session_rpc_frame(notice,state,req,sent.append)
            elif change in ('after-inventory','wrong-returned-id'):
                # Existing setup identity is checked when the session reply arrives.
                case.session_rpc_frame({'jsonrpc':'2.0','id':2,'result':{'sessionId':'native-owned'}},state,req,sent.append)
                if change=='after-inventory':
                    case.session_rpc_frame({'jsonrpc':'2.0','id':3,'result':{'result':{'plugins':[]}}},state,req,sent.append)
                else:notice['params']['sessionId']='foreign'
            elif change=='unknown-method':notice['method']='_x.ai/mcp_initialized_unknown'
            elif change=='callback':notice['id']=4
            else:state['notifications']*=4
            with self.subTest(change=change),self.assertRaises(ValueError):
                case.session_rpc_frame(notice,state,req,sent.append)

    def available_commands_fixture(self):
        # Exact frame13 from artifact14100597648; prior inert fixture uses the same native SID.
        raw = (
            b'{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"01a1d896-e6c5-7e73-b878-fe73299939f2","update"'
            b':{"sessionUpdate":"available_commands_update","availableCommands":[{"name":"compact","description":"Compress con'
            b'versation history to save context window","input":null},{"name":"always-approve","description":"Toggle always-ap'
            b'prove mode (skip all permission prompts)","input":{"hint":"on|off"}},{"name":"context","description":"Show conte'
            b'xt window usage and session stats","input":null},{"name":"plugins","description":"Manage plugins (list, reload, '
            b'trust, add, remove)","input":{"hint":"list | reload | trust <path> | add <path> | remove <path>"}},{"name":"relo'
            b'ad-plugins","description":"Reload plugins from disk (alias for /plugins reload)","input":null},{"name":"session-'
            b'info","description":"Show session details (model, turns, context usage)","input":null},{"name":"feedback","descr'
            b'iption":"Send feedback about the current session","input":{"hint":"feedback text"}},{"name":"deep-research","des'
            b'cription":"Research with bounded parallel agents, cross-check evidence, and write a cited report","input":{"hint'
            b'":"<query>"},"_meta":{"workflowSource":"builtin","workflowPath":"/buildkite/builds/buildkite-agent-ci-cpu-cuda-x'
            b'large-84697955c7-wfl22-1/xai/spacexai-checks/crates/codegen/xai-grok-shell/src/session/workflows/deep_research.r'
            b'hai"}},{"name":"workflow","description":"Launch a saved workflow, list runs, or manage a run (pause, resume, sto'
            b'p, save)","input":{"hint":"<name> [--agent-budget N] [--effort LEVEL] [args] | runs | pause|resume|stop|save [na'
            b'me]"}},{"name":"goal","description":"Set, manage, or check an autonomous goal","input":{"hint":"<objective> [--b'
            b'udget <tokens>] | status | pause | resume | clear"}},{"name":"loop","description":"Run a prompt on a recurring i'
            b'nterval","input":{"hint":"[interval] <prompt>"}}],"_meta":{"tools":["run_terminal_command","read_file","search_r'
            b'eplace","list_dir","grep","kill_command_or_subagent","todo_write","get_command_or_subagent_output","spawn_subage'
            b'nt","scheduler_create","scheduler_delete","scheduler_list","monitor","search_tool","use_tool","workflow","enter_'
            b'plan_mode","exit_plan_mode","ask_user_question","send_feedback","web_search","image_gen","image_edit","image_to_'
            b'video","reference_to_video","write"]}},"_meta":{"totalTokens":1337,"requestTrimmedTokens":0,"eventId":"01a1d896-'
            b'e6c5-7e73-b878-fe73299939f2-0","agentTimestampMs":1794635130595,"updateType":"AvailableCommandsUpdate","updatePa'
            b'rams":{"commandsCount":11}}}}')
        self.assertEqual(case.digest(raw),'f9457d63ac0bb5d432a98790a84c119a3354b2ef53c181fd50600a6da5bfe533')
        frame=case.document(raw);sid=frame['params']['sessionId']
        state,req,sent,notice=self.mcp_initialized_fixture()
        for prior in state['notifications']:
            if prior['params'].get('sessionId') is not None:prior['params']['sessionId']=sid
        notice['params']['sessionId']=sid
        case.session_rpc_frame(notice,state,req,sent.append)
        return state,req,sent,frame

    def test_measured_available_commands_is_only_a_captured_notification(self):
        state,req,sent,frame=self.available_commands_fixture();before=copy.deepcopy(state);outbound=copy.deepcopy(sent)
        with mock.patch.object(case.os,'open',side_effect=AssertionError('metadata path was opened')):
            self.assertFalse(case.session_rpc_frame(frame,state,req,sent.append))
        self.assertEqual(state,dict(before,notifications=before['notifications']+[frame]))
        self.assertIsNone(state['session']);self.assertIsNone(state['plugins']);self.assertEqual(sent,outbound)
        self.assertEqual([x['method'] for x in sent],['session/new'])
        with self.assertRaises(ValueError):case.rpc_frame(frame,case.rpc_state(),req,sent.append)
        self.assertEqual(sent,outbound)

    def test_available_commands_closed_shape_and_bounds_refuse_without_capture_or_send(self):
        changes=[(('params','_meta','agentTimestampMs'),v) for v in (True,0,-1,1<<63,1.0)]
        changes += [(('params','_meta','totalTokens'),v) for v in (True,-1,(1<<20)+1,1.0)]
        changes += [(('params','_meta','requestTrimmedTokens'),v) for v in (True,1,-1,0.0)]
        changes += [(('params','_meta','updateParams','commandsCount'),v) for v in (True,10,12)]
        changes += [(('params','sessionId'),v) for v in (None,True,'','x'*257,'bad\n','nonascii-é')]
        changes += [(('params','_meta','eventId'),'foreign-0'),(('params','_meta','updateType'),'ToolCall'),
            (('params','update','sessionUpdate'),'tool_call'),(('params','extra'),{}),
            (('params','update','_meta','extra'),{}),(('params','_meta','extra'),{}),
            (('params','_meta','updateParams','extra'),{}),
            (('params','update','availableCommands',0,'name'),'unknown'),
            (('params','update','availableCommands',0,'description'),'x'*257),
            (('params','update','availableCommands',0,'description'),'bad\n'),
            (('params','update','availableCommands',0,'_meta'),{}),
            (('params','update','availableCommands',1,'input','hint'),'x'*129),
            (('params','update','availableCommands',1,'input','extra'),True),
            (('params','update','availableCommands',7,'_meta','workflowSource'),'project'),
            (('params','update','availableCommands',7,'_meta','workflowPath'),'x'*1025),
            (('params','update','availableCommands',7,'_meta','workflowPath'),True),
            (('params','update','_meta','tools',0),'unknown'),
            (('params','update','_meta','tools',1),'run_terminal_command')]
        for path,value in changes:
            state,req,sent,frame=self.available_commands_fixture();before=copy.deepcopy(state);outbound=copy.deepcopy(sent)
            parent=frame
            for key in path[:-1]:parent=parent[key]
            parent[path[-1]]=value
            with self.subTest(path=path,value=value),self.assertRaises(ValueError):
                case.session_rpc_frame(frame,state,req,sent.append)
            self.assertEqual(state,before);self.assertEqual(sent,outbound)

    def test_available_commands_order_duplicate_catalog_and_mode_are_finite(self):
        for change in ('before-init','after-result','duplicate','callback','method','mode','total-bound',
                       'missing-key','missing-command','reordered-command','extra-tool','reordered-tool'):
            state,req,sent,frame=self.available_commands_fixture()
            if change=='before-init':state=self.state()
            elif change=='after-result':state['plugins']={'result':{'plugins':[]}}
            elif change=='duplicate':case.session_rpc_frame(frame,state,req,sent.append)
            elif change=='callback':frame['id']=99
            elif change=='method':frame['method']='session/update_unknown'
            elif change=='mode':req['mode']=case.CLI_MODE
            elif change=='total-bound':state['notifications']*=3
            elif change=='missing-key':del frame['params']['update']['_meta']
            elif change=='missing-command':frame['params']['update']['availableCommands'].pop()
            elif change=='reordered-command':frame['params']['update']['availableCommands'].reverse()
            elif change=='extra-tool':frame['params']['update']['_meta']['tools'].append('unknown')
            else:frame['params']['update']['_meta']['tools'].reverse()
            before,outbound=copy.deepcopy(state),copy.deepcopy(sent)
            with self.subTest(change=change),self.assertRaises(ValueError):case.session_rpc_frame(frame,state,req,sent.append)
            self.assertEqual(state,before);self.assertEqual(sent,outbound)

    def test_available_commands_session_identity_joins_prior_and_future_notifications_and_response(self):
        for change in ('prior-setup','prior-mcp','prior-response','future-setup','future-mcp','future-response'):
            state,req,sent,frame=self.available_commands_fixture()
            if change=='prior-setup':state['notifications'][-2]['params']['sessionId']='foreign'
            elif change=='prior-mcp':state['notifications'][-1]['params']['sessionId']='foreign'
            elif change=='prior-response':state['session']={'result':{'sessionId':'foreign'}}
            else:
                case.session_rpc_frame(frame,state,req,sent.append)
                if change=='future-response':frame={'jsonrpc':'2.0','id':2,'result':{'sessionId':'foreign'}}
                elif change=='future-mcp':
                    state['notifications']=[x for x in state['notifications'] if x['method']!='_x.ai/mcp_initialized']
                    frame={'jsonrpc':'2.0','method':'_x.ai/mcp_initialized','params':{'sessionId':'foreign','mcpToolCount':0,'elapsedMs':0}}
                else:frame={'jsonrpc':'2.0','method':'_x.ai/session/setup','params':{'method':'session/new','phase':'response_ready','sessionId':'foreign'}}
            before,outbound=copy.deepcopy(state),copy.deepcopy(sent)
            with self.subTest(change=change),self.assertRaisesRegex(ValueError,'session-notification-identity'):
                case.session_rpc_frame(frame,state,req,sent.append)
            self.assertEqual(state,before);self.assertEqual(sent,outbound)

    def test_selected_config_capture_refuses_before_read_and_never_accepts_effects(self):
        for change in ('none','cleanup','sealed','symlink','hardlink','oversize','mode','owner','parent','foreign','snapshot'):
            with self.subTest(change=change),tempfile.TemporaryDirectory() as temporary:
                root=Path(temporary).resolve();metadata,budget,original,ledger,receipt=self.config_capture_fixture(root)
                path=root/case.SESSION_CONFIG
                if change in ('cleanup','sealed'):receipt['cleanup_complete' if change=='cleanup' else 'sealed_cleanup_complete']=False
                elif change=='symlink':path.unlink();path.symlink_to(root/'foreign')
                elif change=='hardlink':os.link(path,root/'alias')
                elif change=='oversize':path.write_bytes(b'x'*(case.SESSION_CONFIG_LIMIT+1));ledger['after'][case.SESSION_CONFIG]=case.stable(path.lstat())
                elif change=='mode':path.chmod(0o644);ledger['after'][case.SESSION_CONFIG]=case.stable(path.lstat())
                elif change=='owner':
                    info=list(ledger['after'][case.SESSION_CONFIG]);info[4]+=1;ledger['after'][case.SESSION_CONFIG]=info
                elif change=='parent':path.parent.rename(root/'moved');path.parent.mkdir()
                elif change=='foreign':(root/'foreign').write_bytes(b'changed');ledger['after']=metadata.metadata_inventory(root,budget)
                elif change=='snapshot':original['sha256']='f'*64
                with mock.patch.object(metadata.os,'read',wraps=os.read) as read:
                    capture=case.capture_session_config(root,ledger,original,'c'*32,budget,receipt)
                if change!='none':self.assertIsNone(capture);read.assert_not_called();continue
                self.assertGreater(read.call_count,0)
                self.assertEqual(base64.b64decode(capture['after']['body_base64']),path.read_bytes())
                self.assertEqual(capture['before']['sha256'],original['sha256'])
                self.assertFalse(ledger['accepted']);self.assertFalse(ledger['protected_originals_unchanged'])
                self.assertNotEqual(capture['before']['identity'][1],capture['after']['identity'][1])
                self.assertEqual(metadata.metadata_inventory(root,budget),ledger['after'])

    def test_selected_config_capture_discards_bytes_when_final_metadata_changes(self):
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary).resolve();metadata,budget,original,ledger,receipt=self.config_capture_fixture(root)
            read=metadata.read_bootstrap_file
            def change_after_read(*args):
                body=read(*args);(root/'foreign').write_bytes(b'changed after read');return body
            with mock.patch.object(metadata,'read_bootstrap_file',side_effect=change_after_read) as captured:
                self.assertIsNone(case.capture_session_config(root,ledger,original,'c'*32,budget,receipt))
            captured.assert_called_once()
            self.assertNotIn('body_base64',json.dumps(ledger))

    def test_selected_config_capture_discards_midstream_rewrite(self):
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary).resolve();metadata,budget,original,ledger,receipt=self.config_capture_fixture(root)
            read,changes=os.read,[]
            def mutate(descriptor,limit):
                body=read(descriptor,limit)
                if not changes:
                    with (root/case.SESSION_CONFIG).open('ab') as stream:stream.write(b'!')
                    changes.append(True)
                return body
            with mock.patch.object(metadata.os,'read',side_effect=mutate):
                self.assertIsNone(case.capture_session_config(root,ledger,original,'c'*32,budget,receipt))
            self.assertEqual(changes,[True])

    def test_fixed_provider_accepts_only_bounded_gets_and_never_exports_headers(self):
        import socket
        for request, accepted in ((b'GET /v1/models HTTP/1.1',True),(b'GET / HTTP/1.1',True),
                                  (b'POST /v1/chat/completions HTTP/1.1',False),(b'GET /auth HTTP/1.1',False)):
            provider=case.SessionProvider(0,time.monotonic()+3);port=provider.port
            try:
                with socket.create_connection(('127.0.0.1',port),timeout=2) as client:
                    client.sendall(request+b'\r\nHost: 127.0.0.1:'+str(port).encode()+b'\r\nAuthorization: Bearer PRIVATE\r\n\r\n')
                    while client.recv(4096):pass
            finally:provider.close()
            self.assertEqual(provider.facts['requests'],1)
            self.assertEqual(provider.facts['forbidden'],int(not accepted))
            self.assertTrue(provider.facts['settled'])
            self.assertNotIn('PRIVATE',json.dumps(provider.facts))

    def test_provider_body_chunking_and_duplicate_headers_are_refused(self):
        import socket
        for headers in (b'Content-Length: 1\r\n',b'Transfer-Encoding: chunked\r\n',b'Host: duplicate\r\n'):
            provider=case.SessionProvider(0,time.monotonic()+3);port=provider.port
            try:
                with socket.create_connection(('127.0.0.1',port),timeout=2) as client:
                    client.sendall(b'GET /v1/models HTTP/1.1\r\nHost: 127.0.0.1:'+str(port).encode()+b'\r\n'+headers+b'\r\n')
                    while client.recv(4096):pass
            finally:provider.close()
            self.assertEqual(provider.facts['forbidden'],1)

    def test_privileged_constant_has_inert_root_namespace_gates_before_ioctl(self):
        code=self.bootstrap()
        prefix="import fcntl,os; fcntl.ioctl=lambda *a: (_ for _ in ()).throw(AssertionError('ioctl-before-gate')); "
        parent=os.stat('/proc/self/ns/net').st_ino
        roots='os.getuid=os.geteuid=os.getgid=os.getegid=lambda:0; '
        for extra in (roots, roots+'os.geteuid=lambda:1; '):
            result=subprocess.run([sys.executable,'-I','-c',prefix+extra+code,str(parent),'/usr/bin/setpriv','never-execute'],
                                  stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=3)
            self.assertNotEqual(result.returncode,0)
            self.assertEqual(result.stdout,b'')
            self.assertIn(b'loopback-bootstrap-refused',result.stderr)
            self.assertNotIn(b'ioctl-before-gate',result.stderr)

    def test_provider_absolute_deadline_refuses_slow_drip_and_late_send(self):
        from types import SimpleNamespace
        for late_send in (False, True):
            clock=[0.0]
            provider=object.__new__(case.SessionProvider)
            provider.deadline=20;provider.port=23456
            provider.stop=SimpleNamespace(is_set=lambda:False)
            provider.facts=dict(requests=0,models_get=0,root_get=0,forbidden=0,settled=False)
            connection=mock.MagicMock();connection.__enter__.return_value=connection
            def receive(_):
                clock[0]+=3
                return (b'GET / HTTP/1.1\r\nHost: 127.0.0.1:23456\r\n\r\n'
                        if late_send else b'x')
            connection.recv.side_effect=receive
            if late_send:
                connection.recv.side_effect=lambda _: (clock.__setitem__(0,6) or
                    b'GET / HTTP/1.1\r\nHost: 127.0.0.1:23456\r\n\r\n')
            provider.listener=mock.Mock()
            provider.listener.accept.side_effect=[(connection,('127.0.0.1',1)),OSError()]
            with mock.patch.object(case.time,'monotonic',side_effect=lambda:clock[0]):
                provider.serve()
            connection.sendall.assert_not_called()
            self.assertLessEqual(connection.recv.call_count,2)
            self.assertGreaterEqual(provider.facts['forbidden'],1)

    def bootstrap(self):
        path=Path(__file__).resolve().parents[1]/'install/integrationctl/research/grokacpinventory/loopback_linux.go'
        import re
        return re.search(r'const loopbackBootstrap = `([^`]+)`',path.read_text()).group(1)

    def test_actual_constant_modifies_only_lo_up_and_closes_before_drop(self):
        import struct
        from types import SimpleNamespace
        events=[]
        channel=mock.MagicMock();channel.__enter__.return_value=channel;channel.fileno.return_value=7
        channel.__exit__.side_effect=lambda *args:events.append('close')
        values=[8,9]
        def ioctl(fd, operation, payload):
            self.assertEqual((fd,len(payload),payload[:16]),(7,40,b'lo'+b'\0'*14))
            if operation==0x8913:return struct.pack('16sH22x',b'lo',values.pop(0))
            self.assertEqual(operation,0x8914);self.assertEqual(struct.unpack_from('H',payload,16)[0],9);events.append('lo-up')
        fake_os=SimpleNamespace(getuid=lambda:0,geteuid=lambda:0,getgid=lambda:0,getegid=lambda:0,
            uname=lambda:SimpleNamespace(machine='x86_64'),stat=lambda _:SimpleNamespace(st_ino=2),
            execve=lambda exe,argv,env:events.append((exe,argv,env)))
        fake_socket=SimpleNamespace(AF_INET=2,SOCK_DGRAM=2,socket=lambda *args:channel,if_nameindex=lambda:[(1,'lo')])
        fake_sys=SimpleNamespace(platform='linux',byteorder='little',argv=['-c','1','/usr/bin/setpriv','--no-new-privs'])
        with mock.patch.dict(sys.modules,{'os':fake_os,'socket':fake_socket,'fcntl':SimpleNamespace(ioctl=ioctl),'sys':fake_sys}):
            exec(compile(self.bootstrap(),'fixed-bootstrap','exec'),{})
        self.assertEqual(events[:2],['lo-up','close'])
        self.assertEqual(events[-1],('/usr/bin/setpriv',['/usr/bin/setpriv','--no-new-privs'],{'PATH':'/usr/bin:/bin','LANG':'C.UTF-8'}))

    def test_session_config_precedes_only_baseline_and_measurement_never_reads_unknown_bodies(self):
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary).resolve();(root/'home/.grok').mkdir(parents=True)
            case.session_fixture_config(root,'c'*32)
            body=(root/'home/.grok/config.toml').read_bytes()
            self.assertIn(('127.0.0.1:'+str(case.session_port('c'*32))).encode(),body)
            self.assertNotIn(b'trusted',body)
            import devin_structured_inventory_admission as metadata
            budget=type('Budget',(),{'check':lambda self:None})()
            before=metadata.metadata_inventory(root,budget);(root/'unknown-session').write_bytes(b'PRIVATE')
            with mock.patch.object(case,'ordinary',side_effect=AssertionError('unknown body read')):
                ledger=case.session_metadata_ledger(root,before,budget)
            self.assertFalse(ledger['accepted']);self.assertTrue(ledger['protected_originals_unchanged'])
            self.assertNotIn('PRIVATE',json.dumps(ledger))

    def test_session_archive_readout_joins_actual_wire_and_refuses_detached_claims(self):
        import io
        import zipfile
        packet=dict(source_commit='a'*40,source_tree='b'*40,nonce='c'*32)
        frames=[{'jsonrpc':'2.0','id':1,'result':{'protocolVersion':1,'_meta':{'agentVersion':'1.0.70'}}},
                {'jsonrpc':'2.0','method':'_x.ai/session/setup','params':{'method':'session/new','phase':'agent_build','sessionId':'PRIVATE_NATIVE_ID'}},
                {'jsonrpc':'2.0','method':'_x.ai/mcp_initialized','params':{'sessionId':'PRIVATE_NATIVE_ID','mcpToolCount':0,'elapsedMs':0}},
                {'jsonrpc':'2.0','id':2,'result':{'sessionId':'PRIVATE_NATIVE_ID'}},
                {'jsonrpc':'2.0','id':3,'result':{'result':{'plugins':[]}}}]
        rpc=dict(initialize=frames[0],session=frames[3],plugins=frames[4],notifications=frames[1:3])
        members={'acquisition.json':b'{}','a.rpc.json':json.dumps(rpc).encode(),
                 'a.ledger.json':json.dumps(dict(before={},after={},accepted=False)).encode()}
        commands=[]
        for index,label in enumerate(case.LABELS):
            body=(b''.join(json.dumps(frame).encode()+b'\n' for frame in frames) if index==4 else b'help')
            name='a-'+label+'.merged.log';members[name]=body
            commands.append(dict(label=label,merged_file=name,merged_bytes=len(body),merged_sha256=case.digest(body),exit=0,error=None))
        result=dict(packet,schema_version=1,scope='a',mode=case.SESSION_MODE,semantic_complete=True,reason='complete',
                    provider=dict(requests=0,models_get=0,root_get=0,forbidden=0,settled=True),commands=commands)
        receipt=dict(packet,schema_version=1,scope='a',cleanup_complete=True,sealed_cleanup_complete=True,result=result)
        summary=dict(packet,schema_version=1,mode=case.SESSION_MODE,collection_complete=False,cleanup_complete=True,
                     qualification=copy.deepcopy(case.FALSE),scopes=[receipt])
        def read(changed):
            changed['admission.json']=json.dumps(summary).encode()
            output=io.BytesIO()
            with zipfile.ZipFile(output,'w') as archive:
                for name,body in changed.items():archive.writestr(name,body)
            return case.publication_readout(output.getvalue(),packet,time.monotonic()+2,mode=case.SESSION_MODE)
        good=read(dict(members))
        self.assertTrue(good['session_reply']);self.assertTrue(good['inventory_reply'])
        self.assertNotIn('PRIVATE_NATIVE_ID',json.dumps(good))
        for kind in ('scope','schema','reason','empty','missing','forged','truncated','no-receipt','no-lf','late-notification','missing-notification'):
            original=copy.deepcopy(result);changed=dict(members)
            if kind=='no-receipt':summary['scopes']=[]
            elif kind=='scope':result['scope']='b'
            elif kind=='schema':result['schema_version']=True
            elif kind=='reason':result['reason']='help-refused'
            elif kind=='empty':result['commands']=[]
            elif kind=='missing':del changed['a-acp.merged.log']
            elif kind=='forged':
                detached=copy.deepcopy(rpc);detached['session']['result']['sessionId']='OTHER'
                changed['a.rpc.json']=json.dumps(detached).encode()
            elif kind=='missing-notification':
                detached=copy.deepcopy(rpc);detached['notifications'].pop()
                changed['a.rpc.json']=json.dumps(detached).encode()
            elif kind in ('no-lf','late-notification'):
                body=changed['a-acp.merged.log'][:-1] if kind=='no-lf' else changed['a-acp.merged.log']+json.dumps(
                    {'jsonrpc':'2.0','method':'_x.ai/session/setup','params':{'method':'session/new','phase':'agent_build','sessionId':'PRIVATE_NATIVE_ID'}}).encode()+b'\n'
                changed['a-acp.merged.log']=body
                result['commands'][-1]['merged_bytes']=len(body);result['commands'][-1]['merged_sha256']=case.digest(body)
            else:
                body=changed['a-acp.merged.log'].splitlines(keepends=True)[0]
                changed['a-acp.merged.log']=body
                result['commands'][-1]['merged_bytes']=len(body);result['commands'][-1]['merged_sha256']=case.digest(body)
            with self.subTest(kind=kind):
                self.assertEqual(read(changed),dict(schema_version=1,mode=case.SESSION_MODE,reason='readout-refused'))
            result.clear();result.update(original)
            summary['scopes']=[receipt]
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary).resolve();_,budget,original,ledger,settled=self.config_capture_fixture(root)
            ledger['selected_config_capture']=case.capture_session_config(root,ledger,original,'c'*32,budget,settled)
            for change in ('none','null','schema','extra','hash','before-hash','identity','bound','parent','record-type'):
                candidate=copy.deepcopy(ledger);capture=candidate['selected_config_capture']
                if change=='null':candidate['selected_config_capture']=None
                elif change=='schema':capture['schema_version']=True
                elif change=='extra':capture['unknown']='private'
                elif change in ('hash','before-hash'):capture['before' if change=='before-hash' else 'after']['sha256']='f'*64
                elif change=='identity':capture['after']['identity'][1]+=1
                elif change=='bound':capture['after']['body_base64']='x'*(4*((case.SESSION_CONFIG_LIMIT+2)//3)+4)
                elif change=='parent':candidate['before']['home']=candidate['after']['home']='invalid'
                elif change=='record-type':candidate['selected_config_capture']=False
                changed=dict(members);changed['a.ledger.json']=json.dumps(candidate).encode()
                with self.subTest(capture=change):
                    facts=read(changed)
                    self.assertEqual(facts['reason'],'measurement-only' if change in ('none','null') else 'readout-refused')
                    self.assertNotIn('body_base64',json.dumps(facts))

    def test_session_readout_refuses_unknown_or_private_values(self):
        packet=dict(source_commit='a'*40,source_tree='b'*40,nonce='c'*32)
        for value in ({}, {'mode':case.SESSION_MODE,'qualification':{'private':'COOKIE'}}):
            members={'admission.json':json.dumps(value).encode()}
            facts=case.session_readout(members,packet,time.monotonic()+1)
            self.assertEqual(facts,{'schema_version':1,'mode':case.SESSION_MODE,'reason':'readout-refused'})
            self.assertNotIn('COOKIE',json.dumps(facts))



class LockQualificationContracts(unittest.TestCase):
    def fixture(self, root):
        (root/'home/.grok').mkdir(parents=True)
        for name in ('a','b'): (root/name).mkdir()
        (root/'request.json').write_text('{}')
        return case.lock_fixture_config(root)

    def rows(self):
        values=(b'grok 1.0.70 (abc1234)\n',b'Usage: grok mcp <COMMAND>\nCommands:\n  disable  Disable a server\n',
                b'Usage: grok mcp disable <NAME>\n')
        return [dict(label=label,argv=[],exit=0,error=None,merged_base64=base64.b64encode(body).decode(),
                     merged_bytes=len(body),merged_sha256=case.digest(body))
                for label,body in zip(case.LOCK_LABELS,values)]

    @contextmanager
    def bootstrap(self, root):
        # Inert content pins preserve the real 27-name admission set. No vendor
        # executable is substituted or run by this metadata/body guard test.
        pins = {}
        for name in ('docs', 'docs/user-guide', 'logs'):
            path = root/'home/.grok'/name; path.mkdir(); path.chmod(0o755)
        for name in bootstrap_case.PINNED:
            if not name.startswith('docs/user-guide/'):
                continue
            body = ('inert pinned ' + name).encode()
            path = root/'home/.grok'/name; path.write_bytes(body); path.chmod(0o644)
            pins[name] = (len(body), case.digest(body), 0o644)
        path = root/'home/.grok/logs/unified.jsonl'; path.write_bytes(b'inert\n'); path.chmod(0o644)
        with mock.patch.dict(bootstrap_case.PINNED, pins):
            yield

    def test_measured_startup_is_task_born_bounded_and_frozen_across_phases(self):
        import devin_structured_inventory_admission as metadata
        budget=type('Budget',(),{'check':lambda self:None})()
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary);authored=self.fixture(root)
            before=metadata.metadata_inventory(root,budget);saved=copy.deepcopy(before)
            with self.bootstrap(root):
                locked=case.lock_ledger(root,before,case.LOCK_PHASES[0],authored,budget)
                self.assertTrue(locked['accepted'])
                self.assertEqual(len(locked['after'].keys()-before.keys()),31)
                created={name:value for name,value in locked['after'].items() if name not in before}
                config=root/'home/.grok/config.toml'
                config.write_text(config.read_text().replace('disabled_mcp_servers = []',
                    'disabled_mcp_servers = ["'+case.LOCK_NAME+'"]').replace('enabled = true','enabled = false'))
                with (root/'home/.grok/logs/unified.jsonl').open('ab') as stream:stream.write(b'inert second\n')
                released=case.lock_ledger(root,before,case.LOCK_PHASES[1],authored,budget,created=created)
                self.assertTrue(released['accepted'])
                self.assertEqual(released['bootstrap_digests'],locked['bootstrap_digests'])
                self.assertEqual(before,saved)

    def test_actual_flock_proves_supervisor_state_without_releasing_its_lock(self):
        import fcntl
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary);self.fixture(root)
            lock=root/'home/.grok/.config-init.lock'
            with lock.open('rb') as supervisor:
                fcntl.flock(supervisor.fileno(),fcntl.LOCK_EX|fcntl.LOCK_NB)
                self.assertIs(case.lock_file(root,True),True)
                with self.assertRaises(ValueError):case.lock_file(root,False)
                with lock.open('rb') as second:
                    with self.assertRaises(BlockingIOError):fcntl.flock(second.fileno(),fcntl.LOCK_EX|fcntl.LOCK_NB)
            self.assertIs(case.lock_file(root,False),False)
            with self.assertRaises(ValueError):case.lock_file(root,True)
            self.assertIs(case.lock_file(root,False),False)
            os.link(lock,root/'alias')
            with self.assertRaises(ValueError):case.lock_file(root,True)

    def test_flock_other_errors_never_prove_supervisor_ownership(self):
        import errno
        import fcntl
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary);self.fixture(root)
            for error in (OSError(errno.EACCES,'permission'),BlockingIOError(errno.EPERM,'wrong refusal')):
                with mock.patch.object(fcntl,'flock',side_effect=error),self.assertRaises((OSError,ValueError)):
                    case.lock_file(root,True)
            self.assertIs(case.lock_file(root,False),False)

    def test_actual_help_gate_rejects_missing_advertisement_and_wrong_usage(self):
        case.lock_help(self.rows())
        for index,body in ((1,b'Usage: grok mcp <COMMAND>\n  list List\n'),
                           (2,b'Usage: grok mcp enable <NAME>\n'),(2,b'Usage: grok mcp disable\n')):
            rows=self.rows();rows[index]['merged_base64']=base64.b64encode(body).decode()
            with self.assertRaises(ValueError):case.lock_help(rows)

    def test_collector_writes_only_after_help_and_preserves_fatal_capture_fence(self):
        req=dict(schema_version=1,source_commit='a'*40,source_tree='b'*40,nonce='c'*32,scope='a',
                 input_root='/tmp/owned',phase=case.LOCK_PHASES[0],start_monotonic_ns=time.monotonic_ns())
        for failure in ('none','help','capture'):
            labels=[];rows=self.rows()
            def command(argv,label,*args):
                labels.append(label)
                if label=='writer':
                    if failure=='capture':raise ValueError('incomplete drain')
                    return dict(label=label,argv=argv,exit=1,error=None,merged_base64='',merged_bytes=0,merged_sha256=case.digest(b''))
                row=dict(rows[len(labels)-1])
                if failure=='help' and label=='disable-help':row['exit']=1
                return row
            with mock.patch.object(case,'isolation',return_value={}),mock.patch.object(case,'lock_file',return_value=True) as lock, mock.patch.object(case,'command',side_effect=command):
                if failure=='capture':
                    with self.assertRaises(ValueError):case.lock_collector_result(req)
                else:
                    result=case.lock_collector_result(req)
                    self.assertEqual(result['semantic_complete'],failure=='none')
                if failure=='help':
                    lock.assert_not_called();self.assertNotIn('writer',labels)
                else:lock.assert_called_once_with(Path('/tmp/owned'),True)

    def test_authoritative_ledger_requires_both_witness_changes_and_immutable_baseline(self):
        import devin_structured_inventory_admission as metadata
        budget=type('Budget',(),{'check':lambda self:None})()
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary);authored=self.fixture(root)
            before=metadata.metadata_inventory(root,budget);saved=copy.deepcopy(before)
            with self.bootstrap(root):
                first=case.lock_ledger(root,before,case.LOCK_PHASES[0],authored,budget)
                self.assertTrue(first['accepted'])
                created={name:value for name,value in first['after'].items() if name not in before}
                path=root/'home/.grok/config.toml';original=path.read_text()
                path.write_text(original.replace('disabled_mcp_servers = []','disabled_mcp_servers = ["'+case.LOCK_NAME+'"]'))
                self.assertFalse(case.lock_ledger(root,before,case.LOCK_PHASES[1],authored,budget,created=created)['accepted'])
                path.write_text(path.read_text().replace('enabled = true','enabled = false'))
                self.assertTrue(case.lock_ledger(root,before,case.LOCK_PHASES[1],authored,budget,created=created)['accepted'])
                self.assertEqual(before,saved)
                (root/'b/foreign').write_text('PRIVATE')
                with mock.patch.object(case,'lock_config_bytes',side_effect=AssertionError('foreign bytes read')) as read:
                    self.assertFalse(case.lock_ledger(root,before,case.LOCK_PHASES[1],authored,budget,created=created)['accepted'])
                read.assert_not_called()

    def test_startup_membership_authority_and_existing_member_refuse_before_bytes(self):
        import devin_structured_inventory_admission as metadata
        budget=type('Budget',(),{'check':lambda self:None})()
        for fault in ('unknown','missing','mode','hardlink','symlink','oversize-log','existing-log'):
            with self.subTest(fault=fault),tempfile.TemporaryDirectory() as temporary:
                root=Path(temporary);authored=self.fixture(root)
                before=metadata.metadata_inventory(root,budget)
                with self.bootstrap(root):
                    doc=root/'home/.grok/docs/user-guide/01-getting-started.md'
                    if fault=='unknown':(root/'home/.grok/private').touch()
                    elif fault=='missing':doc.unlink()
                    elif fault=='mode':doc.chmod(0o600)
                    elif fault=='hardlink':os.link(doc,root.parent/(root.name+'-alias'))
                    elif fault=='symlink':doc.unlink();doc.symlink_to(root/'witness.py')
                    elif fault=='oversize-log':(root/'home/.grok/logs/unified.jsonl').write_bytes(b'x'*((128<<10)+1))
                    else:before['home/.grok/logs/unified.jsonl']=metadata.metadata_inventory(root,budget)['home/.grok/logs/unified.jsonl']
                    try:
                        with mock.patch.object(case,'lock_config_bytes',side_effect=AssertionError('unadmitted bytes read')) as read:
                            if fault in ('hardlink','symlink'):
                                with self.assertRaises(ValueError):
                                    case.lock_ledger(root,before,case.LOCK_PHASES[0],authored,budget)
                            else:
                                self.assertFalse(case.lock_ledger(root,before,case.LOCK_PHASES[0],authored,budget)['accepted'])
                        read.assert_not_called()
                    finally:
                        if fault=='hardlink':(root.parent/(root.name+'-alias')).unlink()

    def test_pinned_doc_content_and_second_phase_creation_identity_are_not_renewed(self):
        import devin_structured_inventory_admission as metadata
        budget=type('Budget',(),{'check':lambda self:None})()
        for fault in ('wrong-body','later-doc','rotated-log','owner','directory'):
            with self.subTest(fault=fault),tempfile.TemporaryDirectory() as temporary:
                root=Path(temporary);authored=self.fixture(root)
                before=metadata.metadata_inventory(root,budget)
                with self.bootstrap(root):
                    first=case.lock_ledger(root,before,case.LOCK_PHASES[0],authored,budget)
                    self.assertTrue(first['accepted'])
                    created={name:value for name,value in first['after'].items() if name not in before}
                    saved=copy.deepcopy(created)
                    if fault in ('owner','directory'):
                        after=copy.deepcopy(first['after']);name='home/.grok/docs' if fault=='directory' else 'home/.grok/docs/user-guide/01-getting-started.md'
                        meta=list(after[name]);meta[1 if fault=='directory' else 4]+=1;after[name]=meta
                        with self.assertRaises(ValueError):case.lock_bootstrap_metadata(before,after,created)
                        continue
                    path=root/('home/.grok/logs/unified.jsonl' if fault=='rotated-log' else 'home/.grok/docs/user-guide/01-getting-started.md')
                    if fault=='wrong-body':path.write_bytes(b'X'*path.stat().st_size)
                    else:
                        replacement=path.with_suffix('.replacement');replacement.write_bytes(path.read_bytes());replacement.chmod(0o644);replacement.replace(path)
                    if fault=='wrong-body':
                        self.assertFalse(case.lock_ledger(root,before,case.LOCK_PHASES[0],authored,budget)['accepted'])
                    else:
                        with mock.patch.object(case,'lock_config_bytes',side_effect=AssertionError('replacement bytes read')) as read:
                            self.assertFalse(case.lock_ledger(root,before,case.LOCK_PHASES[1],authored,budget,created=created)['accepted'])
                        read.assert_not_called()
                    self.assertEqual(created,saved)

    def test_replaced_original_root_refuses_before_content_read(self):
        import devin_structured_inventory_admission as metadata
        budget=type('Budget',(),{'check':lambda self:None})()
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary)/'owned';root.mkdir();self.fixture(root)
            before=metadata.metadata_inventory(root,budget)
            root.rename(root.with_name('retained'))
            root.mkdir();self.fixture(root)
            after=metadata.metadata_inventory(root,budget)
            with mock.patch.object(case.os,'read',side_effect=AssertionError('replacement bytes read')) as read,self.assertRaises(ValueError):
                case.lock_config_bytes(root,before,after)
            read.assert_not_called()


class LockReadoutContracts(unittest.TestCase):
    def fixture(self, phases=case.LOCK_PHASES):
        packet=dict(source_commit='a'*40,source_tree='b'*40,nonce='c'*32)
        members,receipts={'acquisition.json':b'{}'},[]
        tails=(['--version'],['mcp','--help'],['mcp','disable','--help'],['mcp','disable',case.LOCK_NAME])
        for index,phase in enumerate(phases):
            rows=LockQualificationContracts.rows(self)
            rows.append(dict(label='writer',argv=[],exit=1 if index==0 else 0,error=None,
                             merged_base64=base64.b64encode(b'PRIVATE_NATIVE').decode(),
                             merged_bytes=14,merged_sha256=case.digest(b'PRIVATE_NATIVE')))
            for number,row in enumerate(rows):
                body=base64.b64decode(row.pop('merged_base64'));row['merged_bytes']=len(body)
                row['argv']=['/tmp/uap-grok-acp-sealed-'+str(index+123)+'/grok',*tails[number]]
                name=phase+'-'+case.LOCK_LABELS[number]+'.merged.log';row['merged_file']=name;members[name]=body
            result=dict(packet,schema_version=1,scope='a',mode=case.LOCK_MODE,phase=phase,lock_held=index==0,
                        semantic_complete=True,reason='complete',isolation={},commands=rows)
            systems={name:dict(path='/usr/bin/'+('python3.12' if name=='python3' else name),sha256='d'*64)
                     for name in ('unshare','setpriv','env','python3')}
            receipts.append(dict(packet,schema_version=1,scope='a',cleanup_complete=True,sealed_cleanup_complete=True,
                                 system_executables=systems,result=result))
            members[phase+'.rpc.json']=json.dumps(case.rpc_state()).encode()
            members[phase+'.ledger.json']=json.dumps(dict(phase=phase,accepted=True,config_observed=True,
                config_bytes=12,config_sha256='e'*64,before={'PRIVATE/PATH':[1]},after={'PRIVATE/PATH':[2]},
                bootstrap_digests={bootstrap_case.PREFIX+name:pin[1] for name,pin in bootstrap_case.PINNED.items()
                                   if name.startswith('docs/user-guide/')})).encode()
        summary=dict(packet,schema_version=1,mode=case.LOCK_MODE,collection_complete=False,cleanup_complete=True,
                     qualification=dict(case.FALSE),scopes=receipts,measurement_complete=len(phases)==2)
        members['admission.json']=json.dumps(summary).encode()
        return packet,members

    def read(self, packet, members):
        return case.publication_readout(AdmissionContracts.readout_zip(self,members),packet,time.monotonic()+5,mode=case.LOCK_MODE)

    def test_actual_archive_reports_only_existing_phases_and_closed_facts(self):
        for phases in (case.LOCK_PHASES[:1],case.LOCK_PHASES):
            packet,members=self.fixture(phases);facts=self.read(packet,members)
            self.assertEqual([row['phase'] for row in facts['phases']],list(phases))
            self.assertTrue(all(row['semantic_complete'] and row['ledger_accepted'] for row in facts['phases']))
            for private in ('PRIVATE','argv','sha256','merged_file',case.LOCK_NAME):
                self.assertNotIn(private,json.dumps(facts))

    def test_lock_exit_preserves_measurement_qualification_and_typed_refusals(self):
        packet, members = self.fixture()
        summary = case.document(members['admission.json'])
        before = copy.deepcopy(summary)
        self.assertEqual(self.read(packet, members)['reason'], 'measurement-only')
        self.assertEqual(case.observation_exit(summary, lock_qualification=True), 0)
        self.assertEqual(summary, before)
        self.assertIs(summary['collection_complete'], False)
        self.assertTrue(all(value is False for value in summary['qualification'].values()))
        for key, value in (('measurement_complete', False), ('measurement_complete', 1),
                           ('cleanup_complete', False), ('cleanup_complete', 1),
                           ('error', 'admission-refused'), ('error', False), ('error', ''),
                           ('mode', case.SESSION_MODE), ('mode', None)):
            changed = dict(summary, **{key: value})
            with self.subTest(key=key, value=value):
                self.assertEqual(case.observation_exit(changed, lock_qualification=True), 1)
        for selected in (False, 1):
            self.assertEqual(case.observation_exit(summary, lock_qualification=selected), 1)
        _, partial = self.fixture(case.LOCK_PHASES[:1])
        self.assertEqual(case.observation_exit(case.document(partial['admission.json']), lock_qualification=True), 1)
        self.assertEqual(case.observation_exit(dict(collection_complete=True)), 0)
        self.assertEqual(case.observation_exit(dict(collection_complete=False)), 1)

    def test_real_run_never_completes_after_second_phase_source_or_cleanup_refusal(self):
        import io
        import project_native_research as shared
        import native_client_project_artifacts as artifacts
        from project_native_collectors.grok import ACQUISITION
        commit, tree, native = 'a' * 40, 'b' * 40, b'inert transport fixture; never executed'
        write_zip, remove_root = artifacts.write_zip, case.shutil.rmtree
        for refusal in ('none', 'second-phase', 'source-final', 'cleanup', 'helper-missing', 'cleanup-io', 'archive'):
            with self.subTest(refusal=refusal), tempfile.TemporaryDirectory() as temporary:
                packet, members = self.fixture()
                receipts = case.document(members['admission.json'])['scopes']
                calls, owned = [], []
                def process(argv, **_options):
                    if argv[0] == 'go':
                        helper = Path(argv[argv.index('-o') + 1])
                        owned.append((helper.parent, case.stable(helper.parent.lstat())[:2]))
                        helper.write_bytes(b'inert helper; never executed')
                        return SimpleNamespace(stdout=b'', stderr=b'')
                    self.assertEqual(argv[0], '/usr/bin/git')
                    if argv[1:] == ['rev-parse', 'HEAD']:
                        output = commit.encode()
                    elif argv[1:] == ['rev-parse', 'HEAD^{tree}']:
                        output = tree.encode()
                    elif argv[1:2] == ['status']:
                        calls.append('source')
                        output = b'M changed' if refusal == 'source-final' and calls.count('source') == 2 else b''
                    else:
                        self.assertIn('init', argv)
                        output = b''
                    return SimpleNamespace(stdout=output, stderr=b'')
                def capture(_helper, root, _deadline):
                    request = case.document((root / 'request.json').read_bytes())
                    index = case.LOCK_PHASES.index(request['phase'])
                    calls.append(request['phase'])
                    if refusal == 'helper-missing':
                        raise ValueError('PRIVATE helper stderr/path')
                    receipt = copy.deepcopy(receipts[index])
                    for value in (receipt, receipt['result']):
                        for key in ('source_commit', 'source_tree', 'nonce'):
                            value[key] = request[key]
                    result = receipt['result']
                    for row in result['commands']:
                        row['merged_base64'] = base64.b64encode(members[row.pop('merged_file')]).decode()
                    result['rpc'] = case.rpc_state()
                    if index == 1 and refusal == 'second-phase':
                        result.update(semantic_complete=False, reason='protocol-refused')
                    if index == 1 and refusal == 'cleanup':
                        receipt['cleanup_complete'] = False
                    return receipt
                def archive(*args, **kwargs):
                    if refusal == 'archive':
                        raise OSError('PRIVATE archive path')
                    return write_zip(*args, **kwargs)
                def cleanup(*args, **kwargs):
                    if refusal == 'cleanup-io':
                        raise OSError('PRIVATE cleanup path')
                    return remove_root(*args, **kwargs)
                try:
                    with (mock.patch.object(shared.native, 'require_hosted'),
                          mock.patch.object(shared, 'acquire', return_value=({}, {'grok': native})),
                          mock.patch.dict(ACQUISITION, binary_sha256=case.digest(native)),
                          mock.patch.dict(os.environ, EXPECTED_COMMIT=commit),
                          mock.patch.object(case.os, 'getuid', return_value=1001),
                          mock.patch.object(case.subprocess, 'run', side_effect=process),
                          mock.patch.object(case, 'helper_capture', side_effect=capture),
                          mock.patch.object(case, 'lock_ledger', return_value=dict(accepted=True, before={}, after={})),
                          mock.patch.object(artifacts, 'write_zip', side_effect=archive),
                          mock.patch.object(case.shutil, 'rmtree', side_effect=cleanup),
                          mock.patch.object(sys, 'stdout', io.StringIO()) as stream):
                        status = case.frontend(Path(temporary) / 'output', lock_qualification=True)
                    stdout = stream.getvalue().encode()
                    self.assertNotIn(b'PRIVATE', stdout)
                    packet = case.document(stdout)
                    path = Path(temporary) / 'output/upload/research.zip'
                    if refusal in ('cleanup-io', 'archive'):
                        self.assertFalse(path.exists())
                    else:
                        with zipfile.ZipFile(path) as published:
                            summary = case.document(published.read('admission.json'))
                        self.assertEqual(summary.get('measurement_complete') is True, refusal == 'none')
                        self.assertIs(summary['collection_complete'], False)
                        self.assertTrue(all(value is False for value in summary['qualification'].values()))
                    self.assertEqual(status, 0 if refusal == 'none' else 1)
                    if refusal in ('cleanup', 'helper-missing', 'cleanup-io', 'archive'):
                        self.assertEqual(packet, dict(schema_version=1, kind='frontend-publication-refused',
                            stage={'cleanup': 'receipt', 'helper-missing': 'helper', 'cleanup-io': 'cleanup', 'archive': 'archive'}[refusal],
                            error_class='io' if refusal in ('cleanup-io', 'archive') else 'contract', source_identity_checked=True,
                            cleanup_complete=refusal == 'archive', publication_attempted=refusal != 'cleanup-io'))
                        with redirect_stdout(io.StringIO()) as diagnostics, self.assertRaisesRegex(ValueError, '^frontend-publication-refused$'):
                            case.frontend_packet(stdout, status)
                        self.assertEqual(diagnostics.getvalue(), 'GROK_FRONTEND_REFUSAL=' + stdout.decode())
                    else:
                        self.assertEqual(stdout, json.dumps(dict(schema_version=1, source_commit=commit, source_tree=tree,
                            nonce=packet['nonce'], publication=packet['publication']), separators=(',', ':')).encode() + b'\n')
                        with redirect_stdout(io.StringIO()) as diagnostics:
                            self.assertEqual(case.frontend_packet(stdout, status), packet)
                        self.assertEqual(diagnostics.getvalue(), '')
                    self.assertEqual([value for value in calls if value in case.LOCK_PHASES], list(case.LOCK_PHASES[:1] if refusal == 'helper-missing' else case.LOCK_PHASES))
                    self.assertEqual(calls.count('source'), 2 if refusal in ('none', 'source-final', 'cleanup-io', 'archive') else 1)
                finally:
                    for root, identity in owned:
                        if root.exists():
                            self.assertEqual(case.stable(root.lstat())[:2], identity)
                            case.shutil.rmtree(root)

    def test_source_cleanup_bool_missing_stream_rpc_and_unjoined_members_refuse(self):
        for fault in ('source','cleanup','bool','stream','rpc','extra','no-phase','phase','writer','bootstrap'):
            packet,members=self.fixture();summary=json.loads(members['admission.json'])
            if fault=='source':summary['source_tree']='f'*40
            elif fault=='cleanup':summary['scopes'][0]['cleanup_complete']=False
            elif fault=='bool':summary['scopes'][0]['result']['lock_held']=1
            elif fault=='stream':del members['locked-writer-version.merged.log']
            elif fault=='rpc':members['locked-writer.rpc.json']=b'{"initialize":{},"plugins":null,"notifications":[]}'
            elif fault=='extra':members['a-version.merged.log']=b'PRIVATE'
            elif fault=='no-phase':summary['scopes']=[]
            elif fault=='phase':summary['scopes'][0]['result']['phase']='PRIVATE'
            elif fault=='writer':summary['scopes'][0]['result']['commands'][-1]['exit']=0
            else:
                ledger=json.loads(members['locked-writer.ledger.json']);ledger['bootstrap_digests']={}
                members['locked-writer.ledger.json']=json.dumps(ledger).encode()
            members['admission.json']=json.dumps(summary).encode()
            with self.subTest(fault=fault):
                self.assertEqual(self.read(packet,members),dict(schema_version=1,mode=case.LOCK_MODE,reason='readout-refused'))

    def test_real_publication_keeps_original_on_late_diagnostic_refusal(self):
        packet,members=self.fixture()
        with tempfile.TemporaryDirectory() as temporary:
            output=Path(temporary)/'output';output.mkdir()
            proof=case.publish(output,members,mode=case.LOCK_MODE)
            packet.update(schema_version=1,publication=proof)
            diagnostics=[]
            with mock.patch.object(case.time,'monotonic',return_value=101.0):
                admitted=case.admit_publication(output,packet,packet['source_commit'],packet['source_tree'],
                    Path(temporary)/'admitted',diagnostics,100.0,mode=case.LOCK_MODE)
            broken=copy.deepcopy(packet);broken['publication']['output']['inode']+=1
            untouched=[]
            with mock.patch.object(case,'publication_readout',side_effect=AssertionError('diagnostic before authority')),self.assertRaises(ValueError):
                case.admit_publication(output,broken,packet['source_commit'],packet['source_tree'],
                    Path(temporary)/'must-not-stage',untouched,100.0,mode=case.LOCK_MODE)
            self.assertEqual(untouched,[])
            self.assertEqual(case.digest(admitted.read_bytes()),proof['sha256'])
            self.assertEqual(diagnostics,[dict(schema_version=1,mode=case.LOCK_MODE,reason='readout-refused')])

class FrontendFailureContracts(unittest.TestCase):
    def record(self, stdout=b'', stderr=b''):
        record = dict(probe='grok-acp-frontend', argv=['inert'], cwd='/inert', error=None, exit=1, elapsed_seconds=0.01)
        for name, body in (('stdout',stdout),('stderr',stderr)):
            record.update({name+'_file':'inert.'+name, name+'_bytes':len(body), name+'_sha256':case.digest(body)})
        return record

    def retain(self, root, record, stdout=b'', stderr=b'', deadline=None):
        return case.retain_frontend_failure(root, record, stdout, stderr, 'a'*40, 'b'*40,
                                            time.monotonic()+5 if deadline is None else deadline)

    def test_original_child_streams_survive_empty_malformed_and_successful_parse(self):
        from native_client_research import run_probe
        for stdout in (b'', b'not json\n', b'{}\n'):
            with self.subTest(stdout=stdout), tempfile.TemporaryDirectory() as temporary:
                root=Path(temporary).resolve(); stderr=b'bounded inert frontend diagnostic\n'
                code='import sys;sys.stdout.buffer.write('+repr(stdout)+');sys.stderr.buffer.write('+repr(stderr)+');sys.exit('+('0' if stdout==b'{}\n' else '1')+')'
                record, out, err=run_probe([sys.executable,'-c',code],root,{},root,'grok-acp-frontend',timeout=3,memory_only=True)
                before=copy.deepcopy(record)
                path, facts=self.retain(root,record,out,err)
                self.assertEqual(record,before);self.assertFalse((root/record['stdout_file']).exists())
                if stdout==b'{}\n':self.assertEqual(case.document(out),{})
                else:
                    with self.assertRaises(ValueError):case.document(out)
                with zipfile.ZipFile(path) as archive:
                    self.assertIsNone(archive.testzip())
                    self.assertEqual(set(archive.namelist()),{'frontend.json','frontend.stdout.bin','frontend.stderr.bin'})
                    self.assertEqual(archive.read('frontend.stdout.bin'),stdout)
                    self.assertEqual(archive.read('frontend.stderr.bin'),stderr)
                    capture=json.loads(archive.read('frontend.json'))
                self.assertEqual(capture['record'],before)
                self.assertFalse(capture['collection_complete']);self.assertFalse(capture['native_cleanup_proven'])
                self.assertTrue(all(value is False for value in capture['qualification'].values()))
                self.assertEqual(facts,dict(bytes=path.stat().st_size,sha256=case.digest(path.read_bytes())))

    def test_incomplete_or_unjoined_capture_refuses_before_creating_output(self):
        for change in ('cleanup','exit','bool-exit','digest','size','bool-size','oversize','extra','expired'):
            with self.subTest(change=change),tempfile.TemporaryDirectory() as temporary:
                root=Path(temporary).resolve();out=b'x'*(case.LIMIT+1) if change=='oversize' else b''
                record=self.record(out)
                if change=='cleanup':record['error']='probe deadline exceeded'
                elif change=='exit':record['exit']=2
                elif change=='bool-exit':record['exit']=True
                elif change=='digest':record['stdout_sha256']='f'*64
                elif change=='size':record['stdout_bytes']=1
                elif change=='bool-size':record['stdout_bytes']=False
                elif change=='extra':record['unreviewed']='PRIVATE'
                with self.assertRaises(ValueError):self.retain(root,record,out,deadline=0.0 if change=='expired' else None)
                self.assertEqual(list(root.iterdir()),[])

    def test_late_deadline_and_replaced_output_never_return_publication(self):
        import native_client_project_artifacts as artifacts
        for changed in ('deadline','replacement'):
            with self.subTest(changed=changed),tempfile.TemporaryDirectory() as temporary:
                root=Path(temporary).resolve();clock=[10.0];actual=artifacts.write_zip
                def write(path,bodies):
                    actual(path,bodies)
                    if changed=='deadline':clock[0]=20.0
                    else:
                        original=next(root.iterdir());original.rename(root/'retained')
                        original.mkdir();(original/'foreign').write_bytes(b'preserved')
                with mock.patch.object(artifacts,'write_zip',side_effect=write),mock.patch.object(case.time,'monotonic',side_effect=lambda:clock[0]):
                    with self.assertRaises(ValueError):self.retain(root,self.record(),deadline=15.0)
                if changed=='replacement':self.assertEqual(next(root.glob('*/foreign')).read_bytes(),b'preserved')


class FixedCLIObservationContracts(unittest.TestCase):
    def test_same_run_parent_authority_help_cleanup_and_closed_publication(self):
        import project_native_research as shared
        from project_native_collectors.grok import ACQUISITION
        commit, tree, native = 'a'*40, 'b'*40, b'inert binary; never executed'
        bodies = {name:(b'' if name.endswith('.lock') else ('inert '+name).encode()) for name in bootstrap_case.PINNED}
        pins = {name:(len(body), case.digest(body), 0o644) for name, body in bodies.items()}
        pins['config.toml'] = bootstrap_case.PINNED['config.toml']
        for fault in ('none', 'unjoined-id', 'help', 'between', 'late-ledger', 'cleanup', 'source-final'):
            with self.subTest(fault=fault), tempfile.TemporaryDirectory() as temporary:
                calls, owned, source_calls = [], [], []
                original_post, original_remove = case.post_ledger, case.shutil.rmtree
                def process(argv, **_options):
                    if argv[0] == 'go':
                        helper = Path(argv[argv.index('-o')+1]); helper.write_bytes(b'inert helper')
                        owned.append((helper.parent, case.stable(helper.parent.lstat())[:2]))
                        return SimpleNamespace(stdout=b'', stderr=b'')
                    if argv[1:2] == ['status']:
                        source_calls.append(1)
                    result = commit if argv[1:] == ['rev-parse','HEAD'] else tree if argv[1:] == ['rev-parse','HEAD^{tree}'] else ''
                    if fault == 'source-final' and len(source_calls) == 2: result = 'M source'
                    return SimpleNamespace(stdout=result.encode(), stderr=b'')
                def capture(_helper, root, _deadline):
                    req = case.document((root/'request.json').read_bytes()); phase = req.get('phase', req['scope']); calls.append(phase)
                    ident = {k:req[k] for k in ('schema_version','source_commit','source_tree','scope','nonce')}
                    result = dict(ident, semantic_complete=True, reason='complete', isolation={}, rpc=case.rpc_state())
                    if phase == 'user':
                        profile = root/'home/.grok'
                        for name, mode in bootstrap_case.DIRECTORIES.items():
                            path=profile/name; path.mkdir(parents=True, exist_ok=True); path.chmod(mode)
                        for name, body in bodies.items():
                            if name not in ('config.toml', '.config-init.lock'): path=profile/name; path.write_bytes(body); path.chmod(0o644)
                        for name in bootstrap_case.MUTABLE:
                            path=profile/name; path.write_bytes(b'x'* (32768 if name.endswith('-shm') else 0)); path.chmod(0o644)
                        path=profile/'agent_id'; path.write_bytes(b'01234567-89ab-4cde-8fab-0123456789ab'); path.chmod(0o600)
                    if phase in case.SCOPES:
                        rows = [plugin_row(root, 'user', name) for name in ('uap-acp-foreign-user',)]
                        rows += [plugin_row(root, phase, name) for name in (('uap-acp-collision',) if phase == 'user' else ('uap-acp-collision','uap-acp-foreign-'+phase))]
                        if fault == 'unjoined-id' and phase == 'a': rows[-2]['id'] = 'project/00000000/uap-acp-collision'
                        result['rpc']['plugins'] = dict(result=dict(result=dict(plugins=rows)))
                        result['commands'] = help_rows() + [startup_row(root, phase), dict(help_rows()[0], label='acp')]
                    else:
                        with (root/'home/.grok/logs/unified.jsonl').open('ab') as stream: stream.write(phase.encode())
                        body = b'Usage: grok plugin details <NAME>\n' if phase == 'details-help' else b'[]\n'
                        if phase == 'details-help' and fault == 'help': body = b'Usage: grok plugin details <OTHER>\n'
                        if phase == 'inspect-project':
                            value=case.document(base64.b64decode(startup_row(root, 'a')['merged_base64']))
                            value.update(projectRoot=str(root/'a'), projectTrusted=False, plugins=[dict(name='uap-acp-collision',
                                scope='project', path=str(root/'a/.grok/plugins/uap-acp-collision'), enabled=False)])
                            body=json.dumps(value).encode()
                        result.update(mode=case.CLI_MODE, phase=phase, commands=[dict(label=phase,
                            argv=['/tmp/uap-grok-acp-sealed-inert/grok', *case.cli_arguments(root, phase)],
                            exit=1 if phase in ('details-full-id','details-name') else 0, error=None,
                            merged_base64=base64.b64encode(body).decode(), merged_bytes=len(body), merged_sha256=case.digest(body))])
                        if fault == 'late-ledger' and phase == 'inspect-project': (root/'unknown-native-effect').write_bytes(b'inert')
                    return dict(ident, cleanup_complete=not (fault == 'cleanup' and phase == 'details-help'), sealed_cleanup_complete=True,
                        system_executables={k:dict(path='/usr/bin/'+('python3.12' if k == 'python3' else k), sha256='d'*64)
                                            for k in ('unshare','setpriv','env','python3')}, result=result)
                def post(*args):
                    ledger = original_post(*args)
                    if fault == 'between' and calls[-1] == 'details-help': (args[0]/'inter-command-edit').write_bytes(b'foreign')
                    return ledger
                try:
                    with (mock.patch.object(shared.native, 'require_hosted'), mock.patch.object(shared, 'acquire', return_value=({}, {'grok':native})),
                          mock.patch.dict(ACQUISITION, binary_sha256=case.digest(native)), mock.patch.object(bootstrap_case,'PINNED',pins),
                          mock.patch.dict(os.environ, EXPECTED_COMMIT=commit), mock.patch.object(case.os,'getuid',return_value=1001),
                          mock.patch.object(case.subprocess,'run',side_effect=process), mock.patch.object(case,'helper_capture',side_effect=capture),
                          mock.patch.object(case,'post_ledger',side_effect=post), mock.patch.object(shared,'Budget',wraps=shared.Budget) as budget,
                          redirect_stdout(io.StringIO()) as stdout):
                        status = case.frontend(Path(temporary)/'output', plugin_cli=True)
                    self.assertEqual(status, 0 if fault == 'none' else 1)
                    budget.assert_called_once_with(150)
                    cli_calls = [p for p in calls if p in case.CLI_PHASES]
                    self.assertEqual(cli_calls, [] if fault == 'unjoined-id' else list(case.CLI_PHASES[:1])
                                     if fault in ('help','between','cleanup') else list(case.CLI_PHASES))
                    packet = case.document(stdout.getvalue().encode())
                    if fault == 'cleanup':
                        self.assertEqual(packet['kind'], 'frontend-publication-refused')
                        continue
                    raw = (Path(temporary)/'output/upload/research.zip').read_bytes()
                    with zipfile.ZipFile(io.BytesIO(raw)) as archive:
                        captured = {name:archive.read(name) for name in archive.namelist()}
                        summary = case.document(archive.read('admission.json'))
                        self.assertTrue(all(v is False for v in summary['qualification'].values()))
                        self.assertEqual(summary['measurement_complete'], fault == 'none')
                        if cli_calls:
                            record = case.document(archive.read(case.CLI_MEMBER))
                            self.assertTrue(all(row['ledger']['before'] == case.document(archive.read('b.ledger.json'))['before'] for row in record['observations']))
                    facts = case.publication_readout(raw, packet, time.monotonic()+5, mode=case.CLI_MODE)
                    self.assertEqual(facts.get('measurement_complete'), fault == 'none')
                    if fault == 'none':
                        self.assertEqual(len(archive.infolist()), 27)
                        for changed in ('selected_id', 'rpc', 'phase', 'extra-row', 'stream-bound', 'source'):
                            value = copy.deepcopy(record); first = value['observations'][0]['receipt']['result']
                            if changed == 'selected_id': value['selected_id'] = 'project/00000000/uap-acp-collision'
                            if changed == 'rpc': first['rpc']['plugins'] = {}
                            if changed == 'phase': first['phase'] = 'enable'
                            if changed == 'extra-row': first['commands'].append(copy.deepcopy(first['commands'][0]))
                            if changed == 'stream-bound': first['commands'][0]['merged_bytes'] = 16385
                            if changed == 'source': value['source_commit'] = 'e'*40
                            bad = dict(captured, **{case.CLI_MEMBER:json.dumps(value).encode()})
                            self.assertEqual(case.publication_readout(AdmissionContracts().readout_zip(bad), packet,
                                time.monotonic()+5, mode=case.CLI_MODE)['reason'], 'readout-refused', changed)
                        with mock.patch.object(zipfile, 'ZipFile') as parser:
                            self.assertEqual(case.publication_readout(raw, packet, time.monotonic()+5)['reason'], 'readout-refused')
                            parser.assert_not_called()
                finally:
                    for root, identity in owned:
                        if root.exists() and case.stable(root.lstat())[:2] == identity: original_remove(root)

    def test_collector_one_combined_stream_deadline_and_no_rpc_or_auth(self):
        req = AdmissionContracts().request(Path('/tmp/uap-grok-acp-input-'+'c'*32))
        req.update(mode=case.CLI_MODE, scope='a', phase='details-name')
        real = case.command
        def inert(argv, label, cwd, env, deadline, limit, state, request):
            self.assertEqual(argv[1:], ['plugin','details','uap-acp-collision'])
            self.assertEqual((deadline, limit, state), (req['start_monotonic_ns']/1e9+5, 16<<10, case.rpc_state()))
            self.assertEqual(env['HOME'], req['input_root']+'/home'); self.assertNotIn('XAI_API_KEY',env)
            row = real([sys.executable,'-I','-c',"import sys;sys.stdout.write('out');sys.stdout.flush();sys.stderr.write('err');sys.exit(3)"],
                       label, Path.cwd(), dict(os.environ), deadline, limit, state, request)
            row['argv'] = argv
            return row
        with mock.patch.object(case,'isolation',return_value={}), mock.patch.object(case,'command',side_effect=inert) as invocation:
            result=case.cli_collector_result(req)
        self.assertTrue(result['semantic_complete']); self.assertEqual(result['rpc'],case.rpc_state())
        self.assertEqual(base64.b64decode(result['commands'][0]['merged_base64']),b'outerr'); invocation.assert_called_once()
        with mock.patch.object(case,'isolation',return_value={}), mock.patch.object(case,'command',side_effect=AssertionError('expired child')):
            expired = case.cli_collector_result(dict(req, start_monotonic_ns=1))
        self.assertFalse(expired['semantic_complete']); self.assertEqual(expired['commands'],[])


@unittest.skipUnless(sys.platform == "linux", "Linux namespace research contracts")
class ReadonlyInventoryContracts(unittest.TestCase):
    """Finite parser/authority counterexamples; no vendor, bwrap or sudo."""

    def request(self):
        import stat
        from project_native_collectors.grok import ACQUISITION
        nonce, uid, gid = 'c' * 32, 1000, 1000
        root = '/tmp/uap-grok-acp-input-' + nonce
        sealed = '/tmp/uap-grok-acp-sealed-123456'
        req = dict(schema_version=1, source_commit='a' * 40, source_tree='b' * 40,
                   uid=uid, gid=gid, nonce=nonce, scope='a', input_root=root,
                   parent_network_namespace=102, start_monotonic_ns=time.monotonic_ns(),
                   mode=case.READONLY_MODE)
        roots = {name:dict(path=str(Path(root) / name),
                 identity=[1, index + 1, stat.S_IFDIR | 0o700, uid, gid])
                 for index, name in enumerate(case.READONLY_ROOTS)}
        roots['sealed'] = dict(path=sealed, identity=[1, 100, stat.S_IFDIR | 0o755, 0, 0])
        runtime = {name:dict(path='/usr' if name == '/usr' else '/usr' + name,
                   identity=[2, 200 + index, stat.S_IFDIR | 0o755, 0, 0])
                   for index, name in enumerate(case.READONLY_RUNTIME_ROOTS)}
        paths = [root + '/' + name for name in case.READONLY_FILES]
        paths += [sealed + '/grok', sealed + '/collector.py', '/usr/bin/bwrap', '/usr/bin/python3.12']
        files = {}
        for index, path in enumerate(paths):
            owner, group = (uid, gid) if path.startswith(root + '/') else (0, 0)
            mode = (0o600 if path.startswith(root + '/') else 0o555 if path == sealed + '/grok'
                    else 0o444 if path == sealed + '/collector.py' else 0o755)
            files[path] = dict(path=path, identity=[3, 300 + index, stat.S_IFREG | mode,
                              1, owner, group, 0, 7, 8], sha256='e' * 64)
        files[sealed + '/grok']['sha256'] = ACQUISITION['binary_sha256']
        devices = {name:dict(path='/dev/' + name,
                   identity=[4, 400 + index, stat.S_IFCHR | 0o666, 1, 0, 0, 0, 7, 8])
                   for index, name in enumerate(case.READONLY_DEVICES)}
        before = {name:fact['identity'] for name, fact in roots.items() if name != 'sealed'}
        before.update({path[len(root) + 1:]:fact['identity'] for path, fact in files.items()
                       if path.startswith(root + '/')})
        ledger = case.readonly_ledger_digest(before)
        req['readonly'] = dict(roots=roots, files=files, runtime=runtime, devices=devices,
                               namespaces={name:100 + index for index, name in enumerate(case.READONLY_NAMESPACES)},
                               overflow_uid=65534, overflow_gid=65534,
                               input_sha256=ledger['sha256'], input_count=ledger['count'])
        return req, before

    def original(self, req):
        namespaces = dict(req['readonly']['namespaces'])
        namespaces['net'] += 100
        return dict(namespaces=namespaces, runtime=dict(sha256='f' * 64, count=5))

    def header(self, req, phase, original):
        number = 10000 if phase == 'control' else 20000
        maps = {name:dict(inside=req[name], parent=0, count=1, overflow=65534,
                         sha256=case.digest((str(req[name]) + ' 0 1\n').encode()))
                for name in ('uid', 'gid')}
        body = dict(schema_version=1, mode=case.READONLY_MODE, source_commit=req['source_commit'],
                    source_tree=req['source_tree'], nonce=req['nonce'], scope='a', phase=phase,
                    parent_namespaces=original['namespaces'],
                    namespaces={name:value + number for name, value in original['namespaces'].items()},
                    maps=maps, mount_sha256='d' * 64, runtime=original['runtime'],
                    input=dict(sha256=req['readonly']['input_sha256'], count=req['readonly']['input_count']),
                    authority_sha256=case.digest(json.dumps(req['readonly'], sort_keys=True, separators=(',', ':')).encode()))
        return case.READONLY_HEADER + json.dumps(body, sort_keys=True, separators=(',', ':')).encode() + b'\n'

    def mount_rows(self, req):
        root, sealed = req['input_root'], req['readonly']['roots']['sealed']['path']
        paths = {'/', '/usr', root, sealed, root + '/readonly-witness', '/proc', '/dev', '/dev/pts'}
        paths.update(case.READONLY_RUNTIME_ROOTS)
        paths.update('/dev/' + name for name in case.READONLY_DEVICES)
        rows = []
        for index, path in enumerate(sorted(paths)):
            fs = {'/':'tmpfs', '/dev':'tmpfs', '/proc':'proc', '/dev/pts':'devpts'}.get(path, 'ext4')
            mode = 'rw' if path == root + '/readonly-witness' else 'ro'
            rows.append(f'{index + 1} 99 0:1 / {path} {mode},nosuid - {fs} source rw')
        return ('\n'.join(rows) + '\n').encode()

    def test_fixed_wrapper_has_one_writable_witness_and_final_root_remount(self):
        req, _ = self.request()
        original = self.original(req)
        case.readonly_request(req)
        argv = case.readonly_bwrap_argv(req, 'native', original)
        root = req['input_root']
        self.assertEqual(argv.count('--bind'), 1)
        index = argv.index('--bind')
        self.assertEqual(argv[index + 1:index + 3], [root + '/readonly-witness'] * 2)
        remounts = [argv[index + 1] for index, word in enumerate(argv) if word == '--remount-ro']
        self.assertEqual(remounts, ['/proc'] + ['/dev/' + name for name in case.READONLY_DEVICES]
                         + ['/dev/pts', '/dev', '/'])
        self.assertNotIn('--share-net', argv)
        self.assertNotIn('--ro-bind', argv[argv.index('--') + 1:])
        self.assertEqual(argv[-3:-1], [req['readonly']['roots']['sealed']['path'] + '/collector-request.json', 'native'])
        self.assertEqual(case.document(argv[-1].encode(), 512), original)
        for phase in ('login', 'session', '', None):
            with self.subTest(phase=phase), self.assertRaises(ValueError):
                case.readonly_bwrap_argv(req, phase, original)
        for key, value in (('mode', case.SESSION_MODE), ('scope', 'b')):
            with self.subTest(key=key), self.assertRaises(ValueError):
                case.readonly_bwrap_argv(dict(req, **{key:value}), 'native', original)

    def test_one_clock_refuses_future_zero_exact_active_and_final_expiry(self):
        req, _ = self.request()
        req['start_monotonic_ns'] = 100
        for elapsed, final, accepted in ((0, False, True), (24_999_999_999, False, True),
                (25_000_000_000, False, False), (29_999_999_999, True, True), (30_000_000_000, True, False)):
            with self.subTest(elapsed=elapsed, final=final), mock.patch.object(case.time, 'monotonic_ns', return_value=100 + elapsed):
                if accepted:
                    self.assertEqual(case.readonly_deadline(req, final=final), (100 + (30 if final else 25) * 10**9) / 1e9)
                else:
                    with self.assertRaises(ValueError):
                        case.readonly_deadline(req, final=final)
        for start in (0, True, 101):
            with self.subTest(start=start), mock.patch.object(case.time, 'monotonic_ns', return_value=100), self.assertRaises(ValueError):
                case.readonly_deadline(dict(req, start_monotonic_ns=start))
        with mock.patch.object(case.time, 'monotonic_ns', return_value=100 + 25 * 10**9), mock.patch.object(case.subprocess, 'Popen') as child:
            with self.assertRaises(ValueError):
                case.readonly_command(req, 'native', self.original(req), case.LIMIT, case.rpc_state())
            child.assert_not_called()

    def test_exact_namespace_maps_refuse_alternate_root_and_overflow(self):
        req, _ = self.request()
        raw = {'/proc/self/uid_map':b'      1000          0          1\n',
               '/proc/self/gid_map':b'1000 0 1\n', '/proc/self/setgroups':b'deny\n',
               '/proc/sys/kernel/overflowuid':b'65534\n', '/proc/sys/kernel/overflowgid':b'65534\n'}
        def read(path, limit):
            self.assertIn(path, raw)
            return raw[path]
        with mock.patch.object(case, 'readonly_fixed_proc', side_effect=read):
            self.assertEqual(case.readonly_id_maps(req, req['readonly'])['uid']['inside'], 1000)
            for path, value in (('/proc/self/uid_map', b'0 0 1\n'),
                    ('/proc/self/gid_map', b'1000 1000 1\n'), ('/proc/self/uid_map', b'1000 0 2\n'),
                    ('/proc/self/uid_map', b'1000 0 1\n1001 1 1\n'), ('/proc/self/setgroups', b'allow\n'),
                    ('/proc/sys/kernel/overflowuid', b'1000\n'), ('/proc/sys/kernel/overflowgid', b'0\n')):
                original = raw[path]
                raw[path] = value
                try:
                    with self.subTest(path=path, value=value), self.assertRaises(ValueError):
                        case.readonly_id_maps(req, req['readonly'])
                finally:
                    raw[path] = original

    def test_closed_mount_table_rejects_every_unadmitted_writable_or_missing_mount(self):
        req, _ = self.request()
        raw = self.mount_rows(req)
        accepted = case.readonly_mounts(raw, req)
        self.assertEqual([name for name, value in accepted['rows'].items() if not value['readonly']],
                         [req['input_root'] + '/readonly-witness'])
        lines = raw.splitlines(keepends=True)
        changes = [
            raw.replace(b' / ro,nosuid ', b' / rw,nosuid ', 1),
            raw.replace(b' /dev/null ro,nosuid ', b' /dev/null rw,nosuid ', 1),
            raw + b'500 99 0:2 / /outside ro - ext4 source ro\n',
            raw + lines[0],
            b''.join(line for line in lines if b' /proc ' not in line),
            raw.replace(b' - proc ', b' - tmpfs ', 1),
            raw.replace(b'ro,nosuid', b'ro,rw,nosuid', 1)]
        for changed in changes:
            with self.subTest(body=changed[:100]), self.assertRaises(ValueError):
                case.readonly_mounts(changed, req)
        optional = raw + b'500 99 0:2 /sys /proc/sys ro,nosuid - proc proc rw\n'
        self.assertIn('/proc/sys', case.readonly_mounts(optional, req)['rows'])

    def test_runtime_link_resolution_never_reads_or_exposes_hidden_targets(self):
        import stat
        req, _ = self.request()
        directory = (2, 21, stat.S_IFDIR | 0o755, 2, 0, 0, 4096, 7, 8)
        regular = (2, 22, stat.S_IFREG | 0o755, 1, 0, 0, 1, 7, 8)
        link = (2, 23, stat.S_IFLNK | 0o777, 1, 0, 0, 1, 7, 8)
        rows = {'bin':directory, 'lib':directory, 'lib/runtime':regular}
        with mock.patch.object(case.os, 'stat', side_effect=AssertionError('external stat')), \
             mock.patch.object(case.os, 'readlink', side_effect=AssertionError('external link read')):
            self.assertEqual(case.readonly_link_target(None, 'bin/tool', '../lib/runtime', rows, req), 'visible')
            self.assertEqual(case.readonly_link_target(None, 'bin/tool', '/etc/alternatives/tool', rows, req), 'hidden')
            for target in ('/', '/tmp', req['input_root'] + '/readonly-witness', '/proc/self/fd/1', '/dev/null'):
                with self.subTest(target=target), self.assertRaisesRegex(ValueError, 'readonly-link-cross-domain'):
                    case.readonly_link_target(None, 'bin/tool', target, rows, req)
            self.assertEqual(case.readonly_link_target(None, 'bin/tool', '/usr/lib/missing', rows, req), 'absent')
            rows['bin/again'] = (*link, 'again')
            with self.assertRaisesRegex(ValueError, 'readonly-link-depth'):
                case.readonly_link_target(None, 'bin/tool', 'again', rows, req)

    def test_real_input_metadata_digest_rejects_symlinks_hardlinks_and_fifo(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            root.chmod(0o700)
            leaf = root / 'leaf'
            leaf.write_bytes(b'original')
            leaf.chmod(0o600)
            req = dict(uid=os.getuid(), gid=os.getgid(), start_monotonic_ns=time.monotonic_ns())
            fd = os.open(root, os.O_RDONLY | os.O_DIRECTORY)
            try:
                rows = case.readonly_walk(fd, req, 4096)
                expected = b''.join(name.encode() + b'\0' + ','.join(map(str, rows[name])).encode() + b'\n'
                                    for name in sorted(rows))
                self.assertEqual(case.readonly_ledger_digest(rows), dict(sha256=case.digest(expected), count=2))
                for kind in ('symlink', 'hardlink', 'fifo'):
                    member = root / kind
                    if kind == 'symlink': member.symlink_to('/outside/not-read')
                    elif kind == 'hardlink': os.link(leaf, member)
                    else: os.mkfifo(member, 0o600)
                    try:
                        with self.subTest(kind=kind), self.assertRaises(ValueError):
                            case.readonly_walk(fd, req, 4096)
                    finally:
                        member.unlink()
                with self.assertRaises(ValueError):
                    case.readonly_walk(fd, req, 1)
            finally:
                os.close(fd)

    def test_parent_replacement_is_joined_before_first_regular_byte_read(self):
        with tempfile.TemporaryDirectory() as temporary:
            outer = Path(temporary).resolve()
            root = outer / 'source'
            root.mkdir(mode=0o700)
            leaf = root / 'object'
            leaf.write_bytes(b'original')
            leaf.chmod(0o600)
            req = dict(start_monotonic_ns=time.monotonic_ns(), readonly=dict(
                roots={'selected':dict(path=str(root), identity=list(case.readonly_metadata(root.lstat(), directory=True)))},
                runtime={}, files={str(leaf):dict(path=str(leaf), identity=list(case.stable(leaf.lstat())),
                                                   sha256=case.digest(b'original'))}))
            original = case.readonly_file_parent
            def replaced(*args):
                held = original(*args)
                root.rename(outer / 'retained')
                root.mkdir(mode=0o700)
                (root / 'object').write_bytes(b'foreign')
                return held
            with mock.patch.object(case, 'readonly_file_parent', side_effect=replaced), \
                 mock.patch.object(case.os, 'read', side_effect=AssertionError('read before original parent join')) as read:
                with self.assertRaisesRegex(ValueError, 'readonly-file-parent-before-read'):
                    case.readonly_verify_files(req)
                read.assert_not_called()
            self.assertEqual((root / 'object').read_bytes(), b'foreign')
            self.assertEqual((outer / 'retained/object').read_bytes(), b'original')

    def test_real_control_marker_is_exactly_removed_with_original_handle_receipt(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            witness = root / 'readonly-witness'
            witness.mkdir(mode=0o700)
            req = dict(input_root=str(root), nonce='d' * 32, uid=os.getuid(), gid=os.getgid(),
                       readonly=dict(roots={'readonly-witness':dict(path=str(witness),
                       identity=list(case.readonly_metadata(witness.lstat(), directory=True)))}))
            marker = witness / 'unexpected-startup'
            body = b'uap-readonly:' + req['nonce'].encode() + b'\n'
            marker.write_bytes(body)
            marker.chmod(0o600)
            original = case.stable(marker.lstat())
            proof = case.readonly_control_marker(req)
            self.assertEqual(proof['identity'], list(original))
            self.assertEqual(proof['removed_identity'][3], 0)
            self.assertEqual(list(witness.iterdir()), [])
            case.readonly_marker_proof(dict(control_complete=True, control_marker=proof), req)
            for field, value in (('name', 'foreign'), ('bytes', False), ('sha256', '0' * 64),
                                 ('removed_identity', proof['identity'])):
                with self.subTest(field=field), self.assertRaises(ValueError):
                    case.readonly_marker_proof(dict(control_complete=True, control_marker=dict(proof, **{field:value})), req)
            marker.write_bytes(b'foreign')
            marker.chmod(0o600)
            with self.assertRaisesRegex(ValueError, 'readonly-control-body'):
                case.readonly_control_marker(req)
            self.assertEqual(marker.read_bytes(), b'foreign')

    def test_header_preserves_warnings_but_rejects_duplicate_or_wrong_original_authority(self):
        req, _ = self.request()
        original = self.original(req)
        body = self.header(req, 'native', original)
        proof, warning = case.readonly_header(body + b'non-protocol warning\n', req, 'native', original)
        self.assertEqual(warning, b'non-protocol warning\n')
        for changed in (body + body, body.replace(b'"phase":"native"', b'"phase":"control"'),
                        body.replace(req['nonce'].encode(), b'0' * 32),
                        body.replace(b'"inside":1000', b'"inside":0', 1),
                        body.replace(b'"overflow":65534', b'"overflow":0', 1)):
            with self.subTest(body=changed[:80]), self.assertRaises(ValueError):
                case.readonly_header(changed, req, 'native', original)
        self.assertNotEqual(proof['namespaces'], original['namespaces'])

    def test_actual_inert_child_keeps_protocol_stdout_distinct_from_warning_stderr(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            (root / 'a').mkdir()
            req, _ = self.request()
            req['input_root'] = str(root)
            frames = [
                dict(jsonrpc='2.0', id=1, result=dict(protocolVersion=1, _meta=dict(agentVersion='1.0.70'))),
                dict(jsonrpc='2.0', id=2, result=dict(result=dict(plugins=[])))]
            script = ("import json,os,sys\nsys.stdin.readline()\n"
                      "os.write(2,b'warning on stderr\\n')\n"
                      "os.write(1," + repr(json.dumps(frames[0]).encode() + b'\n') + ")\n"
                      "sys.stdin.readline()\nos.write(1," + repr(json.dumps(frames[1]).encode() + b'\n') + ")\n"
                      "sys.stdin.read()\n")
            argv = [sys.executable, '-I', '-B', '-c', script]
            state = case.rpc_state()
            with mock.patch.object(case, 'readonly_bwrap_argv', return_value=argv):
                row = case.readonly_command(req, 'native', self.original(req), case.LIMIT, state)
            raw = case.readonly_streams(row)
            self.assertIsNone(row['error'])
            self.assertEqual(raw['stderr'], b'warning on stderr\n')
            self.assertEqual(raw['stdout'], b''.join(json.dumps(frame).encode() + b'\n' for frame in frames))
            self.assertEqual(row['wrapper_argv'], argv)
            self.assertEqual(state['plugins'], frames[1])

    def test_actual_inert_child_rejects_post_completion_frame_without_losing_raw_bytes(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            (root / 'a').mkdir()
            req, _ = self.request()
            req['input_root'] = str(root)
            first = dict(jsonrpc='2.0', id=1, result=dict(protocolVersion=1, _meta=dict(agentVersion='1.0.70')))
            last = dict(jsonrpc='2.0', id=2, result=dict(result=dict(plugins=[])))
            script = ("import os,sys\nsys.stdin.readline()\nos.write(1," +
                      repr(json.dumps(first).encode() + b'\n') + ")\nsys.stdin.readline()\nos.write(1," +
                      repr(json.dumps(last).encode() + b'\n{"unexpected":true}\n') + ")\nsys.stdin.read()\n")
            with mock.patch.object(case, 'readonly_bwrap_argv', return_value=[sys.executable, '-I', '-B', '-c', script]):
                row = case.readonly_command(req, 'native', self.original(req), case.LIMIT, case.rpc_state())
            self.assertEqual(row['error'], 'protocol-refused')
            self.assertTrue(case.readonly_streams(row)['stdout'].endswith(b'{"unexpected":true}\n'))

    def receipt(self):
        import stat
        req, before = self.request()
        original = self.original(req)
        root = Path(req['input_root'])
        frames = [
            dict(jsonrpc='2.0', id=1, result=dict(protocolVersion=1, _meta=dict(agentVersion='1.0.70'))),
            dict(jsonrpc='2.0', method='_x.ai/mcp/servers_updated', params=dict(mcpServers=list(case.readonly_servers(req)))),
            dict(jsonrpc='2.0', id=2, result=dict(result=dict(plugins=[
                plugin_row(root, 'user', 'uap-acp-foreign-user'), plugin_row(root, 'a', 'uap-acp-collision'),
                plugin_row(root, 'a', 'uap-acp-foreign-a')])))]
        commands = []
        for phase in ('control', 'native'):
            stdout = b'' if phase == 'control' else b''.join(json.dumps(row).encode() + b'\n' for row in frames)
            stderr = self.header(req, phase, original)
            row = dict(label='readonly-control' if phase == 'control' else 'acp',
                       argv=case.readonly_child_arguments(req, phase), wrapper_argv=case.readonly_bwrap_argv(req, phase, original),
                       exit=93 if phase == 'control' else -15, error=None)
            for name, raw in (('stdout', stdout), ('stderr', stderr)):
                row.update({name + '_base64':base64.b64encode(raw).decode(),
                            name + '_bytes':len(raw), name + '_sha256':case.digest(raw)})
            commands.append(row)
        body = b'uap-readonly:' + req['nonce'].encode() + b'\n'
        identity = [1, 12345, stat.S_IFREG | 0o600, 1, req['uid'], req['gid'], len(body), 7, 8]
        removed = list(identity)
        removed[3], removed[8] = 0, 9
        marker = dict(name='unexpected-startup', identity=identity, removed_identity=removed,
                      directory_identity=req['readonly']['roots']['readonly-witness']['identity'],
                      bytes=len(body), sha256=case.digest(body))
        result = {name:req[name] for name in ('schema_version', 'source_commit', 'source_tree', 'nonce', 'scope')}
        result.update(mode=case.READONLY_MODE, semantic_complete=True, reason='complete',
            isolation=dict(uid=req['uid'], gid=req['gid'], euid=req['uid'], egid=req['gid'], groups=[],
                network_namespace=original['namespaces']['net'], parent_network_namespace=req['parent_network_namespace'],
                interfaces=['lo'], capabilities_zero=True, no_new_privileges=1),
            commands=commands, rpc=dict(initialize=frames[0], plugins=frames[2], notifications=[frames[1]]),
            readonly=dict(authority=req['readonly'], stage='complete', control_complete=True, original=original, control_marker=marker))
        systems = {name:dict(path='/usr/bin/' + ('python3.12' if name == 'python3' else name), sha256='e' * 64)
                   for name in ('unshare', 'setpriv', 'env', 'python3', 'bwrap')}
        receipt = {name:req[name] for name in ('schema_version', 'source_commit', 'source_tree', 'nonce', 'scope')}
        receipt.update(cleanup_complete=True, sealed_cleanup_complete=True, system_executables=systems, result=result)
        request = {key:value for key, value in req.items() if key not in ('input_root', 'parent_network_namespace', 'readonly')}
        return receipt, request, before

    def test_complete_receipt_replays_raw_stdout_and_requires_control_namespace_and_hash_joins(self):
        receipt, request, _ = self.receipt()
        self.assertTrue(case.validate_readonly_receipt(copy.deepcopy(receipt), request)['semantic_complete'])
        changes = [
            lambda r:r['result']['readonly'].update(control_marker=None),
            lambda r:r['result']['readonly'].update(control_complete=False),
            lambda r:r['result']['readonly']['control_marker'].update(bytes=False),
            lambda r:r['result']['readonly']['authority']['namespaces'].update(net=999),
            lambda r:r['system_executables']['bwrap'].update(sha256='0' * 64),
            lambda r:r['result']['commands'][1].update(stderr_sha256='0' * 64),
            lambda r:r['result']['commands'][0].update(exit=0),
            lambda r:r['result']['commands'][1].update(wrapper_argv=['/usr/bin/grok']),
            lambda r:r.update(cleanup_complete=False),
            lambda r:r['result']['rpc'].update(notifications=[])]
        for change in changes:
            altered = copy.deepcopy(receipt)
            change(altered)
            with self.subTest(change=repr(change)), self.assertRaises((ValueError, TypeError)):
                case.validate_readonly_receipt(altered, request)

    def test_old_source_expired_clock_is_diagnostic_only_and_cannot_renew_native_budget(self):
        receipt, request, _ = self.receipt()
        request['start_monotonic_ns'] = 1
        self.assertTrue(case.validate_readonly_receipt(receipt, request)['semantic_complete'])
        with mock.patch.object(case.time, 'monotonic_ns', return_value=30_000_000_001), self.assertRaises(ValueError):
            case.readonly_deadline(request, final=True)

    def test_request_refuses_alias_roots_extra_runtime_and_unowned_or_privileged_executable(self):
        req, _ = self.request()
        mutations = [
            lambda r:r['readonly']['roots']['.'].update(path=r['input_root'] + '/..'),
            lambda r:r['readonly']['runtime'].update({'/etc':r['readonly']['runtime']['/usr']}),
            lambda r:r['readonly']['roots']['sealed']['identity'].__setitem__(3, r['uid']),
            lambda r:r['readonly']['files']['/usr/bin/bwrap']['identity'].__setitem__(2, 0o104755),
            lambda r:r['readonly']['files']['/usr/bin/bwrap']['identity'].__setitem__(3, 2),
            lambda r:r['readonly']['files']['/usr/bin/bwrap']['identity'].__setitem__(4, r['uid'])]
        for change in mutations:
            altered = copy.deepcopy(req)
            change(altered)
            with self.subTest(change=repr(change)), self.assertRaises(ValueError):
                case.readonly_request(altered)

    def test_actual_inert_parent_transport_keeps_fixed_privilege_argv_and_original_clock(self):
        req, _ = self.request()
        original = case.subprocess.Popen
        requested = []
        def inert(argv, **kwargs):
            requested.append(argv)
            return original([sys.executable, '-I', '-B', '-c', 'import sys;sys.stdout.write("{\\"closed\\":true}\\n")'], **kwargs)
        with mock.patch.object(case.subprocess, 'Popen', side_effect=inert):
            value = case.readonly_helper_capture(Path('/tmp/fixed-helper'), Path(req['input_root']), req)
        self.assertEqual(value, {'closed':True})
        self.assertEqual(requested[0], ['/usr/bin/sudo', '-n',
            '--preserve-env=GITHUB_ACTIONS,RUNNER_ENVIRONMENT,AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED,EXPECTED_COMMIT',
            '/tmp/fixed-helper', '--input-root', req['input_root']])
        with (mock.patch.object(case.time, 'monotonic_ns', return_value=30_000_000_001),
              mock.patch.object(case.subprocess, 'Popen') as child, self.assertRaises(ValueError)):
            case.readonly_helper_capture(Path('/tmp/fixed-helper'), Path(req['input_root']),
                                         dict(req, start_monotonic_ns=1))
        child.assert_not_called()

    def test_readout_requires_original_typed_ledger_and_complete_raw_stream_members(self):
        receipt, request, before = self.receipt()
        receipt = copy.deepcopy(receipt)
        result = receipt['result']
        members = {'acquisition.json':b'{}'}
        for row in result['commands']:
            for stream in ('stdout', 'stderr'):
                name = row['label'] + '.' + stream
                members[name] = base64.b64decode(row.pop(stream + '_base64'), validate=True)
                row[stream + '_file'] = name
        members['readonly.rpc.json'] = json.dumps(result.pop('rpc')).encode()
        ledger = dict(before=before, after=copy.deepcopy(before), profiles_before={'fixture':['digest', 0]},
                      profiles_after={'fixture':['digest', 0]}, protected_originals_unchanged=True, witness_absent=True)
        members['readonly.ledger.json'] = json.dumps(ledger).encode()
        summary = {name:request[name] for name in ('schema_version', 'source_commit', 'source_tree', 'nonce', 'start_monotonic_ns')}
        summary.update(mode=case.READONLY_MODE, collection_complete=False, measurement_complete=True,
                       cleanup_complete=True, qualification=dict(case.FALSE), scopes=[receipt], run='1', attempt='1', elapsed_ns=1)
        members['admission.json'] = json.dumps(summary).encode()
        packet = {name:request[name] for name in ('source_commit', 'source_tree', 'nonce')}
        self.assertEqual(case.readonly_readout(copy.deepcopy(members), packet, time.monotonic()+5)['reason'], 'complete')
        changed = copy.deepcopy(ledger)
        changed['profiles_after']['fixture'][1] = False
        mutated = dict(members, **{'readonly.ledger.json':json.dumps(changed).encode()})
        with self.assertRaisesRegex(ValueError, 'readonly-readout-originals'):
            case.readonly_readout(mutated, packet, time.monotonic()+5)
        changed = copy.deepcopy(ledger)
        changed['after']['home/.grok/config.toml'][6] = False
        mutated = dict(members, **{'readonly.ledger.json':json.dumps(changed).encode()})
        with self.assertRaisesRegex(ValueError, 'readonly-ledger-types'):
            case.readonly_readout(mutated, packet, time.monotonic()+5)
        for key in ('readonly-control.stderr', 'acp.stdout'):
            altered = dict(members)
            altered[key] += b'\n'
            with self.subTest(key=key), self.assertRaises(ValueError):
                case.readonly_readout(altered, packet, time.monotonic()+5)
        changed = copy.deepcopy(summary)
        changed['elapsed_ns'] = 30_000_000_000
        with self.assertRaisesRegex(ValueError, 'readonly-readout-summary'):
            case.readonly_readout(dict(members, **{'admission.json':json.dumps(changed).encode()}), packet, time.monotonic()+5)



if __name__ == '__main__':
    unittest.main()
