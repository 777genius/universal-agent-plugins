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
import stat
import subprocess
import sys
import time

SESSION_MODE = 'session-only-v1'
SETUP_PHASES = frozenset(('auth', 'resolve_workspace', 'folder_trust', 'plugin_registry', 'mcp_merge',
    'local_workspace', 'persistence_init', 'spawn_session_actor', 'workspace_handshake', 'model_switch',
    'model_not_allowed', 'git_discovery', 'finalize_response', 'tool_overrides', 'response_ready'))


def session_port(nonce):
    require(type(nonce) is str and re.fullmatch('[0-9a-f]{32}', nonce), 'session-nonce')
    return 20000 + int(nonce[:4], 16) % 40000


def loopback_up():
    import fcntl
    import struct
    with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as channel:
        raw = fcntl.ioctl(channel.fileno(), 0x8913, struct.pack('16sH22x', b'lo', 0))
    return len(raw) == 40 and raw[:16] == b'lo' + b'\0' * 14 and struct.unpack_from('H', raw, 16)[0] & 9 == 9


class SessionProvider:
    """Fixed metadata GETs only; no request values or bodies enter facts."""
    def __init__(self, port, deadline):
        import threading
        self.deadline, self.stop = deadline, threading.Event()
        self.facts = dict(requests=0, models_get=0, root_get=0, forbidden=0, settled=False)
        self.listener = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self.listener.bind(('127.0.0.1', port))
        self.listener.listen(4)
        self.listener.settimeout(.05)
        self.port = self.listener.getsockname()[1]
        self.thread = threading.Thread(target=self.serve, daemon=True)
        self.thread.start()

    def serve(self):
        while not self.stop.is_set() and time.monotonic() < self.deadline:
            try:
                connection, address = self.listener.accept()
            except socket.timeout:
                continue
            except OSError:
                if not self.stop.is_set():
                    self.facts['forbidden'] += 1
                return
            with connection:
                self.facts['requests'] += 1
                if self.facts['requests'] > 12:
                    self.facts['forbidden'] += 1
                    return
                try:
                    connection_deadline = min(time.monotonic() + 5, self.deadline)
                    require(address[0] == '127.0.0.1', 'provider-address')
                    body = bytearray()
                    while b'\r\n\r\n' not in body:
                        remaining = connection_deadline - time.monotonic()
                        require(remaining > 0, 'provider-deadline')
                        connection.settimeout(remaining)
                        piece = connection.recv(min(1024, 8193 - len(body)))
                        require(piece and len(body) + len(piece) <= 8192, 'provider-header-bound')
                        body.extend(piece)
                    header, tail = bytes(body).split(b'\r\n\r\n', 1)
                    lines = header.split(b'\r\n')
                    require(not tail and len(lines) <= 32, 'provider-body')
                    method, route, version = lines[0].split(b' ')
                    require(method == b'GET' and version == b'HTTP/1.1' and route in (b'/', b'/v1/models'), 'provider-route')
                    fields = {}
                    for line in lines[1:]:
                        key, value = line.split(b':', 1)
                        key = key.lower()
                        require(key not in fields, 'provider-duplicate')
                        fields[key] = value.strip()
                    require(fields.get(b'host') == ('127.0.0.1:' + str(self.port)).encode()
                            and b'transfer-encoding' not in fields
                            and fields.get(b'content-length', b'0') == b'0', 'provider-body-host')
                    if route == b'/v1/models':
                        field = 'models_get'
                        response = b'{"object":"list","data":[{"id":"uap-local-driver","object":"model","owned_by":"uap"}]}'
                        status = b'200 OK'
                    else:
                        field = 'root_get'
                        response, status = b'{}', b'404 Not Found'
                    remaining = connection_deadline - time.monotonic()
                    require(remaining > 0, 'provider-deadline')
                    connection.settimeout(remaining)
                    connection.sendall(b'HTTP/1.1 ' + status + b'\r\nContent-Type: application/json\r\nConnection: close\r\nContent-Length: '
                                       + str(len(response)).encode() + b'\r\n\r\n' + response)
                    self.facts[field] += 1
                except (ValueError, OSError):
                    self.facts['forbidden'] += 1

    def close(self):
        self.stop.set()
        self.listener.close()
        self.thread.join(timeout=max(0, self.deadline - time.monotonic()))
        require(not self.thread.is_alive(), 'provider-cleanup-unproved')
        self.facts['settled'] = True


def session_rpc_frame(value, state, req, send):
    require(type(value) is dict and value.get('jsonrpc') == '2.0', 'rpc-envelope')
    if 'method' in value:
        closed(value, 'jsonrpc method params')
        require(len(state['notifications']) < 32, 'notification-bound')
        if value['method'] == '_x.ai/mcp/servers_updated':
            require(value['params'] == {'mcpServers': []}
                    and not any(x['method'] == value['method'] for x in state['notifications']), 'notification-refused')
        else:
            require(value['method'] == '_x.ai/session/setup', 'notification-refused')
            params = closed(value['params'], 'method phase sessionId')
            require(params['method'] == 'session/new' and type(params['phase']) is str and params['phase'] in SETUP_PHASES
                    and sum(x['params'].get('phase') == params['phase'] for x in state['notifications']) < 2,
                    'setup-notification')
            require(params['sessionId'] is None or type(params['sessionId']) is str
                    and 0 < len(params['sessionId']) <= 256 and params['sessionId'].isascii()
                    and all(32 <= ord(x) <= 126 for x in params['sessionId']), 'session-id')
        if state['session'] is not None and value['method'] == '_x.ai/session/setup':
            require(value['params']['sessionId'] in (None, state['session']['result']['sessionId']), 'session-notification-identity')
        state['notifications'].append(value)
        return False
    if state['initialize'] is None:
        rpc_frame(value, state, req, lambda _frame: None)
        send({'jsonrpc':'2.0', 'id':2, 'method':'session/new', 'params':{
            'cwd':str(Path(req['input_root']) / 'a'), 'mcpServers':[], '_meta':{'modelId':'uap-local-driver'}}})
        return False
    closed(value, 'jsonrpc id result')
    require(type(value['id']) is int and type(value['result']) is dict, 'session-response')
    if state['session'] is None:
        result = value['result']
        require(value['id'] == 2 and 'sessionId' in result
                and set(result) <= {'sessionId','models','configOptions','_meta'}, 'session-response')
        sid = result['sessionId']
        require(type(sid) is str and 0 < len(sid) <= 256 and sid.isascii()
                and all(32 <= ord(x) <= 126 for x in sid), 'session-id')
        for item in state['notifications']:
            if item['method'] == '_x.ai/session/setup':
                require(item['params']['sessionId'] in (None, sid), 'session-notification-identity')
        if '_meta' in result:
            require(type(result['_meta']) is dict, 'session-meta')
            if 'currentWorkingDirectory' in result['_meta']:
                require(result['_meta']['currentWorkingDirectory'] == str(Path(req['input_root']) / 'a'), 'session-cwd')
        if 'models' in result: require(type(result['models']) is dict, 'session-models')
        if 'configOptions' in result: require(type(result['configOptions']) is list, 'session-options')
        state['session'] = value
        send({'jsonrpc':'2.0','id':3,'method':'_x.ai/plugins/list','params':{'sessionId':sid}})
        return False
    require(value['id'] == 3 and state['plugins'] is None, 'listing-id')
    closed(value['result'], 'result')
    listing = closed(value['result']['result'], 'plugins')
    require(type(listing['plugins']) is list and len(listing['plugins']) <= 128, 'plugins-bound')
    state['plugins'] = value
    return True


