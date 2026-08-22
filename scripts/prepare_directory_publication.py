#!/usr/bin/env python3
"""Validate canonical Directory source and build a no-secret publication candidate."""

from __future__ import annotations

import argparse
import copy
import hashlib
import os
import stat
import subprocess
import sys
import tempfile
import time
from pathlib import Path
from typing import Any

sys.path.insert(0, str(Path(__file__).resolve().parent))
from build_registry import (
    RegistryError,
    directory_tree_digest as package_tree_digest,
    policy_eligibility_broadened,
    release_eligibility_broadened,
    releases_requiring_validation,
    validate_active_local_runtime_closures,
    validate_bridge_bindings,
    validate_changed_external_releases,
    validate_release_package,
)
from directory_publication import (
    CANDIDATE_SCHEMA,
    DIGEST_RE,
    PublicationError,
    SHA_RE,
    atomic_write,
    candidate_digest,
    canonical_json,
    load_ledger_latest,
    load_public_keys,
    parse_json_bytes,
    read_json,
    require,
    validate_with_schema,
)
from directory_publication_cas import CasError, validate_marker


ROOT = Path(__file__).resolve().parents[1]
SOURCE_SCHEMA = ROOT / "schemas" / "directory-source.schema.json"
EVIDENCE_ARTIFACT_SCHEMA = ROOT / "schemas" / "directory-evidence-artifact.schema.json"
CONFIG_FIELDS = {"schema_version", "repository", "snapshot_lifetime_days"}
OPTIONAL_CONFIG_FIELDS = {"trusted_evidence_workflows", "trusted_external_evidence"}
TRUSTED_WORKFLOW_FIELDS = {
    "workflow", "protected_source_ref", "source_digest_policy", "allow_self_hosted_runners",
}
GIT = "/usr/bin/git"
GH = "/usr/bin/gh"
ACQUISITION_TIMEOUT_SECONDS = 180
MAX_PLUGIN_FILES = 10_000
MAX_PLUGIN_FILE_BYTES = 16 << 20
MAX_PLUGIN_TREE_BYTES = 64 << 20
MAX_EVIDENCE_BYTES = 4 << 20


def load_config(path: Path) -> dict[str, Any]:
    value = read_json(path, max_bytes=64 << 10)
    require(
        isinstance(value, dict)
        and CONFIG_FIELDS <= set(value) <= CONFIG_FIELDS | OPTIONAL_CONFIG_FIELDS,
        f"{path}: invalid publication config fields",
    )
    require(value["schema_version"] == 1, f"{path}: schema_version must be 1")
    require(isinstance(value["repository"], str) and "/" in value["repository"], f"{path}: invalid repository")
    require(isinstance(value["snapshot_lifetime_days"], int) and 1 <= value["snapshot_lifetime_days"] <= 30, f"{path}: invalid lifetime")
    workflows = value.setdefault("trusted_evidence_workflows", [])
    require(isinstance(workflows, list), f"{path}: invalid trusted evidence workflows")
    workflow_names: list[str] = []
    for item in workflows:
        require(isinstance(item, dict) and set(item) == TRUSTED_WORKFLOW_FIELDS, f"{path}: invalid trusted evidence workflow policy")
        require(
            isinstance(item["workflow"], str) and "/.github/workflows/" in item["workflow"]
            and isinstance(item["protected_source_ref"], str) and item["protected_source_ref"].startswith("refs/heads/")
            and item["source_digest_policy"] == "artifact_revision"
            and type(item["allow_self_hosted_runners"]) is bool,
            f"{path}: invalid trusted evidence workflow policy",
        )
        workflow_names.append(item["workflow"])
    require(len(workflow_names) == len(set(workflow_names)), f"{path}: duplicate trusted evidence workflow")
    external = value.setdefault("trusted_external_evidence", [])
    require(isinstance(external, list), f"{path}: invalid trusted external evidence")
    for item in external:
        require(
            isinstance(item, dict) and set(item) == {"repository", "revision", "path", "digest"},
            f"{path}: invalid trusted external evidence entry",
        )
        require(
            isinstance(item["repository"], str) and item["repository"].count("/") == 1
            and isinstance(item["revision"], str) and SHA_RE.fullmatch(item["revision"]) is not None
            and isinstance(item["path"], str) and item["path"]
            and not item["path"].startswith("/") and "\\" not in item["path"] and ".." not in Path(item["path"]).parts
            and isinstance(item["digest"], str) and DIGEST_RE.fullmatch(item["digest"]) is not None,
            f"{path}: invalid trusted external evidence identity",
        )
    return value


