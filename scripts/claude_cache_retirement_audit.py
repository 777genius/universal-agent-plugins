"""Exact native Claude cache custody at removal; never writer-closure evidence."""
import hashlib
import json
import os
from pathlib import Path
import re
from runpy import run_path
import stat
import sys
import time

_FILES = run_path(str(Path(__file__).with_name('native_client_project_artifacts.py')))
MAX_FILES, MAX_FILE_BYTES, MAX_TREE_BYTES, MAX_DEPTH = 10000, 64 << 20, 256 << 20, 64
_DARWIN = (run_path(str(Path(__file__).with_name('claude_darwin_metadata.py')))
           if sys.platform == 'darwin' else None)

_WINDOWS = (run_path(str(Path(__file__).with_name('claude_windows_cache_custody.py')))
            if sys.platform == 'win32' else None)


def metadata(fd):
    if _DARWIN is not None:
        _DARWIN['check'](fd)


def require(value):
    if not value:
        raise ValueError('Claude cache retirement authority is unresolved')


def ordinary(path):
    info = path.lstat()
    require(stat.S_ISDIR(info.st_mode) and path.resolve(strict=True) == path)
    return info


def open_directory(path):
    before = ordinary(path)
    fd = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        require(os.path.samestat(before, os.fstat(fd)))
        metadata(fd)
        require(os.path.samestat(before, os.fstat(fd)))
        return fd
    except BaseException:
        os.close(fd)
        raise


def named_directory(path, fd):
    require(os.path.samestat(ordinary(path), os.fstat(fd)))
    metadata(fd)
    require(os.path.samestat(ordinary(path), os.fstat(fd)))


def tree(fd):
    """Full descriptor-rooted byte/identity comparison, not a package digest engine."""
    result, files, total = {}, 0, 0
    deadline = time.monotonic() + 5
    def budget():
        require(time.monotonic() < deadline)
    def names(directory):
        found = []
        with os.scandir(directory) as entries:
            for entry in entries:
                budget()
                found.append(entry.name)
                require(len(found) <= MAX_FILES)
        return sorted(found)
    def visit(directory, relative, depth):
        nonlocal files, total
        budget()
        require(depth <= MAX_DEPTH and len(result) <= MAX_FILES)
        info = os.fstat(directory)
        metadata(directory)
        result[relative] = ('dir', info.st_mode, info.st_dev, info.st_ino, info.st_uid, info.st_gid)
        before_names = names(directory)
        for name in before_names:
            budget()
            child = relative + '/' + name if relative else name
            require(len(result) < MAX_FILES and len(Path(child).parts) <= MAX_DEPTH)
            before = os.stat(name, dir_fd=directory, follow_symlinks=False)
            require(stat.S_ISDIR(before.st_mode) or stat.S_ISREG(before.st_mode))
            # Same explicit exclusions as rejectClaudeCacheExcludedEntries;
            # excluded package bytes never acquire ownership by snapshot equality.
            require(not (stat.S_ISDIR(before.st_mode) and name == '.git')
                    and not (stat.S_ISREG(before.st_mode)
                             and name in ('.uap-integrations.lock', '.plugin-kit-ai.lock')))
            flags = os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK
            child_fd = os.open(name, flags | (os.O_DIRECTORY if stat.S_ISDIR(before.st_mode) else 0),
                               dir_fd=directory)
            try:
                held = os.fstat(child_fd)
                require(_FILES['same_opened'](before, held))
                metadata(child_fd)
                if stat.S_ISDIR(before.st_mode):
                    visit(child_fd, child, depth + 1)
                else:
                    files += 1
                    total += before.st_size
                    require(files <= MAX_FILES and before.st_nlink == 1
                            and 0 <= before.st_size <= MAX_FILE_BYTES and total <= MAX_TREE_BYTES)
                    digest, consumed = hashlib.sha256(), 0
                    while True:
                        budget()
                        block = os.read(child_fd, min(65536, before.st_size - consumed + 1))
                        if not block:
                            break
                        consumed += len(block)
                        require(consumed <= before.st_size)
                        digest.update(block)
                    require(consumed == before.st_size
                            and _FILES['identity'](held) == _FILES['identity'](os.fstat(child_fd)))
                    result[child] = ('file', before.st_mode, before.st_dev, before.st_ino,
                                     before.st_nlink, before.st_size, before.st_mtime_ns,
                                     before.st_uid, before.st_gid, digest.hexdigest())
                metadata(child_fd)
                require(_FILES['identity'](before) == _FILES['identity'](
                    os.stat(name, dir_fd=directory, follow_symlinks=False)))
            finally:
                os.close(child_fd)
        require(names(directory) == before_names)
        metadata(directory)
        budget()
    visit(fd, '', 0)
    return result, files, total