LIMIT = 1 << 20
SCOPES = ('user', 'a', 'b')
LABELS = ('version', 'help', 'agent-help', 'stdio-help', 'acp')
FALSE = dict.fromkeys(('inventory_admitted', 'native_enablement', 'workspace_trust', 'native_mcp_call',
                       'provider_authentication', 'real_model_inference', 'production_project_activation'), False)
BOOTSTRAP_LIMIT = 256 << 10
BOOTSTRAP_FILES = tuple('home/.grok/' + name for name in (
    '.config-init.lock', '.metadata_version', 'README.md', 'agent_id', 'config.toml',
    'installed-plugins/registry.lock', 'managed_config.lock', 'sessions/session_search.sqlite',
    'sessions/session_search.sqlite-shm', 'sessions/session_search.sqlite-wal'))


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


def ordinary(path, limit=LIMIT, *, expected=None):
    import stat
    info = path.lstat()
    require(expected is None or stable(info) == tuple(expected), 'original-identity')
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
    if req.get('mode') == SESSION_MODE:
        value['loopback_up'] = loopback_up()
        require(value['loopback_up'], 'loopback-down')
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
    closed(value['result'], 'result')
    listing = closed(value['result']['result'], 'plugins')
    require(type(listing['plugins']) is list and len(listing['plugins']) <= 128, 'plugins-bound')
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
                            completed = (session_rpc_frame if req.get('mode') == SESSION_MODE else rpc_frame)(document(bytes(line), 64 << 10), state, req, send)
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
    session, provider, provider_in_flight = req.get('mode') == SESSION_MODE, None, False
    if session:
        result.update(mode=SESSION_MODE, provider={})
        result['rpc']['session'] = None
    try:
        result['isolation'] = isolation(req)
        require(req['scope'] in SCOPES and re.fullmatch('[0-9a-f]{32}', req['nonce']), 'request-identity')
        env = {'PATH': '/usr/bin:/bin', 'LANG': 'C.UTF-8', 'LC_ALL': 'C.UTF-8', 'HOME': str(root / 'home'),
               'TMPDIR': str(root / 'tmp'), 'TERM': 'dumb', 'NO_COLOR': '1', 'GROK_DISABLE_AUTOUPDATER': '1',
               'GIT_CONFIG_NOSYSTEM': '1', 'GIT_CONFIG_GLOBAL': '/dev/null', 'GIT_TERMINAL_PROMPT': '0'}
        env.update({'XDG_' + name.upper() + '_HOME': str(root / ('xdg-' + name)) for name in ('config', 'data', 'cache', 'state')})
        if session:
            require(req['scope'] == 'a', 'session-scope')
            provider_in_flight = True
            provider = SessionProvider(session_port(req['nonce']), deadline)
            provider_in_flight = False
            env['XAI_API_KEY'] = 'uap-inert-local-driver'
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
        if command_in_flight or provider_in_flight:
            raise  # No JSON/result authority for any incomplete command capture.
    finally:
        if session:
            if provider is not None:
                provider.close()
            result['provider'] = dict(provider.facts) if provider is not None else dict(requests=0, models_get=0, root_get=0, forbidden=0, settled=True)
    if session and result['provider']['forbidden']:
        result.update(semantic_complete=False, reason='protocol-refused')
    return result


def collect(request):
    req = document(ordinary(request, 16384), 16384)
    session = req.get('mode') == SESSION_MODE
    lock = req.get('mode') == LOCK_MODE
    require('mode' not in req or session or lock, 'mode')
    closed(req, 'schema_version source_commit source_tree uid gid nonce scope input_root parent_network_namespace start_monotonic_ns' + (' mode' if session else ' mode phase' if lock else ''))
    body = json.dumps(lock_collector_result(req) if lock else collector_result(req), separators=(',', ':')).encode() + b'\n'
    require(len(body) <= 2 << 20, 'result-bound')
    sys.stdout.buffer.write(body)
    sys.stdout.buffer.flush()
    sys.stdout.close()
    sys.stderr.close()
    # CPython's standard streams have closefd=False; the strict parent needs
    # real pipe EOF while this collector remains alive for planned shutdown.
    os.close(1)
    os.close(2)
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
    session = request.get('mode') == SESSION_MODE
    lock = request.get('mode') == LOCK_MODE
    result = closed(receipt['result'], 'schema_version source_commit source_tree scope nonce semantic_complete reason isolation commands rpc' + (' mode provider' if session else ' mode phase lock_held' if lock else ''))
    if session:
        require(result['mode'] == SESSION_MODE, 'session-mode')
        provider = closed(result['provider'], 'requests models_get root_get forbidden settled')
        require(provider['settled'] is True and all(type(provider[key]) is int and 0 <= provider[key] <= 13 for key in ('requests','models_get','root_get','forbidden'))
                and provider['requests'] == provider['models_get'] + provider['root_get'] + provider['forbidden'], 'provider-facts')
    if lock:
        require(result['mode'] == LOCK_MODE and result['phase'] == request['phase'] in LOCK_PHASES
                and type(result['lock_held']) is bool and (not result['semantic_complete']
                or result['lock_held'] == (request['phase'] == LOCK_PHASES[0]) and len(result['commands']) == 4), 'lock-result')
    for key in ('schema_version', 'source_commit', 'source_tree', 'scope', 'nonce'):
        require(type(result[key]) is type(request[key]) and result[key] == request[key], 'collector-identity')
    require(type(result['semantic_complete']) is bool and result['reason'] in ('complete', 'protocol-refused', 'help-refused', 'isolation-refused', 'collector-refused'), 'semantic-state')
    require(result['semantic_complete'] == (result['reason'] == 'complete'), 'semantic-state')
    require(type(result['commands']) is list and len(result['commands']) <= (4 if lock else 5), 'command-count')
    total = 0
    for index, row in enumerate(result['commands']):
        closed(row, 'label argv exit error merged_base64 merged_bytes merged_sha256')
        require(row['label'] == (LOCK_LABELS if lock else LABELS)[index] and type(row['argv']) is list and all(type(x) is str for x in row['argv'])
                and (row['exit'] is None or type(row['exit']) is int)
                and row['error'] in (None, 'deadline', 'stream-bound', 'process-refused', 'protocol-refused'), 'command-record')
        raw = base64.b64decode(row['merged_base64'], validate=True)
        require(type(row['merged_bytes']) is int and len(raw) == row['merged_bytes'] and digest(raw) == row['merged_sha256'], 'stream-identity')
        total += len(raw)
    require(total <= LIMIT and len(json.dumps(result['rpc']).encode()) <= 128 << 10, 'aggregate-bound')
    closed(result['rpc'], 'initialize plugins notifications' + (' session' if session else ''))
    return result