def previous_releases(snapshot: dict[str, Any] | None) -> dict[tuple[str, int], dict[str, Any]]:
    if snapshot is None:
        return {}
    return {
        (distribution["id"], release["sequence"]): release
        for distribution in snapshot["distributions"]
        for release in distribution["releases"]
    }


def previous_distributions(snapshot: dict[str, Any] | None) -> dict[str, dict[str, Any]]:
    return {} if snapshot is None else {item["id"]: item for item in snapshot["distributions"]}


def policy_map(distribution: dict[str, Any]) -> dict[int, dict[str, Any]]:
    return {item["release_sequence"]: item for item in distribution["release_policies"]}


def eligibility_broadened(
    distribution: dict[str, Any], policy: dict[str, Any],
    old_distribution: dict[str, Any] | None, old_policy: dict[str, Any] | None,
) -> bool:
    return policy_eligibility_broadened(
        distribution, policy, old_distribution, old_policy,
    )


def manifest_digest(package_root: Path) -> str:
    manifest = package_root / "plugin.json"
    require(manifest.is_file(), f"{package_root}: plugin.json is missing")
    return "sha256:" + hashlib.sha256(manifest.read_bytes()).hexdigest()


def verify_package(package_root: Path, release: dict[str, Any], identity: str) -> None:
    try:
        validate_release_package(
            package_root, release, label=identity,
            allow_unresolved_revision=release["package_source"]["revision"] is None,
        )
    except RegistryError as error:
        raise PublicationError(str(error)) from error


def acquisition_environment(temporary_root: Path) -> dict[str, str]:
    return {
        "PATH": "/usr/bin:/bin",
        "HOME": str(temporary_root),
        "GIT_CONFIG_NOSYSTEM": "1",
        "GIT_CONFIG_GLOBAL": os.devnull,
        "GIT_TERMINAL_PROMPT": "0",
        "GIT_ASKPASS": os.devnull,
        "SSH_ASKPASS": os.devnull,
        "GIT_LFS_SKIP_SMUDGE": "1",
        "GIT_OPTIONAL_LOCKS": "0",
        "LC_ALL": "C",
    }


def inspect_plugin_root(package_root: Path, identity: str) -> None:
    require(package_root.is_dir(), f"{identity}: package path is unavailable")
    file_count = 0
    total_bytes = 0
    for path in package_root.rglob("*"):
        metadata = path.lstat()
        if stat.S_ISDIR(metadata.st_mode):
            continue
        relative = path.relative_to(package_root)
        require(
            stat.S_ISREG(metadata.st_mode) or stat.S_ISLNK(metadata.st_mode),
            f"{identity}: package contains unsupported special file {relative}",
        )
        file_count += 1
        total_bytes += metadata.st_size
        require(file_count <= MAX_PLUGIN_FILES, f"{identity}: package exceeds {MAX_PLUGIN_FILES} files")
        require(metadata.st_size <= MAX_PLUGIN_FILE_BYTES, f"{identity}: file {relative} exceeds {MAX_PLUGIN_FILE_BYTES} bytes")
        require(total_bytes <= MAX_PLUGIN_TREE_BYTES, f"{identity}: package exceeds {MAX_PLUGIN_TREE_BYTES} bytes")
        if stat.S_ISLNK(metadata.st_mode):
            continue
        with path.open("rb") as source:
            prefix = source.read(256)
        require(
            not prefix.startswith(b"version https://git-lfs.github.com/spec/v1\n"),
            f"{identity}: Git LFS pointer {relative} is unsupported",
        )


