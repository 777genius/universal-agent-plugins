"""Exact Devin Global installation observations with separate native proof.

The caller supplies its validated fixture selections and existing source matcher.
Complete public JSON and native images stay in memory. Reports contain bounded
hashes, counts and checked public states, never native profile contents.
"""
import copy
import atexit
from functools import lru_cache
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys
import subprocess
from tempfile import TemporaryDirectory
import tomllib
from uuid import UUID

from codex_plugin_retention_audit import HeldTree

from windsurf_global_skill_audit import (CASES as GLOBAL_SKILL_CASES, frame_changes,
    frame, member, read_file, strict_json, managed_digest, immutable_source_digest, content, compact, sha, facts)

ACCEPTANCE = "windsurf-global-installation"
PREPARED = {"materialization": "materialized", "activation": "prepared",
            "authentication": "not_checked", "policy": "allowed",
            "verification": "installation_verified"}
ACTIVE_SKILLS = {**PREPARED, "activation": "active", "authentication": "not_required"}
ACTIVE_SLOTS = {22: "sre-extension", 25: "claude-scientific-writer",
                26: "elements-of-style", 29: "recipe-skills"}
COMPONENT_REFUSALS = {23: "itential-builder", 24: "google-cloud-storage",
                     27: "cometchat-skills", 28: "uizze-ui-slop"}
# Sanity is a prepared mixed-component source, outside the four native-only
# Skill cases. Digests are independently framed from all 130 immutable Git
# tree entries at this exact revision, including all 101 regular-file bodies.
PREPARED_GIT_PINS = {
    "github:sanity-io/agent-toolkit@3ed3a143f8d3413faa7790ac0b2274fd3a8a6320": {
        "tree_digest": "sha256:c02e90a28469ff7c67ac1bdae28bee66c4f280aac4f7f00e48509d0522b565c2",
        "manifest_digest": "sha256:3014016b8d78767b677a5a0b0ce05c03260caa15ecca6ff2efbecf79cf89a661",
    },
}
CLIENT_VERSION = "devin 3000.12.57 (96542e930752)"
CLI_VERSION = "agentplugins 0.1.115"
ROOT = Path(__file__).resolve().parents[1]


@lru_cache(maxsize=4)
def projection_helper(producer_revision=None):
    """Compile the canonical audit composition once, outside every case HOME."""
    temporary = TemporaryDirectory(prefix="windsurf-projection-oracle-")
    atexit.register(temporary.cleanup)
    binary = Path(temporary.name) / "oracle"
    subprocess.run(["go", "build", "-buildvcs=false", "-trimpath", "-o", str(binary),
                    "./scripts/testdata/windsurf_projection_oracle"], cwd=ROOT,
                   check=True, capture_output=True, timeout=180)
    return str(binary), producer_file(str(binary))


def producer_file(cli):
    """Pin the named executable to bounded held bytes, including replacement."""
    fd = os.open(cli, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC)
    try:
        info = os.fstat(fd)
        require(stat.S_ISREG(info.st_mode) and info.st_nlink == 1 and info.st_uid == os.getuid()
                and 0 < info.st_size <= 256 << 20)
        def identity(value):
            return (value.st_mode, value.st_dev, value.st_ino, value.st_uid, value.st_gid,
                    value.st_nlink, value.st_size, value.st_mtime_ns, value.st_ctime_ns)
        digest, size = hashlib.sha256(), 0
        while size < info.st_size:
            body = os.read(fd, min(65536, info.st_size - size))
            require(bool(body))
            size += len(body)
            digest.update(body)
        require(identity(info) == identity(os.fstat(fd)) == identity(os.lstat(cli)))
        return {"identity": identity(info), "sha256": digest.hexdigest()}
    finally:
        os.close(fd)


def verify_projection_producer(cli):
    """This lane qualifies the source-built workflow CLI, never another release."""
    original = producer_file(cli)
    built = subprocess.run(["go", "version", "-m", cli], check=True, capture_output=True,
                           text=True, timeout=30).stdout
    revision = subprocess.run(["git", "rev-parse", "HEAD"], cwd=ROOT, check=True,
                              capture_output=True, text=True, timeout=30).stdout.strip()
    require("\tpath\tgithub.com/777genius/plugin-kit-ai/cli/cmd/agentplugins\n" in built
            and "\tbuild\tvcs.revision=" + revision + "\n" in built
            and "\tbuild\tvcs.modified=false\n" in built)
    # VCS labels alone cannot authorize locally changed producer code.
    subprocess.run(["git", "diff", "--quiet", "HEAD"], cwd=ROOT,
                   check=True, capture_output=True, timeout=30)
    untracked = subprocess.run(["git", "ls-files", "--others", "--exclude-standard"],
                              cwd=ROOT, check=True, capture_output=True, timeout=30).stdout
    require(not untracked)
    require(producer_file(cli) == original)
    return {"revision": revision, "executable": original}


def canonical_projection(request, producer_revision=None):
    """Return only production-rendered expected bytes/digest; no receipt input."""
    try:
        binary, original = projection_helper(producer_revision)
        require(producer_file(binary) == original)
        completed = subprocess.run([binary], input=encoded(request, 1 << 20),
                                   check=True, capture_output=True, timeout=90)
        require(producer_file(binary) == original)
    except (OSError, subprocess.SubprocessError) as error:
        raise ValueError("independent Windsurf source projection is unavailable") from error
    result = strict_json(completed.stdout)
    require(type(result) is dict and set(result) == {
        "tree_digest", "manifest_digest", "delivery_digest", "staging_path"})
    require(all(type(result.get(key)) is str and re.fullmatch(r"sha256:[0-9a-f]{64}", result[key])
                for key in ("tree_digest", "manifest_digest", "delivery_digest")))
    return result


def require(condition):
    if not condition:
        raise ValueError("Devin Global installation evidence is unresolved")


def independent_source_pin(source, fixtures):
    """Keep prepared Git sources separate from the closed native Skill gate."""
    pin = (GLOBAL_SKILL_CASES.get(source, PREPARED_GIT_PINS.get(source))
           if source.startswith("github:") else fixtures["signed_release_pins"].get(source))
    require(type(pin) is dict)
    return pin


def encoded(value, limit):
    body = json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False).encode()
    require(len(body) <= limit)
    return body


def digest(value, limit=4 << 20):
    return hashlib.sha256(encoded(value, limit)).hexdigest()


def image_fact(image):
    return {"sha256": digest(image), "entries": len(image)}


def image_drift_counts(before, after):
    """Bounded counts by key class; never print native paths or values."""
    def category(key):
        if key.startswith("metadata:"):
            return "metadata"
        if key.startswith("directory:"):
            return "directory"
        if key.startswith("root-kind:"):
            return "root"
        if key.startswith(("link:", "root-link:")):
            return "link"
        return "file"

    differences = {}
    for status, entries in (
            ("added", set(after) - set(before)),
            ("removed", set(before) - set(after)),
            ("changed", {key for key in set(before) & set(after) if before[key] != after[key]})):
        for key in entries:
            label = category(key) + "_" + status
            differences[label] = differences.get(label, 0) + 1
    return ",".join(f"{key}={differences[key]}" for key in sorted(differences)) or "none"


def image_drift_diagnostic(before, after):
    """Bounded nonauthorizing roles; unknown native names and values stay private."""
    known = {
        "devin/mcp_config.json": "mcp_document",
        "metadata:2:mcp_config.json": "mcp_document",
        "devin/mcp_config.json.agentplugins.lock": "writer_lock",
        "metadata:2:mcp_config.json.agentplugins.lock": "writer_lock",
        "directory:2:skills": "skills_parent",
        "metadata:2:skills": "skills_parent",
        "devin/config.json": "native_config",
        "metadata:2:config.json": "native_config",
    }
    changed = sorted(
        key for key in set(before) | set(after)
        if (key in before) != (key in after)
        or key in before and before[key] != after[key]
    )
    entries = []
    for key in changed[:8]:
        entries.append({
            "role": known.get(key, "unknown"),
            "key_sha256": hashlib.sha256(
                key.encode("utf-8", "surrogatepass")
            ).hexdigest(),
            "operation": (
                "added" if key not in before
                else "removed" if key not in after
                else "changed"
            ),
            "before_present": key in before,
            "after_present": key in after,
        })
    return {
        "diagnostic_only": True,
        "changed_entries": len(changed),
        "omitted_entries": max(0, len(changed) - len(entries)),
        "entries": entries,
    }