def session_fixture_config(root, nonce):
    url = 'http://127.0.0.1:' + str(session_port(nonce)) + '/v1'
    text = ('[features]\ntelemetry = false\ntitle_refresh = false\n'
            '[models]\ndefault = "uap-local-driver"\nsession_summary = "uap-local-driver"\nprompt_suggestion = "uap-local-driver"\n'
            '[endpoints]\nmodels_base_url = "' + url + '"\n'
            '[model.uap-local-driver]\nmodel = "uap-local-driver"\nbase_url = "' + url + '"\n'
            'api_backend = "chat_completions"\napi_key = "uap-inert-local-driver"\ncontext_window = 64000\n')
    path = root / 'home/.grok/config.toml'
    require(not path.exists(), 'session-config-collision')
    path.write_text(text)
    path.chmod(0o600)


def session_metadata_ledger(root, before, budget):
    from devin_structured_inventory_admission import metadata_inventory
    after = metadata_inventory(root, budget)
    protected = all(after.get(name) == identity for name, identity in before.items())
    # First capture is never a new bootstrap/effect admission. No new body read.
    return dict(before=before, after=after, accepted=False, protected_originals_unchanged=protected)


def session_readout(members, packet, deadline):
    refused = dict(schema_version=1, mode=SESSION_MODE, reason='readout-refused')
    try:
        require(type(deadline) is float and time.monotonic() < deadline, 'readout-deadline')
        allowed = {'admission.json','acquisition.json','a.ledger.json','a.rpc.json'} | {'a-' + label + '.merged.log' for label in LABELS}
        require(set(members) <= allowed and len(members) <= 9, 'session-members')
        summary = document(members['admission.json'])
        require(type(summary) is dict and set(summary) <= set('schema_version collection_complete cleanup_complete qualification scopes source_commit source_tree nonce run attempt error mode measurement_complete'.split())
                and type(summary.get('schema_version')) is int and summary['schema_version'] == 1
                and ('measurement_complete' not in summary or type(summary['measurement_complete']) is bool), 'session-summary')
        require(summary.get('mode') == SESSION_MODE and summary.get('collection_complete') is False
                and summary.get('cleanup_complete') is True and summary['qualification'] == FALSE
                and all(value is False for value in summary['qualification'].values()), 'session-qualification')
        require(all(summary[key] == packet[key] for key in ('source_commit','source_tree','nonce')), 'session-source')
        rows = summary['scopes']
        require(type(rows) is list and len(rows) == 1, 'session-scopes')
        facts = dict(schema_version=1, mode=SESSION_MODE, reason='measurement-only',
                     collection_complete=False, cleanup_complete=True, session_reply=False, inventory_reply=False,
                     provider_requests=0, provider_forbidden=0, added_entries=0, removed_entries=0, changed_entries=0)
        if rows:
            receipt = rows[0]
            require(receipt['scope'] == 'a' and receipt['cleanup_complete'] is True and receipt['sealed_cleanup_complete'] is True
                    and all(receipt[key] == packet[key] for key in ('source_commit','source_tree','nonce')), 'session-scope')
            result = receipt['result']
            require(result['mode'] == SESSION_MODE and all(result[key] == packet[key] for key in ('source_commit','source_tree','nonce')), 'session-source')
            provider = closed(result['provider'], 'requests models_get root_get forbidden settled')
            require(provider['settled'] is True and all(type(provider[k]) is int and 0 <= provider[k] <= 13 for k in ('requests','models_get','root_get','forbidden'))
                    and provider['requests'] == provider['models_get'] + provider['root_get'] + provider['forbidden'], 'session-provider')
            for obj in (receipt, result):
                require(type(obj.get('schema_version')) is int and obj['schema_version'] == 1
                        and obj.get('scope') == 'a', 'session-scope')
            require(type(result.get('semantic_complete')) is bool
                    and result.get('reason') in ('complete','protocol-refused','help-refused','isolation-refused','collector-refused')
                    and result['semantic_complete'] == (result['reason'] == 'complete'), 'session-reason')
            commands = result['commands']
            require(type(commands) is list and len(commands) <= 5, 'session-commands')
            for index, row in enumerate(commands):
                require(type(row) is dict and row.get('label') == LABELS[index]
                        and row.get('merged_file') == 'a-' + LABELS[index] + '.merged.log'
                        and type(row.get('merged_bytes')) is int and 0 <= row['merged_bytes'] <= LIMIT
                        and type(row.get('merged_sha256')) is str and re.fullmatch('[0-9a-f]{64}', row['merged_sha256'])
                        and (row.get('exit') is None or type(row['exit']) is int)
                        and row.get('error') in (None,'deadline','stream-bound','process-refused','protocol-refused'), 'session-command')
                body = members[row['merged_file']]
                require(len(body) == row['merged_bytes'] and digest(body) == row['merged_sha256'], 'session-stream')
            rpc = closed(document(members['a.rpc.json']), 'initialize plugins notifications session')
            require(type(rpc['notifications']) is list and len(rpc['notifications']) <= 32, 'session-notifications')
            replay = dict(initialize=None, plugins=None, notifications=[], session=None)
            request = dict(nonce=packet['nonce'], scope='a', input_root='/tmp/uap-grok-acp-input-' + packet['nonce'], mode=SESSION_MODE)
            if any(rpc[key] is not None for key in ('initialize','session','plugins')):
                require(len(commands) == 5, 'session-missing-stream')
            if len(commands) == 5:
                wire = members['a-acp.merged.log']
                require(not wire or wire.endswith(b'\n'), 'session-frame-termination')
                lines = wire.splitlines()
                completed = False
                require(len(lines) <= 64, 'session-frame-bound')
                for line in lines:
                    require(not completed and len(line) <= 64 << 10, 'session-frame-bound')
                    completed = session_rpc_frame(document(line, 64 << 10), replay, request, lambda _frame: None)
                require(replay == rpc, 'session-wire-join')
            else:
                require(rpc == replay, 'session-rpc-before-command')
            require(not result['semantic_complete'] or replay['plugins'] is not None
                    and all(row['error'] is None for row in commands), 'session-incomplete')
            ledger = document(members['a.ledger.json'])
            require(type(ledger['before']) is dict and type(ledger['after']) is dict
                    and len(ledger['before']) <= 4096 and len(ledger['after']) <= 4096 and ledger['accepted'] is False, 'session-ledger')
            before, after = ledger['before'], ledger['after']
            facts.update(session_reply=rpc['session'] is not None, inventory_reply=rpc['plugins'] is not None,
                         provider_requests=provider['requests'], provider_forbidden=provider['forbidden'],
                         added_entries=len(after.keys()-before.keys()), removed_entries=len(before.keys()-after.keys()),
                         changed_entries=sum(before[k] != after[k] for k in before.keys() & after.keys()))
        require(time.monotonic() < deadline, 'readout-deadline')
        return facts
    except Exception:
        return refused



