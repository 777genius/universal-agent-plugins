"""Finite observation of immutable npm 0.1.95 Skills and archived plugins.

Never supplies modern readiness or changes a row's native status.
"""
import base64
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import time

ACCEPTANCE = "published-0.1.95-codex-skills"
PLUGIN_ACCEPTANCE = "published-0.1.95-codex-plugins-archived-feed"
ARCHIVE_COMMIT = "41e6bbe49348eca5b491036e8630c9ddaacc7424"
ARCHIVE_ORIGIN = ("https://raw.githubusercontent.com/777genius/universal-agent-plugins-registry/"
                  + ARCHIVE_COMMIT + "/registry/schemas/1/")
# Independently read from this official immutable commit, not generated here.
ARCHIVE_DIGEST = "3b980f468762c48a9ea978b20de98148b81501c2a7b3d4584adcc9fcb0bf7e53"
ARCHIVE_ENVELOPE_DIGEST = "1d4c0affa2929f1bdb984a4a31dcc30ed15fda5563cc281d40ad2855d97d0d12"
ARCHIVE_GENERATED = "2026-11-14T12:42:17Z"
ARCHIVE_EXPIRES = "2026-12-14T12:42:17Z"
ARCHIVE_KEY = "uap-directory-2026-01"
ARCHIVE_SOURCES = {"atlassian": "777genius/atlassian", "context7": "upstash/context7"}
DEFAULT_ORIGIN = "https://777genius.github.io/universal-agent-plugins-registry/registry/schemas/1/"
COMMIT = "ec85ee7d759944b9799e451c96bfc1859f9d5533"
DIGEST = "af5c1891b27b1707c657f677168dd0495ec8b9fe18db1385dbfa448147b6e9b6"
SIZE = 24686754
# Official release-source Git blobs. Bind the code that chooses/launches the asset.
BLOBS = {
    "bin/agentplugins.js": "243418d32a02a95cab266aa10f0fabe8023574c4",
    "lib/bootstrap.js": "c362d6652cc25229e0ffbed6d505545e018c76f8",
    "lib/platform.js": "d20eee0d0711347fc0815e82a10d2e98e5d2f240",
    "lib/verifier.js": "b7690f80936fb3bd6c03f369f8c687c9607c8cd5",
}
NODE_METADATA = r"""
const fs = require('node:fs'), path = require('node:path');
const root = process.argv[1];
const platform = require(path.join(root, 'lib/platform.js'));
const bootstrap = require(path.join(root, 'lib/bootstrap.js'));
const metadata = new Map([[path.join(root, "package.json"), process.argv[2]],
                          [path.join(root, "assets.json"), process.argv[3]]]);
const originalRead = fs.readFileSync;
fs.readFileSync = function(file, ...args) {
  if (metadata.has(String(file))) return metadata.get(String(file));
  return originalRead.call(fs, file, ...args);
};
const info = platform.detectPlatform();
const release = bootstrap.loadRelease(root, info);
const pkg = JSON.parse(fs.readFileSync(path.join(root, 'package.json'), 'utf8'));
const output = JSON.stringify({name:pkg.name, version:pkg.version,
  platform:info.key, tag:release.manifest.tag,
  commit:release.manifest.producer.commit,
  asset:{file:release.asset.file, sha256:release.asset.sha256, size:release.asset.size},
  binary:path.join(platform.cacheRoot(), release.version, info.key, info.binaryName)});
if (Buffer.byteLength(output) > 8192) throw new Error("baseline metadata output exceeds bound");
process.stdout.write(output);
"""


def identity(info):
    return (info.st_dev, info.st_ino, info.st_mode, info.st_size,
            info.st_mtime_ns, info.st_ctime_ns, info.st_nlink, info.st_uid, info.st_gid)


def directory_chain(root, parent):
    """Reject redirected case-cache ancestors; caller rechecks after held read."""
    if not root.is_absolute() or not parent.is_relative_to(root):
        raise ValueError("historical cache escapes disposable HOME")
    result = {}
    for path in (root, *(root / Path(*parent.relative_to(root).parts[:index])
                         for index in range(1, len(parent.relative_to(root).parts) + 1))):
        info = path.lstat()
        if not stat.S_ISDIR(info.st_mode):
            raise ValueError("historical cache ancestor is not an ordinary directory")
        result[path] = (info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid)
    return result