def acquire_external(repository: str, revision: str, package_path: str, override: Path | None = None) -> tempfile.TemporaryDirectory[str]:
    temporary = tempfile.TemporaryDirectory(prefix="directory-publication-")
    checkout = Path(temporary.name) / "checkout"
    source = override.resolve().as_uri() if override is not None else f"https://github.com/{repository}.git"
    environment = acquisition_environment(Path(temporary.name))
    deadline = time.monotonic() + ACQUISITION_TIMEOUT_SECONDS

    def git(*arguments: str, text: bool = False) -> subprocess.CompletedProcess[Any]:
        remaining = deadline - time.monotonic()
        require(remaining > 0, f"{repository}@{revision}//{package_path}: reacquisition timed out")
        return subprocess.run(
            [
                GIT, "-c", "credential.helper=", "-c", "core.askPass=", "-c", "submodule.recurse=false",
                "-c", "http.followRedirects=false",
                *arguments,
            ],
            check=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            timeout=remaining,
            env=environment,
            text=text,
        )

    try:
        git("init", "--quiet", str(checkout))
        git("-C", str(checkout), "sparse-checkout", "init", "--no-cone")
        git("-C", str(checkout), "sparse-checkout", "set", "--no-cone", f"/{package_path}/")
        git(
            "-C", str(checkout), "-c", "protocol.file.allow=always", "fetch", "--quiet",
            "--no-tags", "--no-recurse-submodules", "--depth=1", "--filter=blob:none", source, revision,
        )
        resolved = git("-C", str(checkout), "rev-parse", "FETCH_HEAD", text=True).stdout.strip()
        require(resolved == revision, f"{repository}@{revision}: reacquisition resolved {resolved}")
        tree = git("-C", str(checkout), "ls-tree", "-r", "-z", "FETCH_HEAD", "--", package_path).stdout
        entries = tree.split(b"\0") if isinstance(tree, bytes) else tree.encode().split(b"\0")
        for entry in entries:
            if not entry:
                continue
            metadata = entry.split(b"\t", 1)[0].split()
            require(metadata[:2] != [b"160000", b"commit"], f"{repository}@{revision}//{package_path}: Git submodule content is unsupported")
        kind = git("-C", str(checkout), "cat-file", "-t", f"FETCH_HEAD:{package_path}", text=True).stdout.strip()
        require(kind == "tree", f"{repository}@{revision}//{package_path}: package path is unavailable")
        git("-C", str(checkout), "checkout", "--quiet", "--detach", "--no-recurse-submodules", "FETCH_HEAD")
        inspect_plugin_root(checkout / package_path, f"{repository}@{revision}//{package_path}")
        return temporary
    except PublicationError:
        temporary.cleanup()
        raise
    except (OSError, subprocess.SubprocessError) as error:
        temporary.cleanup()
        raise PublicationError(f"{repository}@{revision}//{package_path}: reacquisition failed: {error}") from error


def acquire_evidence_bytes(
    artifact: dict[str, str], override: Path | None = None,
) -> tuple[tempfile.TemporaryDirectory[str], bytes]:
    """Acquire one exact evidence blob without executing repository content."""
    repository, revision, artifact_path = (
        artifact["repository"], artifact["revision"], artifact["path"]
    )
    require(not artifact_path.startswith("/") and "\\" not in artifact_path and ".." not in Path(artifact_path).parts, "evidence artifact path is unsafe")
    temporary = tempfile.TemporaryDirectory(prefix="directory-evidence-")
    checkout = Path(temporary.name) / "checkout"
    source = override.resolve().as_uri() if override is not None else f"https://github.com/{repository}.git"
    environment = acquisition_environment(Path(temporary.name))
    label = f"{repository}@{revision}//{artifact_path}"
    try:
        subprocess.run([GIT, "init", "--quiet", str(checkout)], check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=environment)
        subprocess.run(
            [GIT, "-C", str(checkout), "-c", "credential.helper=", "-c", "core.askPass=", "-c", "submodule.recurse=false",
             "-c", "protocol.file.allow=always", "fetch", "--quiet", "--no-tags", "--no-recurse-submodules", "--depth=1", "--filter=blob:none", source, revision],
            check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=ACQUISITION_TIMEOUT_SECONDS, env=environment,
        )
        resolved = subprocess.check_output([GIT, "-C", str(checkout), "rev-parse", "FETCH_HEAD"], text=True, env=environment).strip()
        require(resolved == revision, f"{label}: reacquisition resolved {resolved}")
        object_name = f"FETCH_HEAD:{artifact_path}"
        kind_result = subprocess.run(
            [GIT, "-C", str(checkout), "cat-file", "-t", object_name], check=False,
            text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=environment,
        )
        require(kind_result.returncode == 0 and kind_result.stdout.strip() == "blob", f"{label}: evidence artifact is unavailable or not a regular file")
        size_text = subprocess.check_output([GIT, "-C", str(checkout), "cat-file", "-s", object_name], text=True, env=environment).strip()
        require(size_text.isdigit() and int(size_text) <= MAX_EVIDENCE_BYTES, f"{label}: evidence artifact exceeds {MAX_EVIDENCE_BYTES} bytes")
        body = subprocess.check_output([GIT, "-C", str(checkout), "show", object_name], env=environment, timeout=ACQUISITION_TIMEOUT_SECONDS)
        require(len(body) == int(size_text), f"{label}: evidence artifact size changed during acquisition")
        return temporary, body
    except PublicationError:
        temporary.cleanup()
        raise
    except (OSError, subprocess.SubprocessError) as error:
        temporary.cleanup()
        raise PublicationError(f"{label}: evidence reacquisition failed: {error}") from error


