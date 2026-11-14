"""Inert archive bytes only; these contracts do not execute Grok or qualify its protocol."""
import base64
import contextlib
import importlib.util
import io
import json
from pathlib import Path
import struct
import sys
import tempfile
import unittest
from unittest import mock
import warnings
import zipfile
import zlib

spec = importlib.util.spec_from_file_location('reader', Path(__file__).with_name('native_fixed_original_readout.py'))
reader = importlib.util.module_from_spec(spec)
spec.loader.exec_module(reader)

class FixedOriginalTests(unittest.TestCase):
    def packed(self, members, compression=zipfile.ZIP_DEFLATED):
        output = io.BytesIO()
        with zipfile.ZipFile(output, 'w', compression=compression) as archive:
            for name, body in members.items():
                archive.writestr(name, body)
        return output.getvalue()

    def fixture(self):
        nonce = 'a' * 32
        initial = {'jsonrpc': '2.0', 'id': 1, 'result': {'protocolVersion': 1}}
        notices = [{'jsonrpc': '2.0', 'method': '_x.ai/session/setup',
                    'params': {'method': 'session/new', 'phase': 'auth', 'sessionId': None}}
                   for _ in range(11)]
        selected = json.dumps({'jsonrpc': '2.0', 'method': 'session/update',
                               'params': {'inert': 'ONLY_HASHED_FRAME'}}).encode()
        wire = b'\n'.join([json.dumps(initial).encode(),
                           *(json.dumps(row).encode() for row in notices), selected]) + b'\n'
        rpc = dict(initialize=initial, plugins=None, notifications=notices, session=None)
        members = {'a.rpc.json': json.dumps(rpc).encode(), 'a-acp.merged.log': wire}
        commands = []
        for index, label in enumerate(reader.LABELS):
            name = 'a-' + label + '.merged.log'
            if index < 4:
                members[name] = b'grok 1.0.70 (f50904eaee6c)\n' if index == 0 else b'PRIVATE_HELP_TEXT\n'
            commands.append(dict(label=label, argv=['/tmp/uap-grok-acp-sealed-PRIVATE/grok', *reader.ARGUMENTS[index]],
                                 exit=0 if index < 4 else -9, error=None if index < 4 else 'protocol-refused',
                                 merged_file=name, merged_bytes=len(members[name]), merged_sha256=reader.digest(members[name])))
        identity = dict(schema_version=1, source_commit=reader.SOURCE, source_tree=reader.TREE, scope='a', nonce=nonce)
        isolation = dict(uid=1001, gid=1001, euid=1001, egid=1001, groups=[], network_namespace=42,
                         parent_network_namespace=43, interfaces=['lo'], capabilities_zero=True,
                         no_new_privileges=1, loopback_up=True)
        result = dict(identity, semantic_complete=False, reason='protocol-refused', isolation=isolation,
                      commands=commands, mode=reader.MODE,
                      provider=dict(requests=2, models_get=1, root_get=1, forbidden=0, settled=True))
        executables = {name: dict(path='/usr/bin/' + ('python3.12' if name == 'python3' else name),
                                 sha256='1' * 64) for name in ('unshare', 'setpriv', 'env', 'python3')}
        receipt = dict(identity, cleanup_complete=True, sealed_cleanup_complete=True,
                       system_executables=executables, result=result)
        summary = dict(schema_version=1, source_commit=reader.SOURCE, source_tree=reader.TREE, nonce=nonce,
                       run=reader.RUN, attempt='1', mode=reader.MODE, collection_complete=False,
                       cleanup_complete=True, measurement_complete=True, error='admission-refused',
                       qualification=dict.fromkeys(reader.FLAGS, False), scopes=[receipt])
        acquired = dict(url='https://x.ai/cli/grok-1.0.70-linux-x86_64.gz', resolved_origin='https://PRIVATE.invalid',
                        bytes=76760282, sha256=reader.PIN, integrity='sha256:' + reader.PIN,
                        pin_verified=True, installer_executed=False,
                        members=[dict(name='grok', kind='file', bytes=1024, executable=True, sha256=reader.BINARY)])
        ledger = dict(before={'private': [1]}, after={'private': [2]}, accepted=False,
                      protected_originals_unchanged=False, selected_config_capture={'PRIVATE_CONFIG': 'must not be projected'})
        members.update({'admission.json': json.dumps(summary).encode(),
                        'acquisition.json': json.dumps(acquired).encode(), 'a.ledger.json': json.dumps(ledger).encode()})
        return members, summary, selected

    def pins(self, body, frame):
        return mock.patch.multiple(reader, ARCHIVE_SIZE=len(body), ARCHIVE_HASH=reader.digest(body),
                                   FRAME_SIZE=len(frame), FRAME_HASH=reader.digest(frame))

    def archive(self, members):
        return self.packed({'research.zip': self.packed(members)})

    def set_summary(self, members, summary):
        members['admission.json'] = json.dumps(summary).encode()

    def refresh_wire(self, members, summary):
        row = summary['scopes'][0]['result']['commands'][-1]
        body = members['a-acp.merged.log']
        row.update(merged_bytes=len(body), merged_sha256=reader.digest(body))
        self.set_summary(members, summary)

    def test_complete_real_nested_zip_closed_projection_and_no_native_verdict(self):
        members, _, frame = self.fixture()
        body = self.archive(members)
        with self.pins(body, frame):
            result = reader.audit(body)
        self.assertEqual(base64.b64decode(result['frame_base64'], validate=True), frame)
        self.assertEqual(result['command_streams_joined'], 5)
        self.assertEqual(result['prefix_frames_joined'], 12)
        self.assertEqual(result['inner_members'], 9)
        self.assertTrue(result['crc_verified'])
        self.assertTrue(result['original_failed'])
        self.assertFalse(result['native_qualified'])
        self.assertFalse(result['original_semantic_complete'])
        self.assertTrue(all(value is False for value in result['qualification'].values()))
        encoded = reader.encode(result)
        self.assertLessEqual(len(encoded), 8192)
        self.assertNotIn(b'PRIVATE', encoded)

    def test_original_fixed_outer_identity_is_checked_before_zip_parser(self):
        with mock.patch.object(reader, 'unpack') as unpack:
            for raw in (b'', b'x' * reader.ARCHIVE_SIZE, b'x' * (reader.ARCHIVE_SIZE + 1)):
                with self.subTest(size=len(raw)), self.assertRaises(ValueError):
                    reader.audit(raw)
            unpack.assert_not_called()

    def test_unknown_missing_duplicate_member_and_local_crc_refuse(self):
        for members in ({'../research.zip': b'x'}, {'research.zip': b'x', 'extra': b'y'}, {}):
            with self.subTest(members=list(members)), self.assertRaises(ValueError):
                reader.unpack(self.packed(members), {'research.zip'})
        out = io.BytesIO()
        with warnings.catch_warnings():
            warnings.simplefilter('ignore', UserWarning)
            with zipfile.ZipFile(out, 'w') as archive:
                archive.writestr('research.zip', b'one')
                archive.writestr('research.zip', b'two')
        with self.assertRaises(ValueError):
            reader.unpack(out.getvalue(), {'research.zip'})
        original = self.packed({'research.zip': b'exact stored bytes'}, zipfile.ZIP_STORED)
        corrupt = bytearray(original)
        corrupt[original.index(b'exact stored bytes')] ^= 1
        with self.assertRaises((ValueError, zipfile.BadZipFile)):
            reader.unpack(bytes(corrupt), {'research.zip'})

    def test_eocd_bounds_apply_before_zipfile_and_archive_comment_refuses(self):
        raw = bytearray(self.packed({'research.zip': b'body'}))
        struct.pack_into('<H', raw, len(raw) - 12, 65535)
        with mock.patch.object(reader.zipfile, 'ZipFile') as parser:
            with self.assertRaises(ValueError):
                reader.unpack(bytes(raw), {'research.zip'})
            parser.assert_not_called()
        original = self.packed({'research.zip': b'body'})
        with self.assertRaises(ValueError):
            reader.unpack(original + b'comment', {'research.zip'})

    def test_member_type_encryption_and_expansion_bounds_refuse(self):
        output = io.BytesIO()
        info = zipfile.ZipInfo('research.zip')
        info.external_attr = (0o120777 << 16)
        with zipfile.ZipFile(output, 'w') as archive:
            archive.writestr(info, b'target')
        with self.assertRaises(ValueError):
            reader.unpack(output.getvalue(), {'research.zip'})
        original = bytearray(self.packed({'research.zip': b'body'}))
        central = original.index(b'PK\x01\x02')
        for offset in (6, central + 8):
            flags = struct.unpack_from('<H', original, offset)[0]
            struct.pack_into('<H', original, offset, flags | 1)
        with self.assertRaises(ValueError):
            reader.unpack(bytes(original), {'research.zip'})
        with self.assertRaises(ValueError):
            reader.unpack(self.packed({'research.zip': b'x' * ((1 << 20) + 1)}), {'research.zip'})

    def test_actual_deflate_prefix_forgery_and_trailing_bytes_refuse(self):
        data = b'actual complete expanded bytes'
        original = self.packed({'research.zip': data})
        self.assertEqual(reader.unpack(original, {'research.zip'})['research.zip'], data)
        central = original.index(b'PK\x01\x02')
        forged = bytearray(original)
        prefix = data[:3]
        for offset in (14, central + 16):
            struct.pack_into('<I', forged, offset, zlib.crc32(prefix))
        struct.pack_into('<I', forged, 22, len(prefix))
        struct.pack_into('<I', forged, central + 24, len(prefix))
        with self.assertRaises(ValueError):
            reader.unpack(bytes(forged), {'research.zip'})
        with zipfile.ZipFile(io.BytesIO(original)) as archive:
            info = archive.infolist()[0]
            start = 30 + len(info.filename)
            reader.check_expansion(original, start, info)
            info.compress_size += 1
            with self.assertRaises(ValueError):
                reader.check_expansion(original, start, info)

    def test_closed_source_cleanup_qualification_and_native_failure_types(self):
        mutations = [
            (('schema_version',), True), (('source_commit',), '0' * 40), (('source_tree',), '0' * 40),
            (('run',), 'other'), (('attempt',), 1), (('mode',), 'other'), (('nonce',), 'bad'),
            (('collection_complete',), True), (('cleanup_complete',), False),
            (('measurement_complete',), 1), (('error',), None), (('extra',), 'PRIVATE'),
            (('qualification', reader.FLAGS[0]), 0), (('qualification', reader.FLAGS[-1]), True),
            (('scopes', 0, 'source_commit'), '0' * 40), (('scopes', 0, 'nonce'), 'b' * 32),
            (('scopes', 0, 'cleanup_complete'), 1), (('scopes', 0, 'sealed_cleanup_complete'), False),
            (('scopes', 0, 'result', 'semantic_complete'), True), (('scopes', 0, 'result', 'reason'), 'complete'),
            (('scopes', 0, 'result', 'mode'), 'other'),
            (('scopes', 0, 'result', 'isolation', 'euid'), 0),
            (('scopes', 0, 'result', 'isolation', 'no_new_privileges'), True),
            (('scopes', 0, 'result', 'provider', 'settled'), False),
            (('scopes', 0, 'result', 'provider', 'forbidden'), True),
            (('scopes', 0, 'system_executables', 'env', 'path'), '/PRIVATE/env'),
        ]
        for path, value in mutations:
            with self.subTest(path=path):
                members, summary, frame = self.fixture()
                target = summary
                for part in path[:-1]:
                    target = target[part]
                target[path[-1]] = value
                self.set_summary(members, summary)
                body = self.archive(members)
                with self.pins(body, frame), self.assertRaises(ValueError):
                    reader.audit(body)

    def test_all_command_bytes_argv_exit_and_hashes_are_joined(self):
        mutations = [('merged_bytes', True), ('merged_sha256', '0' * 64),
                     ('merged_file', 'unknown'), ('label', 'other'), ('error', 'deadline'),
                     ('argv', ['/PRIVATE/grok', 'agent', '--no-leader', 'stdio'])]
        for key, value in mutations:
            with self.subTest(key=key):
                members, summary, frame = self.fixture()
                summary['scopes'][0]['result']['commands'][-1][key] = value
                self.set_summary(members, summary)
                body = self.archive(members)
                with self.pins(body, frame), self.assertRaises((ValueError, KeyError)):
                    reader.audit(body)
        members, summary, frame = self.fixture()
        summary['scopes'][0]['result']['commands'][0]['exit'] = True
        self.set_summary(members, summary)
        body = self.archive(members)
        with self.pins(body, frame), self.assertRaises(ValueError):
            reader.audit(body)

    def test_wire_prefix_exact_frame_position_and_stored_rpc_refuse_drift(self):
        for mode in ('tail', 'unterminated', 'frame-copy', 'rpc-copy', 'session-reply'):
            with self.subTest(mode=mode):
                members, summary, frame = self.fixture()
                if mode == 'tail':
                    members['a-acp.merged.log'] += b'{}\n'
                elif mode == 'unterminated':
                    members['a-acp.merged.log'] = members['a-acp.merged.log'][:-1]
                elif mode == 'frame-copy':
                    members['a-acp.merged.log'] = members['a-acp.merged.log'].replace(frame, b'{}')
                else:
                    rpc = json.loads(members['a.rpc.json'])
                    if mode == 'rpc-copy':
                        rpc['initialize']['result']['different'] = True
                    else:
                        rpc['session'] = {'private': True}
                    members['a.rpc.json'] = json.dumps(rpc).encode()
                self.refresh_wire(members, summary)
                body = self.archive(members)
                with self.pins(body, frame), self.assertRaises(ValueError):
                    reader.audit(body)

    def test_acquisition_and_ledger_guards_preserve_negative_readout(self):
        for name, key, value in (
            ('acquisition.json', 'pin_verified', 1),
            ('acquisition.json', 'installer_executed', True),
            ('acquisition.json', 'sha256', '0' * 64),
            ('a.ledger.json', 'accepted', True),
            ('a.ledger.json', 'protected_originals_unchanged', True),
        ):
            with self.subTest(name=name, key=key):
                members, _, frame = self.fixture()
                value_object = json.loads(members[name])
                value_object[key] = value
                members[name] = json.dumps(value_object).encode()
                body = self.archive(members)
                with self.pins(body, frame), self.assertRaises(ValueError):
                    reader.audit(body)

    def test_duplicate_json_nonfinite_wrong_types_and_output_bound_refuse(self):
        for body in (b'{"a":1,"a":2}', b'{"a":NaN}', b'{"a":Infinity}', b'\xff'):
            with self.subTest(body=body), self.assertRaises((ValueError, UnicodeError)):
                reader.document(body)
        with self.assertRaises(ValueError):
            reader.encode({'private': 'x' * 8192})
        with self.assertRaises(ValueError):
            reader.closed([], '')

    def test_cli_mismatch_or_oversized_projection_emits_only_closed_refusal(self):
        with tempfile.TemporaryDirectory() as root:
            path = Path(root) / 'original.zip'
            path.write_bytes(b'not the original')
            for large in (False, True):
                output = io.StringIO()
                patches = contextlib.ExitStack()
                with patches:
                    patches.enter_context(mock.patch.object(sys, 'argv', ['reader', reader.CASE, str(path)]))
                    if large:
                        patches.enter_context(mock.patch.object(reader, 'audit', return_value={'frame_base64': 'PRIVATE' * 2048}))
                    patches.enter_context(contextlib.redirect_stdout(output))
                    self.assertEqual(reader.main(), 1)
                result = json.loads(output.getvalue())
                self.assertEqual(result, dict(kind='fixed-original-readout', case=reader.CASE,
                                              refused=True, native_qualified=False))
                self.assertLessEqual(len(output.getvalue().encode()), 8192)
                self.assertNotIn('PRIVATE', output.getvalue())
                self.assertNotIn('frame_base64', result)


class HermesFixedOriginalTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.helpers, cls.sources = reader.hermes_definitions()

    def packed(self, members):
        output = io.BytesIO()
        with zipfile.ZipFile(output, 'w', compression=zipfile.ZIP_DEFLATED) as archive:
            for name, body in members.items():
                archive.writestr(name, body)
        return output.getvalue()

    def update_proof(self, members, proof):
        proof['streams'] = {name: dict(bytes=len(body), sha256=reader.digest(body))
                            for name, body in members.items() if name != 'profile-admission.json'}
        members['profile-admission.json'] = json.dumps(proof).encode()

    def fixture(self, case='hermes-4cd-source-linux', *, diagnostics=False):
        nonce = 'a' * 64
        function = self.helpers['source_bound_predicate'].__code__
        line = next(line for _, _, line in function.co_lines() if line is not None and line > function.co_firstlineno)
        observed = dict(schema_version=1, measurement_only=True, admission_qualified=False,
                        nonce=nonce, source_head=reader.HERMES_HEAD, phase='profiles',
                        reason='uncaught-observer-exception', exception_kind='RuntimeError', command='guard-a',
                        caller=dict(function=function.co_name, function_line=function.co_firstlineno, line=line))
        members = {'lifecycle.stdout': json.dumps(observed, sort_keys=True, separators=(',', ':')).encode() + b'\n',
                   'lifecycle.stderr': (f'Traceback (most recent call last):\n  File "/PRIVATE/hermes-profile-admission.py", line {line}, in {function.co_name}\nRuntimeError: PRIVATE_NATIVE_DETAILS\n').encode(),
                   'guard-a.stdout': b'PRIVATE_COMMAND_OUT', 'guard-a.stderr': b''}
        status = dict(argv=['/PRIVATE/python', '-c', 'PRIVATE_COMMAND'], pid=1234, exit=0, error=None,
                      streams={suffix: dict(bytes=len(members['guard-a.' + suffix]), sha256=reader.digest(members['guard-a.' + suffix]))
                               for suffix in ('stdout', 'stderr')})
        members['guard-a.status.json'] = json.dumps(status).encode()
        source = dict(commit=reader.HERMES_HEAD, tree=reader.HERMES_TREE, dirty='')
        native_source = dict(commit=reader.HERMES_PIN, tree=reader.HERMES_PIN_TREE, dirty='')
        proof = dict(source=source, source_after=source, hermes_source=native_source, hermes_source_after=native_source,
                     nonce=nonce, run_id=reader.HERMES_RUN, run_attempt='1', runner_os=reader.HERMES_CASES[case][3],
                     runtime_distribution='source', status='failed', cleanup=reader.HERMES_SETTLED, lifecycle_exit=1,
                     error='RuntimeError: native profile observation failed', runtime_error='RuntimeError: PRIVATE_SECONDARY')
        proof.update(dict.fromkeys(reader.HERMES_FLAGS, False))
        if diagnostics:
            _, authored = self.helpers['user_fixture_content'](nonce)
            home = Path('fixture')
            directories = sorted(self.helpers['source_preparation_directories'](
                dict(home=home, user=home / 'user', original=home / 'user/.hermes'), authored))
            prepared = dict(schema_version=1, nonce=nonce, outer_identity=[1, 2, 0o40700, 1001, 1001],
                            admission_qualified=False, original_user=dict(sha256='1' * 64, entries=2),
                            prepared_user=dict(sha256='2' * 64, entries=5), installer_preimages={},
                            declared_directories=directories, authored_members=sorted(authored))
            proof['source_user_preparation'] = prepared
            proof['source_prepared_runtime'] = dict(user=prepared['prepared_user'])
            proof['runtime_after'] = dict(user=dict(sha256='3' * 64, entries=5))
            members['source-user-preparation.json'] = json.dumps(prepared).encode()
            refusal = dict(schema_version=1, measurement_only=True, admission_qualified=False, nonce=nonce,
                           stage='observer-profile-initial', operation='ordinary-profile-bound',
                           member='user/PRIVATE_MEMBER', member_sha256=reader.digest(b'user/PRIVATE_MEMBER'),
                           hash_encoding='utf-8-surrogatepass', integer_saturated=False,
                           metadata=dict(device=1, inode=2, mode=0o100600, links=1, bytes=(1 << 20) + 1,
                                         uid=1001, gid=1001, attributes=None, reparse_tag=None),
                           bytes_read=None, errno=None, winerror=None, target_sha256=None, target_bytes=None,
                           profile_bound=dict(scope='original-user', kind='file', ordinary_count=1, predicate='file-bytes'))
            members['source-payload-refusal.json'] = json.dumps(refusal).encode()
            metadata = dict.fromkeys(self.helpers['SOURCE_DELTA_FIELDS'], 1)
            metadata.update(mode=0o100600, links=1, uid=1001, gid=1001, birthtime_ns=None, attributes=None, reparse_tag=None)
            before = dict(kind='file', metadata=metadata, integer_saturated=False, sha256='4' * 64, target_sha256=None)
            after = dict(kind='file', metadata=dict(metadata, bytes=2), integer_saturated=False, sha256='5' * 64, target_sha256=None)
            delta = dict(schema_version=1, measurement_only=True, admission_qualified=False, nonce=nonce,
                         stage='parent-after-settlement', hash_encoding='utf-8-surrogatepass',
                         before=proof['source_prepared_runtime']['user'], after=proof['runtime_after']['user'],
                         total_changes=1, truncated=False,
                         changes=[dict(member='user/PRIVATE_MEMBER', member_sha256=reader.digest(b'user/PRIVATE_MEMBER'),
                                       before=before, after=after)])
            members['source-user-delta.json'] = json.dumps(delta).encode()
            members['bootstrap-census-request.json'] = json.dumps(dict(schema_version=1, reason='initial-inventory-refused',
                                                                       nonce=nonce, root_identity=[1, 2, 0o40700, 1001, 1001])).encode()
            members['bootstrap-census.json'] = json.dumps(dict(schema_version=1, measurement_only=True, admission_qualified=False,
                                                               complete=False, status='refused', reason='request')).encode()
        self.update_proof(members, proof)
        return members, proof

    @contextlib.contextmanager
    def pinned(self, body, case='hermes-4cd-source-linux'):
        artifact, _, _, os_name = reader.HERMES_CASES[case]
        with mock.patch.dict(reader.HERMES_CASES, {case: (artifact, len(body), reader.digest(body), os_name)}), \
             mock.patch.object(reader, 'hermes_definitions', return_value=(self.helpers, self.sources)):
            yield

    def test_direct_original_all_three_os_cases_closed_metadata_and_all_stream_joins(self):
        for case in reader.HERMES_CASES:
            with self.subTest(case=case):
                members, _ = self.fixture(case, diagnostics=True)
                body = self.packed(members)
                with self.pinned(body, case):
                    result = reader.audit_hermes(body, case)
                self.assertFalse(result['native_qualified'])
                self.assertTrue(result['original_failed'])
                self.assertTrue(all(value is False for value in result['qualification'].values()))
                self.assertEqual(result['command_status_count'], 1)
                self.assertEqual(result['command_streams_joined'], 2)
                self.assertEqual(result['cleanup_state'], 'source-retained-settled')
                self.assertEqual(result['source_refusal']['profile_bound']['predicate'], 'file-bytes')
                self.assertTrue(result['census_present'])
                self.assertTrue(result['observer_locator_present'])
                self.assertEqual(result['observer_stdout_state'], 'validated')
                self.assertEqual(result['source_delta']['projected_rows'], 1)
                self.assertNotIn('member', result['source_delta']['changes'][0])
                encoded = reader.encode(result)
                self.assertNotIn(b'PRIVATE', encoded)
                self.assertLessEqual(len(encoded), 8192)

    def test_actual_definition_import_and_validators_have_no_process_socket_thread_or_write_effects(self):
        # Load stdlib class definitions before forbidding actual instances.
        import builtins
        import http.server
        import os
        import socket
        import ssl
        import subprocess
        import threading
        import types
        forbidden = []
        def reject(*_args, **_kwargs):
            forbidden.append(True)
            raise AssertionError('forbidden import/validator effect')
        original_open = builtins.open
        def readonly_open(file, mode='r', *args, **kwargs):
            if any(flag in mode for flag in 'wax+'):
                return reject()
            return original_open(file, mode, *args, **kwargs)
        original_io_open = io.open
        def readonly_io_open(file, mode='r', *args, **kwargs):
            if any(flag in mode for flag in 'wax+'):
                return reject()
            return original_io_open(file, mode, *args, **kwargs)
        original_os_open = os.open
        def readonly_os_open(path, flags, *args, **kwargs):
            if flags & (os.O_WRONLY | os.O_RDWR | os.O_CREAT | os.O_TRUNC | os.O_APPEND):
                return reject()
            return original_os_open(path, flags, *args, **kwargs)
        foreign = types.ModuleType('hermes_local_provider_fixture')
        foreign.__file__ = '/PRIVATE/foreign.py'
        old_path, old_bytecode = list(sys.path), sys.dont_write_bytecode
        with contextlib.ExitStack() as stack:
            stack.enter_context(mock.patch.dict(sys.modules, {'hermes_local_provider_fixture': foreign}))
            for owner, name in ((subprocess, 'Popen'), (subprocess, 'run'), (socket, 'socket'), (os, 'system'),
                                (threading.Thread, 'start'), (os, 'mkdir'), (os, 'rmdir'), (os, 'unlink'),
                                (os, 'rename'), (os, 'replace'), (os, 'link'), (os, 'symlink')):
                stack.enter_context(mock.patch.object(owner, name, side_effect=reject))
            stack.enter_context(mock.patch.object(builtins, 'open', side_effect=readonly_open))
            stack.enter_context(mock.patch.object(io, 'open', side_effect=readonly_io_open))
            stack.enter_context(mock.patch.object(os, 'open', side_effect=readonly_os_open))
            helpers, sources = reader.hermes_definitions()
            self.assertIs(sys.modules['hermes_local_provider_fixture'], foreign)
            globals_list = (helpers['source_refusal_document'].__globals__,
                            helpers['BASE']['require'].__globals__, helpers['BASE']['NATIVE']['require'].__globals__)
            for namespace in globals_list:
                banned = {name: reject for name in ('run', 'child_case', 'bounded', 'source_root', 'source_inputs',
                                                   'observe_profiles', 'observe_public', 'stage_evidence') if name in namespace}
                stack.enter_context(mock.patch.dict(namespace, banned))
            members, proof = self.fixture(diagnostics=True)
            projected = reader.project_hermes(members, 'hermes-4cd-source-linux', helpers, sources)
            self.assertTrue(projected['source_preparation_present'])
            self.assertTrue(projected['source_refusal_present'])
            self.assertEqual(forbidden, [])
        self.assertEqual(sys.path, old_path)
        self.assertEqual(sys.dont_write_bytecode, old_bytecode)

    def test_all_five_definition_pins_precede_any_import(self):
        for relative, (size, _hash) in reader.HERMES_DEFINITIONS.items():
            with self.subTest(path=relative), mock.patch.dict(reader.HERMES_DEFINITIONS, {relative: (size, '0' * 64)}), \
                 mock.patch.object(reader.runpy, 'run_path') as run:
                with self.assertRaises(ValueError):
                    reader.hermes_definitions()
                run.assert_not_called()

    def test_parser_mode_is_private_typed_and_missing_grok_member_remains_refused(self):
        raw = self.packed({'research.zip': b'opaque'})
        for mode in (0, 1, None, 'false'):
            with self.subTest(mode=mode), self.assertRaises(ValueError):
                reader.unpack(raw, {'research.zip'}, exact_members=mode)
        grok = FixedOriginalTests()
        members, _, frame = grok.fixture()
        members.pop('a.ledger.json')
        body = grok.archive(members)
        with grok.pins(body, frame), self.assertRaises(ValueError):
            reader.audit(body)
        members, _ = self.fixture()
        members['unknown-member'] = b'PRIVATE'
        body = self.packed(members)
        with self.pinned(body), self.assertRaises(ValueError):
            reader.audit_hermes(body, 'hermes-4cd-source-linux')

    def test_wrong_hermes_outer_identity_precedes_definition_loader(self):
        with mock.patch.object(reader, 'hermes_definitions') as load:
            with self.assertRaises(ValueError):
                reader.audit_hermes(b'wrong', 'hermes-4cd-source-linux')
            load.assert_not_called()

    def test_source_runtime_os_cleanup_and_qualification_drift_refuse(self):
        mutations = [
            ('source', dict(commit='0' * 40, tree=reader.HERMES_TREE, dirty='')),
            ('source_after', dict(commit=reader.HERMES_HEAD, tree=reader.HERMES_TREE, dirty='modified')),
            ('hermes_source_after', dict(commit=reader.HERMES_PIN, tree='0' * 40, dirty='')),
            ('run_id', 'other'), ('run_attempt', 1), ('runner_os', 'macOS'), ('runtime_distribution', 'bundled'),
            ('status', 'complete'), ('cleanup', 'completed'), ('lifecycle_exit', True), ('lifecycle_exit', '0'),
            ('lifecycle_exit', -(1 << 32) - 1), ('lifecycle_exit', 1 << 32),
            ('nonce', 'bad'), ('capture_complete', True), ('source_bootstrap_recovery_qualified', 0),
        ]
        for key, value in mutations:
            with self.subTest(key=key, value=value):
                members, proof = self.fixture()
                proof[key] = value
                self.update_proof(members, proof)
                body = self.packed(members)
                with self.pinned(body), self.assertRaises((ValueError, RuntimeError)):
                    reader.audit_hermes(body, 'hermes-4cd-source-linux')

    def test_failed_original_nullable_or_zero_reported_exit_does_not_infer_native_success(self):
        for reported in (None, 0):
            with self.subTest(reported=reported):
                members, proof = self.fixture()
                proof['lifecycle_exit'] = reported
                if reported is None:
                    for name in tuple(members):
                        if name.startswith(('guard-a.', 'lifecycle.')):
                            members.pop(name)
                else:
                    members['lifecycle.stdout'] = b''
                    members['lifecycle.stderr'] = b''
                self.update_proof(members, proof)
                body = self.packed(members)
                with self.pinned(body):
                    result = reader.audit_hermes(body, 'hermes-4cd-source-linux')
                self.assertEqual(result['lifecycle_exit'], reported)
                self.assertIs(type(result['lifecycle_exit']), type(reported))
                self.assertTrue(result['original_failed'])
                self.assertFalse(result['native_qualified'])
                self.assertTrue(all(value is False for value in result['qualification'].values()))
        members, proof = self.fixture()
        proof.pop('lifecycle_exit')
        self.update_proof(members, proof)
        body = self.packed(members)
        with self.pinned(body), self.assertRaises(ValueError):
            reader.audit_hermes(body, 'hermes-4cd-source-linux')

    def test_macos_preobserver_absolute_link_refusal_has_no_invented_commands(self):
        case = 'hermes-4cd-source-macos'
        members, proof = self.fixture(case)
        for name in tuple(members):
            if name.startswith(('guard-a.', 'lifecycle.')):
                members.pop(name)
        proof['lifecycle_exit'] = None
        refusal = dict(schema_version=1, measurement_only=True, admission_qualified=False, nonce=proof['nonce'],
                       stage='parent-before-observer', operation='absolute-link-target',
                       member='install/PRIVATE_LINK', member_sha256=reader.digest(b'install/PRIVATE_LINK'),
                       hash_encoding='utf-8-surrogatepass', integer_saturated=False,
                       metadata=dict(device=1, inode=2, mode=0o120777, links=1, bytes=42,
                                     uid=1001, gid=1001, attributes=None, reparse_tag=None),
                       bytes_read=None, errno=None, winerror=None, target_sha256='b' * 64, target_bytes=42)
        members['source-payload-refusal.json'] = json.dumps(refusal).encode()
        self.update_proof(members, proof)
        body = self.packed(members)
        with self.pinned(body, case):
            result = reader.audit_hermes(body, case)
        self.assertEqual(result['runner_os'], 'macOS')
        self.assertIsNone(result['lifecycle_exit'])
        self.assertEqual(result['command_status_count'], 0)
        self.assertEqual(result['observer_stdout_state'], 'empty')
        self.assertEqual(result['source_refusal']['operation'], 'absolute-link-target')
        self.assertFalse(result['native_qualified'])
        self.assertNotIn(b'PRIVATE', reader.encode(result))

    def test_primary_error_is_optional_but_present_value_remains_bounded(self):
        for reported in (None, 0, 1):
            with self.subTest(reported=reported):
                members, proof = self.fixture()
                proof.pop('error')
                proof['lifecycle_exit'] = reported
                self.update_proof(members, proof)
                body = self.packed(members)
                with self.pinned(body):
                    result = reader.audit_hermes(body, 'hermes-4cd-source-linux')
                self.assertFalse(result['primary_error_present'])
                self.assertIsNone(result['primary_error_kind'])
                self.assertEqual(result['runtime_error_kind'], 'RuntimeError')
                self.assertFalse(result['native_qualified'])
        for malformed in (None, True, 0, 'x' * 401):
            with self.subTest(primary=malformed):
                members, proof = self.fixture()
                proof['error'] = malformed
                self.update_proof(members, proof)
                body = self.packed(members)
                with self.pinned(body), self.assertRaises(ValueError):
                    reader.audit_hermes(body, 'hermes-4cd-source-linux')

    def test_failed_cleanup_is_retained_without_settlement_success(self):
        members, proof = self.fixture()
        proof.update(cleanup='failed', cleanup_error='PermissionError')
        self.update_proof(members, proof)
        body = self.packed(members)
        with self.pinned(body):
            result = reader.audit_hermes(body, 'hermes-4cd-source-linux')
        self.assertEqual(result['cleanup_state'], 'failed')
        self.assertEqual(result['cleanup_error_kind'], 'PermissionError')
        self.assertNotIn('cleanup_complete', result)
        self.assertFalse(result['native_qualified'])

    def test_full_manifest_and_nested_command_stream_join_refuse_rewriting(self):
        for stage in ('manifest', 'nested', 'missing-stream'):
            with self.subTest(stage=stage):
                members, proof = self.fixture()
                if stage == 'manifest':
                    proof['streams']['lifecycle.stderr']['sha256'] = '0' * 64
                    members['profile-admission.json'] = json.dumps(proof).encode()
                elif stage == 'nested':
                    status = json.loads(members['guard-a.status.json'])
                    status['streams']['stdout']['sha256'] = '0' * 64
                    members['guard-a.status.json'] = json.dumps(status).encode()
                    self.update_proof(members, proof)
                else:
                    members.pop('guard-a.stdout')
                    self.update_proof(members, proof)
                body = self.packed(members)
                with self.pinned(body), self.assertRaises((ValueError, KeyError)):
                    reader.audit_hermes(body, 'hermes-4cd-source-linux')

    def test_missing_optional_diagnostics_and_no_commands_do_not_invent_prefix(self):
        members, proof = self.fixture()
        for name in tuple(members):
            if name.startswith('guard-a'):
                members.pop(name)
        members['lifecycle.stdout'] = b''
        self.update_proof(members, proof)
        body = self.packed(members)
        with self.pinned(body):
            result = reader.audit_hermes(body, 'hermes-4cd-source-linux')
        self.assertEqual(result['command_status_count'], 0)
        for name in ('census_present', 'source_refusal_present', 'source_delta_present', 'source_preparation_present',
                     'observer_locator_present'):
            self.assertFalse(result[name])

    def test_optional_schema_nonce_and_parent_joins_refuse_drift(self):
        cases = (
            ('source-payload-refusal.json', 'nonce', 'b' * 64),
            ('source-payload-refusal.json', 'extra', 'PRIVATE'),
            ('source-user-delta.json', 'total_changes', True),
            ('source-user-delta.json', 'nonce', 'b' * 64),
            ('source-user-preparation.json', 'nonce', 'b' * 64),
            ('bootstrap-census-request.json', 'nonce', 'b' * 64),
        )
        for name, key, value in cases:
            with self.subTest(member=name, key=key):
                members, proof = self.fixture(diagnostics=True)
                payload = json.loads(members[name])
                payload[key] = value
                members[name] = json.dumps(payload, sort_keys=True, separators=(',', ':')).encode() + b'\n'
                self.update_proof(members, proof)
                body = self.packed(members)
                with self.pinned(body), self.assertRaises((ValueError, RuntimeError)):
                    reader.audit_hermes(body, 'hermes-4cd-source-linux')
        for mutation in ('delta-parent', 'preparation-parent', 'census-request'):
            with self.subTest(join=mutation):
                members, proof = self.fixture(diagnostics=True)
                if mutation == 'delta-parent':
                    proof['runtime_after']['user'] = dict(sha256='9' * 64, entries=5)
                elif mutation == 'preparation-parent':
                    proof.pop('source_user_preparation')
                else:
                    members.pop('bootstrap-census-request.json')
                self.update_proof(members, proof)
                body = self.packed(members)
                with self.pinned(body), self.assertRaises((ValueError, RuntimeError)):
                    reader.audit_hermes(body, 'hermes-4cd-source-linux')

    def test_optional_stdout_locator_never_suppresses_other_joined_diagnostics(self):
        valid_members, _ = self.fixture(diagnostics=True)
        valid = json.loads(valid_members['lifecycle.stdout'])
        invalid_nonce = dict(valid, nonce='b' * 64)
        invalid_head = dict(valid, source_head='0' * 40)
        invalid_caller = dict(valid, caller=dict(function='PRIVATE', function_line=1, line=1))
        unknown_field = dict(valid, extra='PRIVATE')
        payloads = (
            ('empty', b''),
            ('unvalidated', b'PRIVATE ordinary native stdout\n'),
            ('unvalidated', b'PRIVATE' * 400),
            ('unvalidated', b'{"broken":'),
            ('unvalidated', b'\xff'),
            ('unvalidated', json.dumps(valid, indent=2).encode()),
            ('unvalidated', json.dumps(invalid_nonce, sort_keys=True, separators=(',', ':')).encode() + b'\n'),
            ('unvalidated', json.dumps(invalid_head, sort_keys=True, separators=(',', ':')).encode() + b'\n'),
            ('unvalidated', json.dumps(invalid_caller, sort_keys=True, separators=(',', ':')).encode() + b'\n'),
            ('unvalidated', json.dumps(unknown_field, sort_keys=True, separators=(',', ':')).encode() + b'\n'),
            ('validated', valid_members['lifecycle.stdout']),
        )
        for state, payload in payloads:
            with self.subTest(state=state, size=len(payload)):
                members, proof = self.fixture(diagnostics=True)
                members['lifecycle.stdout'] = payload
                self.update_proof(members, proof)
                body = self.packed(members)
                with self.pinned(body):
                    result = reader.audit_hermes(body, 'hermes-4cd-source-linux')
                self.assertEqual(result['observer_stdout_state'], state)
                self.assertEqual(result['observer_locator_present'], state == 'validated')
                self.assertEqual('observer_locator' in result, state == 'validated')
                self.assertEqual(result['traceback_count'], 1)
                self.assertTrue(result['source_refusal_present'])
                self.assertTrue(result['source_delta_present'])
                self.assertFalse(result['native_qualified'])
                self.assertNotIn(b'PRIVATE', reader.encode(result))

    def test_traceback_names_and_lines_must_belong_to_pinned_source(self):
        members, proof = self.fixture()
        members['lifecycle.stderr'] = b'  File "/PRIVATE/hermes-profile-admission.py", line 1, in PRIVATE\n'
        self.update_proof(members, proof)
        body = self.packed(members)
        with self.pinned(body), self.assertRaises(ValueError):
            reader.audit_hermes(body, 'hermes-4cd-source-linux')

    def test_hermes_cli_refusal_emits_case_without_any_private_projection(self):
        case = 'hermes-4cd-source-linux'
        with tempfile.TemporaryDirectory() as root:
            path = Path(root) / 'archive.zip'
            path.write_bytes(b'wrong')
            stdout = io.StringIO()
            with mock.patch.object(sys, 'argv', ['reader', case, str(path)]), contextlib.redirect_stdout(stdout):
                self.assertEqual(reader.main(), 1)
            self.assertEqual(json.loads(stdout.getvalue()), dict(kind='fixed-original-readout', case=case,
                                                                refused=True, native_qualified=False))