LOCK_MODE, LOCK_PHASES = 'lock-qualification-v1', ('locked-writer', 'released-writer')
LOCK_LABELS, LOCK_NAME = ('version', 'mcp-help', 'disable-help', 'writer'), 'uap-lock-witness'
def lock_file(root, expected_held):
    import errno
    import fcntl
    import stat
    held = []
    try:
        parent = os.open(root, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC)
        held.append(parent)
        for part in ('home', '.grok'):
            child = os.open(part, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=parent)
            held.append(child)
            require(os.fstat(child).st_uid == os.getuid(), 'lock-directory-owner')
            parent = child
        before = os.stat('.config-init.lock', dir_fd=parent, follow_symlinks=False)
        require(stat.S_ISREG(before.st_mode) and before.st_nlink == 1 and before.st_size == 0
                and before.st_uid == os.getuid() and stat.S_IMODE(before.st_mode) == 0o600, 'lock-file-authority')
        fd = os.open('.config-init.lock', os.O_RDWR | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC, dir_fd=parent)
        held.append(fd)
        require(stable(before) == stable(os.fstat(fd)), 'lock-open-identity')
        try:
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as error:
            require(error.errno in (errno.EAGAIN, errno.EWOULDBLOCK) and expected_held, 'supervisor-lock-proof')
            observed_held = True
        else:
            fcntl.flock(fd, fcntl.LOCK_UN)
            require(not expected_held, 'supervisor-lock-missing')
            observed_held = False
        require(stable(before) == stable(os.fstat(fd))
                == stable(os.stat('.config-init.lock', dir_fd=parent, follow_symlinks=False)), 'lock-linked-identity')
        return observed_held
    finally:
        for fd in reversed(held): os.close(fd)


def lock_help(rows):
    require(len(rows) == 3 and all(row['exit'] == 0 and row['error'] is None for row in rows), 'lock-help-exit')
    version = base64.b64decode(rows[0]['merged_base64'], validate=True)
    require(re.fullmatch(rb'grok 1\.0\.70 \([0-9a-f]{7,40}\)( \[stable\])?\n', version), 'lock-version')
    for row, usage in zip(rows[1:], ('grok mcp', 'grok mcp disable')):
        text = base64.b64decode(row['merged_base64'], validate=True).decode('utf-8')
        found = re.findall(r'^Usage: ([^\r\n]+)$', text, re.MULTILINE)
        operand = '<COMMAND>' if usage == 'grok mcp' else '<NAME>'
        require(len(found) == 1 and re.fullmatch(re.escape(usage) + r'[ \t]+(?:\[OPTIONS\][ \t]+)?' + re.escape(operand), found[0]), 'lock-help-usage')
    require(re.search(r'^[ \t]{2,}disable(?:[ \t]+[^\r\n]*)?$', base64.b64decode(rows[1]['merged_base64']).decode(), re.MULTILINE), 'lock-disable-advertised')


def lock_collector_result(req):
    root, native = Path(req['input_root']), Path(__file__).resolve().parent / 'grok'
    result = {key:req[key] for key in ('schema_version','source_commit','source_tree','scope','nonce')}
    result.update(mode=LOCK_MODE, phase=req['phase'], lock_held=False,
                  semantic_complete=False, reason='isolation-refused', isolation={}, commands=[], rpc=rpc_state())
    deadline, in_flight = req['start_monotonic_ns'] / 1e9 + 30, False
    try:
        result['isolation'] = isolation(req)
        require(req['scope'] == 'a' and req['phase'] in LOCK_PHASES, 'lock-phase')
        env = {'PATH':'/usr/bin:/bin','LANG':'C.UTF-8','LC_ALL':'C.UTF-8','HOME':str(root/'home'),
               'TMPDIR':str(root/'tmp'),'TERM':'dumb','NO_COLOR':'1','GROK_DISABLE_AUTOUPDATER':'1',
               'GIT_CONFIG_NOSYSTEM':'1','GIT_CONFIG_GLOBAL':'/dev/null','GIT_TERMINAL_PROMPT':'0'}
        env.update({'XDG_'+name.upper()+'_HOME':str(root/('xdg-'+name)) for name in ('config','data','cache','state')})
        result['reason'] = 'help-refused'
        for label,args in zip(LOCK_LABELS[:3], (['--version'],['mcp','--help'],['mcp','disable','--help'])):
            in_flight = True
            row = command([str(native),*args],label,root/'a',env,min(deadline,time.monotonic()+5),
                          LIMIT-sum(x['merged_bytes'] for x in result['commands']),result['rpc'],req)
            in_flight = False
            result['commands'].append(row)
        lock_help(result['commands'])
        result['lock_held'] = lock_file(root, req['phase'] == LOCK_PHASES[0])
        result['reason'] = 'protocol-refused'
        in_flight = True
        row = command([str(native),'mcp','disable',LOCK_NAME],'writer',root/'a',env,
                      min(deadline,time.monotonic()+5),LIMIT-sum(x['merged_bytes'] for x in result['commands']),result['rpc'],req)
        in_flight = False
        result['commands'].append(row)
        require(row['error'] is None and type(row['exit']) is int
                and (row['exit'] != 0 if result['lock_held'] else row['exit'] == 0), 'lock-writer-exit')
        result.update(semantic_complete=True, reason='complete')
    except (ValueError, OSError, subprocess.SubprocessError):
        if in_flight:
            raise
    return result


def lock_fixture_config(root):
    value = dict(disabled_mcp_servers=[], plugins=dict(enabled=[], disabled=[]),
                 mcp_servers={LOCK_NAME:dict(command='/usr/bin/python3',
                     args=['-I',str(root/'witness.py'),str(root/'unexpected-witness')],enabled=True)})
    text = ('disabled_mcp_servers = []\n[plugins]\nenabled = []\ndisabled = []\n'
            '[mcp_servers.' + LOCK_NAME + ']\ncommand = "/usr/bin/python3"\nargs = '
            + json.dumps(value['mcp_servers'][LOCK_NAME]['args']) + '\nenabled = true\n')
    path = root/'home/.grok/config.toml'
    require(not path.exists(), 'lock-config-collision')
    path.write_text(text); path.chmod(0o600)
    lock = path.with_name('.config-init.lock')
    with lock.open('xb'): pass
    lock.chmod(0o600)
    (root/'witness.py').write_text("import pathlib,sys\npathlib.Path(sys.argv[1]).write_bytes(b'unexpected')\nraise SystemExit(93)\n")
    (root/'witness.py').chmod(0o600)
    return value