def verify_evidence_trust(
    pointer: dict[str, Any], config: dict[str, Any], temporary_root: Path, body: bytes,
) -> None:
    artifact = pointer["artifact"]
    trust = pointer["trust"]
    if trust["kind"] == "reviewed_external":
        require(
            any(artifact == allowed for allowed in config["trusted_external_evidence"]),
            f"{pointer['id']}: external evidence artifact is not explicitly trusted",
        )
        return
    workflow = trust["workflow"]
    policy = next((item for item in config["trusted_evidence_workflows"] if item["workflow"] == workflow), None)
    require(policy is not None, f"{pointer['id']}: evidence workflow has no reviewed trust policy")
    require(workflow.startswith(artifact["repository"] + "/.github/workflows/"), f"{pointer['id']}: workflow and artifact repositories differ")
    require(trust["source_ref"] == policy["protected_source_ref"], f"{pointer['id']}: evidence source ref is not trusted")
    require(
        policy["source_digest_policy"] == "artifact_revision" and trust["source_digest"] == artifact["revision"],
        f"{pointer['id']}: evidence source digest does not match the exact artifact revision",
    )
    require(Path(GH).is_file(), f"reviewed evidence verifier is missing: {GH}")
    acquired = temporary_root / "verified-evidence.json"
    acquired.write_bytes(body)
    command = [
        GH, "attestation", "verify", str(acquired), "--repo", artifact["repository"],
        "--signer-workflow", workflow, "--source-ref", trust["source_ref"],
        "--source-digest", trust["source_digest"],
    ]
    if not policy["allow_self_hosted_runners"]:
        command.append("--deny-self-hosted-runners")
    completed = subprocess.run(
        command,
        check=False, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
        env=acquisition_environment(temporary_root), timeout=ACQUISITION_TIMEOUT_SECONDS,
    )
    require(completed.returncode == 0, f"{pointer['id']}: trusted workflow attestation verification failed")


def verified_evidence(
    pointer: dict[str, Any], config: dict[str, Any], overrides: dict[str, Path],
) -> dict[str, Any]:
    require("trust" in pointer, f"{pointer['id']}: evidence has no trusted workflow or external-attestation path")
    artifact = pointer["artifact"]
    temporary, body = acquire_evidence_bytes(artifact, overrides.get(artifact["repository"]))
    try:
        require("sha256:" + hashlib.sha256(body).hexdigest() == artifact["digest"], f"{pointer['id']}: evidence artifact digest mismatch")
        # Parse the verified bytes directly; never derive signed fields from the
        # contributor-authored pointer.
        payload = parse_json_bytes(body, f"evidence {pointer['id']}", max_bytes=MAX_EVIDENCE_BYTES)
        require(isinstance(payload, dict), f"{pointer['id']}: evidence artifact must be an object")
        validate_with_schema(payload, EVIDENCE_ARTIFACT_SCHEMA)
        require(payload["id"] == pointer["id"], f"{pointer['id']}: evidence artifact identity mismatch")
        verify_evidence_trust(pointer, config, Path(temporary.name), body)
        return {**copy.deepcopy(payload), "artifact": copy.deepcopy(artifact)}
    finally:
        temporary.cleanup()


def selected_evidence(
    source: dict[str, Any], published_distributions: set[str], config: dict[str, Any],
    overrides: dict[str, Path],
) -> list[dict[str, Any]]:
    evidence = {item["id"]: item for item in source["evidence"]}
    require(len(evidence) == len(source["evidence"]), "duplicate evidence identity")
    selected: set[str] = set()
    verified: dict[str, dict[str, Any]] = {}
    releases = {
        (distribution["id"], release["sequence"]): (distribution["product_id"], release)
        for distribution in source["distributions"] for release in distribution["releases"]
    }
    for distribution in source["distributions"]:
        if distribution["id"] not in published_distributions:
            continue
        for policy in distribution["release_policies"]:
            identity = (distribution["id"], policy["release_sequence"])
            require(identity in releases, f"{distribution['id']}: policy references missing release {policy['release_sequence']}")
            for evidence_id in policy["current_evidence"]:
                require(evidence_id in evidence, f"{distribution['id']}@{policy['release_sequence']}: missing evidence {evidence_id}")
                record = verified.get(evidence_id)
                if record is None:
                    record = verified_evidence(evidence[evidence_id], config, overrides)
                    verified[evidence_id] = record
                require((record["distribution_id"], record["release_sequence"]) == identity, f"{evidence_id}: evidence release identity mismatch")
                product_id, release = releases[identity]
                source_identity = release["package_source"]
                require(
                    record["product_id"] == product_id
                    and record["package_tree_digest"] == release["tree_digest"]
                    and record["manifest_digest"] == release["manifest_digest"]
                    and record["source_repository"] == source_identity["repository"]
                    and record["source_revision"] == source_identity["revision"]
                    and record["source_path"] == source_identity["path"],
                    f"{evidence_id}: evidence source identity mismatch",
                )
                selected.add(evidence_id)
    return [copy.deepcopy(verified[item]) for item in sorted(selected)]