class GrokCurrentOriginalTests(unittest.TestCase):
    def command(self, label, argv, cwd, stdout=b'', stderr=b'', exit_code=0, *, child=False):
        row = dict(probe=label, argv=argv, cwd=cwd, error=None, exit=exit_code, elapsed_seconds=0.02)
        for key, raw in (('stdout', stdout), ('stderr', stderr)):
            row.update({key + '_file': label + '.' + key + '.log', key + '_bytes': len(raw), key + '_sha256': reader.digest(raw)})
            if child:
                row.update({key: raw.decode('utf-8', 'replace'), key + '_base64': base64.b64encode(raw).decode()})
        return row

    def fixture(self):
        source, output = '/fixture/PRIVATE_SOURCE', '/fixture/PRIVATE_OUTPUT'
        installer, probe = '/fixture/PRIVATE_BUILD/agentplugins', '/fixture/PRIVATE_BUILD/probe'
        unit = 'uap-grok-noauth-' + 'a' * 32 + '.service'
        env = dict(HOME=output + '/home', PATH='/usr/bin:/bin', LANG='C.UTF-8', LC_ALL='C.UTF-8', NO_COLOR='1',
            TERM='dumb', TMPDIR=output + '/tmp', RUNNER_TEMP=output + '/tmp', GIT_CONFIG_GLOBAL='/dev/null',
            GIT_CONFIG_NOSYSTEM='1', GIT_TERMINAL_PROMPT='0', PYTHONDONTWRITEBYTECODE='1', GITHUB_ACTIONS='true',
            RUNNER_ENVIRONMENT='github-hosted', AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED='1',
            EXPECTED_COMMIT=reader.GROK_CURRENT_HEAD, GITHUB_RUN_ID=reader.GROK_CURRENT_RUN,
            GITHUB_RUN_ATTEMPT='1', GITHUB_EVENT_NAME='pull_request')
        argv = ['/usr/bin/sudo', '-n', '/usr/bin/systemd-run', '--unit=' + unit, '--wait', '--pipe', '--collect', '--quiet',
            '--service-type=exec', '--expand-environment=no', '--uid=1001', '--gid=1001', '--property=Delegate=yes',
            '--property=NoNewPrivileges=yes', '--property=KillMode=control-group', '--property=RuntimeMaxSec=899s',
            '--property=TimeoutStopSec=30s', '--working-directory=' + output, '/usr/bin/env', '-i',
            *(key + '=' + env[key] for key in sorted(env)), '/usr/bin/python3.12', '-B',
            source + '/scripts/grok_noauth_call_e2e.py', '--installer', installer, '--probe', probe, '--output', output + '/child']
        rows, captures = [], {}
        def add(label, arguments, cwd, stdout=b'', stderr=b'', exit_code=0):
            rows.append(self.command(label, arguments, cwd, stdout, stderr, exit_code))
            for key, raw in (('stdout', stdout), ('stderr', stderr)):
                captures[label + '.' + key + '.log'] = base64.b64encode(raw).decode()
        queries = (('commit', ['rev-parse', 'HEAD'], (reader.GROK_CURRENT_HEAD + '\n').encode()),
                   ('tree', ['rev-parse', 'HEAD^{tree}'], (reader.GROK_CURRENT_TREE + '\n').encode()),
                   ('clean', ['status', '--porcelain', '--untracked-files=normal'], b''))
        for name, args, raw in queries:
            add('grok-source-before-' + name, ['/usr/bin/git', *args], source, raw)
        query = ['/usr/bin/systemctl', '--no-pager', '--no-ask-password', 'show',
                 '--property=LoadState,ActiveState,ControlGroup', unit]
        add('grok-noauth-service-absent', query, output, b'LoadState=not-found\nActiveState=inactive\nControlGroup=\n')
        add('grok-noauth-systemd-version', ['/usr/bin/systemd-run', '--version'], output, b'systemd 255 (255.4)\n')
        trace = (b'Traceback (most recent call last):\n'
                 b'  File "/fixture/PRIVATE_SOURCE/scripts/grok_project_protocol_e2e.py", line 138, in run\n'
                 b'ValueError: command failed: repair-enable-owned-full-id\n')
        add('runtime-test', argv, output, b'PRIVATE_PARENT_OUTPUT\n', trace, 1)
        add('grok-noauth-service-state', query, output, b'LoadState=not-found\nActiveState=inactive\nControlGroup=\n')
        for name, args, raw in queries:
            add('grok-source-after-' + name, ['/usr/bin/git', *args], source, raw)
        cwd = '/fixture/PRIVATE_PROJECT'
        dto = dict(schema_version=1, command='repair', result='failure', data=dict(result=dict(mutated=False,
            activation=dict(activation='prepared')), private='PRIVATE_CONFIG'))
        selected = self.command('repair-enable-owned-full-id',
            [installer, 'repair', 'PRIVATE_SELECTOR', '--scope', 'project', '--project-root', cwd, '--target', 'grok',
             '--format', 'json', '--confirm-plugin-enable', '--yes'], cwd, json.dumps(dto).encode(),
            b'agentplugins: fixed source refusal\nPRIVATE_SECRET\n', 1, child=True)
        negative = self.command('untrusted-doctor', ['/fixture/PRIVATE_BIN/grok', 'mcp', 'doctor', '--json'],
                                cwd, b'PRIVATE_NATIVE_OUTPUT', b'', 1, child=True)
        evidence = dict(schema_version=1, source_commit=reader.GROK_CURRENT_HEAD, source_tree=reader.GROK_CURRENT_TREE,
            github_run_id=reader.GROK_CURRENT_RUN, github_run_attempt='1', github_event='pull_request', platform='Linux-x86_64',
            commands=[negative, selected], stages=[], scripted_local_provider=True, installer_sha256='1' * 64, probe_sha256='2' * 64,
            error='ValueError: command failed: repair-enable-owned-full-id',
            failed_command=dict(label='repair-enable-owned-full-id', exit=1, capture_complete=True, protected_originals_unchanged=True))
        evidence.update(dict.fromkeys(reader.GROK_FALSE, False))
        receipt = dict(schema_version=1, kind='grok-delegated-service', case='grok-noauth',
            source_commit=reader.GROK_CURRENT_HEAD, source_tree=reader.GROK_CURRENT_TREE, installer_sha256='1' * 64,
            probe_sha256='2' * 64, python_sha256='3' * 64, original_start_monotonic_ns=1000000000, native_passed=False,
            delegated_service=dict(unit=unit, uid=1001, gid=1001, closed=True, forced_stop=False, forced_kill=False,
                original_start_monotonic_ns=1000000000,
                system_executables={'/usr/bin/' + name: '4' * 64 for name in ('sudo', 'systemd-run', 'systemctl', 'env')}),
            commands=rows, streams_base64=captures)
        return evidence, receipt

    def members(self, evidence, receipt):
        body = json.dumps(evidence).encode()
        receipt = dict(receipt, evidence_bytes=len(body), evidence_sha256=reader.digest(body))
        return {'evidence.json': body, 'delegation.json': json.dumps(receipt).encode()}

    def facts(self):
        return {'grok_project_protocol_e2e.py': {('run', 138)}}, {'fixed source refusal'}

    def read(self, evidence, receipt):
        return reader.project_current_grok(self.members(evidence, receipt), self.facts())

    def raw_stream(self, row, name, raw):
        row.update({name: raw.decode('utf-8', 'replace'), name + '_base64': base64.b64encode(raw).decode(),
                    name + '_bytes': len(raw), name + '_sha256': reader.digest(raw)})

    def packed(self, members):
        output = io.BytesIO()
        with zipfile.ZipFile(output, 'w', compression=zipfile.ZIP_DEFLATED) as archive:
            for name, body in members.items():
                archive.writestr(name, body)
        return output.getvalue()

    def test_original_negative_selects_last_record_not_first_nonzero(self):
        value = self.read(*self.fixture())
        self.assertEqual((value['parent_commands'], value['parent_stream_joins'], value['child_stream_joins']), (10, 20, 4))
        self.assertEqual(value['failed_command']['label'], 'repair-enable-owned-full-id')
        self.assertEqual(value['failed_command']['public_dto'], dict(state='validated-public-fields',
                         command='repair', result='failure', mutated=False, activation='prepared'))
        self.assertEqual(value['known_guard_messages'], ['fixed source refusal'])
        self.assertEqual(value['traceback_frames'], [dict(file='grok_project_protocol_e2e.py', line=138, function='run')])
        self.assertFalse(value['native_qualified'])
        self.assertTrue(value['original_failed'])
        self.assertFalse(value['source_code_imported'])
        encoded = reader.encode(value)
        self.assertNotIn(b'PRIVATE', encoded)
        self.assertNotIn(b'fixture/', encoded)

    def test_source_flags_roles_service_and_parent_record_mutations_refuse(self):
        mutations = (
            lambda e, r: e.update(source_commit='a' * 40),
            lambda e, r: e.update(github_run_id='999'),
            lambda e, r: e.update(actual_model_used=True),
            lambda e, r: e.update(passed=True),
            lambda e, r: e.update(scripted_local_provider=False),
            lambda e, r: e.update(installer_sha256='9' * 64),
            lambda e, r: r.update(native_passed=True),
            lambda e, r: r['delegated_service'].update(closed=False),
            lambda e, r: r['delegated_service'].update(forced_stop=True),
            lambda e, r: r['delegated_service'].update(uid=True),
            lambda e, r: r['delegated_service'].update(original_start_monotonic_ns=2),
            lambda e, r: r['commands'][5].update(argv=['/usr/bin/true']),
            lambda e, r: r['commands'][5]['argv'].__setitem__(15, '--property=RuntimeMaxSec=901s'),
            lambda e, r: r['commands'][5]['argv'].__setitem__(10, '--uid=0'),
            lambda e, r: r['commands'][5].update(exit=0),
            lambda e, r: r['commands'][0].update(error='capture fault'),
            lambda e, r: r['commands'][7].update(cwd='/OTHER_SOURCE'),
            lambda e, r: r['commands'][0].update(elapsed_seconds=float('inf')),
            lambda e, r: r['commands'].append(r['commands'][0]),
        )
        for mutate in mutations:
            with self.subTest(mutate=mutations.index(mutate)):
                e, r = self.fixture()
                mutate(e, r)
                with self.assertRaises((ValueError, KeyError)):
                    self.read(e, r)

    def test_all_original_parent_and_child_stream_joins_precede_projection(self):
        mutations = (
            lambda e, r: r['streams_base64'].__setitem__('grok-source-before-clean.stdout.log', 'YQ=='),
            lambda e, r: r['streams_base64'].update(unknown=''),
            lambda e, r: r['commands'][5].update(stderr_sha256='0' * 64),
            lambda e, r: e['commands'][0].update(stdout='CHANGED'),
            lambda e, r: e['commands'][0].update(stdout_bytes=True),
            lambda e, r: e['commands'][0].update(stdout_base64='!!!!'),
            lambda e, r: e['commands'][0].update(stdout_file='other.stdout.log'),
            lambda e, r: e['commands'][0].update(stderr_sha256='a' * 64),
            lambda e, r: e['commands'][0].update(elapsed_seconds=float('nan')),
            lambda e, r: e['commands'][0].update(probe='foreign'),
            lambda e, r: e['commands'].append(e['commands'][0]),
        )
        for index, mutate in enumerate(mutations):
            with self.subTest(index=index):
                e, r = self.fixture()
                mutate(e, r)
                with self.assertRaises((ValueError, KeyError)):
                    self.read(e, r)

    def test_failure_identity_does_not_infer_expected_exit_policy(self):
        e, r = self.fixture()
        for key, value in (('label', 'untrusted-doctor'), ('exit', 0), ('capture_complete', False),
                           ('protected_originals_unchanged', 1), ('extra', False)):
            with self.subTest(key=key):
                selected = dict(e['failed_command'])
                e['failed_command'][key] = value
                with self.assertRaises(ValueError):
                    self.read(e, r)
                e['failed_command'] = selected
        e['error'] = 'ValueError: PRIVATE_LATER_CLEANUP_ERROR'
        value = self.read(e, r)
        self.assertFalse(value['failed_command']['primary_error_matches'])
        self.assertEqual(value['primary_error_code'], 'unclassified')
        self.assertNotIn(b'PRIVATE', reader.encode(value))

    def test_partial_before_first_child_command_is_still_a_failed_original(self):
        e, r = self.fixture()
        e['commands'] = []
        del e['failed_command']
        e['error'] = 'OSError: PRIVATE_ACQUISITION_ERROR'
        value = self.read(e, r)
        self.assertFalse(value['failed_command_present'])
        self.assertEqual(value['primary_error_kind'], 'OSError')
        self.assertEqual(value['child_commands'], 0)
        self.assertFalse(value['native_qualified'])

    def test_optional_dto_unvalidated_does_not_suppress_other_diagnostics(self):
        bodies = (b'', b'ordinary console output PRIVATE', b'{"schema_version":1,"schema_version":1}',
                  json.dumps(dict(schema_version=1, command='repair', result='failure', data={})).encode(),
                  json.dumps(dict(schema_version=1, command='repair', result='failure',
                                  data=dict(result=dict(mutated=1, activation=dict(activation='prepared'))))).encode())
        for body in bodies:
            with self.subTest(body=body):
                e, r = self.fixture()
                self.raw_stream(e['commands'][-1], 'stdout', body)
                value = self.read(e, r)
                self.assertEqual(value['failed_command']['public_dto']['state'], 'empty' if not body else 'unvalidated')
                self.assertEqual(value['known_guard_messages'], ['fixed source refusal'])
                self.assertEqual(value['traceback_count'], 1)
                self.assertFalse(value['native_qualified'])
        e, r = self.fixture()
        e['commands'][-1]['argv'][0] = '/FOREIGN/agentplugins'
        self.assertEqual(self.read(e, r)['failed_command']['public_dto'], dict(state='unvalidated-role'))

    def test_trace_lines_are_source_bound_and_limited_without_private_projection(self):
        e, r = self.fixture()
        row = e['commands'][-1]
        self.raw_stream(row, 'stderr', b'  File "/PRIVATE/grok_project_protocol_e2e.py", line 139, in run\n')
        with self.assertRaises(ValueError):
            self.read(e, r)
        self.raw_stream(row, 'stderr', b'  File "/PRIVATE/grok_project_protocol_e2e.py", line 138, in run\n' * 10)
        value = self.read(e, r)
        self.assertEqual((value['traceback_count'], len(value['traceback_frames'])), (11, 8))
        self.assertTrue(value['traceback_truncated'])
        self.assertNotIn(b'PRIVATE', reader.encode(value))

    def test_event_bytes_and_full_list_must_join(self):
        e, r = self.fixture()
        raw = b'{"method":"PRIVATE_EVENT"}\n'
        e.update(events_base64=base64.b64encode(raw).decode(), events_sha256=reader.digest(raw),
                 all_original_events=[dict(method='PRIVATE_EVENT')])
        self.assertFalse(self.read(e, r)['native_qualified'])
        e['all_original_events'] = []
        with self.assertRaises(ValueError):
            self.read(e, r)
        del e['events_sha256']
        with self.assertRaises(ValueError):
            self.read(e, r)

    def test_whole_archive_identity_and_exact_two_member_crc_are_required(self):
        members = self.members(*self.fixture())
        original = self.packed(members)
        with mock.patch.object(reader, 'GROK_CURRENT_SIZE', len(original)), \
             mock.patch.object(reader, 'GROK_CURRENT_HASH', reader.digest(original)), \
             mock.patch.object(reader, 'grok_current_sources', return_value=self.facts()):
            self.assertEqual(reader.audit_current_grok(original)['member_count'], 2)
            with self.assertRaises(ValueError):
                reader.audit_current_grok(original + b'x')
        for changed in (dict(members, extra=b''), {'evidence.json': members['evidence.json']}):
            raw = self.packed(changed)
            with mock.patch.object(reader, 'GROK_CURRENT_SIZE', len(raw)), \
                 mock.patch.object(reader, 'GROK_CURRENT_HASH', reader.digest(raw)), \
                 mock.patch.object(reader, 'grok_current_sources', return_value=self.facts()):
                with self.assertRaises(ValueError):
                    reader.audit_current_grok(raw)
        raw = bytearray(self.packed(members))
        with zipfile.ZipFile(io.BytesIO(raw)) as archive:
            info = archive.infolist()[0]
            offset = info.header_offset + 30 + len(info.filename)
        raw[offset + 4] ^= 1
        with mock.patch.object(reader, 'GROK_CURRENT_SIZE', len(raw)), \
             mock.patch.object(reader, 'GROK_CURRENT_HASH', reader.digest(raw)), \
             mock.patch.object(reader, 'grok_current_sources', return_value=self.facts()):
            with self.assertRaises((ValueError, zipfile.BadZipFile, zlib.error)):
                reader.audit_current_grok(bytes(raw))

    def test_source_fact_capture_never_imports_or_executes_the_pinned_programs(self):
        with mock.patch.object(reader.runpy, 'run_path', side_effect=AssertionError('runtime import forbidden')), \
             mock.patch('subprocess.Popen', side_effect=AssertionError('process forbidden')), \
             mock.patch('subprocess.run', side_effect=AssertionError('process forbidden')), \
             mock.patch('socket.socket', side_effect=AssertionError('socket forbidden')), \
             mock.patch('threading.Thread.start', side_effect=AssertionError('thread forbidden')), \
             mock.patch('os.system', side_effect=AssertionError('shell forbidden')):
            locations, guards = reader.grok_current_sources()
        self.assertIn(('run', 138), locations['grok_project_protocol_e2e.py'])
        self.assertIn('grok Project enablement requires verified recovery of native startup effects and original parent identity', guards)
        self.assertIn('selected target does not support explicit Project plugin enablement', guards)
        self.assertEqual(len(locations), 7)

    def test_all_six_fixed_cli_inputs_refuse_wrong_archives_without_private_output(self):
        cases = (reader.CASE, *reader.HERMES_CASES, reader.GROK_CURRENT_CASE, reader.ANTIG_CASE)
        self.assertEqual(len(set(cases)), 6)
        with tempfile.TemporaryDirectory() as root:
            path = Path(root) / 'PRIVATE_ARCHIVE.zip'
            path.write_bytes(b'PRIVATE_INVALID_ARCHIVE')
            for case in cases:
                with self.subTest(case=case):
                    output = io.StringIO()
                    with mock.patch.object(sys, 'argv', ['reader', case, str(path)]), contextlib.redirect_stdout(output):
                        self.assertEqual(reader.main(), 1)
                    self.assertEqual(json.loads(output.getvalue()), dict(kind='fixed-original-readout', case=case,
                                                                        refused=True, native_qualified=False))



    def test_actual_public_preview_argv_and_exact_cli_stderr_prefix(self):
        for label, extra in (('repair-no-enable-consent', ['--yes']),
                             ('repair-enable-dry-run', ['--confirm-plugin-enable', '--dry-run', '--yes'])):
            with self.subTest(label=label):
                e, r = self.fixture()
                original = e['commands'][-1]
                argv = original['argv'][:11] + extra
                e['commands'][-1] = self.command(label, argv, original['cwd'],
                    base64.b64decode(original['stdout_base64']), b'agentplugins: fixed source refusal\n', 1, child=True)
                e['failed_command']['label'] = label
                e['error'] = 'ValueError: command failed: ' + label
                parent = r['commands'][5]
                raw = base64.b64decode(r['streams_base64'][parent['stderr_file']]).replace(
                    b'repair-enable-owned-full-id', label.encode())
                parent.update(stderr_bytes=len(raw), stderr_sha256=reader.digest(raw))
                r['streams_base64'][parent['stderr_file']] = base64.b64encode(raw).decode()
                result = self.read(e, r)
                self.assertEqual(result['failed_command']['public_dto']['state'], 'validated-public-fields')
                self.assertEqual(result['known_guard_messages'], ['fixed source refusal'])
                self.assertTrue(result['failed_command']['primary_error_matches'])
                self.assertFalse(result['native_qualified'])
        for prefix in (b'Error: ', b'PRIVATE: ', b'agentplugins:x ', b'\x1b[1magentplugins: '):
            with self.subTest(prefix=prefix):
                e, r = self.fixture()
                self.raw_stream(e['commands'][-1], 'stderr', prefix + b'fixed source refusal\n')
                value = self.read(e, r)
                self.assertEqual(value['known_guard_messages'], [])
                self.assertNotIn(b'PRIVATE', reader.encode(value))
        self.assertEqual(reader.GROK_STDERR_PREFIX, 'agentplugins: ')

