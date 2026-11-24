"""Additive Grok 1.0.70 raw-Skill proof in an owner-pretrusted disposable profile.

No child is started here. The caller passes original run_probe(memory_only=True)
records and held byte streams, and retains public source/lock/removal checks.
This is neither cold activation nor authentication, MCP, model, or escaped-writer
closure evidence. The cold audit's acceptance and counters are not changed.
"""
import base64
import copy
import hashlib
import json
import math
import os
from pathlib import Path
import re
import stat
import tomllib

from native_client_project_artifacts import identity
from project_native_runtime import strict_json, grok_private_credential

LIMIT = 1 << 20
PHASES = ("before", "add", "repeat", "remove")
VERSION = "grok 1.0.70 (f50904eaee6c)"
BINARY_SHA256 = "22d281c5964c5444147328096ba2a221f3f0874c6858e27c825656a80f6e58e9"
BINARY_LIMIT = 256 << 20
# environment.ManifestDigest's normalized empty manifestIntent (nil slices).
EMPTY_INTENT = "sha256:" + hashlib.sha256(b'{"schema_version":1,"dependencies":null,"targets":null}').hexdigest()
PROBE_KEYS = set("probe argv cwd error exit elapsed_seconds stdout_file stderr_file "
                 "stdout_bytes stderr_bytes stdout_sha256 stderr_sha256".split())


class CustodyError(ValueError):
    """Stop further public/native work when original filesystem authority is lost."""


def require(value, message):
    if not value:
        raise ValueError(message)


def sha(body):
    return hashlib.sha256(body).hexdigest()


def same(left, right):
    return json.dumps(left, sort_keys=True, allow_nan=False) == json.dumps(right, sort_keys=True, allow_nan=False)


def canonical(value):
    raw = os.fspath(value)
    path = Path(raw)
    require(path.is_absolute() and str(path) == raw and ".." not in path.parts
            and not any(char in raw for char in "\x00\r\n"), "noncanonical Grok path")
    return path


def fixture_name(item):
    require(type(item) is dict, "Grok immutable fixture is missing")
    for field in ("repo", "name", "folder", "ref", "source", "skill_md_git_blob_sha1"):
        require(type(item.get(field)) is str, "Grok fixture field type differs")
    require(re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", item["repo"])
            and re.fullmatch(r"[a-z0-9]+(?:-[a-z0-9]+)*", item["name"])
            and re.fullmatch(r"[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)*", item["folder"])
            and not {".", ".."}.intersection(item["folder"].split("/"))
            and re.fullmatch(r"[a-f0-9]{40}", item["ref"])
            and re.fullmatch(r"[a-f0-9]{40}", item["skill_md_git_blob_sha1"]), "Grok fixture identity differs")
    require(item["source"] == f'https://github.com/{item["repo"]}/tree/{item["ref"]}/skills/{item["folder"]}',
            "Grok immutable source differs")
    key = f'github-skill:{item["repo"].lower()}:{item["name"]}:skills/{item["folder"]}'
    return "skill-" + sha(key.encode())[:56]