def regular_bytes(path, limit=65536, *, exact_size=None):
    """Finite no-follow read; final named comparison keeps the descriptor held."""
    before = path.lstat()
    if (not stat.S_ISREG(before.st_mode) or before.st_size > limit
            or (exact_size is not None and before.st_size != exact_size)):
        raise ValueError("historical baseline regular file size mismatch")
    descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(descriptor, "rb") as stream:
        if identity(before) != identity(os.fstat(stream.fileno())):
            raise ValueError("historical baseline file changed")
        deadline, chunks, total = time.monotonic() + 5, [], 0
        while True:
            if time.monotonic() > deadline:
                raise ValueError("historical baseline read budget exhausted")
            body = stream.read(min(1 << 20, limit + 1 - total))
            total += len(body)
            if total > limit:
                raise ValueError("historical baseline file exceeds read bound")
            chunks.append(body)
            if not body:
                break
        if (time.monotonic() > deadline
                or identity(before) != identity(os.fstat(stream.fileno()))
                or identity(before) != identity(path.lstat())
                or total != before.st_size):
            raise ValueError("historical baseline file changed")
        return b"".join(chunks)


class PublishedBaseline:
    def __init__(self, launcher, environment, project):
        self.launcher = Path(launcher).resolve(strict=True)
        self.root = self.launcher.parent.parent
        self.environment, self.project = environment, project
        self.home = Path(environment["HOME"])
        if (not self.home.is_absolute() or self.home != Path(project).parent / "home"
                or environment.get("XDG_CACHE_HOME") != str(self.home / ".cache")
                or environment.get("AGENTPLUGINS_CACHE_DIR")):
            raise ValueError("historical baseline requires the disposable case cache")
        self.binary = self.home / ".cache/agentplugins/0.1.95/linux-amd64/agentplugins"
        self.home_identity = directory_chain(self.home, self.home)
        self.commands = 0
        self.proof = None
        self.verify_sources()
        self.metadata = self.read_metadata()

    def verify_sources(self):
        if self.launcher != self.root / "bin/agentplugins.js":
            raise ValueError("historical baseline requires the actual npm launcher")
        if os.path.lexists(self.root / "public-release.json"):
            raise ValueError("historical baseline forbids another launcher route")
        if any(self.environment.get(key) for key in (
                "AGENTPLUGINS_INTERNAL_PROOF_MODE", "AGENTPLUGINS_INTERNAL_PROOF_BINARY",
                "NODE_OPTIONS", "NODE_PATH")):
            raise ValueError("historical baseline forbids bootstrap overrides")
        for relative, expected in BLOBS.items():
            content = regular_bytes(self.root / relative)
            actual = hashlib.sha1(b"blob " + str(len(content)).encode() + b"\0" + content).hexdigest()
            if actual != expected:
                raise ValueError("historical npm launcher source mismatch")

    def read_metadata(self):
        self.verify_sources()
        metadata = {name: regular_bytes(self.root / name) for name in ("package.json", "assets.json")}
        process = subprocess.run(
            ["node", "-e", NODE_METADATA, str(self.root),
             metadata["package.json"].decode("utf-8"), metadata["assets.json"].decode("utf-8")], cwd=self.project,
            env=self.environment, capture_output=True, text=True, timeout=30, check=True)
        if len(process.stdout.encode("utf-8")) > 8192 or process.stderr:
            raise ValueError("historical metadata output mismatch")
        if any(regular_bytes(self.root / name) != body for name, body in metadata.items()):
            raise ValueError("historical npm metadata changed")
        value = json.loads(process.stdout)
        asset = value.get("asset")
        if (value.get("name") != "universal-agent-plugins" or value.get("version") != "0.1.95"
                or value.get("platform") != "linux-amd64"
                or value.get("tag") != "agentplugins-v0.1.95" or value.get("commit") != COMMIT
                or type(asset) is not dict or asset.get("sha256") != DIGEST
                or type(asset.get("size")) is not int or asset["size"] != SIZE
                or asset.get("file") != "agentplugins_0.1.95_linux_amd64"
                or value.get("binary") != str(self.binary)):
            raise ValueError("historical npm baseline provenance mismatch")
        return value

    def after_command(self):
        value = self.read_metadata()
        if value != self.metadata:
            raise ValueError("historical npm resolution changed")
        ancestors = directory_chain(self.home, self.binary.parent)
        if any(ancestors[path] != previous for path, previous in self.home_identity.items()):
            raise ValueError("historical case HOME changed")
        content = regular_bytes(self.binary, SIZE, exact_size=SIZE)
        if directory_chain(self.home, self.binary.parent) != ancestors:
            raise ValueError("historical cache ancestors changed")
        if len(content) != SIZE or hashlib.sha256(content).hexdigest() != DIGEST:
            raise ValueError("historical release binary mismatch")
        self.commands += 1
        self.proof = {"package": "universal-agent-plugins", "version": "0.1.95",
                      "commit": COMMIT, "binary_sha256": DIGEST, "binary_size": SIZE,
                      "launcher_sources_verified": True, "commands_verified": self.commands}
        if getattr(self, "plugin_archive", None) is not None:
            try:
                archive_cache(self, *self.plugin_archive)
            except (OSError, ValueError, KeyError, TypeError) as error:
                raise ArchiveUnavailable(self.plugin_feed) from error


