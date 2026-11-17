"""Finite observation of the immutable npm 0.1.95 Codex Skill baseline.

Never supplies modern readiness or changes a row's native status.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess

ACCEPTANCE = "published-0.1.95-codex-skills"
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
const info = platform.detectPlatform();
const release = bootstrap.loadRelease(root, info);
const pkg = JSON.parse(fs.readFileSync(path.join(root, 'package.json'), 'utf8'));
console.log(JSON.stringify({name:pkg.name, version:pkg.version,
  platform:info.key, tag:release.manifest.tag,
  commit:release.manifest.producer.commit, asset:release.asset,
  binary:path.join(platform.cacheRoot(), release.version, info.key, info.binaryName)}));
"""


def regular_bytes(path):
    before = path.lstat()
    if not stat.S_ISREG(before.st_mode):
        raise ValueError("historical baseline requires a regular file")
    with path.open("rb") as stream:
        if not os.path.samestat(before, os.fstat(stream.fileno())):
            raise ValueError("historical baseline file changed")
        data = stream.read()
    if not os.path.samestat(before, path.lstat()):
        raise ValueError("historical baseline file changed")
    return data


class PublishedBaseline:
    def __init__(self, launcher, environment, project):
        self.launcher = Path(launcher).resolve(strict=True)
        self.root = self.launcher.parent.parent
        self.environment, self.project = environment, project
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
        process = subprocess.run(
            ["node", "-e", NODE_METADATA, str(self.root)], cwd=self.project,
            env=self.environment, capture_output=True, text=True, timeout=30, check=True)
        value = json.loads(process.stdout)
        asset = value.get("asset")
        if (value.get("name") != "universal-agent-plugins" or value.get("version") != "0.1.95"
                or value.get("platform") != "linux-amd64"
                or value.get("tag") != "agentplugins-v0.1.95" or value.get("commit") != COMMIT
                or type(asset) is not dict or asset.get("sha256") != DIGEST
                or type(asset.get("size")) is not int or asset["size"] != SIZE
                or asset.get("file") != "agentplugins_0.1.95_linux_amd64"):
            raise ValueError("historical npm baseline provenance mismatch")
        return value

    def after_command(self):
        value = self.read_metadata()
        if value != self.metadata:
            raise ValueError("historical npm resolution changed")
        content = regular_bytes(Path(value["binary"]))
        if len(content) != SIZE or hashlib.sha256(content).hexdigest() != DIGEST:
            raise ValueError("historical release binary mismatch")
        self.commands += 1
        self.proof = {"package": "universal-agent-plugins", "version": "0.1.95",
                      "commit": COMMIT, "binary_sha256": DIGEST, "binary_size": SIZE,
                      "launcher_sources_verified": True, "commands_verified": self.commands}


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
    if (any(type(item[key]) is not str for key in item)
            or item["alias"] != fixture["name"] or item["targetId"] != "codex"
            or item["status"] != ("unchanged" if repeated else "succeeded")
            or not re.fullmatch(r"candidate-[0-9a-f]{64}", item["candidateId"])
            or not skill_path or Path(item["destinationPath"]) != Path(skill_path).parent
            or type(installed) is not dict or installed.get("name") != fixture["name"]
            or installed.get("source") != fixture["repo"] + "/skills/" + fixture["folder"]):
        return False
    # Candidate IDs include a new session nonce on each command; they must not
    # be equated across sessions. Each result joins the same immutable fixture
    # through alias/destination and the independently matched list and lock.
    clients = installed.get("clients")
    expected = {"client_id": "codex", "scope": "project",
                "materialization": "materialized", "activation": "active",
                "verification": "installation_verified",
                "authentication": "not_required", "policy": "allowed"}
    return (type(clients) is list and len(clients) == 1 and type(clients[0]) is dict
            and all(clients[0].get(key) == expected_value for key, expected_value in expected.items()))


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
