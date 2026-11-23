"""Closed genuine Devin Global Skill discovery proof for four immutable cases.

The caller runs the original bounded native process and passes held streams.
Only source/receipt-bound four-phase inventory can qualify; capability-level
Prepared support, files, and public Active are individually insufficient.
No login, trust repair, cache exception, or escaped-writer closure is claimed.
"""
import base64
import copy
from contextlib import contextmanager
import hashlib
import json
import math
import os
from pathlib import Path
import re
import stat
import time
import tomllib

import claude_cache_retirement_audit as custody
from devin_project_auto_install_e2e import VERSION, BINARY_SHA
from native_client_project_artifacts import identity
from project_native_runtime import strict_json as _strict_json

LIMIT = 1 << 20
PHASES = ("before", "add", "repeat", "remove")
VERSION_OUTPUT = "devin " + VERSION + " (96542e930752)"
FOREIGN_NAME = "foreign-windsurf-global-audit"
FOREIGN_DESCRIPTION = "Inert foreign Global Skill"
CONFIG = b'{"version":1,"auto_update":false}\n'
FOREIGN = ("---\nname: " + FOREIGN_NAME + "\ndescription: " + FOREIGN_DESCRIPTION + "\n---\nLeave unchanged.\n").encode()
GLOBAL_SKILLS = ".config/devin/skills"
KIND = "devin_global_skill_directory"
EMPTY_INTENT = "sha256:" + hashlib.sha256(b'{"schema_version":1,"dependencies":null,"targets":null}').hexdigest()
PROBE_KEYS = set("probe argv cwd error exit elapsed_seconds stdout_file stderr_file stdout_bytes stderr_bytes stdout_sha256 stderr_sha256".split())
# Independent original fc68 evidence, run 44939583949, rows 22/25/26/29.
# Content-addressed source and normalized manifest formats are unchanged.
# Complete Skill sets were independently read from each immutable Git tree's
# immediate skills/*/SKILL.md entries and its root plugin.json (default Skills
# discovery). Original rows 22/25 have truncated tails and are not set authority.
CASES = {
    "github:gemini-cli-extensions/sre@4130b149c0b069ef2028e42b3b9f3f84cce935e6": {
        "name": "sre-extension", "version": "0.1.23",
        "tree_digest": "sha256:7425c59f550e8113795b12840d2e1ba3ef38586dd78962518a3a37055e6f2744",
        "manifest_digest": "sha256:7d2ba4e4603de17e058be93a47d4cc983096101430491dfa673f624617ede5b1",
        "skills": ("anomaly-detection", "cloud-logging", "cloud-monitoring", "data-ingestion",
                   "gcp-architecture-discovery", "gcp-mcp-setup", "gcp-playbooks", "gcp-setup",
                   "gcp-slo-management", "generic-mitigations", "investigation-entrypoint", "monitoring-graphs",
                   "postmortem-aggregator", "postmortem-generator", "pre-publish-checker", "safe-sre-investigator")},
    "github:K-Dense-AI/claude-scientific-writer@529b9f73ab4b48925027145800d7b49e96ccfd3c": {
        "name": "claude-scientific-writer", "version": "2.22.0",
        "tree_digest": "sha256:bd70f81982eaa8984aee73838542ed2c190ea73f908977d210368fef7275c3ca",
        "manifest_digest": "sha256:c1950c0a0b6727ffda4f895bb179e0d749cc2fd5d52747ccdc034d66854fee34",
        "skills": ("citation-management", "clinical-decision-support", "clinical-reports", "docx", "generate-image",
                   "hypothesis-generation", "infographics", "latex-posters", "literature-review", "market-research-reports",
                   "markitdown", "parallel-web", "pdf", "peer-review", "pptx", "pptx-posters", "research-grants",
                   "research-lookup", "scholar-evaluation", "scientific-critical-thinking", "scientific-schematics",
                   "scientific-slides", "scientific-writing", "treatment-plans", "venue-templates", "xlsx")},
    "github:obra/the-elements-of-style@05fc4f0d2b97b7c042dd9949ad658568e4a1324e": {
        "name": "elements-of-style", "version": "1.0.0",
        "tree_digest": "sha256:32fc89e62a200be082357d2cd5111284aa974b79678835cd60875de8fbba5958",
        "manifest_digest": "sha256:1093f04ee889eedb6fd1030ac7fd7a7a3fa586eba150ac493afb97f0fc1cb5d3",
        "skills": ("writing-clearly-and-concisely",)},
    "github:workato-devs/recipe-skills@afa1e729a013061c7b328271fadcdf240091cf3c": {
        "name": "recipe-skills", "version": "0.1.0",
        "tree_digest": "sha256:17eafbd254c8ae2164d7e3010ee9e42fc33c50b9df9eb2a6e90822f752e6bd39",
        "manifest_digest": "sha256:a72964152942835a188d6d053419d59110525bc6d9d80f3644b7d1397f20f4d4",
        "skills": ("asana-recipes", "datatable-recipes", "gmail-recipes", "jira-recipes",
                   "salesforce-recipes", "slack-recipes", "stripe-recipes", "workato-recipes")},
}


class CustodyError(ValueError):
    """Do not continue children after original filesystem authority is lost."""


def require(value, reason):
    if not value:
        raise ValueError(reason)


def strict_json(body):
    """Reuse the whole-document parser; match Go's folded keys/numeric range."""
    require(type(body) is bytes and len(body) <= LIMIT, "JSON stream is not bounded complete bytes")
    value = _strict_json(body)
    pending, nodes = [(value, 0)], 0
    while pending:
        current, depth = pending.pop()
        nodes += 1
        require(nodes <= 100000 and depth <= 64, "JSON structural bound")
        if type(current) is dict:
            keys = [key.casefold() for key in current]
            require(len(keys) == len(set(keys)), "case-ambiguous JSON member")
            pending.extend((child, depth + 1) for child in current.values())
        elif type(current) is list:
            pending.extend((child, depth + 1) for child in current)
        elif type(current) in (int, float):
            try:
                require(math.isfinite(float(current)), "JSON number is outside finite native range")
            except OverflowError as error:
                raise ValueError("JSON number is outside finite native range") from error
    return value


def sha(body):
    return hashlib.sha256(body).hexdigest()


def compact(value):
    return json.dumps(value, ensure_ascii=False, separators=(",", ":"), allow_nan=False).encode()


def same(left, right):
    return json.dumps(left, sort_keys=True, allow_nan=False) == json.dumps(right, sort_keys=True, allow_nan=False)


def facts(info):
    return {"device": info.st_dev, "inode": info.st_ino, "mode": info.st_mode,
            "links": info.st_nlink, "bytes": info.st_size, "modified_ns": info.st_mtime_ns,
            "changed_ns": info.st_ctime_ns, "uid": info.st_uid, "gid": info.st_gid}