def utc_now():
    return datetime.now(timezone.utc)


def instant(value):
    return datetime.fromisoformat(value.replace("Z", "+00:00"))


def require(value):
    if not value:
        raise ValueError("historical signed Directory observation is unresolved")


def exact(value, expected):
    if type(value) is not type(expected):
        return False
    if type(expected) is dict:
        return value.keys() == expected.keys() and all(exact(value[key], item)
                                                      for key, item in expected.items())
    if type(expected) is list:
        return len(value) == len(expected) and all(exact(left, right)
                                                  for left, right in zip(value, expected))
    return value == expected


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        require(key not in result)
        result[key] = value
    return result


class ArchiveUnavailable(ValueError):
    def __init__(self, observation):
        super().__init__("historical signed Directory observation is unresolved")
        self.observation = observation


def archive_cache(observer, source, pin):
    """Read exact official bytes authenticated by the unchanged original CLI.

    This observer never creates a cache, adds trust, signs bytes or changes time.
    It rechecks the finite cache and its directory authority after every command.
    """
    require(source in ARCHIVE_SOURCES and pin["distribution"] == ARCHIVE_SOURCES[source]
            and observer.environment.get("AGENTPLUGINS_DIRECTORY_ORIGIN") == ARCHIVE_ORIGIN)
    data = Path(observer.environment["AGENTPLUGINS_HOME"])
    require(data == observer.home.parent / "uap-data")
    ancestors = directory_chain(observer.home.parent, data)
    cache = data / "directory-v1-cache.json"
    info = cache.lstat()
    require(stat.S_ISREG(info.st_mode) and info.st_mode & 0o077 == 0
            and info.st_nlink == 1 and info.st_uid == os.geteuid())
    body = regular_bytes(cache, 6 << 20)
    require(identity(info) == identity(cache.lstat()))
    record = json.loads(body, object_pairs_hook=unique_object,
                        parse_constant=lambda _: require(False))
    require(type(record) is dict and set(record) == {"schema_version", "sequence", "snapshot", "envelope"}
            and exact(record["schema_version"], 1) and exact(record["sequence"], 48))
    snapshot = base64.b64decode(record["snapshot"], validate=True)
    envelope = base64.b64decode(record["envelope"], validate=True)
    require(hashlib.sha256(snapshot).hexdigest() == ARCHIVE_DIGEST
            and hashlib.sha256(envelope).hexdigest() == ARCHIVE_ENVELOPE_DIGEST)
    signed, signature = json.loads(snapshot), json.loads(envelope)
    require(exact(signed["sequence"], 48) and exact(signature["sequence"], 48)
            and signature["key_id"] == ARCHIVE_KEY
            and signature["snapshot_digest"] == "sha256:" + ARCHIVE_DIGEST
            and signed["generated_at"] == ARCHIVE_GENERATED and signed["expires_at"] == ARCHIVE_EXPIRES
            and instant(ARCHIVE_GENERATED) <= utc_now() < instant(ARCHIVE_EXPIRES))
    distributions = [row for row in signed["distributions"] if row["id"] == pin["distribution"]]
    require(len(distributions) == 1 and distributions[0]["status"] == "active")
    releases = [row for row in distributions[0]["releases"] if exact(row["sequence"], pin["sequence"])]
    require(len(releases) == 1)
    release = releases[0]
    require(exact(release["package_source"], {"repository": pin["repository"], "path": pin["path"],
                "revision": pin["revision"]}) and release["tree_digest"] == pin["tree_digest"]
            and release["manifest_digest"] == pin["manifest_digest"])
    require(directory_chain(observer.home.parent, data) == ancestors)
    return {"scope": "immutable-official-historical-feed", "origin": ARCHIVE_ORIGIN,
            "ledger_commit": ARCHIVE_COMMIT, "sequence": 48, "key_id": ARCHIVE_KEY,
            "snapshot_sha256": ARCHIVE_DIGEST, "envelope_sha256": ARCHIVE_ENVELOPE_DIGEST,
            "generated_at": ARCHIVE_GENERATED, "expires_at": ARCHIVE_EXPIRES,
            "original_cli_trust_unchanged": True, "release_pin": dict(pin)}