def require_same_image(before, after, phase):
    if before != after:
        diagnostic = encoded(image_drift_diagnostic(before, after), 4096).decode()
        raise ValueError(
            "Devin Global native-image drift " + phase + ": "
            + image_drift_counts(before, after)
            + "; diagnostic=" + diagnostic
        )

def ordinary(path):
    info = path.lstat()
    require(stat.S_ISDIR(info.st_mode) and path.resolve(strict=True) == path)
    return info.st_mode, info.st_dev, info.st_ino, info.st_uid, info.st_gid


def selected(fixtures, index):
    require(type(index) is int and 0 <= index < 30)
    sources = fixtures["plugin_aliases"]
    require(type(sources) is list and len(sources) == 30)
    source = sources[index]
    require(type(source) is str)
    return source, fixtures["plugin_declared_names"].get(source, source)


def document(outcome, operation):
    require(type(outcome) is dict and type(outcome.get("exit_code")) is int
            and outcome["exit_code"] == 0 and outcome.get("timeout") is False
            and outcome.get("stderr_full") == "")
    raw = outcome.get("_json")
    require(type(raw) is dict and type(raw.get("schema_version")) is int
            and raw["schema_version"] == 1 and raw.get("result") == "success")
    encoded(raw, 1 << 20)
    require(raw.get("command") in (operation, "agentplugins " + operation))
    if operation == "list":
        payload = raw.get("data")
        require(type(payload) is dict and type(payload.get("installations")) is list)
    else:
        payload = raw.get("data", raw)
        require(type(payload) is dict and payload.get("command") == "agentplugins " + operation
                and payload.get("result") == "success" and payload.get("outcome") == "completed"
                and payload.get("scope") == "user" and payload.get("scope_selection_reason") == "explicit")
    return raw, payload


def native_reference(row, source, identity, index):
    """Join the separate closed native proof; installation never creates one."""
    require(index in ACTIVE_SLOTS and identity == ACTIVE_SLOTS[index]
            and source in GLOBAL_SKILL_CASES)
    pin = GLOBAL_SKILL_CASES[source]
    proof = row.get("windsurf_global_native_proof")
    require(type(proof) is dict and proof.get("scenario") == "four-immutable-global-pure-skills"
            and proof.get("source") == source and proof.get("native_discovery_verified") is True
            and type(proof.get("cache_exception_granted")) is bool
            and proof.get("writer_closure_claimed") is False
            and proof.get("reasons") == []
            and encoded(proof.get("independent_source_pin"), 1 << 20) == encoded(pin, 1 << 20))
    owner, stored = proof.get("owner"), proof.get("immutable_source_store")
    require(type(owner) is dict and owner.get("name") == identity
            and owner.get("native_lifecycle_qualified") is True
            and type(owner.get("uuid")) is str and str(UUID(owner["uuid"])) == owner["uuid"]
            and owner.get("revision") == {"version": pin["version"], "resolved_revision": source.rsplit("@", 1)[1],
                "tree_digest": pin["tree_digest"], "manifest_digest": pin["manifest_digest"]}
            and row.get("matched_installation_ids") == row.get("repeat_installation_ids") == [owner["uuid"]]
            and type(stored) is dict and stored.get("tree_digest") == pin["tree_digest"]
            and stored.get("complete_skills") == list(pin["skills"])
            and type(stored.get("held_frame")) is dict and stored["held_frame"] == stored.get("last_verified_frame"))
    if "source_skills" in pin:
        require(stored.get("source_skills") == list(pin["source_skills"])
                and stored.get("rejected_skills") == pin["rejected_skills"])
    observations, transitions = proof.get("observations"), proof.get("public_transitions")
    require(type(observations) is list and all(type(v) is dict for v in observations)
            and [v.get("phase") for v in observations]
            == ["before", "add", "repeat", "remove"]
            and all(v.get("verified") is True and type(v.get("custody")) is dict
                    and (v["custody"].get("unchanged") is True
                        and v["custody"].get("before") == v["custody"].get("after")
                        and v["custody"].get("native_cache_only") is False
                         or v["custody"].get("unchanged") is False
                        and v["custody"].get("before") != v["custody"].get("after")
                        and v["custody"].get("native_cache_only") is True
                        and type(v["custody"].get("cache_changed_entries")) is int
                        and 1 <= v["custody"]["cache_changed_entries"] <= 40)
                    for v in observations)
            and type(transitions) is list and all(type(v) is dict for v in transitions)
            and [v.get("phase") for v in transitions]
            == ["add", "repeat", "remove"] and all(v.get("verified") is True for v in transitions))
    cache_observations = [v for v in observations if v["custody"].get("native_cache_only") is True]
    cache_events = proof.get("native_cache_evidence")
    expected_events = [{"phase": observed["phase"], "before": observed["custody"]["before"],
                        "after": observed["custody"]["after"],
                        "changed_entries": observed["custody"]["cache_changed_entries"]}
                       for observed in cache_observations]
    for transition in transitions:
        if "native_cache_custody" in transition:
            event = transition["native_cache_custody"]
            require(type(event) is dict and event.get("phase") == "public-" + transition["phase"]
                    and transition["phase"] in ("add", "repeat"))
            expected_events.append(event)
    require(type(cache_events) is list and len(cache_events) == len(expected_events)
            and proof["cache_exception_granted"] is bool(cache_events)
            and all(type(event) is dict and set(event) == {
                "phase", "before", "after", "changed_entries"}
                and type(event["changed_entries"]) is int and 1 <= event["changed_entries"] <= 40
                and event["before"] != event["after"] for event in cache_events)
            and sorted(cache_events, key=lambda event: event["phase"])
                == sorted(expected_events, key=lambda event: event["phase"]))
    return {"source": source, "installation_id": owner["uuid"], "sha256": digest(proof)}