def skill_description(body, item):
    """Use bounded YAML nodes; metadata is derived only after the pinned blob join."""
    require(type(body) is bytes and len(body) <= LIMIT and hashlib.sha1(
        b"blob " + str(len(body)).encode() + b"\0" + body).hexdigest() == item["skill_md_git_blob_sha1"],
        "Grok Skill bytes differ from the immutable Git blob")
    try:
        import yaml  # Provisioned only for the additive row; no cold-lane dependency.
        text = body.decode("utf-8-sig", errors="strict").replace("\r\n", "\n")
        require(text.startswith("---\n"), "Grok Skill frontmatter opening differs")
        rest = text[4:]
        end = rest.find("\n---\n")
        if end < 0 and rest.endswith("\n---"):
            end = len(rest) - 4
        require(end >= 0, "Grok Skill frontmatter is not closed")
        front = rest[:end]
        require(len(front.encode()) <= 64 << 10, "Grok Skill frontmatter exceeds bound")

        class Loader(yaml.SafeLoader):
            pass

        def mapping(loader, node):
            result = {}
            for key_node, value_node in node.value:
                key = loader.construct_object(key_node, deep=True)
                require(type(key) is str and key not in result, "duplicate/nonstring Grok YAML key")
                result[key] = loader.construct_object(value_node, deep=True)
            return result

        Loader.add_constructor(yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG, mapping)
        nodes, members, active = 0, 0, set()

        def bounded(node, depth):
            nonlocal nodes, members
            nodes += 1
            require(nodes <= 100000 and depth <= 64 and id(node) not in active,
                    "Grok YAML node/depth/alias bound")
            active.add(id(node))
            if isinstance(node, yaml.MappingNode):
                members += len(node.value)
                require(members <= 10000, "Grok YAML member bound")
                for key, value in node.value:
                    bounded(key, depth + 1)
                    bounded(value, depth + 1)
            elif isinstance(node, yaml.SequenceNode):
                for value in node.value:
                    bounded(value, depth + 1)
            active.remove(id(node))

        # Refuse excessive parse nesting before PyYAML composes recursive nodes.
        depth = 0
        for event in yaml.parse(front, Loader=Loader):
            if isinstance(event, (yaml.MappingStartEvent, yaml.SequenceStartEvent)):
                depth += 1
                require(depth <= 64, "Grok YAML parse depth bound")
            elif isinstance(event, (yaml.MappingEndEvent, yaml.SequenceEndEvent)):
                depth -= 1
        loader = Loader(front)
        try:
            node = loader.get_single_node()
            require(isinstance(node, yaml.MappingNode), "Grok YAML must be one mapping")
            bounded(node, 0)
            value = loader.construct_object(node, deep=True)
        finally:
            loader.dispose()
        require(value.get("name") == item["name"] and type(value.get("description")) is str
                and value["description"].strip() and len(value["description"]) <= 1024,
                "Grok Skill name/description differs")
        return value["description"]
    except (ImportError, UnicodeError, RecursionError) as error:
        raise ValueError("Grok bounded Skill metadata parser unavailable") from error
    except Exception as error:
        if isinstance(error, ValueError):
            raise
        raise ValueError("Grok Skill YAML is malformed") from error


def directory_identity(info):
    return info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid


def file_identity(info):
    return (*identity(info), info.st_uid, info.st_gid)


def held_fact(path, record):
    _, frozen, body = record
    device, inode, mode, links, size, modified, changed, uid, gid = frozen
    fact = {"path": str(path), "device": device, "inode": inode, "mode": oct(stat.S_IMODE(mode)),
            "links": links, "bytes": size, "mtime_ns": modified, "ctime_ns": changed, "uid": uid, "gid": gid}
    if body is not None:
        fact.update(sha256=sha(body), base64=base64.b64encode(body).decode())
    return fact


