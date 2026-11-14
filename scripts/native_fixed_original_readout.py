#!/usr/bin/env python3
"""Read six fixed failed captures; never admit or execute a native client."""
import argparse
import ast
import base64
import hashlib
import io
import json
import math
from pathlib import Path, PurePosixPath
import re
import runpy
import stat
import struct
import sys
import zipfile
import zlib

check_expansion = runpy.run_path(str(Path(__file__).with_name('read-authoring-evidence-zip.py')),
                               run_name='fixed_original_zip_checks')['check_expansion']
CASE = 'grok-5ff-session'
ARTIFACT = 14100597648
ARCHIVE_SIZE = 11865
ARCHIVE_HASH = '8c1d6a6a7dba38cc58882ffc8019c35fea4bdaf41808c644aff704dee409ff15'
RUN = '43559930011'
SOURCE = '5ff83587db83f613ebd8c784d2032f24e46b6bdb'
TREE = 'e667fdd088aae50259332f772c7eb23098889083'
FRAME_SIZE = 2493
FRAME_HASH = 'f9457d63ac0bb5d432a98790a84c119a3354b2ef53c181fd50600a6da5bfe533'
MODE = 'session-only-v1'
LABELS = ('version', 'help', 'agent-help', 'stdio-help', 'acp')
ARGUMENTS = (['--version'], ['--help'], ['agent', '--help'],
             ['agent', 'stdio', '--help'], ['agent', '--no-leader', 'stdio'])
MEMBERS = {'admission.json', 'acquisition.json', 'a.ledger.json', 'a.rpc.json'} | {
    'a-' + label + '.merged.log' for label in LABELS}
FLAGS = ('inventory_admitted', 'native_enablement', 'workspace_trust', 'native_mcp_call',
         'provider_authentication', 'real_model_inference', 'production_project_activation')
PIN = '11d27a30adfb75c4b9b180072ab27412a1916f054ff5497ab70fb9a77d304bcc'
BINARY = '22d281c5964c5444147328096ba2a221f3f0874c6858e27c825656a80f6e58e9'

def require(ok):
    if not ok:
        raise ValueError('fixed original refused')

def digest(body):
    return hashlib.sha256(body).hexdigest()

def closed(value, keys):
    require(type(value) is dict and set(value) == set(keys.split()))
    return value

def unique(pairs):
    result = {}
    for key, value in pairs:
        require(key not in result)
        result[key] = value
    return result

def document(body, maximum=1 << 20):
    require(type(body) is bytes and len(body) <= maximum)
    return json.loads(body.decode('utf-8'), object_pairs_hook=unique,
                      parse_constant=lambda _: require(False))

def unpack(body, allowed, *, exact_members=True):
    require(type(exact_members) is bool)
    require(type(body) is bytes and 0 < len(body) <= 1 << 20)
    # No comment/ZIP64; allocation authority is checked before ZipFile constructs entries.
    require(len(body) >= 22 and body[-22:-18] == b"PK\x05\x06")
    disk, start_disk, on_disk, count, central_size, central_offset, comment = struct.unpack("<4H2IH", body[-18:])
    require(disk == start_disk == comment == 0 and on_disk == count and 0 < count <= len(allowed)
            and count != 65535 and central_size <= 16384 and central_offset + central_size == len(body) - 22)
    result, total, intervals = {}, 0, []
    with zipfile.ZipFile(io.BytesIO(body)) as archive:
        require(len(archive.infolist()) == count and not archive.comment)
        for info in archive.infolist():
            require(info.filename in allowed and info.filename not in result and not info.is_dir()
                    and not info.flag_bits & 1 and info.compress_type in (zipfile.ZIP_STORED, zipfile.ZIP_DEFLATED)
                    and len(info.extra) <= 4096 and info.file_size <= 1 << 20 and info.compress_size <= len(body))
            offset = 0
            while offset < len(info.extra):
                require(offset + 4 <= len(info.extra))
                kind, size = struct.unpack_from("<HH", info.extra, offset)
                offset += 4
                require(kind != 1 and offset + size <= len(info.extra))
                offset += size
            mode = info.external_attr >> 16
            require(not stat.S_ISLNK(mode) and (not stat.S_IFMT(mode) or stat.S_ISREG(mode)))
            offset = info.header_offset
            require(0 <= offset and offset + 30 <= central_offset and body[offset:offset + 4] == b"PK\x03\x04")
            flags, method = struct.unpack_from("<HH", body, offset + 6)
            crc, compressed, expanded = struct.unpack_from("<III", body, offset + 14)
            require((flags, method) == (info.flag_bits, info.compress_type)
                    and ((crc, compressed, expanded) == (info.CRC, info.compress_size, info.file_size)
                         or flags & 8 and (crc, compressed, expanded) == (0, 0, 0)))
            name_size, extra_size = struct.unpack_from("<HH", body, offset + 26)
            require(extra_size == 0 and body[offset + 30:offset + 30 + name_size] == info.filename.encode("utf-8"))
            start = offset + 30 + name_size
            end = start + info.compress_size
            require(end <= central_offset)
            check_expansion(body, start, info)
            if flags & 8:
                if body[end:end + 4] == b"PK\x07\x08":
                    end += 4
                require(end + 12 <= central_offset and struct.unpack_from("<III", body, end)
                        == (info.CRC, info.compress_size, info.file_size))
                end += 12
            intervals.append((offset, end))
            total += info.file_size
            require(total <= 16 << 20)
            with archive.open(info) as source:
                data = source.read((1 << 20) + 1)
                require(len(data) == info.file_size and not source.read(1))
            result[info.filename] = data  # ZipExtFile verifies CRC on full bounded read.
    intervals.sort()
    require(intervals[0][0] == 0 and intervals[-1][1] == central_offset
            and all(a[1] == b[0] for a, b in zip(intervals, intervals[1:])))
    require(not exact_members or set(result) == set(allowed))
    return result

def identities(value, nonce):
    require(type(value['schema_version']) is int and value['schema_version'] == 1
            and value['source_commit'] == SOURCE and value['source_tree'] == TREE
            and value['scope'] == 'a' and value['nonce'] == nonce)

def acquisition(value):
    closed(value, 'url resolved_origin bytes sha256 integrity pin_verified members installer_executed')
    require(value['url'] == 'https://x.ai/cli/grok-1.0.70-linux-x86_64.gz'
            and type(value['bytes']) is int and value['bytes'] == 76760282
            and value['sha256'] == PIN and value['integrity'] == 'sha256:' + PIN
            and value['pin_verified'] is True and value['installer_executed'] is False)
    require(type(value['resolved_origin']) is str and len(value['resolved_origin']) <= 256
            and re.fullmatch(r'https://[A-Za-z0-9.-]+', value['resolved_origin']))
    require(type(value['members']) is list and len(value['members']) == 1)
    row = closed(value['members'][0], 'name kind bytes executable sha256')
    require(row['name'] == 'grok' and row['kind'] == 'file' and row['executable'] is True
            and type(row['bytes']) is int and 0 < row['bytes'] <= 256 << 20
            and row['sha256'] == BINARY)

def isolation(value):
    closed(value, 'uid gid euid egid groups network_namespace parent_network_namespace interfaces '
                  'capabilities_zero no_new_privileges loopback_up')
    require(all(type(value[k]) is int and 0 < value[k] < 1 << 64 for k in
                ('uid', 'gid', 'euid', 'egid', 'network_namespace', 'parent_network_namespace'))
            and value['uid'] == value['euid'] and value['gid'] == value['egid']
            and value['groups'] == [] and value['interfaces'] == ['lo']
            and value['capabilities_zero'] is True and value['loopback_up'] is True
            and type(value['no_new_privileges']) is int and value['no_new_privileges'] == 1
            and value['network_namespace'] != value['parent_network_namespace'])

def streams(result, members):
    rows = result['commands']
    require(type(rows) is list and len(rows) == len(LABELS))
    native, total = None, 0
    for index, row in enumerate(rows):
        closed(row, 'label argv exit error merged_bytes merged_sha256 merged_file')
        require(row['label'] == LABELS[index] and row['merged_file'] == 'a-' + LABELS[index] + '.merged.log')
        argv = row['argv']
        require(type(argv) is list and len(argv) == len(ARGUMENTS[index]) + 1
                and all(type(part) is str for part in argv) and argv[1:] == ARGUMENTS[index]
                and len(argv[0]) <= 512 and re.fullmatch(r'/tmp/uap-grok-acp-sealed-[^/]+/grok', argv[0])
                and all(32 <= ord(char) <= 126 for char in argv[0])
                and str(PurePosixPath(argv[0])) == argv[0])
        if native is None:
            native = argv[0]
        require(argv[0] == native)
        raw = members[row['merged_file']]
        require(type(row['merged_bytes']) is int and 0 <= row['merged_bytes'] <= 1 << 20
                and row['merged_bytes'] == len(raw) and row['merged_sha256'] == digest(raw))
        total += len(raw)
        if index < 4:
            require(type(row['exit']) is int and row['exit'] == 0 and row['error'] is None)
        else:
            require((row['exit'] is None or type(row['exit']) is int and -255 <= row['exit'] <= 255)
                    and row['error'] == 'protocol-refused')
    require(total <= 1 << 20 and members['a-version.merged.log'] in
            (b'grok 1.0.70 (f50904eaee6c)\n', b'grok 1.0.70 (f50904eaee6c)\r\n'))
    return len(rows)

