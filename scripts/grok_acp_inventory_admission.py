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
CLI_MODE = 'plugin-cli-observation-v1'
CLI_PHASES = ('details-help', 'list-json', 'details-full-id', 'details-name', 'inspect-project')
CLI_MEMBER = 'plugin-cli-observation.json'
SESSION_CONFIG, SESSION_CONFIG_LIMIT = 'home/.grok/config.toml', 16 << 10
# agent_build is measured in original 14058338742, not the supplementary public enum.
SETUP_PHASES = frozenset(('auth', 'resolve_workspace', 'folder_trust', 'plugin_registry', 'mcp_merge',
    'local_workspace', 'persistence_init', 'spawn_session_actor', 'agent_build', 'workspace_handshake', 'model_switch',
    'model_not_allowed', 'git_discovery', 'finalize_response', 'tool_overrides', 'response_ready'))
# Literal catalogs from original session artifact 14100597648, frame 13. Metadata only.
SESSION_COMMAND_NAMES = ('compact', 'always-approve', 'context', 'plugins', 'reload-plugins',
    'session-info', 'feedback', 'deep-research', 'workflow', 'goal', 'loop')
SESSION_TOOL_NAMES = ('run_terminal_command', 'read_file', 'search_replace', 'list_dir', 'grep',
    'kill_command_or_subagent', 'todo_write', 'get_command_or_subagent_output', 'spawn_subagent',
    'scheduler_create', 'scheduler_delete', 'scheduler_list', 'monitor', 'search_tool', 'use_tool',
    'workflow', 'enter_plan_mode', 'exit_plan_mode', 'ask_user_question', 'send_feedback', 'web_search',
    'image_gen', 'image_edit', 'image_to_video', 'reference_to_video', 'write')


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


def session_metadata_text(value, maximum):
    return type(value) is str and 0 < len(value) <= maximum and value.isascii() and all(32 <= ord(x) <= 126 for x in value)


def session_available_command(value, name):
    row = closed(value, 'name description input _meta' if name == 'deep-research' else 'name description input')
    require(row['name'] == name and session_metadata_text(row['description'], 256), 'available-command')
    if row['input'] is not None:
        require(session_metadata_text(closed(row['input'], 'hint')['hint'], 128), 'available-command-input')
    if name == 'deep-research':
        meta = closed(row['_meta'], 'workflowPath workflowSource')
        require(meta['workflowSource'] == 'builtin' and session_metadata_text(meta['workflowPath'], 1024),
                'available-command-meta')  # Retained text; never a path to open or a command to execute.


def session_available_commands(value, state, req):
    require(req.get('mode') == SESSION_MODE and state['initialize'] is not None and state['plugins'] is None
            and not any(x['method'] == 'session/update' for x in state['notifications']), 'available-command-order')
    params = closed(value, 'sessionId update _meta')
    sid = params['sessionId']
    require(session_metadata_text(sid, 256), 'session-id')
    update = closed(params['update'], 'sessionUpdate availableCommands _meta')
    require(update['sessionUpdate'] == 'available_commands_update', 'available-command-kind')
    tools = closed(update['_meta'], 'tools')['tools']
    require(type(tools) is list and tools == list(SESSION_TOOL_NAMES), 'available-command-tools')
    commands = update['availableCommands']
    require(type(commands) is list and len(commands) == len(SESSION_COMMAND_NAMES), 'available-command-count')
    for row, name in zip(commands, SESSION_COMMAND_NAMES):
        session_available_command(row, name)
    meta = closed(params['_meta'], 'agentTimestampMs eventId requestTrimmedTokens totalTokens updateParams updateType')
    require(type(meta['agentTimestampMs']) is int and 0 < meta['agentTimestampMs'] < 1 << 63
            and type(meta['totalTokens']) is int and 0 <= meta['totalTokens'] <= 1 << 20
            and type(meta['requestTrimmedTokens']) is int and meta['requestTrimmedTokens'] == 0
            and meta['eventId'] == sid + '-0' and meta['updateType'] == 'AvailableCommandsUpdate',
            'available-command-meta')
    count = closed(meta['updateParams'], 'commandsCount')['commandsCount']
    require(type(count) is int and count == len(SESSION_COMMAND_NAMES), 'available-command-count')
    for item in state['notifications']:
        if item['method'] in ('_x.ai/session/setup', '_x.ai/mcp_initialized'):
            require(item['params']['sessionId'] in (None, sid), 'session-notification-identity')


def session_rpc_frame(value, state, req, send):
    require(type(value) is dict and value.get('jsonrpc') == '2.0', 'rpc-envelope')
    if 'method' in value:
        closed(value, 'jsonrpc method params')
        require(len(state['notifications']) < 32, 'notification-bound')
        if value['method'] == '_x.ai/mcp/servers_updated':
            require(value['params'] == {'mcpServers': []}
                    and not any(x['method'] == value['method'] for x in state['notifications']), 'notification-refused')
        elif value['method'] == '_x.ai/mcp_initialized':
            params = closed(value['params'], 'sessionId mcpToolCount elapsedMs')
            require(state['initialize'] is not None and state['plugins'] is None
                    and not any(x['method'] == value['method'] for x in state['notifications']), 'notification-refused')
            require(type(params['mcpToolCount']) is int and params['mcpToolCount'] == 0
                    and type(params['elapsedMs']) is int and params['elapsedMs'] == 0,
                    'mcp-initialized-bound')
            sid = params['sessionId']
            require(type(sid) is str and 0 < len(sid) <= 256 and sid.isascii()
                    and all(32 <= ord(x) <= 126 for x in sid), 'session-id')
            for item in state['notifications']:
                if item['method'] == '_x.ai/session/setup':
                    require(item['params']['sessionId'] in (None, sid), 'session-notification-identity')
        elif value['method'] == 'session/update':
            session_available_commands(value['params'], state, req)
        else:
            require(value['method'] == '_x.ai/session/setup', 'notification-refused')
            params = closed(value['params'], 'method phase sessionId')
            require(params['method'] == 'session/new' and type(params['phase']) is str and params['phase'] in SETUP_PHASES
                    and sum(x['params'].get('phase') == params['phase'] for x in state['notifications']) < 2,
                    'setup-notification')
            require(params['sessionId'] is None or type(params['sessionId']) is str
                    and 0 < len(params['sessionId']) <= 256 and params['sessionId'].isascii()
                    and all(32 <= ord(x) <= 126 for x in params['sessionId']), 'session-id')
        if value['method'] in ('_x.ai/session/setup', '_x.ai/mcp_initialized', 'session/update'):
            if state['session'] is not None:
                require(value['params']['sessionId'] in (None, state['session']['result']['sessionId']), 'session-notification-identity')
            for item in state['notifications']:
                if item['method'] in ('_x.ai/mcp_initialized', 'session/update'):
                    require(value['params']['sessionId'] in (None, item['params']['sessionId']), 'session-notification-identity')
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
            if item['method'] in ('_x.ai/session/setup', '_x.ai/mcp_initialized', 'session/update'):
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
METADATA_LABELS = LABELS[:4] + ('mcp-inventory',) + LABELS[4:]
FRONTEND_STAGES = frozenset(('preflight', 'source', 'acquisition', 'build', 'fixtures', 'baseline',
    'helper', 'receipt', 'ledger', 'inventory', 'source-final', 'cleanup', 'archive', 'receipt-output'))
READONLY_HELPER_CODES = frozenset('''readonly-helper-unclassified readonly-primitive-refused
readonly-primitive-missing readonly-primitive-permission
ordinary-bwrap-refused bwrap-capability-refused readonly-identity-refused
readonly-directory-refused readonly-directory-join-refused overflow-id-refused
readonly-input-authority readonly-input-directory-read readonly-input-bound
readonly-input-directory-open readonly-input-directory-final readonly-input-directory-changed
readonly-root-owner readonly-overflow-ambiguity readonly-input-proof readonly-file-proof
readonly-file-capability-refused readonly-runtime-root readonly-device readonly-parent-namespace
readonly-result-refused readonly-proof-type-refused readonly-stage-refused readonly-completion-refused
readonly-original-authority-missing readonly-original-authority-changed original-deadline-refused
file-stat-refused absolute-path-refused held-file-refused held-file-changed file-authority-refused
file-read-refused held-content-changed system-ancestor-refused system-executable-refused sealed-copy-refused
mount-authority-refused sealed-directory-refused sealed-directory-changed sealed-mount-changed
sealed-deletion-unproved json-structure-refused json-key-refused json-delimiter-refused json-trailing-refused
json-fields-refused json-field-missing result-isolation-refused result-bound-refused result-identity-refused
result-reason-refused collector-frame-refused strict-cleanup-unproved strict-exchange-missing
input-root-mode-refused sealed-cleanup-refused receipt-output-refused system-selection-refused
ordinary-bwrap-selection-refused'''.split())
READONLY_HELPER_ERRORS = frozenset('helper-' + code for code in READONLY_HELPER_CODES)
FRONTEND_ERRORS = frozenset(('none', 'contract', 'io', 'process', 'other')) | READONLY_HELPER_ERRORS
STARTUP_NAME = 'uap-acp-startup-witness'
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


class ReadonlyHelperRefusal(ValueError):
    def __init__(self, code):
        require(type(code) is str and code in READONLY_HELPER_CODES, 'helper-refused')
        self.error_class = 'helper-' + code
        super().__init__(self.error_class)


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


def frontend_packet(body, exit_code):
    """A closed negative frame is diagnostic only and never publication authority."""
    packet = document(body)
    require(body == json.dumps(packet, separators=(',', ':')).encode() + b'\n', 'publication-frame')
    if type(packet) is dict and set(packet) == set('schema_version source_commit source_tree nonce publication'.split()):
        return packet
    closed(packet, 'schema_version kind stage error_class source_identity_checked cleanup_complete publication_attempted')
    require(type(exit_code) is int and exit_code == 1 and packet['kind'] == 'frontend-publication-refused'
            and type(packet['schema_version']) is int and packet['schema_version'] == 1
            and type(packet['stage']) is str and packet['stage'] in FRONTEND_STAGES
            and type(packet['error_class']) is str and packet['error_class'] in FRONTEND_ERRORS
            and all(type(packet[key]) is bool for key in ('source_identity_checked', 'cleanup_complete', 'publication_attempted')),
            'frontend-refusal-frame')
    if packet['error_class'] in READONLY_HELPER_ERRORS:
        require(packet['stage'] == 'helper' and packet['source_identity_checked']
                and not packet['cleanup_complete'] and not packet['publication_attempted'], 'helper-refusal-authority')
    print('GROK_FRONTEND_REFUSAL=' + json.dumps(packet, separators=(',', ':')))
    raise ValueError('frontend-publication-refused')


def frontend_state():
    return dict(stage='preflight', failure_stage=None, error_class='none', source_identity_checked=False,
                cleanup_complete=False, publication_attempted=False, receipt_emitted=False)


def frontend_error(state, error):
    state['failure_stage'] = state['stage']
    state['error_class'] = (error.error_class if isinstance(error, ReadonlyHelperRefusal) else
                            'contract' if isinstance(error, ValueError) else 'io' if isinstance(error, OSError)
                            else 'process' if isinstance(error, subprocess.SubprocessError) else 'other')


def frontend(output, *, session_only=False, lock_qualification=False, plugin_cli=False):
    """Report a missing receipt only after run's existing cleanup/finally path."""
    state = frontend_state()
    try:
        status = run(output, session_only=session_only, lock_qualification=lock_qualification, plugin_cli=plugin_cli, _frontend=state)
    except Exception as error:
        frontend_error(state, error)
        status = 1
    if not state['receipt_emitted']:
        packet = dict(schema_version=1, kind='frontend-publication-refused',
                      stage=state['failure_stage'] or state['stage'], error_class=state['error_class'],
                      source_identity_checked=state['source_identity_checked'], cleanup_complete=state['cleanup_complete'],
                      publication_attempted=state['publication_attempted'])
        print(json.dumps(packet, separators=(',', ':')))
        return 1
    return status


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


