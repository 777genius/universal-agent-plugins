#!/usr/bin/env python3
"""Finite Devin metadata capture; no product or MCP runtime qualification."""
import argparse
import base64
from datetime import datetime
from contextlib import ExitStack
import hashlib
import json
import math
import os
from pathlib import Path
import re
import secrets
import shutil
import stat
import sys
import time

import project_native_research as shared
from pi_project_components_native_e2e import PiHTTPMCPProbe

VERSION = "3000.12.57"
ARCHIVE = "0946784ef886d716e82a8026f76933bf0a6e4f0a804a23722bb62047e8c07d4a"
BINARY = "3f3fb99f472d92f7852be3775ac2b66b5d60baf26f3441b02d54c56b160bc30d"
SPEC = {"url": "https://static.devin.ai/cli/3000.12.57/devin-3000.12.57-x86_64-unknown-linux.tar.gz",
        "integrity": "sha256:" + ARCHIVE, "expected_bytes": 72925933,
        "max_bytes": 96 << 20, "max_expanded_bytes": 256 << 20,
        "format": "tar.gz", "entrypoint": "bin/devin", "runtime": "native", "binary_sha256": BINARY}
HELP = (("version", ["--version"]), ("help", ["--help"]),
        ("mcp-help", ["mcp", "--help"]), ("mcp-list-help", ["mcp", "list", "--help"]),
        ("skills-list-help", ["skills", "list", "--help"]))


class AdmissionContext(shared.Context):
    """A nonreturning inherited runner leaves cleanup authority unproved."""
    process_cleanup_proved = True
    def command(self, *args, **kwargs):
        self.process_cleanup_proved = False
        self.summary["in_flight_command"] = args[0] if args else "unknown"
        result = super().command(*args, **kwargs)
        self.process_cleanup_proved = True
        self.summary.pop("in_flight_command", None)
        return result


def metadata_inventory(root, budget):
    """Finite descriptor-relative metadata; large runtime bytes use verify_files."""
    held = root_identity(root)
    result = {}
    for current, dirs, files, descriptor in os.fwalk(root, follow_symlinks=False):
        budget.check()
        relative = Path(current).relative_to(root)
        require(len(relative.parts) <= 32 and len(result) + len(dirs) + len(files) <= 4096, "outer metadata inventory bound")
        linked = Path(current).lstat()
        opened = os.fstat(descriptor)
        require((linked.st_dev, linked.st_ino, linked.st_mode) == (opened.st_dev, opened.st_ino, opened.st_mode)
                and stat.S_ISDIR(opened.st_mode), "outer directory identity differs")
        result[relative.as_posix()] = (opened.st_dev, opened.st_ino, opened.st_mode, opened.st_uid, opened.st_gid)
        for name in dirs + files:
            info = os.stat(name, dir_fd=descriptor, follow_symlinks=False)
            require(stat.S_ISDIR(info.st_mode) or stat.S_ISREG(info.st_mode), "outer alias/special member refused")
            require(stat.S_ISDIR(info.st_mode) or (info.st_nlink == 1 and info.st_size <= 512 << 20), "outer file alias/size refused")
            result[(relative / name).as_posix()] = (info.st_dev, info.st_ino, info.st_mode, info.st_nlink,
                info.st_uid, info.st_gid, info.st_size, info.st_mtime_ns, info.st_ctime_ns)
    require(root_identity(root) == held, "outer metadata root replaced")
    return result


def require(value, message):
    if not value:
        raise ValueError(message)


def strict_json(body):
    require(isinstance(body, bytes) and 0 < len(body) <= 1 << 20, "bounded JSON required")
    def unique(pairs):
        result = {}
        for key, value in pairs:
            require(key not in result, "duplicate native JSON key")
            result[key] = value
        return result
    value = json.loads(body.decode("utf-8", "strict"), object_pairs_hook=unique,
                       parse_constant=lambda _value: (_ for _ in ()).throw(ValueError("nonfinite JSON")))
    def visit(item, depth=0):
        require(depth <= 16, "native JSON depth")
        # Exponent-overflow JSON numbers may become +/-inf without parse_constant.
        if isinstance(item, float):
            require(math.isfinite(item), "nonfinite native JSON number")
        if isinstance(item, (dict, list)):
            require(len(item) <= 512, "native JSON cardinality")
            for child in item.values() if isinstance(item, dict) else item:
                visit(child, depth + 1)
    visit(value)
    require(isinstance(value, (dict, list)), "native JSON inventory container required")
    return value


def admitted_archive(receipt):
    members = receipt["members"]
    require(receipt["pin_verified"] and receipt["sha256"] == ARCHIVE and receipt["bytes"] == SPEC["expected_bytes"], "archive pin differs")
    require(len(members) == 250 and sum(row["kind"] == "file" for row in members) == 234
            and all(row["kind"] in ("file", "directory") for row in members), "archive closed member inventory differs")
    entry = [row for row in members if row["name"] == "bin/devin"]
    require(len(entry) == 1 and entry[0]["sha256"] == BINARY and entry[0]["bytes"] == 213734376
            and entry[0]["executable"] is True, "selected binary identity differs")
    require([row["name"] for row in members if row["kind"] == "file" and row["executable"]] == ["bin/devin"], "unreviewed executable companion")
    require(sum(row["bytes"] for row in members) == 214280841, "expanded runtime bytes differ")


def snapshot(ctx):
    roots = {"home": ctx.home, "temporary": ctx.root / "tmp", **ctx.projects}
    result = shared.legacy.inventory(roots, ctx.budget)
    require(all(row["kind"] in ("file", "directory") for row in result.values()), "fixture special/alias member")
    return result