def lock_config_bytes(root, before, after):
    descriptors, parents = [], []
    try:
        parent = os.open(root, os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW|os.O_CLOEXEC)
        descriptors.append(parent)
        require(tuple(directory_id(os.fstat(parent)).values()) == tuple(before['.']), 'lock-original-root')
        prefix = ''
        for name in ('home','.grok'):
            prefix = name if not prefix else prefix+'/'+name
            child = os.open(name, os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW|os.O_CLOEXEC, dir_fd=parent)
            descriptors.append(child)
            expected = tuple(before[prefix])
            require(tuple(directory_id(os.fstat(child)).values()) == expected, 'lock-original-parent')
            parents.append((parent,name,child,expected)); parent = child
        info = os.stat('config.toml', dir_fd=parent, follow_symlinks=False)
        require(stable(info) == tuple(after['home/.grok/config.toml']), 'lock-config-link')
        fd = os.open('config.toml', os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK|os.O_CLOEXEC, dir_fd=parent)
        descriptors.append(fd)
        require(stable(os.fstat(fd)) == stable(info) and 0 <= info.st_size <= 16384, 'lock-config-open')
        body = os.read(fd, 16385)
        require(len(body) == info.st_size and stable(os.fstat(fd)) == stable(info)
                == stable(os.stat('config.toml', dir_fd=parent, follow_symlinks=False)), 'lock-config-read')
        for p,name,child,expected in parents:
            require(tuple(directory_id(os.fstat(child)).values()) == expected
                    == tuple(directory_id(os.stat(name,dir_fd=p,follow_symlinks=False)).values()), 'lock-parent-replaced')
        require(tuple(directory_id(root.lstat()).values()) == tuple(before['.']), 'lock-root-replaced')
        return body
    finally:
        for fd in reversed(descriptors): os.close(fd)


def lock_ledger(root, before, phase, authored, budget):
    import tomllib
    from devin_structured_inventory_admission import metadata_inventory
    after = metadata_inventory(root,budget)
    ledger = dict(before=before,after=after,accepted=False,config_observed=False,phase=phase)
    try:
        require(set(after) == set(before), 'lock-unknown-membership')
        for name, original in before.items():
            current = after[name]
            if name == 'request.json':
                require(tuple(current[:6]) == tuple(original[:6]) and current[6] <= 4096, 'lock-control-changed')
            elif name == 'home/.grok/config.toml' and phase == LOCK_PHASES[1]:
                require(tuple(current[i] for i in (0,2,3,4,5)) == tuple(original[i] for i in (0,2,3,4,5))
                        and current[3] == 1 and 0 < current[6] <= 16384, 'lock-config-authority')
            else:
                require(current == original, 'lock-original-changed')
        require(not (root/'unexpected-witness').exists(), 'lock-witness-started')
        body = lock_config_bytes(root,before,after)
        expected = json.loads(json.dumps(authored))
        if phase == LOCK_PHASES[1]:
            expected['disabled_mcp_servers'] = [LOCK_NAME]
            expected['mcp_servers'][LOCK_NAME]['enabled'] = False
        value = tomllib.loads(body.decode('utf-8'))
        require(json.dumps(value,sort_keys=True,allow_nan=False) == json.dumps(expected,sort_keys=True,allow_nan=False),
                'lock-unexpected-native-effect')
        ledger.update(accepted=True,config_observed=True,config_bytes=len(body),config_sha256=digest(body))
    except (ValueError,OSError,KeyError,TypeError):
        pass  # Closed negative metadata survives; baseline is never changed.
    return ledger


def lock_readout(members, packet, deadline):
    refused = dict(schema_version=1, mode=LOCK_MODE, reason='readout-refused')
    try:
        def budget():
            require(type(deadline) is float and time.monotonic() < deadline, 'readout-deadline')
        budget()
        summary = document(members['admission.json'])
        require(type(summary) is dict and set(summary) <= set('schema_version collection_complete cleanup_complete qualification scopes source_commit source_tree nonce run attempt error mode measurement_complete'.split())
                and type(summary.get('schema_version')) is int and summary['schema_version'] == 1
                and summary.get('mode') == LOCK_MODE and summary.get('collection_complete') is False
                and summary.get('cleanup_complete') is True
                and summary.get('error') in (None,'admission-refused')
                and ('measurement_complete' not in summary or type(summary['measurement_complete']) is bool), 'lock-summary')
        require(closed(summary['qualification'], ' '.join(FALSE)) == FALSE
                and all(value is False for value in summary['qualification'].values())
                and all(type(summary.get(key)) is str and summary[key] == packet[key]
                        for key in ('source_commit','source_tree','nonce')), 'lock-source')
        receipts = summary['scopes']
        require(type(receipts) is list and 0 < len(receipts) <= 2, 'lock-scopes')
        used, facts = {'admission.json','acquisition.json'}, []
        for index, receipt in enumerate(receipts):
            budget()
            phase = LOCK_PHASES[index]
            result = dict(receipt['result'])
            commands, restored = result['commands'], []
            require(type(commands) is list and len(commands) <= 4, 'lock-commands')
            for number, row in enumerate(commands):
                budget()
                closed(row, 'label argv exit error merged_bytes merged_sha256 merged_file')
                name = phase+'-'+LOCK_LABELS[number]+'.merged.log'
                require(row['merged_file'] == name, 'lock-stream-label')
                body = members[name]; used.add(name)
                restored.append({key:value for key,value in row.items() if key != 'merged_file'})
                restored[-1]['merged_base64'] = base64.b64encode(body).decode()
            rpc_name, ledger_name = phase+'.rpc.json', phase+'.ledger.json'
            used.update((rpc_name,ledger_name))
            result['rpc'] = document(members[rpc_name])
            require(result['rpc'] == rpc_state(), 'lock-unexpected-rpc')
            result['commands'] = restored
            request = dict(packet,schema_version=1,scope='a',mode=LOCK_MODE,phase=phase)
            validate_receipt(dict(receipt,result=result),request)
            tails = (['--version'],['mcp','--help'],['mcp','disable','--help'],['mcp','disable',LOCK_NAME])
            native = restored[0]['argv'][0] if restored else ''
            require(re.fullmatch(r'/tmp/uap-grok-acp-sealed-[0-9]+/grok',native), 'lock-native-path')
            require(all(row['argv'] == [native,*tails[number]] for number,row in enumerate(restored)), 'lock-native-argv')
            if result['semantic_complete']:
                lock_help(restored[:3])
                require(restored[3]['error'] is None and (restored[3]['exit'] != 0 if index == 0 else restored[3]['exit'] == 0), 'lock-writer-status')
            ledger = document(members[ledger_name])
            fields = set('before after accepted config_observed phase'.split())
            require(type(ledger) is dict and type(ledger.get('accepted')) is bool
                    and type(ledger.get('config_observed')) is bool and ledger['accepted'] == ledger['config_observed']
                    and ledger.get('phase') == phase
                    and set(ledger) == fields | ({'config_bytes','config_sha256'} if ledger['accepted'] else set()), 'lock-ledger')
            if ledger['accepted']:
                require(type(ledger['config_bytes']) is int and 0 < ledger['config_bytes'] <= 16384
                        and type(ledger['config_sha256']) is str and re.fullmatch('[0-9a-f]{64}',ledger['config_sha256']), 'lock-config-fact')
            before, after = ledger['before'], ledger['after']
            require(type(before) is dict and type(after) is dict and len(before) <= 4096 and len(after) <= 4096, 'lock-ledger-bound')
            facts.append(dict(phase=phase,semantic_complete=result['semantic_complete'],lock_held=result['lock_held'],
                              ledger_accepted=ledger['accepted'],config_observed=ledger['config_observed'],commands=len(restored),
                              failed_commands=sum(row['error'] is not None or row['exit'] != 0 for row in restored),
                              added_entries=len(after.keys()-before.keys()),removed_entries=len(before.keys()-after.keys()),
                              changed_entries=sum(before[key] != after[key] for key in before.keys() & after.keys())))
        require(used == set(members), 'lock-unjoined-members')
        require(summary.get('measurement_complete') is not True or len(facts) == 2
                and all(row['semantic_complete'] and row['ledger_accepted'] for row in facts), 'lock-incomplete')
        budget()
        return dict(schema_version=1,mode=LOCK_MODE,reason='measurement-only',collection_complete=False,cleanup_complete=True,phases=facts)
    except Exception:
        return refused


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