def relative_parts(value):
    require(type(value) is str and value and Path(value).as_posix() == value
            and not Path(value).is_absolute() and not any(x in value for x in "\x00\r\n"), "noncanonical relative authority")
    parts = value.split("/")
    require(all(part not in ("", ".", "..") for part in parts), "relative authority traversal")
    return parts


@contextmanager
def member(root_fd, relative):
    """Read one named member through original descriptors, never a path alias."""
    opened = []
    parent = root_fd
    try:
        parts = relative_parts(relative)
        for index, name in enumerate(parts):
            before = os.stat(name, dir_fd=parent, follow_symlinks=False)
            flags = os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC
            if index < len(parts) - 1:
                flags |= os.O_DIRECTORY
            fd = os.open(name, flags, dir_fd=parent)
            opened.append((parent, name, fd, before))
            require(facts(before) == facts(os.fstat(fd)), "named member differs from held authority")
            parent = fd
        yield parent
        for directory, name, fd, before in opened:
            require(facts(before) == facts(os.fstat(fd)) == facts(os.stat(name, dir_fd=directory, follow_symlinks=False)),
                    "member changed during bounded read")
    finally:
        for _, _, fd, _ in reversed(opened):
            os.close(fd)


def read_file(root_fd, relative, limit=LIMIT):
    with member(root_fd, relative) as fd:
        info = os.fstat(fd)
        require(stat.S_ISREG(info.st_mode) and info.st_nlink == 1 and info.st_uid == os.getuid()
                and 0 <= info.st_size <= limit, "file must be bounded, regular and owned")
        body = bytearray()
        while len(body) <= info.st_size:
            block = os.read(fd, min(65536, info.st_size - len(body) + 1))
            if not block:
                break
            body.extend(block)
        require(len(body) == info.st_size and facts(info) == facts(os.fstat(fd)), "file changed during capture")
        return bytes(body), {**facts(info), "sha256": sha(body)}


def frame(root_fd):
    """Existing finite descriptor tree, augmented with full timestamp custody."""
    started = time.monotonic()
    observed, _, _ = custody.tree(root_fd)
    result = {}
    for relative, row in observed.items():
        require(time.monotonic() - started < 10, "bounded custody frame deadline")
        if relative:
            with member(root_fd, relative) as fd:
                info = os.fstat(fd)
        else:
            info = os.fstat(root_fd)
        require(info.st_uid == os.getuid(), "custody tree has a different owner")
        if row[0] == "file":
            require((info.st_mode, info.st_dev, info.st_ino, info.st_nlink, info.st_size,
                     info.st_mtime_ns, info.st_uid, info.st_gid) == row[1:-1], "custody file changed after hashing")
            result[relative] = {"kind": "file", **facts(info), "sha256": row[-1]}
        else:
            require((info.st_mode, info.st_dev, info.st_ino, info.st_uid, info.st_gid) == row[1:], "custody directory changed")
            result[relative] = {"kind": "dir", **facts(info)}
    return result


def frame_fact(value):
    return {"paths": len(value), "sha256": sha(json.dumps(value, sort_keys=True, separators=(",", ":")).encode())}


def frame_changes(before, after):
    """Name bounded sandbox-relative differences without exporting native values."""
    keys = sorted(key for key in before.keys() | after.keys() if before.get(key) != after.get(key))
    entries = []
    fields = ("kind", "device", "inode", "mode", "links", "bytes", "modified_ns", "changed_ns", "uid", "gid", "sha256")
    for key in keys[:8]:
        row = {"key_sha256": sha(key.encode("utf-8", errors="surrogatepass")),
               "change_kind": "added" if key not in before else "removed" if key not in after else "changed"}
        if (key.isascii() and not key.startswith(("/", "\\"))
                and not any(part == ".." for part in re.split(r"[/\\]", key))):
            row["relative_key"] = re.sub(r"[^A-Za-z0-9_./:-]", "?", key[:192])
            row["relative_key_truncated"] = len(key) > 192
        left, right = before.get(key), after.get(key)
        if type(left) is dict and type(right) is dict:
            row["changed_fields"] = [name for name in fields if left.get(name) != right.get(name)]
        elif (key.startswith("metadata:") and type(left) is tuple and type(right) is tuple
              and len(left) == len(right) and len(left) in (3, 4)):
            row["changed_fields"] = [name for i, name in enumerate(("mode", "device", "inode", "mtime_ns")[:len(left)])
                                     if left[i] != right[i]]
        entries.append(row)
    return {"total": len(keys), "omitted": max(0, len(keys) - len(entries)), "entries": entries}


def content(value):
    return {path: (row["kind"], bool(row["mode"] & 0o111), row["bytes"], row["sha256"])
            if row["kind"] == "file" else ("dir",) for path, row in value.items()}


def immutable_source_digest(root_fd, observed):
    """packagedigest v1's fixed binary framing over the actual cached source.

    These four immutable sources contain only regular files/directories. The
    existing nofollow frame deliberately refuses other object kinds.
    """
    digest = hashlib.sha256()

    def field(value):
        body = value.encode("utf-8")
        digest.update(len(body).to_bytes(8, "big"))
        digest.update(body)

    field("agentplugins.package-tree\0sha256\0v1")
    for path in sorted(observed):
        if not path:
            continue
        row = observed[path]
        require(row["kind"] in ("file", "dir"), "immutable source has an unsupported object")
        kind = "directory" if row["kind"] == "dir" else "file"
        mode = "040000" if kind == "directory" else "100755" if row["mode"] & 0o111 else "100644"
        for value in ("entry", path, kind, mode, ""):
            field(value)
        digest.update((row["bytes"] if kind == "file" else 0).to_bytes(8, "big"))
        if kind == "file":
            body, current = read_file(root_fd, path, custody.MAX_FILE_BYTES)
            require(current == {k: v for k, v in row.items() if k != "kind"}, "immutable source changed during digest")
            digest.update(body)
    return "sha256:" + digest.hexdigest()


def managed_digest(root_fd, observed):
    """packagesnapshot.Digest: globally lexical paths, framed size/execute bit."""
    digest = hashlib.sha256()
    for path in sorted(observed):
        if not path:
            continue
        row = observed[path]
        executable = row["kind"] == "file" and bool(row["mode"] & 0o111)
        size = row["bytes"] if row["kind"] == "file" else 0
        digest.update(f'{row["kind"]}\0{path}\0{str(executable).lower()}\0{size}\0'.encode())
        if row["kind"] == "file":
            body, current = read_file(root_fd, path, custody.MAX_FILE_BYTES)
            require(current == {k: v for k, v in row.items() if k != "kind"}, "managed source changed during digest")
            digest.update(body)
    return "sha256:" + digest.hexdigest()


