#!/usr/bin/env python3
"""Finite hosted Grok ACP metadata capture; all product qualifications stay false."""
import argparse
import base64
import hashlib
import json
import math
import os
from pathlib import Path
import re
import selectors
import shutil
import socket
import subprocess
import sys
import time

LIMIT = 1 << 20
SCOPES = ('user', 'a', 'b')
LABELS = ('version', 'help', 'agent-help', 'stdio-help', 'acp')
FALSE = dict.fromkeys(('inventory_admitted', 'native_enablement', 'workspace_trust', 'native_mcp_call',
                       'provider_authentication', 'real_model_inference', 'production_project_activation'), False)


def require(value, reason):
    if not value:
        raise ValueError(reason)


def digest(body):
    return hashlib.sha256(body).hexdigest()


def closed(value, keys):
    require(type(value) is dict and set(value) == set(keys.split()), 'closed-fields')
    return value


def document(body, limit=LIMIT):
    require(isinstance(body, bytes) and 0 < len(body) <= limit, 'json-bound')
    def pairs(rows):
        result = {}
        for key, value in rows:
            require(key not in result, 'duplicate-key')
            result[key] = value
        return result
    def finite(token):
        value = float(token)
        require(math.isfinite(value), 'nonfinite')
        return value
    value = json.loads(body.decode('utf-8', 'strict'), object_pairs_hook=pairs, parse_float=finite,
                       parse_constant=lambda _: (_ for _ in ()).throw(ValueError('nonfinite')))
    def walk(item, depth=0):
        require(depth <= 16, 'json-depth')
        if isinstance(item, (dict, list)):
            require(len(item) <= 512, 'json-count')
            for child in item.values() if isinstance(item, dict) else item:
                walk(child, depth + 1)
    walk(value)
    return value


def stable(info):
    return (info.st_dev, info.st_ino, info.st_mode, info.st_nlink, info.st_uid, info.st_gid,
            info.st_size, info.st_mtime_ns, info.st_ctime_ns)


def ordinary(path, limit=LIMIT):
    import stat
    info = path.lstat()
    require(stat.S_ISREG(info.st_mode) and info.st_nlink == 1 and 0 <= info.st_size <= limit, 'ordinary-bound')
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC)
    with os.fdopen(fd, 'rb') as stream:
        require(stable(info) == stable(os.fstat(stream.fileno())), 'open-identity')
        body = stream.read(limit + 1)
        require(len(body) == info.st_size and stable(info) == stable(os.fstat(stream.fileno()))
                == stable(path.lstat()), 'read-identity')
    return body


def isolation(req):
    with open('/proc/self/status', 'rb') as stream:
        body = stream.read(65537)
    require(len(body) <= 65536, 'status-bound')
    status = dict(line.split(':', 1) for line in body.decode('ascii').splitlines() if ':' in line)
    value = dict(uid=os.getuid(), gid=os.getgid(), euid=os.geteuid(), egid=os.getegid(), groups=os.getgroups(),
                 network_namespace=os.stat('/proc/self/ns/net').st_ino, parent_network_namespace=req['parent_network_namespace'],
                 interfaces=sorted(name for _, name in socket.if_nameindex()),
                 capabilities_zero=all(k in status and int(status[k].strip(), 16) == 0
                                       for k in ('CapInh', 'CapPrm', 'CapEff', 'CapBnd', 'CapAmb')),
                 no_new_privileges=int(status.get('NoNewPrivs', '0').strip()))
    require(value['uid'] == value['euid'] == req['uid'] != 0 and value['gid'] == value['egid'] == req['gid'] != 0
            and not value['groups'] and value['interfaces'] == ['lo'] and value['capabilities_zero']
            and value['no_new_privileges'] == 1 and value['network_namespace'] != value['parent_network_namespace'], 'isolation')
    return value


def rpc_state():
    return dict(initialize=None, plugins=None, notifications=[])


