"""Inert tests execute actual parser, capture fence and artifacts; no vendor."""
import base64
import copy
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


def help_rows():
    bodies = (b'grok 1.0.70 (f50904eaee6c)\n', b'Commands:\n  agent Agent\n',
              b'Commands:\n  stdio Agent\n --no-leader\n', b'Usage: grok agent stdio')
    return [dict(label=label, argv=['/sealed/grok'], exit=0, error=None, merged_base64=base64.b64encode(body).decode(),
                 merged_bytes=len(body), merged_sha256=case.digest(body)) for label, body in zip(case.LABELS, bodies)]


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
        self.assertTrue(case.rpc_frame({'jsonrpc': '2.0', 'id': 2, 'result': {'plugins': []}}, state, req, sent.append))
        with self.assertRaises(ValueError): case.rpc_frame(notification, state, req, sent.append)

    def test_requests_unknown_notifications_ids_and_versions_refuse(self):
        for value in ({'jsonrpc': '2.0', 'id': 1, 'method': 'fs/read_text_file', 'params': {}},
                      {'jsonrpc': '2.0', 'method': '_unknown', 'params': {}},
                      {'jsonrpc': '2.0', 'method': '_x.ai/mcp/servers_updated', 'params': {'mcpServers': ['foreign']}},
                      {'jsonrpc': '2.0', 'id': True, 'result': {}},
                      {'jsonrpc': '2.0', 'id': 1, 'result': {'protocolVersion': '1', '_meta': {'agentVersion': '1.0.70'}}},
                      {'jsonrpc': '2.0', 'id': 1, 'result': {'protocolVersion': 1, '_meta': {'agentVersion': '1.0.71'}}}):
            with self.subTest(value=value), self.assertRaises(ValueError):
                case.rpc_frame(value, case.rpc_state(), {'nonce': 'a' * 32, 'scope': 'a'}, lambda _: self.fail('unexpected request'))

    def test_help_requires_exact_native_pin_and_advertised_syntax(self):
        rows = help_rows()
        case.help_admitted(rows)
        for index, body in ((0, b'grok 1.0.71\n'), (2, b'Commands:\n  stdio Agent\n')):
            changed = copy.deepcopy(rows)
            changed[index]['merged_base64'] = base64.b64encode(body).decode()
            with self.assertRaises(ValueError): case.help_admitted(changed)

    def test_full_id_requires_exact_roots_scope_and_no_sibling(self):
        root = Path('/tmp/uap-fixture')
        rows = [dict(id='user/12345678/' + name, root=str(root / 'home/.grok/plugins' / name), name=name, scope='user', trusted=True, enabled=True)
                for name in ('uap-acp-collision', 'uap-acp-foreign-user')]
        result = {'rpc': {'plugins': {'result': {'plugins': rows}}}}
        case.inventory_rows(result, root, 'user')
        for mutate in ('missing', 'duplicate', 'scope', 'sibling', 'name-only'):
            value = copy.deepcopy(result)
            current = value['rpc']['plugins']['result']['plugins']
            if mutate == 'missing': current.pop()
            elif mutate == 'duplicate': current.append(current[0])
            elif mutate == 'scope': current[0]['scope'] = 'project'
            elif mutate == 'name-only': current[0]['id'] = current[0]['name']
            else: current[0]['root'] = str(root / 'b/.grok/plugins/foreign')
            with self.assertRaises(ValueError): case.inventory_rows(value, root, 'user')

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
        import project_native_research as shared
        from devin_structured_inventory_admission import metadata_inventory
        from client_automation_native_probe import snapshot
        with tempfile.TemporaryDirectory() as directory, shared.Budget(10) as budget:
            root = Path(directory).resolve()
            with mock.patch.object(case.subprocess, 'run'): roots = case.fixtures(root, 'a' * 32)
            (root / 'request.json').write_text('{}')
            baseline = metadata_inventory(root, budget)
            profiles = {name: snapshot(path) for name, path in roots.items()}
            self.assertTrue(case.post_ledger(root, baseline, roots, profiles, budget)['accepted'])
            trust = root / 'home/.grok/trusted_folders.toml'
            trust.write_text('unadmitted')
            self.assertFalse(case.post_ledger(root, baseline, roots, profiles, budget)['accepted'])
            trust.unlink()
            (root / 'unknown').mkdir()
            self.assertFalse(case.post_ledger(root, baseline, roots, profiles, budget)['accepted'])

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



if __name__ == '__main__':
    unittest.main()