def rpc_frame(value, state, req, send, *, expected_servers=()):
    require(type(value) is dict and value.get('jsonrpc') == '2.0', 'rpc-envelope')
    if 'method' in value:
        closed(value, 'jsonrpc method params')
        require(value['method'] == '_x.ai/mcp/servers_updated'
                and value['params'] == {'mcpServers': list(expected_servers)}
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


def command(argv, label, cwd, env, deadline, remaining, state, req, *, expected_servers=()):
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
                            frame = document(bytes(line), 64 << 10)
                            completed = (session_rpc_frame(frame, state, req, send) if req.get('mode') == SESSION_MODE
                                         else rpc_frame(frame, state, req, send, expected_servers=expected_servers))
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
        if not session:
            command_in_flight = True
            row = command([str(native), 'inspect', '--json'], 'mcp-inventory', root / req['scope'], env,
                          min(deadline, time.monotonic() + 5),
                          LIMIT - sum(item['merged_bytes'] for item in result['commands']), result['rpc'], req)
            command_in_flight = False
            result['commands'].append(row)
            startup_inventory(row, root, req['scope'])
        command_in_flight = True
        row = command([str(native), 'agent', '--no-leader', 'stdio'], 'acp', root / req['scope'], env, deadline,
                      LIMIT - sum(row['merged_bytes'] for row in result['commands']), result['rpc'], req,
                      expected_servers=() if session else startup_servers(root))
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


def cli_arguments(root, phase):
    selected = root / 'a/.grok/plugins/uap-acp-collision'
    full_id = 'project/' + digest(str(selected).encode())[:8] + '/uap-acp-collision'
    require(phase in CLI_PHASES, 'cli-phase')
    return dict(zip(CLI_PHASES, (['plugin', 'details', '--help'], ['plugin', 'list', '--json'],
        ['plugin', 'details', full_id], ['plugin', 'details', 'uap-acp-collision'], ['inspect', '--json'])))[phase]


def cli_collector_result(req):
    root = Path(req['input_root'])
    result = {key:req[key] for key in ('schema_version', 'source_commit', 'source_tree', 'scope', 'nonce')}
    result.update(mode=CLI_MODE, phase=req['phase'], semantic_complete=False, reason='isolation-refused',
                  isolation={}, commands=[], rpc=rpc_state())
    in_flight = False
    try:
        require(req['scope'] == 'a', 'cli-scope')
        args = cli_arguments(root, req['phase'])
        result['isolation'] = isolation(req)
        env = {'PATH':'/usr/bin:/bin', 'LANG':'C.UTF-8', 'LC_ALL':'C.UTF-8', 'HOME':str(root/'home'),
               'TMPDIR':str(root/'tmp'), 'TERM':'dumb', 'NO_COLOR':'1', 'GROK_DISABLE_AUTOUPDATER':'1',
               'GIT_CONFIG_NOSYSTEM':'1', 'GIT_CONFIG_GLOBAL':'/dev/null', 'GIT_TERMINAL_PROMPT':'0'}
        env.update({'XDG_'+name.upper()+'_HOME':str(root/('xdg-'+name)) for name in ('config','data','cache','state')})
        result['reason'] = 'protocol-refused'
        require(time.monotonic() < req['start_monotonic_ns']/1e9 + 5, 'cli-deadline')
        in_flight = True
        row = command([str(Path(__file__).resolve().parent/'grok'), *args], req['phase'], root/'a', env,
                      req['start_monotonic_ns']/1e9 + 5, 16 << 10, result['rpc'], req)
        in_flight = False
        result['commands'].append(row)
        require(row['error'] is None and type(row['exit']) is int, 'cli-capture')
        result.update(semantic_complete=True, reason='complete')  # A lookup's nonzero exit is still an observation.
    except (ValueError, OSError, subprocess.SubprocessError):
        if in_flight: raise  # No partial stream or family authority.
    return result


def collect(request):
    req = document(ordinary(request, 16384), 16384)
    session = req.get('mode') == SESSION_MODE
    lock = req.get('mode') == LOCK_MODE
    cli = req.get('mode') == CLI_MODE
    readonly = req.get('mode') == READONLY_MODE
    require('mode' not in req or session or lock or cli or readonly, 'mode')
    closed(req, 'schema_version source_commit source_tree uid gid nonce scope input_root parent_network_namespace start_monotonic_ns' + (' mode readonly' if readonly else ' mode' if session else ' mode phase' if lock or cli else ''))
    body = json.dumps(readonly_collector_result(req) if readonly else cli_collector_result(req) if cli else lock_collector_result(req) if lock else collector_result(req), separators=(',', ':')).encode() + b'\n'
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
    cli = request.get('mode') == CLI_MODE
    result = closed(receipt['result'], 'schema_version source_commit source_tree scope nonce semantic_complete reason isolation commands rpc' + (' mode provider' if session else ' mode phase lock_held' if lock else ' mode phase' if cli else ''))
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
    if cli:
        require(request['scope'] == 'a' and result['mode'] == CLI_MODE and result['phase'] == request['phase'] in CLI_PHASES
                and result['rpc'] == rpc_state() and (not result['semantic_complete'] or len(result['commands']) == 1), 'cli-result')
    labels = (request['phase'],) if cli else LOCK_LABELS if lock else LABELS if session else METADATA_LABELS
    require(type(result['commands']) is list and len(result['commands']) <= len(labels), 'command-count')
    total = 0
    for index, row in enumerate(result['commands']):
        closed(row, 'label argv exit error merged_base64 merged_bytes merged_sha256')
        require(row['label'] == labels[index] and type(row['argv']) is list and all(type(x) is str for x in row['argv'])
                and (row['exit'] is None or type(row['exit']) is int)
                and row['error'] in (None, 'deadline', 'stream-bound', 'process-refused', 'protocol-refused'), 'command-record')
        if cli:
            require(type(row['merged_bytes']) is int and 0 <= row['merged_bytes'] <= 16 << 10
                    and type(row['merged_base64']) is str and len(row['merged_base64']) == 4*((row['merged_bytes']+2)//3), 'cli-stream-bound')
        raw = base64.b64decode(row['merged_base64'], validate=True)
        require(type(row['merged_bytes']) is int and len(raw) == row['merged_bytes'] and digest(raw) == row['merged_sha256'], 'stream-identity')
        if cli:
            root = Path('/tmp/uap-grok-acp-input-' + request['nonce'])
            require(type(row['exit']) is int and -255 <= row['exit'] <= 255 and (not result['semantic_complete'] or row['error'] is None) and len(row['argv']) >= 1
                    and row['argv'][1:] == cli_arguments(root, request['phase'])
                    and re.fullmatch(r'/tmp/uap-grok-acp-sealed-[^/]+/grok', row['argv'][0])
                    and str(Path(row['argv'][0])) == row['argv'][0], 'cli-command')
        total += len(raw)
    require(total <= LIMIT and len(json.dumps(result['rpc']).encode()) <= 128 << 10, 'aggregate-bound')
    closed(result['rpc'], 'initialize plugins notifications' + (' session' if session else ''))
    return result



def session_config_body(nonce):
    url = 'http://127.0.0.1:' + str(session_port(nonce)) + '/v1'
    text = ('[features]\ntelemetry = false\ntitle_refresh = false\n'
            '[models]\ndefault = "uap-local-driver"\nsession_summary = "uap-local-driver"\nprompt_suggestion = "uap-local-driver"\n'
            '[endpoints]\nmodels_base_url = "' + url + '"\n'
            '[model.uap-local-driver]\nmodel = "uap-local-driver"\nbase_url = "' + url + '"\n'
            'api_backend = "chat_completions"\napi_key = "uap-inert-local-driver"\ncontext_window = 64000\n')
    return text.encode()


def session_fixture_config(root, nonce):
    path = root / SESSION_CONFIG
    require(not path.exists(), 'session-config-collision')
    path.write_bytes(session_config_body(nonce))
    path.chmod(0o600)


def session_metadata_ledger(root, before, budget):
    from devin_structured_inventory_admission import metadata_inventory
    after = metadata_inventory(root, budget)
    protected = all(after.get(name) == identity for name, identity in before.items())
    # First capture is never a new bootstrap/effect admission. No new body read.
    return dict(before=before, after=after, accepted=False, protected_originals_unchanged=protected)


def session_config_metadata(ledger):
    before, after = ledger['before'], ledger['after']
    owner = before['.']
    require(type(owner) in (list, tuple) and len(owner) == 5 and all(type(v) is int for v in owner), 'session-config-owner')
    for parent in ('.', 'home', 'home/.grok'):
        info = before[parent]
        require(type(info) in (list, tuple) and len(info) == 5 and all(type(v) is int for v in info)
                and stat.S_ISDIR(info[2]) and info[0] == owner[0] and tuple(info[3:]) == tuple(owner[3:])
                and info == after.get(parent), 'session-config-parent')
    require(all(name == SESSION_CONFIG or after.get(name) == value for name, value in before.items()), 'session-config-originals')
    rows = [before[SESSION_CONFIG], after[SESSION_CONFIG]]
    for info in rows:
        require(type(info) in (list, tuple) and len(info) == 9 and all(type(v) is int for v in info)
                and info[2] == stat.S_IFREG | 0o600 and info[3] == 1 and info[0] == owner[0]
                and tuple(info[4:6]) == tuple(owner[3:]) and 0 <= info[6] <= SESSION_CONFIG_LIMIT,
                'session-config-authority')
    return rows


def validate_session_config_capture(ledger, capture, nonce):
    closed(capture, 'schema_version before after')
    require(type(capture['schema_version']) is int and capture['schema_version'] == 1, 'session-config-schema')
    identities = session_config_metadata(ledger)
    for label, identity in zip(('before', 'after'), identities):
        row = closed(capture[label], 'identity bytes sha256' + (' body_base64' if label == 'after' else ''))
        require(type(row['identity']) is list and all(type(v) is int for v in row['identity'])
                and tuple(row['identity']) == tuple(identity) and type(row['bytes']) is int
                and row['bytes'] == identity[6] and type(row['sha256']) is str
                and re.fullmatch('[0-9a-f]{64}', row['sha256']), 'session-config-record')
        body = session_config_body(nonce)
        if label == 'after':
            require(type(row['body_base64']) is str and len(row['body_base64']) == 4 * ((row['bytes'] + 2) // 3),
                    'session-config-bound')
            body = base64.b64decode(row['body_base64'], validate=True)
        require(len(body) == row['bytes'] and digest(body) == row['sha256'], 'session-config-content')


def capture_session_config(root, ledger, original, nonce, budget, receipt):
    """One selected postimage after positive family settlement; never admit effects."""
    from types import SimpleNamespace
    from devin_structured_inventory_admission import metadata_inventory, read_bootstrap_file
    try:
        require(type(receipt) is dict and receipt.get('cleanup_complete') is True
                and receipt.get('sealed_cleanup_complete') is True, 'session-config-cleanup')
        budget.check()
        before, after = session_config_metadata(ledger)
        require(original == dict(bytes=before[6], device=before[0], inode=before[1], mode=0o600,
                                 sha256=digest(session_config_body(nonce))), 'session-config-original-snapshot')
        ctx = SimpleNamespace(root=root, original_output_identity=tuple(ledger['before']['.']), budget=budget)
        body = read_bootstrap_file(ctx, SESSION_CONFIG, after, ledger['after'])
        capture = dict(schema_version=1, before=dict(identity=list(before), bytes=original['bytes'], sha256=original['sha256']),
                       after=dict(identity=list(after), bytes=len(body), sha256=digest(body), body_base64=base64.b64encode(body).decode()))
        validate_session_config_capture(ledger, capture, nonce)
        require(metadata_inventory(root, budget) == ledger['after'], 'session-config-final-metadata')
        require(len(json.dumps(dict(ledger, selected_config_capture=capture)).encode()) <= LIMIT, 'session-config-ledger-bound')
        return capture
    except (KeyError, TypeError, ValueError, OSError):
        return None  # Failed diagnostics retain no partial body or changed acceptance.


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
            if ledger.get('selected_config_capture') is not None:
                validate_session_config_capture(ledger, ledger['selected_config_capture'], packet['nonce'])
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


def lock_bootstrap_metadata(before, after, created):
    from grok_acp_bootstrap import PINNED, PREFIX, record
    docs = {PREFIX+name:pin for name,pin in PINNED.items() if name.startswith('docs/user-guide/')}
    directories = {PREFIX+name for name in ('docs','docs/user-guide','logs')}
    log = PREFIX+'logs/unified.jsonl'
    names = set(docs) | directories | {log}
    require(len(docs) == 27 and set(after)-set(before) == names, 'lock-unknown-membership')
    require(created is None or set(created) == names, 'lock-startup-proof')
    owner = record(before['.'],5)
    for name in sorted(names):
        meta = record(after[name],5 if name in directories else 9)
        require(meta[0] == owner[0], 'lock-startup-device')
        if name in directories:
            require(meta[2] == stat.S_IFDIR | 0o755 and meta[3:] == owner[3:], 'lock-startup-directory')
        else:
            require(meta[2] == stat.S_IFREG | 0o644 and meta[3] == 1 and meta[4:6] == owner[3:],
                    'lock-startup-file')
            require(meta[6] == docs[name][0] if name in docs else 0 <= meta[6] <= 128 << 10,
                    'lock-startup-bound')
        if created is not None:
            previous = record(created[name],len(meta))
            count = 6 if name == log else len(meta)
            require(meta[:count] == previous[:count], 'lock-startup-replaced')
    return docs


def lock_config_bytes(root, before, after, relative='home/.grok/config.toml'):
    from grok_acp_bootstrap import PINNED, PREFIX
    docs = {PREFIX+name:pin for name,pin in PINNED.items() if name.startswith('docs/user-guide/')}
    require(relative == 'home/.grok/config.toml' or relative in docs, 'lock-read-path')
    limit = 16384 if relative == 'home/.grok/config.toml' else docs[relative][0]
    parts = relative.split('/')
    descriptors, parents = [], []
    try:
        parent = os.open(root, os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW|os.O_CLOEXEC)
        descriptors.append(parent)
        require(tuple(directory_id(os.fstat(parent)).values()) == tuple(before['.']), 'lock-original-root')
        prefix = ''
        for name in parts[:-1]:
            prefix = name if not prefix else prefix+'/'+name
            child = os.open(name, os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW|os.O_CLOEXEC, dir_fd=parent)
            descriptors.append(child)
            expected = tuple(after[prefix])
            require(tuple(directory_id(os.fstat(child)).values()) == expected, 'lock-original-parent')
            parents.append((parent,name,child,expected)); parent = child
        info = os.stat(parts[-1], dir_fd=parent, follow_symlinks=False)
        require(stable(info) == tuple(after[relative]) and stat.S_ISREG(info.st_mode)
                and info.st_nlink == 1 and info.st_dev == before['.'][0]
                and (info.st_uid,info.st_gid) == tuple(before['.'][3:]), 'lock-config-link')
        fd = os.open(parts[-1], os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK|os.O_CLOEXEC, dir_fd=parent)
        descriptors.append(fd)
        require(stable(os.fstat(fd)) == stable(info) and 0 <= info.st_size <= limit, 'lock-config-open')
        body = os.read(fd, limit+1)
        require(len(body) == info.st_size and stable(os.fstat(fd)) == stable(info)
                == stable(os.stat(parts[-1], dir_fd=parent, follow_symlinks=False)), 'lock-config-read')
        for p,name,child,expected in parents:
            require(tuple(directory_id(os.fstat(child)).values()) == expected
                    == tuple(directory_id(os.stat(name,dir_fd=p,follow_symlinks=False)).values()), 'lock-parent-replaced')
        require(tuple(directory_id(root.lstat()).values()) == tuple(before['.']), 'lock-root-replaced')
        return body
    finally:
        for fd in reversed(descriptors): os.close(fd)


def lock_ledger(root, before, phase, authored, budget, *, created=None):
    import tomllib
    from devin_structured_inventory_admission import metadata_inventory
    after = metadata_inventory(root,budget)
    ledger = dict(before=before,after=after,accepted=False,config_observed=False,phase=phase)
    try:
        require(set(before) <= set(after) and phase in LOCK_PHASES, 'lock-original-missing')
        require(phase == LOCK_PHASES[0] or created is not None, 'lock-startup-proof')
        docs = lock_bootstrap_metadata(before,after,created)
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
        hashes = {}
        for name,pin in docs.items():
            budget.check()
            hashes[name] = digest(lock_config_bytes(root,before,after,name))
            require(hashes[name] == pin[1], 'lock-startup-content')
        body = lock_config_bytes(root,before,after)
        expected = json.loads(json.dumps(authored))
        if phase == LOCK_PHASES[1]:
            expected['disabled_mcp_servers'] = [LOCK_NAME]
            expected['mcp_servers'][LOCK_NAME]['enabled'] = False
        value = tomllib.loads(body.decode('utf-8'))
        require(json.dumps(value,sort_keys=True,allow_nan=False) == json.dumps(expected,sort_keys=True,allow_nan=False),
                'lock-unexpected-native-effect')
        require(metadata_inventory(root,budget) == after, 'lock-final-metadata')
        ledger.update(accepted=True,config_observed=True,config_bytes=len(body),config_sha256=digest(body),
                      bootstrap_digests=hashes)
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
                    and set(ledger) == fields | ({'config_bytes','config_sha256','bootstrap_digests'} if ledger['accepted'] else set()), 'lock-ledger')
            if ledger['accepted']:
                require(type(ledger['config_bytes']) is int and 0 < ledger['config_bytes'] <= 16384
                        and type(ledger['config_sha256']) is str and re.fullmatch('[0-9a-f]{64}',ledger['config_sha256']), 'lock-config-fact')
                from grok_acp_bootstrap import PINNED, PREFIX
                require(ledger['bootstrap_digests'] == {PREFIX+name:pin[1] for name,pin in PINNED.items()
                        if name.startswith('docs/user-guide/')}, 'lock-bootstrap-facts')
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


def startup_servers(root):
    """Exact authored witness, not an inventory learned from a native reply."""
    return (dict(name=STARTUP_NAME, source='local', type='stdio', command='/usr/bin/python3',
                 args=['-I', str(root / 'startup-witness.py'), str(root / 'unexpected-startup')]),)


def startup_fixture_config(root):
    """One eligible, unrelated User MCP records any unexpected native startup."""
    path = root / 'home/.grok/config.toml'
    require(not path.exists(), 'startup-config-collision')
    args = startup_servers(root)[0]['args']
    body = ('[marketplace]\ndefault_skills_installs_purged = true\n'
            '[mcp_servers.' + STARTUP_NAME + ']\ncommand = "/usr/bin/python3"\nargs = '
            + json.dumps(args) + '\nenabled = true\n').encode()
    path.write_bytes(body)
    path.chmod(0o600)
    witness = root / 'startup-witness.py'
    witness.write_text("import pathlib,sys\npathlib.Path(sys.argv[1]).write_bytes(b'unexpected')\nraise SystemExit(93)\n")
    witness.chmod(0o600)
    return body


def startup_inventory(row, root, scope):
    require(row['exit'] == 0 and row['error'] is None, 'startup-inventory-exit')
    body = base64.b64decode(row['merged_base64'], validate=True)
    report = document(body)
    require(type(report) is dict and report.get('grokVersion') == '1.0.70'
            and report.get('cwd') == str(root / scope), 'startup-inventory-source')
    servers = report.get('mcpServers')
    require(type(servers) is list and len(servers) == 1, 'startup-inventory-count')
    server = servers[0]
    require(type(server) is dict and server.get('name') == STARTUP_NAME
            and server.get('transport') == 'stdio' and server.get('target') == '/usr/bin/python3'
            and server.get('source') == {'type': 'configToml', 'path': str(root / 'home/.grok/config.toml')}
            and server.get('disabled', False) is False and server.get('disabledReason') is None
            and report.get('mcpConfigProblems', []) == [], 'startup-inventory-enabled')


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


def cli_selected_id(root, native_ids):
    selected = str(root / 'a/.grok/plugins/uap-acp-collision')
    require(native_ids.get(selected) == cli_arguments(root, 'details-full-id')[-1], 'cli-unobserved-id')
    return selected, native_ids[selected]


def cli_request(root, previous, request, budget):
    """Only the original control leaf may change between accepted native families."""
    from devin_structured_inventory_admission import metadata_inventory
    require(metadata_inventory(root, budget) == previous, 'cli-inter-command-change')
    body = json.dumps(request).encode()
    require(len(body) <= 4096, 'cli-request-bound')
    directory = os.open(root, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC)
    descriptor = None
    try:
        info = os.fstat(directory)
        require((info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid) == tuple(previous['.']), 'cli-original-root')
        descriptor = os.open('request.json', os.O_WRONLY | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=directory)
        require(stable(os.fstat(descriptor)) == tuple(previous['request.json']), 'cli-original-request')
        os.ftruncate(descriptor, 0)
        require(os.write(descriptor, body) == len(body), 'cli-request-write')
        after = dict(previous, **{'request.json':stable(os.fstat(descriptor))})
        require(metadata_inventory(root, budget) == after, 'cli-request-only-change')
    finally:
        if descriptor is not None: os.close(descriptor)
        os.close(directory)


def cli_phase_admitted(phase, row, root):
    body = base64.b64decode(row['merged_base64'], validate=True)
    require(row['error'] is None and type(row['exit']) is int, 'cli-command-incomplete')
    if phase == 'details-help':
        usage = re.findall(rb'^Usage: ([^\r\n]+)$', body, re.MULTILINE)
        require(row['exit'] == 0 and len(usage) == 1 and re.fullmatch(
            rb'grok plugin details[ \t]+(?:\[OPTIONS\][ \t]+)?<NAME>', usage[0]), 'cli-details-help')
    if phase == 'list-json':
        require(row['exit'] == 0 and type(document(body, 16 << 10)) is list, 'cli-list-json')
    if phase == 'inspect-project':
        startup_inventory(row, root, 'a')
        value = document(body, 16 << 10)
        rows = value.get('plugins')
        require(value.get('projectRoot') in (str(root/'a'), str(root/'a')+'/')
                and value.get('projectTrusted') is False and type(rows) is list and all(type(r) is dict for r in rows), 'cli-inspect-project')
        selected = [r for r in rows if r.get('name') == 'uap-acp-collision']
        require(len(selected) == 1 and selected[0].get('scope') == 'project'
                and selected[0].get('path') == str(root/'a/.grok/plugins/uap-acp-collision')
                and type(selected[0].get('enabled')) is bool, 'cli-inspect-source')  # Not selector enablement.


def cli_readout(members, summary, packet, facts, deadline):
    require(summary.get('mode') == CLI_MODE and type(summary.get('measurement_complete')) is bool, 'cli-summary')
    if CLI_MEMBER not in members:
        require(not summary['measurement_complete'], 'cli-missing-capture')
        return dict(schema_version=1, mode=CLI_MODE, reason='measurement-only', collection_complete=summary['collection_complete'],
                    measurement_complete=False, cleanup_complete=True, scopes=facts, commands=0)
    record = closed(document(members[CLI_MEMBER]), 'schema_version mode source_commit source_tree nonce selected_root selected_id observations')
    require(type(record['schema_version']) is int and record['schema_version'] == 1 and record['mode'] == CLI_MODE
            and all(record[key] == packet[key] for key in ('source_commit','source_tree','nonce'))
            and len(facts) == 3 and all(r['ledger_accepted'] and r['semantic_complete'] for r in facts), 'cli-prerequisites')
    root = Path('/tmp/uap-grok-acp-input-' + packet['nonce'])
    ids = inventory_rows(dict(rpc=document(members['a.rpc.json'])), root, 'a')
    require((record['selected_root'], record['selected_id']) == cli_selected_id(root, ids), 'cli-observed-source')
    entries = record['observations']
    require(type(entries) is list and 0 < len(entries) <= 5
            and (not summary['measurement_complete'] or len(entries) == 5 and summary['collection_complete'] and summary.get('error') is None), 'cli-observation-count')
    baseline = document(members['b.ledger.json'])
    for index, entry in enumerate(entries):
        require(time.monotonic() < deadline, 'cli-readout-deadline')
        closed(entry, 'receipt ledger')
        result = validate_receipt(entry['receipt'], dict(packet, schema_version=1, scope='a', mode=CLI_MODE, phase=CLI_PHASES[index]))
        ledger = closed(entry['ledger'], 'accepted before after profiles_before profiles_after native_bootstrap')
        require(type(ledger['accepted']) is bool and ledger['before'] == baseline['before']
                and ledger['profiles_before'] == baseline['profiles_before']
                and type(ledger['after']) is dict and len(ledger['after']) <= 4096, 'cli-original-ledger')
        if index + 1 < len(entries) or summary['measurement_complete']:
            require(ledger['accepted'] and result['semantic_complete'], 'cli-unaccepted-prefix')
            cli_phase_admitted(CLI_PHASES[index], result['commands'][0], root)
    return dict(schema_version=1, mode=CLI_MODE, reason='measurement-only', collection_complete=summary['collection_complete'],
                measurement_complete=summary['measurement_complete'], cleanup_complete=True, scopes=facts, commands=len(entries))


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
    allowed = {'admission.json', 'acquisition.json'} | {scope + '.rpc.json' for scope in SCOPES} | {scope + '.ledger.json' for scope in SCOPES} | {scope + '-' + label + '.merged.log' for scope in SCOPES for label in METADATA_LABELS}
    if mode == READONLY_MODE:
        allowed = readonly_members()
    elif mode == LOCK_MODE:
        allowed = {'admission.json','acquisition.json'} | {phase+suffix for phase in LOCK_PHASES for suffix in ('.rpc.json','.ledger.json')} | {phase+'-'+label+'.merged.log' for phase in LOCK_PHASES for label in LOCK_LABELS}
    elif mode == CLI_MODE:
        allowed.add(CLI_MEMBER)
    else:
        require(mode is None, 'publication-mode')
    require(len(bodies) <= (27 if mode == CLI_MODE else 26) and set(bodies) <= allowed and all(len(body) <= LIMIT for body in bodies.values())
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
    if mode in (CLI_MODE, READONLY_MODE):
        refused = dict(schema_version=1, mode=mode, reason='readout-refused')
    try:
        def budget():
            require(type(deadline) is float and time.monotonic() < deadline, 'readout-deadline')
        budget()
        require(type(raw) is bytes and 0 < len(raw) <= 17 << 20, 'readout-archive-bound')
        allowed = {'admission.json', 'acquisition.json'} | {s + suffix for s in SCOPES for suffix in ('.rpc.json', '.ledger.json')} | {s + '-' + label + '.merged.log' for s in SCOPES for label in METADATA_LABELS}
        if mode == READONLY_MODE:
            allowed = readonly_members()
        if mode == LOCK_MODE:
            allowed = {'admission.json','acquisition.json'} | {phase+suffix for phase in LOCK_PHASES for suffix in ('.rpc.json','.ledger.json')} | {phase+'-'+label+'.merged.log' for phase in LOCK_PHASES for label in LOCK_LABELS}
        if mode == CLI_MODE:
            allowed.add(CLI_MEMBER)
        maximum = 27 if mode == CLI_MODE else 26
        require(len(raw) >= 22, 'readout-eocd')
        end = struct.unpack('<4s4H2LH', raw[-22:])
        require(end[0] == b'PK\x05\x06' and end[1] == end[2] == 0
                and end[3] == end[4] and 0 < end[4] <= maximum and end[5] <= 16 << 10
                and end[6] + end[5] == len(raw) - 22 and end[7] == 0, 'readout-central-bound')
        with zipfile.ZipFile(io.BytesIO(raw)) as archive:
            infos = archive.infolist()
            require(len(infos) == end[4], 'readout-central-count')
            require(0 < len(infos) <= maximum and len({i.filename for i in infos}) == len(infos)
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
        if mode == READONLY_MODE:
            return readonly_readout(members, packet, deadline)
        if mode == LOCK_MODE:
            return lock_readout(members, packet, deadline)
        if mode == SESSION_MODE:
            return session_readout(members, packet, deadline)
        require(mode in (None, CLI_MODE), 'readout-mode')
        summary = document(members['admission.json'])
        require(type(summary) is dict and set(summary) <= set(('schema_version collection_complete cleanup_complete qualification scopes source_commit source_tree nonce run attempt error' + (' mode measurement_complete' if mode == CLI_MODE else '')).split())
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
            require(type(commands) is list and len(commands) <= len(METADATA_LABELS), 'readout-commands')
            failed = 0
            for number, row in enumerate(commands):
                budget()
                require(type(row) is dict and row.get('label') == METADATA_LABELS[number]
                        and row.get('merged_file') == scope + '-' + METADATA_LABELS[number] + '.merged.log'
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
        if mode == CLI_MODE:
            return cli_readout(members, summary, packet, facts, deadline)
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


def observation_exit(summary, *, lock_qualification=False):
    if summary.get('mode') == CLI_MODE:
        return 0 if (summary.get('collection_complete') is True and summary.get('measurement_complete') is True
                     and summary.get('cleanup_complete') is True and summary.get('error') is None) else 1
    lock_complete = (lock_qualification is True and summary.get('mode') == LOCK_MODE
                     and summary.get('measurement_complete') is True
                     and summary.get('cleanup_complete') is True
                     and summary.get('error') is None)
    return 0 if summary['collection_complete'] or lock_complete else 1


def retain_frontend_failure(parent, record, stdout, stderr, commit, tree, deadline, *, mode=None):
    """Retain bounded outer transport bytes; this never admits a native family."""
    import tempfile
    from native_client_project_artifacts import write_zip
    require(type(deadline) is float and time.monotonic() < deadline, 'frontend-deadline')
    labels = {None:'grok-acp-frontend', SESSION_MODE:'grok-session-frontend', LOCK_MODE:'grok-lock-frontend', CLI_MODE:'grok-acp-frontend', READONLY_MODE:'grok-readonly-frontend'}
    require((mode is None or type(mode) is str and mode in labels)
            and all(type(value) is str and re.fullmatch('[0-9a-f]{40}', value)
                                  for value in (commit, tree)), 'frontend-source')
    closed(record, 'probe argv cwd error exit elapsed_seconds stdout_file stderr_file stdout_bytes stderr_bytes stdout_sha256 stderr_sha256')
    require(record['probe'] == labels[mode] and record['error'] is None
            and type(record['exit']) is int and record['exit'] in (0, 1), 'frontend-incomplete')
    for name, body in (('stdout', stdout), ('stderr', stderr)):
        require(type(body) is bytes and len(body) <= LIMIT and type(record[name+'_bytes']) is int
                and record[name+'_bytes'] == len(body) and record[name+'_sha256'] == digest(body), 'frontend-stream')
    encoded = json.dumps(record, sort_keys=True, allow_nan=False).encode()
    require(len(encoded) <= 16384, 'frontend-record-bound')
    capture = dict(schema_version=1, kind='grok_frontend_failure_capture', source_commit=commit, source_tree=tree,
                   run=os.environ.get('GITHUB_RUN_ID'), attempt=os.environ.get('GITHUB_RUN_ATTEMPT'), mode=mode,
                   collection_complete=False, native_cleanup_proven=False, qualification=dict(FALSE), record=record)
    bodies = {'frontend.json':json.dumps(capture, sort_keys=True, allow_nan=False).encode(),
              'frontend.stdout.bin':stdout, 'frontend.stderr.bin':stderr}
    parent = Path(parent)
    require(parent.resolve() == parent, 'frontend-parent')
    held = os.open(parent, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC)
    directory, archive = None, None
    try:
        original = directory_id(os.fstat(held))
        require(original == directory_id(parent.lstat()), 'frontend-parent')
        name = Path(tempfile.mkdtemp(prefix=labels[mode]+'-failure-', dir='/proc/self/fd/'+str(held))).name
        output = parent / name
        directory = os.open(name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=held)
        identity = directory_id(os.fstat(directory))
        require(original == directory_id(parent.lstat()) and identity == directory_id(output.lstat())
                and identity['mode'] == stat.S_IFDIR | 0o700,
                'frontend-output')
        require(time.monotonic() < deadline, 'frontend-deadline')
        write_zip(Path('/proc/self/fd/' + str(directory)) / 'research.zip', bodies)
        archive, before, raw = capture_archive(directory)
        require(original == directory_id(parent.lstat()) == directory_id(os.fstat(held))
                and identity == directory_id(output.lstat()) == directory_id(os.fstat(directory)), 'frontend-replaced')
        archive_unchanged(directory, archive, before)
        require(time.monotonic() < deadline, 'frontend-deadline')
        return output / 'research.zip', dict(bytes=len(raw), sha256=digest(raw))
    finally:
        for descriptor in (archive, directory, held):
            if descriptor is not None: os.close(descriptor)


def run(output, *, session_only=False, lock_qualification=False, plugin_cli=False, _frontend=None):
    progress = frontend_state() if _frontend is None else _frontend
    import project_native_research as shared
    from grok_acp_bootstrap import NativeBootstrap
    from project_native_collectors.grok import ACQUISITION
    from devin_structured_inventory_admission import metadata_inventory
    from client_automation_native_probe import snapshot
    shared.native.require_hosted('linux-amd64')
    require(type(plugin_cli) is bool and not (session_only and lock_qualification)
            and (not plugin_cli or session_only is False and lock_qualification is False), 'mode')
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
    if plugin_cli:
        summary.update(mode=CLI_MODE, measurement_complete=False)
    bodies, cleanup_proved = {}, True
    try:
        with shared.Budget(150) as budget:
            progress['stage'] = 'source'
            def source(args):
                value = subprocess.run(['/usr/bin/git', *args], cwd=shared.SOURCE, capture_output=True, timeout=5, check=True,
                                       env={'PATH': '/usr/bin:/bin', 'GIT_CONFIG_NOSYSTEM': '1', 'GIT_CONFIG_GLOBAL': '/dev/null'})
                require(not value.stderr and len(value.stdout) <= 4096, 'source-bound')
                return value.stdout.decode().strip()
            commit, tree = source(['rev-parse', 'HEAD']), source(['rev-parse', 'HEAD^{tree}'])
            require(commit == os.environ.get('EXPECTED_COMMIT') and re.fullmatch('[0-9a-f]{40}', commit)
                    and re.fullmatch('[0-9a-f]{40}', tree) and not source(['status', '--porcelain', '--untracked-files=normal']), 'source-identity')
            summary.update(source_commit=commit, source_tree=tree, nonce=nonce, run=os.environ.get('GITHUB_RUN_ID'), attempt=os.environ.get('GITHUB_RUN_ATTEMPT'))
            progress['source_identity_checked'] = True
            progress['stage'] = 'acquisition'
            receipt, contents = shared.acquire(ACQUISITION, budget)
            require(digest(contents['grok']) == ACQUISITION['binary_sha256'], 'native-pin')
            (root / 'grok').write_bytes(contents['grok'])
            (root / 'grok').chmod(0o700)
            collector = Path(__file__).resolve()
            (root / 'collector.py').write_bytes(ordinary(collector))
            helper = root / 'helper'
            progress['stage'] = 'build'
            subprocess.run(['go', 'build', '-trimpath', '-ldflags', '-X main.sourceCommit=' + commit + ' -X main.sourceTree=' + tree
                            + ' -X main.collectorSHA=' + digest(ordinary(collector)), '-o', str(helper), './research/grokacpinventory'],
                           cwd=shared.SOURCE / 'install/integrationctl', check=True, timeout=60)
            progress['stage'] = 'fixtures'
            roots = fixtures(root, nonce)
            startup_config = startup_fixture_config(root) if not session_only and not lock_qualification else None
            if session_only:
                session_fixture_config(root, nonce)
                summary['mode'] = SESSION_MODE
            if lock_qualification:
                authored = lock_fixture_config(root)
                summary['mode'] = LOCK_MODE
            (root / 'request.json').write_bytes(b'{}\n')
            progress['stage'] = 'baseline'
            baseline = None if session_only or lock_qualification else metadata_inventory(root, budget)
            profiles = {name: snapshot(path) for name, path in roots.items()}
            bootstrap = NativeBootstrap(original_config=startup_config)
            bodies['acquisition.json'] = json.dumps(receipt, sort_keys=True).encode()
            native_ids, lock_created = {}, None
            observations = None
            phases = LOCK_PHASES if lock_qualification else (('a',) if session_only else SCOPES + (CLI_PHASES if plugin_cli else ()))
            for stage in phases:
                cli = plugin_cli and stage in CLI_PHASES
                scope = 'a' if lock_qualification or cli else stage
                prefix = stage if lock_qualification else scope
                require(time.monotonic() < origin + 150, 'outer-deadline')
                if cli:
                    require(len(summary['scopes']) == 3 and all(r['result']['semantic_complete'] for r in summary['scopes']), 'cli-acp-prerequisite')
                    selected_root, selected_id = cli_selected_id(root, native_ids)
                    require(ledger['accepted'] and metadata_inventory(root, budget) == ledger['after'], 'cli-inter-command-change')
                    if observations is None:
                        observations = dict(schema_version=1, mode=CLI_MODE, source_commit=commit, source_tree=tree, nonce=nonce,
                                            selected_root=selected_root, selected_id=selected_id, observations=[])
                request = dict(schema_version=1, source_commit=commit, source_tree=tree, uid=os.getuid(), gid=os.getgid(), nonce=nonce,
                               scope=scope, start_monotonic_ns=time.monotonic_ns(), helper_sha256=digest(ordinary(helper, 64 << 20, expected=ledger['after']['helper'] if cli else None)))
                if session_only:
                    request['mode'] = SESSION_MODE
                if lock_qualification:
                    request.update(mode=LOCK_MODE,phase=stage)
                if cli:
                    request.update(mode=CLI_MODE, phase=stage)
                    cli_request(root, ledger['after'], request, budget)
                else:
                    (root / 'request.json').write_text(json.dumps(request))
                if session_only or lock_qualification and baseline is None:
                    # The one authored control request is fixed before Baseline0.
                    require(baseline is None, 'session-baseline-renewal')
                    baseline = metadata_inventory(root, budget)
                cleanup_proved = False
                progress['stage'] = 'helper'
                receipt = helper_capture(helper, root, min(origin + 150, request['start_monotonic_ns'] / 1e9 + 35))
                progress['stage'] = 'receipt'
                result = validate_receipt(receipt, request)
                cleanup_proved = True
                progress['stage'] = 'ledger'
                ledger = lock_ledger(root,baseline,stage,authored,budget,created=lock_created) if lock_qualification else session_metadata_ledger(root, baseline, budget) if session_only else post_ledger(root, baseline, roots, profiles, budget, bootstrap)
                if cli:
                    observations['observations'].append(dict(receipt=receipt, ledger=ledger))
                    encoded = json.dumps(observations, sort_keys=True).encode()
                    require(len(encoded) <= LIMIT, 'cli-observation-bound')
                    bodies[CLI_MEMBER] = encoded
                    try:
                        require(ledger['accepted'] and result['semantic_complete'], 'cli-native-refused')
                        cli_phase_admitted(stage, result['commands'][0], root)
                    except ValueError:
                        summary['error'] = 'admission-refused'
                        break
                    continue
                if session_only:
                    ledger['selected_config_capture'] = capture_session_config(root, ledger, profiles['user_profile']['config.toml'], nonce, budget, receipt)
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
                    if stage == LOCK_PHASES[0]:
                        lock_created = {name:value for name,value in ledger['after'].items() if name not in baseline}
                    continue
                if session_only:
                    summary['measurement_complete'] = True
                    result.pop('rpc')
                    require(ledger['protected_originals_unchanged'] and result['semantic_complete'], 'native-refused')
                    continue  # Unknown session effects never become an admitted envelope.
                require(ledger['accepted'] and result['semantic_complete'], 'native-refused')
                progress['stage'] = 'inventory'
                observed_ids = inventory_rows(result, root, scope)
                require(all(path not in native_ids or native_ids[path] == value for path, value in observed_ids.items()), 'native-id-changed')
                native_ids.update(observed_ids)
                result.pop('rpc')
            progress['stage'] = 'source-final'
            require(not source(['status', '--porcelain', '--untracked-files=normal']), 'source-changed')
            if plugin_cli:
                require(metadata_inventory(root, budget) == ledger['after'], 'cli-final-change')
            summary['collection_complete'] = not session_only and not lock_qualification
            if plugin_cli:
                summary['measurement_complete'] = bool(observations and len(observations['observations']) == 5 and not summary.get('error'))
            if lock_qualification:
                summary['measurement_complete'] = True
    except Exception as error:
        frontend_error(progress, error)
        summary['error'] = 'admission-refused'
    finally:
        progress['stage'] = 'cleanup'
        if cleanup_proved and stable(root.lstat())[:2] == held:
            shutil.rmtree(root)
            summary['cleanup_complete'] = True
        progress['cleanup_complete'] = summary['cleanup_complete']
        if not summary['cleanup_complete']:
            if progress['failure_stage'] is None:
                progress['failure_stage'] = 'cleanup'
            summary = dict(schema_version=1, collection_complete=False, cleanup_complete=False, qualification=dict(FALSE), scopes=[])
            bodies = {}
        bodies['admission.json'] = json.dumps(summary, sort_keys=True).encode() + b'\n'
        try:
            progress['stage'] = 'archive'
            progress['publication_attempted'] = True
            publication = publish(output, bodies, output_fd, output_identity, mode=LOCK_MODE if lock_qualification else CLI_MODE) if lock_qualification or plugin_cli else publish(output, bodies, output_fd, output_identity)
        finally:
            os.close(output_fd)
        progress['stage'] = 'receipt-output'
        if summary['cleanup_complete'] and all(key in summary for key in ('source_commit', 'source_tree', 'nonce')):
            print(json.dumps(dict(schema_version=1, source_commit=summary['source_commit'], source_tree=summary['source_tree'],
                                  nonce=summary['nonce'], publication=publication), separators=(',', ':')))
            progress['receipt_emitted'] = True
    return observation_exit(summary, lock_qualification=lock_qualification)


READONLY_MODE = 'readonly-inventory-v1'
READONLY_LABELS = ('readonly-control', 'acp')
READONLY_NAMESPACES = ('user', 'pid', 'net', 'ipc', 'uts', 'mnt')
READONLY_DEVICES = ('null', 'zero', 'full', 'random', 'urandom', 'tty')
READONLY_ROOTS = ('.', 'home', 'home/.grok', 'user', 'a', 'b', 'tmp',
                  'xdg-config', 'xdg-data', 'xdg-cache', 'xdg-state', 'readonly-witness')
READONLY_FILES = ('home/.grok/config.toml', 'startup-witness.py')
READONLY_RUNTIME_ROOTS = ('/usr', '/bin', '/lib', '/lib64')
READONLY_HEADER = b'GROK_READONLY_AUTHORITY='
READONLY_RUNTIME_COUNT = 262144


def readonly_deadline(req, *, final=False):
    start = req['start_monotonic_ns']
    require(type(start) is int and start > 0, 'readonly-clock')
    now, deadline = time.monotonic_ns(), start + (30 if final else 25) * 1000000000
    require(start <= now < deadline, 'readonly-deadline')
    return deadline / 1e9


def readonly_fixed_proc(path, limit):
    allowed = {'/proc/self/status':65536, '/proc/self/mountinfo':262144,
               '/proc/self/uid_map':128, '/proc/self/gid_map':128, '/proc/self/setgroups':16,
               '/proc/sys/kernel/overflowuid':32, '/proc/sys/kernel/overflowgid':32}
    require(path in allowed and limit == allowed[path], 'readonly-proc-source')
    descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC)
    with os.fdopen(descriptor, 'rb') as stream:
        body = stream.read(limit + 1)
    require(0 < len(body) <= limit, 'readonly-proc-bound')
    return body


def readonly_id_maps(req, runtime):
    """Interpret only the two maps produced by this fixed unprivileged profile."""
    values = {}
    for kind in ('uid', 'gid'):
        raw = readonly_fixed_proc('/proc/self/' + kind + '_map', 128)
        require(re.fullmatch(rb'[ \t]*[0-9]+[ \t]+0[ \t]+1[ \t]*\n', raw), 'readonly-id-map-shape')
        inside, parent, count = map(int, raw.split())
        require(inside == req[kind] > 0 and parent == 0 and count == 1, 'readonly-id-map')
        overflow = readonly_fixed_proc('/proc/sys/kernel/overflow' + kind, 32)
        require(re.fullmatch(rb'[1-9][0-9]{0,9}\n', overflow), 'readonly-overflow-shape')
        number = int(overflow)
        require(number == runtime['overflow_' + kind] and number != req[kind] and number < 2**32,
                'readonly-overflow-identity')
        values[kind] = dict(inside=inside, parent=parent, count=count, overflow=number, sha256=digest(raw))
    require(readonly_fixed_proc('/proc/self/setgroups', 16) == b'deny\n', 'readonly-groups-policy')
    return values


def readonly_host_owner(identity, maps, *, file=False, device=False):
    """Translate a known host-owner fact; never recover ownership from overflow."""
    value = list(identity)
    indexes = (4, 5) if file else (3, 4)
    for kind, index in zip(('uid', 'gid'), indexes):
        original = value[index]
        require(type(original) is int and (original in (0, maps[kind]['inside']) or device and kind == 'gid' and 0 <= original < 2**32), 'readonly-original-owner')
        value[index] = original if original == maps[kind]['inside'] else maps[kind]['overflow']
    return tuple(value)


def readonly_environment(root):
    env = {'PATH':'/usr/bin:/bin', 'LANG':'C.UTF-8', 'LC_ALL':'C.UTF-8',
           'HOME':str(root / 'home'), 'TMPDIR':str(root / 'tmp'), 'TERM':'dumb',
           'NO_COLOR':'1', 'GROK_DISABLE_AUTOUPDATER':'1', 'GIT_CONFIG_NOSYSTEM':'1',
           'GIT_CONFIG_GLOBAL':'/dev/null', 'GIT_TERMINAL_PROMPT':'0', 'PWD':str(root / 'a')}
    env.update({'XDG_' + name.upper() + '_HOME':str(root / ('xdg-' + name))
                for name in ('config', 'data', 'cache', 'state')})
    return env


def readonly_bwrap_argv(req, phase, original):
    require(req.get('mode') == READONLY_MODE and req['scope'] == 'a'
            and phase in ('control', 'native'), 'readonly-child-mode')
    root = Path(req['input_root'])
    sealed = Path(req['readonly']['roots']['sealed']['path'])
    facts = json.dumps(original, separators=(',', ':'), sort_keys=True)
    require(len(facts.encode()) <= 512, 'readonly-parent-proof-bound')
    argv = ['/usr/bin/bwrap', '--unshare-user', '--uid', str(req['uid']),
            '--gid', str(req['gid']), '--unshare-pid', '--unshare-net', '--unshare-ipc',
            '--unshare-uts', '--disable-userns', '--die-with-parent', '--new-session',
            '--cap-drop', 'ALL', '--tmpfs', '/', '--ro-bind', '/usr', '/usr']
    for path in READONLY_RUNTIME_ROOTS[1:]:
        argv += ['--ro-bind-try', path, path]
    argv += ['--ro-bind', str(root), str(root), '--ro-bind', str(sealed), str(sealed),
             '--bind', str(root / 'readonly-witness'), str(root / 'readonly-witness'),
             '--proc', '/proc', '--remount-ro', '/proc', '--dev', '/dev']
    for name in READONLY_DEVICES:
        argv += ['--remount-ro', '/dev/' + name]
    argv += ['--remount-ro', '/dev/pts', '--remount-ro', '/dev', '--remount-ro', '/',
             '--chdir', str(root / 'a'), '--', '/usr/bin/python3.12', '-I', '-B',
             str(sealed / 'collector.py'), '--readonly-inner',
             str(sealed / 'collector-request.json'), phase, facts]
    return argv


def readonly_mount_path(raw):
    require(type(raw) is bytes and 0 < len(raw) <= 4096 and raw.startswith(b'/'), 'readonly-mount-path')
    replacements = {b'040':b' ', b'011':b'\t', b'012':b'\n', b'134':b'\\'}
    def replace(match):
        require(match[1] in replacements, 'readonly-mount-escape')
        return replacements[match[1]]
    value = re.sub(rb'\\([0-9]{3})', replace, raw)
    require(b'\\' not in raw.replace(b'\\040', b'').replace(b'\\011', b'')
            .replace(b'\\012', b'').replace(b'\\134', b''), 'readonly-mount-escape')
    path = value.decode('utf-8', 'strict')
    require(os.path.normpath(path) == path and '\0' not in path, 'readonly-mount-canonical')
    return path


def readonly_mounts(body, req):
    require(type(body) is bytes and 0 < len(body) <= 262144, 'readonly-mount-bound')
    root, runtime = req['input_root'], req['readonly']
    witness, sealed = root + '/readonly-witness', runtime['roots']['sealed']['path']
    required = {'/', '/usr', root, sealed, witness, '/proc', '/dev', '/dev/pts'}
    required.update('/dev/' + name for name in READONLY_DEVICES)
    required.update(runtime['runtime'])
    optional = {'/proc/sys', '/proc/sysrq-trigger', '/proc/irq', '/proc/bus'}
    rows, seen_ids = {}, set()
    for raw in body.splitlines():
        require(len(rows) < 128 and len(raw) <= 8192 and raw.count(b' - ') == 1, 'readonly-mount-row')
        left, right = raw.split(b' - ')
        fields, filesystem = left.split(), right.split()
        require(len(fields) >= 6 and len(filesystem) == 3
                and fields[0].isdigit() and fields[1].isdigit()
                and re.fullmatch(rb'[0-9]+:[0-9]+', fields[2]), 'readonly-mount-fields')
        mount, parent = int(fields[0]), int(fields[1])
        target = readonly_mount_path(fields[4])
        require(mount > 0 and parent > 0 and mount not in seen_ids and target not in rows
                and target in required | optional, 'readonly-mount-membership')
        options = fields[5].split(b',')
        require(len(options) == len(set(options)) and (b'rw' in options) is (target == witness)
                and (b'ro' in options) is (target != witness), 'readonly-mount-writable')
        if target == '/proc':
            require(filesystem[0] == b'proc', 'readonly-proc-mount')
        if target == '/dev/pts':
            require(filesystem[0] == b'devpts', 'readonly-devpts-mount')
        if target in ('/', '/dev'):
            require(filesystem[0] == b'tmpfs', 'readonly-private-mount')
        rows[target] = dict(id=mount, parent=parent, readonly=target != witness,
                            filesystem=filesystem[0].decode('ascii'))
        seen_ids.add(mount)
    require(required <= rows.keys(), 'readonly-mount-missing')
    return dict(rows=rows, sha256=digest(body))



def readonly_metadata(info, *, directory=False):
    return (info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid) if directory else stable(info)


def readonly_directory(path, expected, maps=None):
    """Join the selected original mount object before opening descendants."""
    expected = tuple(expected) if maps is None else readonly_host_owner(expected, maps)
    before = path.lstat()
    require(readonly_metadata(before, directory=True) == expected and stat.S_ISDIR(before.st_mode),
            'readonly-directory-original')
    descriptor = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC)
    try:
        require(readonly_metadata(os.fstat(descriptor), directory=True)
                == readonly_metadata(path.lstat(), directory=True) == expected, 'readonly-directory-open')
        return descriptor
    except BaseException:
        os.close(descriptor)
        raise


def readonly_walk(descriptor, req, limit, *, runtime=False, maps=None):
    """Only descriptor-relative metadata; every edge is checked before descent."""
    rows, aliases = {}, {}
    def visit(parent, prefix, depth):
        readonly_deadline(req)
        require(depth <= 32, 'readonly-census-depth')
        before = os.fstat(parent)
        require(stat.S_ISDIR(before.st_mode), 'readonly-census-directory')
        row = readonly_metadata(before, directory=not runtime)
        if maps is not None and runtime:
            require(row[4] == maps['uid']['overflow'] and row[5] == maps['gid']['overflow'], 'readonly-runtime-owner')
            row = row[:4] + (0, 0) + row[6:]
        rows[prefix] = row
        require(len(rows) <= limit, 'readonly-census-count')
        names = []
        with os.scandir(parent) as entries:
            for entry in entries:
                readonly_deadline(req)
                require(len(rows) + len(names) < limit and entry.name not in ('.', '..')
                        and '\0' not in entry.name and '\n' not in entry.name and '\r' not in entry.name,
                        'readonly-census-count')
                names.append(entry.name)
        for name in sorted(names):
            readonly_deadline(req)
            relative = name if prefix == '.' else prefix + '/' + name
            require(len(relative.encode()) <= 4096, 'readonly-census-path')
            info = os.stat(name, dir_fd=parent, follow_symlinks=False)
            require(info.st_mode & 0o022 == 0 or stat.S_ISLNK(info.st_mode), 'readonly-census-writable')
            if runtime:
                uid = maps['uid']['overflow'] if maps else 0
                gid = maps['gid']['overflow'] if maps else 0
                require(info.st_uid == uid and info.st_gid == gid, 'readonly-runtime-owner')
            else:
                require(info.st_uid == req['uid'] and info.st_gid == req['gid'], 'readonly-input-owner')
            if stat.S_ISDIR(info.st_mode):
                child = os.open(name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=parent)
                try:
                    require(stable(os.fstat(child)) == stable(info), 'readonly-census-open')
                    visit(child, relative, depth + 1)
                    require(stable(os.fstat(child)) == stable(info)
                            == stable(os.stat(name, dir_fd=parent, follow_symlinks=False)), 'readonly-census-rejoin')
                finally:
                    os.close(child)
            elif stat.S_ISREG(info.st_mode):
                require(info.st_nlink > 0 and (runtime or info.st_nlink == 1 and 0 <= info.st_size <= 512 << 20),
                        'readonly-census-alias')
                value = stable(info)
                if maps is not None and runtime:
                    value = value[:4] + (0, 0) + value[6:]
                rows[relative] = value
                aliases.setdefault((info.st_dev, info.st_ino), []).append(info.st_nlink)
                require(stable(info) == stable(os.stat(name, dir_fd=parent, follow_symlinks=False)),
                        'readonly-census-file-rejoin')
            elif runtime and stat.S_ISLNK(info.st_mode):
                require(info.st_nlink == 1, 'readonly-runtime-link-alias')
                target = os.readlink(name, dir_fd=parent)
                require(0 < len(target.encode()) <= 4096 and '\0' not in target, 'readonly-runtime-link-bound')
                require(stable(info) == stable(os.stat(name, dir_fd=parent, follow_symlinks=False)),
                        'readonly-runtime-link-rejoin')
                value = stable(info)
                if maps is not None:
                    value = value[:4] + (0, 0) + value[6:]
                rows[relative] = (*value, target)
            else:
                raise ValueError('readonly-special-member')
            require(len(rows) <= limit, 'readonly-census-count')
        require(stable(before) == stable(os.fstat(parent)), 'readonly-census-directory-rejoin')
    visit(descriptor, '.', 0)
    for counts in aliases.values():
        require(all(count == len(counts) for count in counts), 'readonly-external-hardlink')
    return rows


def readonly_ledger_digest(rows):
    hasher = hashlib.sha256()
    for name in sorted(rows):
        values = rows[name]
        require(type(name) is str and os.path.normpath(name) == name and not name.startswith('/')
                and all(part != '..' for part in name.split('/')) and len(name.encode()) <= 4096
                and name.count('/') <= 32 and not any(char in name for char in ('\0', '\r', '\n'))
                and type(values) in (tuple, list) and len(values) in (5, 9)
                and all(type(value) is int and value >= 0 for value in values), 'readonly-ledger-types')
        require(values[0] > 0 and values[1] > 0 and values[2] & 0o022 == 0
                and ((len(values) == 5 and stat.S_ISDIR(values[2]))
                     or (len(values) == 9 and stat.S_ISREG(values[2]) and values[3] == 1
                         and values[6] <= 512 << 20)), 'readonly-ledger-record')
        hasher.update(name.encode() + b'\0' + ','.join(map(str, values)).encode() + b'\n')
    return dict(sha256=hasher.hexdigest(), count=len(rows))


def readonly_input(req, maps=None):
    root, grant = Path(req['input_root']), req['readonly']
    descriptor = readonly_directory(root, grant['roots']['.']['identity'], maps)
    try:
        rows = readonly_walk(descriptor, req, 4096, maps=maps)
        result = readonly_ledger_digest(rows)
        require(result == dict(sha256=grant['input_sha256'], count=grant['input_count']), 'readonly-original-input')
        require(readonly_metadata(os.fstat(descriptor), directory=True)
                == readonly_metadata(root.lstat(), directory=True), 'readonly-input-final')
        return result
    finally:
        os.close(descriptor)


def readonly_visible_target(path, grant):
    """Map only the four already admitted visible runtime roots."""
    path = os.path.normpath(path)
    require(path.startswith('/') and '\0' not in path and len(path.encode()) <= 4096, 'readonly-link-target')
    for root in sorted(READONLY_RUNTIME_ROOTS, key=len, reverse=True):
        if path == root or path.startswith(root + '/'):
            return grant['runtime'][root]['path'] + path[len(root):]
    require(path != '/' and not any(path == root or path.startswith(root + '/')
            for root in ('/tmp', '/proc', '/dev')), 'readonly-link-cross-domain')
    return None


def readonly_link_target(root, relative, target, rows, req):
    # The entire namespace is already closed; resolution uses its captured
    # no-follow records and never stats a hidden target.
    grant = req['readonly']
    path = readonly_visible_target(os.path.join('/usr', os.path.dirname(relative), target), grant)
    if path is None:
        return 'hidden'
    for _ in range(16):
        readonly_deadline(req)
        require(path == '/usr' or path.startswith('/usr/'), 'readonly-link-domain')
        parts = [] if path == '/usr' else path[len('/usr/'):].split('/')
        changed = False
        for index in range(len(parts)):
            current = '/'.join(parts[:index + 1])
            if current not in rows:
                return 'absent'
            value = rows[current]
            if stat.S_ISLNK(value[2]):
                suffix = parts[index + 1:]
                path = readonly_visible_target(os.path.join('/usr', os.path.dirname(current), value[-1], *suffix), grant)
                if path is None:
                    return 'hidden'
                changed = True
                break
            require(index == len(parts) - 1 or stat.S_ISDIR(value[2]), 'readonly-link-parent')
        if not changed:
            return 'visible'
    raise ValueError('readonly-link-depth')


def readonly_runtime(req, maps=None):
    grant, roots = req['readonly'], {}
    for name, fact in grant['runtime'].items():
        readonly_deadline(req)
        path = Path(name if maps else fact['path'])
        expected = fact['identity']
        descriptor = readonly_directory(path, expected, maps)
        try:
            roots[name] = tuple(expected)
        finally:
            os.close(descriptor)
        if maps is None and name != fact['path']:
            alias = Path(name).lstat()
            require(stat.S_ISLNK(alias.st_mode) and os.path.normpath(os.path.join('/', os.readlink(name))) == fact['path'],
                    'readonly-runtime-top-alias')
    descriptor = readonly_directory(Path('/usr'), grant['runtime']['/usr']['identity'], maps)
    try:
        rows = readonly_walk(descriptor, req, READONLY_RUNTIME_COUNT, runtime=True, maps=maps)
        hasher = hashlib.sha256()
        for name in sorted(rows):
            readonly_deadline(req)
            values = rows[name]
            classification = readonly_link_target(descriptor, name, values[-1], rows, req) if stat.S_ISLNK(values[2]) else None
            hasher.update(json.dumps([name, values, classification], ensure_ascii=True, separators=(',', ':')).encode() + b'\n')
        require(readonly_metadata(os.fstat(descriptor), directory=True)
                == readonly_metadata(Path('/usr').lstat(), directory=True), 'readonly-runtime-final')
        return dict(sha256=hasher.hexdigest(), count=len(rows))
    finally:
        os.close(descriptor)


def readonly_file_parent(req, path, maps):
    grant, known = req['readonly'], {}
    for fact in list(grant['roots'].values()) + list(grant['runtime'].values()):
        known[fact['path']] = fact['identity']
    parent = Path(path).parent
    chain = []
    while str(parent) in known:
        chain.append(parent)
        parent = parent.parent
    require(chain, 'readonly-file-parent')
    chain.reverse()
    descriptors = []
    try:
        for index, item in enumerate(chain):
            expected = tuple(known[str(item)]) if maps is None else readonly_host_owner(known[str(item)], maps)
            if index == 0:
                descriptor = readonly_directory(item, known[str(item)], maps)
            else:
                descriptor = os.open(item.name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC,
                                     dir_fd=descriptors[-1][0])
            descriptors.append((descriptor, item, expected))
            require(readonly_metadata(os.fstat(descriptor), directory=True)
                    == readonly_metadata(item.lstat(), directory=True) == expected, 'readonly-file-parent-join')
        return descriptors
    except BaseException:
        for descriptor, _, _ in reversed(descriptors):
            os.close(descriptor)
        raise


def readonly_verify_files(req, maps=None):
    grant = req['readonly']
    for path, fact in grant['files'].items():
        readonly_deadline(req)
        closed(fact, 'path identity sha256')
        require(path == fact['path'] and type(fact['identity']) is list and len(fact['identity']) == 9,
                'readonly-file-fact')
        parents = readonly_file_parent(req, path, maps)
        fd = None
        try:
            parent = parents[-1][0]
            before = os.stat(Path(path).name, dir_fd=parent, follow_symlinks=False)
            expected = tuple(fact['identity']) if maps is None else readonly_host_owner(fact['identity'], maps, file=True)
            require(stable(before) == expected and stat.S_ISREG(before.st_mode) and before.st_nlink == 1
                    and before.st_mode & 0o6022 == 0 and 0 <= before.st_size <= 256 << 20, 'readonly-file-original')
            fd = os.open(Path(path).name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC, dir_fd=parent)
            require(stable(os.fstat(fd)) == expected, 'readonly-file-open')
            for descriptor, named, original in parents:
                require(readonly_metadata(os.fstat(descriptor), directory=True)
                        == readonly_metadata(named.lstat(), directory=True) == original, 'readonly-file-parent-before-read')
            hasher, total = hashlib.sha256(), 0
            while True:
                readonly_deadline(req)
                block = os.read(fd, 1 << 20)
                if not block:
                    break
                total += len(block)
                require(total <= before.st_size, 'readonly-file-bound')
                hasher.update(block)
            require(total == before.st_size and hasher.hexdigest() == fact['sha256']
                    and stable(os.fstat(fd)) == stable(os.stat(Path(path).name, dir_fd=parent, follow_symlinks=False)) == expected,
                    'readonly-file-final')
            for descriptor, named, original in parents:
                require(readonly_metadata(os.fstat(descriptor), directory=True)
                        == readonly_metadata(named.lstat(), directory=True) == original, 'readonly-file-parent-after-read')
        finally:
            if fd is not None:
                os.close(fd)
            for descriptor, _, _ in reversed(parents):
                os.close(descriptor)


def readonly_request(req, *, check_clock=True):
    closed(req, 'schema_version source_commit source_tree uid gid nonce scope input_root parent_network_namespace start_monotonic_ns mode readonly')
    require(type(req['schema_version']) is int and req['schema_version'] == 1 and req['mode'] == READONLY_MODE
            and req['scope'] == 'a' and re.fullmatch('[0-9a-f]{32}', req['nonce'])
            and req['input_root'] == '/tmp/uap-grok-acp-input-' + req['nonce']
            and all(type(req[key]) is int and 0 < req[key] < 2**30 for key in ('uid', 'gid'))
            and all(type(req[key]) is str and re.fullmatch('[0-9a-f]{40}', req[key]) for key in ('source_commit', 'source_tree')),
            'readonly-request')
    grant = closed(req['readonly'], 'roots files runtime devices namespaces overflow_uid overflow_gid input_sha256 input_count')
    closed(grant['roots'], ' '.join(READONLY_ROOTS) + ' sealed')
    closed(grant['runtime'], ' '.join(READONLY_RUNTIME_ROOTS))
    closed(grant['devices'], ' '.join(READONLY_DEVICES))
    closed(grant['namespaces'], ' '.join(READONLY_NAMESPACES))
    sealed = grant['roots']['sealed']['path']
    require(type(sealed) is str and re.fullmatch(r'/tmp/uap-grok-acp-sealed-[A-Za-z0-9]+', sealed), 'readonly-sealed-root')
    files = (req['input_root'] + '/' + name for name in READONLY_FILES)
    require(set(grant['files']) == {*files, sealed + '/grok', sealed + '/collector.py', '/usr/bin/bwrap', '/usr/bin/python3.12'},
            'readonly-files')
    for name, fact in grant['roots'].items():
        closed(fact, 'path identity')
        expected_path = sealed if name == 'sealed' else str(Path(req['input_root']) / name)
        require(fact['path'] == expected_path and type(fact['identity']) is list and len(fact['identity']) == 5
                and all(type(value) is int and value >= 0 for value in fact['identity'])
                and stat.S_ISDIR(fact['identity'][2]) and fact['identity'][2] & 0o6022 == 0
                and fact['identity'][3:] == ([0, 0] if name == 'sealed' else [req['uid'], req['gid']])
                and (name != 'sealed' or fact['identity'][2] == stat.S_IFDIR | 0o755)
                and (name not in ('.', 'readonly-witness') or fact['identity'][2] == stat.S_IFDIR | 0o700),
                'readonly-root-fact')
    for name, fact in grant['runtime'].items():
        closed(fact, 'path identity')
        require(type(fact['path']) is str and os.path.normpath(fact['path']) == fact['path']
                and (fact['path'] == '/usr' if name == '/usr' else fact['path'].startswith('/usr/')),
                'readonly-runtime-domain')
        require(type(fact['identity']) is list and len(fact['identity']) == 5
                and all(type(value) is int and value >= 0 for value in fact['identity'])
                and stat.S_ISDIR(fact['identity'][2]) and fact['identity'][2] & 0o6022 == 0
                and fact['identity'][3:] == [0, 0], 'readonly-runtime-root-owner')
    require(all(type(value) is int and value > 0 for value in grant['namespaces'].values())
            and grant['namespaces']['net'] == req['parent_network_namespace']
            and type(grant['input_count']) is int and 0 < grant['input_count'] <= 4096
            and re.fullmatch('[0-9a-f]{64}', grant['input_sha256']), 'readonly-grant')
    for path, fact in grant['files'].items():
        closed(fact, 'path identity sha256')
        values = fact['identity']
        owner = req['uid'] if path.startswith(req['input_root'] + '/') else 0
        group = req['gid'] if owner == req['uid'] else 0
        require(fact['path'] == path and type(values) is list and len(values) == 9
                and all(type(number) is int and number >= 0 for number in values)
                and values[0] > 0 and values[1] > 0 and stat.S_ISREG(values[2])
                and values[2] & 0o6022 == 0 and values[3] == 1 and values[4:6] == [owner, group]
                and values[6] <= 256 << 20 and type(fact['sha256']) is str
                and re.fullmatch('[0-9a-f]{64}', fact['sha256']), 'readonly-file-schema')
        expected_mode = (0o600 if path.startswith(req['input_root'] + '/') else
                         0o555 if path == sealed + '/grok' else 0o444 if path == sealed + '/collector.py' else None)
        require((values[2] == stat.S_IFREG | expected_mode if expected_mode is not None else values[2] & 0o111 != 0),
                'readonly-file-mode-schema')
    for name, fact in grant['devices'].items():
        closed(fact, 'path identity')
        values = fact['identity']
        require(fact['path'] == '/dev/' + name and type(values) is list and len(values) == 9
                and all(type(number) is int and number >= 0 for number in values)
                and values[0] > 0 and values[1] > 0 and stat.S_ISCHR(values[2]) and values[4] == 0,
                'readonly-device-schema')
    require(all(type(grant[key]) is int and 0 < grant[key] < 2**32 for key in ('overflow_uid', 'overflow_gid'))
            and grant['overflow_uid'] != req['uid'] and grant['overflow_gid'] != req['gid'],
            'readonly-overflow-schema')
    if check_clock:
        readonly_deadline(req)
    return req



def readonly_child_arguments(req, phase):
    root, sealed = Path(req['input_root']), Path(req['readonly']['roots']['sealed']['path'])
    require(phase in ('control', 'native'), 'readonly-phase')
    return (['/usr/bin/python3', '-I', '-B', str(root / 'startup-witness.py'), str(root / 'readonly-witness/unexpected-startup')]
            if phase == 'control' else [str(sealed / 'grok'), 'agent', '--no-leader', 'stdio'])


def readonly_servers(req):
    root = Path(req['input_root'])
    return (dict(name=STARTUP_NAME, source='local', type='stdio', command='/usr/bin/python3',
                 args=['-I', '-B', str(root / 'startup-witness.py'), str(root / 'readonly-witness/unexpected-startup')]),)


def readonly_root_checks(req, maps=None):
    for fact in req['readonly']['roots'].values():
        readonly_deadline(req)
        descriptor = readonly_directory(Path(fact['path']), fact['identity'], maps)
        os.close(descriptor)
    if maps is not None:
        for name, fact in req['readonly']['devices'].items():
            closed(fact, 'path identity')
            require(fact['path'] == '/dev/' + name, 'readonly-device-path')
            observed = os.lstat(fact['path'])
            require(stable(observed) == readonly_host_owner(fact['identity'], maps, file=True, device=True)
                    and stat.S_ISCHR(observed.st_mode), 'readonly-device-original')
            numbers = {'null':(1,3), 'zero':(1,5), 'full':(1,7), 'random':(1,8), 'urandom':(1,9), 'tty':(5,0)}
            require((os.major(observed.st_rdev), os.minor(observed.st_rdev)) == numbers[name], 'readonly-device-number')
        require(set(os.listdir('/')) == {'usr', 'bin', 'lib', 'lib64', 'tmp', 'proc', 'dev'}, 'readonly-visible-root')
        require(set(os.listdir('/tmp')) == {Path(req['input_root']).name, Path(req['readonly']['roots']['sealed']['path']).name},
                'readonly-temporary-root')
        require(not os.path.lexists('/dev/console'), 'readonly-unexpected-console')


def readonly_parent_proof(req):
    names = {name:os.stat('/proc/self/ns/' + name).st_ino for name in READONLY_NAMESPACES}
    require(all(names[name] == req['readonly']['namespaces'][name] for name in READONLY_NAMESPACES if name != 'net')
            and names['net'] != req['parent_network_namespace'], 'readonly-outer-namespaces')
    return dict(namespaces=names, runtime=readonly_runtime(req))


def readonly_original_proof(value, req):
    closed(value, 'namespaces runtime')
    closed(value['namespaces'], ' '.join(READONLY_NAMESPACES))
    closed(value['runtime'], 'sha256 count')
    require(all(type(number) is int and number > 0 for number in value['namespaces'].values())
            and all(value['namespaces'][name] == req['readonly']['namespaces'][name]
                    for name in READONLY_NAMESPACES if name != 'net')
            and value['namespaces']['net'] != req['parent_network_namespace']
            and type(value['runtime']['count']) is int and 0 < value['runtime']['count'] <= READONLY_RUNTIME_COUNT
            and re.fullmatch('[0-9a-f]{64}', value['runtime']['sha256']), 'readonly-original-proof')
    return value


def readonly_stdio():
    for fd in (0, 1, 2):
        require(stat.S_ISFIFO(os.fstat(fd).st_mode) and not os.isatty(fd), 'readonly-piped-stdio')
    for name in os.listdir('/proc/self/fd'):
        require(name.isdecimal(), 'readonly-fd-name')
        if int(name) <= 2:
            continue
        try:
            os.fstat(int(name))
        except OSError as error:
            import errno
            require(error.errno == errno.EBADF, 'readonly-fd-inspection')
        else:
            raise ValueError('readonly-inherited-descriptor')


def readonly_inner(request_path, phase, original_text):
    require(len(original_text.encode()) <= 512, 'readonly-parent-proof-bound')
    req = readonly_request(document(ordinary(request_path, 16384), 16384))
    original = readonly_original_proof(document(original_text.encode(), 512), req)
    sealed = Path(req['readonly']['roots']['sealed']['path'])
    require(Path(__file__).absolute() == sealed / 'collector.py'
            and request_path == sealed / 'collector-request.json' and os.environ == readonly_environment(Path(req['input_root'])),
            'readonly-inner-source')
    readonly_stdio()
    maps = readonly_id_maps(req, req['readonly'])
    isolation(req)
    namespaces = {name:os.stat('/proc/self/ns/' + name).st_ino for name in READONLY_NAMESPACES}
    require(all(namespaces[name] != original['namespaces'][name]
                and namespaces[name] != req['readonly']['namespaces'][name] for name in READONLY_NAMESPACES),
            'readonly-fresh-namespaces')
    mounts = readonly_mounts(readonly_fixed_proc('/proc/self/mountinfo', 262144), req)
    readonly_root_checks(req, maps)
    readonly_verify_files(req, maps)
    before = readonly_input(req, maps)
    runtime = readonly_runtime(req, maps)
    require(runtime == original['runtime'], 'readonly-original-runtime')
    header = dict(schema_version=1, mode=READONLY_MODE, source_commit=req['source_commit'], source_tree=req['source_tree'],
                  nonce=req['nonce'], scope='a', phase=phase, parent_namespaces=original['namespaces'],
                  namespaces=namespaces, maps=maps, mount_sha256=mounts['sha256'], runtime=runtime, input=before,
                  authority_sha256=digest(json.dumps(req['readonly'], sort_keys=True, separators=(',', ':')).encode()))
    readonly_root_checks(req, maps)
    require({name:os.stat('/proc/self/ns/' + name).st_ino for name in READONLY_NAMESPACES} == namespaces
            and readonly_mounts(readonly_fixed_proc('/proc/self/mountinfo', 262144), req)['sha256'] == mounts['sha256'],
            'readonly-final-namespace')
    readonly_deadline(req)
    encoded = READONLY_HEADER + json.dumps(header, sort_keys=True, separators=(',', ':')).encode() + b'\n'
    require(len(encoded) <= 2048, 'readonly-header-bound')
    require(os.write(2, encoded) == len(encoded), 'readonly-header-write')
    argv = readonly_child_arguments(req, phase)
    os.execve(argv[0], argv, readonly_environment(Path(req['input_root'])))


def readonly_header(raw, req, phase, original):
    line, separator, remainder = raw.partition(b'\n')
    require(separator and line.startswith(READONLY_HEADER) and len(line) + 1 <= 2048, 'readonly-header')
    value = document(line[len(READONLY_HEADER):], 2048)
    closed(value, 'schema_version mode source_commit source_tree nonce scope phase parent_namespaces namespaces maps mount_sha256 runtime input authority_sha256')
    require(line == READONLY_HEADER + json.dumps(value, sort_keys=True, separators=(',', ':')).encode(),
            'readonly-header-canonical')
    require(type(value['schema_version']) is int and value['schema_version'] == 1 and value['mode'] == READONLY_MODE
            and value['source_commit'] == req['source_commit'] and value['source_tree'] == req['source_tree']
            and value['nonce'] == req['nonce'] and value['scope'] == 'a' and value['phase'] == phase
            and value['parent_namespaces'] == original['namespaces'] and value['runtime'] == original['runtime']
            and value['input'] == dict(sha256=req['readonly']['input_sha256'], count=req['readonly']['input_count'])
            and value['authority_sha256'] == digest(json.dumps(req['readonly'], sort_keys=True, separators=(',', ':')).encode())
            and re.fullmatch('[0-9a-f]{64}', value['mount_sha256']), 'readonly-header-join')
    closed(value['namespaces'], ' '.join(READONLY_NAMESPACES))
    require(all(type(number) is int and number > 0 and number != original['namespaces'][name]
                and number != req['readonly']['namespaces'][name] for name, number in value['namespaces'].items()),
            'readonly-header-namespaces')
    closed(value['maps'], 'uid gid')
    for name, row in value['maps'].items():
        closed(row, 'inside parent count overflow sha256')
        require(all(type(row[key]) is int for key in ('inside', 'parent', 'count', 'overflow'))
                and row['inside'] == req[name] and row['parent'] == 0 and row['count'] == 1
                and row['overflow'] == req['readonly']['overflow_' + name]
                and re.fullmatch('[0-9a-f]{64}', row['sha256']), 'readonly-header-map')
    require(READONLY_HEADER not in remainder, 'readonly-duplicate-header')
    return value, remainder


def readonly_command(req, phase, original, remaining, state):
    root, deadline = Path(req['input_root']), readonly_deadline(req)
    require(type(remaining) is int and 0 < remaining <= LIMIT, 'readonly-stream-budget')
    wrapper = readonly_bwrap_argv(req, phase, original)
    process = subprocess.Popen(wrapper, cwd=root / 'a', env=readonly_environment(root),
                               stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                               start_new_session=True, close_fds=True)
    streams = {'stdout':bytearray(), 'stderr':bytearray()}
    pending, error, completed, frames = bytearray(), None, False, 0
    def send(value):
        encoded = json.dumps(value, separators=(',', ':')).encode() + b'\n'
        require(len(encoded) <= 4096, 'request-bound')
        process.stdin.write(encoded)
        process.stdin.flush()
    def read_chunk(selector, key):
        chunk = os.read(key.fd, 65536)
        if not chunk:
            selector.unregister(key.fileobj)
            return b''
        streams[key.data].extend(chunk)
        require(sum(map(len, streams.values())) <= remaining, 'stream-bound')
        return chunk
    try:
        with selectors.DefaultSelector() as selector:
            for name, pipe in (('stdout', process.stdout), ('stderr', process.stderr)):
                os.set_blocking(pipe.fileno(), False)
                selector.register(pipe, selectors.EVENT_READ, name)
            if phase == 'native':
                send({'jsonrpc':'2.0', 'id':1, 'method':'initialize',
                      'params':{'protocolVersion':1, 'clientCapabilities':{},
                                'clientInfo':{'name':'uap-inventory-admission', 'version':'1'}}})
            else:
                process.stdin.close()
            while selector.get_map() or process.poll() is None:
                readonly_deadline(req)
                for key, _ in selector.select(min(.05, max(.001, deadline - time.monotonic()))):
                    chunk = read_chunk(selector, key)
                    if key.data != 'stdout' or not chunk:
                        continue
                    if phase == 'control':
                        require(not chunk, 'readonly-control-output')
                    pending.extend(chunk)
                    while b'\n' in pending:
                        line, _, pending = pending.partition(b'\n')
                        frames += 1
                        require(frames <= 64 and len(line) <= 64 << 10 and not completed, 'frame-bound')
                        completed = rpc_frame(document(bytes(line), 64 << 10), state, req, send,
                                              expected_servers=readonly_servers(req))
                        require(len(json.dumps(state).encode()) <= 128 << 10, 'rpc-bound')
                        if completed:
                            process.stdin.close()
                            process.terminate()
                    require(len(pending) <= 64 << 10, 'frame-bound')
            process.wait(timeout=max(.001, deadline - time.monotonic()))
            require(phase != 'native' or completed and not pending, 'rpc-incomplete')
    except (ValueError, OSError, subprocess.SubprocessError):
        error = 'deadline' if time.monotonic() >= deadline else 'protocol-refused'
    finally:
        process.kill()
        process.wait(timeout=max(.001, deadline - time.monotonic()))
        process.stdin.close()
        try:
            with selectors.DefaultSelector() as selector:
                for name, pipe in (('stdout', process.stdout), ('stderr', process.stderr)):
                    selector.register(pipe, selectors.EVENT_READ, name)
                while selector.get_map():
                    readonly_deadline(req)
                    for key, _ in selector.select(min(.05, max(.001, deadline - time.monotonic()))):
                        read_chunk(selector, key)
        finally:
            process.stdout.close()
            process.stderr.close()
    raw = {name:bytes(body) for name, body in streams.items()}
    row = dict(label='readonly-control' if phase == 'control' else 'acp', argv=readonly_child_arguments(req, phase),
               wrapper_argv=wrapper, exit=process.returncode, error=error)
    for name, body in raw.items():
        row[name + '_base64'] = base64.b64encode(body).decode()
        row[name + '_bytes'], row[name + '_sha256'] = len(body), digest(body)
    return row


def readonly_control_marker(req):
    root = Path(req['input_root'])
    fact = req['readonly']['roots']['readonly-witness']
    descriptor = readonly_directory(root / 'readonly-witness', fact['identity'])
    marker = None
    try:
        require(set(os.listdir(descriptor)) == {'unexpected-startup'}, 'readonly-control-members')
        before = os.stat('unexpected-startup', dir_fd=descriptor, follow_symlinks=False)
        require(stat.S_ISREG(before.st_mode) and before.st_mode & 0o7777 == 0o600 and before.st_nlink == 1
                and before.st_uid == req['uid'] and before.st_gid == req['gid'], 'readonly-control-file')
        marker = os.open('unexpected-startup', os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC, dir_fd=descriptor)
        require(stable(os.fstat(marker)) == stable(before), 'readonly-control-held')
        expected = b'uap-readonly:' + req['nonce'].encode() + b'\n'
        require(before.st_size == len(expected) and os.read(marker, len(expected) + 1) == expected
                and stable(os.fstat(marker)) == stable(before)
                == stable(os.stat('unexpected-startup', dir_fd=descriptor, follow_symlinks=False)), 'readonly-control-body')
        require(readonly_metadata(os.fstat(descriptor), directory=True)
                == readonly_metadata((root / 'readonly-witness').lstat(), directory=True) == tuple(fact['identity']),
                'readonly-control-directory')
        os.unlink('unexpected-startup', dir_fd=descriptor)
        require(os.fstat(marker).st_nlink == 0 and not os.listdir(descriptor)
                and readonly_metadata(os.fstat(descriptor), directory=True)
                == readonly_metadata((root / 'readonly-witness').lstat(), directory=True) == tuple(fact['identity']),
                'readonly-control-removal')
        return dict(name='unexpected-startup', identity=list(stable(before)),
                    removed_identity=list(stable(os.fstat(marker))), directory_identity=fact['identity'],
                    bytes=len(expected), sha256=digest(expected))
    finally:
        if marker is not None:
            os.close(marker)
        os.close(descriptor)


def readonly_collector_result(req):
    readonly_request(req)
    result = {key:req[key] for key in ('schema_version', 'source_commit', 'source_tree', 'scope', 'nonce')}
    result.update(mode=READONLY_MODE, semantic_complete=False, reason='isolation-refused', isolation={}, commands=[], rpc=rpc_state(),
                  readonly=dict(authority=req['readonly'], stage='isolation', control_complete=False, original=None, control_marker=None))
    in_flight = False
    try:
        result['isolation'] = isolation(req)
        readonly_root_checks(req)
        readonly_verify_files(req)
        readonly_input(req)
        result['readonly']['stage'] = 'runtime'
        original = readonly_parent_proof(req)
        result['readonly']['original'] = original
        result['reason'] = 'protocol-refused'
        headers = []
        for phase in ('control', 'native'):
            result['readonly']['stage'] = phase
            readonly_deadline(req)
            readonly_root_checks(req)
            readonly_verify_files(req)
            readonly_input(req)
            require({name:os.stat('/proc/self/ns/' + name).st_ino for name in READONLY_NAMESPACES} == original['namespaces'],
                    'readonly-parent-drift')
            in_flight = True
            row = readonly_command(req, phase, original,
                                   LIMIT - sum(item['stdout_bytes'] + item['stderr_bytes'] for item in result['commands']),
                                   result['rpc'])
            in_flight = False
            result['commands'].append(row)
            header, trailing = readonly_header(base64.b64decode(row['stderr_base64'], validate=True), req, phase, original)
            headers.append(header)
            require(row['error'] is None, 'readonly-child-refused')
            if phase == 'control':
                require(row['exit'] == 93 and row['stdout_bytes'] == 0 and not trailing, 'readonly-control-exit')
                result['readonly']['control_marker'] = readonly_control_marker(req)
                result['readonly']['control_complete'] = True
            else:
                require(result['rpc']['plugins'] is not None and not list((Path(req['input_root']) / 'readonly-witness').iterdir()),
                        'readonly-unexpected-native-startup')
        require(all(headers[0]['namespaces'][name] != headers[1]['namespaces'][name] for name in READONLY_NAMESPACES),
                'readonly-namespace-reuse')
        readonly_input(req)
        readonly_verify_files(req)
        require(result['rpc']['notifications'] == [{'jsonrpc':'2.0', 'method':'_x.ai/mcp/servers_updated',
                'params':{'mcpServers':list(readonly_servers(req))}}], 'readonly-native-witness-inventory')
        inventory_rows(result, Path(req['input_root']), 'a')
        result['readonly']['stage'] = 'complete'
        result.update(semantic_complete=True, reason='complete')
    except (ValueError, OSError, subprocess.SubprocessError):
        if in_flight:
            raise
    return result


def readonly_marker_proof(selected, req):
    proof = selected['control_marker']
    if not selected['control_complete']:
        require(proof is None, 'readonly-uncompleted-marker')
        return
    closed(proof, 'name identity removed_identity directory_identity bytes sha256')
    original, removed = proof['identity'], proof['removed_identity']
    expected = b'uap-readonly:' + req['nonce'].encode() + b'\n'
    require(proof['name'] == 'unexpected-startup' and type(original) is list and type(removed) is list
            and len(original) == len(removed) == 9
            and all(type(value) is int and value >= 0 for value in original + removed)
            and original[0] > 0 and original[1] > 0 and original[2] == stat.S_IFREG | 0o600
            and original[3] == 1 and removed[3] == 0 and original[4:6] == [req['uid'], req['gid']]
            and original[:3] == removed[:3] and original[4:8] == removed[4:8]
            and removed[8] >= original[8] and original[6] == len(expected)
            and proof['directory_identity'] == req['readonly']['roots']['readonly-witness']['identity']
            and type(proof['bytes']) is int and proof['bytes'] == len(expected)
            and proof['sha256'] == digest(expected), 'readonly-marker-proof')




def readonly_fixture(root, nonce):
    witness = root / 'readonly-witness'
    witness.mkdir(mode=0o700)
    path = root / 'home/.grok/config.toml'
    require(not path.exists() and not (root / 'startup-witness.py').exists(), 'readonly-authored-collision')
    args = ['-I', '-B', str(root / 'startup-witness.py'), str(witness / 'unexpected-startup')]
    body = ('[marketplace]\ndefault_skills_installs_purged = true\n'
            '[mcp_servers.' + STARTUP_NAME + ']\ncommand = "/usr/bin/python3"\nargs = '
            + json.dumps(args) + '\nenabled = true\n').encode()
    with path.open('xb') as stream:
        stream.write(body)
    path.chmod(0o600)
    program = ("import os,stat,sys\n"
               "from pathlib import Path\n"
               "p=Path(sys.argv[1]);d=os.open(p.parent,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW|os.O_CLOEXEC)\n"
               "assert p.name=='unexpected-startup' and not os.listdir(d)\n"
               "f=os.open(p.name,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW|os.O_CLOEXEC,0o600,dir_fd=d)\n"
               "b=" + repr(b'uap-readonly:' + nonce.encode() + b'\n') + "\n"
               "assert os.write(f,b)==len(b)\nos.fsync(f)\nos.close(f)\nos.close(d)\nraise SystemExit(93)\n")
    with (root / 'startup-witness.py').open('x', encoding='utf-8') as stream:
        stream.write(program)
    (root / 'startup-witness.py').chmod(0o600)


def readonly_streams(row):
    closed(row, 'label argv wrapper_argv exit error stdout_base64 stdout_bytes stdout_sha256 stderr_base64 stderr_bytes stderr_sha256')
    raw = {}
    require(type(row['exit']) is int and -255 <= row['exit'] <= 255
            and row['error'] in (None, 'deadline', 'protocol-refused')
            and all(type(row[key]) is list and all(type(value) is str for value in row[key])
                    for key in ('argv', 'wrapper_argv')), 'readonly-command-types')
    for name in ('stdout', 'stderr'):
        length, encoded = row[name + '_bytes'], row[name + '_base64']
        require(type(length) is int and 0 <= length <= LIMIT and type(encoded) is str
                and len(encoded) == 4*((length + 2)//3), 'readonly-stream-bound')
        body = base64.b64decode(encoded, validate=True)
        require(len(body) == length and digest(body) == row[name + '_sha256'], 'readonly-stream-join')
        raw[name] = body
    require(sum(map(len, raw.values())) <= LIMIT, 'readonly-command-aggregate')
    return raw


def validate_readonly_receipt(receipt, request):
    closed(receipt, 'schema_version source_commit source_tree scope nonce cleanup_complete sealed_cleanup_complete system_executables result')
    for key in ('schema_version', 'source_commit', 'source_tree', 'scope', 'nonce'):
        require(type(receipt[key]) is type(request[key]) and receipt[key] == request[key], 'readonly-helper-source')
    require(receipt['cleanup_complete'] is True and receipt['sealed_cleanup_complete'] is True, 'readonly-cleanup')
    for name, fact in closed(receipt['system_executables'], 'unshare setpriv env python3 bwrap').items():
        closed(fact, 'path sha256')
        require(fact['path'] == '/usr/bin/' + ('python3.12' if name == 'python3' else name)
                and re.fullmatch('[0-9a-f]{64}', fact['sha256']), 'readonly-system')
    result = closed(receipt['result'], 'schema_version source_commit source_tree scope nonce semantic_complete reason isolation commands rpc mode readonly')
    require(result['mode'] == READONLY_MODE and request['mode'] == READONLY_MODE and request['scope'] == 'a',
            'readonly-result-mode')
    for key in ('schema_version', 'source_commit', 'source_tree', 'scope', 'nonce'):
        require(type(result[key]) is type(request[key]) and result[key] == request[key], 'readonly-result-source')
    require(type(result['semantic_complete']) is bool and result['reason'] in
            ('complete', 'protocol-refused', 'help-refused', 'isolation-refused', 'collector-refused')
            and result['semantic_complete'] == (result['reason'] == 'complete'), 'readonly-result-state')
    selected = closed(result['readonly'], 'authority stage control_complete original control_marker')
    require(type(selected['control_complete']) is bool and selected['stage'] in
            ('isolation', 'runtime', 'control', 'native', 'complete'), 'readonly-stage')
    proof = closed(result['isolation'], 'uid gid euid egid groups network_namespace parent_network_namespace interfaces capabilities_zero no_new_privileges')
    require(proof['uid'] == proof['euid'] == request['uid'] and proof['gid'] == proof['egid'] == request['gid']
            and proof['groups'] == [] and proof['interfaces'] == ['lo'] and proof['capabilities_zero'] is True
            and type(proof['no_new_privileges']) is int and proof['no_new_privileges'] == 1
            and type(proof['network_namespace']) is int and proof['network_namespace'] > 0
            and type(proof['parent_network_namespace']) is int and proof['parent_network_namespace'] > 0
            and proof['network_namespace'] != proof['parent_network_namespace'], 'readonly-isolation')
    req = {key:request[key] for key in ('schema_version', 'source_commit', 'source_tree', 'uid', 'gid', 'nonce',
                                      'scope', 'start_monotonic_ns', 'mode')}
    req.update(input_root='/tmp/uap-grok-acp-input-' + request['nonce'],
               parent_network_namespace=proof['parent_network_namespace'], readonly=selected['authority'])
    # Validate recorded authority without renewing or requiring the past native clock.
    readonly_request(req, check_clock=False)
    readonly_marker_proof(selected, req)
    from project_native_collectors.grok import ACQUISITION
    grant = selected['authority']
    sealed = grant['roots']['sealed']['path']
    require(grant['files'][sealed + '/grok']['sha256'] == ACQUISITION['binary_sha256'], 'readonly-recorded-native-pin')
    for name in ('python3', 'bwrap'):
        system = receipt['system_executables'][name]
        require(grant['files'][system['path']]['sha256'] == system['sha256'], 'readonly-recorded-system-join')
    require(type(result['commands']) is list and len(result['commands']) <= 2, 'readonly-command-count')
    original = selected['original']
    if original is not None:
        readonly_original_proof(original, req)
        require(original['namespaces']['net'] == proof['network_namespace'], 'readonly-outer-net-join')
    else:
        require(not result['commands'] and selected['stage'] in ('isolation', 'runtime')
                and not selected['control_complete'], 'readonly-missing-original')
    total, headers = 0, []
    for index, row in enumerate(result['commands']):
        phase = ('control', 'native')[index]
        raw = readonly_streams(row)
        total += sum(map(len, raw.values()))
        require(row['label'] == READONLY_LABELS[index] and row['argv'] == readonly_child_arguments(req, phase)
                and row['wrapper_argv'] == readonly_bwrap_argv(req, phase, original), 'readonly-wrapper-argv')
        if result['semantic_complete'] or selected['control_complete'] and index == 0:
            header, remainder = readonly_header(raw['stderr'], req, phase, original)
            headers.append(header)
            require(row['error'] is None, 'readonly-complete-command')
            if phase == 'control':
                require(row['exit'] == 93 and raw['stdout'] == b'' and remainder == b'', 'readonly-control-evidence')
    require(total <= LIMIT and len(json.dumps(result['rpc']).encode()) <= 128 << 10, 'readonly-aggregate')
    closed(result['rpc'], 'initialize plugins notifications')
    if result['semantic_complete']:
        require(len(result['commands']) == 2 and selected['control_complete'] and selected['stage'] == 'complete'
                and all(headers[0]['namespaces'][name] != headers[1]['namespaces'][name] for name in READONLY_NAMESPACES),
                'readonly-complete-proof')
        replay = rpc_state()
        native = readonly_streams(result['commands'][1])['stdout']
        require(native.endswith(b'\n') and len(native.splitlines()) <= 64, 'readonly-stdout-frames')
        complete = False
        for line in native.splitlines():
            require(not complete, 'readonly-post-completion-frame')
            complete = rpc_frame(document(line, 64 << 10), replay, req, lambda _:None, expected_servers=readonly_servers(req))
        require(complete and replay == result['rpc'] and replay['notifications'] == [{'jsonrpc':'2.0',
                'method':'_x.ai/mcp/servers_updated', 'params':{'mcpServers':list(readonly_servers(req))}}],
                'readonly-rpc-replay')
        inventory_rows(result, Path(req['input_root']), 'a')
    return result


def readonly_source(shared, args):
    value = subprocess.run(['/usr/bin/git', *args], cwd=shared.SOURCE, capture_output=True, timeout=5, check=True,
                           env={'PATH':'/usr/bin:/bin', 'GIT_CONFIG_NOSYSTEM':'1', 'GIT_CONFIG_GLOBAL':'/dev/null'})
    require(not value.stderr and len(value.stdout) <= 4096, 'readonly-source-bound')
    return value.stdout.decode().strip()



def readonly_helper_capture(helper, root, req):
    """Same fixed transport as helper_capture, with no renewed cleanup grace."""
    deadline = readonly_deadline(req, final=True)
    argv = ['/usr/bin/sudo', '-n', '--preserve-env=GITHUB_ACTIONS,RUNNER_ENVIRONMENT,AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED,EXPECTED_COMMIT',
            str(helper), '--input-root', str(root)]
    env = {key:os.environ[key] for key in ('GITHUB_ACTIONS', 'RUNNER_ENVIRONMENT',
           'AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED', 'EXPECTED_COMMIT') if key in os.environ}
    env.update(PATH='/usr/bin:/bin', LANG='C.UTF-8')
    process = subprocess.Popen(argv, env=env, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                               stderr=subprocess.PIPE, start_new_session=True, close_fds=True)
    streams, failure = {'stdout':bytearray(), 'stderr':bytearray()}, None
    try:
        with selectors.DefaultSelector() as selector:
            for name, pipe in (('stdout', process.stdout), ('stderr', process.stderr)):
                os.set_blocking(pipe.fileno(), False)
                selector.register(pipe, selectors.EVENT_READ, name)
            while selector.get_map() or process.poll() is None:
                readonly_deadline(req, final=True)
                for key, _ in selector.select(min(.05, max(.001, deadline - time.monotonic()))):
                    chunk = os.read(key.fd, 65536)
                    if not chunk:
                        selector.unregister(key.fileobj)
                    else:
                        streams[key.data].extend(chunk)
                        maximum = (2 << 20) + 16384 if key.data == 'stdout' else 65536
                        require(len(streams[key.data]) <= maximum, 'helper-stream-bound')
            readonly_deadline(req, final=True)
            status = process.wait(timeout=max(.001, deadline - time.monotonic()))
            if status == 2 and not streams['stdout']:
                for code in READONLY_HELPER_CODES:
                    if streams['stderr'] == ('grok-acp-controller-refused:' + code + '\n').encode():
                        raise ReadonlyHelperRefusal(code)
            require(status == 0 and not streams['stderr'], 'helper-refused')
    except Exception as error:
        failure = error
    finally:
        try:
            process.stdin.close()
            if process.poll() is None:
                remaining = deadline - time.monotonic()
                if remaining > 0:
                    try:
                        process.wait(timeout=remaining)
                    except subprocess.TimeoutExpired:
                        pass
                if process.poll() is None:
                    process.kill()
                    remaining = deadline - time.monotonic()
                    if remaining > 0:
                        process.wait(timeout=remaining)
        except Exception as error:
            if failure is None:
                failure = error
        finally:
            process.stdout.close()
            process.stderr.close()
    if failure is not None:
        raise failure
    readonly_deadline(req, final=True)
    return document(bytes(streams['stdout']), (2 << 20) + 16384)


def readonly_run(output, progress):
    import project_native_research as shared
    from project_native_collectors.grok import ACQUISITION
    from devin_structured_inventory_admission import metadata_inventory
    from client_automation_native_probe import snapshot
    shared.native.require_hosted('linux-amd64')
    output = output.absolute()
    require(output.resolve() == output and not output.exists() and os.getuid() != 0, 'readonly-output')
    output.mkdir(mode=0o700, parents=True)
    output_fd = os.open(output, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC)
    output_identity = directory_id(os.fstat(output_fd))
    nonce = os.urandom(16).hex()
    root = Path('/tmp/uap-grok-acp-input-' + nonce)
    root.mkdir(mode=0o700)
    root_id = directory_id(root.lstat())
    summary = dict(schema_version=1, mode=READONLY_MODE, collection_complete=False, measurement_complete=False,
                   cleanup_complete=False, qualification=dict(FALSE), scopes=[])
    bodies, cleanup_proved, request = {}, True, None
    try:
        with shared.Budget(150) as budget:
            progress['stage'] = 'source'
            commit, tree = readonly_source(shared, ['rev-parse', 'HEAD']), readonly_source(shared, ['rev-parse', 'HEAD^{tree}'])
            require(commit == os.environ.get('EXPECTED_COMMIT') and re.fullmatch('[0-9a-f]{40}', commit)
                    and re.fullmatch('[0-9a-f]{40}', tree)
                    and not readonly_source(shared, ['status', '--porcelain', '--untracked-files=normal']), 'readonly-source-identity')
            summary.update(source_commit=commit, source_tree=tree, nonce=nonce, run=os.environ.get('GITHUB_RUN_ID'),
                           attempt=os.environ.get('GITHUB_RUN_ATTEMPT'))
            progress['source_identity_checked'] = True
            progress['stage'] = 'acquisition'
            acquired, contents = shared.acquire(ACQUISITION, budget)
            require(digest(contents['grok']) == ACQUISITION['binary_sha256'], 'readonly-native-pin')
            (root / 'grok').write_bytes(contents['grok'])
            (root / 'grok').chmod(0o700)
            collector = Path(__file__).resolve()
            source_body = ordinary(collector)
            (root / 'collector.py').write_bytes(source_body)
            helper = root / 'helper'
            progress['stage'] = 'build'
            subprocess.run(['go', 'build', '-trimpath', '-ldflags', '-X main.sourceCommit=' + commit + ' -X main.sourceTree=' + tree
                            + ' -X main.collectorSHA=' + digest(source_body), '-o', str(helper), './research/grokacpinventory'],
                           cwd=shared.SOURCE / 'install/integrationctl', check=True, timeout=60)
            progress['stage'] = 'fixtures'
            roots = fixtures(root, nonce)
            readonly_fixture(root, nonce)
            bodies['acquisition.json'] = json.dumps(acquired, sort_keys=True).encode()
            request = dict(schema_version=1, source_commit=commit, source_tree=tree, uid=os.getuid(), gid=os.getgid(),
                           nonce=nonce, scope='a', start_monotonic_ns=time.monotonic_ns(),
                           helper_sha256=digest(ordinary(helper, 64 << 20)), mode=READONLY_MODE)
            summary['start_monotonic_ns'] = request['start_monotonic_ns']
            (root / 'request.json').write_text(json.dumps(request), encoding='utf-8')
            progress['stage'] = 'baseline'
            before = metadata_inventory(root, budget)
            profiles = {name:snapshot(path) for name, path in roots.items()}
            readonly_deadline(request)
            cleanup_proved = False
            progress['stage'] = 'helper'
            receipt = readonly_helper_capture(helper, root, request)
            progress['stage'] = 'receipt'
            result = validate_readonly_receipt(receipt, request)
            cleanup_proved = True
            readonly_deadline(request, final=True)
            progress['stage'] = 'ledger'
            after = metadata_inventory(root, budget)
            current = {name:snapshot(path) for name, path in roots.items()} if after == before else {}
            unchanged = after == before and current == profiles
            require(metadata_inventory(root, budget) == after, 'readonly-post-read-drift')
            bodies['readonly.ledger.json'] = json.dumps(dict(before=before, after=after, profiles_before=profiles,
                profiles_after=current, protected_originals_unchanged=unchanged,
                witness_absent=not list((root / 'readonly-witness').iterdir())), sort_keys=True).encode()
            for row in result['commands']:
                for name in ('stdout', 'stderr'):
                    member = row['label'] + '.' + name
                    bodies[member] = base64.b64decode(row.pop(name + '_base64'), validate=True)
                    row[name + '_file'] = member
            bodies['readonly.rpc.json'] = json.dumps(result.pop('rpc'), sort_keys=True).encode()
            summary['scopes'].append(receipt)
            summary['measurement_complete'] = True
            require(unchanged and result['semantic_complete'], 'readonly-native-refused')
            progress['stage'] = 'source-final'
            require(not readonly_source(shared, ['status', '--porcelain', '--untracked-files=normal']), 'readonly-source-changed')
            readonly_deadline(request, final=True)
    except Exception as error:
        frontend_error(progress, error)
        summary['error'] = 'admission-refused'
    finally:
        progress['stage'] = 'cleanup'
        if cleanup_proved and directory_id(root.lstat()) == root_id:
            shutil.rmtree(root)
            summary['cleanup_complete'] = True
        progress['cleanup_complete'] = summary['cleanup_complete']
        try:
            require(summary['cleanup_complete'] and request is not None and len(summary['scopes']) == 1,
                    'readonly-publication-withheld')
            readonly_deadline(request, final=True)
            summary['elapsed_ns'] = time.monotonic_ns() - request['start_monotonic_ns']
            bodies['admission.json'] = json.dumps(summary, sort_keys=True).encode() + b'\n'
            progress['stage'] = 'archive'
            progress['publication_attempted'] = True
            publication = publish(output, bodies, output_fd, output_identity, mode=READONLY_MODE)
            readonly_deadline(request, final=True)
            progress['stage'] = 'receipt-output'
            print(json.dumps(dict(schema_version=1, source_commit=summary['source_commit'], source_tree=summary['source_tree'],
                                  nonce=summary['nonce'], publication=publication), separators=(',', ':')))
            progress['receipt_emitted'] = True
        finally:
            os.close(output_fd)
    return 0 if summary['measurement_complete'] and 'error' not in summary else 1


def readonly_frontend(output):
    progress = frontend_state()
    try:
        status = readonly_run(output, progress)
    except Exception as error:
        if progress['failure_stage'] is None:
            frontend_error(progress, error)
        status = 1
    if not progress['receipt_emitted']:
        print(json.dumps(dict(schema_version=1, kind='frontend-publication-refused',
                              stage=progress['failure_stage'] or progress['stage'], error_class=progress['error_class'],
                              source_identity_checked=progress['source_identity_checked'],
                              cleanup_complete=progress['cleanup_complete'], publication_attempted=progress['publication_attempted']),
                         separators=(',', ':')))
        return 1
    return status



def readonly_members():
    return {'admission.json', 'acquisition.json', 'readonly.ledger.json', 'readonly.rpc.json'} | {
        label + '.' + stream for label in READONLY_LABELS for stream in ('stdout', 'stderr')}


def readonly_readout(members, packet, deadline):
    require(type(deadline) is float and time.monotonic() < deadline, 'readonly-readout-deadline')
    summary = document(members['admission.json'])
    expected = 'schema_version mode collection_complete measurement_complete cleanup_complete qualification scopes source_commit source_tree nonce run attempt start_monotonic_ns elapsed_ns'
    closed(summary, expected + (' error' if 'error' in summary else ''))
    require(type(summary['schema_version']) is int and summary['schema_version'] == 1 and summary['mode'] == READONLY_MODE
            and summary['collection_complete'] is False and summary['measurement_complete'] is True
            and summary['cleanup_complete'] is True and summary.get('error') in (None, 'admission-refused')
            and type(summary['start_monotonic_ns']) is int and summary['start_monotonic_ns'] > 0
            and type(summary['elapsed_ns']) is int and 0 <= summary['elapsed_ns'] < 30_000_000_000
            and closed(summary['qualification'], ' '.join(FALSE)) == FALSE
            and all(value is False for value in summary['qualification'].values()), 'readonly-readout-summary')
    for key in ('source_commit', 'source_tree', 'nonce'):
        require(type(summary[key]) is str and summary[key] == packet[key], 'readonly-readout-source')
    require(type(summary['scopes']) is list and len(summary['scopes']) == 1, 'readonly-readout-scope')
    ledger = document(members['readonly.ledger.json'])
    closed(ledger, 'before after profiles_before profiles_after protected_originals_unchanged witness_absent')
    require(all(type(ledger[name]) is dict and len(ledger[name]) <= 4096
                for name in ('before', 'after', 'profiles_before', 'profiles_after'))
            and type(ledger['protected_originals_unchanged']) is bool and type(ledger['witness_absent']) is bool,
            'readonly-readout-ledger')
    readonly_ledger_digest(ledger['before'])
    readonly_ledger_digest(ledger['after'])
    encoded = lambda value: json.dumps(value, sort_keys=True, separators=(',', ':'), allow_nan=False)
    unchanged = (encoded(ledger['before']) == encoded(ledger['after'])
                 and encoded(ledger['profiles_before']) == encoded(ledger['profiles_after']))
    require(ledger['protected_originals_unchanged'] == unchanged, 'readonly-readout-originals')
    root = ledger['before']['.']
    require(type(root) is list and len(root) == 5 and all(type(value) is int for value in root)
            and root[2] == stat.S_IFDIR | 0o700 and root[3] > 0 and root[4] > 0, 'readonly-readout-root')
    receipt = summary['scopes'][0]
    result = receipt['result']
    require(type(result['commands']) is list and len(result['commands']) <= 2, 'readonly-readout-count')
    expected_members = {'admission.json', 'acquisition.json', 'readonly.ledger.json', 'readonly.rpc.json'}
    for index, row in enumerate(result['commands']):
        for stream in ('stdout', 'stderr'):
            member = READONLY_LABELS[index] + '.' + stream
            require(row.get(stream + '_file') == member, 'readonly-readout-stream-name')
            row.pop(stream + '_file')
            row[stream + '_base64'] = base64.b64encode(members[member]).decode()
            expected_members.add(member)
    require(set(members) == expected_members, 'readonly-readout-members')
    result['rpc'] = document(members['readonly.rpc.json'], 128 << 10)
    request = dict(schema_version=1, source_commit=packet['source_commit'], source_tree=packet['source_tree'],
                   nonce=packet['nonce'], uid=root[3], gid=root[4], scope='a', mode=READONLY_MODE,
                   start_monotonic_ns=summary['start_monotonic_ns'])
    validate_readonly_receipt(receipt, request)
    require(readonly_ledger_digest(ledger['before']) == dict(sha256=result['readonly']['authority']['input_sha256'],
            count=result['readonly']['authority']['input_count']), 'readonly-readout-baseline-join')
    if summary.get('error') is None:
        require(result['semantic_complete'] is True and unchanged and ledger['witness_absent'], 'readonly-readout-verdict')
    require(time.monotonic() < deadline, 'readonly-readout-deadline')
    return dict(schema_version=1, mode=READONLY_MODE, reason='complete' if summary.get('error') is None else 'admission-refused',
                measurement_complete=True, cleanup_complete=True, semantic_complete=result['semantic_complete'],
                protected_originals_unchanged=unchanged, witness_absent=ledger['witness_absent'],
                stage=result['readonly']['stage'], control_complete=result['readonly']['control_complete'],
                commands=len(result['commands']), qualification=dict(FALSE))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path)
    parser.add_argument('--collect', type=Path)
    parser.add_argument('--session-only', action='store_true')
    parser.add_argument('--lock-qualification', action='store_true')
    parser.add_argument('--plugin-cli-observation', action='store_true')
    parser.add_argument('--readonly-inventory', action='store_true')
    parser.add_argument('--readonly-inner', nargs=3)
    args = parser.parse_args()
    if args.readonly_inner is not None:
        require(not args.readonly_inventory and args.output is None and args.collect is None
                and not args.session_only and not args.lock_qualification and not args.plugin_cli_observation, 'mode')
        readonly_inner(Path(args.readonly_inner[0]), args.readonly_inner[1], args.readonly_inner[2])
    elif args.readonly_inventory:
        require(args.output is not None and args.collect is None and not args.session_only
                and not args.lock_qualification and not args.plugin_cli_observation, 'mode')
        raise SystemExit(readonly_frontend(args.output))
    elif args.collect is not None and args.output is None and not args.session_only and not args.lock_qualification and not args.plugin_cli_observation:
        collect(args.collect)
    else:
        require(args.output is not None and args.collect is None, 'mode')
        raise SystemExit(frontend(args.output, session_only=args.session_only, lock_qualification=args.lock_qualification, plugin_cli=args.plugin_cli_observation))