def root_identity(path):
    require(path.is_absolute() and path.resolve() == path and not path.is_symlink(), "fixture root alias")
    info = path.lstat()
    require(stat.S_ISDIR(info.st_mode), "fixture root is not a directory")
    return (info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid)


def fixture(ctx, urls, nonce):
    original = Path(ctx.env["XDG_CONFIG_HOME"]) / "devin"
    roots = {"user": original, **{name: ctx.projects[name] / ".devin" for name in ("a", "b")}}
    startup = ctx.root / "stdio-started"
    marker = ctx.root / "stdio-effect.py"
    marker.write_text("from pathlib import Path\nPath(" + repr(str(startup)) + ").write_text('unexpected native start')\n")
    expected = {}
    for name, root in roots.items():
        root.mkdir(mode=0o700, parents=True, exist_ok=True)
        entries = {"same-name": {"url": urls[name], "disabled": False, "disabledTools": ["never-auto-approve"]},
                   "foreign-" + name: {"url": urls[name], "disabled": True},
                   "stdio-" + name: {"command": sys.executable, "args": [str(marker)], "disabled": False}}
        body = json.dumps({"mcpServers": entries}, sort_keys=True).encode() + b"\n"
        (root / "mcp_config.json").write_bytes(body)
        (root / "config.json").write_text('{"read_config_from":{"cloud":false}}\n')
        skill = root / "skills" / "same-skill" / "SKILL.md"
        skill.parent.mkdir(parents=True)
        skill.write_text("---\nname: same-skill\ndescription: Inert " + name + " " + nonce + "\n---\nPreserved source " + name + "\n")
        expected[name] = {"root": str(root), "mcp": entries,
                          "mcp_config_sha256": hashlib.sha256(body).hexdigest(), "skill": str(skill)}
    legacy = ctx.home / ".codeium" / "windsurf"
    legacy.mkdir(parents=True)
    (legacy / "mcp_config.json").write_text('{"mcpServers":{"legacy-foreign":{"url":"' + urls["legacy"] + '","disabled":true}},"sentinel":"' + nonce + '"}\n')
    (legacy / "config.json").write_text(json.dumps({"inert_legacy_sentinel": nonce}) + "\n")
    legacy_skill = legacy / "skills" / "legacy-foreign" / "SKILL.md"
    legacy_skill.parent.mkdir(parents=True)
    legacy_skill.write_text("---\nname: legacy-foreign\ndescription: Preserve legacy Windsurf skill.\n---\n" + nonce + "\n")
    local = ctx.projects["a"] / ".devin" / "mcp_config.local.json"
    body = json.dumps({"mcpServers": {"same-name": {"url": urls["local"], "disabled": True}}}).encode()
    local.write_bytes(body)
    expected["local"] = {"path": str(local), "sha256": hashlib.sha256(body).hexdigest(), "url": urls["local"]}
    expected["legacy_url"] = urls["legacy"]
    return expected, startup, marker


def zero_effects(probes, startup):
    require(not startup.exists() and not startup.is_symlink(), "native metadata launched stdio")
    for probe in probes.values():
        calls, invalid = probe.call_records()
        require(not probe.methods() and not calls and invalid == 0 and not probe.accepted_connections, "native metadata contacted MCP endpoint")