def rpc_frame(value, state, req, send):
    require(type(value) is dict and value.get('jsonrpc') == '2.0', 'rpc-envelope')
    if 'method' in value:
        closed(value, 'jsonrpc method params')
        require(value['method'] == '_x.ai/mcp/servers_updated' and value['params'] == {'mcpServers': []}
                and not state['notifications'], 'notification-refused')
        state['notifications'].append(value)
        return False
    closed(value, 'jsonrpc id result')
    require(type(value['id']) is int, 'rpc-id')
    if state['initialize'] is None:
        require(value['id'] == 1 and type(value['result']) is dict, 'initialize-id')
        result = value['result']
        require(type(result.get('protocolVersion')) is int and result['protocolVersion'] == 1
                and type(result.get('_meta')) is dict
                and result['_meta'].get('agentVersion') in ('1.0.70', '1.0.70 (f50904eaee6c)'), 'initialize-version')
        state['initialize'] = value
        send({'jsonrpc': '2.0', 'id': 2, 'method': '_x.ai/plugins/list',
              'params': {'sessionId': 'uap-admission-' + req['nonce'] + '-' + req['scope']}})
        return False
    require(value['id'] == 2 and state['plugins'] is None, 'listing-id')
    closed(value['result'], 'plugins')
    require(type(value['result']['plugins']) is list and len(value['result']['plugins']) <= 128, 'plugins-bound')
    state['plugins'] = value
    return True


def help_admitted(commands):
    require(len(commands) == 4 and tuple(row['label'] for row in commands) == LABELS[:4], 'help-order')
    require(all(row['exit'] == 0 and row['error'] is None for row in commands), 'help-exit')
    bodies = [base64.b64decode(row['merged_base64'], validate=True) for row in commands]
    require(bodies[0] in (b'grok 1.0.70 (f50904eaee6c)\n', b'grok 1.0.70 (f50904eaee6c)\r\n'), 'version')
    require(re.search(rb'(?m)^\s+agent\s', bodies[1]) and re.search(rb'(?m)^\s+stdio\s', bodies[2])
            and b'--no-leader' in bodies[2] and b'Usage:' in bodies[3] and b'stdio' in bodies[3], 'help-grammar')


def drain_stopped(pipe, body, deadline, remaining):
    """Require the complete held stream, never a clipped negative receipt."""
    require(len(body) <= remaining, 'stream-bound')
    with selectors.DefaultSelector() as selector:
        selector.register(pipe, selectors.EVENT_READ)
        while selector.get_map():
            require(time.monotonic() < deadline, 'drain-deadline')
            for key, _ in selector.select(min(.05, max(.001, deadline - time.monotonic()))):
                chunk = os.read(key.fd, 65536)
                if not chunk:
                    selector.unregister(key.fileobj)
                else:
                    body.extend(chunk)
                    require(len(body) <= remaining, 'stream-bound')


def command(argv, label, cwd, env, deadline, remaining, state, req):
    process = subprocess.Popen(argv, cwd=cwd, env=env, stdin=subprocess.PIPE,
                               stdout=subprocess.PIPE, stderr=subprocess.STDOUT, start_new_session=True)
    body, buffer, error, completed = bytearray(), bytearray(), None, False
    def send(value):
        frame = json.dumps(value, separators=(',', ':')).encode() + b'\n'
        require(len(frame) <= 4096, 'request-bound')
        process.stdin.write(frame)
        process.stdin.flush()
    try:
        with selectors.DefaultSelector() as selector:
            os.set_blocking(process.stdout.fileno(), False)
            selector.register(process.stdout, selectors.EVENT_READ)
            if label == 'acp':
                send({'jsonrpc': '2.0', 'id': 1, 'method': 'initialize', 'params': {'protocolVersion': 1,
                      'clientCapabilities': {}, 'clientInfo': {'name': 'uap-inventory-admission', 'version': '1'}}})
            else:
                process.stdin.close()
            frames = 0
            while selector.get_map() or process.poll() is None:
                require(time.monotonic() < deadline, 'deadline')
                for key, _ in selector.select(min(.05, max(.001, deadline - time.monotonic()))):
                    chunk = os.read(key.fd, 65536)
                    if not chunk:
                        selector.unregister(key.fileobj)
                        continue
                    body.extend(chunk)
                    require(len(body) <= remaining, 'stream-bound')
                    if label == 'acp':
                        buffer.extend(chunk)
                        while b'\n' in buffer:
                            line, _, buffer = buffer.partition(b'\n')
                            frames += 1
                            require(frames <= 64 and len(line) <= 64 << 10 and not completed, 'frame-bound')
                            completed = rpc_frame(document(bytes(line), 64 << 10), state, req, send)
                            require(len(json.dumps(state).encode()) <= 128 << 10, 'rpc-bound')
                            if completed:
                                process.stdin.close()
                                process.terminate()
                        require(len(buffer) <= 64 << 10, 'frame-bound')
            process.wait(timeout=max(.001, deadline - time.monotonic()))
            require(label != 'acp' or (completed and not buffer), 'rpc-incomplete')
    except (ValueError, OSError, subprocess.SubprocessError):
        error = 'deadline' if time.monotonic() >= deadline else 'protocol-refused' if label == 'acp' else 'process-refused'
    finally:
        process.kill()  # Popen protects already-reaped PID reuse; not tree authority.
        process.wait(timeout=max(.001, deadline - time.monotonic()))
        process.stdin.close()
        try:
            drain_stopped(process.stdout, body, deadline, remaining)
        finally:
            process.stdout.close()
    raw = bytes(body)
    return dict(label=label, argv=argv, exit=process.returncode, error=error,
                merged_base64=base64.b64encode(raw).decode(), merged_bytes=len(raw), merged_sha256=digest(raw))