def skill_digest(root_fd, observed):
    """shared.DigestSkillDirectory: WalkDir depth-first, its distinct framing."""
    digest = hashlib.sha256()
    for path in sorted(observed, key=lambda value: tuple(value.split("/"))):
        if not path:
            continue
        row = observed[path]
        digest.update(("d" if row["kind"] == "dir" else "f").encode() + b"\0" + path.encode() + b"\0")
        if row["kind"] == "file":
            body, current = read_file(root_fd, path, custody.MAX_FILE_BYTES)
            require(current == {k: v for k, v in row.items() if k != "kind"}, "native Skill changed during digest")
            digest.update(b"x\0" if row["mode"] & 0o111 else b"-\0")
            digest.update(body + b"\0")
    return "sha256:" + digest.hexdigest()


def description(body, expected_name):
    """Strict bounded YAML, called only after immutable source/receipt joins."""
    import yaml  # Existing pinned dependency, provisioned only for this closed row.
    try:
        return _description(body, expected_name, yaml)
    except yaml.YAMLError as error:
        raise ValueError("bounded Skill YAML is malformed") from error


def _description(body, expected_name, yaml):
    require(type(body) is bytes and len(body) <= LIMIT, "Skill document bound")
    text = body.decode("utf-8-sig", errors="strict").replace("\r\n", "\n")
    require(text.startswith("---\n"), "Skill frontmatter opening differs")
    rest = text[4:]
    end = rest.find("\n---\n")
    if end < 0 and rest.endswith("\n---"):
        end = len(rest) - 4
    require(end >= 0 and len(rest[:end].encode()) <= 64 << 10, "Skill frontmatter must be closed and bounded")
    front = rest[:end]

    class Loader(yaml.SafeLoader):
        pass

    def mapping(loader, node):
        result = {}
        for key_node, value_node in node.value:
            key = loader.construct_object(key_node, deep=True)
            require(type(key) is str and key not in result, "duplicate/nonstring YAML mapping key")
            result[key] = loader.construct_object(value_node, deep=True)
        return result

    Loader.add_constructor(yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG, mapping)
    depth = nodes = members = 0
    active = set()
    for event in yaml.parse(front, Loader=Loader):
        if isinstance(event, (yaml.MappingStartEvent, yaml.SequenceStartEvent)):
            depth += 1
            require(depth <= 64, "YAML composition depth")
        elif isinstance(event, (yaml.MappingEndEvent, yaml.SequenceEndEvent)):
            depth -= 1

    def bounded(node, depth):
        nonlocal nodes, members
        nodes += 1
        require(nodes <= 100000 and depth <= 64 and id(node) not in active, "YAML node/depth/alias bound")
        active.add(id(node))
        if isinstance(node, yaml.MappingNode):
            members += len(node.value)
            require(members <= 10000, "YAML member bound")
            for key, value in node.value:
                bounded(key, depth + 1)
                bounded(value, depth + 1)
        elif isinstance(node, yaml.SequenceNode):
            for value in node.value:
                bounded(value, depth + 1)
        active.remove(id(node))

    loader = Loader(front)
    try:
        node = loader.get_single_node()
        require(isinstance(node, yaml.MappingNode), "Skill metadata must be one mapping")
        bounded(node, 0)
        value = loader.construct_object(node, deep=True)
    finally:
        loader.dispose()
    require(value.get("name") == expected_name and type(value.get("description")) is str
            and value["description"].strip() and len(value["description"]) <= 1024, "Skill name/description differs")
    return value["description"]


def skill_row(name, text, provider, base, triggers=("model", "user")):
    return {"name": name, "description": text, "provider": provider, "base_dir": str(base),
            "display_name": name, "triggers": list(triggers), "warnings": [], "errors": []}


def inventory(body):
    rows = strict_json(body)
    require(type(rows) is list and len(rows) <= 256, "native inventory row bound")
    result, folded = {}, set()
    for row in rows:
        require(type(row) is dict and set(row) == set("name description provider base_dir display_name triggers warnings errors".split()),
                "native inventory field set differs")
        for key in ("name", "description", "provider", "base_dir", "display_name"):
            require(type(row[key]) is str, "native inventory scalar type differs")
        name, base = row["name"], row["base_dir"]
        require(name and row["display_name"] and name.casefold() not in folded, "duplicate/empty native Skill")
        require(row["provider"] in ("Builtin", "Devin", "Windsurf", "Claude", "Copilot")
                and ((not base and row["provider"] == "Builtin") or (Path(base).is_absolute() and str(Path(base)) == base
                     and ".." not in Path(base).parts)), "native inventory origin differs")
        require(type(row["triggers"]) is list and 1 <= len(row["triggers"]) <= 2
                and all(type(x) is str and x in ("user", "model") for x in row["triggers"])
                and len(set(row["triggers"])) == len(row["triggers"])
                and row["warnings"] == [] and row["errors"] == [], "native triggers/errors differ")
        folded.add(name.casefold())
        result[name] = {**row, "triggers": sorted(row["triggers"])}
    return result