def wire_prefix(members):
    rpc = closed(document(members['a.rpc.json']), 'initialize plugins notifications session')
    require(rpc['plugins'] is None and rpc['session'] is None
            and type(rpc['notifications']) is list and len(rpc['notifications']) == 11)
    initial = closed(rpc['initialize'], 'jsonrpc id result')
    require(initial['jsonrpc'] == '2.0' and type(initial['id']) is int and initial['id'] == 1
            and type(initial['result']) is dict)
    wire = members['a-acp.merged.log']
    require(wire.endswith(b'\n'))
    lines = wire.splitlines()
    require(len(lines) == 13 and all(0 < len(line) <= 64 << 10 for line in lines))
    last = lines[12]
    require(len(last) == FRAME_SIZE and digest(last) == FRAME_HASH and type(document(last, 64 << 10)) is dict)
    # This joins the stored prefix, not a new acceptance of the unrecognized frame.
    seen_initial, notices = False, []
    for line in lines[:12]:
        frame = document(line, 64 << 10)
        if frame == initial:
            require(not seen_initial)
            seen_initial = True
        else:
            closed(frame, 'jsonrpc method params')
            require(frame['jsonrpc'] == '2.0' and frame['method'] in
                    ('_x.ai/mcp/servers_updated', '_x.ai/session/setup', '_x.ai/mcp_initialized')
                    and type(frame['params']) is dict)
            notices.append(frame)
    require(seen_initial and notices == rpc['notifications'])
    return last

def project(members):
    summary = closed(document(members['admission.json']),
                     'schema_version collection_complete cleanup_complete qualification scopes '
                     'source_commit source_tree nonce run attempt error mode measurement_complete')
    require(type(summary['schema_version']) is int and summary['schema_version'] == 1
            and summary['source_commit'] == SOURCE and summary['source_tree'] == TREE
            and summary['run'] == RUN and summary['attempt'] == '1' and summary['mode'] == MODE
            and summary['collection_complete'] is False and summary['cleanup_complete'] is True
            and summary['measurement_complete'] is True and summary['error'] == 'admission-refused')
    flags = closed(summary['qualification'], ' '.join(FLAGS))
    require(all(flags[key] is False for key in FLAGS))
    nonce = summary['nonce']
    require(type(nonce) is str and re.fullmatch('[0-9a-f]{32}', nonce)
            and type(summary['scopes']) is list and len(summary['scopes']) == 1)
    receipt = closed(summary['scopes'][0], 'schema_version source_commit source_tree scope nonce '
                     'cleanup_complete sealed_cleanup_complete system_executables result')
    identities(receipt, nonce)
    require(receipt['cleanup_complete'] is True and receipt['sealed_cleanup_complete'] is True)
    executables = closed(receipt['system_executables'], 'unshare setpriv env python3')
    for name, row in executables.items():
        closed(row, 'path sha256')
        require(row['path'] == '/usr/bin/' + ('python3.12' if name == 'python3' else name)
                and type(row['sha256']) is str and re.fullmatch('[0-9a-f]{64}', row['sha256']))
    result = closed(receipt['result'], 'schema_version source_commit source_tree scope nonce '
                    'semantic_complete reason isolation commands mode provider')
    identities(result, nonce)
    require(result['mode'] == MODE and result['semantic_complete'] is False
            and result['reason'] == 'protocol-refused')
    isolation(result['isolation'])
    provider = closed(result['provider'], 'requests models_get root_get forbidden settled')
    require(provider['settled'] is True
            and all(type(provider[key]) is int and 0 <= provider[key] <= 13
                    for key in ('requests', 'models_get', 'root_get', 'forbidden'))
            and provider['requests'] == provider['models_get'] + provider['root_get'] + provider['forbidden']
            and provider['forbidden'] == 0)
    acquisition(document(members['acquisition.json']))
    joined = streams(result, members)
    frame = wire_prefix(members)
    ledger = closed(document(members['a.ledger.json']),
                    'before after accepted protected_originals_unchanged selected_config_capture')
    require(ledger['accepted'] is False and type(ledger['protected_originals_unchanged']) is bool
            and type(ledger['before']) is dict and type(ledger['after']) is dict
            and 0 < len(ledger['before']) <= 4096 and 0 < len(ledger['after']) <= 4096
            and ledger['protected_originals_unchanged'] is
                all(ledger['after'].get(name) == item for name, item in ledger['before'].items()))
    # Do not project private config captures, absolute paths, native stderr, or arbitrary diagnostics.
    return dict(kind='fixed-original-readout', case=CASE, artifact_id=ARTIFACT, run=RUN, source=SOURCE, tree=TREE,
                original_failed=True, native_qualified=False, qualification=dict.fromkeys(FLAGS, False),
                cleanup_complete=True, original_semantic_complete=False, original_reason='protocol-refused',
                command_streams_joined=joined, prefix_frames_joined=12, session_reply=False, inventory_reply=False,
                provider_requests=provider['requests'], provider_forbidden=0,
                frame_index=13, frame_bytes=len(frame), frame_sha256=digest(frame),
                frame_base64=base64.b64encode(frame).decode('ascii'))

def audit(body):
    require(type(body) is bytes and len(body) == ARCHIVE_SIZE and digest(body) == ARCHIVE_HASH)
    outer = unpack(body, {'research.zip'})
    inner = outer['research.zip']
    members = unpack(inner, MEMBERS)
    result = project(members)
    result.update(archive_bytes=len(body), archive_sha256=digest(body), inner_bytes=len(inner),
                  inner_sha256=digest(inner), outer_members=1, inner_members=len(members), crc_verified=True)
    return result

def encode(result):
    body = json.dumps(result, sort_keys=True, separators=(',', ':'), ensure_ascii=True, allow_nan=False).encode('ascii') + b'\n'
    require(len(body) <= 8192)
    return body


HERMES_HEAD = '4cd5b613ae5948742231ed0e7babe67b9c3a1c81'
HERMES_TREE = 'e9e1fcb436ddb68dd45c817aecd9450196aeae72'
HERMES_RUN = '43565523950'
HERMES_PIN = '2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19'
HERMES_PIN_TREE = '675b4cea2d2aef80460f07235b0e27e6c6922743'
HERMES_CASES = {
    'hermes-4cd-source-linux': (14104470250, 14557, 'c4b54471f5f08a4316897d6dff0bead8c67aafffc66278a25d39bc521fe6a90b', 'Linux'),
    'hermes-4cd-source-windows': (14104186309, 14880, 'a4ad3420283d338a553bf370fb7fd37eea69757af72bc31e18ed63bca3c0fdbd', 'Windows'),
    'hermes-4cd-source-macos': (14103825881, 1668, '949fe61943f330e85f53c4ca4396428c1d3056f075c7600e62638c8f34b16ccd', 'macOS'),
}
HERMES_DEFINITIONS = {
    'e2e/hermes-profile-admission.py': (201233, '4a139d68932b768611366dcb050430602c80fa3e8b36f806982e18091e05cacf'),
    'e2e/hermes-noauth-tool-call.py': (22090, '7fbb811ae4b5c6d791b128537ba3a56fb1367c1587e88ffee580517606ddd6fb'),
    'e2e/hermes-native.py': (10113, '87c5b152dcde6d2a2c659e8e101364f788f9632b63e1d84077a3b689080611f2'),
    'hermes_local_provider_fixture.py': (11821, '672e463b73b98f1af77b039717f0a1be39c4438c6a60640b2697061796b7f357'),
    'native_client_project_artifacts.py': (10942, '59a46146b1f51642a5f6562a10a920375327b244bc530115fb0aaa243f44e175'),
}
HERMES_FLAGS = ('capture_complete', 'admission_qualified', 'production_project_mcp', 'vendor_authentication',
                'real_model_inference', 'tools_call', 'source_install_custom_root_qualified',
                'source_bootstrap_recovery_qualified')
HERMES_SETTLED = 'owned-processes-settled; installed source and measured client-owned state retained'
HERMES_ERROR_KINDS = ('RuntimeError', 'ValueError', 'TypeError', 'KeyError', 'IndexError', 'AttributeError',
                     'OSError', 'PermissionError', 'FileNotFoundError', 'FileExistsError', 'TimeoutError',
                     'TimeoutExpired', 'AssertionError', 'ImportError', 'ModuleNotFoundError',
                     'SystemExit', 'KeyboardInterrupt', 'JSONDecodeError', 'PayloadHardlinkRefusal')
HERMES_TRACE = re.compile(r'^  File "(?:[^"\r\n]*[/\\])?(hermes-profile-admission\.py|hermes-noauth-tool-call\.py)", line ([1-9][0-9]{0,5}), in ([A-Za-z_][A-Za-z_0-9]{0,100}|<module>)$')