def collector_result(req):
    root, native = Path(req['input_root']), Path(__file__).resolve().parent / 'grok'
    result = {key: req[key] for key in ('schema_version', 'source_commit', 'source_tree', 'scope', 'nonce')}
    result.update(semantic_complete=False, reason='isolation-refused', isolation={}, commands=[], rpc=rpc_state())
    deadline, command_in_flight = req['start_monotonic_ns'] / 1e9 + 30, False
    try:
        result['isolation'] = isolation(req)
        require(req['scope'] in SCOPES and re.fullmatch('[0-9a-f]{32}', req['nonce']), 'request-identity')
        env = {'PATH': '/usr/bin:/bin', 'LANG': 'C.UTF-8', 'LC_ALL': 'C.UTF-8', 'HOME': str(root / 'home'),
               'TMPDIR': str(root / 'tmp'), 'TERM': 'dumb', 'NO_COLOR': '1', 'GROK_DISABLE_AUTOUPDATER': '1',
               'GIT_CONFIG_NOSYSTEM': '1', 'GIT_CONFIG_GLOBAL': '/dev/null', 'GIT_TERMINAL_PROMPT': '0'}
        env.update({'XDG_' + name.upper() + '_HOME': str(root / ('xdg-' + name)) for name in ('config', 'data', 'cache', 'state')})
        result['reason'] = 'help-refused'
        for label, args in zip(LABELS[:4], (['--version'], ['--help'], ['agent', '--help'], ['agent', 'stdio', '--help'])):
            remaining = LIMIT - sum(row['merged_bytes'] for row in result['commands'])
            command_in_flight = True
            row = command([str(native), *args], label, root / req['scope'], env,
                          min(deadline, time.monotonic() + 5), remaining, result['rpc'], req)
            command_in_flight = False
            result['commands'].append(row)
            require(row['error'] is None, 'help-command')
        help_admitted(result['commands'])
        result['reason'] = 'protocol-refused'
        command_in_flight = True
        row = command([str(native), 'agent', '--no-leader', 'stdio'], 'acp', root / req['scope'], env, deadline,
                      LIMIT - sum(row['merged_bytes'] for row in result['commands']), result['rpc'], req)
        command_in_flight = False
        result['commands'].append(row)
        require(row['error'] is None and result['rpc']['plugins'] is not None, 'acp-command')
        result.update(semantic_complete=True, reason='complete')
    except (ValueError, OSError, subprocess.SubprocessError):
        if command_in_flight:
            raise  # No JSON/result authority for any incomplete command capture.
    return result