class WindsurfInstallationAudit:
    """Observe each actual public call before the harness consumes `_json`."""

    def __init__(self, cli, env, project, case_root, audit, *, native_runtime=None):
        require(sys.platform == "linux" and type(cli) is str and Path(cli).is_absolute())
        self.cli, self.audit, self.native_runtime = cli, audit, native_runtime
        self.producer = verify_projection_producer(cli)
        self.case, self.project = Path(case_root), Path(project)
        self.home, self.data = self.case / "home", self.case / "uap-data"
        self.xdg = self.home / ".config"
        require(self.case.resolve(strict=True) == self.case and self.project == self.case / "project"
                and env.get("HOME") == str(self.home)
                and env.get("XDG_CONFIG_HOME") == str(self.xdg)
                and env.get("AGENTPLUGINS_HOME") == str(self.data))
        self.anchors = {p: ordinary(p) for p in
                        (self.case, self.home, self.xdg, self.project, self.data)}
        self.global_root = self.xdg / "devin"
        if native_runtime is None:
            # Validate the canonical directory before creating a sentinel, and
            # resolve every write through held descriptors. A substituted path
            # cannot redirect the write into a sibling, even inside this case.
            self.foreign = self.global_root / "foreign-neighbor.txt"
            directory = os.open(self.xdg, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
            try:
                require(ordinary(self.xdg) == self.directory_identity(os.fstat(directory)) == self.anchors[self.xdg])
                try:
                    os.mkdir("devin", mode=0o700, dir_fd=directory)
                except FileExistsError:
                    pass
                require(ordinary(self.xdg) == self.directory_identity(os.fstat(directory)) == self.anchors[self.xdg])
                self.anchors[self.global_root] = ordinary(self.global_root)
                held = os.open("devin", os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=directory)
                try:
                    require(self.directory_identity(os.fstat(held)) == self.anchors[self.global_root])
                    fd = os.open(self.foreign.name, os.O_WRONLY | os.O_CREAT | os.O_EXCL
                                 | os.O_NOFOLLOW | os.O_CLOEXEC, 0o600, dir_fd=held)
                    try:
                        body = b"unmanaged Devin Global neighbor must survive\n"
                        require(os.write(fd, body) == len(body))
                    finally:
                        os.close(fd)
                    require(ordinary(self.global_root) == self.directory_identity(os.fstat(held))
                            == self.anchors[self.global_root])
                finally:
                    os.close(held)
            finally:
                os.close(directory)
        else:
            # Reuse only the exact native observer's authored, still pre-child
            # profile. Adding another file would invalidate its frozen HOME.
            native_class = audit.get("WINDSURF_GLOBAL_AUDIT", {}).get("WindsurfGlobalSkillAudit")
            require(type(native_class) is type and type(native_runtime) is native_class)
            require(native_runtime.case == self.case and native_runtime.home == self.home
                    and native_runtime.project == self.project and native_runtime.data == self.data
                    and native_runtime.profile == self.global_root and native_runtime.item in GLOBAL_SKILL_CASES
                    and encoded(native_runtime.pin, 1 << 20) == encoded(GLOBAL_SKILL_CASES[native_runtime.item], 1 << 20)
                    and native_runtime.closed is False and native_runtime.finished is False
                    and native_runtime.removing is False and native_runtime.owner is None
                    and native_runtime.probe_frame is None and native_runtime.bound_list == set()
                    and native_runtime.observations == native_runtime.transitions == native_runtime.reasons == [])
            self.foreign = native_runtime.skills / "foreign-windsurf-global-audit/SKILL.md"
        self.anchors[self.global_root] = ordinary(self.global_root)
        self.native_roots = (self.home / ".windsurf", self.home / ".devin", self.global_root)
        self.observations, self.saved_observations, self.source_projection = [], [], None
        self.infrastructure_frames, self.retained_infrastructure = [], []
        self.initial = self.image()
        self.terminal_image = dict(self.initial)
        self.initial_saved = self.saved_image()
        self.project_initial = self.audit["scoped_snapshot"]((self.project,))

    @staticmethod
    def directory_identity(info):
        return info.st_mode, info.st_dev, info.st_ino, info.st_uid, info.st_gid

    def runtime_roots(self, data_fd, state):
        """Qualify npm link namespaces from retained ownership and source bytes."""
        roots = set()
        for record in state["installations"]:
            receipts = record.get("data_receipts", {})
            require(type(receipts) is dict)
            for receipt_id, receipt in receipts.items():
                require(type(receipt) is dict)
                physical = receipt.get("physical_backend_id")
                require(type(physical) is str and re.fullmatch(r"[a-z0-9][a-z0-9.-]{0,63}", physical))
                require(physical == record["declared_name"][:51].rstrip(".-") + "-"
                        + sha(record["installation_id"].encode())[:12])
                relative = "plugin-data/" + physical
                locator = str(self.data / relative)
                require(receipt_id == "data_" + sha("\0".join((record["installation_id"], physical, "user")).encode())[:24]
                        and receipt.get("data_receipt_id") == receipt_id
                        and receipt.get("scope") == "user" and receipt.get("state") == "owned"
                        and receipt.get("locator") == locator
                        and receipt.get("ownership_digest") == "sha256:" + sha(
                            ("agentplugins-plugin-data-v1\0" + receipt_id + "\0" + locator).encode()))
                require(encoded(strict_json(read_file(data_fd, relative + "/.agentplugins-data-owner.json")[0]), 1 << 20)
                        == encoded(receipt, 1 << 20))
                with member(data_fd, relative) as plugin_fd:
                    if "npm-runtime" not in os.listdir(plugin_fd):
                        continue
                bindings = record.get("clients")
                require(type(bindings) is dict)
                if bindings:
                    require(len(bindings) == 1 and all(binding.get("client_id") == "windsurf"
                            and binding.get("physical_artifact_id") == physical
                            and binding.get("data_receipt_id") == receipt_id for binding in bindings.values()))
                else:
                    require(record.get("data_retained") is True)
                tree_digest = record.get("source", {}).get("tree_digest")
                require(type(tree_digest) is str and re.fullmatch(r"sha256:[0-9a-f]{64}", tree_digest))
                source_relative = "store/packages/agentplugins-tree-sha256-v1/" + tree_digest[7:] + "/package"
                with member(data_fd, source_relative) as source_fd:
                    source_image = frame(source_fd)
                    require(immutable_source_digest(source_fd, source_image) == tree_digest)
                    source_runtime = "io.github.777genius.agentplugins/runtime/"
                    config = strict_json(read_file(source_fd, source_runtime + "runtime.json")[0])
                    package_body = read_file(source_fd, source_runtime + "package.json")[0]
                    lock_body = read_file(source_fd, source_runtime + "package-lock.json")[0]
                require(type(config) is dict and type(config.get("schema_version")) is int
                        and config["schema_version"] == 1
                        and type(config.get("omit_optional", False)) is bool)
                lock_digest = "sha256:" + sha(lock_body)
                require(config.get("package_lock_sha256") == lock_digest)
                runtime = relative + "/npm-runtime/" + lock_digest[7:]
                require(read_file(data_fd, runtime + "/package.json")[0] == package_body
                        and read_file(data_fd, runtime + "/package-lock.json")[0] == lock_body)
                expected = {"schema_version": 1, "lock_digest": lock_digest,
                            **{key: config[key] for key in ("package", "version", "entrypoint")},
                            "omit_optional": config.get("omit_optional", False)}
                require(encoded(strict_json(read_file(data_fd, runtime + "/.agentplugins-runtime.json")[0]), 1 << 20)
                        == encoded(expected, 1 << 20))
                roots.add(runtime + "/node_modules")
        return roots

    def data_frame(self, data_fd, roots):
        """Reuse held full-byte custody; only qualified npm .bin links may pass."""
        if not roots:
            return frame(data_fd)
        tree = HeldTree(self.data, bin_links=True)
        try:
            require(self.directory_identity(os.fstat(tree.fds["."]))
                    == self.directory_identity(os.fstat(data_fd)))
            result = {}
            for name, (kind, _, value) in tree.nodes.items():
                relative = "" if name == "." else name
                info = os.fstat(tree.fds[name])
                require(info.st_uid == os.getuid())
                # Preserve frame()/custody.tree's existing exclusions even when
                # qualified npm bin links require the shared HeldTree reader.
                require(not (kind == "directory" and Path(relative).name == ".git")
                        and not (kind == "file" and Path(relative).name in (
                            ".uap-integrations.lock", ".plugin-kit-ai.lock")))
                row = {"kind": "dir" if kind == "directory" else kind, **facts(info)}
                if kind == "link":
                    root = str(Path(relative).parent.parent)
                    require(root in roots and Path(relative).parent.name == ".bin")
                    target = tree._link_target(name, value)
                    require(target.startswith(root + "/"))
                    row["target"] = value
                elif kind == "file":
                    row["sha256"] = value["sha256"]
                result[relative] = row
            tree.verify()
            return result
        finally:
            tree.close()

    def saved_image(self):
        """Read existing saved authorities through the shared bounded custody API."""
        self.guard()
        fd = os.open(self.case, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        try:
            require(self.directory_identity(os.fstat(fd)) == self.anchors[self.case])
            with member(fd, "uap-data") as data_fd:
                require(self.directory_identity(os.fstat(data_fd)) == self.anchors[self.data])
                docs, blobs, authorities = {}, {}, {}
                names = os.listdir(data_fd)
                for name in ("uap.toml", "uap.lock", "state-v2.json"):
                    if name in names:
                        body, authorities[name] = read_file(data_fd, name)
                        blobs[name] = hashlib.sha256(body).hexdigest()
                        docs[name] = (tomllib.loads(body.decode("utf-8", errors="strict"))
                                      if name.endswith("toml") else strict_json(body))
                if "uap.toml" in docs:
                    require(type(docs["uap.toml"].get("schema_version")) is int
                            and docs["uap.toml"]["schema_version"] == 1)
                if "uap.lock" in docs:
                    locked = docs["uap.lock"]
                    require(type(locked) is dict and type(locked.get("lock_version")) is int
                            and locked["lock_version"] == 1 and type(locked.get("packages")) is list)
                deliveries, sources = {}, {}
                state = docs.get("state-v2.json", {"schema_version": 5, "installations": []})
                require(type(state) is dict and type(state.get("schema_version")) is int
                        and state["schema_version"] == 5 and type(state.get("installations")) is list)
                roots = self.runtime_roots(data_fd, state)
                observed = self.data_frame(data_fd, roots)
                require(all(observed[name] == {"kind": "file", **fact}
                            for name, fact in authorities.items()))
                for record in state["installations"]:
                    require(type(record) is dict and type(record.get("clients")) is dict)
                    for binding in record["clients"].values():
                        require(type(binding) is dict)
                        if binding.get("client_id") != "windsurf":
                            continue
                        physical = binding.get("physical_artifact_id")
                        require(type(physical) is str and re.fullmatch(r"[a-z0-9][a-z0-9.-]{0,63}", physical))
                        relative = "managed/clients/windsurf/" + physical
                        require(binding.get("target_locator") == str(self.data / relative))
                        with member(data_fd, relative) as managed_fd:
                            tree = frame(managed_fd)
                            deliveries[relative] = {"image": tree, "digest": managed_digest(managed_fd, tree)}
                        revision = binding.get("package_revision")
                        require(type(revision) is dict and type(revision.get("tree_digest")) is str
                                and re.fullmatch(r"sha256:[0-9a-f]{64}", revision["tree_digest"]))
                        source_relative = "store/packages/agentplugins-tree-sha256-v1/" + revision["tree_digest"][7:] + "/package"
                        with member(data_fd, source_relative) as source_fd:
                            source_image = frame(source_fd)
                            sources[source_relative] = {"image": source_image,
                                "digest": immutable_source_digest(source_fd, source_image)}
                require(observed == self.data_frame(data_fd, self.runtime_roots(data_fd, state)))
        except OSError as error:
            raise ValueError("Devin Global saved installation authority is unavailable") from error
        finally:
            os.close(fd)
        self.guard()
        # Cache trees remain bounded by frame(); saved documents and managed
        # ownership are checked separately. Other data files cannot be silently
        # created by an exact refusal. Cache names come from main.go; cache
        # adapters append .lock during authenticated reconciliation. Store and
        # scanner locations come from compose.go. frame() supplies the existing
        # regular-file, owner, link, byte, entry and depth bounds for all of them.
        ancillary = {key: value for key, value in observed.items() if value["kind"] == "file"
                     and key not in {"uap.toml", "uap.lock", "state-v2.json", "mutation.lock", "environment.lock",
                                     "directory-v1-cache.json", "directory-v1-cache.json.lock",
                                     "discovery-v1-cache.json", "discovery-v1-cache.json.lock"}
                     and not key.startswith(("store/", "security/", "managed/"))}
        return {"documents": docs, "blobs": blobs, "authorities": authorities, "deliveries": deliveries,
                "sources": sources,
                "runtime_authority": {key: value for key, value in observed.items()
                    if any(key == root or key.startswith(root + "/") for root in roots)},
                "managed_authority": {key: value for key, value in observed.items() if key.startswith("managed/")},
                "ancillary": content(ancillary),
                "managed": content({key: value for key, value in observed.items()
                                    if key.startswith("managed/")})}

    @staticmethod
    def saved_intent(saved):
        docs = saved["documents"]
        manifest, locked = docs.get("uap.toml", {}), docs.get("uap.lock", {})
        require(type(manifest) is dict and type(locked) is dict)
        return {"dependencies": manifest.get("dependencies", {}),
                "dev-dependencies": manifest.get("dev-dependencies", {}),
                "targets": {key: value for key, value in manifest.get("targets", {}).items() if value},
                "packages": locked.get("packages", []),
                "installations": docs.get("state-v2.json", {}).get("installations", [])}

    def guard(self):
        require(all(ordinary(path) == identity for path, identity in self.anchors.items()))
        for root in self.native_roots:
            require(self.audit["path_inside_case"](root, self.case))
        if self.native_runtime is not None:
            self.native_runtime.guard()

    def image(self):
        self.guard()
        image = self.audit["scoped_snapshot"](self.native_roots)
        require(type(image) is dict and all(type(key) is str for key in image)
                and not any(key.startswith(("link:", "root-link:")) for key in image)
                and not any(value in ("unreadable", "symlink", "non-regular")
                            for value in image.values()))
        if self.native_runtime is not None and self.native_runtime.native_cache_events:
            # This is strictly the independently qualified pinned Devin CLI
            # cache. No public UAP operation may write it: runtime.guard()
            # anchors the full HOME including every cached file and digest.
            image = {key: value for key, value in image.items()
                if not (key == "directory:2:cli" or key.startswith("directory:2:cli/")
                        or key == "metadata:2:cli" or key.startswith("metadata:2:cli/")
                        or key == "devin/cli" or key.startswith("devin/cli/"))}
        encoded(image, 4 << 20)
        self.guard()
        return image

    def _lock_frame(self, image):
        name = "mcp_config.json.agentplugins.lock"
        key = "devin/" + name
        if key not in image or key in self.initial:
            return None
        path = self.global_root / name
        native = self.audit["NATIVE_ARTIFACTS"]
        before = path.lstat()
        require(stat.S_ISREG(before.st_mode) and before.st_nlink == 1
                and 0 <= before.st_size <= 4096)
        descriptor = native["open_member"](path)
        try:
            opened = os.fstat(descriptor)
            require(native["same_opened"](before, opened)
                    and (opened.st_uid, opened.st_gid) == (before.st_uid, before.st_gid))
            body = os.read(descriptor, before.st_size + 1)
            require(len(body) == before.st_size)
            final_held = os.fstat(descriptor)
            final_named = path.lstat()
            require(native["identity"](opened) == native["identity"](final_held)
                    == native["identity"](final_named)
                    and (before.st_uid, before.st_gid) == (opened.st_uid, opened.st_gid)
                    == (final_held.st_uid, final_held.st_gid)
                    == (final_named.st_uid, final_named.st_gid))
        finally:
            os.close(descriptor)
        identity = (before.st_mode, before.st_dev, before.st_ino,
                    before.st_uid, before.st_gid, before.st_nlink, before.st_mtime_ns)
        require(image.get(key) == hashlib.sha256(body).hexdigest()
                and image.get("metadata:2:" + name)
                == (before.st_mode, before.st_dev, before.st_ino, before.st_mtime_ns))
        return identity, body

    def _infrastructure_frame(self, image):
        self.guard()
        try:
            lock = self._lock_frame(image)
            skill = None
            key = "directory:2:skills"
            if key in image and key not in self.initial:
                skill = ordinary(self.global_root / "skills")
                require(image.get("metadata:2:skills") == skill[:3])
            facts = (lock, skill)
        except Exception:
            # Extra retention proof is optional for exact restoration.
            # An unproven frame cannot authorize retained infrastructure.
            facts = None
        self.guard()
        return facts

    def _neutral_file(self, name, frame):
        require(name in ("mcp_config.json", "mcp_config.json.agentplugins.lock"))
        self.guard()
        path = self.global_root / name
        require(self.audit["path_inside_case"](path, self.case))
        native = self.audit["NATIVE_ARTIFACTS"]
        before = path.lstat()
        require(stat.S_ISREG(before.st_mode) and before.st_mode == 0o100600
                and before.st_nlink == 1 and 0 <= before.st_size <= 4096
                and (before.st_uid, before.st_gid) == self.anchors[self.global_root][3:])
        descriptor = native["open_member"](path)
        try:
            opened = os.fstat(descriptor)
            require(native["same_opened"](before, opened)
                    and (opened.st_uid, opened.st_gid) == (before.st_uid, before.st_gid))
            body = os.read(descriptor, before.st_size + 1)
            require(len(body) == before.st_size)
            final_held = os.fstat(descriptor)
            final_named = path.lstat()
            require(native["identity"](opened) == native["identity"](final_held)
                    == native["identity"](final_named)
                    and (before.st_uid, before.st_gid) == (opened.st_uid, opened.st_gid)
                    == (final_held.st_uid, final_held.st_gid)
                    == (final_named.st_uid, final_named.st_gid))
        finally:
            os.close(descriptor)
        if name == "mcp_config.json":
            value = json.loads(body.decode("utf-8"), object_pairs_hook=self.audit["_unique_json_object"],
                               parse_constant=self.audit["_invalid_json_constant"])
            require(type(value) is dict and set(value) == {"mcpServers"}
                    and type(value["mcpServers"]) is dict and not value["mcpServers"])
        else:
            require(body == b"")
        file_key, metadata_key = "devin/" + name, "metadata:2:" + name
        metadata = (before.st_mode, before.st_dev, before.st_ino, before.st_mtime_ns)
        require(frame.get(file_key) == hashlib.sha256(body).hexdigest()
                and frame.get(metadata_key) == metadata)
        self.guard()
        return {file_key: frame[file_key], metadata_key: metadata}

    def _expected_removed_image(self, frames, *, mcp, skills):
        require(type(mcp) is bool and type(skills) is bool and len(frames) == 6)
        require_same_image(frames[4], frames[5], "remove/list")
        if frames[4] == self.initial:
            require_same_image(self.initial, self.image(), "final/original")
            self.guard()
            return dict(self.initial), []
        require(len(self.infrastructure_frames) == 6
                and all(type(fact) is tuple and len(fact) == 2
                        for fact in self.infrastructure_frames))
        expected, roles = dict(self.initial), []
        if mcp:
            for name, role in (("mcp_config.json", "mcp_document"),
                               ("mcp_config.json.agentplugins.lock", "writer_lock")):
                keys = ("devin/" + name, "metadata:2:" + name)
                require(all(key not in self.initial for key in keys))
                require(all(key in frame for frame in frames for key in keys))
                additions = self._neutral_file(name, frames[5])
                if role == "writer_lock":
                    require(all(all(frame[key] == additions[key] for key in keys) for frame in frames))
                    locked = self.infrastructure_frames[5][0]
                    require(locked is not None and locked[1] == b""
                            and locked[0][0] == 0o100600
                            and locked[0][3:6] == (*self.anchors[self.global_root][3:], 1)
                            and all(fact[0] == locked for fact in self.infrastructure_frames))
                expected.update(additions)
                roles.append(role)
        if skills:
            keys = ("directory:2:skills", "metadata:2:skills")
            require(all(key not in self.initial for key in keys))
            path = self.global_root / "skills"
            identity = ordinary(path)
            require(identity[0] == 0o40700
                    and identity[3:] == self.anchors[self.global_root][3:]
                    and all(fact[1] == identity for fact in self.infrastructure_frames))
            with os.scandir(path) as entries:
                require(next(entries, None) is None)
            require(ordinary(path) == identity)
            additions = {keys[0]: "present", keys[1]: identity[:3]}
            require(all(all(frame.get(key) == value for key, value in additions.items())
                        for frame in frames))
            expected.update(additions)
            roles.append("skills_parent")
        self.guard()
        require_same_image(expected, frames[4], "remove/neutral-profile")
        require_same_image(expected, frames[5], "post-remove/neutral-profile")
        require_same_image(frames[5], self.image(), "final/observed")
        return expected, sorted(roles)

    def removed_empty_native_coordination(self, observed):
        """Accept only an exact empty MCP writer file/lock and optional empty Skills parent.

        The common nativeconfig writer leaves these inert paths after removal.
        No pre-existing entry, foreign file or native registration may change.
        """
        if any(observed.get(key) != value for key, value in self.initial.items()):
            return False
        added = set(observed) - set(self.initial)
        files = {"devin/mcp_config.json", "metadata:2:mcp_config.json",
                 "devin/mcp_config.json.agentplugins.lock",
                 "metadata:2:mcp_config.json.agentplugins.lock"}
        optional = {"directory:2:skills", "metadata:2:skills"}
        if added not in (files, files | optional):
            return False
        fd = os.open(self.global_root,
                     os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC)
        try:
            require(self.directory_identity(os.fstat(fd)) == self.anchors[self.global_root])
            config, config_fact = read_file(fd, "mcp_config.json", limit=4096)
            lock, lock_fact = read_file(fd, "mcp_config.json.agentplugins.lock", limit=4096)
            require(strict_json(config) == {"mcpServers": {}} and lock == b""
                    and stat.S_IMODE(config_fact["mode"]) == 0o600
                    and stat.S_IMODE(lock_fact["mode"]) == 0o600)
            if added == files | optional:
                with member(fd, "skills") as skill_fd:
                    require(set(frame(skill_fd)) == {""})
            require(self.directory_identity(os.fstat(fd)) == self.anchors[self.global_root])
        finally:
            os.close(fd)
        return self.image() == observed

    def observe(self, argv, outcome):
        """Consume complete existing parsed JSON, without rerunning any command."""
        require(type(argv) is list and argv and argv[0] == self.cli and len(self.observations) < 6
                and all(type(arg) is str and "\0" not in arg for arg in argv))
        require(type(outcome) is dict)
        encoded(outcome.get("_json"), 1 << 20)
        image = self.image()
        require(self.audit["scoped_snapshot"]((self.project,)) == self.project_initial)
        # Existing foreign files and directory identities remain authoritative.
        require(all(image.get(key) == value for key, value in self.initial.items()))
        self.infrastructure_frames.append(self._infrastructure_frame(image))
        self.saved_observations.append(self.saved_image())
        self.observations.append((list(argv), copy.deepcopy(outcome), image))

    def finish(self, row, item, index, fixtures):
        source, identity = selected(fixtures, index)
        require(item == source and row.get("index") == index
                and (row.get("target"), row.get("kind"), row.get("source"), row.get("identity"))
                == ("windsurf", "plugin", source, identity))
        native_proof = self.native_runtime.evidence() if self.native_runtime is not None else None
        reference = None
        if row.get("status") == "passed":
            require(self.native_runtime is not None and self.native_runtime.finished is True)
            reference = native_reference(row, source, identity, index)
        if self.native_runtime is not None:
            require(index in ACTIVE_SLOTS and self.native_runtime.item == source)
            self.same_native_evidence(row.get("windsurf_global_native_proof"), native_proof)
        self.guard()
        require(self.image() == (self.observations[-1][2] if self.observations else self.initial))
        require(self.audit["scoped_snapshot"]((self.project,)) == self.project_initial)
        require(self.saved_observations and self.saved_image() == self.saved_observations[-1])
        scope = ["--target", "windsurf", "--scope", "user", "--format", "json"]
        add, listed = [self.cli, "add", source, *scope], [self.cli, "list", *scope]
        negative = index == 13 or index in COMPONENT_REFUSALS
        remove_identity = identity
        if self.native_runtime is not None:
            owner = self.native_runtime.owner
            require(type(owner) is dict and row.get("matched_installation_ids") == [owner.get("uuid")])
            remove_identity = owner["uuid"]
        expected = [add, listed] if negative else [add, listed, add, listed,
                    [self.cli, "remove", remove_identity, *scope], listed]
        require([entry[0] for entry in self.observations] == expected)
        require(row.get("sentinels_preserved") is True
                and all(row.get(key) is False for key in (
                    "failed_install_registered", "failed_install_residue", "post_remove_identity_registered"))
                and row.get("native_residuals") == [])
        if negative:
            self.negative(row, source, identity, index)
            state = None
        else:
            state = self.positive(row, source, identity, index, fixtures)
        if self.native_runtime is not None:
            # Never call finish() twice or rewrite the row. Read-only guards may
            # advance their diagnostic count, but cannot alter any proof fact.
            self.same_native_evidence(native_proof, self.native_runtime.evidence())
        return {"status": "verified-installation-negative" if negative else "verified-installation-lifecycle",
                **({"preserved_native_proof": reference} if reference is not None else {}),
                "native_post_remove_state": ("empty-writer-coordination"
                    if not negative and self.retained_empty_native_coordination else "pristine"),
                "scope": "user", "canonical_client_id": "windsurf", "fixture_index": index,
                "source": source, "identity": identity, "public_state": state,
                "source_projection": self.source_projection,
                "native_runtime_qualified": False, "writer_closure_claimed": False,
                "native_roots": [".windsurf", ".devin", ".config/devin"],
                "project_unchanged": True, "original_native_preserved": True,
                "initial_native_image": image_fact(self.initial),
                "terminal_mode": ("retained-neutral-infrastructure" if self.retained_infrastructure
                                  else "restored-original-image"),
                "retained_infrastructure": list(self.retained_infrastructure),
                "terminal_native_image": image_fact(self.terminal_image),
                "initial_saved_authority_sha256": digest(self.initial_saved),
                "observations": [{"argv": argv, "json_sha256": digest(outcome.get("_json"), 1 << 20),
                                  "native_image": image_fact(image), "saved_authority_sha256": digest(saved)}
                                 for (argv, outcome, image), saved in zip(self.observations, self.saved_observations)]}

    @staticmethod
    def same_native_evidence(before, after):
        require(type(before) is dict and type(after) is dict)
        prior, current = before.get("successful_guards"), after.get("successful_guards")
        require(type(prior) is int and type(current) is int and 0 < prior <= current
                and encoded({k: v for k, v in before.items() if k != "successful_guards"}, 4 << 20)
                == encoded({k: v for k, v in after.items() if k != "successful_guards"}, 4 << 20))

    def negative(self, row, source, identity, index):
        require(all(image == self.initial for _, _, image in self.observations))
        # These exact refusals precede saved publication: Directory selection
        # fails in resolveSavedAdd; component preflight fails in prepareSavedSync
        # before applyDependencySync. list only loads state. No authority-file
        # initialization or replacement is legitimate on either refusal path.
        require(all(saved["authorities"] == self.initial_saved["authorities"]
                    and saved["managed_authority"] == self.initial_saved["managed_authority"]
                    and saved["ancillary"] == self.initial_saved["ancillary"]
                    for saved in self.saved_observations))
        first, second = (entry[1] for entry in self.observations)
        if index == 13:
            require(source == identity == "marketing-kit"
                    and row.get("status") == "documented-or-source-unsupported")
            diagnostic = ('agentplugins: no eligible directory release for "marketing-kit": '
                          '777genius/marketing-kit: release does not support complete target set; missing windsurf\n')
        else:
            require(identity == COMPONENT_REFUSALS[index]
                    and re.fullmatch(r"github:[^\s@]+@[0-9a-f]{40}", source)
                    and row.get("status") == "install-failed")
            diagnostic = (f"agentplugins: preflight {identity}/windsurf: "
                          "devin Global supports only selected Skills and MCP components\n")
        require(type(first.get("exit_code")) is int and first["exit_code"] == 1
                and first.get("timeout") is False and first.get("stdout_tail") == ""
                and first.get("stderr_full") == diagnostic and first.get("_json") is None
                and row.get("listed_identity") is False
                and type(row.get("native_file_delta")) is int and row["native_file_delta"] == 0
                and all(row.get(key) == [] for key in (
                    "native_identity_files", "native_component_candidates", "matched_installation_ids",
                    "repeat_installation_ids", "post_removal_leftovers"))
                and all(row.get(key) is False for key in ("repeat_verified", "removal_verified"))
                and all(key in row and row[key] is None for key in
                        ("repeat_add", "repeat_list", "cleanup", "post_remove_list")))
        _, payload = document(second, "list")
        require(payload["installations"] == [])

    def positive(self, row, source, identity, index, fixtures):
        state = ACTIVE_SKILLS if index in ACTIVE_SLOTS else PREPARED
        if source.startswith("github:"):
            require(re.fullmatch(r"github:[^\s@]+@[0-9a-f]{40}", source))
        if index in ACTIVE_SLOTS:
            require(identity == ACTIVE_SLOTS[index] and source.startswith("github:"))
        require((row.get("status") == "prepared-lifecycle-passed"
                 or (row.get("status") == "passed" and self.native_runtime is not None and index in ACTIVE_SLOTS))
                and row.get("prepared_components_only") is True
                and all(row.get(key) is True for key in (
                    "listed_identity", "repeat_verified", "repeat_native_unchanged", "repeat_lock_unchanged",
                    "removal_verified", "post_list_schema_valid"))
                and row.get("post_removal_leftovers") == [])
        frames = [image for _, _, image in self.observations]
        try:
            for phase, before, after in (
                    ("add/list", frames[0], frames[1]),
                    ("add/repeat-add", frames[0], frames[2]),
                    ("add/repeat-list", frames[0], frames[3]),
                    ("remove/list", frames[4], frames[5])):
                require_same_image(before, after, phase)
        except ValueError as error:
            comparisons = []
            for phase, expected, observed in (
                    ("after-list", frames[0], frames[1]), ("after-repeat-add", frames[0], frames[2]),
                    ("after-repeat-list", frames[0], frames[3]), ("after-remove", self.initial, frames[4]),
                    ("after-remove-list", self.initial, frames[5])):
                changes = frame_changes(expected, observed)
                for entry in changes["entries"]:
                    entry.pop("relative_key", None)
                    entry.pop("relative_key_truncated", None)
                comparisons.append({"phase": phase, **changes})
            error.windsurf_installation_diagnostics = {
                "native_roots": ["HOME/.windsurf", "HOME/.devin", "HOME/.config/devin"],
                "comparisons": comparisons,
            }
            raise
        documents = [document(outcome, operation) for (_, outcome, _), operation in
                     zip(self.observations, ("add", "list", "add", "list", "remove", "list"))]
        prior = None
        for position in (1, 3):
            raw, payload = documents[position]
            entries = payload["installations"]
            matched = self.audit["matching_installations"](
                raw, identity, "windsurf", "plugin", source, fixtures["signed_release_pins"])
            require(type(entries) is list and len(entries) == 1 and matched == entries)
            node = entries[0]
            clients = node.get("clients")
            require(type(clients) is list and len(clients) == 1 and type(clients[0]) is dict
                    and clients[0].get("client_id") == "windsurf" and clients[0].get("scope") == "user"
                    and all(clients[0].get(key) == value for key, value in state.items()))
            require(type(node.get("installation_id")) is str and bool(node["installation_id"])
                    and node.get("mixed_version") is False)
            require(prior is None or node == prior)
            prior = node
        installation = prior["installation_id"]
        self.saved_lifecycle(prior, source, identity, state, fixtures)
        require(row.get("matched_installation_ids") == row.get("repeat_installation_ids") == [installation]
                and documents[5][1]["installations"] == [])
        for position, action, operation in ((0, "install", "install"), (2, "unchanged", "install"),
                                            (4, "remove", "remove")):
            payload = documents[position][1]
            actions, results = payload.get("actions"), payload.get("results")
            require(type(actions) is list and len(actions) == 1 and type(actions[0]) is dict
                    and actions[0].get("alias") == identity and actions[0].get("client_id") == "windsurf"
                    and actions[0].get("action") == action and type(results) is list)
            if position == 0:
                components = actions[0].get("component_support")
                require(type(components) is list and bool(components)
                        and all(type(component) is dict and component.get("support") == "prepared"
                                for component in components))
            if position != 0 or "installation_id" in actions[0]:
                require(actions[0].get("installation_id") == installation)
            if position in (0, 2):
                revision = prior["clients"][0]["package_revision"]
                require(actions[0].get("package_digest") == revision.get("tree_digest")
                        and actions[0].get("revision") == revision.get("resolved_revision"))
            if position == 2 and not results:
                continue
            require(len(results) == 1 and type(results[0]) is dict
                    and results[0].get("client_id") == "windsurf"
                    and results[0].get("installation_id") == installation
                    and results[0].get("operation") == operation
                    and results[0].get("outcome") in (("completed", "unchanged") if position == 2 else ("completed",)))
        components = documents[0][1]["actions"][0]["component_support"]
        require(all(component.get("kind") in ("skill", "mcp_server") for component in components))
        kinds = {component["kind"] for component in components}
        self.terminal_image, self.retained_infrastructure = self._expected_removed_image(
            frames, mcp="mcp_server" in kinds, skills="skill" in kinds)
        # Retained paths need both the held native writer check and their
        # original, stable authority identity through every public call.
        if "mcp_document" in self.retained_infrastructure:
            require(self.removed_empty_native_coordination(frames[4]))
        self.retained_empty_native_coordination = bool(self.retained_infrastructure)
        return dict(state)

    def saved_lifecycle(self, public, source, identity, public_state, fixtures):
        """Join saved source, public binding, lock and managed receipt authorities.

        This follows Prepared Kiro's source/public/physical ownership joins and
        uses the existing Windsurf receipt digest and finite custody readers.
        Prepared components may legitimately leave the native profile unchanged.
        """
        first, expected = None, None
        for saved in self.saved_observations[:4]:
            docs = saved["documents"]
            require(set(docs) == {"uap.toml", "uap.lock", "state-v2.json"})
            manifest, locked, state = (docs[key] for key in ("uap.toml", "uap.lock", "state-v2.json"))
            require(type(manifest.get("schema_version")) is int and manifest["schema_version"] == 1
                    and set(manifest.get("dependencies", {})) == {identity}
                    and manifest.get("dev-dependencies", {}) == {}
                    and manifest.get("targets") == {"windsurf": [identity]}
                    and type(locked.get("lock_version")) is int and locked["lock_version"] == 1
                    and type(locked.get("packages")) is list and len(locked["packages"]) == 1
                    and len(state["installations"]) == 1)
            record = state["installations"][0]
            require(record.get("installation_id") == public["installation_id"]
                    and record.get("declared_name") == identity and len(record["clients"]) == 1)
            key, binding = next(iter(record["clients"].items()))
            physical = identity[:51].rstrip(".-") + "-" + hashlib.sha256(public["installation_id"].encode()).hexdigest()[:12]
            relative = "managed/clients/windsurf/" + physical
            locator = str(self.data / relative)
            require(key == binding.get("client_binding_id") == "client_" + hashlib.sha256(
                    "\0".join((public["installation_id"], "windsurf", "user", locator)).encode()).hexdigest()[:24]
                    and binding.get("client_id") == "windsurf" and binding.get("scope") == "user"
                    and binding.get("physical_artifact_id") == physical and binding.get("target_locator") == locator
                    and binding.get("native_profile_root") == str(self.global_root)
                    and all(binding.get(k) == v for k, v in public_state.items()))
            revision, origin, package = binding.get("package_revision"), record.get("source"), record.get("package")
            require(type(revision) is dict and type(origin) is dict and type(package) is dict
                    and all(revision.get(k) == v for k, v in public["clients"][0]["package_revision"].items())
                    and all(type(revision.get(k)) is str and re.fullmatch(r"sha256:[0-9a-f]{64}", revision[k])
                            for k in ("tree_digest", "manifest_digest")))
            pin = locked["packages"][0]
            require(type(pin) is dict and pin.get("alias") == identity and pin.get("targets") == ["windsurf"]
                    and pin.get("digest_algorithm") == "agentplugins-tree-sha256-v1"
                    and all(pin.get(k) == revision[k] for k in ("tree_digest", "manifest_digest"))
                    and origin.get("resolved_revision") == revision.get("resolved_revision")
                    and origin.get("tree_digest") == revision["tree_digest"]
                    and package.get("manifest_digest") == revision["manifest_digest"])
            dependency = manifest["dependencies"][identity]
            if source.startswith("github:"):
                repo, commit = source[7:].rsplit("@", 1)
                require(type(dependency) is dict and dependency == {
                        "git": "https://github.com/" + repo.lower() + ".git", "commit": commit}
                        and revision.get("resolved_revision") == commit
                        and pin.get("source", {}).get("git") == {"repository": repo.lower(),
                            "requested": {"kind": "commit", "value": commit}, "commit": commit})
                subpath = ""
                source_key, selector = "github:" + repo.lower() + ":", {"kind": "commit", "value": commit}
            else:
                release = fixtures["signed_release_pins"][source]
                repo, subpath = release["repository"], release["path"]
                source_key = "directory:" + release["distribution"].lower()
                selector = {"kind": "release", "value": str(release["sequence"])}
                registry = pin.get("source", {}).get("registry")
                require(type(dependency) is dict and dependency.get("registry") == "uap"
                        and dependency.get("distribution") == release["distribution"]
                        and str(dependency.get("release")) == str(release["sequence"])
                        and type(registry) is dict and registry.get("distribution_id") == release["distribution"]
                        and registry.get("release_sequence") == release["sequence"]
                        and registry.get("repository", "").lower() == repo.lower()
                        and registry.get("path", "") == subpath and registry.get("commit") == release["revision"])
            intent = {"schema_version": 1, "dependencies": [{"alias": identity, "source_key": source_key,
                      "selector": selector}], "targets": [{"id": "windsurf", "aliases": [identity]}]}
            require(pin.get("source_key") == source_key
                    and locked.get("manifest_digest") == "sha256:" + sha(compact(intent)))
            require(origin.get("repository", "").lower() == repo.lower()
                    and origin.get("package_subpath", "") == subpath)
            independent = independent_source_pin(source, fixtures)
            require(all(revision.get(key) == independent[key] for key in ("tree_digest", "manifest_digest")))
            source_relative = "store/packages/agentplugins-tree-sha256-v1/" + independent["tree_digest"][7:] + "/package"
            source_observation = saved["sources"][source_relative]
            require(source_observation["digest"] == independent["tree_digest"])
            if expected is None:
                expected = self.expected_projection(source_relative, source_observation, independent, record, binding)
                self.source_projection = {"tree_digest": independent["tree_digest"],
                    "manifest_digest": independent["manifest_digest"], "delivery_digest": expected}
            require(saved["deliveries"][relative]["digest"] == expected)
            objects = binding.get("native_objects")
            require(type(objects) is list and all(type(obj) is dict for obj in objects))
            receipts = [obj for obj in objects if obj.get("kind") == "managed_package_directory"]
            require(len(receipts) == 1 and receipts[0] == {"object_id": "package:windsurf:" + physical,
                    "kind": "managed_package_directory", "logical_name": identity, "path": locator,
                    "managed_digest": saved["deliveries"][relative]["digest"], "protection_class": "managed"}
                    and len(saved["deliveries"][relative]["image"]) > 1)
            require(saved["runtime_authority"] == self.saved_observations[0]["runtime_authority"])
            agreement = (record, saved["blobs"]["uap.toml"], saved["blobs"]["uap.lock"], saved["deliveries"], saved["sources"])
            require(first is None or agreement == first)
            first = agreement
        for saved in self.saved_observations[4:]:
            require(saved["runtime_authority"] == self.saved_observations[0]["runtime_authority"])
            intent = self.saved_intent(saved)
            require(intent["dependencies"] == intent["dev-dependencies"] == intent["targets"] == {}
                    and intent["packages"] == [] and saved["deliveries"] == {}
                    and relative not in saved["managed"]
                    and all(record.get("clients") == {} for record in intent["installations"])
                    and not any(value[0] == "file" for value in saved["managed"].values()))

    def expected_projection(self, relative, observed, pin, record, binding):
        """Independently pinned source bytes → canonical projection → delivery."""
        require(verify_projection_producer(self.cli) == self.producer)
        data_path = ""
        receipt_id = binding.get("data_receipt_id")
        if receipt_id:
            receipts = record.get("data_receipts")
            require(type(receipts) is dict and type(receipts.get(receipt_id)) is dict)
            receipt = receipts[receipt_id]
            data_path = str(self.data / "plugin-data" / binding["physical_artifact_id"])
            require(receipt.get("data_receipt_id") == receipt_id
                    and receipt.get("physical_backend_id") == binding["physical_artifact_id"]
                    and receipt.get("scope") == "user" and receipt.get("state") == "owned"
                    and receipt.get("locator") == data_path
                    and receipt.get("ownership_digest") == "sha256:" + sha(
                        ("agentplugins-plugin-data-v1\0" + receipt_id + "\0" + data_path).encode()))
        fd = os.open(self.case, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        try:
            require(self.directory_identity(os.fstat(fd)) == self.anchors[self.case])
            with member(fd, "uap-data/" + relative) as source_fd:
                require(frame(source_fd) == observed["image"]
                        and immutable_source_digest(source_fd, observed["image"]) == pin["tree_digest"])
                with TemporaryDirectory(prefix="windsurf-expected-projection-") as temporary:
                    result = canonical_projection({"source_root": str(self.data / relative),
                        "tree_digest": pin["tree_digest"], "manifest_digest": pin["manifest_digest"],
                        "physical_artifact_id": binding["physical_artifact_id"], "active_path": binding["target_locator"],
                        "data_path": data_path, "cli": self.cli, "cli_version": CLI_VERSION.removeprefix("agentplugins "),
                        "work_root": str(Path(temporary).resolve())},
                        self.producer["revision"] if self.producer is not None else None)
                require(frame(source_fd) == observed["image"]
                        and result["tree_digest"] == pin["tree_digest"]
                        and result["manifest_digest"] == pin["manifest_digest"])
        finally:
            os.close(fd)
        self.guard()
        require(verify_projection_producer(self.cli) == self.producer)
        return result["delivery_digest"]


def summary(report, fixtures, *, fixture_sha256):
    """Accept installation coverage and retain the existing four-case native gate."""
    result = {"installation_passed": 0, "installation_prepared_lifecycle_passed": 0,
              "qualification_scope": "owned Devin Global installation lifecycle and exact refusals",
              "installation_active_skill_lifecycle_passed": 0,
              "installation_verified_negative_coverage": 0, "installation_acceptance_passed": False,
              "retained_native_acceptance_passed": False,
              "native_runtime_qualified": False, "authentication_qualified": False,
              "model_qualified": False, "mcp_tools_qualified": False}
    try:
        require((report.get("target"), report.get("kind"), report.get("platform"), report.get("architecture"))
                == ("windsurf", "plugin", "linux", "x86_64")
                and report.get("cli_version") == {"status": "reported", "value": CLI_VERSION, "exit_code": 0}
                and report.get("client_version") == {"status": "reported", "value": CLIENT_VERSION, "exit_code": 0}
                and type(report.get("client_binary")) is str
                and Path(report["client_binary"]).is_absolute() and Path(report["client_binary"]).name == "devin"
                and type(report.get("cli")) is str and Path(report["cli"]).is_absolute()
                and re.fullmatch(r"[0-9a-f]{64}", fixture_sha256)
                and report.get("fixture_sha256") == fixture_sha256
                and report.get("source_selection_complete") is True and report.get("full_coverage") is True
                and type(report.get("fixture_count")) is int and report["fixture_count"] == 30
                and type(report.get("cases_attempted")) is int and report["cases_attempted"] == 30
                and type(report.get("passed")) is int and 0 <= report["passed"] <= len(ACTIVE_SLOTS)
                and type(report["cli_version"].get("exit_code")) is int
                and type(report["client_version"].get("exit_code")) is int
                and type(report.get("results")) is list
                and len(report["results"]) == 30)
        preserved_native = 0
        for index, row in enumerate(report["results"]):
            require(type(row) is dict)
            source, identity = selected(fixtures, index)
            proof = row.get("windsurf_installation_audit")
            negative = index == 13 or index in COMPONENT_REFUSALS
            require(type(row.get("index")) is int and row["index"] == index
                    and (row.get("target"), row.get("kind"), row.get("source"), row.get("identity"))
                    == ("windsurf", "plugin", source, identity)
                    and type(proof) is dict and type(proof.get("fixture_index")) is int
                    and proof["fixture_index"] == index
                    and (proof.get("source"), proof.get("identity"), proof.get("canonical_client_id"), proof.get("scope"))
                    == (source, identity, "windsurf", "user")
                    and proof.get("native_runtime_qualified") is False
                    and proof.get("writer_closure_claimed") is False
                    and proof.get("native_roots") == [".windsurf", ".devin", ".config/devin"]
                    and proof.get("project_unchanged") is True
                    and proof.get("original_native_preserved") is True
                    and proof.get("status") == ("verified-installation-negative" if negative else "verified-installation-lifecycle")
                    and proof.get("public_state") == (None if negative else ACTIVE_SKILLS if index in ACTIVE_SLOTS else PREPARED)
                    and type(proof.get("observations")) is list
                    and len(proof["observations"]) == (2 if negative else 6))
            require(row.get("sentinels_preserved") is True
                    and row.get("native_residuals") == []
                    and all(row.get(key) is False for key in (
                        "failed_install_registered", "failed_install_residue", "post_remove_identity_registered")))
            initial = proof.get("initial_native_image")
            terminal = proof.get("terminal_native_image")
            roles = proof.get("retained_infrastructure")
            mode = proof.get("terminal_mode")
            require(type(roles) is list and roles in (
                [], ["mcp_document", "writer_lock"], ["skills_parent"],
                ["mcp_document", "skills_parent", "writer_lock"]))
            frames = [entry.get("native_image") for entry in proof["observations"] if type(entry) is dict]
            require(len(frames) == (2 if negative else 6))
            for frame in [initial, terminal, *frames]:
                require(type(frame) is dict and set(frame) == {"sha256", "entries"}
                        and type(frame["sha256"]) is str and re.fullmatch(r"[0-9a-f]{64}", frame["sha256"])
                        and type(frame["entries"]) is int and frame["entries"] > 0)
            require(type(proof.get("initial_saved_authority_sha256")) is str
                    and re.fullmatch(r"[0-9a-f]{64}", proof["initial_saved_authority_sha256"]))
            require(all(type(entry.get("saved_authority_sha256")) is str
                        and re.fullmatch(r"[0-9a-f]{64}", entry["saved_authority_sha256"])
                        for entry in proof["observations"]))
            require(all(type(entry.get("json_sha256")) is str
                        and re.fullmatch(r"[0-9a-f]{64}", entry["json_sha256"])
                        for entry in proof["observations"]))
            if roles:
                require(mode == "retained-neutral-infrastructure" and not negative
                        and terminal != initial
                        and terminal["entries"] == initial["entries"] + 2 * len(roles)
                        and (index not in ACTIVE_SLOTS or roles == ["skills_parent"]))
            else:
                require(mode == "restored-original-image" and terminal == initial)
            if row.get("status") == "passed":
                require(proof.get("preserved_native_proof") == native_reference(row, source, identity, index)
                        and row["windsurf_global_native_proof"]["binary"]["path"] == report["client_binary"])
                preserved_native += 1
            else:
                require("preserved_native_proof" not in proof)
            if negative:
                require(frames == [initial, initial]
                        and proof.get("native_post_remove_state") == "pristine"
                        and row.get("status") == ("documented-or-source-unsupported" if index == 13 else "install-failed"))
            else:
                retained = proof.get("native_post_remove_state") == "empty-writer-coordination"
                require(proof.get("native_post_remove_state") in ("pristine", "empty-writer-coordination")
                        and retained is bool(roles)
                        and frames[0] == frames[1] == frames[2] == frames[3]
                        and frames[4] == frames[5] == terminal
                        and row.get("status") in ("prepared-lifecycle-passed", "passed")
                        and row.get("prepared_components_only") is True
                        and all(row.get(key) is True for key in (
                            "listed_identity", "repeat_verified", "repeat_native_unchanged", "repeat_lock_unchanged",
                            "removal_verified", "post_list_schema_valid"))
                        and row.get("post_removal_leftovers") == [])
                projection = proof.get("source_projection")
                independent = independent_source_pin(source, fixtures)
                require(type(projection) is dict and set(projection) == {"tree_digest", "manifest_digest", "delivery_digest"}
                        and all(projection.get(key) == independent[key] for key in ("tree_digest", "manifest_digest"))
                        and type(projection.get("delivery_digest")) is str
                        and re.fullmatch(r"sha256:[0-9a-f]{64}", projection["delivery_digest"]))
            result["installation_verified_negative_coverage" if negative else "installation_passed"] += 1
            if not negative:
                result["installation_active_skill_lifecycle_passed" if index in ACTIVE_SLOTS
                       else "installation_prepared_lifecycle_passed"] += 1
        require(report["passed"] == preserved_native)
        require((result["installation_passed"], result["installation_verified_negative_coverage"],
                 result["installation_prepared_lifecycle_passed"], result["installation_active_skill_lifecycle_passed"])
                == (25, 5, 21, 4))
        result["installation_acceptance_passed"] = True
        result["retained_native_acceptance_passed"] = preserved_native == len(ACTIVE_SLOTS)
    except (ValueError, TypeError, KeyError, RecursionError):
        pass
    return result
