#!/usr/bin/env python3
"""Validate canonical Directory source and build a no-secret publication candidate."""

from __future__ import annotations

import argparse
import copy
import hashlib
import subprocess
import sys
import tempfile
from pathlib import Path
from typing import Any

sys.path.insert(0, str(Path(__file__).resolve().parent))
from build_agentplugins_catalog import package_tree_digest
from directory_publication import (
    CANDIDATE_SCHEMA,
    PublicationError,
    SHA_RE,
    atomic_write,
    candidate_digest,
    canonical_json,
    load_ledger_latest,
    load_public_keys,
    read_json,
    require,
    validate_with_schema,
)


ROOT = Path(__file__).resolve().parents[1]
SOURCE_SCHEMA = ROOT / "schemas" / "directory-source.schema.json"
CONFIG_FIELDS = {"schema_version", "repository", "snapshot_lifetime_days"}


def load_config(path: Path) -> dict[str, Any]:
    value = read_json(path, max_bytes=64 << 10)
    require(isinstance(value, dict) and set(value) == CONFIG_FIELDS, f"{path}: invalid publication config fields")
    require(value["schema_version"] == 1, f"{path}: schema_version must be 1")
    require(isinstance(value["repository"], str) and "/" in value["repository"], f"{path}: invalid repository")
    require(isinstance(value["snapshot_lifetime_days"], int) and 1 <= value["snapshot_lifetime_days"] <= 30, f"{path}: invalid lifetime")
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


def target_keys(policy: dict[str, Any]) -> set[tuple[str, str]]:
    return {(target["client"], scope) for target in policy["targets"] for scope in target["scopes"]}


def eligibility_broadened(
    distribution: dict[str, Any], policy: dict[str, Any],
    old_distribution: dict[str, Any] | None, old_policy: dict[str, Any] | None,
) -> bool:
    if old_distribution is None or old_policy is None:
        return True
    if distribution["kind"] != old_distribution["kind"] or distribution["packager"] != old_distribution["packager"]:
        return True
    if old_distribution["status"] != "active" and distribution["status"] == "active":
        return True
    if old_policy["status"] != "active" and policy["status"] == "active":
        return True
    if not target_keys(policy).issubset(target_keys(old_policy)):
        return True
    if any(target not in old_policy["targets"] for target in policy["targets"]):
        return True
    old_minimum = tuple(int(part) for part in old_policy["minimum_installer_version"].split("."))
    new_minimum = tuple(int(part) for part in policy["minimum_installer_version"].split("."))
    if new_minimum < old_minimum:
        return True
    return policy["current_evidence"] != old_policy["current_evidence"]


def manifest_digest(package_root: Path) -> str:
    manifest = package_root / "plugin.json"
    require(manifest.is_file(), f"{package_root}: plugin.json is missing")
    return "sha256:" + hashlib.sha256(manifest.read_bytes()).hexdigest()


def verify_package(package_root: Path, release: dict[str, Any], identity: str) -> None:
    require(package_root.is_dir(), f"{identity}: package path is unavailable")
    actual_tree = package_tree_digest(package_root)
    actual_manifest = manifest_digest(package_root)
    require(actual_tree == release["tree_digest"], f"{identity}: reacquired tree digest differs from reviewed digest")
    require(actual_manifest == release["manifest_digest"], f"{identity}: reacquired manifest digest differs from reviewed digest")