def default_refusal(default, started, finished):
    if type(default.get("stderr_full")) is not str:
        return False
    match = re.fullmatch(
        r"agentplugins: load signed Directory: directory snapshot is expired: "
        r"expires_at=2026-10-30T20:03:50Z now=(\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ)\n",
        default["stderr_full"])
    return (exact(default.get("exit_code"), 1) and default.get("timeout") is False
            and default.get("stdout_full") == "" and default.get("stdout_tail") == ""
            and default.get("stderr_tail") == default["stderr_full"] and default.get("_json") is None
            and match is not None and int(started.timestamp()) <= instant(match[1]).timestamp()
            <= finished.timestamp())


def recorded_default_refusal(value):
    try:
        return (type(value) is dict and value.get("status") == "current-default-compatibility-failed"
                and value.get("origin") == DEFAULT_ORIGIN and value.get("compatibility_passed") is False
                and value.get("native_passed") is False and value.get("complete_output_verified") is True
                and type(value.get("observation")) is dict
                and default_refusal(value["observation"], instant(value["started_at"]), instant(value["ended_at"])))
    except (ValueError, KeyError, TypeError, AttributeError):
        return False


def prepare_plugin_archive(observer, call, source, pin):
    """Record the real default refusal before explicitly selecting an archive."""
    require(source in ARCHIVE_SOURCES and "AGENTPLUGINS_DIRECTORY_ORIGIN" not in observer.environment)
    argv = [str(observer.launcher), "info", source, "--target", "codex", "--format", "json"]
    started = utc_now()
    default = call(argv, observer.environment, observer.project, 80, capture_full=True)
    finished = utc_now()
    observation = {"current_default": {"status": "unexpected-default-outcome", "origin": DEFAULT_ORIGIN,
        "compatibility_passed": None, "native_passed": False,
        "started_at": started.isoformat(), "ended_at": finished.isoformat(),
        "observation": bounded_command(default)}}
    observer.plugin_feed = observation
    try:
        require(default_refusal(default, started, finished))
        # Only this exact fixed diagnostic is safe to persist completely.
        observation["current_default"].update(status="current-default-compatibility-failed",
            compatibility_passed=False, complete_output_verified=True,
            observation={**bounded_command(default), "stdout_full": "", "stderr_full": default["stderr_full"]})
        observer.environment["AGENTPLUGINS_DIRECTORY_ORIGIN"] = ARCHIVE_ORIGIN
        archived = call(argv, observer.environment, observer.project, 80, capture_full=True)
        document = archived.pop("_json", None)
        observation["archive_info"] = bounded_command(archived)
        require(exact(archived.get("exit_code"), 0) and archived.get("timeout") is False
                and archived.get("stderr_full") == "" and archived.get("stderr_tail") == ""
                and type(archived.get("stdout_full")) is str and bool(archived["stdout_full"])
                and type(document) is dict
                and exact(document.get("schema_version"), 1) and document.get("command") == "info"
                and document.get("result") == "success")
        data, expected = document["data"], {key: pin[field] for key, field in (
            ("repository", "repository"), ("package_path", "path"), ("resolved_revision", "revision"),
            ("release_sequence", "sequence"), ("tree_digest", "tree_digest"), ("manifest_digest", "manifest_digest"))}
        require(exact(data["snapshot_sequence"], 48) and data["snapshot_digest"] == "sha256:" + ARCHIVE_DIGEST
                and data["selected_distribution"] == pin["distribution"]
                and exact({key: data["selected_release"][key] for key in expected}, expected))
        observation["archive_info"]["complete_output_verified"] = True
        observation["archive"] = archive_cache(observer, source, pin)
        observer.plugin_archive = source, dict(pin)
        return observation
    except (ValueError, OSError, KeyError, TypeError, IndexError) as error:
        raise ArchiveUnavailable(observation) from error


