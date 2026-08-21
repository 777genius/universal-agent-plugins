#!/usr/bin/env python3
"""Validate disposable promotion and contributor journeys at exact Git revisions.

This is deliberately a read-only decision tool.  A successful promotion writes
one deterministic review candidate when ``--candidate-output`` is supplied;
every refusal removes no files and writes no partial candidate.  Submission
validation never creates a PR or invokes a network client.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import subprocess
import sys
import tempfile
from pathlib import Path, PurePosixPath
from typing import Any

import jsonschema

sys.path.insert(0, str(Path(__file__).resolve().parent))
from build_bridges import BridgeError, check_all
from build_registry import DIRECTORY_TREE_DIGEST_ALGORITHM, RegistryError, directory_tree_digest, validate_directory
from validate_catalog import PLUGIN_SCHEMA, ValidationError, validate_plugin


ROOT = Path(__file__).resolve().parents[1]
SHA_RE = __import__("re").compile(r"^[0-9a-f]{40}$")
REPOSITORY_RE = __import__("re").compile(r"^[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9][A-Za-z0-9._-]*$")
REQUIRED_PROMOTION_GATES = ("repository_identity", "reviewed_identity", "candidate_identity", "package", "policy", "evidence")
REQUIRED_SUBMISSION_GATES = ("git_fork_branch", "schema", "package", "registry_policy", "bridge_reproduction", "side_effect_boundary")
REQUIRED_PROMOTION_EVIDENCE = ("package-validation", "registry-policy")


class JourneyError(Exception):
    pass


def require(condition: bool, message: str) -> None:
    if not condition:
        raise JourneyError(message)


def digest(body: bytes) -> str:
    return "sha256:" + hashlib.sha256(body).hexdigest()


def canonical(value: object) -> bytes:
    return (json.dumps(value, sort_keys=True, separators=(",", ":")) + "\n").encode()


def read_object(path: Path) -> dict[str, Any]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError) as error:
        raise JourneyError(f"{path}: invalid JSON: {error}") from error
    require(isinstance(value, dict), f"{path}: expected a JSON object")
    return value


def git(repository: Path, *arguments: str, allow_failure: bool = False) -> subprocess.CompletedProcess[bytes]:
    environment = {
        "PATH": os.environ.get("PATH", ""), "GIT_CONFIG_NOSYSTEM": "1",
        "GIT_CONFIG_GLOBAL": os.devnull, "GIT_TERMINAL_PROMPT": "0",
        "GIT_OPTIONAL_LOCKS": "0", "LANG": "C", "LC_ALL": "C",
    }
    try:
        completed = subprocess.run(
            ["git", *arguments], cwd=repository, env=environment,
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=False, timeout=60,
        )
    except (OSError, subprocess.TimeoutExpired) as error:
        raise JourneyError(f"Git invocation failed: {error}") from error
    if completed.returncode and not allow_failure:
        detail = completed.stderr.decode("utf-8", "replace").strip()
        raise JourneyError(f"git {' '.join(arguments[:2])} failed: {detail}")
    return completed


def exact_commit(repository: Path, revision: str, field: str) -> str:
    require(SHA_RE.fullmatch(revision) is not None, f"{field} must be a full lowercase Git SHA")
    actual = git(repository, "rev-parse", f"{revision}^{{commit}}").stdout.decode().strip()
    require(actual == revision, f"{field} did not resolve to its exact commit")
    return actual


def safe_git_path(value: str) -> str:
    path = PurePosixPath(value)
    require(value and value == path.as_posix() and not path.is_absolute(), "package path must be normalized and relative")
    require(not ({"", ".", "..", ".git"} & set(path.parts)), "package path contains a forbidden segment")
    return value


def materialize(repository: Path, revision: str, source: str, destination: Path) -> None:
    safe_git_path(source)
    records = git(repository, "ls-tree", "-rz", "-r", revision, "--", source).stdout.split(b"\0")
    prefix = source.rstrip("/") + "/"
    count = 0
    for record in records:
        if not record:
            continue
        metadata, raw_path = record.split(b"\t", 1)
        mode, kind, _object = metadata.decode("ascii").split(" ")
        path = raw_path.decode("utf-8")
        require(kind == "blob" and mode in {"100644", "100755"}, f"unsupported Git entry at {path}")
        require(path.startswith(prefix), f"Git returned a path outside {source}")
        relative = safe_git_path(path[len(prefix):])
        target = destination.joinpath(*PurePosixPath(relative).parts)
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(git(repository, "show", f"{revision}:{path}").stdout)
        target.chmod(0o755 if mode == "100755" else 0o644)
        count += 1
    require(count > 0, f"{source} is absent or empty at {revision}")


def package_facts(package: Path) -> dict[str, Any]:
    mcp_count, skill_count = validate_plugin(package)
    manifest_body = (package / "plugin.json").read_bytes()
    manifest = json.loads(manifest_body)
    components = []
    if manifest.get("extensions"):
        components.append("extensions")
    if mcp_count:
        components.append("mcp")
    if skill_count:
        components.append("skills")
    return {
        "manifest_name": manifest["name"], "package_version": manifest["version"],
        "tree_digest": directory_tree_digest(package), "manifest_digest": digest(manifest_body),
        "components": sorted(components),
    }


def gate(name: str, artifact: object) -> dict[str, Any]:
    return {"name": name, "outcome": "passed", "artifact_digest": digest(canonical(artifact)), "artifact": artifact}


def promotion(args: argparse.Namespace) -> dict[str, Any]:
    record = read_object(args.review_record)
    expected_fields = {
        "schema_version", "repository", "path", "reviewed_revision", "reviewed_tree_digest",
        "reviewed_manifest_digest", "product_id", "distribution_id", "required_components",
        "required_targets", "policy_status", "evidence_artifacts",
    }
    require(set(record) == expected_fields and record["schema_version"] == 1, "review record has an invalid field set or schema version")
    require(REPOSITORY_RE.fullmatch(args.repository_id) is not None, "repository identity is invalid")
    require(record["repository"] == args.repository_id, "requested repository differs from reviewed repository")
    require(record["path"] == args.path, "requested package path differs from reviewed path")
    require(record["reviewed_revision"] == args.reviewed_revision, "requested reviewed revision differs from the review record")
    reviewed_revision = exact_commit(args.repository, args.reviewed_revision, "reviewed revision")
    candidate_revision = exact_commit(args.repository, args.candidate_revision, "candidate revision")
    require(record["policy_status"] == "active", "promotion policy must be active")
    require(isinstance(record["required_targets"], list) and record["required_targets"], "promotion requires target policy")
    require(record["required_targets"] == sorted(set(record["required_targets"])), "required targets must be unique and sorted")
    require(set(record["required_targets"]).issubset({"codex", "chatgpt", "cursor", "copilot", "vscode", "kiro"}), "promotion policy contains an unknown target")
    require(isinstance(record["required_components"], list) and record["required_components"] == sorted(set(record["required_components"])), "required components must be unique and sorted")
    require(set(record["required_components"]).issubset({"extensions", "mcp", "skills"}), "promotion policy contains an unknown component")
    evidence = record["evidence_artifacts"]
    require(isinstance(evidence, list) and evidence, "promotion requires command evidence artifacts")
    evidence_names = []
    for item in evidence:
        require(isinstance(item, dict) and set(item) == {"name", "exit_code", "stdout_digest"}, "invalid promotion evidence artifact")
        require(item["exit_code"] == 0 and isinstance(item["stdout_digest"], str) and item["stdout_digest"].startswith("sha256:"), "promotion evidence command did not pass")
        evidence_names.append(item["name"])
    require(evidence_names == sorted(set(evidence_names)), "promotion evidence names must be unique and sorted")
    require(tuple(evidence_names) == REQUIRED_PROMOTION_EVIDENCE, "promotion evidence gates are incomplete")

    gates = [gate("repository_identity", {"repository": args.repository_id, "path": args.path})]
    with tempfile.TemporaryDirectory(prefix="promotion-review-") as reviewed_tmp, tempfile.TemporaryDirectory(prefix="promotion-candidate-") as candidate_tmp:
        package_name = PurePosixPath(args.path).name
        reviewed_root, candidate_root = Path(reviewed_tmp) / package_name, Path(candidate_tmp) / package_name
        reviewed_root.mkdir()
        candidate_root.mkdir()
        materialize(args.repository, reviewed_revision, args.path, reviewed_root)
        reviewed = package_facts(reviewed_root)
        require(reviewed["tree_digest"] == record["reviewed_tree_digest"], "reviewed package tree digest differs from the review record")
        require(reviewed["manifest_digest"] == record["reviewed_manifest_digest"], "reviewed manifest digest differs from the review record")
        require(reviewed["manifest_name"] == record["product_id"], "reviewed manifest identity differs from product identity")
        gates.append(gate("reviewed_identity", {"revision": reviewed_revision, **reviewed}))
        materialize(args.repository, candidate_revision, args.path, candidate_root)
        candidate = package_facts(candidate_root)
        require(candidate["manifest_name"] == record["product_id"], "candidate manifest identity differs from product identity")
        require(set(record["required_components"]).issubset(candidate["components"]), "candidate lost a required component")
        gates.append(gate("candidate_identity", {"revision": candidate_revision, **candidate}))
        gates.append(gate("package", {"validator": "validate_catalog.validate_plugin", "components": candidate["components"]}))
        gates.append(gate("policy", {"status": record["policy_status"], "required_targets": record["required_targets"], "required_components": record["required_components"]}))
        gates.append(gate("evidence", {"commands": evidence}))

    exact_match = candidate["tree_digest"] == reviewed["tree_digest"] and candidate["manifest_digest"] == reviewed["manifest_digest"]
    require(exact_match, "candidate package bytes differ from the reviewed package")
    candidate_output = {
        "schema_version": 1, "decision": "reviewable_promotion_candidate",
        "repository": args.repository_id, "path": args.path,
        "reviewed_revision": reviewed_revision, "candidate_revision": candidate_revision,
        "product_id": record["product_id"], "distribution_id": record["distribution_id"],
        "tree_digest": candidate["tree_digest"], "manifest_digest": candidate["manifest_digest"],
        "components": candidate["components"], "required_targets": record["required_targets"],
        "gate_artifacts": [{"name": item["name"], "artifact_digest": item["artifact_digest"]} for item in gates],
    }
    if args.candidate_output:
        require(not args.candidate_output.exists(), "refusing to overwrite a promotion candidate")
        args.candidate_output.parent.mkdir(parents=True, exist_ok=True)
        args.candidate_output.write_bytes(canonical(candidate_output))
    return {
        "schema_version": 1, "kind": "promotion", "outcome": "accepted", "exact_match": True,
        "candidate_emitted": bool(args.candidate_output), "candidate_digest": digest(canonical(candidate_output)),
        "candidate": candidate_output, "gates": gates,
        "required_gate_names": list(REQUIRED_PROMOTION_GATES),
    }


def submission_source(record: dict[str, Any], revision: str, facts: dict[str, Any], bridge: dict[str, Any]) -> dict[str, Any]:
    components = facts["components"]
    required = {"skills": "required" if "skills" in components else "optional", "mcp": "required" if "mcp" in components else "optional"}
    distribution_id = record["distribution_id"]
    product_id = record["product_id"]
    return {
        "schema_version": 1,
        "products": [{
            "schema_version": 1, "id": product_id, "display_name": "Fixture Bridge",
            "description": "Disposable external contributor validation fixture.", "manifest_name": product_id,
            "aliases": [product_id], "reserved_aliases": [product_id], "categories": ["bridge"],
            "minimum_capabilities": required, "default_distribution": distribution_id,
            "distributions": [distribution_id],
        }],
        "distributions": [{
            "schema_version": 1, "id": distribution_id, "product_id": product_id,
            "kind": "community_bridge", "status": "active", "packager": distribution_id.split("/", 1)[0],
            "releases": [{
                "sequence": 1, "package_version": facts["package_version"], "manifest_name": product_id,
                "agent_plugins_schema": PLUGIN_SCHEMA,
                "package_source": {"repository": record["fork_repository"], "revision": revision, "path": record["package_path"]},
                "build_provenance": {"upstream_repository": bridge["upstream_repository"], "upstream_revision": bridge["upstream_revision"]},
                "tree_digest_algorithm": DIRECTORY_TREE_DIGEST_ALGORITHM, "tree_digest": facts["tree_digest"],
                "manifest_digest": facts["manifest_digest"], "components": components,
            }],
            "release_policies": [{
                "release_sequence": 1, "status": "active", "minimum_installer_version": "0.1.8",
                "targets": [{
                    "client": "codex", "scopes": ["user"], "delivery": "managed",
                    "authentication": "not_required",
                }],
                "current_evidence": [],
            }],
        }],
        "evidence": [],
    }


def submission(args: argparse.Namespace) -> dict[str, Any]:
    record = read_object(args.submission_record)
    expected_fields = {"schema_version", "fork_repository", "base_revision", "branch", "branch_revision", "package_path", "bridge_root", "bridge_id", "product_id", "distribution_id"}
    require(set(record) == expected_fields and record["schema_version"] == 1, "submission record has an invalid field set or schema version")
    require(REPOSITORY_RE.fullmatch(record["fork_repository"]) is not None, "fork repository identity is invalid")
    require(record["branch"] not in {"main", "master"} and record["branch"].startswith("contribution/"), "submission must use a contribution branch")
    base = exact_commit(args.repository, record["base_revision"], "base revision")
    revision = exact_commit(args.repository, record["branch_revision"], "branch revision")
    head = git(args.repository, "rev-parse", "HEAD^{commit}").stdout.decode().strip()
    branch = git(args.repository, "branch", "--show-current").stdout.decode().strip()
    require(head == revision and branch == record["branch"], "checked-out fork branch identity differs from the submission")
    require(git(args.repository, "merge-base", "--is-ancestor", base, revision, allow_failure=True).returncode == 0, "submission branch is not based on the pinned base")
    require(base != revision, "submission branch has no contribution commit")
    origin = git(args.repository, "remote", "get-url", "origin").stdout.decode().strip()
    require(not origin.startswith(("http://", "https://", "ssh://", "git@")), "submission validation requires a local disposable fork remote")
    gates = [gate("git_fork_branch", {"base_revision": base, "branch_revision": revision, "branch": branch, "origin_kind": "local"})]

    package = args.repository / safe_git_path(record["package_path"])
    bridge_root = args.repository if record["bridge_root"] == "." else args.repository / safe_git_path(record["bridge_root"])
    manifest = read_object(package / "plugin.json")
    schema = read_object(ROOT / "schemas/1.0.0/plugin.schema.json")
    jsonschema.Draft202012Validator(schema).validate(manifest)
    gates.append(gate("schema", {"schema": PLUGIN_SCHEMA, "manifest_digest": digest((package / "plugin.json").read_bytes())}))
    facts = package_facts(package)
    require(facts["manifest_name"] == record["product_id"], "submission product and manifest identities differ")
    gates.append(gate("package", {"validator": "validate_catalog.validate_plugin", **facts}))
    reports = check_all(bridge_root, args.upstream_mirror)
    report = next((item for item in reports if item["bridge_id"] == record["bridge_id"]), None)
    require(report is not None, "submitted bridge was not reproduced")
    require(report["tree_digest"] == facts["tree_digest"] and report["manifest_digest"] == facts["manifest_digest"], "bridge reproduction differs from submitted package")
    source = submission_source(record, revision, facts, report)
    validate_directory(source, verify_packages=False)
    gates.append(gate("registry_policy", {"validator": "build_registry.validate_directory", "source_digest": digest(canonical(source)), "status": "active", "targets": ["codex"]}))
    gates.append(gate("bridge_reproduction", {"validator": "build_bridges.check_all", "report_digest": digest(canonical(report)), "tree_digest": report["tree_digest"]}))
    gates.append(gate("side_effect_boundary", {"remote_kind": "local", "network_commands": 0, "pr_created": 0, "publication_created": 0}))
    return {
        "schema_version": 1, "kind": "submission", "outcome": "accepted",
        "repository": record["fork_repository"], "base_revision": base, "branch_revision": revision,
        "branch": branch, "package": facts, "gates": gates,
        "required_gate_names": list(REQUIRED_SUBMISSION_GATES),
        "side_effects": {"network_commands": 0, "pr_created": 0, "publication_created": 0},
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)
    promote = subparsers.add_parser("promotion")
    promote.add_argument("--repository", type=Path, required=True)
    promote.add_argument("--repository-id", required=True)
    promote.add_argument("--reviewed-revision", required=True)
    promote.add_argument("--candidate-revision", required=True)
    promote.add_argument("--path", required=True)
    promote.add_argument("--review-record", type=Path, required=True)
    promote.add_argument("--candidate-output", type=Path)
    submit = subparsers.add_parser("submission")
    submit.add_argument("--repository", type=Path, required=True)
    submit.add_argument("--submission-record", type=Path, required=True)
    submit.add_argument("--upstream-mirror", type=Path, required=True)
    args = parser.parse_args()
    try:
        result = promotion(args) if args.command == "promotion" else submission(args)
    except (JourneyError, ValidationError, RegistryError, BridgeError, jsonschema.ValidationError, OSError, ValueError, json.JSONDecodeError) as error:
        print(json.dumps({"schema_version": 1, "kind": args.command, "outcome": "rejected", "reason": str(error)}, sort_keys=True, separators=(",", ":")))
        return 2
    print(json.dumps(result, sort_keys=True, separators=(",", ":")))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