class WindsurfGlobalSkillAudit:
    def __init__(self, case, home, project, item, executable, client_version):
        self.dirs, self.files, self.retired = {}, {}, []
        self.observations, self.reasons, self.transitions = [], [], []
        self.closed = self.finished = self.removing = False
        self.owner = self.probe_frame = self.managed_frame = self.source_frame = None
        self.managed_fd = None
        self.bound_list = set()
        self.guard_count = 0
        try:
            require(os.name == "posix" and hasattr(os, "O_NOFOLLOW"), "Devin custody requires POSIX nofollow")
            require(type(item) is str and item in CASES, "source is outside the closed four-case proof")
            self.case, self.home, self.project = map(Path, (case, home, project))
            self.data, self.tmp = self.case / "uap-data", self.case / "tmp"
            require(self.home == self.case / "home" and self.project == self.case / "project", "case sibling authority differs")
            self.item, self.pin = item, copy.deepcopy(CASES[item])
            # compose.newAgentpluginsCLIApp passes explicit AGENTPLUGINS_HOME
            # as cacheRoot; empty User project keeps that same data root and
            # assembleAgentpluginsApp sets EnvironmentStoreRoot=cacheRoot/store.
            self.source_store = self.data / "store/packages/agentplugins-tree-sha256-v1" / self.pin["tree_digest"][7:] / "package"
            self.name = self.pin["name"]
            self.repo, self.commit = item[7:].split("@")
            self.repo = self.repo.lower()
            self.source_key = "github:" + self.repo + ":"
            self.source = "github:" + self.repo + "@" + self.commit
            self.revision = {"version": self.pin["version"], "resolved_revision": self.commit,
                             "tree_digest": self.pin["tree_digest"], "manifest_digest": self.pin["manifest_digest"]}
            require(type(client_version) is dict and client_version.get("status") == "reported"
                    and client_version.get("value") == VERSION_OUTPUT and type(client_version.get("exit_code")) is int
                    and client_version["exit_code"] == 0, "exact Devin version evidence differs")
            self.client_version = copy.deepcopy(client_version)
            for root in (self.case, self.home, self.project, self.data, self.tmp):
                fd = self._directory(root)
                info = os.fstat(fd)
                require(not stat.S_IMODE(info.st_mode) & 0o022, "case authority is writable by others")
                if root != self.tmp:
                    require(stat.S_IMODE(info.st_mode) == 0o700, "case/HOME/project/data must be private from creation")
            require(set(os.listdir(self.dirs[self.case][0])) == {"home", "project", "uap-data", "tmp"}
                    and not os.listdir(self.dirs[self.data][0]) and not os.listdir(self.dirs[self.tmp][0]), "case is not fresh before its first child")
            self.profile = self.home / ".config/devin"
            self.skills = self.profile / "skills"
            for directory in (self.profile, self.skills, self.skills / FOREIGN_NAME):
                directory.mkdir(mode=0o700)
                self._directory(directory)
            for path, body in ((self.profile / "config.json", CONFIG), (self.skills / FOREIGN_NAME / "SKILL.md", FOREIGN)):
                directory = self.dirs[path.parent][0]
                fd = os.open(path.name, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW | os.O_CLOEXEC, 0o600, dir_fd=directory)
                try:
                    require(os.write(fd, body) == len(body), "fixture write was incomplete")
                finally:
                    os.close(fd)
            self.initial_home = frame(self.dirs[self.home][0])
            expected_files = {"foreign-untouched.txt": b"foreign profile content never belongs to UAP\n",
                              ".windsurf/foreign-neighbor.txt": b"unmanaged native neighbor must survive\n",
                              ".config/devin/config.json": CONFIG,
                              GLOBAL_SKILLS + "/" + FOREIGN_NAME + "/SKILL.md": FOREIGN}
            require({p for p, r in self.initial_home.items() if r["kind"] == "file"} == set(expected_files), "initial HOME contains unauthored files")
            expected_dirs = {"", ".cache", ".config", ".config/devin", GLOBAL_SKILLS,
                             GLOBAL_SKILLS + "/" + FOREIGN_NAME, ".windsurf", ".local", ".local/share"}
            require({p for p, r in self.initial_home.items() if r["kind"] == "dir"} == expected_dirs, "initial HOME contains unauthored directories")
            self.fixed = {}
            for relative, body in expected_files.items():
                actual, fact = self._hold("home/" + relative)
                require(actual == body, "authored foreign/config bytes differ")
                self.fixed["home/" + relative] = {**fact, "base64": base64.b64encode(body).decode()}
            body, fact = self._hold("project/foreign-untouched.txt")
            require(body == b"foreign content never belongs to UAP\n", "project foreign fixture differs")
            self.fixed["project/foreign-untouched.txt"] = {**fact, "base64": base64.b64encode(body).decode()}
            self.initial_project = frame(self.dirs[self.project][0])
            require(set(self.initial_project) == {"", "foreign-untouched.txt"}, "User case project is not fresh")
            self.home_current = self.initial_home
            self.binary = Path(executable)
            require(self.binary.is_absolute() and str(self.binary) == os.fspath(executable)
                    and self.binary.resolve(strict=True) == self.binary and self.binary.name == "devin", "selected Devin executable is not canonical")
            self.binary_fd = os.open(self.binary, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC)
            self.retired.append(self.binary_fd)
            initial = os.fstat(self.binary_fd)
            require(stat.S_ISREG(initial.st_mode) and initial.st_nlink == 1 and not stat.S_IMODE(initial.st_mode) & 0o022
                    and 0 < initial.st_size <= 256 << 20 and initial.st_mode & 0o111, "selected binary is not bounded regular executable")
            digest, consumed = hashlib.sha256(), 0
            while consumed <= initial.st_size:
                block = os.read(self.binary_fd, min(65536, initial.st_size - consumed + 1))
                if not block:
                    break
                digest.update(block)
                consumed += len(block)
            require(consumed == initial.st_size and facts(initial) == facts(os.fstat(self.binary_fd)) == facts(self.binary.lstat())
                    and digest.hexdigest() == BINARY_SHA, "selected binary differs from existing vendor pin")
            self.binary_fact = {**facts(initial), "sha256": digest.hexdigest()}
            self.baseline = {
                "devin-cli": skill_row("devin-cli", "Look up Devin CLI documentation (skills, extensibility, configuration, commands, models, troubleshooting)",
                                       "Builtin", self.binary.parent.parent / "share/devin", ("model",)),
                "declarative-repo-setup": skill_row("declarative-repo-setup", "Generate and verify a working environment.yaml (Devin snapshot-setup blueprint) for a repo", "Builtin", ""),
                "upload-secrets": skill_row("upload-secrets", "Securely upload local secrets (dotenv files, env vars, API keys) to the Devin Cloud secrets manager — values never enter the conversation", "Builtin", ""),
                FOREIGN_NAME: skill_row(FOREIGN_NAME, FOREIGN_DESCRIPTION, "Devin", self.skills / FOREIGN_NAME),
            }
            self.guard()
        except BaseException:
            self.close()
            raise

    def _directory(self, path):
        require(path.is_absolute() and path.resolve(strict=True) == path and ".." not in path.parts, "directory authority is redirected")
        fd = custody.open_directory(path)
        info = os.fstat(fd)
        if info.st_uid != os.getuid():
            os.close(fd)
            raise ValueError("directory is not owned")
        self.dirs[path] = fd, facts(info)
        return fd

    def _hold(self, relative):
        body, fact = read_file(self.dirs[self.case][0], relative)
        held = None
        try:
            with member(self.dirs[self.case][0], relative) as fd:
                require(facts(os.fstat(fd)) == {k: v for k, v in fact.items() if k != "sha256"}, "file changed before descriptor hold")
                held = os.dup(fd)
        except BaseException:
            if held is not None:
                os.close(held)
            raise
        if relative in self.files:
            self.retired.append(self.files[relative][0])
        self.files[relative] = (held, body, fact)
        return body, fact

    def _failure(self, reason):
        self.finished = False
        if reason not in self.reasons:
            self.reasons.append(reason[:240])

    def _custody_error(self, error):
        failure = CustodyError(str(error))
        if hasattr(self, "binary_fact"):
            failure.audit_evidence = self.evidence()
        return failure

    def _fixed_guard(self):
        for path, (fd, initial) in self.dirs.items():
            custody.named_directory(path, fd)
            now = facts(os.fstat(fd))
            require(all(now[key] == initial[key] for key in ("device", "inode", "mode", "uid", "gid")), "original directory identity changed")
        require(set(os.listdir(self.dirs[self.case][0])) == {"home", "project", "uap-data", "tmp"}, "case root gained a foreign authority")
        require(self.binary.resolve(strict=True) == self.binary
                and facts(self.binary.lstat()) == facts(os.fstat(self.binary_fd)) == {k: v for k, v in self.binary_fact.items() if k != "sha256"}, "pinned binary identity changed")
        require(frame(self.dirs[self.project][0]) == self.initial_project, "User operation changed unrelated project")
        if self.source_frame is not None:
            current = frame(self.dirs[self.source_store][0])
            require(current == self.source_frame, "original independently pinned source store changed")
            self.source_store_proof["last_verified_frame"] = frame_fact(current)
        for name in (".git", ".jj", ".devin", ".agents", ".windsurf", ".claude", ".cursor", ".github", ".zed", "opencode.json", ".mcp.json"):
            require(not os.path.lexists(Path("/") / name), "neutral native cwd imports a project authority")
        for relative, (fd, body, fact) in self.files.items():
            if relative.startswith("uap-data/"):
                continue
            require(facts(os.fstat(fd)) == {k: v for k, v in fact.items() if k != "sha256"}, "held fixed file changed")
            current, observed = read_file(self.dirs[self.case][0], relative)
            require(current == body and observed == fact, "fixed file bytes/identity changed")

    def _state_guard(self):
        for relative, (fd, body, fact) in self.files.items():
            if not relative.startswith("uap-data/"):
                continue
            require(facts(os.fstat(fd)) == {k: v for k, v in fact.items() if k != "sha256"}, "held saved authority changed")
            current, observed = read_file(self.dirs[self.case][0], relative)
            require(current == body and observed == fact, "saved authority changed outside public transition")
        if self.managed_frame is not None and not self.removing:
            with member(self.dirs[self.data][0], self.owner["managed_relative"]) as fd:
                require(frame(fd) == self.managed_frame, "owned managed package changed")

    def guard(self):
        try:
            require(not self.closed, "audit already closed")
            self._fixed_guard()
            require(frame(self.dirs[self.home][0]) == self.home_current, "HOME changed outside exact public transition")
            self._state_guard()
            self.guard_count += 1
        except (OSError, ValueError) as error:
            self._failure("filesystem custody lost: " + str(error))
            raise self._custody_error(error) from error

    def _home_transition(self, owned):
        current = frame(self.dirs[self.home][0])
        selected = [GLOBAL_SKILLS + "/" + name for name in owned]
        for path, previous in self.initial_home.items():
            require(path in current, "original HOME path disappeared")
            now = current[path]
            if path == GLOBAL_SKILLS:
                require(all(now[key] == previous[key] for key in ("kind", "mode", "device", "inode", "uid", "gid")), "Global Skills parent authority changed")
            else:
                require(now == previous, "original HOME bytes/identity changed; no cache exception")
        added = set(current) - set(self.initial_home)
        require(all(any(path == root or path.startswith(root + "/") for root in selected) for path in added), "unowned HOME delta, including cache, is refused")
        require(all(root in added and current[root]["kind"] == "dir" for root in selected), "native owned Skill root is absent")
        return current

    def _manifest_lock(self):
        body, lock_fact = read_file(self.dirs[self.data][0], "uap.lock")
        locked = strict_json(body)
        body, manifest_fact = read_file(self.dirs[self.data][0], "uap.toml")
        manifest = tomllib.loads(body.decode("utf-8", errors="strict"))
        require(type(manifest.get("schema_version")) is int and manifest["schema_version"] == 1
                and set(manifest) <= {"schema_version", "dependencies", "dev-dependencies", "targets"}
                and manifest.get("dev-dependencies", {}) == {}, "portable manifest schema differs")
        return manifest, locked, lock_fact, manifest_fact

    def _bound_lock(self):
        manifest, locked, _, _ = self._manifest_lock()
        require(set(manifest.get("dependencies", {})) == {self.name} and manifest.get("targets") == {"windsurf": [self.name]}, "portable manifest owner/target differs")
        dependency = manifest["dependencies"][self.name]
        # environment_session.manifestDependencyValue writes GitSource.Endpoint.
        require(type(dependency) is dict and set(dependency) == {"git", "commit"}
                and dependency["git"] == "https://github.com/" + self.repo + ".git"
                and dependency["commit"] == self.commit, "portable manifest immutable source differs")
        intent = {"schema_version": 1, "dependencies": [{"alias": self.name, "source_key": self.source_key,
                  "selector": {"kind": "commit", "value": self.commit}}], "targets": [{"id": "windsurf", "aliases": [self.name]}]}
        require(type(locked) is dict and set(locked) == {"lock_version", "manifest_digest", "packages"}
                and type(locked["lock_version"]) is int and locked["lock_version"] == 1
                and locked["manifest_digest"] == "sha256:" + sha(compact(intent))
                and type(locked["packages"]) is list and len(locked["packages"]) == 1, "portable lock intent differs")
        pin = locked["packages"][0]
        require(type(pin) is dict and pin.get("alias") == self.name and pin.get("source_key") == self.source_key
                and pin.get("targets") == ["windsurf"] and pin.get("version") == self.pin["version"]
                and pin.get("digest_algorithm") == "agentplugins-tree-sha256-v1"
                and all(pin.get(k) == self.pin[k] for k in ("tree_digest", "manifest_digest"))
                and pin.get("required_by", []) == [], "portable lock independently pinned package differs")
        require(pin.get("source") == {"git": {"repository": self.repo, "requested": {"kind": "commit", "value": self.commit}, "commit": self.commit}}, "portable lock exact Git tuple differs")
        return locked

    def _public_add(self, document, repeated):
        require(type(document) is dict and type(document.get("schema_version")) is int
                and document["schema_version"] == 1 and document.get("command") == "add" and document.get("result") == "success", "public add envelope differs")
        data = document.get("data")
        require(type(data) is dict and data.get("command") == "agentplugins add" and data.get("result") == "success"
                and data.get("outcome") == "completed" and data.get("scope") == "user"
                and type(data.get("schema_version")) is int and data["schema_version"] == 1
                and type(data.get("actions")) is list and len(data["actions"]) == 1, "public saved add did not complete")
        environment = data.get("environment")
        require(type(environment) is dict and environment.get("saved") is True and environment.get("scope") == "user"
                and environment.get("alias") == self.name and environment.get("selected_targets") == ["windsurf"]
                and environment.get("lock_path") == str(self.data / "uap.lock")
                and environment.get("manifest_path") == str(self.data / "uap.toml"), "public saved environment differs")
        action = data["actions"][0]
        require(type(action) is dict and action.get("alias") == self.name and action.get("client_id") == "windsurf"
                and action.get("action") == ("unchanged" if repeated else "install")
                and action.get("revision") == self.commit and action.get("package_digest") == self.pin["tree_digest"]
                and action.get("to_version") == self.pin["version"], "public immutable add action differs")
        if repeated:
            require(self.owner is not None and action.get("installation_id") == self.owner["uuid"]
                    and action.get("binding_id") == self.owner["binding_id"] and data.get("results") == []
                    and action.get("from_revision") == self.commit and action.get("from_digest") == self.pin["tree_digest"], "public repeat reminted or changed source")
            return self.owner["uuid"], self.owner["skills"]
        components = action.get("component_support")
        require(type(components) is list and 0 < len(components) <= 252, "selected Skill set is missing")
        names = []
        for component in components:
            require(type(component) is dict and set(component) <= {"kind", "name", "support", "reason"}
                    and component.get("kind") == "skill" and component.get("support") == "prepared"
                    and type(component.get("name")) is str and re.fullmatch(r"[a-z0-9]+(?:-[a-z0-9]+)*", component["name"])
                    and len(component["name"]) <= 64 and component["name"].casefold() not in {n.casefold() for n in names}, "selected component is not an unambiguous pure Skill")
            names.append(component["name"])
        require(sorted(names) == sorted(self.pin["skills"]), "selected Skills differ from the complete immutable source set")
        results = data.get("results")
        require(type(results) is list and len(results) == 1 and type(results[0]) is dict
                and results[0].get("client_id") == "windsurf" and results[0].get("operation") == "install"
                and results[0].get("outcome") == "completed", "public installation owner is missing")
        uid = results[0].get("installation_id")
        require(type(uid) is str and re.fullmatch(r"[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}", uid), "public owner is not a canonical fresh UUID")
        require(not set(names).intersection(self.baseline), "owned Skill collides with foreign/builtin inventory")
        return uid, sorted(names)

    def _state_owner(self, uid, skills):
        state = strict_json(read_file(self.dirs[self.data][0], "state-v2.json")[0])
        require(type(state) is dict and type(state.get("schema_version")) is int and state["schema_version"] == 5
                and type(state.get("installations")) is list and len(state["installations"]) == 1, "saved state-v2 owner schema differs")
        installed = state["installations"][0]
        require(type(installed) is dict and installed.get("installation_id") == uid and installed.get("declared_name") == self.name
                and installed.get("origin_mode", "direct") == "direct" and "directory" not in installed, "saved immutable owner differs")
        expected_source = {"source_binding_id": "src_" + sha((self.source_key + "\0" + self.repo + "\0").encode())[:24],
                           "requested_source": self.source, "canonical_source": self.source, "repository": self.repo,
                           "resolved_revision": self.commit, "tree_digest": self.pin["tree_digest"]}
        require(same(installed.get("source"), expected_source), "saved source is not the independently pinned immutable tuple")
        require(self.source_frame is None, "immutable source authority was already bound")
        source_fd = self._directory(self.source_store)
        source_frame = frame(source_fd)
        actual_digest = immutable_source_digest(source_fd, source_frame)
        require(actual_digest == self.pin["tree_digest"], "actual immutable source bytes do not match the independent tree pin")
        source_skills = sorted(path.split("/")[1] for path, row in source_frame.items()
                               if row["kind"] == "file" and re.fullmatch(r"skills/[^/]+/SKILL\.md", path))
        require(source_skills == sorted(self.pin["skills"]), "cached source lacks its complete independently closed Skill set")
        self.source_frame = source_frame
        self.source_store_proof = {"path": str(self.source_store), "algorithm": "agentplugins-tree-sha256-v1",
            "tree_digest": actual_digest, "root_identity": source_frame[""], "held_frame": frame_fact(source_frame),
            "last_verified_frame": frame_fact(source_frame), "complete_skills": source_skills}
        package = installed.get("package")
        require(type(package) is dict and package.get("declared_name") == self.name and package.get("version") == self.pin["version"]
                and package.get("manifest_digest") == self.pin["manifest_digest"]
                and type(package.get("inventory")) is dict and type(package["inventory"].get("skills")) is list
                and sorted(package["inventory"]["skills"]) == skills, "saved package inventory/manifest differs")
        clients = installed.get("clients")
        require(type(clients) is dict and len(clients) == 1, "saved owner has ambiguous target bindings")
        key, binding = next(iter(clients.items()))
        physical = self.name[:51].rstrip(".-") + "-" + sha(uid.encode())[:12]
        managed_relative = "managed/clients/windsurf/" + physical
        managed = self.data / managed_relative
        binding_id = "client_" + sha("\0".join((uid, "windsurf", "user", str(managed))).encode())[:24]
        require(type(binding) is dict and key == binding_id and binding.get("client_binding_id") == binding_id
                and binding.get("client_id") == "windsurf" and binding.get("scope") == "user"
                and binding.get("physical_artifact_id") == physical and binding.get("target_locator") == str(managed)
                and binding.get("native_profile_root") == str(self.profile)
                and binding.get("materialization") == "materialized" and binding.get("policy") == "allowed"
                and binding.get("verification") == "installation_verified" and binding.get("affected_surfaces") == ["windsurf"]
                and same(binding.get("package_revision"), self.revision), "saved source/UUID/revision/native profile join differs")
        objects = binding.get("native_objects")
        require(type(objects) is list and len(objects) == len(skills) + 1 and all(type(obj) is dict for obj in objects), "native receipt coverage differs")
        packages = [obj for obj in objects if obj.get("kind") == "managed_package_directory"]
        require(len(packages) == 1 and type(packages[0].get("managed_digest")) is str
                and re.fullmatch(r"sha256:[0-9a-f]{64}", packages[0]["managed_digest"]), "managed package digest receipt is missing")
        expected_package = {"object_id": "package:windsurf:" + physical, "kind": "managed_package_directory",
                            "logical_name": self.name, "path": str(managed), "managed_digest": packages[0]["managed_digest"], "protection_class": "managed"}
        require(same(packages[0], expected_package), "managed package receipt is not exact")
        native_rows = {}
        with member(self.dirs[self.data][0], managed_relative) as managed_fd:
            managed_frame = frame(managed_fd)
            require(managed_digest(managed_fd, managed_frame) == expected_package["managed_digest"], "actual managed package differs from its receipt")
            for name in skills:
                found = [obj for obj in objects if obj.get("kind") == KIND and obj.get("logical_name") == name]
                require(len(found) == 1, "native Skill receipt is missing or duplicated")
                obj = found[0]
                relative = obj.get("source_relative")
                require(relative == "skills/" + name, "native receipt differs from the fixed immutable Skill path")
                require(same(obj, {"object_id": KIND + ":" + name, "kind": KIND, "logical_name": name,
                                   "path": str(self.skills / name), "source_relative": relative,
                                   "managed_digest": obj.get("managed_digest"), "protection_class": "managed"})
                        and type(obj.get("managed_digest")) is str and re.fullmatch(r"sha256:[0-9a-f]{64}", obj["managed_digest"]), "native Skill receipt authority differs")
                with member(source_fd, relative) as cached_fd, member(managed_fd, relative) as managed_skill_fd, member(self.dirs[self.home][0], GLOBAL_SKILLS + "/" + name) as native_fd:
                    cached_frame, managed_skill_frame, native_frame = frame(cached_fd), frame(managed_skill_fd), frame(native_fd)
                    require(content(cached_frame) == content(managed_skill_frame) == content(native_frame)
                            and skill_digest(cached_fd, cached_frame) == obj["managed_digest"]
                            and skill_digest(managed_skill_fd, managed_skill_frame) == obj["managed_digest"]
                            and skill_digest(native_fd, native_frame) == obj["managed_digest"], "actual source/native Skill bytes differ from receipt")
                    text = description(read_file(cached_fd, "SKILL.md")[0], name)
                native_rows[name] = skill_row(name, text, "Devin", self.skills / name)
            # Keep the original directory inode alive across its later removal.
            self.managed_fd = os.dup(managed_fd)
            self.retired.append(self.managed_fd)
        qualified = (binding.get("install_intent", "") == "" and binding.get("activation") == "active"
                     and binding.get("authentication") == "not_required")
        if not qualified:
            self._failure("public/saved native lifecycle is not automatic Active/not_required")
        owner = {"uuid": uid, "name": self.name, "binding_id": binding_id, "physical": physical,
                 "managed_relative": managed_relative, "skills": skills, "source": installed["source"],
                 "package": package, "revision": self.revision, "native_objects": objects,
                 "native_rows": native_rows, "native_lifecycle_qualified": qualified}
        return owner, managed_frame

    def after_public_transition(self, phase, document, outcome, argv):
        """Close the pending UAP delta before any guard/next child can trust it."""
        try:
            self._fixed_guard()
            require(phase in ("add", "repeat", "remove") and type(outcome) is dict
                    and type(outcome.get("exit_code")) is int and outcome["exit_code"] == 0
                    and outcome.get("timeout") is False and type(outcome.get("stdout_full")) is str,
                    "public transition did not complete with full capture")
            encoded = outcome["stdout_full"].encode("utf-8")
            require(len(encoded) <= LIMIT and same(strict_json(encoded), document), "public full-stream join differs")
            selector = self.owner["uuid"] if phase == "remove" and self.owner is not None else self.item
            expected = ["remove" if phase == "remove" else "add", selector, "--target", "windsurf", "--scope", "user", "--format", "json"]
            require(type(argv) is list and len(argv) == len(expected) + 1 and argv[1:] == expected, "public transition argv differs")
            if phase == "add":
                require(self.owner is None and len(self.observations) == 1, "initial public add order differs")
                uid, skills = self._public_add(document, False)
                self._bound_lock()
                owner, managed_frame = self._state_owner(uid, skills)
                current = self._home_transition(skills)
                self.owner, self.managed_frame, self.home_current = owner, managed_frame, current
                for filename in ("state-v2.json", "uap.lock", "uap.toml"):
                    self._hold("uap-data/" + filename)
                self.initial_agreement = {name: copy.deepcopy(self.files["uap-data/" + name][2]) for name in ("state-v2.json", "uap.lock", "uap.toml")}
            elif phase == "repeat":
                require(self.owner is not None and len(self.observations) == 2, "public repeat order differs")
                self._public_add(document, True)
                self._bound_lock()
                # The no-op wire result omits component_support; the original
                # complete bound set and all saved/owned identities remain fixed.
                self.guard()
            else:
                require(self.removing and self.owner is not None, "remove lacks original owned installation")
                self._removed(document)
            self.transitions.append({"phase": phase, "argv": copy.deepcopy(argv), "stdout_bytes": len(encoded), "stdout_sha256": sha(encoded), "verified": True})
            self.guard()
            return True
        except (OSError, ValueError, TypeError, KeyError, UnicodeError, RecursionError) as error:
            self._failure("public " + str(phase) + ": " + str(error))
            raise self._custody_error(error) from error

    def bind_installation(self, document, *, repeated=False):
        """The independent public list must confirm the already closed owner."""
        self.guard()
        phase = "repeat" if repeated else "add"
        try:
            require(self.owner is not None and type(document) is dict and type(document.get("schema_version")) is int
                    and document["schema_version"] == 1 and document.get("command") == "list" and document.get("result") == "success", "public list envelope differs")
            nodes = document["data"]["installations"]
            require(type(nodes) is list and len(nodes) == 1 and type(nodes[0]) is dict, "public owner list is not sole")
            node = nodes[0]
            require(node.get("installation_id") == self.owner["uuid"] and node.get("name") == self.name
                    and node.get("source") == self.repo and node.get("version") == self.pin["version"]
                    and node.get("mixed_version") is False, "public list source/UUID/version differs")
            clients = node.get("clients")
            expected = {"client_id": "windsurf", "scope": "user", "materialization": "materialized", "activation": "active",
                        "authentication": "not_required", "policy": "allowed", "verification": "installation_verified",
                        "package_revision": self.revision, "affected_surfaces": ["windsurf"]}
            require(type(clients) is list and len(clients) == 1 and same(clients[0], expected)
                    and self.owner["native_lifecycle_qualified"], "public list lacks exact automatic native lifecycle")
            self.bound_list.add(phase)
            return True
        except (ValueError, TypeError, KeyError):
            self._failure("public " + phase + " list does not confirm original native owner")
            return False

    def _removed(self, document):
        uid = self.owner["uuid"]
        require(type(document) is dict and type(document.get("schema_version")) is int and document["schema_version"] == 1
                and document.get("command") == "agentplugins remove" and document.get("scope") == "user"
                and document.get("outcome") == "completed" and document.get("result") == "success", "public saved remove did not complete")
        actions = document.get("actions")
        require(type(actions) is list and len(actions) == 1 and type(actions[0]) is dict
                and all(actions[0].get(key) == value for key, value in {
                    "alias": self.name, "client_id": "windsurf", "installation_id": uid, "binding_id": self.owner["binding_id"],
                    "action": "remove", "from_revision": self.commit, "from_digest": self.pin["tree_digest"]}.items())
                and document.get("results") == [{"installation_id": uid, "client_id": "windsurf", "operation": "remove", "outcome": "completed"}], "public remove changed original owner")
        manifest, locked, lock_fact, manifest_fact = self._manifest_lock()
        require(same(locked, {"lock_version": 1, "manifest_digest": EMPTY_INTENT, "packages": []})
                and all(type(value) is dict and value == {} for key, value in manifest.items() if key != "schema_version"), "remove did not publish the exact empty agreement")
        old = self.files["uap-data/uap.lock"][2]
        require(lock_fact["device"] == old["device"] and lock_fact["inode"] != old["inode"], "saved remove did not atomically replace original lock")
        state = strict_json(read_file(self.dirs[self.data][0], "state-v2.json")[0])
        require(type(state) is dict and type(state.get("schema_version")) is int and state["schema_version"] == 5
                and type(state.get("installations")) is list and len(state["installations"]) <= 1, "removed saved state is ambiguous")
        for item in state["installations"]:
            require(type(item) is dict and item.get("installation_id") == uid and item.get("declared_name") == self.name
                    and item.get("clients") == {} and item.get("data_retained") is True
                    and same(item.get("source"), self.owner["source"]) and same(item.get("package"), self.owner["package"]), "removed saved state reminted or retained an active owner")
        require(not os.path.lexists(self.data / self.owner["managed_relative"]), "removed managed package remains")
        require(self.managed_fd is not None and os.fstat(self.managed_fd).st_nlink == 0,
                "original managed package remains linked")
        self.home_current = self._home_transition(())
        for filename in ("state-v2.json", "uap.lock", "uap.toml"):
            self._hold("uap-data/" + filename)
        self.removal_agreement = {"lock": lock_fact, "manifest": manifest_fact, "manifest_digest": EMPTY_INTENT}

    def begin_remove(self):
        self.guard()
        self.removing = True
        return self.owner is not None

    def confirm_removed_list(self, document):
        self.guard()
        valid = (type(document) is dict and type(document.get("schema_version")) is int and document["schema_version"] == 1
                 and document.get("command") == "list" and document.get("result") == "success"
                 and type(document.get("data")) is dict and document["data"].get("installations") == [])
        if valid:
            self.bound_list.add("remove")
        else:
            self._failure("removed public target list contains a reminted name/source/UUID")
        return valid

    def begin_probe(self, phase):
        self.guard()
        require(self.probe_frame is None and phase in PHASES
                and phase == PHASES[len(self.observations)], "native observation order differs")
        self.probe_frame = phase, frame(self.dirs[self.case][0])

    def observe(self, phase, record, stdout, stderr):
        capture = {"phase": phase, "record": copy.deepcopy(record), "verified": False}
        for key, body in (("stdout", stdout), ("stderr", stderr)):
            if type(body) is bytes and len(body) <= LIMIT:
                capture[key] = {"bytes": len(body), "sha256": sha(body), "base64": base64.b64encode(body).decode()}
        self.observations.append(capture)
        try:
            require(self.probe_frame is not None and self.probe_frame[0] == phase, "native probe lacks its original whole-case frame")
            before = self.probe_frame[1]
            after = frame(self.dirs[self.case][0])
            capture["custody"] = {"before": frame_fact(before), "after": frame_fact(after), "unchanged": before == after}
            if before != after:
                capture["custody"]["changes"] = frame_changes(before, after)
            require(before == after, "native probe mutated HOME/tmp/project/state/locks/managed files; no cache exception")
            self.probe_frame = None
            self.guard()
        except (OSError, ValueError) as error:
            self._failure("native custody: " + str(error))
            raise self._custody_error(error) from error
        try:
            require([row["phase"] for row in self.observations] == list(PHASES[:len(self.observations)]), "native phase order differs")
            require(type(record) is dict and set(record) == PROBE_KEYS and record["probe"] == "windsurf-global-" + phase
                    and record["argv"] == [str(self.binary), "skills", "list", "--json"] and record["cwd"] == "/"
                    and record["error"] is None and type(record["exit"]) is int and record["exit"] == 0, "original native command failed or differs")
            elapsed = record["elapsed_seconds"]
            require(type(elapsed) in (int, float) and math.isfinite(elapsed) and 0 <= elapsed <= 20, "native probe elapsed bound differs")
            for key, body in (("stdout", stdout), ("stderr", stderr)):
                require(key in capture and type(record[key + "_bytes"]) is int and record[key + "_bytes"] == len(body)
                        and record[key + "_sha256"] == sha(body) and record[key + "_file"] == record["probe"] + "." + key + ".log", "native complete stream join differs")
            require(stderr == b"", "native metadata observation emitted stderr")
            expected = dict(self.baseline)
            if phase in ("add", "repeat"):
                require(self.owner is not None and phase in self.bound_list and self.owner["native_lifecycle_qualified"], "native inventory lacks its public/receipt owner")
                expected.update(self.owner["native_rows"])
            elif phase == "remove":
                require(self.removing and "remove" in self.bound_list and hasattr(self, "removal_agreement"), "native removal lacks empty public/saved agreement")
            require(same(inventory(stdout), expected), "native inventory differs from complete source-bound owner/foreign set")
            capture["verified"] = True
            return True
        except (ValueError, TypeError, KeyError, UnicodeError, RecursionError) as error:
            capture["reason"] = str(error)[:240]
            self._failure("native " + str(phase) + ": " + str(error))
            return False

    def finish(self):
        self.finished = False
        self.guard()
        self.finished = (not self.reasons and self.removing and self.probe_frame is None
                         and self.bound_list == {"add", "repeat", "remove"}
                         and [v["phase"] for v in self.observations] == list(PHASES)
                         and all(v["verified"] is True for v in self.observations)
                         and [v["phase"] for v in self.transitions] == ["add", "repeat", "remove"])
        return self.finished

    def evidence(self):
        return copy.deepcopy({"scenario": "four-immutable-global-pure-skills", "native_discovery_verified": self.finished,
            "writer_closure_claimed": False, "cache_exception_granted": False, "case_root": str(self.case),
            "source": self.item, "independent_source_pin": self.pin, "owner": self.owner,
            "immutable_source_store": getattr(self, "source_store_proof", None),
            "profile_authored_before_first_child": {"files": self.fixed, "home_frame": frame_fact(self.initial_home),
                "global_root": str(self.profile), "native_cwd": "/", "last_home_frame": frame_fact(self.home_current)},
            "binary": {"path": str(self.binary), "version": self.client_version, **self.binary_fact},
            "successful_guards": self.guard_count, "original_agreement": getattr(self, "initial_agreement", None),
            "removed_agreement": getattr(self, "removal_agreement", None), "public_transitions": self.transitions,
            "observations": self.observations, "reasons": self.reasons})

    def close(self):
        if not self.closed:
            self.closed = True
            for fd in [value[0] for value in self.dirs.values()] + [value[0] for value in self.files.values()] + self.retired:
                os.close(fd)