def bounded_command(command):
    """Never export arbitrary complete output from failed/unexpected probes."""
    return {"exit_code": command.get("exit_code"), "timeout": command.get("timeout"),
            **{key: value[-1800:] if type(value := command.get(key)) is str else None
               for key in ("stdout_tail", "stderr_tail")}}


def plugin_summary(report, fixtures):
    """Qualify only the explicitly archived lifecycle; current native PASS stays zero."""
    result = {"historical_plugin_observation_complete": False,
              "qualification_scope": "immutable historical npm and archived signed feed",
              "current_default_compatibility_passed": None, "current_default_native_passed": 0,
              "current_default_refusals_observed": 0, "archived_plugin_lifecycles_observed": 0,
              "native_discovery_qualified": False, "native_activation_qualified": False,
              "model_execution_qualified": False, "authenticated_execution_qualified": False}
    rows = report.get("results")
    if type(rows) is list:
        result["current_default_refusals_observed"] = sum(
            type(row) is dict and type(row.get("historical_plugin_feed")) is dict
            and recorded_default_refusal(row["historical_plugin_feed"].get("current_default")) for row in rows)
    if result["current_default_refusals_observed"] == 2:
        result["current_default_compatibility_passed"] = False
    if (report.get("acceptance") != PLUGIN_ACCEPTANCE
            or (report.get("target"), report.get("kind"), report.get("platform"), report.get("architecture"))
               != ("codex", "plugin", "linux", "x86_64")
            or not exact(report.get("cli_version"), {"status": "reported", "value": "agentplugins 0.1.95", "exit_code": 0})
            or not exact(report.get("client_version"), {"status": "reported", "value": "codex-cli 0.153.4", "exit_code": 0})
            or not exact(report.get("cases_attempted"), 2) or not exact(report.get("passed"), 0)
            or report.get("full_coverage") is not False or report.get("source_selection_complete") is not False
            or not exact(report.get("verified_negative_coverage"), 0)
            or fixtures.get("plugin_aliases", [])[:2] != list(ARCHIVE_SOURCES)
            or type(rows) is not list or len(rows) != 2):
        return result
    for index, (row, source) in enumerate(zip(rows, ARCHIVE_SOURCES)):
        if type(row) is not dict:
            return result
        feed, proof = row.get("historical_plugin_feed"), row.get("historical_npm_proof")
        if (not exact(row.get("index"), index) or row.get("source") != source or row.get("identity") != source
                or (row.get("target"), row.get("kind")) != ("codex", "plugin")
                or row.get("status") != "historical-archive-lifecycle-observed"
                or row.get("historical_archive_status") != "passed" or type(feed) is not dict
                or not exact(proof, {"package": "universal-agent-plugins", "version": "0.1.95", "commit": COMMIT,
                    "binary_sha256": DIGEST, "binary_size": SIZE, "launcher_sources_verified": True,
                    "commands_verified": 8}) or not recorded_default_refusal(feed.get("current_default"))):
            return result
        expected_archive = {"scope": "immutable-official-historical-feed", "origin": ARCHIVE_ORIGIN,
            "ledger_commit": ARCHIVE_COMMIT, "sequence": 48, "key_id": ARCHIVE_KEY,
            "snapshot_sha256": ARCHIVE_DIGEST, "envelope_sha256": ARCHIVE_ENVELOPE_DIGEST,
            "generated_at": ARCHIVE_GENERATED, "expires_at": ARCHIVE_EXPIRES,
            "original_cli_trust_unchanged": True, "release_pin": fixtures["signed_release_pins"][source]}
        if (not exact(feed.get("archive"), expected_archive)
                or not instant(ARCHIVE_GENERATED) <= utc_now() < instant(ARCHIVE_EXPIRES)):
            return result
        for phase in ("archive_info",):
            command = feed.get(phase)
            if (type(command) is not dict or not exact(command.get("exit_code"), 0)
                    or command.get("timeout") is not False or command.get("stderr_tail") != ""
                    or command.get("complete_output_verified") is not True):
                return result
        if (any(row.get(key) is not True for key in ("repeat_verified", "repeat_native_unchanged",
                "repeat_lock_unchanged", "removal_verified", "sentinels_preserved", "listed_identity", "post_list_schema_valid"))
                or any(row.get(key) is not False for key in ("failed_install_registered", "failed_install_residue",
                    "post_remove_identity_registered")) or row.get("native_residuals") != []):
            return result
        ids = row.get("matched_installation_ids")
        if (type(ids) is not list or len(ids) != 1 or type(ids[0]) is not str or not ids[0]
                or row.get("repeat_installation_ids") != ids):
            return result
        for phase in ("install", "list", "repeat_add", "repeat_list", "cleanup", "post_remove_list"):
            command = row.get(phase)
            if (type(command) is not dict or not exact(command.get("exit_code"), 0)
                    or command.get("timeout") is not False):
                return result
        native = row.get("native_uninstall")
        if type(native) is not dict or any(native.get(key) is not True for key in (
                "completed", "native_plugin_absent", "native_marketplace_absent")):
            return result
        sqlite = row.get("codex_plugin_sqlite_proof")
        if (type(sqlite) is not dict or sqlite.get("native_version") != "0.153.4"
                or sqlite.get("runtime_semantics_unchanged") is not True
                or type(sqlite.get("observations")) is not list or not sqlite["observations"]):
            return result
        result["archived_plugin_lifecycles_observed"] += 1
    result["historical_plugin_observation_complete"] = result["archived_plugin_lifecycles_observed"] == 2
    return result


