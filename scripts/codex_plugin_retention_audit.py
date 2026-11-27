"""Exact-object checkpoint proof for Linux Codex 0.162 native retention.

No catalog writer attribution, startup-sync completion or writer closure is
claimed. Every raw finding remains evidence; uncertain custody refuses proof.
The five-second filesystem budgets are cooperative, not syscall preemption.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys
import time
import tomllib

FILE_LIMIT = 64 << 20
TOTAL_LIMIT = 256 << 20
# One shared byte pool for the caller-qualified dependency scope.
RUNTIME_TOTAL_LIMIT = 256 << 20
MEMBER_LIMIT = 10000
# One separately qualified dependency subtree; never an ownership grant.
RUNTIME_MEMBER_LIMIT = 24576
DEPTH_LIMIT = 64
FD_LIMIT = 2 * MEMBER_LIMIT + 3 * DEPTH_LIMIT + 1
SECONDS_LIMIT = 5
CHECKPOINT_SCOPE = {"atomic_snapshot_claimed": False, "writer_closure_claimed": False,
                    "semantics": "sequential-per-object-checkpoints"}


class Budget:
    def __init__(self):
        self.deadline = time.monotonic() + SECONDS_LIMIT

    def check(self):
        if time.monotonic() >= self.deadline:
            raise ValueError("retention cooperative filesystem deadline exceeded")


class Descriptors:
    """One aggregate quota for both retained trees and transient authority reads."""
    def __init__(self, *, runtime_envelope=False):
        if type(runtime_envelope) is not bool:
            raise ValueError("retention runtime descriptor envelope must be explicit")
        self.runtime_envelope = runtime_envelope
        self.owned = set()
        self.scans = 0

    @property
    def limit(self):
        # Retain the existing shared-tree/ancestor reserve plus one finite scope.
        return FD_LIMIT + (RUNTIME_MEMBER_LIMIT if self.runtime_envelope else 0)

    def open(self, path, flags, *, parent=None, budget):
        budget.check()
        if len(self.owned) + self.scans >= self.limit:
            raise ValueError("retention aggregate descriptor quota exceeded")
        fd = os.open(path, flags, dir_fd=parent)
        self.owned.add(fd)
        try:
            budget.check()
        except BaseException:
            self.close_all([fd])
            raise
        return fd

    def close_all(self, descriptors):
        first = None
        for fd in tuple(descriptors):
            if fd not in self.owned:
                continue
            self.owned.remove(fd)
            try:
                os.close(fd)
            except OSError as error:
                if first is None:
                    first = error
        if first is not None:
            current = sys.exception()
            if current is not None:
                current.add_note("retention descriptor cleanup also failed: " + str(first))
            else:
                raise first


def _object(info):
    return (info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid)


def _identity(info):
    return (*_object(info), info.st_nlink, info.st_size,
            info.st_mtime_ns, info.st_ctime_ns)


def _unique(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate retention authority key")
        result[key] = value
    return result


def _read_regular(fd, before, budget, *, collect=False, limit=None):
    limit = FILE_LIMIT if limit is None else limit
    if (not stat.S_ISREG(before.st_mode) or before.st_nlink != 1
            or not 0 <= before.st_size <= limit):
        raise ValueError("retention leaf is not bounded exclusively linked regular data")
    budget.check()
    os.lseek(fd, 0, os.SEEK_SET)
    digest, chunks, size = hashlib.sha256(), [], 0
    while True:
        budget.check()
        block = os.read(fd, min(65536, before.st_size + 1 - size))
        budget.check()
        if not block:
            break
        size += len(block)
        if size > before.st_size:
            raise ValueError("retention file grew during read")
        digest.update(block)
        if collect:
            chunks.append(block)
    if size != before.st_size or _identity(os.fstat(fd)) != _identity(before):
        raise ValueError("retention file changed during full read")
    budget.check()
    result = {"bytes": size, "sha256": digest.hexdigest()}
    return (result, b"".join(chunks)) if collect else result


def _runtime_prefixes(values):
    """Resource scopes only: the caller must establish receipt/source authority."""
    if type(values) not in (tuple, list, set, frozenset) or len(values) > 1:
        raise ValueError("retention permits at most one qualified runtime prefix")
    for value in values:
        if (type(value) is not str or not re.fullmatch(r"[A-Za-z0-9_.@/-]{1,4096}", value)
                or Path(value).is_absolute() or Path(value).as_posix() != value
                or any(part in ("", ".", "..") for part in value.split("/"))
                or value.split("/")[-1] != "node_modules" or len(value.split("/")) > DEPTH_LIMIT):
            raise ValueError("retention runtime prefix is not canonical node_modules authority")
    return tuple(values)


class HeldTree:
    """Hold every original object; callers qualify any extended runtime scope."""

    def __init__(self, root, *, bin_links=False, quota=None, budget=None, runtime_prefixes=()):
        self.root, self.bin_links = Path(root), bin_links
        self.runtime_prefixes = _runtime_prefixes(runtime_prefixes)
        self.ordinary_members, self.runtime_members = 0, 0
        self.ordinary_total, self.runtime_total = 0, 0
        self.ordinary_limit = TOTAL_LIMIT
        self.runtime_limit = RUNTIME_TOTAL_LIMIT if self.runtime_prefixes else 0
        self.quota = quota if quota is not None else Descriptors(runtime_envelope=bool(self.runtime_prefixes))
        budget = budget if budget is not None else Budget()
        self.fds, self.nodes, self.ancestors, self.total = {}, {}, {}, 0
        try:
            budget.check()
            if (not self.root.is_absolute() or len(self.root.parents) > DEPTH_LIMIT
                    or self.root.resolve(strict=True) != self.root):
                raise ValueError("retention root is redirected or too deep")
            for parent in reversed(self.root.parents):
                fd = self.quota.open(parent, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW,
                                     budget=budget)
                self.fds[("ancestor", str(parent))] = fd
                self.ancestors[parent] = _object(os.fstat(fd))
            self._capture(".", None, str(self.root), 0, budget)
            if any(self.nodes.get(prefix, (None,))[0] != "directory" for prefix in self.runtime_prefixes):
                raise ValueError("retention runtime prefix is not a captured directory")
            self.verify(budget=budget)
            budget.check()
        except BaseException:
            self.close()
            raise

    def close(self):
        descriptors = tuple(self.fds.values())
        self.fds.clear()
        self.quota.close_all(descriptors)

    def _named(self, relative):
        if relative == ".":
            return self.root.lstat()
        parent = str(Path(relative).parent)
        return os.stat(Path(relative).name, dir_fd=self.fds[parent],
                       follow_symlinks=False)

    def _names(self, fd, budget):
        budget.check()
        # CPython scandir(fd) owns one temporary duplicate descriptor.
        if len(self.quota.owned) + self.quota.scans >= self.quota.limit:
            raise ValueError("retention directory scan descriptor quota exceeded")
        self.quota.scans += 1
        try:
            names = []
            with os.scandir(fd) as entries:
                for entry in entries:
                    budget.check()
                    names.append(entry.name)
                    if len(names) > MEMBER_LIMIT:
                        raise ValueError("retention directory exceeds member bound")
            budget.check()
            return tuple(sorted(names))
        finally:
            self.quota.scans -= 1

    def _capture(self, relative, parent, name, depth, budget):
        budget.check()
        self._claim_member(relative)
        if depth > DEPTH_LIMIT:
            raise ValueError("retention tree exceeds bound")
        before = os.stat(name, dir_fd=parent, follow_symlinks=False)
        mode = before.st_mode
        if stat.S_ISLNK(mode):
            if not self.bin_links or Path(relative).parent.name != ".bin":
                raise ValueError("retention symlink is not an npm bin member")
            fd = self.quota.open(name, os.O_PATH | os.O_NOFOLLOW, parent=parent,
                                 budget=budget)
            self.fds[relative] = fd
            target = os.readlink(name, dir_fd=parent)
            self.nodes[relative] = ("link", _identity(before), target)
        else:
            flags = os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK
            if stat.S_ISDIR(mode):
                flags |= os.O_DIRECTORY
            elif not stat.S_ISREG(mode):
                raise ValueError("retention special file refused")
            fd = self.quota.open(name, flags, parent=parent, budget=budget)
            self.fds[relative] = fd
            if _identity(os.fstat(fd)) != _identity(before):
                raise ValueError("retention opened another object")
            if stat.S_ISDIR(mode):
                names = self._names(fd, budget)
                self.nodes[relative] = ("directory", _identity(before), names)
                for child in names:
                    member = child if relative == "." else relative + "/" + child
                    self._capture(member, fd, child, depth + 1, budget)
                if self._names(fd, budget) != names:
                    raise ValueError("retention members changed during capture")
            else:
                scoped = self._runtime_member(relative)
                used = self.runtime_total if scoped else self.ordinary_total
                limit = self.runtime_limit if scoped else self.ordinary_limit
                if used + before.st_size > limit:
                    error = ValueError("retention qualified runtime bytes exceed bound" if scoped
                                       else "retention total bytes exceed bound")
                    # Partial observations only; the caller can diagnose the
                    # existing refusal without another filesystem traversal.
                    error.retention_byte_checkpoint = (self, relative, before.st_size, limit,
                                                       "qualified-runtime" if scoped else "ordinary")
                    raise error
                digest = _read_regular(fd, before, budget)
                self.total += digest["bytes"]
                if scoped:
                    self.runtime_total += digest["bytes"]
                else:
                    self.ordinary_total += digest["bytes"]
                self.nodes[relative] = ("file", _identity(before), digest)
        if (_identity(os.fstat(fd)) != _identity(before)
                or _identity(self._named(relative)) != _identity(before)):
            raise ValueError("retention object changed during capture")
        budget.check()

    def _runtime_member(self, relative):
        return any(relative == prefix or relative.startswith(prefix + "/") for prefix in self.runtime_prefixes)

    def _claim_member(self, relative):
        if self._runtime_member(relative):
            if self.runtime_members >= RUNTIME_MEMBER_LIMIT:
                raise ValueError("retention qualified runtime exceeds fixed member bound")
            self.runtime_members += 1
        else:
            if self.ordinary_members >= MEMBER_LIMIT:
                raise ValueError("retention tree exceeds bound")
            self.ordinary_members += 1

    def _link_target(self, relative, target):
        if not target or Path(target).is_absolute() or target.endswith(("/", "/.")):
            raise ValueError("retention npm link is not relative")
        parts = list(Path(relative).parent.parts)
        segments = target.split("/")
        for index, part in enumerate(segments):
            if part in ("", "."):
                continue
            if part == "..":
                # The path at this traversal point must already have been
                # proved a physical directory before a parent step is applied.
                current = "/".join(parts) or "."
                if not parts or self.nodes.get(current, (None,))[0] != "directory":
                    raise ValueError("retention npm parent traversal is unqualified")
                parts.pop()
            else:
                parts.append(part)
                current = "/".join(parts)
                kind = self.nodes.get(current, (None,))[0]
                later = any(segment not in ("", ".") for segment in segments[index + 1:])
                if kind != ("directory" if later else "file"):
                    raise ValueError("retention npm link crosses missing/nonphysical member")
        joined = "/".join(parts)
        if self.nodes.get(joined, (None,))[0] != "file":
            raise ValueError("retention npm link target is not one-hop regular data")
        return joined

    def _selected(self, selected):
        if selected is None:
            return set(self.nodes)
        relevant = {"."}
        for leaf in selected:
            relevant.add(leaf)
            relevant.update(str(parent) for parent in Path(leaf).parents)
        return relevant

    def guard(self, *, complete=True, selected=None, budget=None):
        """Closing named/held identity and membership sweep, without byte reads."""
        budget = budget if budget is not None else Budget()
        relevant = self._selected(selected)
        for path, original in self.ancestors.items():
            budget.check()
            fd = self.fds[("ancestor", str(path))]
            if (_object(path.lstat()) != original or _object(os.fstat(fd)) != original):
                raise ValueError("retention ancestor changed")
        for relative in sorted(relevant, key=lambda value: (value.count("/"), value)):
            budget.check()
            kind, identity, value = self.nodes[relative]
            current = self._named(relative)
            held = os.fstat(self.fds[relative])
            named_path = self.root if relative == "." else self.root / relative
            if _identity(named_path.lstat()) != _identity(current):
                raise ValueError("retention named ancestry changed")
            if kind == "directory" and not complete:
                if _object(current) != identity[:5] or _object(held) != identity[:5]:
                    raise ValueError("retention directory replaced")
            elif _identity(current) != identity or _identity(held) != identity:
                raise ValueError("retention member identity changed")
            if kind == "directory" and complete:
                if self._names(self.fds[relative], budget) != value:
                    raise ValueError("retention directory members changed")
            elif kind == "link":
                if os.readlink(named_path) != value:
                    raise ValueError("retention link changed")
                self._link_target(relative, value)
        # Close with leaves before their containing directories, then physical
        # ancestors. Membership and metadata catch later-in-pass mutation.
        for relative in sorted(relevant, key=lambda value: (value.count("/"), value), reverse=True):
            budget.check()
            kind, identity, value = self.nodes[relative]
            if kind == "directory" and complete:
                if self._names(self.fds[relative], budget) != value:
                    raise ValueError("retention closing members changed")
            current = self._named(relative)
            named_path = self.root if relative == "." else self.root / relative
            if _identity(named_path.lstat()) != _identity(current):
                raise ValueError("retention closing named ancestry changed")
            held = os.fstat(self.fds[relative])
            partial = kind == "directory" and not complete
            if ((_object(current) if partial else _identity(current))
                    != (identity[:5] if partial else identity)
                    or (_object(held) if partial else _identity(held))
                    != (identity[:5] if partial else identity)):
                raise ValueError("retention closing named object changed")
        for path, original in reversed(tuple(self.ancestors.items())):
            budget.check()
            descriptor = self.fds[("ancestor", str(path))]
            if (_object(path.lstat()) != original
                    or _object(os.fstat(descriptor)) != original):
                raise ValueError("retention closing ancestor changed")
        budget.check()

    def verify(self, *, complete=True, selected=None, budget=None):
        budget = budget if budget is not None else Budget()
        relevant = self._selected(selected)
        self.guard(complete=complete, selected=selected, budget=budget)
        for relative in relevant:
            budget.check()
            kind, identity, value = self.nodes[relative]
            if kind != "file":
                continue
            before = self._named(relative)
            if _identity(before) != identity:
                raise ValueError("retention file replaced before read")
            if _read_regular(self.fds[relative], before, budget) != value:
                raise ValueError("retention full bytes changed")
            if _identity(self._named(relative)) != identity:
                raise ValueError("retention named leaf changed after read")
        self.guard(complete=complete, selected=selected, budget=budget)
        budget.check()

    def matches(self, path, reason):
        try:
            relative = str(path.relative_to(self.root))
        except ValueError:
            return False
        node = self.nodes.get(relative)
        return bool(node and ((node[0] == "file" and reason in ("", "unscanned-too-large"))
                             or (node[0] == "link" and reason == "symlink")))

    def evidence(self):
        return {"members": len(self.nodes), "fully_read_bytes": self.total,
                "files": [{"path": name, **node[2]} for name, node in sorted(self.nodes.items())
                          if node[0] == "file"]}



class HeldAbsence:
    """Hold the nearest physical directory and the exact missing-component suffix."""
    def __init__(self, target, *, quota, budget):
        self.target, self.quota = Path(target), quota
        self.anchors, self.missing = [], None
        try:
            budget.check()
            if (not self.target.is_absolute() or ".." in self.target.parts
                    or len(self.target.parents) > DEPTH_LIMIT
                    or str(self.target) != str(target)):
                raise ValueError("retention absent namespace path is unqualified")
            parent, path = None, None
            for index, component in enumerate(self.target.parts):
                budget.check()
                path = Path(component) if path is None else path / component
                try:
                    before = os.stat(component, dir_fd=parent, follow_symlinks=False)
                except FileNotFoundError:
                    if parent is None:
                        raise ValueError("retention absence has no physical parent")
                    self.missing = self.target.parts[index:]
                    break
                if not stat.S_ISDIR(before.st_mode):
                    raise ValueError("retention absence parent is redirected or non-directory")
                descriptor = quota.open(component, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW,
                                        parent=parent, budget=budget)
                self.anchors.append((path, descriptor, _object(before)))
                if _object(os.fstat(descriptor)) != _object(before):
                    raise ValueError("retention absence parent replaced during open")
                parent = descriptor
            if self.missing is None:
                raise ValueError("retention absent namespace already exists")
            self.guard(budget=budget)
            budget.check()
        except BaseException:
            self.close()
            raise

    def close(self):
        descriptors = [descriptor for _, descriptor, _ in self.anchors]
        self.anchors.clear()
        self.quota.close_all(descriptors)

    def guard(self, *, budget):
        def anchors():
            for path, descriptor, identity in reversed(self.anchors):
                budget.check()
                if (_object(path.lstat()) != identity
                        or _object(os.fstat(descriptor)) != identity):
                    raise ValueError("retention absence parent identity changed")
        anchors()
        budget.check()
        try:
            os.stat(self.missing[0], dir_fd=self.anchors[-1][1], follow_symlinks=False)
        except FileNotFoundError:
            pass
        else:
            raise ValueError("retention absent namespace suffix appeared")
        anchors()
        budget.check()

    def evidence(self):
        return {"status": "exact-namespace-absent-at-checkpoint",
                "nearest_existing_parent": str(self.anchors[-1][0]),
                "missing_components": list(self.missing)}

def _read(path, quota, budget):
    fds, anchors = [], {}
    try:
        budget.check()
        if (not path.is_absolute() or len(path.parents) > DEPTH_LIMIT
                or path.resolve(strict=True) != path):
            raise ValueError("retention authority path is redirected or too deep")
        for parent in reversed(path.parents):
            fd = quota.open(parent, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW,
                            budget=budget)
            fds.append(fd)
            anchors[parent] = (_object(os.fstat(fd)), fd)
        before = path.lstat()
        fd = quota.open(path.name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK,
                        parent=fds[-1], budget=budget)
        fds.append(fd)
        if _identity(os.fstat(fd)) != _identity(before):
            raise ValueError("retention authority file replaced")
        _, body = _read_regular(fd, before, budget, collect=True, limit=4 << 20)
        if (_identity(path.lstat()) != _identity(before)
                or _identity(os.stat(path.name, dir_fd=fds[-2], follow_symlinks=False))
                != _identity(before)):
            raise ValueError("retention authority changed after read")
        for parent, (original, descriptor) in anchors.items():
            budget.check()
            if (_object(parent.lstat()) != original
                    or _object(os.fstat(descriptor)) != original):
                raise ValueError("retention authority ancestor replaced")
        budget.check()
        return body
    finally:
        quota.close_all(fds)
        if sys.exception() is None:
            budget.check()


def _same(left, right):
    """Canonical JSON equality preserves bool/int/float and nested type distinctions."""
    return json.dumps(left, sort_keys=True, separators=(",", ":"), allow_nan=False) == json.dumps(
        right, sort_keys=True, separators=(",", ":"), allow_nan=False)


def owned_namespace(profile, data, public, identity, quota, budget, *, selector, expected_pin=None):
    """Join the existing public projection, state binding and native TOML."""
    state_body = _read(data / "state-v2.json", quota, budget)
    state = json.loads(state_body, object_pairs_hook=_unique)
    if (type(state) is not dict or type(state.get("schema_version")) is not int
            or state["schema_version"] != 5):
        raise ValueError("retention requires current typed state")
    if (type(state.get("installations")) is not list
            or any(type(item) is not dict for item in state["installations"])
            or type(public) is not dict or type(public.get("installation_id")) is not str
            or not public["installation_id"]):
        raise ValueError("retention installation authority shape differs")
    records = [item for item in state["installations"]
               if item.get("installation_id") == public["installation_id"]]
    if len(records) != 1 or public.get("name") != identity:
        raise ValueError("retention public installation is ambiguous")
    for flag in ("needs_rebind", "mixed_version"):
        if flag in public and public[flag] is not False:
            raise ValueError("retention public lifecycle flag is unsettled or untyped")
    record = records[0]
    if record.get("declared_name") != identity or record.get("needs_rebind", False) is not False:
        raise ValueError("retention state identity is not settled")
    bindings = record["clients"]
    public_clients = public["clients"]
    if (type(bindings) is not dict or len(bindings) != 1
            or type(public_clients) is not list or len(public_clients) != 1
            or any(type(value) is not dict for value in bindings.values())
            or type(public_clients[0]) is not dict):
        raise ValueError("retention needs one exact Codex binding")
    binding_id, binding = next(iter(bindings.items()))
    client = public_clients[0]
    for key, expected in (("client_id", "codex"), ("scope", "user"),
                          ("materialization", "materialized"), ("activation", "active"),
                          ("verification", "installation_verified"), ("policy", "allowed")):
        if binding.get(key) != expected or client.get(key) != expected:
            raise ValueError("retention lifecycle binding is unqualified")
    if (not isinstance(binding_id, str) or not binding_id
            or binding.get("client_binding_id") != binding_id
            or binding.get("native_profile_root") != str(profile)):
        raise ValueError("retention binding profile differs")
    revision = binding["package_revision"]
    visible_revision = client["package_revision"]
    source, package = record["source"], record["package"]
    if any(type(value) is not dict for value in (revision, visible_revision, source, package)):
        raise ValueError("retention source/revision authority shape differs")
    for value in (revision, visible_revision):
        if (any(key in value and type(value[key]) is not str for key in (
                "version", "resolved_revision", "tree_digest", "manifest_digest", "distribution_id"))
                or ("release_sequence" in value and (type(value["release_sequence"]) is not int
                                                     or value["release_sequence"] < 0))):
            raise ValueError("retention revision fields are not typed")
    if (any(type(value) is not str for value in source.values())
            or any(key in package and type(package[key]) is not str for key in (
                "version", "declared_name", "manifest_digest"))):
        raise ValueError("retention source/package fields are not typed")
    # These are proof-scope restrictions, not assertions that optional domain
    # fields are universally required. Existing matching_installations qualifies
    # the reviewed direct/signed Git lane; URL/archive/local forms stay unresolved.
    if (source.get("source_kind", "") not in ("", "git")
            or not source.get("repository")
            or re.fullmatch(r"[0-9a-f]{40}", source.get("resolved_revision", "")) is None):
        raise ValueError("retention source is outside the reviewed immutable Git lane")
    if (revision.get("resolved_revision") != source["resolved_revision"]
            or revision.get("tree_digest") != source.get("tree_digest")
            or revision.get("manifest_digest") != package.get("manifest_digest")
            or package.get("declared_name") != identity
            or revision.get("version", "") != package.get("version", "")):
        raise ValueError("retention immutable source/package join differs")
    for key in ("tree_digest", "manifest_digest"):
        if re.fullmatch(r"sha256:[0-9a-f]{64}", revision.get(key, "")) is None:
            raise ValueError("retention immutable digest missing")
    for key in ("version", "resolved_revision", "tree_digest", "manifest_digest",
                "distribution_id", "release_sequence"):
        if not _same(revision.get(key), visible_revision.get(key)):
            raise ValueError("retention public revision differs from binding")
    expected_source = source["repository"].strip("/")
    if source.get("package_subpath"):
        expected_source += "/" + source["package_subpath"].strip("/")
    if public.get("source") != expected_source:
        raise ValueError("retention public source differs from typed Git source")
    if type(selector) is not str or not selector:
        raise ValueError("retention selected source is absent")
    if selector.startswith("github:"):
        if (expected_pin is not None or record.get("origin_mode", "direct") != "direct"
                or selector.rsplit("@", 1)[-1] != source["resolved_revision"]):
            raise ValueError("retention direct source selection differs")
    else:
        pin, directory, visible_directory = expected_pin, record.get("directory"), public.get("directory")
        if (type(pin) is not dict or type(directory) is not dict
                or type(visible_directory) is not dict or record.get("origin_mode") != "directory"
                or any(type(pin.get(key)) is not str or not pin[key] for key in (
                    "distribution", "repository", "path", "revision", "tree_digest", "manifest_digest"))
                or type(pin.get("sequence")) is not int or pin["sequence"] < 1
                or type(directory.get("product_id")) is not str or not directory["product_id"]
                or type(directory.get("distribution_kind")) is not str
                or directory["distribution_kind"] not in ("upstream", "community_bridge", "community")
                or type(directory.get("desired_release_sequence")) is not int
                or type(revision.get("release_sequence")) is not int
                or type(visible_directory.get("recorded_release_sequence")) is not int):
            raise ValueError("retention signed source authority is not typed")
        if (any(key in directory and type(directory[key]) is not str for key in (
                "product_id", "distribution_id", "distribution_kind", "feed_id", "snapshot_digest"))
                or any(key in directory and (type(directory[key]) is not int or directory[key] < 0)
                       for key in ("snapshot_schema", "snapshot_sequence"))):
            raise ValueError("retention Directory origin fields are not typed")
        for actual, expected in (
                (directory.get("product_id"), visible_directory.get("product_id")),
                (directory.get("distribution_id"), pin["distribution"]),
                (directory.get("desired_release_sequence"), pin["sequence"]),
                (revision.get("distribution_id"), pin["distribution"]),
                (revision.get("release_sequence"), pin["sequence"]),
                (visible_directory.get("recorded_distribution"), pin["distribution"]),
                (visible_directory.get("recorded_release_sequence"), pin["sequence"]),
                (visible_directory.get("recorded_revision"), pin["revision"]),
                (source["repository"], pin["repository"]),
                (source.get("package_subpath", ""), pin["path"]),
                (source["resolved_revision"], pin["revision"]),
                (revision["tree_digest"], pin["tree_digest"]),
                (revision["manifest_digest"], pin["manifest_digest"])):
            if not _same(actual, expected):
                raise ValueError("retention signed Directory/source pin differs")
    physical = binding["physical_artifact_id"]
    if (not isinstance(physical, str) or not physical or physical != physical.strip()
            or Path(physical).name != physical or physical in (".", "..")):
        raise ValueError("retention physical identity is unqualified")
    target = Path(binding["target_locator"])
    if (not target.is_absolute() or target.resolve(strict=True) != target
            or not target.is_relative_to(data) or not target.is_dir()):
        raise ValueError("retention managed target escapes data authority")
    receipts = binding.get("receipts")
    if (not isinstance(receipts, list) or not receipts
            or any(type(receipt) is not dict or receipt.get("client_binding_id") != binding_id
                   or receipt.get("active_path") != str(target)
                   or receipt.get("phase") not in ("committed", "state_committed")
                   for receipt in receipts)):
        raise ValueError("retention binding receipt differs")
    marketplace = "agentplugins-" + hashlib.sha256(physical.encode()).hexdigest()[:12]
    config_body = _read(profile / "config.toml", quota, budget)
    config = tomllib.loads(config_body.decode())
    spec = identity + "@" + marketplace
    if (type(config.get("marketplaces")) is not dict
            or type(config.get("plugins")) is not dict):
        raise ValueError("retention native registration tables are malformed")
    entry = config["marketplaces"].get(marketplace)
    plugin = config["plugins"].get(spec)
    if (type(plugin) is not dict or plugin.get("enabled") is not True
            or type(entry) is not dict or entry.get("source_type", "local") != "local"
            or entry.get("source") != str(target)):
        raise ValueError("retention native registration differs from exact binding")
    budget.check()
    namespace = hashlib.sha256((marketplace + "\0" + identity).encode()).hexdigest()
    return profile / "plugins/data/agent-plugins" / namespace, {
        "installation_id": public["installation_id"], "client_binding_id": binding_id,
        "physical_artifact_id": physical, "marketplace": marketplace,
        "source": source, "package_revision": revision, "target_locator": str(target),
        "state_sha256": hashlib.sha256(state_body).hexdigest(),
        "config_sha256": hashlib.sha256(config_body).hexdigest()}


def residual_path(profile, raw):
    prefix = profile.name + "/"
    if not isinstance(raw, str) or not raw.startswith(prefix):
        raise ValueError("retention residual is outside selected profile")
    relative = raw[len(prefix):]
    reason = ""
    for suffix in (": symlink", ": unscanned-too-large", ": non-regular",
                   ": unreadable", ": unreadable-walk"):
        if relative.endswith(suffix):
            relative, reason = relative[:-len(suffix)], suffix[2:]
            break
    path = Path(relative)
    if (reason in ("non-regular", "unreadable", "unreadable-walk")
            or not relative or path.is_absolute() or ".." in path.parts
            or str(path) != relative or any(": " in part for part in path.parts)):
        raise ValueError("retention residual cannot be qualified")
    return profile / path, reason


def lifecycle_ready(*, installed, repeat_verified, sentinels_preserved,
                    native_uninstall, cleanup, post_list, post_list_schema_valid,
                    post_list_empty, old_ids, remaining_ids,
                    post_remove_identity_registered):
    """Retention never substitutes for the existing successful lifecycle."""
    return bool(installed and repeat_verified and sentinels_preserved
                and native_uninstall and native_uninstall.get("completed") is True
                and cleanup and cleanup["exit_code"] == 0
                and post_list and post_list["exit_code"] == 0
                and post_list_schema_valid and post_list_empty
                and old_ids.isdisjoint(remaining_ids)
                and not post_remove_identity_registered)


class CodexPluginRetention:
    def __init__(self, profile, native_version):
        if sys.platform != "linux" or native_version != "0.162.0":
            raise ValueError("retention requires genuine Linux Codex 0.162.0 initialization")
        self.profile, self.quota = profile, Descriptors()
        self.baseline = HeldTree(profile, quota=self.quota, budget=Budget())
        self.data_tree, self.absence, self.owner = None, None, None

    def close(self):
        first = None
        for tree in (self.baseline, self.data_tree, self.absence):
            if tree is None:
                continue
            try:
                tree.close()
            except OSError as error:
                if first is None:
                    first = error
        if first is not None:
            raise first

    def before_remove(self, data, public, identity, *, selector, expected_pin=None):
        budget = Budget()
        namespace, owner = owned_namespace(
            self.profile, data, public, identity, self.quota, budget,
            selector=selector, expected_pin=expected_pin)
        relative = str(namespace.relative_to(self.profile))
        if any(path == relative or path.startswith(relative + "/")
               for path in self.baseline.nodes):
            raise ValueError("selected retained data namespace existed before install")
        if namespace.exists() or namespace.is_symlink():
            self.data_tree = HeldTree(namespace, bin_links=True, quota=self.quota, budget=budget)
        else:
            self.absence = HeldAbsence(namespace, quota=self.quota, budget=budget)
        next_namespace, next_owner = owned_namespace(
            self.profile, data, public, identity, self.quota, budget,
            selector=selector, expected_pin=expected_pin)
        if next_namespace != namespace or not _same(next_owner, owner):
            raise ValueError("retention authority changed during namespace custody")
        self.owner = owner
        budget.check()

    def finish(self, residuals):
        try:
            budget = Budget()
            if self.owner is None:
                raise ValueError("retention has no joined owner")
            selected, matches = set(), []
            for raw in residuals:
                budget.check()
                path, reason = residual_path(self.profile, raw)
                authorities = []
                if self.baseline.matches(path, reason):
                    authorities.append("unchanged-preinstall-object")
                    selected.add(str(path.relative_to(self.profile)))
                if self.data_tree is not None and self.data_tree.matches(path, reason):
                    authorities.append("ordinary-remove-preserved-data")
                if len(authorities) != 1:
                    raise ValueError("retention residual has no unique exact object proof")
                matches.append({"raw": raw, "authority": authorities[0]})
            if self.absence is not None:
                self.absence.guard(budget=budget)
            self.baseline.verify(complete=False, selected=selected, budget=budget)
            if self.data_tree is not None:
                self.data_tree.verify(budget=budget)
            # Cross-authority closing sweeps catch changes to baseline objects
            # while the data tree's full bytes were being inspected.
            self.baseline.guard(complete=False, selected=selected, budget=budget)
            if self.data_tree is not None:
                self.data_tree.guard(budget=budget)
            if self.absence is not None:
                self.absence.guard(budget=budget)
            result = {"status": "verified-unchanged-or-preserved-at-checkpoint",
                      **CHECKPOINT_SCOPE,
                      "namespace_absence": self.absence.evidence() if self.absence else None,
                      "owner": self.owner, "residuals": matches,
                      "baseline": self.baseline.evidence(),
                      "preserved_data": self.data_tree.evidence() if self.data_tree else None}
            budget.check()
            return result
        except (OSError, ValueError, TypeError, KeyError):
            return {"status": "unresolved-plugin-retention", **CHECKPOINT_SCOPE}