def hermes_definition_bytes():
    root, bodies = Path(__file__).resolve().parent, {}
    for relative, (size, expected_hash) in HERMES_DEFINITIONS.items():
        path = root / relative
        before = path.lstat()
        require(path.resolve() == path and stat.S_ISREG(before.st_mode) and before.st_nlink == 1
                and before.st_size == size)
        with path.open('rb') as source:
            body = source.read(size + 1)
        after = path.lstat()
        fields = ('st_dev', 'st_ino', 'st_mode', 'st_nlink', 'st_size', 'st_mtime_ns', 'st_ctime_ns')
        require(all(getattr(before, field) == getattr(after, field) for field in fields)
                and len(body) == size and digest(body) == expected_hash)
        bodies[relative] = body
    return root, bodies

def hermes_definitions():
    """Load the five pinned definition files without any native entrypoint."""
    root, bodies = hermes_definition_bytes()
    names = ('hermes_local_provider_fixture', 'native_client_project_artifacts')
    absent = object()
    previous_modules = {name: sys.modules.get(name, absent) for name in names}
    previous_path, previous_bytecode = list(sys.path), sys.dont_write_bytecode
    try:
        # Do not adopt an ambient cached object instead of the reviewed definition bytes.
        for name in names:
            sys.modules.pop(name, None)
        sys.dont_write_bytecode = True
        helpers = runpy.run_path(str(root / 'e2e/hermes-profile-admission.py'),
                                 run_name='fixed_hermes_definition_only')
        for name in names:
            module = sys.modules.get(name)
            require(module is not None and Path(module.__file__).resolve() == root / (name + '.py'))
        after_root, after_bodies = hermes_definition_bytes()
        require(after_root == root and after_bodies == bodies)
        return helpers, bodies
    finally:
        sys.path[:] = previous_path
        sys.dont_write_bytecode = previous_bytecode
        for name in names:
            sys.modules.pop(name, None)
            if previous_modules[name] is not absent:
                sys.modules[name] = previous_modules[name]

def hermes_error_kind(value):
    require(type(value) is str and len(value) <= 400)
    return next((kind for kind in HERMES_ERROR_KINDS if value.startswith(kind + ': ')), 'unknown')

def hermes_trace_readout(members, helpers, bodies, proof):
    # These dictionaries are computed from pinned repository source, never artifact code.
    guards, source_locations = set(), {}
    for relative in ('e2e/hermes-profile-admission.py', 'e2e/hermes-noauth-tool-call.py'):
        source = bodies[relative].decode('utf-8')
        pending = [compile(source, Path(relative).name, 'exec')]
        code_type, locations = type(pending[0]), set()
        for _ in range(2048):
            if not pending:
                break
            own_code = pending.pop()
            locations.update((own_code.co_name, line) for _, _, line in own_code.co_lines() if line is not None)
            pending.extend(value for value in own_code.co_consts if type(value) is code_type)
        require(not pending)
        source_locations[Path(relative).name] = locations
        for node in ast.walk(ast.parse(source)):
            candidate = None
            if isinstance(node, ast.Call) and isinstance(node.func, ast.Name):
                if node.func.id == 'require' and len(node.args) >= 2:
                    candidate = node.args[1]
                elif node.func.id in HERMES_ERROR_KINDS and node.args:
                    candidate = node.args[0]
            if (isinstance(candidate, ast.Constant) and type(candidate.value) is str
                    and 0 < len(candidate.value) <= 200 and candidate.value.isascii()
                    and all(32 <= ord(char) <= 126 for char in candidate.value)):
                guards.add(candidate.value)
    frames, classes, matched_guards = [], set(), set()
    trace_count = 0
    for name in ('lifecycle.stderr', *(label + '.stderr' for label in sorted(helpers['COMMANDS']))):
        if name not in members:
            continue
        lines = members[name].decode('utf-8', errors='strict').splitlines()
        require(len(lines) <= 32768)
        for line in lines:
            match = HERMES_TRACE.fullmatch(line)
            if match:
                require((match[3], int(match[2])) in source_locations[match[1]])
                trace_count += 1
                require(trace_count <= 64)
                if len(frames) < 16:
                    frames.append(dict(file=match[1], line=int(match[2]), function=match[3]))
            for kind in HERMES_ERROR_KINDS:
                prefix = kind + ': '
                if line.startswith(prefix):
                    classes.add(kind)
                    message = line[len(prefix):]
                    if message in guards:
                        matched_guards.add(message)
    result = dict(primary_error_present='error' in proof,
                  primary_error_kind=hermes_error_kind(proof['error']) if 'error' in proof else None,
                  traceback_frames=frames,
                  traceback_count=trace_count, traceback_truncated=trace_count > len(frames),
                  stream_error_kinds=sorted(classes), known_guard_messages=sorted(matched_guards)[:8],
                  known_guard_count=len(matched_guards), known_guards_truncated=len(matched_guards) > 8)
    for key in ('runtime_error', 'preparation_error'):
        if key in proof:
            result[key + '_kind'] = hermes_error_kind(proof[key])
    if proof['cleanup'] == 'failed':
        require(type(proof.get('cleanup_error')) is str and proof['cleanup_error'] in HERMES_ERROR_KINDS)
        result['cleanup_error_kind'] = proof['cleanup_error']
    locator = members.get('lifecycle.stdout', b'')
    result.update(observer_locator_present=False, observer_stdout_state='empty')
    if locator:
        result['observer_stdout_state'] = 'unvalidated'
        try:
            document(locator, 2048)
            observed = helpers['observer_failure_document'](locator, proof['nonce'], HERMES_HEAD)
        except (RuntimeError, ValueError, TypeError, KeyError, RecursionError):
            pass  # The pinned producer also treats this locator as optional.
        else:
            result.update(observer_locator_present=True, observer_stdout_state='validated')
            result['observer_locator'] = {key: observed[key] for key in ('phase', 'reason', 'exception_kind', 'command', 'caller')}
    return result

def hermes_optional_readout(members, helpers, proof):
    result = dict(census_present='bootstrap-census.json' in members,
                  source_refusal_present='source-payload-refusal.json' in members,
                  source_delta_present='source-user-delta.json' in members,
                  source_preparation_present='source-user-preparation.json' in members)
    nonce = proof['nonce']
    require('bootstrap-census.json' not in members or 'bootstrap-census-request.json' in members)
    for name in ('bootstrap-census-request.json', 'bootstrap-census.json'):
        if name in members:
            value = document(members[name], 16 << 10)
            validated = helpers['validate_census_member'](name, members[name])
            if name.endswith('request.json'):
                require(value['nonce'] == nonce)
            else:
                result['census'] = validated
    if 'source-user-preparation.json' in members:
        body = members['source-user-preparation.json']
        document(body, 16 << 10)
        prepared = helpers['source_preparation_document'](body)
        require(prepared['nonce'] == nonce and proof.get('source_user_preparation') == prepared
                and type(proof.get('source_prepared_runtime')) is dict
                and proof['source_prepared_runtime'].get('user') == prepared['prepared_user'])
        result['source_preparation'] = dict(original_user=prepared['original_user'], prepared_user=prepared['prepared_user'],
                                           authored_count=len(prepared['authored_members']),
                                           declared_directory_count=len(prepared['declared_directories']),
                                           retained_preimage_count=len(prepared['installer_preimages']))
    if 'source-payload-refusal.json' in members:
        body = members['source-payload-refusal.json']
        document(body, 2048)
        value = helpers['source_refusal_document'](body)
        require(value['nonce'] == nonce)
        result['source_refusal'] = {key: value[key] for key in ('stage', 'operation', 'member_sha256',
            'metadata', 'integer_saturated', 'bytes_read', 'errno', 'winerror', 'target_sha256', 'target_bytes')}
        if 'profile_bound' in value:
            result['source_refusal']['profile_bound'] = value['profile_bound']
    if 'source-user-delta.json' in members:
        body = members['source-user-delta.json']
        document(body, 16 << 10)
        value = helpers['source_delta_document'](body)
        require(value['nonce'] == nonce and type(proof.get('source_prepared_runtime')) is dict
                and type(proof.get('runtime_after')) is dict
                and proof['source_prepared_runtime'].get('user') == value['before']
                and proof['runtime_after'].get('user') == value['after'])
        rows = [{key: row[key] for key in ('member_sha256', 'before', 'after')} for row in value['changes'][:4]]
        result['source_delta'] = dict(before=value['before'], after=value['after'], total_changes=value['total_changes'],
                                      source_truncated=value['truncated'], source_rows=len(value['changes']),
                                      projected_rows=len(rows), projection_truncated=len(rows) < len(value['changes']), changes=rows)
    return result