def historical_add(value, matches, fixture, skill_path, repeated=False):
    # Old renderSkillsResult always emits these three fields; [] is not null.
    if (type(value) is not dict or set(value) != {"status", "items", "repairInstructions"}
            or value["status"] != "succeeded" or value["repairInstructions"] != []
            or type(value["repairInstructions"]) is not list
            or type(value["items"]) is not list or len(value["items"]) != 1
            or type(matches) is not list or len(matches) != 1):
        return False
    item, installed = value["items"][0], matches[0]
    if type(item) is not dict or set(item) != {
            "alias", "candidateId", "destinationPath", "message", "status", "targetId"}:
        return False
    if (type(installed) is not dict or type(installed.get("installation_id")) is not str
            or not installed["installation_id"] or not skill_path):
        return False
    # ec85ee7: skillsource discovery/normalize, environment.SourceKey and
    # domain.ComputePhysicalArtifactID distinguish the selected native Skill
    # from its deterministic candidate, synthetic package and managed path.
    path = "skills/" + fixture["folder"]
    candidate = "sha256:" + hashlib.sha256(
        "\0".join((fixture["repo"], fixture["ref"], path)).encode()).hexdigest()
    source_key = "github-skill:" + fixture["repo"].lower() + ":" + fixture["name"] + ":" + path
    bundle = "skill-" + hashlib.sha256(source_key.encode()).hexdigest()[:56]
    native = Path(skill_path)
    if (not native.is_absolute()
            or native.parts[-4:] != (".agents", "skills", fixture["name"], "SKILL.md")):
        return False
    leaf = bundle[:51].rstrip(".-") + "-" + hashlib.sha256(
        installed["installation_id"].encode()).hexdigest()[:12]
    destination = native.parents[3] / ".uap/managed/clients/codex" / leaf
    if (any(type(item[key]) is not str for key in item)
            or item["alias"] != fixture["name"] or item["targetId"] != "codex"
            or item["status"] != ("unchanged" if repeated else "succeeded")
            or item["candidateId"] != candidate
            or item["destinationPath"] != str(destination)
            or installed.get("name") != bundle
            or installed.get("source") != fixture["repo"] + "/skills/" + fixture["folder"]):
        return False
    clients = installed.get("clients")
    expected = {"client_id": "codex", "scope": "project",
                "materialization": "materialized", "activation": "prepared",
                "verification": "installation_verified",
                "authentication": "not_checked", "policy": "allowed"}
    return (type(clients) is list and len(clients) == 1 and type(clients[0]) is dict
            and all(clients[0].get(key) == expected_value for key, expected_value in expected.items())
            and type(clients[0].get("package_revision")) is dict
            and clients[0]["package_revision"].get("resolved_revision") == fixture["ref"])


