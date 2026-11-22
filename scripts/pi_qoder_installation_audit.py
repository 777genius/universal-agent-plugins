"""Closed Pi/Qoder installation receipts, separate from native qualification.

Only the explicitly selected catalogue routes below use this observer.
It reads existing public output and UAP-owned delivery receipts. It never runs
a client, authors a profile, grants trust, or changes the original row status.
Pi's existing project hook is observed as owned installation content; its
presence says nothing about the hook's trust decision or native discovery.
"""
import base64
import copy
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import tomllib

from prepared_installation_audit import exact, scoped_operation, completed_ids, grok_skill_identity
from windsurf_global_skill_audit import (
    LIMIT, EMPTY_INTENT, facts, member, read_file, frame, frame_fact, content,
    managed_digest, skill_digest, strict_json,
)

ROUTES = {"pi-installation": ("pi", "plugin"),
          "pi-skills-installation": ("pi", "skill"),
          "qoder-skills-installation": ("qoder", "skill")}
PROJECT_ACCEPTANCES = {
    "pi-foreign-prepared-skills-installation": ("pi", "skill"),
    "pi-clean-native-skills": ("pi", "skill"),
    "qoder-prepared-skills-installation": ("qoder", "skill"),
}
ROUTES.update(PROJECT_ACCEPTANCES)
PI_CLEAN_ACCEPTANCE = "pi-clean-native-skills"
COMMANDS = ("install", "list", "repeat_add", "repeat_list", "cleanup", "post_remove_list")
PI_ACTIVE = {13: "marketing-kit", 22: "sre-extension", 25: "claude-scientific-writer", 29: "recipe-skills"}
PI_NAMESPACE = "com.universal-agent-plugins.pi"
HOOK = ".pi/agent/extensions/agentplugins-project-trust"
# Exact piTrustHookMarker + piTrustHookSource, project_trust_hook.go on 03b7321.
HOOK_SHA256 = "eb4fa44cb880e1df05e42356f7e4bef9efef4abad82495cdfc87867b76b90a73"
HOOK_BYTES = 9078
REFUSAL = "pi package manifest overrides selected extension discovery"
REFUSAL_SOURCE = "github:obra/the-elements-of-style@05fc4f0d2b97b7c042dd9949ad658568e4a1324e"
UUID = re.compile(r"[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\Z")
DIGEST = re.compile(r"sha256:[0-9a-f]{64}\Z")
STATE_KEYS = ("materialization", "activation", "authentication", "policy", "verification")
QUALIFICATION = {key: False for key in (
    "native_discovery_qualified", "native_activation_qualified", "native_mcp_qualified",
    "model_execution_qualified", "authenticated_execution_qualified")}


class PiQoderInstallationError(ValueError):
    """First-party, bounded failure receipt; never an installation verdict."""

    def __init__(self, reason, evidence):
        super().__init__(reason)
        self.audit_evidence = {"pi_qoder_installation_audit": evidence}


def require(value, reason):
    if not value:
        raise ValueError("Pi/Qoder installation evidence: " + reason)


def sha(body):
    return hashlib.sha256(body).hexdigest()


def compact(value):
    return json.dumps(value, ensure_ascii=False, separators=(",", ":"), allow_nan=False).encode()


def go_json(value):
    """The closed MCP string/list/map shape uses Go's canonical JSON escaping."""
    body = json.dumps(value, sort_keys=True, ensure_ascii=False, separators=(",", ":"), allow_nan=False)
    for old, new in (("&", "\\u0026"), ("<", "\\u003c"), (">", "\\u003e"),
                     ("\u2028", "\\u2028"), ("\u2029", "\\u2029")):
        body = body.replace(old, new)
    return body.encode()


def pi_tree_digest(observed):
    """The fixed project_trust_digest.go framing, distinct from Skill receipts."""
    digest = hashlib.sha256()
    for path in sorted((p for p in observed if p), key=lambda p: tuple(p.split("/"))):
        row = observed[path]
        digest.update(("d\0" + path + "\0" if row["kind"] == "dir"
                       else "f\0" + path + "\0" + row["sha256"] + "\0").encode())
    return "sha256:" + digest.hexdigest()


def expected_state(active=False):
    return {"materialization": "materialized", "activation": "active" if active else "prepared",
            "authentication": "not_required" if active else "not_checked",
            "policy": "allowed", "verification": "installation_verified"}


def native_mcp(entry):
    """Pi's already-resolved mcp.json -> generic mcpServers projection only."""
    require(type(entry) is dict, "managed MCP entry is not an object")
    if entry.get("type") == "stdio":
        require(set(entry) <= {"type", "command", "args", "env", "cwd"}
                and type(entry.get("command")) is str and bool(entry["command"].strip()), "managed stdio shape differs")
        require(type(entry.get("args", [])) is list and all(type(v) is str for v in entry.get("args", [])), "MCP args differ")
        require(type(entry.get("env", {})) is dict and all(type(v) is str for v in entry.get("env", {}).values()), "MCP environment differs")
        require(type(entry.get("cwd", "")) is str, "MCP cwd differs")
        return {key: copy.deepcopy(entry[key]) for key in ("command", "args", "env", "cwd") if entry.get(key)}
    require(entry.get("type") == "streamable-http" and set(entry) <= {"type", "url", "headers"}
            and type(entry.get("url")) is str and bool(entry["url"].strip())
            and type(entry.get("headers", {})) is dict
            and all(type(v) is str for v in entry.get("headers", {}).values()), "managed remote MCP shape differs")
    return {key: copy.deepcopy(entry[key]) for key in ("url", "headers") if entry.get(key)}


def _stable_dir(value):
    return {key: value[key] for key in ("kind", "device", "inode", "mode", "uid", "gid")}


def lifecycle_row(row, kind, uid, *, acceptance=None):
    """The same original row gates at finish and at the report's final join."""
    clean = acceptance == PI_CLEAN_ACCEPTANCE
    status = ("passed" if clean else "prepared-lifecycle-passed" if acceptance in PROJECT_ACCEPTANCES
              else "incomplete-native-proof" if kind == "skill" else "package-installed-native-unqualified")
    if (type(row) is not dict or type(uid) is not str or UUID.fullmatch(uid) is None
            or row.get("status") != status
            or any(row.get(k) is not True for k in ("listed_identity", "repeat_verified", "repeat_native_unchanged",
                "repeat_lock_unchanged", "removal_verified", "post_list_schema_valid", "sentinels_preserved"))
            or any(row.get(k) is not False for k in ("repeat_native_runtime_only", "failed_install_registered", "failed_install_residue", "post_remove_identity_registered"))
            or row.get("native_residuals") != [] or row.get("post_removal_leftovers") != []
            or row.get("matched_installation_ids") != [uid] or row.get("repeat_installation_ids") != [uid]):
        return False
    for phase in COMMANDS:
        outcome = row.get(phase)
        if type(outcome) is not dict or not exact(outcome.get("exit_code"), 0) or outcome.get("timeout") is not False:
            return False
    return kind != "skill" or (
        all(row.get(k) is True for k in ("lock_written", "lock_contains_identity", "lock_exact_source_pinned", "skill_bytes_match"))
        and row.get("post_removal_skill") is None and row.get("post_lock_pinned") is False
        and exact(row.get("public_skill_ready"), {"after-add": clean, "repeat-add": clean}))


def refusal_row(row):
    return (type(row) is dict and row.get("source") == REFUSAL_SOURCE and row.get("identity") == "elements-of-style"
            and row.get("status") == "documented-or-source-unsupported"
            and row.get("sentinels_preserved") is True and row.get("native_residuals") == []
            and exact(row.get("native_file_delta"), 0) and row.get("native_identity_files") == []
            and all(row.get(k) is False for k in ("listed_identity", "failed_install_registered", "failed_install_residue", "post_remove_identity_registered"))
            and type(row.get("install")) is dict and exact(row["install"].get("exit_code"), 1)
            and row["install"].get("timeout") is False)