def content(snapshot):
    return {path: (row[0], bool(row[1] & 0o111), row[5], row[-1]) if row[0] == 'file'
            else ('dir',) for path, row in snapshot.items()}



def same_json(left, right):
    # Python equality conflates True/1 and False/0 inside foreign documents.
    def canonical(value):
        return json.dumps(value, sort_keys=True, separators=(',', ':'), allow_nan=False)
    return canonical(left) == canonical(right)


def entries(fd):
    result = {}
    with os.scandir(fd) as children:
        for entry in children:
            require(len(result) < MAX_FILES)
            info = entry.stat(follow_symlinks=False)
            require(stat.S_ISDIR(info.st_mode))
            result[entry.name] = (info.st_mode, info.st_dev, info.st_ino)
    return result


def retirement_diagnostic(error):
    """Locate an authored refusal without exporting traceback contents."""
    unavailable = {"site": "unavailable"}
    try:
        directory = Path(__file__).parent
        authored = {
            str(directory / "claude_cache_retirement_audit.py"): "claude-cache-retirement",
            str(directory / "claude_windows_cache_custody.py"): "claude-windows-cache-custody",
            str(directory / "gemini_windows_custody.py"): "gemini-windows-custody",
        }
        current, selected = error.__traceback__, unavailable
        for _ in range(64):
            if current is None:
                return selected
            code = current.tb_frame.f_code
            module = authored.get(code.co_filename)
            line = current.tb_lineno
            if (module is not None and code.co_name not in ("require", "_refuse")
                    and type(line) is int and line > 0):
                selected = {"module": module, "line": line}
            current = current.tb_next
        return selected if current is None else unavailable
    except BaseException:
        return unavailable