def validate_upstream_default_evidence(
    products: list[dict[str, Any]], distributions: list[dict[str, Any]],
    evidence: list[dict[str, Any]],
) -> None:
    distributions_by_id = {item["id"]: item for item in distributions}
    evidence_by_id = {item["id"]: item for item in evidence}
    for product in products:
        distribution = distributions_by_id[product["default_distribution"]]
        if distribution["kind"] != "upstream" or distribution["status"] != "active":
            continue
        policies = policy_map(distribution)
        required = {
            component for component, state in product["minimum_capabilities"].items()
            if state == "required"
        }
        eligible = [
            release for release in distribution["releases"]
            if policies[release["sequence"]]["status"] == "active"
            and required.issubset(release["components"])
        ]
        # A safety publication must be able to leave a product with no eligible
        # release. Resolution remains fail-closed because only an active
        # distribution with an active release is selectable.
        if not eligible:
            continue
        release = eligible[-1]
        policy = policies[release["sequence"]]
        passed = {
            observation.get("client")
            for evidence_id in policy["current_evidence"]
            for observation in [evidence_by_id[evidence_id]]
            if observation.get("distribution_id") == distribution["id"]
            and observation.get("release_sequence") == release["sequence"]
            and observation.get("package_tree_digest") == release["tree_digest"]
            and observation.get("level") == "materialization"
            and observation.get("outcome") == "passed"
        }
        missing = sorted(target["client"] for target in policy["targets"] if target["client"] not in passed)
        require(
            not missing,
            f"{product['id']}: upstream default {distribution['id']}@{release['sequence']} lacks exact passed materialization evidence for {','.join(missing)}",
        )


def validate_reproduced_bridges(
    source: dict[str, Any], repository_root: Path, repository: str,
    previous: dict[str, Any] | None,
) -> None:
    """Reproduce every local bridge binding that gains or changes eligibility."""
    try:
        # First reject cheap source/recipe/package mismatches without network.
        validate_bridge_bindings(
            source, repository_root=repository_root, repository=repository,
        )
        old_distributions = previous_distributions(previous)
        products = {item["id"]: item for item in source["products"]}
        old_products = {
            item["id"]: item for item in previous["products"]
        } if previous else {}
        reproduce: set[str] = set()
        for distribution in source["distributions"]:
            if distribution["kind"] != "community_bridge":
                continue
            local = [
                release for release in distribution["releases"]
                if release["package_source"]["repository"] == repository
            ]
            if not local:
                continue
            current_release = max(local, key=lambda item: item["sequence"])
            policies = policy_map(distribution)
            old_distribution = old_distributions.get(distribution["id"])
            old_releases = {
                item["sequence"]: item for item in old_distribution["releases"]
            } if old_distribution else {}
            old_policies = policy_map(old_distribution) if old_distribution else {}
            product = products[distribution["product_id"]]
            old_product = old_products.get(distribution["product_id"])
            for release in local:
                sequence = release["sequence"]
                policy = policies[sequence]
                old_release = old_releases.get(sequence)
                old_policy = old_policies.get(sequence)
                eligible = distribution["status"] == "active" and policy["status"] == "active"
                changed_eligible_binding = eligible and (
                    old_release is None
                    or old_release != release
                    or old_policy is None
                    or old_policy != policy
                    or eligibility_broadened(
                        distribution, policy, old_distribution, old_policy,
                    )
                    or release_eligibility_broadened(
                        product, distribution, release, policy,
                        old_product, old_distribution, old_policy,
                    )
                )
                if changed_eligible_binding:
                    require(
                        sequence == current_release["sequence"],
                        f"{distribution['id']}@{sequence}: active historical bridge requires reproduction, "
                        f"but the canonical recipe represents release {current_release['sequence']}; "
                        "versioned historical reproduction inputs are unavailable",
                    )
                    reproduce.add(distribution["product_id"])
                elif not eligible:
                    require(
                        old_release is not None and old_release == release,
                        f"{distribution['id']}@{sequence}: inactive bridge has no unchanged previously signed immutable binding",
                    )
        if not reproduce:
            return
        from build_bridges import BridgeError, assemble, compare_trees

        with tempfile.TemporaryDirectory(prefix="bridge-publication-check-") as temporary:
            for bridge_id in sorted(reproduce):
                destination = Path(temporary) / bridge_id
                destination.mkdir()
                try:
                    report = assemble(repository_root, bridge_id, destination, None)
                    compare_trees(repository_root / str(report["package_path"]), destination)
                except (BridgeError, OSError, ValueError) as error:
                    raise PublicationError(f"bridge reproduction failed: {error}") from error
    except RegistryError as error:
        raise PublicationError(str(error)) from error