def inventory(document, project):
    required = set("grokVersion cwd projectRoot projectTrusted plugins skills mcpServers".split())
    optional = {"channel": str, "updateChannel": (str, type(None)),
                **dict.fromkeys("projectInstructions hooks agents marketplaces lspServers experiments".split(), list),
                **dict.fromkeys("permissions loginPolicy configSources externalCompat".split(), dict)}
    require(type(document) is dict and required <= set(document) and set(document) <= required | optional.keys(),
            "Grok inspection envelope differs")
    require(document["grokVersion"] == "1.0.70" and document["cwd"] == str(project)
            and document["projectRoot"] in (str(project), str(project) + "/")
            and document["projectTrusted"] is True and document["mcpServers"] == [],
            "Grok inspection version/project/trust/MCP differs")
    for key, kind in optional.items():
        require(key not in document or isinstance(document[key], kind), "Grok diagnostic field type differs")
    result = {}
    for kind in ("plugins", "skills"):
        rows, parsed = document[kind], {}
        require(type(rows) is list and len(rows) <= 1024, "Grok inventory bound")
        for row in rows:
            require(type(row) is dict and type(row.get("name")) is str and row["name"].strip(),
                    "Grok inventory name differs")
            name = row["name"].casefold()
            require(name not in parsed, "Grok inventory has folded duplicate identity")
            if kind == "plugins":
                require(set(row) == {"name", "scope", "path", "enabled", "provides"}
                        and row["scope"] in ("project", "user") and type(row["enabled"]) is bool,
                        "Grok plugin fields differ")
                canonical(row["path"])
                provides = row["provides"]
                require(type(provides) is dict and set(provides) == {"skills", "agents", "hooks", "mcpServers"}
                        and type(provides["hooks"]) is bool and all(type(provides[key]) is int and provides[key] >= 0
                        for key in ("skills", "agents", "mcpServers")), "Grok plugin counts differ")
            else:
                base = {"name", "description", "source", "userInvocable"}
                extras = {"disabled", "vendor", "compatibilityStatus", "collidesWith", "invocableAs"}
                require(base <= set(row) <= base | extras and type(row["description"]) is str
                        and type(row["userInvocable"]) is bool, "Grok Skill fields differ")
                for key in set(row) & extras:
                    value = row[key]
                    require((type(value) is bool if key == "disabled" else value in ("enabled", "disabled")
                             if key == "compatibilityStatus" else type(value) is str and bool(value)),
                            "Grok optional Skill field differs")
                origin = row["source"]
                require(type(origin) is dict and type(origin.get("path")) is str and origin["path"],
                        "Grok Skill source differs")
                expected = {"type", "path", "plugin_name"} if origin.get("type") == "plugin" else {"type", "path"}
                require(set(origin) == expected and origin.get("type") in
                        ("plugin", "project", "user", "bundled", "configToml", "server")
                        and ("plugin_name" not in origin or type(origin["plugin_name"]) is str and origin["plugin_name"]),
                        "Grok Skill origin differs")
            parsed[name] = copy.deepcopy(row)
        result[kind] = parsed
    return result