def collect(request):
    req = closed(document(ordinary(request, 16384), 16384), 'schema_version source_commit source_tree uid gid nonce scope input_root parent_network_namespace start_monotonic_ns')
    body = json.dumps(collector_result(req), separators=(',', ':')).encode() + b'\n'
    require(len(body) <= 2 << 20, 'result-bound')
    sys.stdout.buffer.write(body)
    sys.stdout.buffer.flush()
    sys.stdout.close()
    sys.stderr.close()
    sys.stdin.buffer.read(1)
    while True:
        time.sleep(1)  # Strict parent owns termination after closed pipe EOF.


def helper_capture(helper, root, deadline):
    argv = ['/usr/bin/sudo', '-n', '--preserve-env=GITHUB_ACTIONS,RUNNER_ENVIRONMENT,AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED,EXPECTED_COMMIT', str(helper), '--input-root', str(root)]
    env = {key: os.environ[key] for key in ('GITHUB_ACTIONS', 'RUNNER_ENVIRONMENT', 'AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED', 'EXPECTED_COMMIT') if key in os.environ}
    env.update(PATH='/usr/bin:/bin', LANG='C.UTF-8')
    process = subprocess.Popen(argv, env=env, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True)
    streams = {'stdout': bytearray(), 'stderr': bytearray()}
    try:
        with selectors.DefaultSelector() as selector:
            for name, pipe in (('stdout', process.stdout), ('stderr', process.stderr)):
                os.set_blocking(pipe.fileno(), False)
                selector.register(pipe, selectors.EVENT_READ, name)
            while selector.get_map() or process.poll() is None:
                require(time.monotonic() < deadline, 'helper-deadline')
                for key, _ in selector.select(.05):
                    chunk = os.read(key.fd, 65536)
                    if not chunk:
                        selector.unregister(key.fileobj)
                    else:
                        streams[key.data].extend(chunk)
                        require(len(streams[key.data]) <= ((2 << 20) + 16384 if key.data == 'stdout' else 65536), 'helper-stream-bound')
            require(process.wait(timeout=1) == 0 and not streams['stderr'], 'helper-refused')
            return document(bytes(streams['stdout']), (2 << 20) + 16384)
    finally:
        process.stdin.close()  # EOF cancels root helper; never a raw cleanup proof.
        if process.poll() is None:
            try:
                process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait(timeout=1)
        process.stdout.close()
        process.stderr.close()


def validate_receipt(receipt, request):
    closed(receipt, 'schema_version source_commit source_tree scope nonce cleanup_complete sealed_cleanup_complete system_executables result')
    for key in ('schema_version', 'source_commit', 'source_tree', 'scope', 'nonce'):
        require(type(receipt[key]) is type(request[key]) and receipt[key] == request[key], 'helper-identity')
    require(receipt['cleanup_complete'] is True and receipt['sealed_cleanup_complete'] is True, 'tree-cleanup')
    for name, record in closed(receipt['system_executables'], 'unshare setpriv env python3').items():
        closed(record, 'path sha256')
        require(record['path'] == '/usr/bin/' + ('python3.12' if name == 'python3' else name)
                and type(record['sha256']) is str and re.fullmatch('[0-9a-f]{64}', record['sha256']), 'system-executable')
    result = closed(receipt['result'], 'schema_version source_commit source_tree scope nonce semantic_complete reason isolation commands rpc')
    for key in ('schema_version', 'source_commit', 'source_tree', 'scope', 'nonce'):
        require(type(result[key]) is type(request[key]) and result[key] == request[key], 'collector-identity')
    require(type(result['semantic_complete']) is bool and result['reason'] in ('complete', 'protocol-refused', 'help-refused', 'isolation-refused', 'collector-refused'), 'semantic-state')
    require(result['semantic_complete'] == (result['reason'] == 'complete'), 'semantic-state')
    require(type(result['commands']) is list and len(result['commands']) <= 5, 'command-count')
    total = 0
    for index, row in enumerate(result['commands']):
        closed(row, 'label argv exit error merged_base64 merged_bytes merged_sha256')
        require(row['label'] == LABELS[index] and type(row['argv']) is list and all(type(x) is str for x in row['argv'])
                and (row['exit'] is None or type(row['exit']) is int)
                and row['error'] in (None, 'deadline', 'stream-bound', 'process-refused', 'protocol-refused'), 'command-record')
        raw = base64.b64decode(row['merged_base64'], validate=True)
        require(type(row['merged_bytes']) is int and len(raw) == row['merged_bytes'] and digest(raw) == row['merged_sha256'], 'stream-identity')
        total += len(raw)
    require(total <= LIMIT and len(json.dumps(result['rpc']).encode()) <= 128 << 10, 'aggregate-bound')
    closed(result['rpc'], 'initialize plugins notifications')
    return result