def hermes_command_readout(members, helpers):
    count, incomplete, failures = 0, 0, []
    for label in sorted(helpers['COMMANDS']):
        name = label + '.status.json'
        if name not in members:
            if label + '.stdout' in members or label + '.stderr' in members:
                incomplete += 1
            continue
        status = closed(document(members[name]), 'argv pid exit error streams')
        require(type(status['argv']) is list and 0 < len(status['argv']) <= 256
                and all(type(arg) is str and len(arg) <= 4096 for arg in status['argv'])
                and type(status['pid']) is int and 0 < status['pid'] < 1 << 32
                and (status['exit'] is None or type(status['exit']) is int and -(1 << 32) <= status['exit'] < 1 << 32)
                and status['error'] in (None, 'output bound', 'command timeout', 'native command left inherited capture pipe open'))
        streams = closed(status['streams'], 'stdout stderr')
        for suffix, row in streams.items():
            closed(row, 'bytes sha256')
            raw = members[label + '.' + suffix]
            require(type(row['bytes']) is int and row['bytes'] == len(raw) and row['sha256'] == digest(raw))
        count += 1
        if status['error'] is not None or status['exit'] != 0:
            failures.append(dict(command=label, exit=status['exit'], capture_error=status['error']))
    return dict(command_status_count=count, command_streams_joined=2 * count,
                streams_without_status=incomplete, failing_status_count=len(failures),
                failing_statuses=failures[:8], failing_statuses_truncated=len(failures) > 8)

def project_hermes(members, case, helpers, bodies):
    _, _, _, os_name = HERMES_CASES[case]
    proof = document(members['profile-admission.json'])
    require(type(proof) is dict
            and proof.get('source') == proof.get('source_after') == dict(commit=HERMES_HEAD, tree=HERMES_TREE, dirty='')
            and proof.get('hermes_source') == proof.get('hermes_source_after') == dict(commit=HERMES_PIN, tree=HERMES_PIN_TREE, dirty='')
            and proof.get('run_id') == HERMES_RUN and proof.get('run_attempt') == '1' and proof.get('runner_os') == os_name
            and proof.get('runtime_distribution') == 'source' and proof.get('status') == 'failed'
            and proof.get('cleanup') in (HERMES_SETTLED, 'failed') and all(proof.get(key) is False for key in HERMES_FLAGS)
            and type(proof.get('nonce')) is str and re.fullmatch('[0-9a-f]{64}', proof['nonce'])
            and 'lifecycle_exit' in proof
            and (proof['lifecycle_exit'] is None or type(proof['lifecycle_exit']) is int
                 and -(1 << 32) <= proof['lifecycle_exit'] < 1 << 32))
    streams = proof.get('streams')
    require(type(streams) is dict and set(streams) == set(members) - {'profile-admission.json'})
    for name, row in streams.items():
        closed(row, 'bytes sha256')
        require(type(row['bytes']) is int and row['bytes'] == len(members[name])
                and row['sha256'] == digest(members[name]))
    result = dict(kind='fixed-original-readout', case=case, artifact_id=HERMES_CASES[case][0],
                  source=HERMES_HEAD, tree=HERMES_TREE, run=HERMES_RUN, runner_os=os_name,
                  original_failed=True, native_qualified=False, qualification=dict.fromkeys(HERMES_FLAGS, False),
                  runtime_distribution='source', cleanup_state='source-retained-settled' if proof['cleanup'] == HERMES_SETTLED else 'failed',
                  stream_joins=len(streams), lifecycle_exit=proof['lifecycle_exit'])
    result.update(hermes_command_readout(members, helpers))
    result.update(hermes_trace_readout(members, helpers, bodies, proof))
    result.update(hermes_optional_readout(members, helpers, proof))
    return result

def audit_hermes(body, case):
    require(case in HERMES_CASES)
    _, size, expected_hash, _ = HERMES_CASES[case]
    require(type(body) is bytes and len(body) == size and digest(body) == expected_hash)
    helpers, sources = hermes_definitions()
    members = unpack(body, helpers['MEMBERS'], exact_members=False)
    require('profile-admission.json' in members)
    result = project_hermes(members, case, helpers, sources)
    result.update(archive_bytes=size, archive_sha256=expected_hash, member_count=len(members), crc_verified=True)
    return result


GROK_CURRENT_CASE = 'grok-440-noauth'
GROK_CURRENT_ARTIFACT = 14106267930
GROK_CURRENT_SIZE = 23383
GROK_CURRENT_HASH = '8379f03a4fcc475bf50aa815ea182057448726cbd96e812907f832dd3e256952'
GROK_CURRENT_HEAD = '440f0666dad9255c1667c86f95654f7d960ffeb6'
GROK_CURRENT_TREE = '5efcc6f7d254089f111a0097dc968c87299882dc'
GROK_CURRENT_RUN = '43574321330'
GROK_CURRENT_SOURCES = {
    "scripts/grok_project_protocol_service.py": [
        23791,
        "2807c8f49a35620a3132033e792c9f192d37dcb59326c1ceb9fd97edb3c0f2f1"
    ],
    "scripts/grok_noauth_call_e2e.py": [
        28456,
        "a419e3941f8c67b834db8746ed7fc56a8c4f954a6cc9cc48c38b6e2dca8e2223"
    ],
    "scripts/grok_project_protocol_e2e.py": [
        25243,
        "322a42d56217a417616b35ac2a784f20aa513393090a88128e3e31dfefd27148"
    ],
    "scripts/grok_project_protocol_enablement.py": [
        14396,
        "4741e1bd4f7e2280e2157e4bf0441f8292f1c99c5e42c91685e977b385875739"
    ],
    "scripts/grok_project_protocol_profile.py": [
        12237,
        "b50b938ef3c7edbed8666bc3cc043904ff21778608a3f8ad33763752a5da7748"
    ],
    "scripts/grok_project_protocol_client.py": [
        14976,
        "8010178ed61f0d627d2b2de4a524cb0eeb541e12c19041c999282fe075ee0f70"
    ],
    "scripts/native_client_research.py": [
        5554,
        "f95b86c12bd9e4a77d1c304b8bd7b70422d24c997ee55161b700e134f8af8fd4"
    ],
    "install/integrationctl/agentplugins/clients/grok/enablement_authority.go": [
        4625,
        "f4c4db66427d2625a7ebd3b9c0c4f1a83d1580509a127b63e1c7852516d5271f"
    ],
    "cli/internal/agentpluginscli/plugin_enablement.go": [
        1003,
        "cb3267d269011965dce8ef363aa8176b81664a472bfec660a68db225dfd4d981"
    ],
    "cli/cmd/agentplugins/main.go": [
        15114,
        "1d8e3b65780c3f73ce646d2f3618b8738e4be2eeac68e6f51cbc2cb841741331"
    ],
    "cli/internal/agentpluginscli/color_error.go": [
        1176,
        "950bda3855f187a6670e6de60eda11757169f5d37b3d04fb70811706be4c1930"
    ]
}
GROK_PARENT_LABELS = ('grok-source-before-commit', 'grok-source-before-tree', 'grok-source-before-clean',
    'grok-noauth-service-absent', 'grok-noauth-systemd-version', 'runtime-test', 'grok-noauth-service-state',
    'grok-source-after-commit', 'grok-source-after-tree', 'grok-source-after-clean')
GROK_STAGES = ('installed', 'updated', 'repaired', 'plugin-cwd', 'data-cwd', 'restarted-data-cwd')
GROK_CHILD_LABELS = frozenset(('version native-help-admission git-a git-b native-profile-mcp-help '
    'native-profile-disable-help native-profile-disable-witness install untrusted-inspect untrusted-doctor '
    'repeat-install repair-no-enable-consent repair-enable-dry-run repair-enable-owned-full-id repair-repeat-owned-full-id '
    'trusted-inspect update damaged-inspect partial-repair-refusal missing-package-inspect repair-missing-package '
    'plugin-cwd-update data-cwd-update remove-owned-full-id removed-inspect removed-doctor '
    'project-b-negative-selection project-b-negative removed-native-negative-selection removed-native-negative').split()
    + [stage + '-native-call' for stage in GROK_STAGES])
GROK_RECORD_KEYS = 'probe argv cwd error exit elapsed_seconds stdout_file stderr_file stdout_bytes stderr_bytes stdout_sha256 stderr_sha256'
GROK_REQUIRED = frozenset(('schema_version source_commit source_tree passed commands stages vendor_authentication_used '
    'unmanaged_project_owned_enablement_qualified installer_sha256 probe_sha256 github_run_id github_run_attempt '
    'github_event platform actual_model_used scripted_local_provider grok_native_tools_call_qualified').split())
GROK_OPTIONAL = frozenset(('acquisition all_original_events events_sha256 events_base64 error failed_command '
    'native_profile_preparation installed_config_receipts enablement_previews owned_enablement owned_enablement_removal '
    'repair prepared_owned_state pre_removal_data_receipts post_removal_installation project_b_unchanged '
    'protected_fixture_files_match_expected untrusted_refusal_verified removed_absent persistent_data_retained '
    'project_b_negative removed_native_negative native_provider_sessions').split())
GROK_FALSE = ('passed', 'vendor_authentication_used', 'unmanaged_project_owned_enablement_qualified',
              'actual_model_used', 'grok_native_tools_call_qualified')