class CacheRetirement:
    def __init__(self, case, home, project, public, added, source, read_document, snapshotter, before_native):
        self.fds = []
        self.windows = None
        try:
            self.case, self.profile, self.project = case, home / '.claude', project
            self.read_document, self.snapshotter, self.before_native = read_document, snapshotter, before_native
            state = read_document(project / '.uap/state-v2.json')
            require(type(state.get('schema_version')) is int and state['schema_version'] == 5)
            records = [row for row in state['installations'] if row['installation_id'] == public['installation_id']]
            require(len(records) == 1)
            record = records[0]
            require(record['declared_name'] == record['package']['declared_name'] == public['name']
                    and record['source']['repository'] == source['repo']
                    and record['source']['package_subpath'] == 'skills/' + source['folder']
                    and record['source']['repository'].strip('/') + '/'
                        + record['source']['package_subpath'].strip('/') == public['source'])
            public_clients = [v for v in public['clients'] if v['client_id'] == 'claude' and v['scope'] == 'project']
            bindings = [v for v in record['clients'].values() if v['client_id'] == 'claude' and v['scope'] == 'project']
            require(len(public_clients) == len(public['clients']) == len(bindings) == len(record['clients']) == 1)
            binding = bindings[0]
            self.installation_id, self.binding_id = record['installation_id'], binding['client_binding_id']
            require(record['clients'].get(binding['client_binding_id']) == binding)
            for key, expected in {'materialization': 'materialized', 'activation': 'active',
                    'verification': 'installation_verified', 'authentication': 'not_required', 'policy': 'allowed'}.items():
                require(binding[key] == public_clients[0][key] == expected)
            revision = public_clients[0]['package_revision']
            require(record['source']['resolved_revision'] == revision['resolved_revision']
                    and record['source']['tree_digest'] == revision['tree_digest']
                    and record['package']['manifest_digest'] == revision['manifest_digest'])
            rows = added['items']
            require(len(rows) == 1 and rows[0]['targetId'] == 'claude')
            self.active = Path(rows[0]['destinationPath'])
            require(self.active == project / '.claude/skills' / binding['physical_artifact_id']
                    and binding['target_locator'] == str(self.active))
            objects = [v for v in binding['native_objects'] if v['kind'] == 'claude_project_plugin_cache']
            require(len(objects) == 1)
            receipt = objects[0]
            parts = Path(receipt['source_relative']).parts
            require(len(parts) == 5 and parts[:2] == ('plugins', 'cache')
                    and re.fullmatch(r'agentplugins-[0-9a-f]{12}', parts[2])
                    and parts[3] == record['declared_name'] and parts[4] not in ('.', '..')
                    and receipt['source_relative'] == '/'.join(parts)
                    and parts[4] == (record['package'].get('version') or '0.0.0'))
            self.market, self.name = parts[2], parts[3]
            self.key = self.name + '@' + self.market
            require(receipt['logical_name'] == self.name
                    and receipt['object_id'] == 'claude-project-cache:' + self.key
                    and receipt['path'] == str(self.active / '.agentplugins-plugin')
                    and receipt['protection_class'] == 'managed' and not receipt.get('user_modified', False)
                    and re.fullmatch(r'sha256:[0-9a-f]{64}', receipt['managed_digest']))
            self.receipt = {k: receipt[k] for k in ('object_id', 'source_relative', 'managed_digest')}
            self.cache = self.profile.joinpath(*parts)
            require('metadata:0:' + '/'.join(parts) not in before_native)
            self.parent = self.cache.parent
            if _WINDOWS is not None:
                self.windows = _WINDOWS['Backend'](
                    case, self.profile, self.parent, self.cache, self.active / '.agentplugins-plugin')
                self.before = self.windows.before
                self.file_count, self.byte_count = self.windows.file_count, self.windows.byte_count
            else:
                self.anchors = {}
                for path in (self.profile, *list(self.parent.parents)[:3], self.parent):
                    require(path == self.profile or path.is_relative_to(self.profile))
                    info = ordinary(path)
                    self.anchors[path] = (info.st_mode, info.st_dev, info.st_ino, info.st_uid, info.st_gid)
                self.parent_fd = open_directory(self.parent); self.fds.append(self.parent_fd)
                self.cache_fd = open_directory(self.cache); self.fds.append(self.cache_fd)
                self.before_siblings = entries(self.parent_fd)
                self.before, self.file_count, self.byte_count = tree(self.cache_fd)
                require(self.file_count > 0)
                require('.orphaned_at' not in self.before)
                bundle_fd = open_directory(self.active / '.agentplugins-plugin')
                try:
                    bundle, _, _ = tree(bundle_fd)
                    require(content(bundle) == content(self.before))
                finally:
                    os.close(bundle_fd)
            self.installed = read_document(self.profile / 'plugins/installed_plugins.json')
            self.markets = read_document(self.profile / 'plugins/known_marketplaces.json')
            require(type(self.installed.get('version')) is int and self.installed['version'] == 2)
            selected = {k: v for k, v in self.installed['plugins'].items() if k.endswith('@' + self.market)}
            require(set(selected) == {self.key} and len(selected[self.key]) == 1)
            row = selected[self.key][0]
            require(row['scope'] == 'project' and row['projectPath'] == str(project)
                    and row['installPath'] == str(self.cache))
            registered = self.markets[self.market]
            require(registered['source'] == {'source': 'directory', 'path': str(self.active)}
                    and registered['installLocation'] == str(self.active))
            self.guard()
        except BaseException:
            self.close()
            raise

    def close(self):
        if self.windows is not None:
            self.windows.close()
        while self.fds:
            os.close(self.fds.pop())

    def guard(self):
        if self.windows is not None:
            self.windows.guard()
            return
        for path, original in self.anchors.items():
            info = ordinary(path)
            require((info.st_mode, info.st_dev, info.st_ino, info.st_uid, info.st_gid) == original)
        named_directory(self.parent, self.parent_fd)

    def finish(self, residuals, cleanup):
        try:
            require(cleanup['schema_version'] == 1 and cleanup['result'] == 'success'
                    and cleanup['outcome'] == 'completed' and cleanup['scope'] == 'project')
            actions = cleanup['actions']
            require(len(actions) == 1 and actions[0]['installation_id'] == self.installation_id
                    and actions[0]['binding_id'] == self.binding_id and actions[0]['client_id'] == 'claude'
                    and actions[0]['action'] == 'remove')
            self.guard()
            if self.windows is not None:
                quarantine, moved, after, marker = self.windows.retired()
            else:
                require(not self.cache.exists() and not self.cache.is_symlink())
                after_siblings = entries(self.parent_fd)
                retained = set(after_siblings) - set(self.before_siblings)
                require(len(retained) == 1)
                nonce = retained.pop()
                require(re.fullmatch(r'\.agentplugins-cache-retire-[0-9a-f]{32}', nonce))
                require({k: v for k, v in self.before_siblings.items() if k != self.cache.name}
                        == {k: v for k, v in after_siblings.items() if k != nonce})
                quarantine = self.parent / nonce
                quarantine_fd = open_directory(quarantine); self.fds.append(quarantine_fd)
                info = os.fstat(quarantine_fd)
                require(stat.S_IMODE(info.st_mode) == 0o700 and info.st_uid == os.geteuid()
                        and set(entries(quarantine_fd)) == {'retired'})
                moved = quarantine / 'retired'
                named_directory(moved, self.cache_fd)
                after, _, _ = tree(self.cache_fd)
                marker = after.pop('.orphaned_at', None)
                if marker is not None:
                    require(marker[0] == 'file' and marker[5] == 13)
                    fd = os.open('.orphaned_at', os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK,
                                 dir_fd=self.cache_fd)
                    try:
                        held = os.fstat(fd)
                        metadata(fd)
                        require(stat.S_ISREG(held.st_mode) and held.st_nlink == 1
                                and (held.st_dev, held.st_ino, held.st_size) == (marker[2], marker[3], 13))
                        body = os.read(fd, 14)
                        metadata(fd)
                        require(_FILES['identity'](held) == _FILES['identity'](os.fstat(fd))
                                == _FILES['identity'](os.stat('.orphaned_at', dir_fd=self.cache_fd, follow_symlinks=False)))
                        require(re.fullmatch(rb'[1-9][0-9]{12}', body)
                                and hashlib.sha256(body).hexdigest() == marker[-1])
                    finally:
                        os.close(fd)
                require(after == self.before)
            installed = self.read_document(self.profile / 'plugins/installed_plugins.json')
            markets = self.read_document(self.profile / 'plugins/known_marketplaces.json')
            require(same_json(installed, dict(self.installed, plugins={k: v for k, v in self.installed['plugins'].items()
                    if k != self.key})) and same_json(markets, {k: v for k, v in self.markets.items() if k != self.market}))
            current = self.snapshotter((self.profile,))
            require(all(current.get(path) == value for path, value in self.before_native.items()))
            if self.windows is not None:
                # Match snapshot's original Windows keys; never normalize raw evidence.
                exact_files = {'.claude/' + str(moved.relative_to(self.profile) / Path(path))
                               for path, row in after.items() if row[0] == 'file'}
            else:
                prefix = '.claude/' + moved.relative_to(self.profile).as_posix() + '/'
                exact_files = {prefix + path for path, row in after.items() if row[0] == 'file'}
            require(residuals and all(path in exact_files for path in residuals))
            # Recheck the complete retained tree after native-registry/foreign reads.
            if self.windows is not None:
                self.windows.recheck(quarantine, moved, marker)
            else:
                stable, _, _ = tree(self.cache_fd)
                if marker is not None:
                    require(stable.pop('.orphaned_at', None) == marker)
                require(stable == self.before)
                self.guard(); named_directory(quarantine, quarantine_fd); named_directory(moved, self.cache_fd)
                final = os.fstat(quarantine_fd)
                require((final.st_mode, final.st_uid) == (info.st_mode, info.st_uid))
                require(not self.cache.exists() and not self.cache.is_symlink())
            return {'status': 'verified-cache-retirement-at-checkpoint',
                    'original': self.cache.relative_to(self.case).as_posix(),
                    'retired': moved.relative_to(self.case).as_posix(), 'receipt': self.receipt,
                    'file_count': self.file_count, 'bytes': self.byte_count,
                    'snapshot_equality_sha256': hashlib.sha256(json.dumps(self.before, sort_keys=True).encode()).hexdigest(),
                    **(self.windows.proof() if self.windows is not None else {
                        'cache_device': os.fstat(self.cache_fd).st_dev,
                        'cache_inode': os.fstat(self.cache_fd).st_ino}),
                    'orphaned_marker_present': marker is not None, 'writer_closure_claimed': False,
                    'matched_residual_count': len(residuals)}
        except (OSError, ValueError, TypeError, KeyError) as error:
            return {'status': 'unresolved-cache-retirement',
                    'diagnostic': retirement_diagnostic(error)}
        finally:
            self.close()