def fixtures(root, nonce):
    from client_automation_native_probe import put
    roots = {'user_profile': root / 'home/.grok', 'a': root / 'a', 'b': root / 'b'}
    for part in ('home', 'user', 'a', 'b', 'tmp', 'xdg-config', 'xdg-data', 'xdg-cache', 'xdg-state'):
        (root / part).mkdir(mode=0o700)
    for scope in SCOPES:
        subprocess.run(['/usr/bin/git', '-c', 'core.hooksPath=/dev/null', 'init', '-q', str(root / scope)], check=True,
                       env={'PATH': '/usr/bin:/bin', 'GIT_CONFIG_NOSYSTEM': '1', 'GIT_CONFIG_GLOBAL': '/dev/null'}, timeout=5)
        base = root / 'home/.grok/plugins' if scope == 'user' else root / scope / '.grok/plugins'
        for name in ('uap-acp-collision', 'uap-acp-foreign-' + scope):
            plugin = base / name
            put(plugin / '.grok-plugin/plugin.json', {'name': name, 'version': '1.0.0', 'skills': './skills/'})
            put(plugin / 'skills/inert/SKILL.md', '---\nname: inert\ndescription: Inert inventory fixture.\n---\n' + nonce + scope)
    return roots


def post_ledger(root, before, roots, profiles, budget):
    from devin_structured_inventory_admission import metadata_inventory
    from client_automation_native_probe import snapshot, grok_profile_bootstrap
    after = metadata_inventory(root, budget)
    def allowed(path):
        return path == 'request.json' or path in ('home/.grok/docs', 'home/.grok/docs/user-guide', 'home/.grok/logs') or bool(re.fullmatch(r'home/\.grok/docs/user-guide/[0-9]{2}-[a-z0-9-]+\.md', path)) or path == 'home/.grok/logs/unified.jsonl'
    accepted = all(after.get(path) == value for path, value in before.items() if not allowed(path)) and all(path in before or allowed(path) for path in after)
    current = {name: snapshot(path) for name, path in roots.items()}
    try:
        added = grok_profile_bootstrap(profiles, current)
    except ValueError:
        added, accepted = [], False
    return dict(accepted=accepted, before=before, after=after, profiles_before=profiles, profiles_after=current, native_bootstrap=added)


def inventory_rows(result, root, scope):
    rows = result['rpc']['plugins']['result']['plugins']
    wanted = {str(root / 'home/.grok/plugins' / name): ('user', name) for name in ('uap-acp-collision', 'uap-acp-foreign-user')}
    if scope != 'user':
        wanted.update({str(root / scope / '.grok/plugins' / name): ('project', name) for name in ('uap-acp-collision', 'uap-acp-foreign-' + scope)})
    seen, matched = set(), set()
    for row in rows:
        require(type(row) is dict and type(row.get('id')) is str and row['id'] and row['id'] not in seen, 'plugin-id')
        seen.add(row['id'])
        require(type(row.get('trusted')) is bool and type(row.get('enabled')) is bool, 'plugin-state')
        if row.get('root') in wanted:
            expected_scope, name = wanted[row['root']]
            require(row.get('scope') == expected_scope and row.get('name') == name and row['root'] not in matched, 'plugin-source')
            require(re.fullmatch(re.escape(expected_scope) + r'/[0-9a-f]{8}/' + re.escape(name), row['id']), 'full-plugin-id')
            matched.add(row['root'])
        elif str(root) in str(row.get('root', '')):
            raise ValueError('sibling-plugin')
    require(matched == set(wanted), 'exact-inventory-incomplete')
    return {row['root']: row['id'] for row in rows if row.get('root') in wanted}