def post_ledger(root, before, roots, profiles, budget, bootstrap):
    from devin_structured_inventory_admission import metadata_inventory
    from client_automation_native_probe import snapshot
    after = metadata_inventory(root, budget)
    current, added, accepted = {}, [], False
    try:
        bootstrap.metadata(before, after)  # All membership/authority before any new byte read.
        budget.check()
        current = {name: snapshot(path) for name, path in roots.items()}
        uuid_body = ordinary(root / 'home/.grok/agent_id', 36, expected=after['home/.grok/agent_id'])
        require(metadata_inventory(root, budget) == after, 'profile changed while hashing')
        added = bootstrap.admit(before, after, profiles, current, uuid_body)
        accepted = True
    except (ValueError, OSError):
        current = {}  # Never publish a partial/refused byte snapshot as complete.
    return dict(accepted=accepted, before=before, after=after, profiles_before=profiles, profiles_after=current, native_bootstrap=added)


def validate_bootstrap_capture(ledger, capture):
    """Join candidate bytes to both original post-ledgers; this admits no effects."""
    closed(capture, 'schema_version files')
    require(type(capture['schema_version']) is int and capture['schema_version'] == 1, 'bootstrap-schema')
    files = capture['files']
    require(type(files) is dict and set(files) == set(BOOTSTRAP_FILES), 'bootstrap-files')
    owner = ledger['before']['.']
    require(len(owner) == 5 and all(type(v) is int for v in owner), 'bootstrap-owner')
    total = 0
    for path, row in files.items():
        if path not in ledger['after']:
            require(row is None, 'bootstrap-absent')
            continue
        closed(row, 'identity bytes sha256 body_base64')
        identity = row['identity']
        require(type(identity) in (list, tuple) and len(identity) == 9
                and all(type(v) is int for v in identity)
                and tuple(identity) == tuple(ledger['after'][path])
                and stat.S_ISREG(identity[2]) and identity[3] == 1
                and identity[0] == owner[0] and tuple(identity[4:6]) == tuple(owner[3:5]), 'bootstrap-identity')
        require(type(row['bytes']) is int and 0 <= row['bytes'] <= min(LIMIT, BOOTSTRAP_LIMIT - total)
                and row['bytes'] == identity[6] and type(row['body_base64']) is str
                and len(row['body_base64']) <= 4 * ((row['bytes'] + 2) // 3), 'bootstrap-bound')
        body = base64.b64decode(row['body_base64'], validate=True)
        profile = ledger['profiles_after']['user_profile'][path.removeprefix('home/.grok/')]
        require(len(body) == row['bytes'] and digest(body) == row['sha256'] == profile['sha256']
                and profile['bytes'] == row['bytes'] and profile['device'] == identity[0]
                and profile['inode'] == identity[1] and profile['mode'] == stat.S_IMODE(identity[2]), 'bootstrap-content')
        total += len(body)


def capture_bootstrap_files(root, ledger, budget):
    """Read only ten task-owned candidates after the strict controller has settled."""
    from devin_structured_inventory_admission import metadata_inventory
    require(type(ledger.get('profiles_after')) is dict
            and type(ledger['profiles_after'].get('user_profile')) is dict, 'bootstrap-profile-snapshot')
    capture, total = dict(schema_version=1, files={}), 0
    for path in BOOTSTRAP_FILES:
        budget.check()
        if path not in ledger['after']:
            capture['files'][path] = None
            continue
        target = root / path
        require(tuple(ledger['after'][path]) == stable(target.lstat()), 'bootstrap-original-identity')
        body = ordinary(target, min(LIMIT, BOOTSTRAP_LIMIT - total), expected=ledger['after'][path])
        require(tuple(ledger['after'][path]) == stable(target.lstat()), 'bootstrap-final-identity')
        capture['files'][path] = dict(identity=list(ledger['after'][path]), bytes=len(body),
                                     sha256=digest(body), body_base64=base64.b64encode(body).decode())
        total += len(body)
    validate_bootstrap_capture(ledger, capture)
    require(metadata_inventory(root, budget) == ledger['after'], 'bootstrap-final-metadata')
    require(len(json.dumps(dict(ledger, bootstrap_capture=capture)).encode()) <= LIMIT, 'bootstrap-ledger-bound')
    return capture


def inventory_rows(result, root, scope):
    require(scope in SCOPES, 'inventory-scope')
    rows = result['rpc']['plugins']['result']['result']['plugins']
    user_names = ('uap-acp-collision', 'uap-acp-foreign-user') if scope == 'user' else ('uap-acp-foreign-user',)
    wanted = {str(root / 'home/.grok/plugins' / name): ('user', name) for name in user_names}
    if scope != 'user':
        wanted.update({str(root / scope / '.grok/plugins' / name): ('project', name) for name in ('uap-acp-collision', 'uap-acp-foreign-' + scope)})
    require(type(rows) is list and len(rows) == len(wanted), 'exact-inventory-count')
    seen, matched = set(), set()
    for row in rows:
        require(type(row) is dict and type(row.get('id')) is str and row['id'] and row['id'] not in seen, 'plugin-id')
        seen.add(row['id'])
        require(type(row.get('trusted')) is bool and type(row.get('enabled')) is bool, 'plugin-state')
        require(type(row.get('root')) is str and row['root'] in wanted, 'unknown-plugin-root')
        expected_scope, name = wanted[row['root']]
        require(row.get('scope') == expected_scope and row.get('name') == name and row['root'] not in matched, 'plugin-source')
        require(row['id'] == expected_scope + '/' + digest(row['root'].encode())[:8] + '/' + name, 'full-plugin-id')
        conflict = 'Name collision: shadowing "uap-acp-collision" from ' + str(root / 'home/.grok/plugins/uap-acp-collision')
        require(row.get('conflict') == (conflict if scope != 'user' and name == 'uap-acp-collision' else None), 'plugin-precedence')
        matched.add(row['root'])
    require(matched == set(wanted), 'exact-inventory-incomplete')
    return {row['root']: row['id'] for row in rows}


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


def publish(output, bodies, held=None, expected=None, *, mode=None):
    """Single descriptor-anchored attempt; only the returned receipt permits upload."""
    import native_client_project_artifacts as artifacts
    allowed = {'admission.json', 'acquisition.json'} | {scope + '.rpc.json' for scope in SCOPES} | {scope + '.ledger.json' for scope in SCOPES} | {scope + '-' + label + '.merged.log' for scope in SCOPES for label in LABELS}
    if mode == LOCK_MODE:
        allowed = {'admission.json','acquisition.json'} | {phase+suffix for phase in LOCK_PHASES for suffix in ('.rpc.json','.ledger.json')} | {phase+'-'+label+'.merged.log' for phase in LOCK_PHASES for label in LOCK_LABELS}
    else:
        require(mode is None, 'publication-mode')
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


def publication_readout(raw, packet, deadline, *, mode=None):
    """Diagnostic only: never withhold an already admitted original ZIP."""
    import io
    import struct
    import zipfile
    refused = dict(schema_version=1, mode=LOCK_MODE, reason='readout-refused') if mode == LOCK_MODE else dict(schema_version=1, reason='readout-refused', scopes=[])
    try:
        def budget():
            require(type(deadline) is float and time.monotonic() < deadline, 'readout-deadline')
        budget()
        require(type(raw) is bytes and 0 < len(raw) <= 17 << 20, 'readout-archive-bound')
        allowed = {'admission.json', 'acquisition.json'} | {s + suffix for s in SCOPES for suffix in ('.rpc.json', '.ledger.json')} | {s + '-' + label + '.merged.log' for s in SCOPES for label in LABELS}
        if mode == LOCK_MODE:
            allowed = {'admission.json','acquisition.json'} | {phase+suffix for phase in LOCK_PHASES for suffix in ('.rpc.json','.ledger.json')} | {phase+'-'+label+'.merged.log' for phase in LOCK_PHASES for label in LOCK_LABELS}
        require(len(raw) >= 22, 'readout-eocd')
        end = struct.unpack('<4s4H2LH', raw[-22:])
        require(end[0] == b'PK\x05\x06' and end[1] == end[2] == 0
                and end[3] == end[4] and 0 < end[4] <= 23 and end[5] <= 16 << 10
                and end[6] + end[5] == len(raw) - 22 and end[7] == 0, 'readout-central-bound')
        with zipfile.ZipFile(io.BytesIO(raw)) as archive:
            infos = archive.infolist()
            require(len(infos) == end[4], 'readout-central-count')
            require(0 < len(infos) <= 23 and len({i.filename for i in infos}) == len(infos)
                    and all(i.filename in allowed and not i.is_dir() and not i.flag_bits & 1
                            and i.compress_type in (zipfile.ZIP_STORED, zipfile.ZIP_DEFLATED)
                            and 0 <= i.file_size <= LIMIT for i in infos)
                    and sum(i.file_size for i in infos) <= 16 << 20, 'readout-members')
            members = {}
            for info in infos:
                budget()
                body = archive.read(info)
                require(len(body) == info.file_size, 'readout-member-size')
                members[info.filename] = body
        budget()
        if mode == LOCK_MODE:
            return lock_readout(members, packet, deadline)
        if mode == SESSION_MODE:
            return session_readout(members, packet, deadline)
        require(mode is None, 'readout-mode')
        summary = document(members['admission.json'])
        require(type(summary) is dict and set(summary) <= set('schema_version collection_complete cleanup_complete qualification scopes source_commit source_tree nonce run attempt error'.split())
                and type(summary.get('schema_version')) is int and summary['schema_version'] == 1
                and type(summary.get('collection_complete')) is bool and summary.get('cleanup_complete') is True
                and summary.get('error') in (None, 'admission-refused'), 'readout-summary')
        require(closed(summary['qualification'], ' '.join(FALSE)) == FALSE
                and all(v is False for v in summary['qualification'].values()), 'readout-qualification')
        for key in ('source_commit', 'source_tree', 'nonce'):
            require(type(summary.get(key)) is str and summary[key] == packet[key], 'readout-source')
        receipts = summary['scopes']
        require(type(receipts) is list and len(receipts) <= len(SCOPES), 'readout-scopes')
        require(not summary['collection_complete'] or len(receipts) == len(SCOPES), 'readout-incomplete')
        facts = []
        for index, receipt in enumerate(receipts):
            budget()
            scope = SCOPES[index]
            require(type(receipt) is dict and receipt.get('scope') == scope
                    and receipt.get('cleanup_complete') is True and receipt.get('sealed_cleanup_complete') is True, 'readout-cleanup')
            result = receipt['result']
            for obj in (receipt, result):
                for key in ('source_commit', 'source_tree', 'nonce'):
                    require(type(obj.get(key)) is str and obj[key] == packet[key], 'readout-scope-source')
                require(type(obj.get('schema_version')) is int and obj['schema_version'] == 1 and obj.get('scope') == scope, 'readout-scope')
            reason = result['reason']
            require(type(reason) is str and reason in ('complete', 'protocol-refused', 'help-refused', 'isolation-refused', 'collector-refused')
                    and type(result['semantic_complete']) is bool and result['semantic_complete'] == (reason == 'complete'), 'readout-reason')
            commands = result['commands']
            require(type(commands) is list and len(commands) <= 5, 'readout-commands')
            failed = 0
            for number, row in enumerate(commands):
                budget()
                require(type(row) is dict and row.get('label') == LABELS[number]
                        and row.get('merged_file') == scope + '-' + LABELS[number] + '.merged.log'
                        and type(row.get('merged_bytes')) is int and 0 <= row['merged_bytes'] <= LIMIT
                        and type(row.get('merged_sha256')) is str and re.fullmatch('[0-9a-f]{64}', row['merged_sha256'])
                        and (row.get('exit') is None or type(row['exit']) is int)
                        and row.get('error') in (None, 'deadline', 'stream-bound', 'process-refused', 'protocol-refused'), 'readout-command')
                body = members[row['merged_file']]
                require(len(body) == row['merged_bytes'] and digest(body) == row['merged_sha256'], 'readout-stream')
                failed += row.get('exit') != 0 or row.get('error') is not None
            ledger = document(members[scope + '.ledger.json'])
            require(type(ledger) is dict and type(ledger.get('accepted')) is bool
                    and all(type(ledger.get(k)) is dict and len(ledger[k]) <= 4096 for k in ('before', 'after')), 'readout-ledger')
            if 'bootstrap_capture' in ledger:
                require(scope == 'user', 'bootstrap-scope')
                validate_bootstrap_capture(ledger, ledger['bootstrap_capture'])
            before, after = ledger['before'], ledger['after']
            facts.append(dict(scope=scope, reason=reason, semantic_complete=result['semantic_complete'],
                              ledger_accepted=ledger['accepted'], commands=len(commands), failed_commands=failed,
                              added_entries=len(after.keys() - before.keys()), removed_entries=len(before.keys() - after.keys()),
                              changed_entries=sum(before[k] != after[k] for k in before.keys() & after.keys())))
        budget()
        return dict(schema_version=1, reason='collection-complete' if summary['collection_complete'] else 'admission-refused',
                    collection_complete=summary['collection_complete'], cleanup_complete=True, scopes=facts)
    except Exception:
        return refused


def admit_publication(output, packet, expected_commit, expected_tree, destination, diagnostics=None, diagnostic_deadline=None, mode=None):
    """Workflow parent validates held bytes; no scanning a stale output prefix."""
    require(diagnostics is None or type(diagnostics) is list and not diagnostics, 'readout-output')
    closed(packet, 'schema_version source_commit source_tree nonce publication')
    require(type(packet['schema_version']) is int and packet['schema_version'] == 1
            and packet['source_commit'] == expected_commit and packet['source_tree'] == expected_tree
            and re.fullmatch('[0-9a-f]{40}', expected_tree)
            and re.fullmatch('[0-9a-f]{32}', packet['nonce']), 'publication-source')
    proof = closed(packet['publication'], 'output upload archive bytes sha256')
    for name in ('output', 'upload', 'archive'):
        fields = 'device inode mode uid gid' + (' links size mtime_ns ctime_ns' if name == 'archive' else '')
        identity = closed(proof[name], fields)
        require(all(type(number) is int for number in identity.values()), 'publication-identity-types')
    require(type(proof['sha256']) is str and re.fullmatch('[0-9a-f]{64}', proof['sha256']), 'publication-digest')
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
        if diagnostics is not None:
            diagnostics.append(publication_readout(raw, packet, diagnostic_deadline) if mode is None
                               else publication_readout(raw, packet, diagnostic_deadline, mode=mode))
        return destination / 'research.zip'
    finally:
        for descriptor in reversed(descriptors): os.close(descriptor)


def run(output, *, session_only=False, lock_qualification=False):
    import project_native_research as shared
    from grok_acp_bootstrap import NativeBootstrap
    from project_native_collectors.grok import ACQUISITION
    from devin_structured_inventory_admission import metadata_inventory
    from client_automation_native_probe import snapshot
    shared.native.require_hosted('linux-amd64')
    require(not (session_only and lock_qualification), 'mode')
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
            if session_only:
                session_fixture_config(root, nonce)
                summary['mode'] = SESSION_MODE
            if lock_qualification:
                authored = lock_fixture_config(root)
                summary['mode'] = LOCK_MODE
            (root / 'request.json').write_bytes(b'{}\n')
            baseline = None if session_only or lock_qualification else metadata_inventory(root, budget)
            profiles = {name: snapshot(path) for name, path in roots.items()}
            bootstrap = NativeBootstrap()
            bodies['acquisition.json'] = json.dumps(receipt, sort_keys=True).encode()
            native_ids = {}
            phases = LOCK_PHASES if lock_qualification else (('a',) if session_only else SCOPES)
            for stage in phases:
                scope = 'a' if lock_qualification else stage
                prefix = stage if lock_qualification else scope
                require(time.monotonic() < origin + 150, 'outer-deadline')
                request = dict(schema_version=1, source_commit=commit, source_tree=tree, uid=os.getuid(), gid=os.getgid(), nonce=nonce,
                               scope=scope, start_monotonic_ns=time.monotonic_ns(), helper_sha256=digest(ordinary(helper, 64 << 20)))
                if session_only:
                    request['mode'] = SESSION_MODE
                if lock_qualification:
                    request.update(mode=LOCK_MODE,phase=stage)
                (root / 'request.json').write_text(json.dumps(request))
                if session_only or lock_qualification and baseline is None:
                    # The one authored control request is fixed before Baseline0.
                    require(baseline is None, 'session-baseline-renewal')
                    baseline = metadata_inventory(root, budget)
                cleanup_proved = False
                receipt = helper_capture(helper, root, min(origin + 150, request['start_monotonic_ns'] / 1e9 + 35))
                result = validate_receipt(receipt, request)
                cleanup_proved = True
                ledger = lock_ledger(root,baseline,stage,authored,budget) if lock_qualification else session_metadata_ledger(root, baseline, budget) if session_only else post_ledger(root, baseline, roots, profiles, budget, bootstrap)
                bodies[prefix + '.ledger.json'] = json.dumps(ledger, sort_keys=True).encode()
                for row in result['commands']:
                    name = prefix + '-' + row['label'] + '.merged.log'
                    bodies[name] = base64.b64decode(row.pop('merged_base64'), validate=True)
                    row['merged_file'] = name
                bodies[prefix + '.rpc.json'] = json.dumps(result.pop('rpc'), sort_keys=True).encode()
                summary['scopes'].append(receipt)
                result['rpc'] = document(bodies[prefix + '.rpc.json'])
                if scope == 'user' and ledger['accepted']:
                    ledger['bootstrap_capture'] = capture_bootstrap_files(root, ledger, budget)
                    bodies[prefix + '.ledger.json'] = json.dumps(ledger, sort_keys=True).encode()
                if lock_qualification:
                    result.pop('rpc')
                    require(ledger['accepted'] and result['semantic_complete'], 'lock-observation-refused')
                    continue
                if session_only:
                    summary['measurement_complete'] = True
                    result.pop('rpc')
                    require(ledger['protected_originals_unchanged'] and result['semantic_complete'], 'native-refused')
                    continue  # Unknown session effects never become an admitted envelope.
                require(ledger['accepted'] and result['semantic_complete'], 'native-refused')
                observed_ids = inventory_rows(result, root, scope)
                require(all(path not in native_ids or native_ids[path] == value for path, value in observed_ids.items()), 'native-id-changed')
                native_ids.update(observed_ids)
                result.pop('rpc')
            require(not source(['status', '--porcelain', '--untracked-files=normal']), 'source-changed')
            summary['collection_complete'] = not session_only and not lock_qualification
            if lock_qualification:
                summary['measurement_complete'] = True
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
            publication = publish(output, bodies, output_fd, output_identity, mode=LOCK_MODE) if lock_qualification else publish(output, bodies, output_fd, output_identity)
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
    parser.add_argument('--session-only', action='store_true')
    parser.add_argument('--lock-qualification', action='store_true')
    args = parser.parse_args()
    if args.collect is not None and args.output is None and not args.session_only and not args.lock_qualification:
        collect(args.collect)
    else:
        require(args.output is not None and args.collect is None, 'mode')
        raise SystemExit(run(args.output, session_only=args.session_only, lock_qualification=args.lock_qualification))