def summary(report, fixtures):
    result = {"historical_observation_complete": False, "historical_lifecycle_observed": 0}
    rows = report.get("results")
    if (report.get("acceptance") != ACCEPTANCE or report.get("target") != "codex"
            or report.get("kind") != "skill" or report.get("platform") != "linux"
            or report.get("architecture") != "x86_64" or type(report.get("cases_attempted")) is not int
            or report["cases_attempted"] != 2 or type(report.get("passed")) is not int
            or report["passed"] != 0 or report.get("full_coverage") is not False
            or report.get("source_selection_complete") is not False
            or report.get("cli_version") != {"status": "reported", "value": "agentplugins 0.1.95", "exit_code": 0}
            or report.get("client_version") != {"status": "reported", "value": "codex-cli 0.153.4", "exit_code": 0}
            or type(rows) is not list or len(rows) != 2):
        return result
    for index, row in enumerate(rows):
        fixture = fixtures["skills"][index]
        proof = row.get("historical_npm_proof")
        if (type(row.get("index")) is not int or row["index"] != index or row.get("source") != fixture["source"]
                or row.get("identity") != fixture["name"] or row.get("target") != "codex"
                or row.get("kind") != "skill" or row.get("status") != "incomplete-native-proof"
                or row.get("codex_public_add_ready") is not False
                or row.get("codex_public_repeat_ready") is not False
                or row.get("repeat_verified") is not False
                or proof != {"package": "universal-agent-plugins", "version": "0.1.95",
                             "commit": COMMIT, "binary_sha256": DIGEST, "binary_size": SIZE,
                             "launcher_sources_verified": True, "commands_verified": 6}):
            return result
        if not all(row.get(key) is True for key in (
                "historical_add_verified", "historical_repeat_add_verified",
                "historical_repeat_lifecycle_verified", "listed_identity",
                "lock_written", "lock_contains_identity", "lock_exact_source_pinned",
                "skill_bytes_match", "removal_verified", "sentinels_preserved",
                "post_list_schema_valid", "repeat_lock_unchanged")):
            return result
        if (any(row.get(key) is not False for key in (
                "failed_install_registered", "failed_install_residue",
                "post_remove_identity_registered", "post_lock_pinned"))
                or row.get("native_residuals") != [] or row.get("post_removal_skill") is not None
                or row.get("post_removal_leftovers") != [] or row.get("post_skill_tree_delta") != []
                or not row.get("matched_installation_ids")
                or row.get("matched_installation_ids") != row.get("repeat_installation_ids")
                or not (row.get("repeat_native_unchanged") is True
                        or row.get("repeat_native_runtime_only") is True)):
            return result
        for command in ("install", "list", "repeat_add", "repeat_list", "cleanup", "post_remove_list"):
            outcome = row.get(command)
            if (type(outcome) is not dict or type(outcome.get("exit_code")) is not int
                    or outcome["exit_code"] != 0 or outcome.get("timeout") is not False):
                return result
        native = row.get("codex_runtime_proof")
        if type(native) is not dict or native.get("native_version") != "0.153.4":
            return result
        observations = native.get("observations")
        phases = [("before-install", False), ("installed", True),
                  ("repeat-installed", True), ("removed", False)]
        if type(observations) is not list or len(observations) != len(phases):
            return result
        for observation, (phase, present) in zip(observations, phases):
            if (type(observation) is not dict or observation.get("phase") != phase
                    or observation.get("selected_skill_present") is not present
                    or type(observation.get("native_exit_code")) is not int
                    or observation["native_exit_code"] != 0
                    or any(observation.get(key) is not True for key in (
                        "foreign_skill_present", "runtime_integrity_verified", "runtime_semantics_unchanged"))
                    or type(observation.get("runtime_files_fully_read")) is not dict
                    or type(observation.get("temporary_git_files_fully_read")) is not dict):
                return result
        result["historical_lifecycle_observed"] += 1
    result["historical_observation_complete"] = result["historical_lifecycle_observed"] == 2
    return result