class KnownFileDiagnostics:
    """Held original parents; two independently stable, bounded file versions."""
    PATHS = {"user-config": "home/config/devin/config.json",
             "git-a-exclude": "project-a/.git/info/exclude"}

    def __init__(self, root, original_identity, baseline):
        self.root, self.stack, self.parents, self.before = root, ExitStack(), {}, {}
        self.original_bodies = {}
        try:
            require(root_identity(root) == original_identity, "diagnostic independent root differs")
            self.root_fd = self.open_directory(root)
            self.root_info = self.identity(os.fstat(self.root_fd))
            require(self.root_info == original_identity, "diagnostic opened root differs")
            for label, relative in self.PATHS.items():
                fd, chain = self.root_fd, []
                for component in Path(relative).parts[:-1]:
                    child = self.open_directory(component, fd)
                    info = os.fstat(child)
                    require(info.st_dev == self.root_info[0] and info.st_uid == self.root_info[3],
                            "diagnostic parent ownership/device differs")
                    chain.append((fd, component, child, self.identity(info)))
                    fd = child
                self.parents[label] = (fd, Path(relative).name, chain)
            self.verify()
            for label in self.PATHS:
                self.before[label] = self.read(label)
                baseline_key = "home/config/devin/config.json" if label == "user-config" else "a/.git/info/exclude"
                expected = baseline[baseline_key]
                require(expected["kind"] == "file" and self.before[label]["sha256"] == expected["sha256"]
                        and self.before[label]["bytes"] == expected["size"], "diagnostic original fixture differs")
                if label == "user-config":
                    require(self.original_bodies[label] == b'{"read_config_from":{"cloud":false}}\n',
                            "diagnostic authored User config differs")
                self.before[label]["base64"] = base64.b64encode(self.original_bodies[label]).decode()
        except BaseException:
            self.close()
            raise

    @staticmethod
    def identity(info):
        return (info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid)

    def open_directory(self, name, parent=None):
        fd = os.open(name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=parent)
        self.stack.callback(os.close, fd)
        return fd

    def verify(self):
        require(self.identity(self.root.lstat()) == self.root_info
                and self.identity(os.fstat(self.root_fd)) == self.root_info, "diagnostic original root changed")
        for _fd, _name, chain in self.parents.values():
            for parent, name, child, expected in chain:
                require(self.identity(os.fstat(child)) == expected
                        and self.identity(os.stat(name, dir_fd=parent, follow_symlinks=False)) == expected,
                        "diagnostic original parent changed")

    def read(self, label):
        self.verify()
        parent, name, _chain = self.parents[label]
        fd = os.open(name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=parent)
        self.stack.callback(os.close, fd)
        first = os.fstat(fd)
        def stable(info):
            return (*self.identity(info), info.st_nlink, info.st_size, info.st_mtime_ns, info.st_ctime_ns)
        require(stat.S_ISREG(first.st_mode) and first.st_nlink == 1 and first.st_size <= 4096
                and first.st_dev == self.root_info[0] and first.st_uid == self.root_info[3],
                "diagnostic bounded ordinary single-link file required")
        body = os.read(fd, 4097)
        require(len(body) == first.st_size and stable(os.fstat(fd)) == stable(first)
                and stable(os.stat(name, dir_fd=parent, follow_symlinks=False)) == stable(first),
                "diagnostic file changed during read")
        self.verify()
        self.original_bodies.setdefault(label, body)
        return {"identity": list(stable(first)), "bytes": len(body),
                "sha256": hashlib.sha256(body).hexdigest(), "projection": self.project(label, body)}

    def project(self, label, body):
        def opaque(item):
            encoded = json.dumps(item, sort_keys=True, separators=(",", ":")).encode()
            return {"type": type(item).__name__, "bytes": len(encoded),
                    "sha256": hashlib.sha256(encoded).hexdigest()}
        def fields(value, depth=0):
            require(depth <= 4 and isinstance(value, dict) and len(value) <= 64,
                    "diagnostic config projection bound")
            rows = []
            for key, item in sorted(value.items()):
                row = {"key": key} if re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]{0,63}", key) else {"key": opaque(key)}
                if isinstance(item, dict):
                    row["value"] = fields(item, depth + 1)
                elif item is None or type(item) is bool:
                    row["value"] = item
                else:
                    row["value"] = opaque(item)
                rows.append(row)
            return rows
        if label == "user-config":
            return {"kind": "closed-config-projection", "fields": fields(strict_json(body))}
        before = self.original_bodies[label].splitlines(keepends=True)
        lines = []
        for line in body.splitlines(keepends=True):
            text = line.decode("ascii", "replace").rstrip("\r\n")
            safe_pattern = (len(text) <= 128 and re.fullmatch(r"\.devin(?:/[A-Za-z0-9_.*?/-]*)?/?", text)
                            and ".." not in text and "//" not in text)
            row = {"bytes": len(line), "sha256": hashlib.sha256(line).hexdigest()}
            if line in before or safe_pattern:
                row["base64"] = base64.b64encode(line).decode()
            lines.append(row)
        return {"kind": "bounded-exclude-lines", "lines": lines}

    def finish(self):
        after = {label: self.read(label) for label in self.PATHS}
        self.verify()
        return {"schema_version": 1, "complete": True,
                "files": {label: {"before": self.before[label], "after": after[label]} for label in self.PATHS}}

    def close(self):
        self.stack.close()


BOOTSTRAP_DIRS = frozenset(("home/cache/devin", "home/cache/devin/cli", "home/config/devin/cli", "home/data/devin",
    "home/data/devin/cli", "home/data/devin/cli/logs", "home/data/devin/cli/plugins", "home/data/devin/mcp", "home/data/devin/mcp/oauth"))
DEFINITIONS_CACHE = "home/cache/devin/cli/unleash_definitions.bf84fa7496e6f844.bin"
DEFINITIONS_BYTES = 674340
BOOTSTRAP_FIXED = {
    DEFINITIONS_CACHE: (DEFINITIONS_BYTES, "8626d6fd4ce3381b2eb32b6c60b7ad752d88c73d8a4fb848e3576bff113393f1"),
    "home/cache/devin/telemetry_state.json": (16, "7445ff8769cba2e1b004c754a770a85293e7548228e4fa3d363263c96c9b6290"),
    "home/data/devin/cli/plugins/lock.json": (57, "cbce3a5464482eb4cfb7b152ad15183bcb5db5c5557c625ee0cec1edb582aa88"),
    "home/data/devin/cli/plugins/lock.json.lock": (0, hashlib.sha256(b"").hexdigest())}
METADATA_CONFIG = "private/metadata-config.json"
INSTALLATION_ID = "home/data/devin/cli/installation_id"
MAINTENANCE = "home/data/devin/cli/logs/.log-maintenance.lock"
BOOTSTRAP_LOG = re.compile(r"home/data/devin/cli/logs/devin_[0-9]{8}-[0-9]{6}_[1-9][0-9]{0,9}\.log\Z")