GROK_STDERR_PREFIX = 'agentplugins: '
GROK_PUBLIC = {
    'install': ('add', ('--no-save',)), 'repeat-install': ('add', ('--no-save',)),
    'repair-enable-owned-full-id': ('repair', ('--confirm-plugin-enable', '--yes')),
    'repair-repeat-owned-full-id': ('repair', ('--confirm-plugin-enable', '--yes')),
    'repair-enable-dry-run': ('repair', ('--confirm-plugin-enable', '--dry-run', '--yes')),
    'repair-no-enable-consent': ('repair', ('--yes',)),
}
GROK_ERROR_KINDS = ('ValueError', 'RuntimeError', 'TypeError', 'KeyError', 'IndexError', 'AttributeError',
    'OSError', 'PermissionError', 'FileNotFoundError', 'FileExistsError', 'TimeoutError', 'TimeoutExpired',
    'AssertionError', 'ImportError', 'ModuleNotFoundError', 'JSONDecodeError', 'OverflowError')

def grok_current_sources():
    # Read and compile pinned source as data; never import or execute journey code.
    root, bodies, locations, guards = Path(__file__).resolve().parents[1], {}, {}, set()
    for relative, (size, expected_hash) in GROK_CURRENT_SOURCES.items():
        path = root / relative
        before = path.lstat()
        require(path.resolve() == path and stat.S_ISREG(before.st_mode) and before.st_nlink == 1 and before.st_size == size)
        with path.open('rb') as stream:
            body = stream.read(size + 1)
        after = path.lstat()
        require(all(getattr(before, key) == getattr(after, key) for key in
                    ('st_dev', 'st_ino', 'st_mode', 'st_nlink', 'st_size', 'st_mtime_ns', 'st_ctime_ns'))
                and len(body) == size and digest(body) == expected_hash)
        bodies[relative] = body
    for relative, body in bodies.items():
        source = body.decode('utf-8')
        if relative.endswith('.go'):
            guards.update(re.findall(r'(?:errors\.New|fmt\.Errorf)\("([^"\\\r\n]{1,200})"\)', source))
            continue
        codes, known = [compile(source, PurePosixPath(relative).name, 'exec')], set()
        code_type = type(codes[0])
        for _ in range(2048):
            if not codes:
                break
            item = codes.pop()
            known.update((item.co_name, line) for _, _, line in item.co_lines() if line is not None)
            codes.extend(value for value in item.co_consts if type(value) is code_type)
        require(not codes)
        locations[PurePosixPath(relative).name] = known
        for node in ast.walk(ast.parse(source)):
            value = None
            if isinstance(node, ast.Call) and isinstance(node.func, ast.Name):
                if node.func.id == 'require' and len(node.args) > 1:
                    value = node.args[1]
                elif node.func.id in GROK_ERROR_KINDS and node.args:
                    value = node.args[0]
            if (isinstance(value, ast.Constant) and type(value.value) is str and 0 < len(value.value) <= 200
                    and all(32 <= ord(char) <= 126 for char in value.value)):
                guards.add(value.value)
    return locations, guards

def grok_string(value, maximum=4096):
    require(type(value) is str and 0 < len(value) <= maximum and '\x00' not in value)
    return value

def grok_path(value):
    grok_string(value)
    require(value.startswith('/') and not value.startswith('//') and str(PurePosixPath(value)) == value and '..' not in PurePosixPath(value).parts)
    return value

def grok_hash(value):
    require(type(value) is str and re.fullmatch('[0-9a-f]{64}', value))
    return value

def grok_document(body):
    require(type(body) is bytes and len(body) <= 1 << 20)
    def finite(value):
        result = float(value)
        require(math.isfinite(result))
        return result
    return json.loads(body.decode('utf-8'), object_pairs_hook=unique, parse_float=finite,
                      parse_constant=lambda _: require(False))