class GrokPretrustedSkillAudit:
    def __init__(self, case_root, home, project, item, executable, client_version):
        self.directories, self.files, self.captures, self.reasons = {}, {}, [], []
        self.closed = self.removing = self.finished = False
        self.guard_count, self.last_guard_passed, self.last_fixed = 0, False, {}
        self.retired_files, self.removal_agreement = [], None
        self.binding = self.active = self.baseline = None
        self.bound_phases = set()
        try:
            require(os.name == "posix" and hasattr(os, "O_NOFOLLOW"), "Grok custody requires POSIX nofollow")
            self.case, self.home, self.project = map(canonical, (case_root, home, project))
            self.executable, self.item = canonical(executable), copy.deepcopy(item)
            self.client_version = copy.deepcopy(client_version)
            self.name = fixture_name(self.item)
            self.source_key = f'github-skill:{self.item["repo"].lower()}:{self.item["name"]}:skills/{self.item["folder"]}'
            self.lock = self.project / "uap.lock"
            require(self.home == self.case / "home" and self.project == self.case / "project", "Grok sibling roots differ")
            require(type(client_version) is dict and client_version.get("status") == "reported"
                    and client_version.get("value") == VERSION and type(client_version.get("exit_code")) is int
                    and client_version["exit_code"] == 0, "Grok pinned version evidence differs")
            for path in (self.case, self.home, self.project, self.case / "uap-data"):
                fd = self._directory(path)
                info = os.fstat(fd)
                require(info.st_uid == os.getuid() and stat.S_IMODE(info.st_mode) == 0o700, "Grok roots must be private owned directories")
            require(not os.listdir(self.directories[self.case / "uap-data"][0]), "Grok UAP data must be cold")
            for path in (self.project / "uap.toml", self.project / "uap.lock", self.project / ".uap", self.project / ".git"):
                require(not os.path.lexists(path), "Grok fixture already contains installation or child state")
            self.profile = self.home / ".grok"
            try:
                os.mkdir(".grok", 0o700, dir_fd=self.directories[self.home][0])
            except FileExistsError:
                pass
            profile_fd = self._directory(self.profile)
            info = os.fstat(profile_fd)
            require(info.st_uid == os.getuid() and not stat.S_IMODE(info.st_mode) & 0o022, "Grok profile ownership differs")
            if os.path.lexists(self.project / ".grok"):
                self._directory(self.project / ".grok")
            self._empty_plugins()
            config = ('[marketplace]\ndefault_skills_installs_purged = true\n[plugins]\nenabled = ['
                      + json.dumps(self.name) + ']\ndisabled = []\n[features]\nturn_summary = false\n').encode()
            trust = ('[folders.' + json.dumps(str(self.project), ensure_ascii=False) + ']\ntrusted = true\n').encode()
            for name, body in (("config.toml", config), ("trusted_folders.toml", trust)):
                fd = os.open(name, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW | os.O_CLOEXEC,
                             0o600, dir_fd=profile_fd)
                try:
                    with os.fdopen(fd, "wb", closefd=False) as stream:
                        stream.write(body)
                        stream.flush()
                finally:
                    os.close(fd)
                self._file(self.profile / name, body=body)
                require(stat.S_IMODE(os.fstat(self.files[self.profile / name][0]).st_mode) == 0o600,
                        "Grok authored profile file mode differs")
            foreign = {
                self.project / "foreign-untouched.txt": b"foreign content never belongs to UAP\n",
                self.home / "foreign-untouched.txt": b"foreign profile content never belongs to UAP\n",
                self.profile / "foreign-neighbor.txt": b"unmanaged native neighbor must survive\n",
                self.project / ".grok/skills/foreign-neighbor/SKILL.md":
                    b"---\nname: foreign-neighbor\ndescription: unrelated\n---\n",
            }
            for path, body in foreign.items():
                self._file(path, body=body)
            self.fixed_paths = [self.profile / "config.toml", self.profile / "trusted_folders.toml", *foreign]
            self.initial_fixed = {str(path.relative_to(self.case)): held_fact(path, self.files[path]) for path in self.fixed_paths}
            self._file(self.executable, executable=True)
            self.original_directories = set(self.directories)
            self.guard()
        except BaseException:
            self.close()
            raise

    def _directory(self, path):
        if path in self.directories:
            return self.directories[path][0]
        parent = self._directory(path.parent) if path != Path("/") else None
        try:
            fd = os.open(path.name if parent is not None else "/", os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=parent)
        except FileNotFoundError:
            raise
        except OSError as error:
            raise CustodyError("Grok directory cannot be opened without following links") from error
        try:
            info = os.fstat(fd)
            named = os.stat(path.name, dir_fd=parent, follow_symlinks=False) if parent is not None else path.lstat()
            require(directory_identity(info) == directory_identity(named), "Grok directory changed during capture")
            if path.is_relative_to(self.case):
                require(info.st_uid == os.getuid() and not stat.S_IMODE(info.st_mode) & 0o022,
                        "Grok case directory ownership/mode differs")
            self.directories[path] = fd, directory_identity(info)
            return fd
        except (OSError, ValueError) as error:
            os.close(fd)
            raise CustodyError(str(error)) from error

    def _file(self, path, *, body=None, executable=False):
        if path in self.files:
            return self.files[path][2]
        parent = self._directory(path.parent)
        try:
            fd = os.open(path.name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC, dir_fd=parent)
        except FileNotFoundError:
            raise
        except OSError as error:
            raise CustodyError("Grok file cannot be opened without following links") from error
        try:
            info = os.fstat(fd)
            if (not stat.S_ISREG(info.st_mode) or info.st_nlink != 1 or info.st_uid not in (os.getuid(), 0)
                    or stat.S_IMODE(info.st_mode) & 0o022
                    or not (bool(info.st_mode & 0o111) if executable else info.st_uid == os.getuid())):
                raise CustodyError("Grok held file ownership/type/mode differs")
            if executable:
                observed = None
                require(0 < info.st_size <= BINARY_LIMIT, "Grok executable exceeds its bounded pin")
                digest, offset = hashlib.sha256(), 0
                while offset < info.st_size:
                    chunk = os.pread(fd, min(LIMIT, info.st_size - offset), offset)
                    require(chunk, "Grok executable ended before its held size")
                    digest.update(chunk)
                    offset += len(chunk)
                require(not os.pread(fd, 1, info.st_size) and digest.hexdigest() == BINARY_SHA256,
                        "Grok executable does not match the qualified Linux x86_64 binary")
                self.executable_digest = digest.hexdigest()
            else:
                require(info.st_size <= LIMIT, "Grok held file exceeds bound")
                observed = os.pread(fd, info.st_size + 1, 0)
                require(len(observed) == info.st_size and (body is None or observed == body), "Grok held bytes differ")
            if (file_identity(info) != file_identity(os.fstat(fd))
                    or file_identity(info) != file_identity(os.stat(path.name, dir_fd=parent, follow_symlinks=False))):
                raise CustodyError("Grok held file changed during capture")
            self.files[path] = fd, file_identity(info), observed
            return observed
        except BaseException:
            os.close(fd)
            raise

    def _empty_plugins(self):
        for path in (self.project / ".grok/plugins", self.profile / "plugins"):
            if os.path.lexists(path):
                fd = self._directory(path)
                require(not os.listdir(fd), "Grok pretrusted fixture already contains plugin packages")

    def guard(self):
        self.last_guard_passed = False
        try:
            require(not self.closed, "Grok custody is closed")
            for path, (fd, expected) in self.directories.items():
                parent = self.directories[path.parent][0] if path != Path("/") else None
                try:
                    named = os.stat(path.name, dir_fd=parent, follow_symlinks=False) if parent is not None else path.lstat()
                except FileNotFoundError:
                    if (self.removing and self.active is not None and path not in self.original_directories
                            and (path.is_relative_to(self.active) or self.active.is_relative_to(path))):
                        continue
                    raise
                require(directory_identity(os.fstat(fd)) == expected == directory_identity(named),
                        "Grok original directory authority changed")
            for path, (fd, expected, body) in list(self.files.items()):
                parent = self.directories[path.parent][0]
                try:
                    named = os.stat(path.name, dir_fd=parent, follow_symlinks=False)
                except FileNotFoundError:
                    if self.removing and (path == self.lock or self.active is not None and path.is_relative_to(self.active)):
                        require(path != self.lock or self.removal_agreement is None,
                                "Grok verified empty lock disappeared")
                        continue
                    raise
                if (self.removing and path == self.lock and self.removal_agreement is None
                        and file_identity(named) != expected):
                    self._accept_removed_agreement()
                    continue
                require(file_identity(os.fstat(fd)) == expected == file_identity(named),
                        "Grok held file identity changed")
                if body is not None:
                    require(os.pread(fd, len(body) + 1, 0) == body, "Grok held file bytes changed")
                    require(file_identity(os.fstat(fd)) == expected == file_identity(os.stat(path.name, dir_fd=parent, follow_symlinks=False)),
                            "Grok held file changed during verification")
            if not self.captures:
                self._empty_plugins()
            self.guard_count += 1
            self.last_guard_passed = True
            self.last_fixed = {str(path.relative_to(self.case)): held_fact(path, self.files[path]) for path in self.fixed_paths}
        except (OSError, ValueError) as error:
            self._failed("filesystem custody lost")
            raise CustodyError(str(error)) from error

    def _accept_removed_agreement(self):
        """Admit only saved remove's one atomic transition to an empty agreement."""
        original = self.files.pop(self.lock)
        try:
            locked = strict_json(self._file(self.lock))
            current = self.files[self.lock][1]
            require(current[0] == original[1][0] and current[1] != original[1][1],
                    "Grok saved removal did not publish an atomic lock replacement")
            require(same(locked, {"lock_version": 1, "manifest_digest": EMPTY_INTENT, "packages": []}),
                    "Grok removal lock is not the exact empty saved agreement")
            manifest = self.project / "uap.toml"
            intent = tomllib.loads(self._file(manifest).decode("utf-8", errors="strict"))
            require(type(intent.get("schema_version")) is int and intent["schema_version"] == 1
                    and set(intent) <= {"schema_version", "dependencies", "dev-dependencies", "targets"}
                    and all(type(value) is dict and value == {} for key, value in intent.items() if key != "schema_version"),
                    "Grok removed manifest retains dependency or target intent")
            self.removal_agreement = {"lock": held_fact(self.lock, self.files[self.lock]),
                                      "manifest": held_fact(manifest, self.files[manifest])}
            self.retired_files.append(original)
        except BaseException:
            current = self.files.pop(self.lock, None)
            if current is not None:
                os.close(current[0])
            self.files[self.lock] = original
            raise

    def _removed_absent(self):
        """Check the owned path through held project authority without symlink traversal."""
        parent, opened, path = self.directories[self.project][0], [], self.project
        try:
            parts = self.active.relative_to(self.project).parts
            for index, name in enumerate(parts):
                path = path / name
                try:
                    named = os.stat(name, dir_fd=parent, follow_symlinks=False)
                except FileNotFoundError:
                    return True
                if index == len(parts) - 1:
                    return False
                require(stat.S_ISDIR(named.st_mode) and path in self.directories
                        and directory_identity(named) == self.directories[path][1],
                        "Grok removal ancestor authority changed")
                child = os.open(name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=parent)
                opened.append(child)
                require(directory_identity(os.fstat(child)) == directory_identity(named), "Grok removal ancestor changed during check")
                parent = child
        except (OSError, ValueError) as error:
            self._failed("filesystem custody lost")
            raise CustodyError(str(error)) from error
        finally:
            for fd in opened:
                os.close(fd)
        return False

    def _failed(self, reason):
        if reason not in self.reasons:
            self.reasons.append(reason[:240])
        self.finished = False

    def bind_installation(self, installation, skill_path, public_result, repeated=False):
        self.guard()
        try:
            require(not self.removing and type(repeated) is bool, "Grok binding phase differs")
            phase = "repeat" if repeated else "add"
            path = Path(skill_path)
            require(type(skill_path) is str and not path.is_absolute() and str(path) == skill_path and ".." not in path.parts,
                    "Grok Skill path must be exact project-relative")
            parts = path.parts
            require(len(parts) == 6 and parts[:2] == (".grok", "plugins") and parts[3:] == ("skills", self.item["name"], "SKILL.md")
                    and re.fullmatch(re.escape(self.name[:51]) + r"-[0-9a-f]{12}", parts[2]), "Grok normalized Skill location differs")
            active = self.project.joinpath(*parts[:3])
            require(type(installation) is dict and installation.get("name") == self.name
                    and installation.get("source") == f'{self.item["repo"]}/skills/{self.item["folder"]}'
                    and re.fullmatch(r"[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}", installation.get("installation_id", "")),
                    "Grok installed immutable identity differs")
            require(parts[2] == self.name[:51] + "-" + sha(installation["installation_id"].encode())[:12],
                    "Grok physical artifact does not belong to the installation UUID")
            clients = installation.get("clients")
            require(type(clients) is list and len(clients) == 1 and type(clients[0]) is dict, "Grok sole client binding differs")
            client = clients[0]
            expected = dict(client_id="grok", scope="project", materialization="materialized", activation="active",
                            authentication="not_required", verification="installation_verified", policy="allowed")
            require(all(client.get(key) == value for key, value in expected.items()), "Grok public native state is unqualified")
            revision = client.get("package_revision")
            require(type(revision) is dict and revision.get("resolved_revision") == self.item["ref"] and all(
                re.fullmatch(r"sha256:[a-f0-9]{64}", revision.get(key, "")) for key in ("tree_digest", "manifest_digest")), "Grok immutable revision differs")
            locked = strict_json(self._file(self.lock))
            require(type(locked) is dict and type(locked.get("lock_version")) is int and locked["lock_version"] == 1
                    and type(locked.get("packages")) is list and len(locked["packages"]) == 1,
                    "Grok sole immutable lock package differs")
            package = locked["packages"][0]
            require(type(package) is dict and package.get("alias") == self.item["name"]
                    and package.get("source_key") == self.source_key and package.get("targets") == ["grok"]
                    and package.get("digest_algorithm") == "agentplugins-tree-sha256-v1"
                    and all(package.get(key) == revision[key] for key in ("tree_digest", "manifest_digest")),
                    "Grok lock and public package revision disagree")
            source = package.get("source")
            require(type(source) is dict and set(source) == {"git"} and type(source["git"]) is dict,
                    "Grok immutable Git lock source differs")
            git = source["git"]
            location = "skills/" + self.item["folder"]
            require(git.get("kind") == "skill" and git.get("skill") == self.item["name"]
                    and type(git.get("repository")) is str and git["repository"].lower() == self.item["repo"].lower()
                    and git.get("commit") == self.item["ref"] and git.get("path") == location
                    and git.get("resolved_skill_path") == location and git.get("normalizer") == "agent-skill-package/v1"
                    and git.get("requested") == {"kind": "commit", "value": self.item["ref"]},
                    "Grok lock source is not the selected immutable Skill")
            require(type(public_result) is dict and type(public_result.get("schema_version")) is int
                    and public_result["schema_version"] == 1 and public_result.get("command") == "skills add"
                    and public_result.get("result") == "success" and public_result.get("status") == "succeeded"
                    and public_result.get("scope") == "project" and public_result.get("readiness") == "ready"
                    and public_result.get("next_actions") in (None, []), "Grok public add is not Ready")
            items = public_result.get("items")
            require(type(items) is list and len(items) == 1 and type(items[0]) is dict, "Grok public item differs")
            selected = items[0]
            require(selected.get("alias") == self.item["name"] and selected.get("targetId") == "grok"
                    and selected.get("status") == ("unchanged" if repeated else "succeeded")
                    and selected.get("destinationPath") == str(active)
                    and re.fullmatch(r"sha256:[a-f0-9]{64}", selected.get("candidateId", "")), "Grok public selected source differs")
            binding = {"installation_id": installation["installation_id"], "revision": revision,
                       "candidate_id": selected["candidateId"], "active": str(active),
                       "source_key": self.source_key, "lock": held_fact(self.lock, self.files[self.lock])}
            require(self.binding is None and not repeated or same(self.binding, binding), "Grok repeated binding changed")
            self.active = active
            body = self._file(self.project / path)
            description = skill_description(body, self.item)
            manifest = strict_json(self._file(active / ".grok-plugin/plugin.json"))
            require(type(manifest) is dict and manifest.get("name") == self.name and manifest.get("skills") == "./skills/",
                    "Grok native manifest identity differs")
            self.binding, self.description, self.skill_path = copy.deepcopy(binding), description, self.project / path
            self.bound_phases.add(phase)
            self.guard()
            return True
        except (KeyError, TypeError, ValueError, OSError) as error:
            if isinstance(error, CustodyError):
                self._failed("filesystem custody lost")
                raise
            self._failed("binding: " + str(error))
            self.guard()
            return False

    def observe(self, phase, record, stdout, stderr):
        self.guard()
        capture = {"phase": phase, "record": copy.deepcopy(record), "verified": False}
        for key, body in (("stdout", stdout), ("stderr", stderr)):
            if type(body) is bytes and len(body) <= LIMIT and not grok_private_credential(body):
                capture[key] = {"bytes": len(body), "sha256": sha(body), "base64": base64.b64encode(body).decode()}
        self.captures.append(capture)
        try:
            require(phase in PHASES and [row["phase"] for row in self.captures] == list(PHASES[:len(self.captures)]), "Grok observation order differs")
            require(type(record) is dict and set(record) == PROBE_KEYS and record["probe"] == "grok-pretrusted-" + phase
                    and record["argv"] == [str(self.executable), "inspect", "--json"] and record["cwd"] == str(self.project)
                    and record["error"] is None and type(record["exit"]) is int and record["exit"] == 0,
                    "Grok original native command failed or differs")
            elapsed = record["elapsed_seconds"]
            require(type(elapsed) in (int, float) and math.isfinite(elapsed) and 0 <= elapsed <= 40, "Grok native elapsed bound differs")
            for key, body in (("stdout", stdout), ("stderr", stderr)):
                require(key in capture and type(record[key + "_bytes"]) is int and record[key + "_bytes"] == len(body)
                        and record[key + "_sha256"] == sha(body) and record[key + "_file"] == record["probe"] + "." + key + ".log",
                        "Grok full native stream join differs")
            require(stderr == b"", "Grok native inspection emitted stderr")
            actual = inventory(strict_json(stdout), self.project)
            selected_plugins = [row for key, row in actual["plugins"].items() if key == self.name.casefold()]
            selected_skills = [row for row in actual["skills"].values() if row["name"].casefold() == self.item["name"].casefold()
                               or str(row["source"].get("plugin_name", "")).casefold() == self.name.casefold()]
            foreign = {"plugins": {key: row for key, row in actual["plugins"].items() if row not in selected_plugins},
                       "skills": {key: row for key, row in actual["skills"].items() if row not in selected_skills}}
            if phase == "before":
                require(not selected_plugins and not selected_skills, "Grok selected Skill already exists before add")
                self.baseline = foreign
            else:
                require(self.baseline is not None and same(foreign, self.baseline), "Grok foreign native inventory changed")
                if phase in ("add", "repeat"):
                    require(phase in self.bound_phases and self.binding is not None, "Grok native observation lacks held installed source")
                    expected_plugin = {"name": self.name, "scope": "project", "path": str(self.active), "enabled": True,
                                       "provides": {"skills": 1, "agents": 0, "hooks": False, "mcpServers": 0}}
                    require(same(selected_plugins, [expected_plugin]) and len(selected_skills) == 1, "Grok owned native plugin differs")
                    row = dict(selected_skills[0])
                    require("disabled" not in row or row.pop("disabled") is False, "Grok selected Skill is disabled")
                    expected_skill = {"name": self.item["name"], "description": self.description, "userInvocable": True,
                                      "source": {"type": "plugin", "plugin_name": self.name, "path": str(self.skill_path)}}
                    require(same(row, expected_skill), "Grok loaded Skill source/metadata differs")
                else:
                    require(self.removing and self.binding is not None and not selected_plugins and not selected_skills,
                            "Grok removed owned native inventory remains")
                    require(self._removed_absent(), "Grok removed owned directory remains")
                    require(self.removal_agreement is not None, "Grok removal lacks the empty saved agreement")
            capture["verified"] = True
            self.guard()
            return True
        except (KeyError, TypeError, ValueError, UnicodeError, RecursionError) as error:
            if isinstance(error, CustodyError):
                raise
            self._failed("native " + str(phase) + ": " + str(error))
            capture["reason"] = str(error)[:240]
            self.guard()
            return False

    def begin_remove(self):
        self.guard()
        self.removing = True  # Failed/partial native proof must never prevent safe owned cleanup.
        return self.binding is not None

    def finish(self):
        self.guard()
        self.finished = (not self.reasons and self.removing and len(self.captures) == 4
                         and all(row["verified"] for row in self.captures))
        return self.finished

    def evidence(self):
        return copy.deepcopy({"scenario": "owner-pretrusted-project-raw-skill", "native_discovery_verified": self.finished,
                              "cold_activation_claimed": False, "writer_closure_claimed": False,
                              "case_root": str(self.case), "home": str(self.home), "project": str(self.project),
                              "immutable_fixture": self.item, "bundle_name": self.name, "binding": self.binding,
                              "profile": {"initial": self.initial_fixed, "last_verified": self.last_fixed,
                                          "successful_guards": self.guard_count, "last_guard_passed": self.last_guard_passed,
                                          "enabled_before_first_observation": [self.name],
                                          "trusted_project_before_first_observation": str(self.project)},
                              "binary": {**held_fact(self.executable, self.files[self.executable]),
                                         "sha256": self.executable_digest, "version": self.client_version},
                              "removal_agreement": self.removal_agreement,
                              "reasons": self.reasons,
                              "observations": self.captures})

    def close(self):
        if not self.closed:
            self.closed = True
            for fd in ([value[0] for value in self.files.values()] + [value[0] for value in self.retired_files]
                       + [value[0] for value in self.directories.values()]):
                os.close(fd)