def read_bootstrap_file(ctx, relative, expected, inventory):
    """Only the prospective fixed bootstrap recipe calls this anchored reader."""
    def stable(info):
        return (info.st_dev, info.st_ino, info.st_mode, info.st_nlink, info.st_uid, info.st_gid,
                info.st_size, info.st_mtime_ns, info.st_ctime_ns)
    require(root_identity(ctx.root) == ctx.original_output_identity, "bootstrap original root changed")
    with ExitStack() as stack:
        parent = os.open(ctx.root, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        stack.callback(os.close, parent)
        require(KnownFileDiagnostics.identity(os.fstat(parent)) == ctx.original_output_identity, "bootstrap opened root differs")
        chain, prefix = [], Path()
        for name in Path(relative).parts[:-1]:
            child = os.open(name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=parent)
            stack.callback(os.close, child)
            identity = KnownFileDiagnostics.identity(os.fstat(child))
            prefix /= name
            require(identity == tuple(inventory[prefix.as_posix()])
                    and identity[0] == ctx.original_output_identity[0]
                    and identity[3:] == ctx.original_output_identity[3:], "bootstrap parent authority differs")
            chain.append((parent, name, child, identity)); parent = child
        name = Path(relative).name
        descriptor = os.open(name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=parent)
        stack.callback(os.close, descriptor)
        before = os.fstat(descriptor)
        maximum = DEFINITIONS_BYTES if relative == DEFINITIONS_CACHE else 16384
        require(stable(before) == tuple(expected) and stat.S_ISREG(before.st_mode) and before.st_nlink == 1
                and before.st_size <= maximum and (before.st_uid, before.st_gid) == ctx.original_output_identity[3:]
                and (relative != DEFINITIONS_CACHE or before.st_size == DEFINITIONS_BYTES), "bootstrap opened file differs")
        chunks, remaining = [], maximum + 1
        while remaining:
            ctx.budget.check()
            chunk = os.read(descriptor, remaining)
            if not chunk: break
            chunks.append(chunk)
            remaining -= len(chunk)
        body = b"".join(chunks)
        require(len(body) == before.st_size and stable(os.fstat(descriptor)) == stable(before)
                and stable(os.stat(name, dir_fd=parent, follow_symlinks=False)) == stable(before), "bootstrap file changed")
        for ancestor, leaf, child, identity in chain:
            require(KnownFileDiagnostics.identity(os.fstat(child)) == identity
                    and KnownFileDiagnostics.identity(os.stat(leaf, dir_fd=ancestor, follow_symlinks=False)) == identity, "bootstrap parent changed")
        require(root_identity(ctx.root) == ctx.original_output_identity, "bootstrap root changed during read")
        return body


class BootstrapEffects:
    config_path, config_mode = METADATA_CONFIG, 0o600
    metadata_route = "explicit-owned-config; not ordinary default startup"

    @classmethod
    def prepare(cls, ctx):
        path = ctx.root / cls.config_path
        path.write_text('{"read_config_from":{"cloud":false}}\n'); path.chmod(cls.config_mode)
        return ["--config", str(path)]

    def __init__(self, ctx, baseline):
        self.ctx, self.baseline = ctx, baseline
        require(DEFINITIONS_CACHE not in baseline, "definition cache preexists original baseline")
        self.definitions_identity = None
        self.installation_sha, self.version_seen = None, False
        self.proofs = []

    def verify(self, current):
        ctx, baseline = self.ctx, self.baseline
        require(root_identity(ctx.root) == ctx.original_output_identity, "bootstrap root replaced")
        require(all(key in current and (key == self.config_path or current[key] == value) for key, value in baseline.items()), "protected original authority changed")
        if self.definitions_identity is not None:
            require(all(current.get(key) == value for key, value in self.definitions_identity.items()),
                    "definition cache original identity changed")
        new = set(current) - set(baseline)
        logs = {key for key in new if BOOTSTRAP_LOG.fullmatch(key)}
        require(len(new) <= 24 and len(logs) <= 8, "bootstrap membership count")
        total, records = 0, {}
        for relative in sorted(new | {self.config_path}):
            ctx.budget.check()
            info = current[relative]
            require(info[0] == ctx.original_output_identity[0], "bootstrap filesystem changed")
            if relative in BOOTSTRAP_DIRS:
                require(len(info) == 5 and info[2] == stat.S_IFDIR | 0o755 and tuple(info[3:]) == ctx.original_output_identity[3:], "bootstrap directory ownership/mode")
                continue
            require(relative in BOOTSTRAP_FIXED or relative in (self.config_path, INSTALLATION_ID, MAINTENANCE) or relative in logs, "unknown native bootstrap path")
            mode = self.config_mode if relative == self.config_path else (0o600 if relative == DEFINITIONS_CACHE else 0o644)
            require(len(info) == 9 and info[2] == stat.S_IFREG | mode and info[3] == 1, "bootstrap ordinary file mode/link")
            body = read_bootstrap_file(ctx, relative, info, current)
            # One separately pinned native cache does not enlarge mutable-file allowance.
            if relative != DEFINITIONS_CACHE:
                total += len(body)
                require(total <= 256 << 10, "bootstrap byte count")
            digest = hashlib.sha256(body).hexdigest()
            if relative in BOOTSTRAP_FIXED:
                require((len(body), digest) == BOOTSTRAP_FIXED[relative], "bootstrap fixed bytes differ")
                if relative == DEFINITIONS_CACHE and self.definitions_identity is None:
                    parents = Path(relative).parents
                    keys = [str(parent) for parent in parents if str(parent) != "."] + [relative]
                    self.definitions_identity = {key: current[key] for key in keys}
            elif relative == INSTALLATION_ID:
                require(re.fullmatch(rb"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}", body), "unmeasured installation ID format")
                require(self.installation_sha in (None, digest), "installation ID continuity changed")
                self.installation_sha = digest
            elif relative == MAINTENANCE:
                require(re.fullmatch(rb"[1-9][0-9]{0,9}\n", body), "unmeasured maintenance lock format")
            elif relative == self.config_path:
                self.verify_config(body, info)
            elif relative in logs:
                datetime.strptime(Path(relative).name[6:21], "%Y%m%d-%H%M%S")
            records[relative] = {"bytes": len(body), "sha256": digest}
        require(self.installation_sha is None or INSTALLATION_ID in current, "installation ID disappeared")
        self.proofs.append(records)

    def verify_config(self, body, _info):
        require(len(body) <= 256, "metadata config byte bound")
        value = strict_json(body)
        allowed = {"read_config_from": {"cloud": False}}
        if "version" in value:
            require(type(value["version"]) is int and value["version"] == 1, "metadata config version")
            allowed["version"] = 1
        require(value == allowed and value["read_config_from"]["cloud"] is False, "metadata config unexpected settings")
        require(not self.version_seen or "version" in value, "metadata config version regressed")
        self.version_seen |= "version" in value

    def verify_command(self, before, after):
        require(len({key for key in after.keys() - before.keys() if BOOTSTRAP_LOG.fullmatch(key)}) <= 1, "multiple logs born in one command")
        self.verify(after)

    def verify_snapshot(self, before, after):
        # Auxiliary parent size can reflect exactly the permitted child creation;
        # outer verification independently holds its original inode/owner/mode.
        parents = {str(Path(key).parent) for key in BOOTSTRAP_DIRS}
        for key in before.keys() | after.keys():
            if key == self.config_path:
                continue  # Independently bounded content/identity predicate above.
            if key in before:
                left, right = dict(before[key]), dict(after.get(key, {}))
                if key in parents and left.get("kind") == right.get("kind") == "directory":
                    left.pop("size", None); right.pop("size", None)
                require(left == right, "original fixture bytes or identity changed")
            else:
                require(key.startswith("home/") and (key in BOOTSTRAP_DIRS or key in BOOTSTRAP_FIXED
                        or key in (INSTALLATION_ID, MAINTENANCE) or BOOTSTRAP_LOG.fullmatch(key)), "unknown fixture effect")


def verify_metadata_sources(body, ctx, expected, scope):
    rows = strict_json(body)
    require(type(rows) is list and all(type(row) is dict for row in rows), "MCP inventory rows")
    wanted = {name: (entry, Path(expected["user"]["root"]) / "mcp_config.json") for name, entry in expected["user"]["mcp"].items()}
    if scope in ("a", "b"):
        wanted.update({name: (entry, Path(expected[scope]["root"]) / "mcp_config.json") for name, entry in expected[scope]["mcp"].items()})
    if scope == "a":
        wanted["same-name"] = ({"url": expected["local"]["url"], "disabled": True}, Path(expected["local"]["path"]))
    wanted["legacy-foreign"] = ({"url": expected["legacy_url"], "disabled": True}, ctx.home / ".codeium/windsurf/mcp_config.json")
    require(len(rows) == len(wanted) and {row.get("name") for row in rows} == set(wanted), "MCP original scope members differ")
    for row in rows:
        require(set(row) == {"name", "enabled", "disabled_reason", "transport", "command", "args", "url", "disabled_tools", "source"}, "unreviewed MCP row schema")
        entry, source = wanted[row["name"]]
        spelling = {str(source)}
        if source.is_relative_to(ctx.home): spelling.add("~/" + source.relative_to(ctx.home).as_posix())
        enabled = not entry.get("disabled", False)
        require(row.get("source") in spelling and row.get("enabled") is enabled
                and row.get("disabled_reason") == (None if enabled else "disabled")
                and row.get("transport") == ("stdio" if "command" in entry else "http")
                and row.get("url") == entry.get("url") and row.get("command") == entry.get("command")
                and row.get("args") == entry.get("args", []) and row.get("disabled_tools") == entry.get("disabledTools", []), "explicit config changed MCP source or semantics")


def verify_skill_metadata_sources(body, ctx, expected, scope, nonce):
    """Pinned native rows identify collisions by source, never by display name."""
    def row(name, description, provider, base_dir, display_name, triggers=("user", "model")):
        return dict(name=name, description=description, triggers=list(triggers), provider=provider,
                    base_dir=str(base_dir), display_name=display_name, warnings=[], errors=[])
    wanted = [row("legacy-foreign", "Preserve legacy Windsurf skill.", "Windsurf",
                  ctx.home / ".codeium/windsurf/skills/legacy-foreign", "legacy-foreign")]
    selected = ("user", scope) if scope in ("a", "b") else ("user",)
    name = str(ctx.projects[scope]) + ":same-skill" if scope in ("a", "b") else "same-skill"
    wanted.extend(row(name, "Inert " + source + " " + nonce, "Devin",
                      Path(expected[source]["skill"]).parent, "same-skill") for source in selected)
    wanted.extend((
        row("devin-cli", "Look up Devin CLI documentation (skills, extensibility, configuration, commands, models, troubleshooting)",
            "Builtin", ctx.root / "client/share/devin", "devin-cli", ("model",)),
        row("declarative-repo-setup", "Generate and verify a working environment.yaml (Devin snapshot-setup blueprint) for a repo",
            "Builtin", "", "declarative-repo-setup", ("model", "user")),
        row("upload-secrets", "Securely upload local secrets (dotenv files, env vars, API keys) to the Devin Cloud secrets manager — values never enter the conversation",
            "Builtin", "", "upload-secrets", ("model", "user"))))
    rows = strict_json(body)
    require(type(rows) is list and len(rows) == len(wanted), "Skills original scope members differ")
    keys = ("provider", "base_dir", "display_name")
    remaining = {tuple(item[key] for key in keys): item for item in wanted}
    for item in rows:
        require(type(item) is dict and set(item) == set(wanted[0])
                and all(type(item[key]) is str for key in (*keys, "name", "description")), "unreviewed Skills row schema")
        key = tuple(item[field] for field in keys)
        require(key in remaining and item == remaining.pop(key), "Skills original source or semantics differ")
    require(not remaining, "Skills original scope members differ")


METADATA_SCHEMA = "devin-3000.12.57-exact-metadata"
INVENTORY_PROBES = tuple(kind + "-" + scope for scope in ("a", "b", "user", "a-repeat") for kind in ("mcp", "skills"))
METADATA_PROBES = tuple(name for name, _ in HELP) + INVENTORY_PROBES


def require_complete_metadata(summary):
    """Completion joins the finite measured sequence without qualifying runtime."""
    require(summary.get("metadata_capture_complete") is True and summary.get("admission_qualified") is False
            and summary.get("exact_source_admission") == summary.get("native_json_schema") == METADATA_SCHEMA
            and "error" not in summary, "metadata completion state")
    qualification = summary.get("qualification")
    require(type(qualification) is dict and set(qualification) == set(shared.QUALIFICATION_CASES)
            and all(value is False for value in qualification.values()), "metadata runtime qualification")
    commands = summary.get("commands")
    probes = ("source-commit", "source-tree", "source-clean", "git-init-a", "git-init-b", "git-init-c") + METADATA_PROBES + ("source-final-clean",)
    require(type(commands) is list and len(commands) == len(probes)
            and all(type(command) is dict and command.get("probe") == probe
                    and type(command.get("exit")) is int and command["exit"] == 0
                    and "error" in command and command["error"] is None for command, probe in zip(commands, probes)), "metadata command sequence")
    effects = summary.get("command_effects")
    require(type(effects) is list and len(effects) == len(METADATA_PROBES)
            and all(type(effect) is dict and effect.get("label") == probe
                    for effect, probe in zip(effects, METADATA_PROBES)), "metadata effect sequence")
    for field, empty in (("probe_methods", []), ("accepted_connections", 0)):
        values = summary.get(field)
        require(type(values) is dict and set(values) == {"a", "b", "user", "local", "legacy"}
                and all(type(value) is type(empty) and value == empty for value in values.values()), "metadata endpoint effects")


READOUT_LABELS = frozenset(name for name, _ in HELP) | frozenset(kind + '-' + scope for kind in ('mcp', 'skills') for scope in ('a', 'b', 'user', 'a-repeat'))
READOUT_PHASES = READOUT_LABELS | frozenset(('preparation', 'source', 'acquisition', 'materialization', 'probes', 'fixture', 'source-final-clean'))
READOUT_ERRORS = {
    'native metadata command refused': 'native-command-refused',
    'native help/version diagnostic': 'native-help-diagnostic',
    'help changed original or metadata input': 'help-effects-refused',
    'protected original authority changed': 'original-authority-refused',
    'original fixture bytes or identity changed': 'original-fixture-refused',
    'unknown native bootstrap path': 'bootstrap-path-refused',
    'bootstrap membership count': 'bootstrap-count-refused',
    'bootstrap ordinary file mode/link': 'bootstrap-mode-link-refused',
    'bootstrap fixed bytes differ': 'bootstrap-fixed-content-refused',
    'unmeasured installation ID format': 'installation-format-refused',
    'installation ID continuity changed': 'installation-continuity-refused',
    'unmeasured maintenance lock format': 'maintenance-format-refused',
    'metadata config unexpected settings': 'metadata-settings-refused',
    'metadata config version': 'metadata-version-refused',
    'metadata config version regressed': 'metadata-version-refused',
    'multiple logs born in one command': 'log-count-refused',
    'MCP original scope members differ': 'mcp-members-refused',
    'explicit config changed MCP source or semantics': 'mcp-source-refused',
    'unreviewed MCP row schema': 'mcp-schema-refused',
    'Skills original scope members differ': 'skills-members-refused',
    'Skills original source or semantics differ': 'skills-source-refused',
    'unreviewed Skills row schema': 'skills-schema-refused',
    'metadata repeat differs': 'metadata-repeat-refused',
    'native metadata launched stdio': 'stdio-effect-refused',
    'native metadata contacted MCP endpoint': 'endpoint-effect-refused',
}


def readout_reason(error):
    try:
        return READOUT_ERRORS.get(str(error), 'unclassified-refusal')
    except Exception:
        return 'unclassified-refusal'


def closed_readout(summary, state):
    def budget():
        require(type(state['deadline']) is float and time.monotonic() < state['deadline'], 'readout deadline')
    budget()
    require(type(state['phase']) is str and state['phase'] in READOUT_PHASES
            and type(state['reason']) is str and state['reason'] in set(READOUT_ERRORS.values()) | {'capture-incomplete', 'capture-finished-unqualified', 'metadata-complete', 'unclassified-refusal'}, 'readout state')
    require(type(summary.get('collection_complete')) is bool and summary.get('admission_qualified') is False
            and summary.get('cleanup_complete') is True and type(summary.get('metadata_capture_complete')) is bool
            and summary.get('artifact_admission_failed', False) is False, 'readout admission')
    if summary['collection_complete']:
        require_complete_metadata(summary)
        require(state['phase'] == 'source-final-clean' and state['reason'] == 'metadata-complete', 'readout completion')
    qualification = summary['qualification']
    require(type(qualification) is dict and set(qualification) == set(shared.QUALIFICATION_CASES)
            and all(value is False for value in qualification.values()), 'readout qualification')
    require(all(type(summary.get(key)) is str and re.fullmatch('[0-9a-f]{40}', summary[key])
                for key in ('source_commit', 'source_tree')), 'readout source')
    commands, effects = summary['commands'], summary.get('command_effects', [])
    require(type(commands) is list and len(commands) <= 60 and type(effects) is list and len(effects) <= 13, 'readout counts')
    failed, rows, seen = 0, [], set()
    for command in commands:
        budget()
        require(type(command) is dict and 'exit' in command and 'error' in command
                and (command.get('exit') is None or type(command['exit']) is int)
                and (command.get('error') is None or type(command['error']) is str), 'readout command')
        failed += command.get('exit') != 0 or command.get('error') is not None
    def counts(changes):
        require(type(changes) is dict and len(changes) <= 8192, 'readout effect bound')
        added = removed = changed = 0
        for delta in changes.values():
            budget()
            require(type(delta) is dict and set(delta) == {'before', 'after'}
                    and (delta['before'] is not None or delta['after'] is not None), 'readout delta')
            added += delta['before'] is None
            removed += delta['after'] is None
            changed += delta['before'] is not None and delta['after'] is not None
        return dict(added=added, removed=removed, changed=changed)
    for effect in effects:
        budget()
        require(type(effect) is dict and set(effect) == {'label', 'changes', 'outer_changes'}
                and type(effect['label']) is str and effect['label'] in READOUT_LABELS and effect['label'] not in seen, 'readout effect')
        seen.add(effect['label'])
        rows.append(dict(phase=effect['label'], fixture=counts(effect['changes']), outer=counts(effect['outer_changes'])))
    value = dict(schema_version=1, phase=state['phase'], reason=state['reason'], collection_complete=summary['collection_complete'],
                 metadata_capture_complete=summary['metadata_capture_complete'], cleanup_complete=True,
                 commands=len(commands), failed_commands=failed, effects=rows)
    budget()
    require(len(json.dumps(value).encode()) <= 16 << 10, 'readout output bound')
    return value


def publish_with_readout(summary, ctx, output, identity, state):
    shared.publish(summary, ctx, output)
    try:
        require(root_identity(output) == identity, 'readout original root')
        value = closed_readout(summary, state)
    except Exception:
        value = dict(schema_version=1, reason='readout-refused')
    try:
        os.write(1, ('DEVIN_CLOSED_READOUT=' + json.dumps(value, separators=(',', ':')) + '\n').encode())
    except (OSError, ValueError):
        pass


def collect(ctx, probes, readout_state=None, *, effects_type=BootstrapEffects):
    ctx.summary["metadata_route"] = effects_type.metadata_route
    nonce = secrets.token_hex(32)
    expected, startup, marker = fixture(ctx, {key: value.url for key, value in probes.items()}, nonce)
    roots = {"home": ctx.home, "temporary": ctx.root / "tmp", **ctx.projects}
    identities = {name: root_identity(path) for name, path in roots.items()}
    prefix = effects_type.prepare(ctx)
    for scope in ("a", "b"):
        exclude = ctx.projects[scope] / ".git/info/exclude"
        with exclude.open("ab") as stream: stream.write(b".devin/mcp_config.local.json\n")
    baseline = snapshot(ctx)
    ctx.known_file_diagnostics = KnownFileDiagnostics(ctx.root, ctx.original_output_identity, baseline)
    outer_before = metadata_inventory(ctx.root, ctx.budget)
    effects = effects_type(ctx, outer_before)
    ctx.summary["prospective_bootstrap_effects"] = effects.proofs
    ctx.identities[marker] = hashlib.sha256(marker.read_bytes()).hexdigest()
    ctx.summary["stdio_effect_guard"] = {"path": str(marker), "sha256": ctx.identities[marker], "python": sys.executable}
    ctx.summary.update(fixture=expected, fixture_before=baseline, outer_before=outer_before, command_effects=[],
                       exact_source_admission="unreviewed-native-schema", admission_qualified=False)

    def command(label, arguments, scope="a"):
        if readout_state is not None: readout_state["phase"] = label
        before = snapshot(ctx)
        command_outer_before = metadata_inventory(ctx.root, ctx.budget)
        effects.verify(command_outer_before)
        effects.verify_snapshot(baseline, before)
        try:
            record, out, err = ctx.probe(label, [*prefix, *arguments], project=scope)
            require(record["exit"] == 0, "native metadata command refused")
            return out, err
        finally:
            after = snapshot(ctx)
            delta = {key: {"before": before.get(key), "after": after.get(key)}
                     for key in before.keys() | after.keys() if before.get(key) != after.get(key)}
            outer_after = metadata_inventory(ctx.root, ctx.budget)
            outer_delta = {key: {"before": outer_before.get(key), "after": outer_after.get(key)}
                           for key in outer_before.keys() | outer_after.keys() if outer_before.get(key) != outer_after.get(key)}
            ctx.summary["command_effects"].append({"label": label, "changes": delta, "outer_changes": outer_delta})
            require({name: root_identity(path) for name, path in roots.items()} == identities, "fixture root replaced")
            if label in {name for name, _arguments in HELP}:
                require(outer_after == outer_before and after == baseline, "help changed original or metadata input")
            effects.verify_command(command_outer_before, outer_after)
            effects.verify_snapshot(baseline, after)
            zero_effects(probes, startup)

    for label, arguments in HELP:
        out, err = command(label, arguments)
        require(not err, "native help/version diagnostic")
        text = out.decode("utf-8", "strict")
        if label == "version":
            require(re.fullmatch(r"devin 3000\.12\.57 \([0-9a-f]{7,40}\)\n", text), "native version/build grammar differs")
        if label.endswith("list-help"):
            require("--json" in text, "native help omits JSON inventory")
    first_a = {}
    for label, scope in (("a", "a"), ("b", "b"), ("user", "user"), ("a-repeat", "a")):
        for kind in ("mcp", "skills"):
            out, _err = command(kind + "-" + label, [kind, "list", "--json"], scope)
            if kind == "mcp": verify_metadata_sources(out, ctx, expected, scope)
            else: verify_skill_metadata_sources(out, ctx, expected, scope, nonce)
            if label == "a": first_a[kind] = out
            if label == "a-repeat": require(out == first_a[kind], "metadata repeat differs")
    ctx.summary["prospective_bootstrap_effects"] = effects.proofs
    ctx.summary.update(fixture_after=snapshot(ctx), metadata_capture_complete=True,
                       native_json_schema=METADATA_SCHEMA, exact_source_admission=METADATA_SCHEMA,
                       probe_methods={name: list(probe.methods()) for name, probe in probes.items()},
                       accepted_connections={name: len(probe.accepted_connections) for name, probe in probes.items()})


def observe_connections(probe):
    """Observe the existing fixture's accepted connections, including non-MCP HTTP."""
    probe.accepted_connections = []
    original = probe.server.get_request
    def get_request():
        connection, address = original()
        connection.settimeout(2)
        probe.accepted_connections.append(address)
        return connection, address
    probe.server.get_request = get_request
    return probe


def run(output, *, collector=collect, recipe="private"):
    require(recipe in ("private", "ordinary") and ((collector is collect) == (recipe == "private")), "unknown Devin research recipe")
    shared.native.require_hosted("linux-amd64")
    require(output.is_absolute() and output.resolve() == output and not output.exists()
            and shared.SOURCE not in output.parents, "fresh output outside source required")
    output.mkdir(mode=0o700, parents=True)
    identity = root_identity(output)
    prefix = "devin-structured-inventory-" if recipe == "private" else "devin-ordinary-inventory-"
    summary = {"client": "windsurf", "phase": "observe", "candidate": prefix + VERSION, "metadata_recipe": recipe,
               "collection_complete": False, "metadata_capture_complete": False, "admission_qualified": False,
               "qualification": {name: False for name in shared.QUALIFICATION_CASES}, "commands": [],
               "host": {key: os.environ.get(key) for key in ("RUNNER_OS", "RUNNER_ARCH", "ImageOS", "ImageVersion", "GITHUB_RUN_ID", "GITHUB_RUN_ATTEMPT")}}
    ctx, held_cleanup, probes_settled = None, {}, True
    readout_state = dict(phase="preparation", reason="capture-incomplete", deadline=None)
    try:
        with shared.Budget(360) as budget:
            readout_state["deadline"] = budget.deadline
            ctx = AdmissionContext(output, summary, budget)
            ctx.original_output_identity = identity
            held_cleanup = {path: root_identity(path) for path in (ctx.home, ctx.private, ctx.output, output / "tmp", *ctx.projects.values())}
            readout_state["phase"] = "source"
            ctx.source_and_projects()
            readout_state["phase"] = "acquisition"
            receipt, contents = shared.acquire(SPEC, budget)
            admitted_archive(receipt)
            summary["acquisition"] = receipt
            readout_state["phase"] = "materialization"
            binary, ctx.identities = shared.materialize(output / "client", SPEC, receipt, contents)
            held_cleanup[output / "client"] = root_identity(output / "client")
            ctx.argv = [str(binary)]
            summary["runtime_files"] = [{"path": str(path), "sha256": digest} for path, digest in ctx.identities.items()]
            readout_state["phase"] = "probes"
            probes_settled = False
            probes = {}
            try:
                with ExitStack() as stack:
                    for key in ("a", "b", "user", "local", "legacy"):
                        probes[key] = observe_connections(stack.enter_context(PiHTTPMCPProbe(nonce=secrets.token_hex(32))))
                    try:
                        readout_state["phase"] = "fixture"
                        collector(ctx, probes, readout_state)
                    finally:
                        summary.update(probe_methods={name: list(probe.methods()) for name, probe in probes.items()},
                                       accepted_connections={name: len(probe.accepted_connections) for name, probe in probes.items()})
            finally:
                probes_settled = len(probes) == 5 and all(not probe.thread.is_alive() for probe in probes.values())
            zero_effects(probes, output / "stdio-started")
            shared.verify_files(ctx.identities, budget)
            readout_state["phase"] = "source-final-clean"
            record, out, err = ctx.command("source-final-clean", ["/usr/bin/git", "status", "--porcelain", "--untracked-files=normal"], shared.SOURCE, ctx.env, 10)
            require(record["exit"] == 0 and not out and not err, "source changed")
            require_complete_metadata(summary)
            summary["collection_complete"] = True
            readout_state["reason"] = "metadata-complete"
    except Exception as error:
        summary["collection_complete"] = False
        readout_state["reason"] = readout_reason(error)
        summary["error"] = type(error).__name__ + ": " + str(error)[:300]
    finally:
        require(root_identity(output) == identity, "outer root replaced")
        try:
            require((ctx is None or ctx.process_cleanup_proved) and probes_settled, "owned process/probe cleanup unproved")
            diagnostics = getattr(ctx, "known_file_diagnostics", None)
            if diagnostics is not None:
                try:
                    summary["known_file_diagnostics"] = diagnostics.finish()
                except (ValueError, OSError, UnicodeError):
                    summary["known_file_diagnostics"] = {"schema_version": 1, "complete": False}
                finally:
                    diagnostics.close()
            for path, held in held_cleanup.items():
                require(root_identity(path) == held, "cleanup directory identity changed")
                shutil.rmtree(path)
            summary["cleanup_complete"] = True
        except Exception as error:
            summary.update(collection_complete=False, cleanup_complete=False,
                           cleanup_error=type(error).__name__)
        diagnostics = getattr(ctx, "known_file_diagnostics", None)
        if diagnostics is not None:
            diagnostics.close()
        if not summary.get("cleanup_complete"):
            summary.pop("known_file_diagnostics", None)
        # Existing publisher admits only held bounded streams and closed JSON ZIP.
        # Unproved cleanup retains a fixed failure report, never raw captures.
        if not summary.get("cleanup_complete"):
            safe = {"collection_complete": False, "admission_qualified": False,
                    "qualification": summary["qualification"], "commands": [], "cleanup_complete": False,
                    "metadata_recipe": recipe}
            publish_with_readout(safe, None, output, identity, readout_state)
        else:
            try:
                bodies = shared.evidence(summary, ctx.captures if ctx else {}, [])
                require(sum(map(len, bodies.values())) <= 16 << 20, "admission evidence aggregate exceeds16MiB")
                publish_with_readout(summary, ctx, output, identity, readout_state)
            except (ValueError, KeyError, TypeError, OSError):
                summary["collection_complete"] = False
                safe = {"collection_complete": False, "admission_qualified": False, "commands": [],
                        "qualification": summary["qualification"], "artifact_admission_failed": True,
                        "metadata_recipe": recipe}
                publish_with_readout(safe, None, output, identity, readout_state)
    return 0 if summary["collection_complete"] else 1


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, type=Path)
    raise SystemExit(run(parser.parse_args().output))