def validate_signing_boundary_packages(
    source: dict[str, Any], previous: dict[str, Any] | None, repository_root: Path,
    repository: str, overrides: dict[str, Path],
) -> None:
    """Invoke the centralized package policy at the last pre-signing boundary."""
    try:
        validate_changed_external_releases(
            source, previous, repository=repository,
            repository_overrides=overrides,
        )
        # Existing local bindings normally remain offline-capable.  Reacquire
        # only a previously signed binding whose eligibility is broadened, and
        # always from its immutable revision rather than the newest checkout.
        prior = previous_releases(previous)
        distributions = {item["id"]: item for item in source["distributions"]}
        for identity in sorted(releases_requiring_validation(source, previous)):
            if identity not in prior:
                continue  # New/current bindings were validated while binding above.
            distribution = distributions[identity[0]]
            release = next(item for item in distribution["releases"] if item["sequence"] == identity[1])
            package_source = release["package_source"]
            if package_source["repository"] != repository:
                continue  # Central external reacquisition handled above.
            revision = package_source["revision"]
            require(
                isinstance(revision, str) and SHA_RE.fullmatch(revision) is not None,
                f"{identity[0]}@{identity[1]}: historical source requires a full pinned revision",
            )
            temporary = acquire_external(
                repository, revision, package_source["path"], repository_root,
            )
            try:
                validate_release_package(
                    Path(temporary.name) / "checkout" / package_source["path"],
                    release, label=f"{identity[0]}@{identity[1]}",
                )
            finally:
                temporary.cleanup()
        validate_active_local_runtime_closures(
            source, repository_root=repository_root, repository=repository,
        )
    except RegistryError as error:
        raise PublicationError(str(error)) from error