def directory_id(info):
    return dict(device=info.st_dev, inode=info.st_ino, mode=info.st_mode, uid=info.st_uid, gid=info.st_gid)


def archive_id(info):
    return dict(directory_id(info), links=info.st_nlink, size=info.st_size, mtime_ns=info.st_mtime_ns, ctime_ns=info.st_ctime_ns)


def archive_unchanged(parent, descriptor, before):
    require(stable(before) == stable(os.fstat(descriptor)) == stable(os.stat('research.zip', dir_fd=parent, follow_symlinks=False)), 'archive-replaced')


def capture_archive(parent):
    """Fixed ZIP leaf, held continuously through publication or admitted copy."""
    import stat
    before = os.stat('research.zip', dir_fd=parent, follow_symlinks=False)
    require(stat.S_ISREG(before.st_mode) and before.st_nlink == 1 and 0 <= before.st_size <= 17 << 20, 'archive-bound')
    descriptor = os.open('research.zip', os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC, dir_fd=parent)
    try:
        archive_unchanged(parent, descriptor, before)
        with os.fdopen(descriptor, 'rb', closefd=False) as stream: raw = stream.read((17 << 20) + 1)
        require(len(raw) == before.st_size, 'archive-length')
        archive_unchanged(parent, descriptor, before)
        return descriptor, before, raw
    except BaseException:
        os.close(descriptor)
        raise


def rename_pending(descriptor):
    """Linux atomic no-replace publication beneath the held output descriptor."""
    import ctypes
    libc = ctypes.CDLL(None, use_errno=True)
    rename = libc.renameat2
    rename.argtypes = (ctypes.c_int, ctypes.c_char_p, ctypes.c_int, ctypes.c_char_p, ctypes.c_uint)
    rename.restype = ctypes.c_int
    if rename(descriptor, b'pending', descriptor, b'upload', 1) != 0:
        raise OSError(ctypes.get_errno(), 'atomic publication refused')


def publish(output, bodies, held=None, expected=None):
    """Single descriptor-anchored attempt; only the returned receipt permits upload."""
    import native_client_project_artifacts as artifacts
    allowed = {'admission.json', 'acquisition.json'} | {scope + '.rpc.json' for scope in SCOPES} | {scope + '.ledger.json' for scope in SCOPES} | {scope + '-' + label + '.merged.log' for scope in SCOPES for label in LABELS}
    require(set(bodies) <= allowed and all(len(body) <= LIMIT for body in bodies.values())
            and sum(map(len, bodies.values())) <= 16 << 20, 'artifact-bound')
    owned = held is None
    descriptor = os.open(output, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC) if owned else held
    pending, archive = None, None
    try:
        root_id = directory_id(os.fstat(descriptor))
        require((owned or expected == root_id), 'original-output-authority')
        require(root_id == directory_id(output.lstat()) and output.parent.resolve() == output.parent, 'output-authority')
        os.mkdir('pending', 0o700, dir_fd=descriptor)
        pending = os.open('pending', os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=descriptor)
        pending_id = directory_id(os.fstat(pending))
        artifacts.write_zip(Path('/proc/self/fd/' + str(pending)) / 'research.zip', bodies)
        require(pending_id == directory_id(os.stat('pending', dir_fd=descriptor, follow_symlinks=False)), 'pending-replaced')
        archive, before, raw = capture_archive(pending)
        require(root_id == directory_id(output.lstat()), 'output-replaced')
        rename_pending(descriptor)
        require(root_id == directory_id(output.lstat())
                and pending_id == directory_id(os.stat('upload', dir_fd=descriptor, follow_symlinks=False)), 'publication-replaced')
        archive_unchanged(pending, archive, before)
        return dict(output=root_id, upload=pending_id, archive=archive_id(before), bytes=len(raw), sha256=digest(raw))
    finally:
        if archive is not None: os.close(archive)
        if pending is not None: os.close(pending)
        if owned: os.close(descriptor)