def grok_stream(encoded, count, expected):
    require(type(encoded) is str and len(encoded) <= 4 * (((1 << 20) + 2) // 3))
    raw = base64.b64decode(encoded, validate=True)
    require(base64.b64encode(raw).decode('ascii') == encoded and type(count) is int
            and 0 <= count <= 1 << 20 and len(raw) == count and grok_hash(expected) == digest(raw))
    return raw

def grok_record(row, child, captures=None):
    closed(row, GROK_RECORD_KEYS + (' stdout stderr stdout_base64 stderr_base64' if child else ''))
    label = grok_string(row['probe'], 100)
    require(label in (GROK_CHILD_LABELS if child else GROK_PARENT_LABELS))
    argv = row['argv']
    require(type(argv) is list and 1 <= len(argv) <= 64)
    for value in argv:
        grok_string(value)
    grok_path(row['cwd'])
    require(type(row['exit']) is int and -255 <= row['exit'] <= 255
            and type(row['elapsed_seconds']) in (int, float) and math.isfinite(row['elapsed_seconds'])
            and 0 <= row['elapsed_seconds'] <= 900
            and (row['error'] is None or child and type(row['error']) is str and len(row['error']) <= 512))
    result = {}
    for name in ('stdout', 'stderr'):
        filename = label + '.' + name + '.log'
        require(row[name + '_file'] == filename)
        raw = grok_stream(row[name + '_base64'] if child else captures[filename],
                          row[name + '_bytes'], row[name + '_sha256'])
        require(not child or type(row[name]) is str and row[name] == raw.decode('utf-8', 'replace'))
        result[name] = raw
    return result

def grok_service_state(raw, unit):
    value = {}
    for line in raw.decode('ascii').splitlines():
        key, separator, item = line.partition('=')
        require(separator and key in ('LoadState', 'ActiveState', 'ControlGroup') and key not in value)
        value[key] = item
    closed(value, 'LoadState ActiveState ControlGroup')
    require(value['LoadState'] in ('loaded', 'not-found')
            and value['ActiveState'] in ('active', 'inactive', 'failed', 'activating', 'deactivating')
            and value['ControlGroup'] in ('', '/system.slice/' + unit)
            and (value['LoadState'] != 'not-found' or value['ActiveState'] == 'inactive' and not value['ControlGroup']))
    return value

def grok_parent(receipt, body):
    closed(receipt, 'schema_version kind case source_commit source_tree installer_sha256 probe_sha256 python_sha256 '
        'original_start_monotonic_ns native_passed evidence_bytes evidence_sha256 delegated_service commands streams_base64')
    require(type(receipt['schema_version']) is int and receipt['schema_version'] == 1
            and receipt['kind'] == 'grok-delegated-service' and receipt['case'] == 'grok-noauth'
            and receipt['source_commit'] == GROK_CURRENT_HEAD and receipt['source_tree'] == GROK_CURRENT_TREE
            and receipt['native_passed'] is False and type(receipt['evidence_bytes']) is int
            and receipt['evidence_bytes'] == len(body) and receipt['evidence_sha256'] == digest(body)
            and type(receipt['original_start_monotonic_ns']) is int and 0 < receipt['original_start_monotonic_ns'] < 1 << 63)
    for name in ('installer_sha256', 'probe_sha256', 'python_sha256'):
        grok_hash(receipt[name])
    closure = closed(receipt['delegated_service'],
        'unit uid gid forced_stop forced_kill closed system_executables original_start_monotonic_ns')
    unit = grok_string(closure['unit'], 100)
    require(re.fullmatch(r'uap-grok-noauth-[0-9a-f]{32}\.service', unit)
            and all(type(closure[key]) is int and 0 < closure[key] < 1 << 32 for key in ('uid', 'gid'))
            and closure['closed'] is True and closure['forced_stop'] is False and closure['forced_kill'] is False
            and type(closure['original_start_monotonic_ns']) is int
            and closure['original_start_monotonic_ns'] == receipt['original_start_monotonic_ns'])
    systems = closure['system_executables']
    require(type(systems) is dict and set(systems) == {'/usr/bin/' + name for name in ('sudo', 'systemd-run', 'systemctl', 'env')})
    for value in systems.values():
        grok_hash(value)
    rows, captures = receipt['commands'], receipt['streams_base64']
    require(type(rows) is list and len(rows) == 10 and tuple(row.get('probe') for row in rows if type(row) is dict) == GROK_PARENT_LABELS
            and type(captures) is dict and set(captures) == {label + '.' + stream + '.log'
                for label in GROK_PARENT_LABELS for stream in ('stdout', 'stderr')})
    streams = [grok_record(row, False, captures) for row in rows]
    source, output = grok_path(rows[0]['cwd']), grok_path(rows[5]['cwd'])
    require(source != output and PurePosixPath(source) not in PurePosixPath(output).parents)
    for offset in (0, 7):
        for index, (args, value) in enumerate(((['rev-parse', 'HEAD'], GROK_CURRENT_HEAD + '\n'),
                (['rev-parse', 'HEAD^{tree}'], GROK_CURRENT_TREE + '\n'),
                (['status', '--porcelain', '--untracked-files=normal'], ''))):
            row, raw = rows[offset + index], streams[offset + index]
            require(row['argv'] == ['/usr/bin/git', *args] and row['cwd'] == source
                    and row['exit'] == 0 and raw['stdout'] == value.encode('ascii') and not raw['stderr'])
    query = ['/usr/bin/systemctl', '--no-pager', '--no-ask-password', 'show',
             '--property=LoadState,ActiveState,ControlGroup', unit]
    for index in (3, 6):
        require(rows[index]['argv'] == query and rows[index]['cwd'] == output
                and rows[index]['exit'] == 0 and not streams[index]['stderr'])
    require(grok_service_state(streams[3]['stdout'], unit)['LoadState'] == 'not-found'
            and grok_service_state(streams[6]['stdout'], unit)['ActiveState'] in ('inactive', 'failed'))
    require(rows[4]['argv'] == ['/usr/bin/systemd-run', '--version'] and rows[4]['cwd'] == output
            and rows[4]['exit'] == 0 and not streams[4]['stderr']
            and re.match(rb'systemd 255(?: \([^\r\n]*\))?\n', streams[4]['stdout'])
            and rows[5]['exit'] == 1)
    argv = rows[5]['argv']
    fixed = ['/usr/bin/sudo', '-n', '/usr/bin/systemd-run', '--unit=' + unit, '--wait', '--pipe', '--collect', '--quiet',
        '--service-type=exec', '--expand-environment=no', '--uid=' + str(closure['uid']), '--gid=' + str(closure['gid']),
        '--property=Delegate=yes', '--property=NoNewPrivileges=yes', '--property=KillMode=control-group']
    require(len(argv) == 48 and argv[:15] == fixed and re.fullmatch(r'--property=RuntimeMaxSec=([1-9][0-9]{1,2})s', argv[15])
            and 15 < int(argv[15].split('=')[-1][:-1]) <= 900
            and argv[16:20] == ['--property=TimeoutStopSec=30s', '--working-directory=' + output, '/usr/bin/env', '-i'])
    env = dict(HOME=output + '/home', PATH='/usr/bin:/bin', LANG='C.UTF-8', LC_ALL='C.UTF-8', NO_COLOR='1',
        TERM='dumb', TMPDIR=output + '/tmp', RUNNER_TEMP=output + '/tmp', GIT_CONFIG_GLOBAL='/dev/null',
        GIT_CONFIG_NOSYSTEM='1', GIT_TERMINAL_PROMPT='0', PYTHONDONTWRITEBYTECODE='1', GITHUB_ACTIONS='true',
        RUNNER_ENVIRONMENT='github-hosted', AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED='1', EXPECTED_COMMIT=GROK_CURRENT_HEAD,
        GITHUB_RUN_ID=GROK_CURRENT_RUN, GITHUB_RUN_ATTEMPT='1', GITHUB_EVENT_NAME='pull_request')
    offset = 20 + len(env)
    require(len(argv) == offset + 9 and argv[20:offset] == [key + '=' + env[key] for key in sorted(env)])
    python, installer, probe = argv[offset], argv[offset + 4], argv[offset + 6]
    require(re.fullmatch(r'/usr/bin/python3(?:\.[0-9]{1,2})?', python)
            and argv[offset + 1:offset + 4] == ['-B', source + '/scripts/grok_noauth_call_e2e.py', '--installer']
            and argv[offset + 5] == '--probe' and argv[offset + 7:] == ['--output', output + '/child'])
    grok_path(installer)
    grok_path(probe)
    require(PurePosixPath(installer).name == 'agentplugins' and PurePosixPath(probe).name == 'probe'
            and PurePosixPath(installer).parent == PurePosixPath(probe).parent)
    return installer, streams[5]['stderr']

def grok_public_dto(row, raw, installer):
    result = dict(state='not-applicable')
    if row['probe'] not in GROK_PUBLIC:
        return result
    verb, extra = GROK_PUBLIC[row['probe']]
    argv = row['argv']
    if not (len(argv) == 11 + len(extra) and argv[0] == installer and argv[1] == verb
            and argv[3:11] == ['--scope', 'project', '--project-root', row['cwd'], '--target', 'grok', '--format', 'json']
            and argv[11:] == list(extra)):
        return dict(state='unvalidated-role')
    if not raw:
        return dict(state='empty')
    try:
        value = closed(grok_document(raw), 'schema_version command result data')
        require(type(value['schema_version']) is int and value['schema_version'] == 1
                and value['command'] == verb and value['result'] in ('failure', 'success') and type(value['data']) is dict)
        nested = value['data'].get('result')
        require(type(nested) is dict and type(nested.get('mutated')) is bool)
        activation = nested.get('activation')
        require(type(activation) is dict and activation.get('activation') in
                ('not_requested', 'prepared', 'active', 'failed', 'blocked', 'unsupported', 'unknown'))
        result = dict(state='validated-public-fields', command=verb, result=value['result'],
                      mutated=nested['mutated'], activation=activation['activation'])
    except (ValueError, TypeError, KeyError, RecursionError):
        return dict(state='unvalidated')
    return result

def grok_failure_readout(evidence, rows, streams, installer, parent_stderr, locations, guards):
    error = grok_string(evidence['error'])
    kind, separator, message = error.partition(': ')
    primary_kind = kind if separator and kind in GROK_ERROR_KINDS else 'unclassified'
    primary_code = ('command_failed' if kind == 'ValueError' and message.startswith('command failed: ')
                    and message[16:] in GROK_CHILD_LABELS else 'known_literal' if message in guards else 'unclassified')
    result = dict(primary_error_kind=primary_kind, primary_error_code=primary_code,
        primary_error_bytes=len(error.encode('utf-8')), primary_error_sha256=digest(error.encode('utf-8')),
        failed_command_present='failed_command' in evidence)
    selected, selected_stderr = None, b''
    if 'failed_command' in evidence:
        failed = closed(evidence['failed_command'], 'label exit capture_complete protected_originals_unchanged')
        require(rows and failed['label'] == rows[-1]['probe'] and type(failed['exit']) is int
                and failed['exit'] == rows[-1]['exit'] and type(failed['capture_complete']) is bool
                and failed['capture_complete'] is (rows[-1]['error'] is None)
                and type(failed['protected_originals_unchanged']) is bool)
        selected, selected_streams = rows[-1], streams[-1]
        selected_stderr = selected_streams['stderr']
        result['failed_command'] = dict(failed, primary_error_matches=error == 'ValueError: command failed: ' + failed['label'],
            stdout_bytes=selected['stdout_bytes'], stdout_sha256=selected['stdout_sha256'],
            stderr_bytes=selected['stderr_bytes'], stderr_sha256=selected['stderr_sha256'],
            public_dto=grok_public_dto(selected, selected_streams['stdout'], installer))
    frames, found_guards, classes, total = [], set(), set(), 0
    names = '|'.join(re.escape(name) for name in locations)
    pattern = re.compile(r'^  File "(?:[^"\r\n]*/)?(' + names + r')", line ([1-9][0-9]{0,5}), in ([A-Za-z_][A-Za-z_0-9.]{0,100}|<[a-z]+>)$')
    for raw in (parent_stderr, selected_stderr):
        lines = raw.decode('utf-8', 'replace').splitlines()
        require(len(lines) <= 32768)
        for line in lines:
            match = pattern.fullmatch(line)
            if match:
                require((match[3], int(match[2])) in locations[match[1]])
                total += 1
                require(total <= 64)
                if len(frames) < 8:
                    frames.append(dict(file=match[1], line=int(match[2]), function=match[3]))
            for class_name in GROK_ERROR_KINDS:
                if line.startswith(class_name + ': '):
                    classes.add(class_name)
                    if line[len(class_name) + 2:] in guards:
                        found_guards.add(line[len(class_name) + 2:])
            candidate = line[len(GROK_STDERR_PREFIX):] if line.startswith(GROK_STDERR_PREFIX) else line
            if candidate in guards:
                found_guards.add(candidate)
    if message in guards:
        found_guards.add(message)
    result.update(traceback_frames=frames, traceback_count=total, traceback_truncated=total > len(frames),
        stream_error_kinds=sorted(classes), known_guard_messages=sorted(found_guards)[:8],
        known_guard_count=len(found_guards), known_guards_truncated=len(found_guards) > 8)
    return result

def project_current_grok(members, source_facts):
    require(type(members) is dict and set(members) == {'evidence.json', 'delegation.json'})
    evidence, receipt = grok_document(members['evidence.json']), grok_document(members['delegation.json'])
    installer, parent_stderr = grok_parent(receipt, members['evidence.json'])
    require(type(evidence) is dict and GROK_REQUIRED <= set(evidence) <= GROK_REQUIRED | GROK_OPTIONAL
            and type(evidence['schema_version']) is int and evidence['schema_version'] == 1
            and evidence['source_commit'] == GROK_CURRENT_HEAD and evidence['source_tree'] == GROK_CURRENT_TREE
            and evidence['github_run_id'] == GROK_CURRENT_RUN and evidence['github_run_attempt'] == '1'
            and evidence['github_event'] == 'pull_request' and evidence['platform'] == 'Linux-x86_64'
            and all(evidence[key] is False for key in GROK_FALSE) and evidence['scripted_local_provider'] is True
            and all(evidence[key] == receipt[key] for key in ('installer_sha256', 'probe_sha256'))
            and type(evidence['stages']) is list and len(evidence['stages']) <= len(GROK_STAGES)
            and all(type(row) is dict for row in evidence['stages'])
            and tuple(row.get('stage') for row in evidence['stages']) == GROK_STAGES[:len(evidence['stages'])])
    rows = evidence['commands']
    require(type(rows) is list and len(rows) <= 128)
    streams = [grok_record(row, True) for row in rows]
    require(len({row['probe'] for row in rows}) == len(rows))
    if 'acquisition' in evidence:
        acquisition(evidence['acquisition'])
    for row, raw in zip(rows, streams):
        if row['probe'] == 'version':
            require('acquisition' in evidence and row['argv'][1:] == ['--version']
                    and PurePosixPath(row['argv'][0]).name == 'grok'
                    and raw['stdout'] == b'grok 1.0.70 (f50904eaee6c)\n')
    if 'events_base64' in evidence or 'events_sha256' in evidence:
        require('events_base64' in evidence and 'events_sha256' in evidence and type(evidence.get('all_original_events')) is list)
        encoded = evidence['events_base64']
        require(type(encoded) is str and len(encoded) <= 4 * (((1 << 20) + 2) // 3))
        raw = base64.b64decode(encoded, validate=True)
        grok_stream(encoded, len(raw), evidence['events_sha256'])
        require([grok_document(line) for line in raw.splitlines() if line.strip()] == evidence['all_original_events'])
    elif 'all_original_events' in evidence:
        require(evidence['all_original_events'] == [])
    result = dict(kind='fixed-original-readout', case=GROK_CURRENT_CASE, artifact_id=GROK_CURRENT_ARTIFACT,
        source=GROK_CURRENT_HEAD, tree=GROK_CURRENT_TREE, run=GROK_CURRENT_RUN, attempt='1',
        original_failed=True, native_qualified=False, qualification=dict.fromkeys(GROK_FALSE, False),
        reported_service_closed=True, reported_forced_stop=False, reported_forced_kill=False,
        parent_commands=10, parent_stream_joins=20, child_commands=len(rows), child_stream_joins=2 * len(rows),
        stage_prefix_count=len(evidence['stages']), source_code_imported=False)
    result.update(grok_failure_readout(evidence, rows, streams, installer, parent_stderr, *source_facts))
    return result

def audit_current_grok(body):
    require(type(body) is bytes and len(body) == GROK_CURRENT_SIZE and digest(body) == GROK_CURRENT_HASH)
    members = unpack(body, {'evidence.json', 'delegation.json'})
    result = project_current_grok(members, grok_current_sources())
    result.update(archive_bytes=len(body), archive_sha256=digest(body), member_count=2, crc_verified=True)
    return result



ANTIG_CASE = 'antigravity-440-coldrepair'
ANTIG_ARTIFACT = 14106353842
ANTIG_ARCHIVE_SIZE = 14776
ANTIG_ARCHIVE_HASH = '4b77301410328eedb1e251e7a0cb210908cd6b8066ab8d77fc12a422d7049633'
ANTIG_SOURCE = '440f0666dad9255c1667c86f95654f7d960ffeb6'
ANTIG_TREE = '5efcc6f7d254089f111a0097dc968c87299882dc'
ANTIG_RUN = '43574321243'
ANTIG_PRODUCER = '6ef10f40cb6a41e362f73fbeab6514343cf09155'
ANTIG_SUPERVISOR = 'dbeec7b9fec275d166ac8d341edb1183a8584b05'
ANTIG_NATIVE_ACTIONS = tuple((initial, action) for initial, actions in (
    (False, ('version', 'list', 'enable', 'list', 'disable', 'list')),
    (True, ('version', 'list', 'enable', 'list', 'disable', 'list', 'enable', 'list')))
    for action in actions)
ANTIG_PUBLIC_ACTIONS = ('uap-add', 'uap-repeat', 'uap-repeat', 'uap-update', 'uap-repair',
                       'uap-repair', 'uap-remove', 'uap-add', 'uap-repeat', 'uap-saved-add',
                       'uap-cold-install', 'uap-cold-observe')
ANTIG_PUBLIC_WORLDS = ('enabled',) * 7 + ('disabled',) * 2 + ('saved', 'cold', 'cold')
ANTIG_FLAGS = ('native_project_qualified', 'ide_activation', 'native_mcp_call',
               'vendor_authentication', 'real_model_inference')
ANTIG_REQUEST_FIELDS = 'binary binary_sha256 root selector action source_commit source_tree deadline_unix_ns'
ANTIG_RECEIPT_FIELDS = ('source_commit source_tree root selector action binary_sha256 cleanup_complete '
                        'stdout_base64 stdout_bytes stdout_sha256')
ANTIG_MANIFEST_FIELDS = ('schema_version source_commit source_tree native_sha256 helper_sha256 expected_slots '
    'evidence_root_identity capture_complete harness_complete slots native_project_qualified ide_activation '
    'native_mcp_call vendor_authentication real_model_inference fixtures_retained public_expected_slots '
    'public_slots installer_sha256 public_capture_complete public_effects_verified')
ANTIG_MANUAL_ACTIONS = (
    'The selected User plugin is enabled in CLI inventory; open or reload Antigravity to verify IDE loading',
    'The selected User plugin remains disabled; decide whether to enable it in Antigravity',
    'The exact UAP-owned plugin is absent from native User inventory; inspect it in Antigravity')
ANTIG_MEMBERS = {'manifest.json'} | {
    prefix + str(index).zfill(2) + '.' + suffix + '.json'
    for prefix, count in (('', 14), ('public-', 12))
    for index in range(count) for suffix in ('request', 'receipt')}

def antig_pin(value, length=64):
    require(type(value) is str and re.fullmatch('[0-9a-f]{' + str(length) + '}', value) is not None)

def antig_path(value, *, root=False):
    require(type(value) is str and 0 < len(value) <= 4096 and value.startswith('/')
            and not value.startswith('//') and not any(ord(char) < 32 for char in value)
            and str(PurePosixPath(value)) == value and '..' not in PurePosixPath(value).parts)
    if root:
        require(PurePosixPath(value).name.startswith('uap-agy-direct-'))

def antig_frame(body, maximum):
    require(type(body) is bytes and 0 < len(body) <= maximum and body.endswith(b'\n')
            and body.count(b'\n') == 1)
    return document(body, maximum)

def antig_stream(receipt, name, bound):
    encoded, size, expected = (receipt[name + suffix] for suffix in ('_base64', '_bytes', '_sha256'))
    require(type(encoded) is str and len(encoded) <= ((bound + 2) // 3) * 4
            and type(size) is int and 0 <= size < bound)
    antig_pin(expected)
    raw = base64.b64decode(encoded, validate=True)
    require(base64.b64encode(raw).decode('ascii') == encoded and len(raw) == size and digest(raw) == expected)
    return raw

def antig_pair(members, row, index, *, public, manifest):
    prefix = 'public-' if public else ''
    require(type(row['files']) is list and len(row['files']) == 2)
    values = []
    for suffix, entry in zip(('request', 'receipt'), row['files']):
        closed(entry, 'name bytes sha256')
        name = prefix + str(index).zfill(2) + '.' + suffix + '.json'
        raw = members[name]
        require(entry['name'] == name and type(entry['bytes']) is int and entry['bytes'] == len(raw)
                and entry['sha256'] == digest(raw))
        values.append(antig_frame(raw, (1 << 20) if public else 16384))
    request, receipt = values
    closed(request, ANTIG_REQUEST_FIELDS)
    closed(receipt, ANTIG_RECEIPT_FIELDS + (' exit stderr_base64 stderr_bytes stderr_sha256' if public else ''))
    pin = manifest['installer_sha256' if public else 'native_sha256']
    require(request['source_commit'] == ANTIG_SOURCE and request['source_tree'] == ANTIG_TREE
            and request['action'] == row['action'] and request['binary_sha256'] == pin
            and type(request['deadline_unix_ns']) is int and 0 < request['deadline_unix_ns'] < 1 << 63)
    antig_path(request['binary'])
    antig_path(request['root'], root=True)
    require(type(request['selector']) is str
            and re.fullmatch('[A-Za-z0-9][A-Za-z0-9_.-]{0,127}', request['selector']) is not None
            and request['selector'] not in ('.', '..'))
    if public:
        selector = 'uap-cline-public-probe' if index >= 9 else 'uap-agy-native-smoke'
        require(request['selector'] == selector and type(receipt['exit']) is int and receipt['exit'] == 0)
    require(receipt['cleanup_complete'] is True and all(receipt[key] == request[key] for key in
            ('source_commit', 'source_tree', 'action', 'selector', 'root', 'binary_sha256')))
    stdout = antig_stream(receipt, 'stdout', 1 << 20)
    if public:
        antig_stream(receipt, 'stderr', 32768)
    return request, stdout

def antig_recorded_authorities(requests):
    worlds, selectors, binaries = {}, {}, {}
    for world, family, request in requests:
        root = request['root']
        if world in worlds:
            require(worlds[world] == root and selectors[world] == request['selector'])
        else:
            require(root not in worlds.values())
            worlds[world], selectors[world] = root, request['selector']
        if family in binaries:
            require(binaries[family] == request['binary'])
        else:
            binaries[family] = request['binary']
    require(len(worlds) == 6 and set(binaries) == {'native', 'public'})

def antig_value_shape(value, allowed=()):
    if type(value) is str:
        raw = value.encode('utf-8', errors='surrogatepass')
        result = dict(type='string', bytes=len(raw), sha256=digest(raw))
        if value in allowed:
            result['known_value'] = value
        return result
    if value is None:
        return dict(type='null')
    if type(value) is bool:
        return dict(type='boolean', value=value)
    if type(value) is int:
        return dict(type='integer', value=value) if -(1 << 63) <= value < 1 << 63 else dict(type='integer', bounded=False)
    if type(value) is list or type(value) is dict:
        return dict(type='array' if type(value) is list else 'object', count=len(value))
    return dict(type='number' if type(value) is float else 'unrepresented')

def antig_fields(value, fields):
    result = antig_value_shape(value)
    if type(value) is not dict:
        return result
    result['fields'] = {key: dict(present=True, **antig_value_shape(value[key], allowed)) if key in value
                       else dict(present=False) for key, allowed in fields.items()}
    unknown = sorted(set(value) - set(fields))
    result.update(unknown_key_count=len(unknown),
                  unknown_key_sha256=[digest(key.encode('utf-8', errors='surrogatepass')) for key in unknown[:4]],
                  unknown_keys_truncated=len(unknown) > 4)
    return result

def antig_dto_projection(body):
    value = document(body)
    top = dict(schema_version=(), command=('agentplugins repair', 'doctor'), result=('success', 'failure'),
               outcome=('completed', 'partial', 'failed'), scope=('user', 'project'),
               scope_selection_reason=('explicit',), actions=(), results=(), data=())
    action_fields = dict(alias=('uap-cline-public-probe',), client_id=('antigravity',), installation_id=(),
        binding_id=(), action=('repair',), from_version=('1.0.0',), to_version=('1.0.0',),
        from_revision=(), revision=(), from_digest=(), package_digest=(), reason=(), component_support=())
    result_fields = dict(installation_id=(), client_id=('antigravity',), operation=('repair',),
                        outcome=('unchanged', 'completed', 'failed'), reason=(), next_actions=(), manual_actions=())
    projected = dict(envelope=antig_fields(value, top))
    if type(value) is not dict:
        return projected
    for key, fields in (('actions', action_fields), ('results', result_fields)):
        if key not in value:
            continue
        rows = value[key]
        projection = dict(shape=antig_value_shape(rows))
        if type(rows) is list:
            projection.update(projected_rows=1 if rows else 0, truncated=len(rows) > 1)
            if rows:
                row = rows[0]
                projection['first'] = antig_fields(row, fields)
                if type(row) is dict and key == 'actions' and 'component_support' in row:
                    components = row['component_support']
                    projection['components'] = dict(shape=antig_value_shape(components))
                    if type(components) is list:
                        projection['components'].update(
                            rows=[antig_fields(component, dict(kind=('skill', 'mcp_server'),
                                name=('check', 'uap_public_probe'), support=('native', 'unsupported')))
                                for component in components[:4]], truncated=len(components) > 4)
                if type(row) is dict and key == 'results' and 'manual_actions' in row:
                    manual = row['manual_actions']
                    projection['manual_actions'] = dict(shape=antig_value_shape(manual))
                    if type(manual) is list:
                        projection['manual_actions'].update(rows=[antig_value_shape(item, ANTIG_MANUAL_ACTIONS)
                            for item in manual[:4]], truncated=len(manual) > 4)
        projected[key] = projection
    return projected

def audit_current_antigravity(body):
    require(type(body) is bytes and len(body) == ANTIG_ARCHIVE_SIZE and digest(body) == ANTIG_ARCHIVE_HASH)
    outer = unpack(body, {'receipts.zip'})
    members = unpack(outer['receipts.zip'], ANTIG_MEMBERS)
    manifest = closed(document(members['manifest.json'], 32768), ANTIG_MANIFEST_FIELDS)
    require(type(manifest['schema_version']) is int and manifest['schema_version'] == 2
            and manifest['source_commit'] == ANTIG_SOURCE and manifest['source_tree'] == ANTIG_TREE
            and type(manifest['expected_slots']) is int and manifest['expected_slots'] == 14
            and type(manifest['public_expected_slots']) is int and manifest['public_expected_slots'] == 23
            and manifest['fixtures_retained'] is True
            and all(manifest[key] is False for key in ANTIG_FLAGS +
                    ('capture_complete', 'harness_complete', 'public_capture_complete', 'public_effects_verified')))
    for key in ('native_sha256', 'helper_sha256', 'installer_sha256'):
        antig_pin(manifest[key])
    identity = closed(manifest['evidence_root_identity'], 'path held')
    for row in identity.values():
        require(type(row) is list and len(row) == 5 and all(type(item) is int and 0 <= item < 1 << 64 for item in row)
                and row[1] != 0 and stat.S_ISDIR(row[2]))
    require(identity['path'][:2] == identity['held'][:2] and stat.S_IMODE(identity['held'][2]) == 0o700)
    require(type(manifest['slots']) is list and len(manifest['slots']) == 14
            and type(manifest['public_slots']) is list and len(manifest['public_slots']) == 12)
    requests, previous = [], {}
    for index, (row, (initial, action)) in enumerate(zip(manifest['slots'], ANTIG_NATIVE_ACTIONS)):
        closed(row, 'initial_enabled action files effects_verified config_before_sha256 config_after_sha256')
        require(row['initial_enabled'] is initial and row['action'] == action and row['effects_verified'] is True)
        for key in ('config_before_sha256', 'config_after_sha256'):
            antig_pin(row[key])
        if initial in previous:
            require(row['config_before_sha256'] == previous[initial])
        if action in ('version', 'list'):
            require(row['config_before_sha256'] == row['config_after_sha256'])
        previous[initial] = row['config_after_sha256']
        request, _ = antig_pair(members, row, index, public=False, manifest=manifest)
        requests.append(('native-enabled' if initial else 'native-disabled', 'native', request))
    selected = None
    for index, (row, action, world) in enumerate(zip(manifest['public_slots'], ANTIG_PUBLIC_ACTIONS, ANTIG_PUBLIC_WORLDS)):
        closed(row, 'world action files')
        require(row['action'] == action and row['world'] == world)
        request, stdout = antig_pair(members, row, index, public=True, manifest=manifest)
        requests.append(('public-' + world, 'public', request))
        if index == 11:
            selected = stdout
    antig_recorded_authorities(requests)
    require(type(selected) is bytes)
    return dict(kind='fixed-original-readout', case=ANTIG_CASE, artifact_id=ANTIG_ARTIFACT,
        source=ANTIG_SOURCE, tree=ANTIG_TREE, run=ANTIG_RUN, original_failed=True, native_qualified=False,
        qualification=dict.fromkeys(ANTIG_FLAGS, False), capture_complete=False, harness_complete=False,
        public_capture_complete=False, public_effects_verified=False, recorded_cleanup_complete=True,
        native_slots=14, public_slots=12, selected_public_index=11, selected_public_exit=0, stream_joins=38,
        member_count=len(members), crc_verified=True, archive_bytes=len(body), archive_sha256=digest(body),
        inner_archive_sha256=digest(outer['receipts.zip']), stdout_bytes=len(selected), stdout_sha256=digest(selected),
        nonce_recorded=False, runtime_argv_recorded=False,
        source_derived_argv=['<captured-installer>', 'repair', 'uap-cline-public-probe',
                            '--target', 'antigravity', '--scope', 'user', '--yes', '--format', 'json'],
        producer_blob=ANTIG_PRODUCER, supervisor_blob=ANTIG_SUPERVISOR, dto=antig_dto_projection(selected))

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('case', choices=(CASE, *HERMES_CASES, GROK_CURRENT_CASE, ANTIG_CASE))
    parser.add_argument('archive', type=Path)
    args = parser.parse_args()
    try:
        with args.archive.open('rb') as source:
            size = (ARCHIVE_SIZE if args.case == CASE else GROK_CURRENT_SIZE if args.case == GROK_CURRENT_CASE
                    else ANTIG_ARCHIVE_SIZE if args.case == ANTIG_CASE else HERMES_CASES[args.case][1])
            body = source.read(size + 1)
        result = (audit(body) if args.case == CASE else audit_current_grok(body) if args.case == GROK_CURRENT_CASE
                  else audit_current_antigravity(body) if args.case == ANTIG_CASE else audit_hermes(body, args.case))
        output = encode(result)
    except (ValueError, OSError, UnicodeError, KeyError, TypeError, RecursionError,
            zipfile.BadZipFile, RuntimeError, struct.error, zlib.error):
        output = encode(dict(kind='fixed-original-readout', case=args.case, refused=True, native_qualified=False))
        status = 1
    else:
        status = 0
    print(output.decode('ascii'), end='')
    return status

if __name__ == '__main__':
    raise SystemExit(main())