class PiQoderInstallationAudit:
    def __init__(self, cli, env, project, case_root, audit, *, target, kind, item, index, fixtures, signed_release_pins, acceptance=None):
        self.closed, self.finished, self.errors = False, False, []
        self.dirs, self.observations, self.deliveries = {}, [], []
        self.current_files, self.current_owned, self.first_owned = {}, {}, None
        self.owners, self.selected, self.coordination, self.refusal_dirs = {}, {}, {}, {}
        self.runtime_stores, self.source_metadata = {}, None
        self.proof = None
        try:
            require((target, kind) in ROUTES.values() and type(index) is int and 0 <= index < 30, "route differs")
            legacy = {("pi", "plugin"): "pi-installation", ("pi", "skill"): "pi-skills-installation",
                      ("qoder", "skill"): "qoder-skills-installation"}
            self.acceptance = legacy[(target, kind)] if acceptance is None else acceptance
            require(type(self.acceptance) is str and ROUTES.get(self.acceptance) == (target, kind), "acceptance route differs")
            self.clean_pi = self.acceptance == PI_CLEAN_ACCEPTANCE
            self.target, self.kind, self.index, self.cli, self.audit = target, kind, index, cli, audit
            self.item, self.fixtures, self.pins = copy.deepcopy(item), copy.deepcopy(fixtures), copy.deepcopy(signed_release_pins)
            sources = fixtures.get("skills" if kind == "skill" else "plugin_aliases")
            require(type(sources) is list and len(sources) == 30 and exact(sources[index], item), "fixture slot differs")
            self.name = item["name"] if kind == "skill" else fixtures["plugin_declared_names"].get(item, item)
            self.source = item["source"] if kind == "skill" else item
            self.scope = "project" if kind == "skill" else "user"
            self.case, self.project = Path(case_root), Path(project)
            self.home, self.data = self.case / "home", self.case / "uap-data"
            self.state_root = self.project / ".uap" if kind == "skill" else self.data
            self.lock_root = self.project if kind == "skill" else self.data
            self.profile = ((self.project / ".pi" if kind == "skill" else self.home / ".pi/agent")
                            if target == "pi" else self.project / ".uap/managed/clients/qoder")
            self.native_skills = ((self.project / ("." + target) if kind == "skill" else self.profile) / "skills")
            require(self.case.resolve(strict=True) == self.case and self.project == self.case / "project"
                    and env.get("HOME") == env.get("USERPROFILE") == str(self.home)
                    and env.get("AGENTPLUGINS_HOME") == str(self.data)
                    and env.get("TMPDIR") == str(self.case / "tmp")
                    and all(env.get(key) == str(self.home / rel) for key, rel in (
                        ("XDG_CONFIG_HOME", ".config"), ("XDG_CACHE_HOME", ".cache"),
                        ("XDG_DATA_HOME", ".local/share"), ("XDG_STATE_HOME", ".local/state"))), "private environment differs")
            for path in (self.case, self.home, self.project, self.data, self.case / "tmp"):
                self._anchor(path)
            self.native_root = self.project / ("." + target)
            self.roots = [self.home]
            if kind == "skill":
                # Both raw routes have an existing foreign Skill in this root.
                self._anchor(self.native_root)
                self.roots.append(self.native_root)
            self.initial = self._image()
            self.current = copy.deepcopy(self.initial)
            self.tmp_frame = frame(self.dirs[self.case / "tmp"][0])
            if self.clean_pi:
                require(set(self.initial[str(self.native_root)]) == {"", "skills", "extensions"}
                        and all(row["kind"] == "dir" for row in self.initial[str(self.native_root)].values())
                        and not self._exists(self.home / ".pi/agent"), "clean Pi must start with an empty independent native namespace")
            self.foreign = {}
            paths = [self.home / "foreign-untouched.txt", self.project / "foreign-untouched.txt",
                     self.home / ("." + target) / "foreign-neighbor.txt"]
            if kind == "skill" and not self.clean_pi:
                paths.append(self.native_skills / "foreign-neighbor/SKILL.md")
            for path in paths:
                self.foreign[str(path)] = self._file(path)
            require(not self._exists(self.lock_root / "uap.lock")
                    and not self._exists(self.lock_root / "uap.toml")
                    and not self._exists(self.state_root / "state-v2.json"), "case already has saved owners")
            if target == "pi":
                require(not self._exists(self.profile / "mcp.json"), "fresh native MCP config is not absent")
            if target == "qoder":
                require(not self._exists(self.project / ".mcp.json"), "fresh Qoder raw-Skill MCP authority is not absent")
            if target == "pi" and kind == "skill":
                require(not self._exists(self.home / HOOK), "fresh project hook already exists")
            self.alias_sources = {self.name: self.source}
            if kind == "plugin" and self.name == "marketing-kit":
                mapping, closure = audit["MARKETING_KIT_DISTRIBUTIONS"], audit["MARKETING_KIT_SIGNED_CLOSURE"]
                require(set(mapping) == {"marketing-kit", "notion", "hubspot-crm", "playwright"}, "Marketing closure differs")
                for alias, distribution in mapping.items():
                    if alias == self.name:
                        continue
                    anchor = closure[distribution]
                    self.pins[alias] = {"distribution": distribution, "sequence": anchor["release_sequence"],
                        "repository": anchor["repository"], "path": anchor["package_path"],
                        "revision": anchor["resolved_revision"], "tree_digest": anchor["tree_digest"],
                        "manifest_digest": anchor["manifest_digest"]}
                    self.alias_sources[alias] = alias
            self.negative = kind == "plugin" and index == 26 and self.source == REFUSAL_SOURCE and self.name == "elements-of-style"
            self.guard()
        except BaseException:
            self.close()
            raise

    def _anchor(self, path):
        require(path.resolve(strict=True) == path, "directory is not canonical")
        before = path.lstat()
        require(stat.S_ISDIR(before.st_mode) and before.st_uid == os.getuid() and not before.st_mode & 0o022,
                "directory ownership/mode differs")
        fd = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC)
        try:
            require(facts(before) == facts(os.fstat(fd)), "directory changed while opening")
            self.dirs[path] = (fd, before)
        except BaseException:
            os.close(fd)
            raise

    def _relative(self, path):
        require(type(path) is Path or isinstance(path, Path), "authority is not a path")
        value = path.relative_to(self.case).as_posix()
        require(value and not any(x in ("", ".", "..") for x in value.split("/")), "authority escapes case")
        return value

    def _exists(self, path):
        try:
            with member(self.dirs[self.case][0], self._relative(path)):
                return True
        except FileNotFoundError:
            return False

    def _file(self, path):
        return read_file(self.dirs[self.case][0], self._relative(path))

    def _document(self, path):
        body, fact = self._file(path)
        return strict_json(body), fact

    def _tree(self, path):
        with member(self.dirs[self.case][0], self._relative(path)) as fd:
            observed = frame(fd)
            return observed, managed_digest(fd, observed)

    def _image(self):
        return {str(path): frame(self.dirs[path][0]) for path in self.roots}

    def guard(self):
        try:
            require(not self.closed, "observer closed")
            for path, (fd, original) in self.dirs.items():
                current, held = path.lstat(), os.fstat(fd)
                require(path.resolve(strict=True) == path and stat.S_ISDIR(current.st_mode)
                        and all(getattr(current, key) == getattr(held, key) == getattr(original, key)
                                for key in ("st_dev", "st_ino", "st_mode", "st_uid", "st_gid")), "held directory replaced")
            if self.target == "qoder":
                require(not self._exists(self.project / ".mcp.json"), "pure Qoder Skill acquired an unowned MCP authority")
            require(exact(self._image(), self.current), "native tree changed outside receipt-bound public transition")
            require(exact(frame(self.dirs[self.case / "tmp"][0]), self.tmp_frame), "private temp changed outside its original authority")
            for path, previous in self.foreign.items():
                require(self._file(Path(path)) == previous, "foreign bytes or identity changed")
            for path, previous in self.current_files.items():
                require(self._file(Path(path)) == previous, "saved receipt changed between public calls")
            for path, previous in self.current_owned.items():
                require(exact(self._tree(Path(path))[0], previous), "managed/native package changed between public calls")
            for path, previous in self.runtime_stores.items():
                require(self._file(Path(path)) == previous, "original clean Pi runtime store changed")
        except (OSError, ValueError) as error:
            self.errors.append(str(error)[:2000])
            raise PiQoderInstallationError(str(error)[:2000], self.evidence()) from error

    def _expected_source(self, alias):
        source = self.alias_sources[alias]
        if self.kind == "skill":
            repo, subpath, commit = self.item["repo"], "skills/" + self.item["folder"], self.item["ref"]
            return repo, subpath, commit, "github-skill:" + repo.lower() + ":" + self.name + ":" + subpath
        if source.startswith("github:"):
            repo, commit = source[len("github:"):].rsplit("@", 1)
            return repo.lower(), "", commit, "github:" + repo.lower() + ":"
        pin = self.pins[source]
        return pin["repository"], pin["path"], pin["revision"], "directory:" + pin["distribution"].lower()

    def _lock(self, removed=False):
        locked, _ = self._document(self.lock_root / "uap.lock")
        body, _ = self._file(self.lock_root / "uap.toml")
        manifest = tomllib.loads(body.decode("utf-8", errors="strict"))
        require(type(manifest) is dict and exact(manifest.get("schema_version"), 1)
                and set(manifest) <= {"schema_version", "dependencies", "dev-dependencies", "targets"}
                and manifest.get("dev-dependencies", {}) == {}, "portable manifest schema differs")
        require(type(locked) is dict and set(locked) == {"lock_version", "manifest_digest", "packages"}
                and exact(locked["lock_version"], 1) and type(locked["packages"]) is list, "portable lock schema differs")
        if removed:
            require(exact(locked, {"lock_version": 1, "manifest_digest": EMPTY_INTENT, "packages": []})
                    and manifest.get("dependencies", {}) == {} and manifest.get("targets", {}) == {}, "remove left nonempty portable intent")
            return locked
        require(type(manifest.get("dependencies")) is dict and set(manifest["dependencies"]) == {self.name}
                and exact(manifest.get("targets"), {self.target: [self.name]}), "portable root owner/target differs")
        repo, subpath, commit, key = self._expected_source(self.name)
        dependency = manifest["dependencies"][self.name]
        if self.kind == "skill" or self.source.startswith("github:"):
            wanted = {"git": "https://github.com/" + repo + ".git", "commit": commit}
            if self.kind == "skill":
                wanted.update(kind="skill", skill=self.name, path=subpath)
            selector = {"kind": "commit", "value": commit}
        else:
            pin = self.pins[self.source]
            wanted = {"registry": "uap", "distribution": pin["distribution"], "release": pin["sequence"]}
            selector = {"kind": "release", "value": str(pin["sequence"])}
        require(exact(dependency, wanted), "portable source intent differs")
        dep = {"alias": self.name, "source_key": key}
        if subpath and self.kind == "skill":
            dep["path"] = subpath
        dep["selector"] = selector
        intent = {"schema_version": 1, "dependencies": [dep], "targets": [{"id": self.target, "aliases": [self.name]}]}
        require(locked["manifest_digest"] == "sha256:" + sha(compact(intent)), "portable manifest digest differs")
        packages = locked["packages"]
        require(len(packages) == len(self.alias_sources) and all(type(p) is dict for p in packages)
                and {p.get("alias") for p in packages} == set(self.alias_sources), "lock closure differs")
        result = {}
        for package in packages:
            alias = package["alias"]
            require(alias not in result, "duplicate lock alias")
            repo, subpath, commit, key = self._expected_source(alias)
            require(package.get("source_key") == key and package.get("digest_algorithm") == "agentplugins-tree-sha256-v1"
                    and exact(package.get("targets"), [self.target])
                    and all(type(package.get(k)) is str and DIGEST.fullmatch(package[k]) for k in ("tree_digest", "manifest_digest")), "lock source/revision digests differ")
            source = package.get("source")
            require(type(source) is dict and len(source) == 1, "lock source kind differs")
            if self.kind == "skill" or self.alias_sources[alias].startswith("github:"):
                git = source.get("git")
                require(type(git) is dict and type(git.get("repository")) is str
                        and git["repository"].lower() == repo.lower() and git.get("path", "") == subpath
                        and git.get("commit") == commit and exact(git.get("requested"), {"kind": "commit", "value": commit})
                        and not any(git.get(k) for k in ("url", "resolved_ref", "default_branch")), "locked Git selection differs")
                if self.kind == "skill":
                    require(git.get("kind") == "skill" and git.get("skill") == self.name
                            and git.get("resolved_skill_path") == subpath and git.get("normalizer") == "agent-skill-package/v1"
                            and type(git.get("raw_tree_digest")) is str and DIGEST.fullmatch(git["raw_tree_digest"]), "locked raw Skill differs")
                else:
                    require(not any(git.get(k) for k in ("kind", "skill", "resolved_skill_path", "normalizer", "raw_tree_digest")), "plugin lock substituted a raw Skill source")
            else:
                pin, registry = self.pins[self.alias_sources[alias]], source.get("registry")
                require(type(registry) is dict and registry.get("distribution") == registry.get("distribution_id") == pin["distribution"]
                        and exact(registry.get("release_sequence"), pin["sequence"])
                        and registry.get("repository") == repo and registry.get("path", "") == subpath and registry.get("commit") == commit
                        and all(package[k] == pin[k] for k in ("tree_digest", "manifest_digest")), "signed lock tuple differs")
                requested = registry.get("requested")
                require(type(requested) is dict and set(requested) == {"kind", "value"}
                        and requested["kind"] in ("release", "version", "range") and type(requested["value"]) is str and requested["value"], "signed request selector differs")
                if alias == self.name:
                    require(exact(requested, selector), "root signed release selector differs")
            parents = package.get("required_by", [])
            require((parents == [] if alias == self.name else type(parents) is list and len(parents) == 1
                     and type(parents[0]) is dict and parents[0].get("alias") == self.name
                     and type(parents[0].get("selector")) is dict), "lock dependency ownership differs")
            result[alias] = package
        return result

    def _components(self, document):
        if self.kind == "skill":
            self.component_decisions = {self.name: [{"kind": "skill", "name": self.name, "support": "native"}]}
            return {self.name: [("skill", self.name)]}
        actions = document["data"].get("actions")
        require(type(actions) is list and len(actions) == len(self.alias_sources), "first add closure actions differ")
        result, self.component_decisions = {}, {}
        for action in actions:
            require(type(action) is dict and action.get("alias") in self.alias_sources and action["alias"] not in result
                    and action.get("client_id") == self.target and action.get("action") == "install", "first install action differs")
            components = action.get("component_support")
            require(type(components) is list and components and all(type(c) is dict for c in components), "component authority missing")
            selected = [(c.get("kind"), c.get("name")) for c in components]
            require(len(set(selected)) == len(selected) and all(type(name) is str and re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_.-]*", name)
                    and kind in ("skill", "mcp_server", "extension") for kind, name in selected)
                    and all(c.get("support") == "native" or c.get("kind") == "extension" and c.get("support") == "unsupported" for c in components)
                    and all(c["name"] == PI_NAMESPACE for c in components if c["kind"] == "extension" and c["support"] == "native"), "component decision is not the closed native projection")
            self.component_decisions[action["alias"]] = copy.deepcopy(components)
            result[action["alias"]] = sorted((c["kind"], c["name"]) for c in components if c["support"] == "native")
            require(result[action["alias"]], "package has no selected native installation projection")
        return result

    def _installed(self, components):
        locked = self._lock()
        state, _ = self._document(self.state_root / "state-v2.json")
        require(type(state) is dict and exact(state.get("schema_version"), 5)
                and type(state.get("installations")) is list and len(state["installations"]) == len(self.alias_sources), "saved owner closure differs")
        owners, owned, mcp, allowed = {}, {}, {}, set()
        for record in state["installations"]:
            require(type(record) is dict and type(record.get("installation_id")) is str and UUID.fullmatch(record["installation_id"]), "saved owner UUID differs")
            uid = record["installation_id"]
            name = record.get("declared_name")
            alias = self.name if self.kind == "skill" else name
            require(alias in self.alias_sources and alias not in owners, "saved logical closure differs")
            if self.kind == "skill":
                require(name == grok_skill_identity(self.item, uid)[1], "raw Skill generated name differs")
            physical = name[:51].rstrip(".-") + "-" + sha(uid.encode())[:12]
            active = self.profile / ("extensions" if self.target == "pi" else "") / physical
            repo, subpath, commit, key = self._expected_source(alias)
            origin, package, bindings = record.get("source"), record.get("package"), record.get("clients")
            require(type(origin) is dict and type(origin.get("repository")) is str and origin["repository"].lower() == repo.lower()
                    and origin.get("package_subpath", "") == subpath and origin.get("resolved_revision") == commit
                    and origin.get("tree_digest") == locked[alias]["tree_digest"]
                    and type(package) is dict and package.get("declared_name") == name
                    and package.get("manifest_digest") == locked[alias]["manifest_digest"]
                    and type(bindings) is dict and len(bindings) == 1, "saved source/package/binding differs")
            if self.kind == "plugin" and not self.alias_sources[alias].startswith("github:"):
                pin, directory = self.pins[self.alias_sources[alias]], record.get("directory")
                require(type(directory) is dict and directory.get("distribution_id") == pin["distribution"]
                        and exact(directory.get("desired_release_sequence"), pin["sequence"]), "saved signed origin differs")
            binding_id, binding = next(iter(bindings.items()))
            expected_binding_id = "client_" + sha("\0".join((uid, self.target, self.scope, str(active))).encode())[:24]
            require(type(binding) is dict and binding_id == expected_binding_id == binding.get("client_binding_id")
                    and binding.get("client_id") == self.target and binding.get("scope") == self.scope
                    and binding.get("target_locator") == str(active) and binding.get("physical_artifact_id") == physical
                    and binding.get("native_profile_root") == str(self.profile)
                    and binding.get("install_intent", "") == ""
                    and exact(binding.get("affected_surfaces"), [self.target]), "saved exact native binding differs")
            active_state = self.clean_pi or self.kind == "plugin" and alias == self.name and self.index in PI_ACTIVE
            require(exact({k: binding.get(k) for k in STATE_KEYS}, expected_state(active_state)), "saved state differs from explicit installation contract")
            revision = binding.get("package_revision")
            require(type(revision) is dict and revision.get("resolved_revision") == commit
                    and all(revision.get(k) == locked[alias][k] for k in ("tree_digest", "manifest_digest"))
                    and package.get("version", "") == locked[alias].get("version", "") == revision.get("version", ""), "saved immutable revision differs")
            if self.kind == "plugin" and not self.alias_sources[alias].startswith("github:"):
                pin = self.pins[self.alias_sources[alias]]
                require(revision.get("distribution_id") == pin["distribution"] and exact(revision.get("release_sequence"), pin["sequence"]), "saved signed revision differs")
            inventory = package.get("inventory")
            require(type(inventory) is dict and not any(inventory.get(k) for k in ("invalid_mcp_servers", "invalid_skills", "invalid_skills_root", "app_bindings", "app_present")), "saved component inventory is invalid")
            for kind, field in (("skill", "skills"), ("mcp_server", "mcp_servers"), ("extension", "extensions")):
                require(type(inventory.get(field, [])) is list and sorted(inventory.get(field, [])) == sorted(
                    c["name"] for c in self.component_decisions[alias] if c["kind"] == kind), "complete package inventory differs")
            objects = binding.get("native_objects")
            require(type(objects) is list and all(type(o) is dict for o in objects)
                    and len(objects) == 1 + sum(k != "extension" for k, _ in components[alias]), "native receipt closure differs")
            packages = [o for o in objects if o.get("kind") == "managed_package_directory"]
            require(len(packages) == 1, "managed package receipt missing")
            observed, actual_digest = self._tree(active)
            expected_package = {"object_id": "package:" + self.target + ":" + physical, "kind": "managed_package_directory",
                                "logical_name": name, "path": str(active), "managed_digest": actual_digest, "protection_class": "managed"}
            require(exact(packages[0], expected_package), "actual managed package bytes differ from sealed receipt")
            unsupported = {c["name"] for c in self.component_decisions[alias] if c["support"] == "unsupported"}
            if unsupported:
                manifest, _ = self._document(active / "plugin.json")
                require(type(manifest) is dict and type(manifest.get("extensions", {})) is dict
                        and not unsupported.intersection(manifest.get("extensions", {})), "unsupported extension survived production sanitization")
            owned[str(active)] = observed
            allowed.update(str(active / p) if p else str(active) for p in observed)
            skills, servers = {}, {}
            seen = set()
            for obj in objects:
                if obj is packages[0]:
                    continue
                logical = obj.get("logical_name")
                kind = "skill" if obj.get("kind") == self.target + "_" + self.scope + "_skill_directory" else "mcp_server"
                wanted_kind = self.target + "_" + self.scope + ("_skill_directory" if kind == "skill" else "_mcp_server")
                require((kind, logical) in components[alias] and (kind, logical) not in seen
                        and obj.get("kind") == wanted_kind and obj.get("object_id") == wanted_kind + ":" + logical
                        and obj.get("protection_class") == "managed" and obj.get("user_modified", False) is False
                        and not obj.get("before_digest"), "native component receipt ownership differs")
                seen.add((kind, logical))
                if kind == "skill":
                    relative = obj.get("source_relative")
                    require(type(relative) is str and relative and not Path(relative).is_absolute()
                            and all(p not in ("", ".", "..") for p in relative.split("/"))
                            and (self.kind != "skill" or relative == "skills/" + self.name), "Skill source relative differs")
                    native = self.native_skills / logical
                    require(obj.get("path") == str(native), "native Skill path differs")
                    source_frame, _ = self._tree(active / relative)
                    native_frame, _ = self._tree(native)
                    require(exact(content(source_frame), content(native_frame)) and "SKILL.md" in native_frame, "managed and native Skill bytes differ")
                    with member(self.dirs[self.case][0], self._relative(active / relative)) as fd:
                        digest = skill_digest(fd, source_frame)
                    require(obj.get("managed_digest") == digest, "actual Skill digest differs from native receipt")
                    if self.kind == "skill":
                        body, _ = self._file(native / "SKILL.md")
                        require(hashlib.sha1(b"blob " + str(len(body)).encode() + b"\0" + body).hexdigest() == self.item["skill_md_git_blob_sha1"], "raw Skill differs from immutable Git blob")
                    owned[str(native)] = native_frame
                    allowed.update(str(native / p) if p else str(native) for p in native_frame)
                    skills[logical] = {"path": str(native), "source_relative": relative, "digest": digest,
                                       "frame": frame_fact(native_frame)}
                else:
                    require(self.target == "pi" and self.kind == "plugin" and obj.get("path") == str(self.profile / "mcp.json")
                            and not obj.get("source_relative"), "MCP receipt path differs")
                    rendered, _ = self._document(active / "mcp.json")
                    require(type(rendered) is dict and type(rendered.get("mcpServers")) is dict and logical in rendered["mcpServers"], "managed MCP member missing")
                    entry = native_mcp(rendered["mcpServers"][logical])
                    digest = "sha256:" + sha(b"agentplugins-native-mcp-v1\0mcpServers\0" + logical.encode() + b"\0" + go_json(entry))
                    require(obj.get("managed_digest") == digest and logical not in mcp, "MCP digest or closure collision")
                    mcp[logical], servers[logical] = entry, digest
            require(seen == {(k, n) for k, n in components[alias] if k != "extension"}, "native receipt omits a selected component")
            owners[alias] = {"uuid": uid, "name": name, "physical": physical, "active": str(active),
                "binding_id": binding_id, "revision": copy.deepcopy(revision), "source_key": key,
                "record": copy.deepcopy(record), "skills": skills, "mcp_servers": servers,
                "managed_digest": actual_digest, "managed_frame": frame_fact(observed)}
        if mcp:
            config, _ = self._document(self.profile / "mcp.json")
            require(exact(config, {"mcpServers": mcp}), "native MCP config is not the complete exact owned closure")
            allowed.add(str(self.profile / "mcp.json"))
            self._coordination(self.profile / "mcp.json.agentplugins.lock", allowed)
        elif self.target == "pi":
            require(not self._exists(self.profile / "mcp.json"), "unexpected MCP config in pure Skill/extension delivery")
        if self.target == "pi" and self.kind == "skill":
            self._trust(owners[self.name], owned, allowed)
        if self.clean_pi:
            self._runtime_stores(allowed)
        return owners, owned, allowed

    def _runtime_stores(self, allowed):
        """Only the two empty stores already admitted by main's clean Pi proof."""
        parent = self.home / ".pi/agent"
        with member(self.dirs[self.case][0], self._relative(parent)) as fd:
            info = os.fstat(fd)
            require(stat.S_ISDIR(info.st_mode) and stat.S_IMODE(info.st_mode) == 0o700
                    and info.st_uid == os.getuid(), "clean Pi runtime directory differs")
        for name in ("auth.json", "models-store.json"):
            path = parent / name
            value = self._file(path)
            require(value[0] == b"{}" and stat.S_IMODE(value[1]["mode"]) == 0o600,
                    "clean Pi runtime store is not the exact empty private file")
            if str(path) in self.runtime_stores:
                require(value == self.runtime_stores[str(path)], "clean Pi runtime store was replaced")
            else:
                require(not self.observations, "runtime store appeared after the original install transition")
                self.runtime_stores[str(path)] = value
            allowed.add(str(path))

    def _coordination(self, path, allowed):
        value = self._file(path)
        require(value[0] == b"" and stat.S_IMODE(value[1]["mode"]) == 0o600, "coordination lock has unexpected bytes/mode")
        if str(path) in self.coordination:
            require(value == self.coordination[str(path)], "original coordination lock replaced")
        else:
            self.coordination[str(path)] = value
        allowed.add(str(path))

    def _trust(self, owner, owned, allowed):
        hook_root = self.home / HOOK
        body, _ = self._file(hook_root / "index.ts")
        require(len(body) == HOOK_BYTES and sha(body) == HOOK_SHA256, "owned Pi hook is not exact source bytes")
        native = str(self.native_skills / self.name)
        expected = {"schema_version": 1, "projects": {str(self.project): {owner["physical"]: {
            "extensions": {owner["physical"]: pi_tree_digest(owned[owner["active"]])},
            "skills": {self.name: pi_tree_digest(owned[native])}, "mcp_servers": {}}}}}
        document, _ = self._document(hook_root / "projects.json")
        require(exact(document, expected), "Pi hook registry differs from exact source-owned project content")
        for name in ("index.ts", "projects.json"):
            allowed.add(str(hook_root / name))
            self._coordination(hook_root / (name + ".agentplugins.lock"), allowed)

    def _transition(self, allowed):
        if self.target == "qoder":
            require(not self._exists(self.project / ".mcp.json"), "pure Qoder Skill public transition created an MCP authority")
        image = self._image()
        allowed = {Path(p) for p in allowed}
        ancestors = {p for path in allowed for p in path.parents if p != self.case and self.case in p.parents}
        for root, previous in self.initial.items():
            current = image[root]
            for rel, old in previous.items():
                require(rel in current, "original native path disappeared")
                path, new = Path(root) / rel, current[rel]
                require(exact(_stable_dir(new), _stable_dir(old)) if old["kind"] == "dir" and (path in ancestors or path in allowed)
                        else exact(new, old), "foreign native bytes or identity changed")
            for rel in set(current) - set(previous):
                path, entry = Path(root) / rel, current[rel]
                require(path in allowed or path in ancestors and entry["kind"] == "dir", "unowned native delta, including cache, is refused")
            # Receipt-bound additions do not authorize replacing their parent
            # authorities. Removal can delete owned children, not redirect a
            # surviving native directory to a different identity.
            for rel, old in self.current[root].items():
                if old["kind"] == "dir" and rel in current:
                    require(exact(_stable_dir(current[rel]), _stable_dir(old)), "native directory authority was replaced")
        for path, old in self.foreign.items():
            require(self._file(Path(path)) == old, "foreign sentinel changed")
        require(exact(frame(self.dirs[self.case / "tmp"][0]), self.tmp_frame), "public command left private temp residue")
        return image

    def _hold_saved(self):
        self.current_files = {str(path): self._file(path) for path in (
            self.lock_root / "uap.lock", self.lock_root / "uap.toml", self.state_root / "state-v2.json") if self._exists(path)}

    def _public_list(self, document):
        require(type(document) is dict and exact(document.get("schema_version"), 1)
                and document.get("command") == "list" and document.get("result") == "success"
                and type(document.get("data")) is dict and type(document["data"].get("installations")) is list, "typed list envelope differs")
        rows = document["data"]["installations"]
        require(len(rows) == len(self.owners), "public list closure differs")
        selected = {}
        for alias, owner in self.owners.items():
            matches = self.audit["matching_installations"](document, owner["name"], self.target, self.kind,
                self.item if self.kind == "skill" else self.alias_sources[alias], self.pins)
            require(type(matches) is list and len(matches) == 1, "public list fails original immutable source matcher")
            node = matches[0]
            require(node.get("installation_id") == owner["uuid"] and node.get("name") == owner["name"]
                    and node.get("mixed_version") is False and type(node.get("clients")) is list and len(node["clients"]) == 1, "public UUID/name/binding differs")
            client, binding = node["clients"][0], next(iter(owner["record"]["clients"].values()))
            repo, path, _, _ = self._expected_source(alias)
            require(node.get("source") == repo + ("/" + path if path else "")
                    and node.get("version", "") == owner["revision"].get("version", "")
                    and client.get("client_id") == self.target and client.get("scope") == self.scope
                    and exact(client.get("package_revision"), owner["revision"])
                    and exact({k: client.get(k) for k in STATE_KEYS}, {k: binding[k] for k in STATE_KEYS}), "public source/revision/state differs")
            selected[alias] = copy.deepcopy(node)
        if self.selected:
            require(exact(selected, self.selected), "repeat public identity/state changed")
        self.selected = selected

    def _public_operation(self, document, repeated=False, removed=False):
        ids = {owner["uuid"] for owner in self.owners.values()}
        if self.kind == "skill" and not removed:
            owner = self.owners[self.name]
            require(type(document) is dict and exact(document.get("schema_version"), 1)
                    and document.get("command") == "skills add" and document.get("result") == "success"
                    and document.get("status") == "succeeded" and document.get("scope") == "project"
                    and document.get("scope_selection_reason") == "explicit" and document.get("repairInstructions") == []
                    and document.get("readiness") == ("ready" if self.clean_pi else "needs_action"), "raw Skill envelope differs from the explicit cohort")
            items, actions = document.get("items"), document.get("next_actions")
            require(type(items) is list and len(items) == 1 and type(items[0]) is dict
                    and items[0].get("alias") == self.name and items[0].get("targetId") == self.target
                    and items[0].get("candidateId") == grok_skill_identity(self.item, owner["uuid"])[0]
                    and items[0].get("destinationPath") == owner["active"]
                    and items[0].get("status") == ("unchanged" if repeated else "succeeded"), "raw Skill candidate/readiness differs")
            if self.clean_pi:
                require(actions is None or exact(actions, []), "clean Pi still has a pending native action")
            else:
                require(type(actions) is list and len(actions) == 1 and type(actions[0]) is dict
                        and actions[0].get("code") == "verify_native" and actions[0].get("actor") == "native_client", "raw Skill candidate/readiness differs")
            return
        if removed:
            data = document
            require(scoped_operation(data, "remove", self.scope), "remove envelope did not complete")
        else:
            require(type(document) is dict and exact(document.get("schema_version"), 1)
                    and document.get("command") == "add" and document.get("result") == "success", "plugin add outer envelope differs")
            data = document.get("data")
            require(scoped_operation(data, "add", self.scope), "plugin add did not complete")
            env = data.get("environment")
            require(type(env) is dict and env.get("alias") == self.name and env.get("saved") is True
                    and env.get("scope") == self.scope and env.get("scope_selection_reason") == "explicit"
                    and exact(env.get("selected_targets"), [self.target]) and env.get("intent_change") is (not repeated)
                    and env.get("lock_path") == str(self.lock_root / "uap.lock")
                    and env.get("manifest_path") == str(self.lock_root / "uap.toml"), "saved public environment differs")
        actions = data.get("actions")
        require(type(actions) is list and len(actions) == len(self.owners) and all(type(a) is dict for a in actions)
                and {a.get("alias") for a in actions} == set(self.owners), "public action closure differs")
        for action in actions:
            owner = self.owners[action["alias"]]
            revision = owner["revision"]
            require(action.get("client_id") == self.target and action.get("action") == ("remove" if removed else "unchanged" if repeated else "install"), "public operation differs")
            if removed or repeated:
                require(action.get("installation_id") == owner["uuid"] and action.get("binding_id") == owner["binding_id"]
                        and action.get("from_revision") == revision["resolved_revision"]
                        and action.get("from_digest") == revision["tree_digest"], "public operation changed the original owner")
            if not removed:
                require(action.get("revision") == revision["resolved_revision"] and action.get("package_digest") == revision["tree_digest"], "public action immutable revision differs")
        require(data.get("results") == [] if repeated else completed_ids(data.get("results"), "remove" if removed else "install", ids, self.target), "public result closure differs")

    def _removed(self):
        self._lock(removed=True)
        state, _ = self._document(self.state_root / "state-v2.json")
        require(type(state) is dict and exact(state.get("schema_version"), 5) and type(state.get("installations")) is list, "removed state schema differs")
        for record in state["installations"]:
            matching = [o for o in self.owners.values() if o["uuid"] == record.get("installation_id")]
            require(len(matching) == 1 and record.get("clients") == {} and record.get("data_retained") is True
                    and record.get("declared_name") == matching[0]["name"]
                    and exact(record.get("source"), matching[0]["record"]["source"])
                    and exact(record.get("package"), matching[0]["record"]["package"]), "removed state retained or reminted an owner")
        require(len({r["installation_id"] for r in state["installations"]}) == len(state["installations"]), "duplicate retained owner")
        for path in self.current_owned:
            require(not self._exists(Path(path)), "owned native or managed package survived removal")
        allowed = set(self.coordination)
        for path, previous in self.coordination.items():
            require(self._file(Path(path)) == previous, "retained coordination lock changed")
        for path, previous in self.runtime_stores.items():
            require(self._file(Path(path)) == previous, "removed clean Pi runtime store changed")
            allowed.add(path)
        if self.kind == "plugin" and any(o["mcp_servers"] for o in self.owners.values()):
            document, _ = self._document(self.profile / "mcp.json")
            require(exact(document, {"mcpServers": {}}), "removed MCP config retains an owned/foreign member")
            allowed.add(str(self.profile / "mcp.json"))
        if self.target == "pi" and self.kind == "skill":
            require(not self._exists(self.home / HOOK / "index.ts") and not self._exists(self.home / HOOK / "projects.json"), "last Pi project owner left hook payload")
        # Only ancestors of the exact formerly owned paths may remain empty.
        ancestors = {str(p) for value in self.current_owned for p in Path(value).parents if p != self.case and self.case in p.parents}
        for path in ancestors:
            if self._exists(Path(path)):
                with member(self.dirs[self.case][0], self._relative(Path(path))) as fd:
                    require(stat.S_ISDIR(os.fstat(fd).st_mode), "removed owned ancestor became a non-directory")
                allowed.add(path)
        self.current_owned = {}
        return allowed

    def observe(self, argv, outcome):
        if self.clean_pi and argv == self._metadata_argv():
            return self._observe_metadata(argv, outcome)
        require(not self.closed and not self.finished and len(self.observations) < 6
                and type(argv) is list and argv and argv[0] == self.cli and all(type(v) is str and "\0" not in v for v in argv)
                and type(outcome) is dict, "public capture invocation differs")
        position = len(self.observations)
        capture = None
        try:
            selector = self.owners[self.name]["uuid"] if self.owners else None
            expected = self._argv(selector)
            stdout, stderr = outcome.get("stdout_full"), outcome.get("stderr_full")
            require(type(stdout) is str and type(stderr) is str and len(stdout.encode()) <= LIMIT and len(stderr.encode()) <= LIMIT
                    and type(outcome.get("exit_code")) is int and outcome.get("timeout") is False, "public capture is incomplete")
            capture = {"phase": COMMANDS[position], "argv": list(argv), "exit_code": outcome["exit_code"],
                "timeout": False, "stdout_bytes": len(stdout.encode()), "stderr_bytes": len(stderr.encode()),
                "stdout_sha256": sha(stdout.encode()), "stderr_sha256": sha(stderr.encode()),
                "stdout_base64": base64.b64encode(stdout.encode()).decode(), "stderr_base64": base64.b64encode(stderr.encode()).decode(),
                "document": None, "admission": "unqualified"}
            require(position < len(expected) and argv == expected[position], "public command/phase argv differs")
            document = strict_json(stdout.encode()) if stdout else None
            capture["document"] = copy.deepcopy(document)
            if self.negative:
                require(position < 2, "refusal must have only add/list observations")
                if position == 0:
                    require(outcome["exit_code"] == 1, "exact refused source unexpectedly succeeded")
                    self._refusal_capture(capture)
                else:
                    require(outcome["exit_code"] == 0 and stderr == ""
                            and exact(document, {"schema_version": 1, "command": "list", "result": "success", "data": {"installations": []}}), "refusal left public inventory")
                    require(exact(self._image(), self.current), "refusal list changed the original native frame")
                # Stager.prepareStage creates TargetRoot before this exact
                # source's projection fails. Discard removes staging only.
                # Admit these two empty 0700 parent directories, never any
                # child package, cache, coordination file, or entrypoint.
                allowed = set()
                for path in (self.profile, self.profile / "extensions"):
                    with member(self.dirs[self.case][0], self._relative(path)) as fd:
                        info = os.fstat(fd)
                        require(stat.S_ISDIR(info.st_mode) and stat.S_IMODE(info.st_mode) == 0o700
                                and info.st_uid == os.getuid(), "refusal parent is not a private owned directory")
                        value = {"kind": "dir", **facts(info)}
                    if str(path) in self.refusal_dirs:
                        require(exact(value, self.refusal_dirs[str(path)]), "refusal parent was replaced or changed")
                    else:
                        self.refusal_dirs[str(path)] = value
                    allowed.add(str(path))
                self.current = self._transition(allowed)
                self._hold_saved()
            else:
                require(outcome["exit_code"] == 0 and stderr == "", "public lifecycle command failed")
                if position == 0:
                    components = self._components(document)
                    owners, owned, allowed = self._installed(components)
                    self.components, self.owners = components, owners
                    self._public_operation(document)
                    self.first_owned = copy.deepcopy(owned)
                    self.first_lock = self._file(self.lock_root / "uap.lock")[0]
                elif position < 4:
                    owners, owned, allowed = self._installed(self.components)
                    require(exact(owned, self.first_owned) and self._file(self.lock_root / "uap.lock")[0] == self.first_lock, "repeat changed owned bytes/identity or portable lock")
                    require(exact(self._image(), self.current), "read/repeat changed the native profile's bytes or identity")
                    for alias, owner in owners.items():
                        original = self.owners[alias]
                        require(all(exact(owner[k], original[k]) for k in ("uuid", "name", "physical", "active", "binding_id", "revision", "source_key", "skills", "mcp_servers", "managed_digest"))
                                and exact(owner["record"]["source"], original["record"]["source"])
                                and exact(owner["record"]["package"], original["record"]["package"]), "repeat changed source or owner receipt")
                    if position == 2:
                        self._public_operation(document, repeated=True)
                    else:
                        self._public_list(document)
                else:
                    if position == 4:
                        self._public_operation(document, removed=True)
                        allowed = self._removed()
                    else:
                        require(exact(document, {"schema_version": 1, "command": "list", "result": "success", "data": {"installations": []}}), "final target list contains a reminted owner")
                        self.guard()
                        allowed = None
                    owned = {}
                if allowed is not None:
                    self.current = self._transition(allowed)
                self.current_owned = owned
                self._hold_saved()
                self.deliveries.append({"phase": COMMANDS[position], "native_frame": {root: frame_fact(value) for root, value in self.current.items()},
                    "saved_files": {path: fact for path, (_, fact) in self.current_files.items()},
                    "owned_trees": {path: frame_fact(value) for path, value in owned.items()}})
            capture["admission"] = "verified"
            self.observations.append(capture)
            self.guard()
        except (OSError, ValueError, KeyError, TypeError) as error:
            if capture is not None and len(self.observations) == position:
                self.observations.append(capture)
            self.errors.append(str(error)[:2000])
            raise PiQoderInstallationError(str(error)[:2000], self.evidence()) from error

    def _metadata_argv(self):
        return [self.cli, "skills", "list", self.item["source"],
            "--path", "skills/" + self.item["folder"], "--scope", "project",
            "--project-root", str(self.project), "--format", "json"]

    def _observe_metadata(self, argv, outcome):
        """The existing clean Pi metadata read never advances the six phases."""
        capture = None
        try:
            require(not self.closed and not self.finished and self.clean_pi
                    and len(self.observations) == 2 and self.source_metadata is None
                    and argv == self._metadata_argv() and type(outcome) is dict,
                    "clean Pi metadata call is not the one original read-only phase")
            stdout, stderr = outcome.get("stdout_full"), outcome.get("stderr_full")
            require(type(stdout) is str and type(stderr) is str
                    and len(stdout.encode()) <= LIMIT and len(stderr.encode()) <= LIMIT
                    and type(outcome.get("exit_code")) is int and outcome["exit_code"] == 0
                    and outcome.get("timeout") is False, "clean Pi metadata capture is incomplete")
            capture = {"phase": "source_metadata", "argv": list(argv), "exit_code": 0, "timeout": False,
                "stdout_bytes": len(stdout.encode()), "stderr_bytes": len(stderr.encode()),
                "stdout_sha256": sha(stdout.encode()), "stderr_sha256": sha(stderr.encode()),
                "stdout_base64": base64.b64encode(stdout.encode()).decode(),
                "stderr_base64": base64.b64encode(stderr.encode()).decode(),
                "document": None, "admission": "unqualified"}
            self.source_metadata = capture
            self.guard()
            require(stderr == "", "clean Pi metadata read emitted stderr")
            document = strict_json(stdout.encode())
            capture["document"] = copy.deepcopy(document)
            require(type(document) is dict and exact(document.get("schema_version"), 1)
                    and document.get("command") == "skills list" and document.get("result") == "success",
                    "clean Pi metadata envelope differs")
            data = document.get("data")
            require(type(data) is dict and data.get("scope") == "project"
                    and data.get("scope_selection_reason") == "explicit"
                    and data.get("source") == self.item["repo"] and data.get("revision") == self.item["ref"]
                    and data.get("source_path") == "skills/" + self.item["folder"]
                    and data.get("complete") is False and exact(data.get("diagnostics"), [{
                        "code": "exact_skill_selection", "severity": "warning",
                        "message": "Catalog scan is incomplete; explicitly scan a source folder to discover nested Skills"}]),
                    "clean Pi metadata source selection differs")
            candidates = data.get("candidates")
            require(type(candidates) is list and len(candidates) == 1 and type(candidates[0]) is dict,
                    "clean Pi metadata selected more than one candidate")
            candidate = candidates[0]
            require(candidate.get("name") == self.name and candidate.get("path") == "skills/" + self.item["folder"]
                    and candidate.get("status") == "available" and candidate.get("diagnostic_codes") in (None, [])
                    and type(candidate.get("description")) is str and bool(candidate["description"]),
                    "clean Pi metadata candidate differs")
            self.guard()
            capture["admission"] = "verified"
        except (OSError, ValueError, KeyError, TypeError) as error:
            self.errors.append(str(error)[:2000])
            raise PiQoderInstallationError(str(error)[:2000], self.evidence()) from error
        finally:
            try:
                self.guard()
            except (OSError, ValueError):
                if capture is not None:
                    capture["admission"] = "unqualified"
                raise

    def _argv(self, selector=None):
        scope = ["--scope", "project", "--project-root", str(self.project)] if self.kind == "skill" else ["--scope", "user"]
        add = ([self.cli, "skills", "add", self.source, "--skill", self.name, "--target", self.target, *scope,
                "--no-ui", "--yes", "--format", "json"] if self.kind == "skill" else
               [self.cli, "add", self.source, "--target", self.target, *scope, "--format", "json"])
        listed = [self.cli, "list", "--target", self.target, *scope, "--format", "json"]
        return [add, listed] if selector is None else [add, listed, add, listed,
            [self.cli, "remove", selector, "--target", self.target, *scope, "--format", "json"], listed]

    def _refusal_capture(self, failed):
        require(base64.b64decode(failed["stderr_base64"]).decode() == "agentplugins: dependency sync stopped; inspect binding results and next actions: 1 of 1 bindings failed: " + REFUSAL + "\n", "refusal diagnostic differs")
        doc = failed["document"]
        require(type(doc) is dict and exact(doc.get("schema_version"), 1) and doc.get("command") == "add" and doc.get("result") == "failure", "refusal outer envelope differs")
        data = doc.get("data")
        require(type(data) is dict and exact(data.get("schema_version"), 1) and data.get("command") == "agentplugins add"
                and data.get("result") == "failure" and data.get("outcome") == "partial" and data.get("scope") == "user"
                and data.get("scope_selection_reason") == "explicit"
                and exact(data.get("bindings"), [{"alias": self.name, "client_id": "pi", "status": "incomplete", "code": "FAILED"}]), "refusal typed state differs")
        results, actions = data.get("results"), data.get("actions")
        require(type(results) is list and len(results) == 1 and type(results[0]) is dict
                and type(results[0].get("installation_id")) is str and UUID.fullmatch(results[0]["installation_id"])
                and results[0].get("operation") == "install" and results[0].get("outcome") == "incomplete" and results[0].get("reason") == REFUSAL
                and type(actions) is list and len(actions) == 1 and type(actions[0]) is dict
                and actions[0].get("alias") == self.name and actions[0].get("client_id") == "pi"
                and actions[0].get("action") == "install" and actions[0].get("revision") == self.source.rsplit("@", 1)[1]
                and actions[0].get("package_digest") == "sha256:32fc89e62a200be082357d2cd5111284aa974b79678835cd60875de8fbba5958", "refusal source/owner differs")
        environment = data.get("environment")
        require(type(environment) is dict and environment.get("alias") == self.name
                and environment.get("saved") is True and environment.get("intent_change") is True
                and environment.get("scope") == "user" and environment.get("scope_selection_reason") == "explicit"
                and exact(environment.get("selected_targets"), ["pi"])
                and environment.get("lock_path") == str(self.lock_root / "uap.lock")
                and environment.get("manifest_path") == str(self.lock_root / "uap.toml"), "refusal saved scope differs")

    def _refused(self, row):
        require(len(self.observations) == 2 and [o["argv"] for o in self.observations] == self._argv()
                and row.get("status") == "documented-or-source-unsupported"
                and self.audit["expected_negative_coverage"](row) is True, "existing exact refusal contract differs")
        self._refusal_capture(self.observations[0])
        require(refusal_row(row), "refusal has materialized residue")
        if self._exists(self.state_root / "state-v2.json"):
            state, _ = self._document(self.state_root / "state-v2.json")
            require(type(state) is dict and exact(state.get("schema_version"), 5) and type(state.get("installations")) is list
                    and all(type(r) is dict and r.get("clients") == {} for r in state["installations"]), "refusal retained a saved client")

    def finish(self, row):
        require(not self.finished, "finish called twice")
        self.guard()
        try:
            require(not self.errors, "an earlier observation failed")
            require(type(row) is dict and (row.get("target"), row.get("kind"), row.get("source"), row.get("identity"))
                    == (self.target, self.kind, self.source, self.name) and exact(row.get("index"), self.index), "final row fixture identity differs")
            if self.negative:
                self._refused(row)
            else:
                uid = self.owners[self.name]["uuid"]
                require(len(self.observations) == 6 and [o["argv"] for o in self.observations] == self._argv(uid)
                        and set(self.selected) == set(self.owners), "six original-owner public calls missing")
                require(not self.clean_pi or type(self.source_metadata) is dict
                        and self.source_metadata.get("admission") == "verified", "clean Pi metadata read is missing")
                require(lifecycle_row(row, self.kind, uid, acceptance=self.acceptance), "original lifecycle, byte/lock or readiness gates differ")
            self.finished = True
            self.proof = {"status": "verified", "installation_qualified": not self.negative,
                          "exact_refusal_verified": self.negative, "original_status": row["status"]}
        except (ValueError, TypeError, KeyError) as error:
            self.errors.append(str(error))
            self.finished = True
            self.proof = {"status": "unqualified", "installation_qualified": False, "exact_refusal_verified": False,
                          "original_status": row.get("status") if type(row) is dict else None}
        return self.evidence()

    def evidence(self):
        return {"schema_version": 1, "scope": "owned installation lifecycle only", **QUALIFICATION,
            "target": getattr(self, "target", None), "kind": getattr(self, "kind", None), "index": getattr(self, "index", None),
            "source": getattr(self, "source", None), "identity": getattr(self, "name", None),
            "acceptance": getattr(self, "acceptance", None),
            **(self.proof or {"status": "unqualified", "installation_qualified": False, "exact_refusal_verified": False}),
            "case_root": str(getattr(self, "case", "")), "project_root": str(getattr(self, "project", "")),
            "fixed_absent_paths": [str(self.project / ".mcp.json")] if getattr(self, "target", None) == "qoder" else [],
            "source_authority": "immutable selector/signed tuple + saved lock/state + sealed managed/native receipts; no independent runtime assertion",
            "original_native_frames": {root: frame_fact(value) for root, value in getattr(self, "initial", {}).items()},
            "final_native_frames": {root: frame_fact(value) for root, value in getattr(self, "current", {}).items()},
            "foreign_files": {path: fact for path, (_, fact) in getattr(self, "foreign", {}).items()},
            "coordination_files": {path: fact for path, (_, fact) in self.coordination.items()},
            "runtime_stores": {path: fact for path, (_, fact) in self.runtime_stores.items()},
            "source_metadata": copy.deepcopy(self.source_metadata),
            "refusal_directories": copy.deepcopy(self.refusal_dirs),
            "owners": copy.deepcopy(self.owners), "selected": copy.deepcopy(self.selected),
            "observations": copy.deepcopy(self.observations), "deliveries": copy.deepcopy(self.deliveries), "errors": list(self.errors)}

    def close(self):
        if getattr(self, "closed", False):
            return
        self.closed = True
        for fd, _ in reversed(list(getattr(self, "dirs", {}).values())):
            os.close(fd)