def acquire_external(repository: str, revision: str, package_path: str, override: Path | None = None) -> tempfile.TemporaryDirectory[str]:
    temporary = tempfile.TemporaryDirectory(prefix="directory-publication-")
    checkout = Path(temporary.name) / "checkout"
    source = str(override) if override is not None else f"https://github.com/{repository}.git"
    try:
        subprocess.run(["git", "init", "--quiet", str(checkout)], check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        subprocess.run(
            ["git", "-C", str(checkout), "-c", "protocol.file.allow=always", "fetch", "--quiet", "--depth=1", source, revision],
            check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=180,
        )
        subprocess.run(["git", "-C", str(checkout), "checkout", "--quiet", "--detach", "FETCH_HEAD"], check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        resolved = subprocess.check_output(["git", "-C", str(checkout), "rev-parse", "HEAD"], text=True).strip()
        require(resolved == revision, f"{repository}@{revision}: reacquisition resolved {resolved}")
        require((checkout / package_path).is_dir(), f"{repository}@{revision}//{package_path}: package path is unavailable")
        return temporary
    except (OSError, subprocess.SubprocessError) as error:
        temporary.cleanup()
        raise PublicationError(f"{repository}@{revision}//{package_path}: reacquisition failed: {error}") from error


def selected_evidence(source: dict[str, Any], published_distributions: set[str]) -> list[dict[str, Any]]:
    evidence = {item["id"]: item for item in source["evidence"]}
    require(len(evidence) == len(source["evidence"]), "duplicate evidence identity")
    selected: set[str] = set()
    release_digests = {
        (distribution["id"], release["sequence"]): release["tree_digest"]
        for distribution in source["distributions"] for release in distribution["releases"]
    }
    for distribution in source["distributions"]:
        if distribution["id"] not in published_distributions:
            continue
        for policy in distribution["release_policies"]:
            identity = (distribution["id"], policy["release_sequence"])
            require(identity in release_digests, f"{distribution['id']}: policy references missing release {policy['release_sequence']}")
            for evidence_id in policy["current_evidence"]:
                require(evidence_id in evidence, f"{distribution['id']}@{policy['release_sequence']}: missing evidence {evidence_id}")
                record = evidence[evidence_id]
                require((record["distribution_id"], record["release_sequence"]) == identity, f"{evidence_id}: evidence release identity mismatch")
                require(record["package_tree_digest"] == release_digests[identity], f"{evidence_id}: evidence package digest mismatch")
                selected.add(evidence_id)
    return [copy.deepcopy(evidence[item]) for item in sorted(selected)]


def build_candidate(
    source: dict[str, Any], config: dict[str, Any], source_commit: str,
    publication_id: str, previous: dict[str, Any] | None,
    *, repository_root: Path = ROOT,
    external_overrides: dict[str, Path] | None = None,
) -> dict[str, Any]:
    validate_with_schema(source, SOURCE_SCHEMA)
    require(SHA_RE.fullmatch(source_commit) is not None, "source commit must be a full lowercase SHA")
    prior = previous_releases(previous)
    prior_distributions = previous_distributions(previous)
    all_distributions = {item["id"]: item for item in source["distributions"]}
    require(len(all_distributions) == len(source["distributions"]), "duplicate distribution identity")
    distributions_by_id = {identity: item for identity, item in all_distributions.items() if item["status"] != "candidate"}
    products = sorted((copy.deepcopy(item) for item in source["products"]), key=lambda item: item["id"])
    aliases: set[str] = set()
    referenced_distributions: set[str] = set()
    for product in products:
        for alias in product["aliases"]:
            require(alias not in aliases, f"duplicate active alias {alias}")
            aliases.add(alias)
        referenced_distributions.update(product["distributions"])
        require(all(item in all_distributions for item in product["distributions"]), f"{product['id']}: references an unknown distribution")
        product["distributions"] = [item for item in product["distributions"] if item in distributions_by_id]
        require(product["default_distribution"] in product["distributions"], f"{product['id']}: default distribution is not listed")
        for distribution_id in product["distributions"]:
            require(distribution_id in distributions_by_id, f"{product['id']}: missing distribution {distribution_id}")
            require(distributions_by_id[distribution_id]["product_id"] == product["id"], f"{product['id']}: mismatched distribution {distribution_id}")
        default = distributions_by_id[product["default_distribution"]]
        require(default["status"] == "active", f"{product['id']}: default distribution must be active")
        active_sequences = {policy["release_sequence"] for policy in default["release_policies"] if policy["status"] == "active"}
        active_releases = [release for release in default["releases"] if release["sequence"] in active_sequences]
        require(active_releases, f"{product['id']}: default distribution has no active release")
        require(any(
            (product["minimum_capabilities"]["skills"] != "required" or "skills" in release["components"])
            and (product["minimum_capabilities"]["mcp"] != "required" or "mcp" in release["components"])
            for release in active_releases
        ), f"{product['id']}: default distribution does not satisfy minimum capabilities")
    require(referenced_distributions == set(all_distributions), "every distribution must be owned by exactly one product")

    output_distributions: list[dict[str, Any]] = []
    overrides = external_overrides or {}
    for original in sorted(distributions_by_id.values(), key=lambda item: item["id"]):
        distribution = copy.deepcopy(original)
        policies = policy_map(distribution)
        require(len(policies) == len(distribution["release_policies"]), f"{distribution['id']}: duplicate release policy")
        require(set(policies) == {release["sequence"] for release in distribution["releases"]}, f"{distribution['id']}: releases and policies must be one-to-one")
        old_distribution = prior_distributions.get(distribution["id"])
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
                release["published_at"] = None
                if in_repository:
                    # Review source cannot author this binding.  Null and stale/guessed
                    # values are both replaced only after the checked-out merge tree
                    # passes the reviewed digest checks below.
                    verify_package(repository_root / package_source["path"], release, label)
                    package_source["revision"] = source_commit
                else:
                    revision = package_source["revision"]
                    require(isinstance(revision, str) and SHA_RE.fullmatch(revision) is not None, f"{label}: external source requires reviewed full SHA")
            reacquire = not in_repository and (
                old is None or eligibility_broadened(distribution, policies[release["sequence"]], old_distribution, old_policies.get(release["sequence"]))
            )
            if reacquire:
                temporary = acquire_external(package_source["repository"], package_source["revision"], package_source["path"], overrides.get(package_source["repository"]))
                try:
                    verify_package(Path(temporary.name) / "checkout" / package_source["path"], release, label)
                finally:
                    temporary.cleanup()
        distribution["releases"].sort(key=lambda item: item["sequence"])
        distribution["release_policies"].sort(key=lambda item: item["release_sequence"])
        output_distributions.append(distribution)

    # Published releases are append-only even if review source accidentally drops one.
    for identity in prior:
        require(any(d["id"] == identity[0] and any(r["sequence"] == identity[1] for r in d["releases"]) for d in output_distributions), f"published release {identity} was removed from canonical source")

    evidence = selected_evidence(source, set(distributions_by_id))
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
    parser.add_argument("--publication-id", required=True)
    parser.add_argument("--ledger", type=Path)
    parser.add_argument("--trusted-keys", type=Path)
    parser.add_argument("--external-repository", action="append", default=[])
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--digest-output", type=Path, required=True)
    args = parser.parse_args()
    try:
        resolved_head = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()
        require(resolved_head == args.source_commit, f"checked-out HEAD {resolved_head} does not match --source-commit {args.source_commit}")
        previous = None
        if args.ledger:
            require(args.trusted_keys is not None, "--trusted-keys is required with --ledger")
            loaded = load_ledger_latest(args.ledger, load_public_keys(args.trusted_keys))
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
    except (OSError, PublicationError, KeyError, TypeError, subprocess.SubprocessError) as error:
        print(f"prepare-directory-publication: {error}", file=sys.stderr)
        return 1
    print(digest)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