def build_candidate(
    source: dict[str, Any], config: dict[str, Any], source_commit: str,
    publication_id: str, previous: dict[str, Any] | None,
    *, repository_root: Path = ROOT,
    external_overrides: dict[str, Path] | None = None,
) -> dict[str, Any]:
    validate_with_schema(source, SOURCE_SCHEMA)
    require(SHA_RE.fullmatch(source_commit) is not None, "source commit must be a full lowercase SHA")
    try:
        # Establish recipe/source binding before package validation so a
        # contributor cannot substitute provenance while retaining package bytes.
        validate_bridge_bindings(
            source, repository_root=repository_root,
            repository=config["repository"],
        )
    except RegistryError as error:
        raise PublicationError(str(error)) from error
    prior = previous_releases(previous)
    prior_distributions = previous_distributions(previous)
    all_distributions = {item["id"]: item for item in source["distributions"]}
    require(len(all_distributions) == len(source["distributions"]), "duplicate distribution identity")
    distributions_by_id = {identity: item for identity, item in all_distributions.items() if item["status"] != "candidate"}
    products = sorted((copy.deepcopy(item) for item in source["products"]), key=lambda item: item["id"])
    aliases: dict[str, str] = {}
    referenced_distributions: set[str] = set()
    for product in products:
        require(set(product["aliases"]).issubset(product["reserved_aliases"]), f"{product['id']}: active aliases must remain reserved")
        for alias in product["reserved_aliases"]:
            require(alias not in aliases, f"reserved alias {alias} is assigned to multiple products")
            aliases[alias] = product["id"]
        referenced_distributions.update(product["distributions"])
        require(all(item in all_distributions for item in product["distributions"]), f"{product['id']}: references an unknown distribution")
        product["distributions"] = [item for item in product["distributions"] if item in distributions_by_id]
        require(product["default_distribution"] in product["distributions"], f"{product['id']}: default distribution is not listed")
        for distribution_id in product["distributions"]:
            require(distribution_id in distributions_by_id, f"{product['id']}: missing distribution {distribution_id}")
            require(distributions_by_id[distribution_id]["product_id"] == product["id"], f"{product['id']}: mismatched distribution {distribution_id}")
    require(referenced_distributions == set(all_distributions), "every distribution must be owned by exactly one product")

    if previous is not None:
        product_map = {product["id"]: product for product in products}
        for old_product in previous["products"]:
            product_id = old_product["id"]
            require(product_id in product_map, f"published product {product_id} was removed")
            product = product_map[product_id]
            require(product["manifest_name"] == old_product["manifest_name"], f"published product {product_id} manifest name changed")
            historical_aliases = set(old_product["aliases"]) | set(old_product["reserved_aliases"])
            require(historical_aliases.issubset(product["reserved_aliases"]), f"published product {product_id} reserved alias was removed")
            require(set(old_product["distributions"]).issubset(product["distributions"]), f"published product {product_id} distribution was removed")

    output_distributions: list[dict[str, Any]] = []
    overrides = external_overrides or {}
    for original in sorted(distributions_by_id.values(), key=lambda item: item["id"]):
        distribution = copy.deepcopy(original)
        policies = policy_map(distribution)
        require(len(policies) == len(distribution["release_policies"]), f"{distribution['id']}: duplicate release policy")
        require(set(policies) == {release["sequence"] for release in distribution["releases"]}, f"{distribution['id']}: releases and policies must be one-to-one")
        old_distribution = prior_distributions.get(distribution["id"])
        if old_distribution is not None:
            for field in ("id", "product_id", "kind", "packager"):
                require(distribution[field] == old_distribution[field], f"published distribution {distribution['id']} {field} changed")
        old_policies = policy_map(old_distribution) if old_distribution else {}
        for release in distribution["releases"]:
            owning_product = next(product for product in products if product["id"] == distribution["product_id"])
            require(release["manifest_name"] == owning_product["manifest_name"], f"{distribution['id']}@{release['sequence']}: manifest identity differs from product")
            identity = (distribution["id"], release["sequence"])
            label = f"{identity[0]}@{identity[1]}"
            old = prior.get(identity)
            prior_for_distribution = [item for (distribution_id, _), item in prior.items() if distribution_id == distribution["id"]]
            package_source = release["package_source"]
            in_repository = package_source["repository"] == config["repository"]
            if old is not None:
                immutable = {key: value for key, value in release.items() if key not in ("published_at", "package_source")}
                old_immutable = {key: value for key, value in old.items() if key not in ("published_at", "package_source")}
                require(immutable == old_immutable, f"{label}: published immutable release fields changed")
                require(package_source["repository"] == old["package_source"]["repository"] and package_source["path"] == old["package_source"]["path"], f"{label}: published package source changed")
                if not in_repository:
                    require(package_source["revision"] == old["package_source"]["revision"], f"{label}: published external source revision changed")
                release["package_source"] = copy.deepcopy(old["package_source"])
                release["published_at"] = old["published_at"]
            else:
                if prior_for_distribution:
                    highest = max(item["sequence"] for item in prior_for_distribution)
                    require(release["sequence"] > highest, f"{label}: new release sequence must be above {highest}")
                    require(
                        all((release["tree_digest"], release["manifest_digest"]) != (item["tree_digest"], item["manifest_digest"]) for item in prior_for_distribution),
                        f"{label}: unchanged package bytes must reuse their existing release; publish policy/evidence only",
                    )
                reviewed_revision = package_source["revision"]
                reviewed_published_at = release.get("published_at")
                historical = isinstance(reviewed_revision, str) and reviewed_published_at is not None
                if historical:
                    require(SHA_RE.fullmatch(reviewed_revision) is not None, f"{label}: historical source requires reviewed full SHA")
                    if in_repository:
                        temporary = acquire_external(
                            package_source["repository"], reviewed_revision, package_source["path"],
                            repository_root,
                        )
                        try:
                            verify_package(
                                Path(temporary.name) / "checkout" / package_source["path"], release, label,
                            )
                        finally:
                            temporary.cleanup()
                elif in_repository:
                    require(
                        reviewed_revision is None and reviewed_published_at is None,
                        f"{label}: new in-repository release must have an unresolved revision and no published_at",
                    )
                    release["published_at"] = None
                    # Review source cannot author this binding.  Only an unresolved
                    # revision is bound after the checked-out merge tree passes the
                    # reviewed digest checks below.
                    verify_package(repository_root / package_source["path"], release, label)
                    package_source["revision"] = source_commit
                else:
                    require(
                        isinstance(reviewed_revision, str) and SHA_RE.fullmatch(reviewed_revision) is not None and reviewed_published_at is None,
                        f"{label}: new external release requires a reviewed full SHA and no published_at",
                    )
                    release["published_at"] = None
        distribution["releases"].sort(key=lambda item: item["sequence"])
        distribution["release_policies"].sort(key=lambda item: item["release_sequence"])
        output_distributions.append(distribution)

    # Published releases are append-only even if review source accidentally drops one.
    for identity in prior:
        require(any(d["id"] == identity[0] and any(r["sequence"] == identity[1] for r in d["releases"]) for d in output_distributions), f"published release {identity} was removed from canonical source")

    candidate_source = {**source, "distributions": output_distributions}
    validate_reproduced_bridges(
        candidate_source, repository_root, config["repository"], previous,
    )
    validate_signing_boundary_packages(
        candidate_source, previous, repository_root, config["repository"], overrides,
    )
    evidence = selected_evidence(candidate_source, set(distributions_by_id), config, overrides)
    validate_upstream_default_evidence(products, output_distributions, evidence)
    revocations = [
        {"distribution_id": distribution["id"], "release_sequence": policy["release_sequence"]}
        for distribution in output_distributions for policy in distribution["release_policies"]
        if policy["status"] == "revoked"
    ]
    return {
        "candidate_schema_version": 1,
        "snapshot_schema_version": 1,
        "publication_id": publication_id,
        "source_commit": source_commit,
        "lifetime_days": config["snapshot_lifetime_days"],
        "products": products,
        "distributions": output_distributions,
        "evidence": evidence,
        "revocations": revocations,
    }