def admit_publication(output, packet, expected_commit, expected_tree, destination):
    """Workflow parent validates held bytes; no scanning a stale output prefix."""
    closed(packet, 'schema_version source_commit source_tree nonce publication')
    require(type(packet['schema_version']) is int and packet['schema_version'] == 1
            and packet['source_commit'] == expected_commit and packet['source_tree'] == expected_tree
            and re.fullmatch('[0-9a-f]{40}', expected_tree)
            and re.fullmatch('[0-9a-f]{32}', packet['nonce']), 'publication-source')
    proof = closed(packet['publication'], 'output upload archive bytes sha256')
    descriptors = []
    try:
        require(output.parent.resolve() == output.parent, 'output-parent')
        root_fd = os.open(output, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC)
        descriptors.append(root_fd)
        upload_fd = os.open('upload', os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=root_fd)
        descriptors.append(upload_fd)
        require(set(os.listdir(root_fd)) == {'upload'} and set(os.listdir(upload_fd)) == {'research.zip'}, 'closed-publication')
        require(directory_id(os.fstat(root_fd)) == directory_id(output.lstat()) == proof['output']
                and directory_id(os.fstat(upload_fd)) == directory_id(os.stat('upload', dir_fd=root_fd, follow_symlinks=False)) == proof['upload'], 'publication-directory')
        archive, before, raw = capture_archive(upload_fd)
        descriptors.append(archive)
        require(archive_id(before) == proof['archive'] and type(proof['bytes']) is int
                and len(raw) == proof['bytes'] and digest(raw) == proof['sha256'], 'publication-archive')
        require(not destination.exists() and destination.parent.resolve() == destination.parent, 'admitted-destination')
        destination.mkdir(mode=0o700)
        with (destination / 'research.zip').open('xb') as stream: stream.write(raw)
        require(digest(ordinary(destination / 'research.zip', 17 << 20)) == proof['sha256'], 'admitted-copy-identity')
        archive_unchanged(upload_fd, archive, before)
        require(proof['output'] == directory_id(os.fstat(root_fd)) == directory_id(output.lstat())
                and proof['upload'] == directory_id(os.fstat(upload_fd))
                == directory_id(os.stat('upload', dir_fd=root_fd, follow_symlinks=False))
                and set(os.listdir(root_fd)) == {'upload'} and set(os.listdir(upload_fd)) == {'research.zip'}, 'publication-final-authority')
        return destination / 'research.zip'
    finally:
        for descriptor in reversed(descriptors): os.close(descriptor)