def summary(report, fixtures, acceptance, *, signed_release_pins):
    try:
        return _summary(report, fixtures, acceptance, signed_release_pins=signed_release_pins)
    except (KeyError, TypeError, ValueError):
        return {"installation_passed": 0, "installation_verified_negative_coverage": 0,
                "installation_acceptance_passed": False,
                "qualification_scope": "owned Pi/Qoder installation lifecycle only", **QUALIFICATION}


def _summary(report, fixtures, acceptance, *, signed_release_pins):
    result = {"installation_passed": 0, "installation_verified_negative_coverage": 0, "installation_acceptance_passed": False,
              "qualification_scope": "owned Pi/Qoder installation lifecycle only", **QUALIFICATION}
    route = ROUTES.get(acceptance)
    if route is None or type(report) is not dict or type(fixtures) is not dict:
        return result
    target, kind = route
    clean = acceptance == PI_CLEAN_ACCEPTANCE
    prepared = acceptance in PROJECT_ACCEPTANCES and not clean
    if acceptance in PROJECT_ACCEPTANCES and report.get("acceptance") != acceptance:
        return result
    sources, rows = fixtures.get("skills" if kind == "skill" else "plugin_aliases"), report.get("results")
    version = "1.5.0" if target == "pi" else "1.1.81"
    if ((report.get("target"), report.get("kind"), report.get("platform"), report.get("architecture")) != (target, kind, "linux", "x86_64")
            or not exact(report.get("client_version"), {"status": "reported", "value": version, "exit_code": 0})
            or not exact(report.get("cli_version"), {"status": "reported", "value": "agentplugins 0.1.115", "exit_code": 0})
            or any(not exact(report.get(k), 30) for k in ("fixture_count", "cases_attempted"))
            or report.get("source_selection_complete") is not True or report.get("full_coverage") is not True
            or not exact(report.get("passed"), 30 if clean else 0)
            or not exact(report.get("prepared_lifecycle_passed"), 30 if prepared else 0)
            or not exact(report.get("verified_negative_coverage"), int(kind == "plugin"))
            or type(sources) is not list or len(sources) != 30 or type(rows) is not list or len(rows) != 30):
        return result
    for index, (row, item) in enumerate(zip(rows, sources)):
        source, identity = ((item["source"], item["name"]) if kind == "skill" else (item, fixtures["plugin_declared_names"].get(item, item)))
        proof = row.get("pi_qoder_installation_audit") if type(row) is dict else None
        negative = kind == "plugin" and index == 26
        if (type(proof) is not dict or proof.get("status") != "verified" or proof.get("errors") != []
                or not exact(proof.get("schema_version"), 1)
                or any(proof.get(key) is not False for key in QUALIFICATION)
                or (row.get("target"), row.get("kind"), row.get("source"), row.get("identity")) != (target, kind, source, identity)
                or not exact(row.get("index"), index)
                or any(not exact(proof.get(k), row.get(k)) for k in ("target", "kind", "source", "identity", "index"))
                or proof.get("original_status") != row.get("status")
                or proof.get("acceptance") != acceptance
                or proof.get("installation_qualified") is not (not negative) or proof.get("exact_refusal_verified") is not negative
                or len(proof.get("observations", [])) != (2 if negative else 6)
                or (not negative and (len(proof.get("deliveries", [])) != 6 or not proof.get("owners") or not proof.get("selected")))):
            return result
        if (not refusal_row(row) if negative else not lifecycle_row(
                row, kind, proof["owners"].get(identity, {}).get("uuid"), acceptance=acceptance)):
            return result
        if clean and (type(proof.get("source_metadata")) is not dict
                      or proof["source_metadata"].get("admission") != "verified"
                      or len(proof.get("runtime_stores", {})) != 2):
            return result
        if kind == "plugin" and not negative and not source.startswith("github:"):
            pin = signed_release_pins.get(source)
            owner = proof["owners"].get(identity, {})
            revision = owner.get("revision", {})
            if (type(pin) is not dict or revision.get("distribution_id") != pin["distribution"]
                    or not exact(revision.get("release_sequence"), pin["sequence"])
                    or revision.get("resolved_revision") != pin["revision"]
                    or any(revision.get(k) != pin[k] for k in ("tree_digest", "manifest_digest"))):
                return result
        result["installation_verified_negative_coverage" if negative else "installation_passed"] += 1
    result["installation_acceptance_passed"] = (result["installation_passed"] == (29 if kind == "plugin" else 30)
        and result["installation_verified_negative_coverage"] == int(kind == "plugin"))
    return result