def parse_overrides(values: list[str]) -> dict[str, Path]:
    result: dict[str, Path] = {}
    for value in values:
        repository, separator, raw_path = value.partition("=")
        require(bool(separator and repository and raw_path), "--external-repository must be REPOSITORY=LOCAL_GIT_REPOSITORY")
        require(repository not in result, f"duplicate external repository override {repository}")
        result[repository] = Path(raw_path).resolve()
    return result


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--directory", type=Path, required=True)
    parser.add_argument("--config", type=Path, required=True)
    parser.add_argument("--source-commit", required=True)
    parser.add_argument("--source-tree-commit")
    parser.add_argument("--publication-id", required=True)
    parser.add_argument("--ledger", type=Path)
    parser.add_argument("--trusted-keys", type=Path)
    parser.add_argument("--initialize-ledger", action="store_true")
    parser.add_argument("--ledger-seed-commit")
    parser.add_argument("--ledger-sequence-floor", type=int)
    parser.add_argument("--external-repository", action="append", default=[])
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--digest-output", type=Path, required=True)
    args = parser.parse_args()
    try:
        resolved_head = subprocess.check_output([GIT, "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()
        checked_out_source = args.source_tree_commit or args.source_commit
        mismatch_label = "source tree commit" if args.source_tree_commit else "--source-commit"
        require(resolved_head == checked_out_source, f"checked-out HEAD {resolved_head} does not match {mismatch_label} {checked_out_source}")
        if args.source_tree_commit:
            validate_marker(ROOT, args.source_commit, args.source_tree_commit, args.publication_id)
        previous = None
        if args.ledger:
            require(args.trusted_keys is not None, "--trusted-keys is required with --ledger")
            loaded = load_ledger_latest(
                args.ledger, load_public_keys(args.trusted_keys),
                allow_initialization=args.initialize_ledger,
                seed_commit=args.ledger_seed_commit,
                minimum_sequence=args.ledger_sequence_floor,
                require_external_floor=True,
            )
            previous = loaded[0] if loaded else None
        candidate = build_candidate(
            read_json(args.directory), load_config(args.config), args.source_commit,
            args.publication_id, previous, external_overrides=parse_overrides(args.external_repository),
        )
        validate_with_schema(candidate, CANDIDATE_SCHEMA)
        body = canonical_json(candidate)
        digest = candidate_digest(body)
        atomic_write(args.output, body)
        atomic_write(args.digest_output, (digest + "\n").encode("ascii"))
    except (OSError, CasError, PublicationError, KeyError, TypeError, subprocess.SubprocessError) as error:
        print(f"prepare-directory-publication: {error}", file=sys.stderr)
        return 1
    print(digest)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