class AntigravityFixedOriginalTests(unittest.TestCase):
    """Synthetic nested receipts; no native execution or actual-original qualification."""

    def write_json(self, members, name, value):
        members[name] = json.dumps(value, sort_keys=True).encode() + b'\n'

    def reindex(self, members, manifest):
        for row in manifest['slots'] + manifest['public_slots']:
            for entry in row['files']:
                raw = members[entry['name']]
                entry.update(bytes=len(raw), sha256=reader.digest(raw))
        self.write_json(members, 'manifest.json', manifest)

    def fixture(self):
        members = {}
        identity = dict(path=[7, 19, 0o40700, 1001, 1001], held=[7, 19, 0o40700, 1001, 1001])
        manifest = dict(schema_version=2, source_commit=reader.ANTIG_SOURCE, source_tree=reader.ANTIG_TREE,
            native_sha256='1' * 64, helper_sha256='2' * 64, installer_sha256='3' * 64,
            expected_slots=14, public_expected_slots=23, evidence_root_identity=identity,
            capture_complete=False, harness_complete=False, fixtures_retained=True,
            public_capture_complete=False, public_effects_verified=False, slots=[], public_slots=[])
        manifest.update(dict.fromkeys(reader.ANTIG_FLAGS, False))
        dto = dict(schema_version=1, command='agentplugins repair', result='success', outcome='completed',
            scope='user', scope_selection_reason='explicit', actions=[dict(
                from_version='1.0.0', to_version='1.0.0', from_revision='b' * 40, from_digest='sha256:' + 'a' * 64,
                component_support=[dict(kind='skill', name='check', support='native'),
                                   dict(kind='mcp_server', name='uap_public_probe', support='native')],
                alias='uap-cline-public-probe', client_id='antigravity', installation_id='PRIVATE_INSTALLATION',
                binding_id='PRIVATE_BINDING', action='repair', package_digest='sha256:' + 'a' * 64, revision='b' * 40)],
            results=[dict(installation_id='PRIVATE_INSTALLATION', client_id='antigravity', operation='repair',
                          outcome='unchanged', manual_actions=[reader.ANTIG_MANUAL_ACTIONS[0]])])
        configs = {False: 'a' * 64, True: 'b' * 64}
        def pair(prefix, index, world, action, public, initial=None):
            root = '/fixture/uap-agy-direct-' + world
            selector = ('uap-cline-public-probe' if index >= 9 else 'uap-agy-native-smoke') if public else 'owned-native'
            request = dict(binary='/fixture/' + ('agentplugins' if public else 'agy'),
                binary_sha256=manifest['installer_sha256' if public else 'native_sha256'],
                root=root, selector=selector, action=action, source_commit=reader.ANTIG_SOURCE,
                source_tree=reader.ANTIG_TREE, deadline_unix_ns=1_800_000_000_000_000_000)
            stdout = json.dumps(dto).encode() if public and index == 11 else b'PRIVATE_OTHER_STREAM\n'
            receipt = {key: request[key] for key in (
                'source_commit', 'source_tree', 'root', 'selector', 'action', 'binary_sha256')}
            receipt.update(cleanup_complete=True, stdout_base64=base64.b64encode(stdout).decode(),
                           stdout_bytes=len(stdout), stdout_sha256=reader.digest(stdout))
            if public:
                stderr = b'PRIVATE_RECORDED_STDERR'
                receipt.update(exit=0, stderr_base64=base64.b64encode(stderr).decode(),
                               stderr_bytes=len(stderr), stderr_sha256=reader.digest(stderr))
            files = []
            for suffix, value in (('request', request), ('receipt', receipt)):
                name = prefix + str(index).zfill(2) + '.' + suffix + '.json'
                self.write_json(members, name, value)
                files.append(dict(name=name, bytes=0, sha256='0' * 64))
            if public:
                manifest['public_slots'].append(dict(world=world.removeprefix('public-'), action=action, files=files))
            else:
                before = configs[initial]
                if action in ('enable', 'disable'):
                    configs[initial] = reader.digest((str(initial) + str(index)).encode())
                manifest['slots'].append(dict(initial_enabled=initial, action=action, files=files,
                    effects_verified=True, config_before_sha256=before, config_after_sha256=configs[initial]))
        for index, (initial, action) in enumerate(reader.ANTIG_NATIVE_ACTIONS):
            pair('', index, 'native-enabled' if initial else 'native-disabled', action, False, initial)
        for index, (action, world) in enumerate(zip(reader.ANTIG_PUBLIC_ACTIONS, reader.ANTIG_PUBLIC_WORLDS)):
            pair('public-', index, 'public-' + world, action, True)
        self.reindex(members, manifest)
        return members, manifest, dto

    def archive(self, members, *, outer_name='receipts.zip'):
        helper = FixedOriginalTests()
        inner = helper.packed(members, zipfile.ZIP_STORED)
        return helper.packed({outer_name: inner})

    @contextlib.contextmanager
    def pinned(self, body):
        with mock.patch.object(reader, 'ANTIG_ARCHIVE_SIZE', len(body)), \
             mock.patch.object(reader, 'ANTIG_ARCHIVE_HASH', reader.digest(body)):
            yield

    def project(self, members):
        body = self.archive(members)
        with self.pinned(body):
            return reader.audit_current_antigravity(body)

    def change_receipt_stdout(self, members, manifest, dto):
        name = 'public-11.receipt.json'
        receipt = json.loads(members[name])
        stdout = json.dumps(dto).encode()
        receipt.update(stdout_base64=base64.b64encode(stdout).decode(),
                       stdout_bytes=len(stdout), stdout_sha256=reader.digest(stdout))
        self.write_json(members, name, receipt)
        self.reindex(members, manifest)

    def test_complete_fifty_three_members_thirty_eight_streams_and_two_components(self):
        members, _, _ = self.fixture()
        result = self.project(members)
        self.assertEqual(result['member_count'], 53)
        self.assertEqual(result['stream_joins'], 38)
        self.assertEqual((result['native_slots'], result['public_slots'], result['selected_public_index']), (14, 12, 11))
        self.assertEqual(result['selected_public_exit'], 0)
        self.assertTrue(result['original_failed'])
        self.assertFalse(result['native_qualified'])
        self.assertTrue(all(value is False for value in result['qualification'].values()))
        self.assertFalse(result['capture_complete'])
        self.assertFalse(result['public_effects_verified'])
        components = result['dto']['actions']['components']['rows']
        self.assertEqual([row['fields']['name']['known_value'] for row in components], ['check', 'uap_public_probe'])
        self.assertFalse(result['nonce_recorded'])
        self.assertFalse(result['runtime_argv_recorded'])
        self.assertEqual(result['source_derived_argv'][1:3], ['repair', 'uap-cline-public-probe'])
        encoded = reader.encode(result)
        self.assertLessEqual(len(encoded), 8192)
        self.assertNotIn(b'PRIVATE', encoded)
        self.assertNotIn(b'/fixture/', encoded)

    def test_wrong_whole_original_precedes_any_zip_parsing(self):
        with mock.patch.object(reader, 'unpack') as unpack:
            with self.assertRaises(ValueError):
                reader.audit_current_antigravity(b'wrong')
            unpack.assert_not_called()

    def test_exact_outer_and_inner_members_refuse_missing_unknown_or_swapped_roles(self):
        for mutation in ('outer', 'missing', 'unknown', 'swap'):
            with self.subTest(mutation=mutation):
                members, manifest, _ = self.fixture()
                outer_name = 'receipts.zip'
                if mutation == 'outer':
                    outer_name = 'other.zip'
                elif mutation == 'missing':
                    members.pop('public-11.receipt.json')
                elif mutation == 'unknown':
                    members['unexpected.json'] = b'PRIVATE'
                else:
                    members['public-11.request.json'], members['public-11.receipt.json'] = (
                        members['public-11.receipt.json'], members['public-11.request.json'])
                    self.reindex(members, manifest)
                body = self.archive(members, outer_name=outer_name)
                with self.pinned(body), self.assertRaises(ValueError):
                    reader.audit_current_antigravity(body)

    def test_closed_manifest_source_identity_qualification_and_partial_truth(self):
        mutations = [('schema_version', True), ('expected_slots', True), ('public_expected_slots', 12),
            ('source_commit', '0' * 40), ('source_tree', '0' * 40), ('helper_sha256', 'bad'),
            ('native_sha256', '4' * 64), ('installer_sha256', '4' * 64),
            ('fixtures_retained', False), ('capture_complete', True), ('harness_complete', True),
            ('public_capture_complete', True), ('public_effects_verified', True),
            ('native_mcp_call', 0), ('vendor_authentication', True), ('unexpected', 'PRIVATE')]
        for key, value in mutations:
            with self.subTest(key=key):
                members, manifest, _ = self.fixture()
                manifest[key] = value
                self.reindex(members, manifest)
                with self.assertRaises(ValueError):
                    self.project(members)

    def test_slot_counts_actions_worlds_effects_and_config_continuity_refuse(self):
        for mutation in ('native-count', 'public-count', 'native-action', 'public-action', 'world',
                         'effects', 'initial-type', 'digest-continuity', 'readonly-effect'):
            with self.subTest(mutation=mutation):
                members, manifest, _ = self.fixture()
                if mutation == 'native-count': manifest['slots'].pop()
                elif mutation == 'public-count': manifest['public_slots'].pop()
                elif mutation == 'native-action': manifest['slots'][0]['action'] = 'enable'
                elif mutation == 'public-action': manifest['public_slots'][11]['action'] = 'uap-cold-repeat'
                elif mutation == 'world': manifest['public_slots'][11]['world'] = 'saved'
                elif mutation == 'effects': manifest['slots'][0]['effects_verified'] = False
                elif mutation == 'initial-type': manifest['slots'][0]['initial_enabled'] = 0
                elif mutation == 'digest-continuity': manifest['slots'][1]['config_before_sha256'] = 'f' * 64
                else: manifest['slots'][0]['config_after_sha256'] = 'f' * 64
                self.reindex(members, manifest)
                with self.assertRaises(ValueError):
                    self.project(members)

    def test_all_request_receipt_fields_source_pins_selectors_deadlines_and_cleanup_join(self):
        mutations = (
            ('request', 'source_commit', '0' * 40), ('request', 'source_tree', '0' * 40),
            ('request', 'binary_sha256', '4' * 64), ('request', 'selector', 'foreign'),
            ('request', 'deadline_unix_ns', True), ('request', 'deadline_unix_ns', 0),
            ('request', 'deadline_unix_ns', 1 << 63), ('request', 'binary', 'relative'),
            ('request', 'extra', 'PRIVATE'), ('receipt', 'cleanup_complete', 1),
            ('receipt', 'source_commit', '0' * 40), ('receipt', 'root', '/fixture/uap-agy-direct-foreign'),
            ('receipt', 'exit', True), ('receipt', 'exit', 1), ('receipt', 'exit', 0.0),
            ('receipt', 'stdout_bytes', True), ('receipt', 'stdout_sha256', '0' * 64),
            ('receipt', 'stderr_sha256', '0' * 64), ('receipt', 'stdout_base64', '%%%%'),
            ('receipt', 'extra', 'PRIVATE'))
        for suffix, key, value in mutations:
            with self.subTest(suffix=suffix, key=key):
                members, manifest, _ = self.fixture()
                name = 'public-11.' + suffix + '.json'
                packet = json.loads(members[name])
                packet[key] = value
                self.write_json(members, name, packet)
                self.reindex(members, manifest)
                with self.assertRaises(ValueError):
                    self.project(members)

    def test_hashes_frames_and_native_streams_are_checked_outside_selected_public_slot(self):
        for mutation in ('manifest-hash', 'native-stream', 'earlier-public-stream', 'missing-field', 'trailing-frame'):
            with self.subTest(mutation=mutation):
                members, manifest, _ = self.fixture()
                if mutation == 'manifest-hash':
                    manifest['slots'][0]['files'][0]['sha256'] = '0' * 64
                    self.write_json(members, 'manifest.json', manifest)
                elif mutation == 'trailing-frame':
                    members['01.request.json'] += b'{}\n'
                    self.reindex(members, manifest)
                else:
                    name = '00.receipt.json' if mutation == 'native-stream' else 'public-00.receipt.json'
                    packet = json.loads(members[name])
                    if mutation == 'missing-field': packet.pop('stderr_sha256')
                    else: packet['stdout_sha256'] = '0' * 64
                    self.write_json(members, name, packet)
                    self.reindex(members, manifest)
                with self.assertRaises(ValueError):
                    self.project(members)

    def test_joined_root_substitution_and_binary_rebinding_refuse(self):
        for mutation in ('cold-root', 'same-root-different-world', 'binary'):
            with self.subTest(mutation=mutation):
                members, manifest, _ = self.fixture()
                request = json.loads(members['public-11.request.json'])
                receipt = json.loads(members['public-11.receipt.json'])
                if mutation == 'binary':
                    request['binary'] = '/fixture/foreign-installer'
                else:
                    root = '/fixture/uap-agy-direct-' + ('public-saved' if mutation == 'same-root-different-world' else 'replacement')
                    request['root'] = receipt['root'] = root
                self.write_json(members, 'public-11.request.json', request)
                self.write_json(members, 'public-11.receipt.json', receipt)
                self.reindex(members, manifest)
                with self.assertRaises(ValueError):
                    self.project(members)

    def test_dto_shape_differences_are_reported_without_weakening_native_verdict(self):
        for mutation in ('mcp-only', 'unsupported', 'extra-field', 'missing-result', 'boolean-schema', 'null-actions'):
            with self.subTest(mutation=mutation):
                members, manifest, dto = self.fixture()
                if mutation == 'mcp-only': dto['actions'][0]['component_support'].pop(0)
                elif mutation == 'unsupported': dto['actions'][0]['component_support'][0]['support'] = 'unsupported'
                elif mutation == 'extra-field': dto['PRIVATE_SECRET_KEY'] = '/PRIVATE/secret-token'
                elif mutation == 'missing-result': dto.pop('result')
                elif mutation == 'boolean-schema': dto['schema_version'] = True
                else: dto['actions'] = None
                self.change_receipt_stdout(members, manifest, dto)
                result = self.project(members)
                self.assertTrue(result['original_failed'])
                self.assertFalse(result['native_qualified'])
                self.assertNotIn(b'PRIVATE', reader.encode(result))
                if mutation == 'mcp-only':
                    self.assertEqual(result['dto']['actions']['components']['shape']['count'], 1)
                elif mutation == 'unsupported':
                    self.assertEqual(result['dto']['actions']['components']['rows'][0]['fields']['support']['known_value'], 'unsupported')
                elif mutation == 'extra-field':
                    self.assertEqual(result['dto']['envelope']['unknown_key_count'], 1)
                elif mutation == 'missing-result':
                    self.assertFalse(result['dto']['envelope']['fields']['result']['present'])
                elif mutation == 'boolean-schema':
                    self.assertEqual(result['dto']['envelope']['fields']['schema_version']['type'], 'boolean')
                else:
                    self.assertEqual(result['dto']['actions']['shape']['type'], 'null')

    def test_unknown_manual_actions_ids_paths_and_component_names_are_hash_only(self):
        members, manifest, dto = self.fixture()
        dto['results'][0]['manual_actions'] = ['PRIVATE token /private/path']
        dto['results'][0]['reason'] = 'PRIVATE reason'
        dto['actions'][0]['component_support'][0]['name'] = 'PRIVATE name'
        self.change_receipt_stdout(members, manifest, dto)
        result = self.project(members)
        manual = result['dto']['results']['manual_actions']['rows'][0]
        self.assertNotIn('known_value', manual)
        self.assertEqual(manual['sha256'], reader.digest(b'PRIVATE token /private/path'))
        self.assertNotIn(b'PRIVATE', reader.encode(result))
        self.assertNotIn(b'/private/path', reader.encode(result))

    def test_duplicate_selected_json_and_malformed_stderr_encoding_refuse(self):
        for raw in (b'{"schema_version":1,"schema_version":1}', b'{"bad":'):
            members, manifest, _ = self.fixture()
            receipt = json.loads(members['public-11.receipt.json'])
            receipt.update(stdout_base64=base64.b64encode(raw).decode(), stdout_bytes=len(raw), stdout_sha256=reader.digest(raw))
            self.write_json(members, 'public-11.receipt.json', receipt)
            self.reindex(members, manifest)
            with self.assertRaises(ValueError):
                self.project(members)
        members, manifest, _ = self.fixture()
        receipt = json.loads(members['public-10.receipt.json'])
        receipt['stderr_base64'] = '%%%%'
        self.write_json(members, 'public-10.receipt.json', receipt)
        self.reindex(members, manifest)
        with self.assertRaises(ValueError):
            self.project(members)

if __name__ == '__main__':
    unittest.main()