def run(output):
    import project_native_research as shared
    from project_native_collectors.grok import ACQUISITION
    from devin_structured_inventory_admission import metadata_inventory
    from client_automation_native_probe import snapshot
    shared.native.require_hosted()
    output = output.absolute()
    require(output.resolve() == output and not output.exists() and os.getuid() != 0, 'output-authority')
    output.mkdir(mode=0o700, parents=True)
    output_fd = os.open(output, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC)
    output_identity = directory_id(os.fstat(output_fd))
    origin, nonce = time.monotonic(), os.urandom(16).hex()
    root = Path('/tmp/uap-grok-acp-input-' + nonce)
    root.mkdir(mode=0o700)
    held = stable(root.lstat())[:2]
    summary = dict(schema_version=1, collection_complete=False, cleanup_complete=False, qualification=dict(FALSE), scopes=[])
    bodies, cleanup_proved = {}, True
    try:
        with shared.Budget(150) as budget:
            def source(args):
                value = subprocess.run(['/usr/bin/git', *args], cwd=shared.SOURCE, capture_output=True, timeout=5, check=True,
                                       env={'PATH': '/usr/bin:/bin', 'GIT_CONFIG_NOSYSTEM': '1', 'GIT_CONFIG_GLOBAL': '/dev/null'})
                require(not value.stderr and len(value.stdout) <= 4096, 'source-bound')
                return value.stdout.decode().strip()
            commit, tree = source(['rev-parse', 'HEAD']), source(['rev-parse', 'HEAD^{tree}'])
            require(commit == os.environ.get('EXPECTED_COMMIT') and re.fullmatch('[0-9a-f]{40}', commit)
                    and re.fullmatch('[0-9a-f]{40}', tree) and not source(['status', '--porcelain', '--untracked-files=normal']), 'source-identity')
            summary.update(source_commit=commit, source_tree=tree, nonce=nonce, run=os.environ.get('GITHUB_RUN_ID'), attempt=os.environ.get('GITHUB_RUN_ATTEMPT'))
            receipt, contents = shared.acquire(ACQUISITION, budget)
            require(digest(contents['grok']) == ACQUISITION['binary_sha256'], 'native-pin')
            (root / 'grok').write_bytes(contents['grok'])
            (root / 'grok').chmod(0o700)
            collector = Path(__file__).resolve()
            (root / 'collector.py').write_bytes(ordinary(collector))
            helper = root / 'helper'
            subprocess.run(['go', 'build', '-trimpath', '-ldflags', '-X main.sourceCommit=' + commit + ' -X main.sourceTree=' + tree
                            + ' -X main.collectorSHA=' + digest(ordinary(collector)), '-o', str(helper), './research/grokacpinventory'],
                           cwd=shared.SOURCE / 'install/integrationctl', check=True, timeout=60)
            roots = fixtures(root, nonce)
            (root / 'request.json').write_bytes(b'{}\n')
            baseline = metadata_inventory(root, budget)
            profiles = {name: snapshot(path) for name, path in roots.items()}
            bodies['acquisition.json'] = json.dumps(receipt, sort_keys=True).encode()
            native_ids = {}
            for scope in SCOPES:
                require(time.monotonic() < origin + 150, 'outer-deadline')
                request = dict(schema_version=1, source_commit=commit, source_tree=tree, uid=os.getuid(), gid=os.getgid(), nonce=nonce,
                               scope=scope, start_monotonic_ns=time.monotonic_ns(), helper_sha256=digest(ordinary(helper, 64 << 20)))
                (root / 'request.json').write_text(json.dumps(request))
                cleanup_proved = False
                receipt = helper_capture(helper, root, min(origin + 150, request['start_monotonic_ns'] / 1e9 + 35))
                result = validate_receipt(receipt, request)
                cleanup_proved = True
                ledger = post_ledger(root, baseline, roots, profiles, budget)
                bodies[scope + '.ledger.json'] = json.dumps(ledger, sort_keys=True).encode()
                for row in result['commands']:
                    name = scope + '-' + row['label'] + '.merged.log'
                    bodies[name] = base64.b64decode(row.pop('merged_base64'), validate=True)
                    row['merged_file'] = name
                bodies[scope + '.rpc.json'] = json.dumps(result.pop('rpc'), sort_keys=True).encode()
                summary['scopes'].append(receipt)
                result['rpc'] = document(bodies[scope + '.rpc.json'])
                require(ledger['accepted'] and result['semantic_complete'], 'native-refused')
                observed_ids = inventory_rows(result, root, scope)
                require(all(path not in native_ids or native_ids[path] == value for path, value in observed_ids.items()), 'native-id-changed')
                native_ids.update(observed_ids)
                result.pop('rpc')
            require(not source(['status', '--porcelain', '--untracked-files=normal']), 'source-changed')
            summary['collection_complete'] = True
    except Exception:
        summary['error'] = 'admission-refused'
    finally:
        if cleanup_proved and stable(root.lstat())[:2] == held:
            shutil.rmtree(root)
            summary['cleanup_complete'] = True
        if not summary['cleanup_complete']:
            summary = dict(schema_version=1, collection_complete=False, cleanup_complete=False, qualification=dict(FALSE), scopes=[])
            bodies = {}
        bodies['admission.json'] = json.dumps(summary, sort_keys=True).encode() + b'\n'
        try:
            publication = publish(output, bodies, output_fd, output_identity)
        finally:
            os.close(output_fd)
        if summary['cleanup_complete'] and all(key in summary for key in ('source_commit', 'source_tree', 'nonce')):
            print(json.dumps(dict(schema_version=1, source_commit=summary['source_commit'], source_tree=summary['source_tree'],
                                  nonce=summary['nonce'], publication=publication), separators=(',', ':')))
    return 0 if summary['collection_complete'] else 1


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path)
    parser.add_argument('--collect', type=Path)
    args = parser.parse_args()
    if args.collect is not None and args.output is None:
        collect(args.collect)
    else:
        require(args.output is not None and args.collect is None, 'mode')
        raise SystemExit(run(args.output))
