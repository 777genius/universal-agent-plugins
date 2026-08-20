#!/usr/bin/env python3
"""Build one bounded canonical no-secret Directory publication candidate."""

from __future__ import annotations

import argparse
import copy
import hashlib
import subprocess
import sys
from pathlib import Path
from typing import Any

sys.path.insert(0, str(Path(__file__).resolve().parent))
from build_agentplugins_catalog import package_tree_digest
from directory_publication import (
    CANDIDATE_SCHEMA,
    DIST_ID_RE,
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
CONFIG_FIELDS = {
    "schema_version", "repository", "publisher_namespace", "snapshot_lifetime_days",
    "release_sequences", "distribution_status", "release_status", "current_evidence",
}


def load_config(path: Path) -> dict[str, Any]:
    value = read_json(path, max_bytes=256 << 10)
    require(isinstance(value, dict) and set(value) == CONFIG_FIELDS, f"{path}: invalid publication config fields")
    require(value["schema_version"] == 1, f"{path}: schema_version must be 1")
    require(isinstance(value["repository"], str) and "/" in value["repository"], f"{path}: invalid repository")
    namespace = value["publisher_namespace"]
    require(isinstance(namespace, str) and namespace.isascii() and namespace.islower(), f"{path}: invalid publisher namespace")
    require(isinstance(value["snapshot_lifetime_days"], int) and 1 <= value["snapshot_lifetime_days"] <= 30, f"{path}: invalid lifetime")
    for field in ("release_sequences", "distribution_status"):
        require(isinstance(value[field], dict), f"{path}: {field} must be an object")
        for distribution_id in value[field]:
            require(DIST_ID_RE.fullmatch(distribution_id) is not None, f"{path}: invalid distribution ID {distribution_id}")
    release_key = __import__("re").compile(r"^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?/[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?@[1-9][0-9]*$")
    for field in ("release_status", "current_evidence"):
        require(isinstance(value[field], dict), f"{path}: {field} must be an object")
        for identity in value[field]:
            require(release_key.fullmatch(identity) is not None, f"{path}: invalid release identity {identity}")
    for sequence in value["release_sequences"].values():
        require(isinstance(sequence, int) and sequence >= 1, f"{path}: release sequence must be positive")
    for status in value["distribution_status"].values():
        require(status in ("active", "suspended"), f"{path}: invalid distribution status")
    for status in value["release_status"].values():
        require(status in ("active", "superseded", "revoked"), f"{path}: invalid release status")
    return value


def immutable_payload(release: dict[str, Any]) -> dict[str, Any]:
    return {
        key: release[key]
        for key in ("distribution_id", "sequence", "package_version", "tree_digest", "manifest_digest", "components")
    }


def previous_releases(snapshot: dict[str, Any] | None) -> dict[tuple[str, int], dict[str, Any]]:
    result: dict[tuple[str, int], dict[str, Any]] = {}
    if snapshot is None:
        return result
    for product in snapshot["products"]:
        for distribution in product["distributions"]:
            for release in distribution["releases"]:
                result[(distribution["id"], release["sequence"])] = release
    return result


def release_override(config: dict[str, Any], field: str, distribution_id: str, sequence: int, default: Any) -> Any:
    return config[field].get(f"{distribution_id}@{sequence}", default)


def build_candidate(
    index: dict[str, Any], config: dict[str, Any], source_commit: str,
    publication_id: str, previous: dict[str, Any] | None,
) -> dict[str, Any]:
    require(index.get("schema_version") == 1 and isinstance(index.get("plugins"), list), "registry index is invalid")
    require(SHA_RE.fullmatch(source_commit) is not None, "source commit must be a full lowercase SHA")
    prior = previous_releases(previous)
    products: list[dict[str, Any]] = []
    seen: set[str] = set()
    for plugin in sorted(index["plugins"], key=lambda item: item["name"]):
        name = plugin["name"]
        require(name not in seen, f"duplicate product {name}")
        seen.add(name)
        distribution_id = f"{config['publisher_namespace']}/{name}"
        sequence = config["release_sequences"].get(distribution_id, 1)
        source = plugin["source"]
        package_root = ROOT / source["path"]
        require(plugin["built_in"] is True, f"{name}: external releases require a dedicated reacquisition input")
        require(package_root.is_dir(), f"{name}: package path is missing")
        actual_tree = package_tree_digest(package_root)
        actual_manifest = "sha256:" + hashlib.sha256((package_root / "plugin.json").read_bytes()).hexdigest()
        require(actual_tree == source["tree_sha256"], f"{name}: post-merge tree digest differs from reviewed digest")
        require(actual_manifest == source["manifest_sha256"], f"{name}: post-merge manifest digest differs from reviewed digest")
        release: dict[str, Any] = {
            "distribution_id": distribution_id,
            "sequence": sequence,
            "package_version": plugin.get("version"),
            "package_source": {
                "repository": config["repository"],
                "revision": source_commit,
                "path": source["path"],
            },
            "tree_digest": actual_tree,
            "manifest_digest": actual_manifest,
            "components": sorted(plugin["components"]),
            "published_at": None,
            "policy": {
                "release_sequence": sequence,
                "status": release_override(config, "release_status", distribution_id, sequence, "active"),
                "compatible_clients": sorted(plugin["client_support"]["clients"]),
                "scopes": ["user"],
                "minimum_installer_version": "0.1.6",
                "current_evidence": release_override(config, "current_evidence", distribution_id, sequence, []),
            },
        }
        prior_for_distribution = sorted(
            (copy.deepcopy(item) for (dist, _), item in prior.items() if dist == distribution_id),
            key=lambda item: item["sequence"],
        )
        if prior_for_distribution:
            highest = prior_for_distribution[-1]["sequence"]
            require(sequence in (highest, highest + 1), f"{distribution_id}: next release sequence must be {highest + 1} (or {highest} for unchanged bytes)")
            if sequence == highest + 1:
                last = prior_for_distribution[-1]
                unchanged_bytes = all(
                    last[field] == release[field]
                    for field in ("package_version", "tree_digest", "manifest_digest", "components")
                )
                require(not unchanged_bytes, f"{distribution_id}: unchanged package bytes must reuse release {highest}; publish policy/evidence only")
        old = prior.get((distribution_id, sequence))
        if old is not None:
            require(immutable_payload(old) == immutable_payload(release), f"{distribution_id} release {sequence}: immutable bytes changed; allocate a new release sequence")
            release["package_source"] = old["package_source"]
            release["published_at"] = old["published_at"]
            if old["policy"]["status"] == "revoked":
                require(release["policy"]["status"] == "revoked", f"{distribution_id} release {sequence}: revocation is terminal")
        historical: list[dict[str, Any]] = []
        for old_release in prior_for_distribution:
            if old_release["sequence"] == sequence:
                continue
            identity = f"{distribution_id}@{old_release['sequence']}"
            status = config["release_status"].get(identity, old_release["policy"]["status"])
            if old_release["policy"]["status"] == "revoked":
                require(status == "revoked", f"{identity}: revocation is terminal")
            old_release["policy"]["status"] = status
            if identity in config["current_evidence"]:
                old_release["policy"]["current_evidence"] = config["current_evidence"][identity]
            historical.append(old_release)
        product = {
            "id": name,
            "display_name": name.replace("-", " ").title(),
            "manifest_name": name,
            "aliases": [name],
            "default_distribution": distribution_id,
            "distributions": [{
                "id": distribution_id,
                "kind": "community",
                "status": config["distribution_status"].get(distribution_id, "active"),
                "releases": sorted([*historical, release], key=lambda item: item["sequence"]),
            }],
        }
        products.append(product)
    return {
        "candidate_schema_version": 1,
        "snapshot_schema_version": 1,
        "publication_id": publication_id,
        "source_commit": source_commit,
        "lifetime_days": config["snapshot_lifetime_days"],
        "products": products,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--index", type=Path, required=True)
    parser.add_argument("--config", type=Path, required=True)
    parser.add_argument("--source-commit", required=True)
    parser.add_argument("--publication-id", required=True)
    parser.add_argument("--ledger", type=Path)
    parser.add_argument("--trusted-keys", type=Path)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--digest-output", type=Path, required=True)
    args = parser.parse_args()
    try:
        resolved_head = subprocess.check_output(
            ["git", "rev-parse", "HEAD"], cwd=ROOT, text=True
        ).strip()
        require(
            resolved_head == args.source_commit,
            f"checked-out HEAD {resolved_head} does not match --source-commit {args.source_commit}",
        )
        previous = None
        if args.ledger:
            require(args.trusted_keys is not None, "--trusted-keys is required with --ledger")
            loaded = load_ledger_latest(args.ledger, load_public_keys(args.trusted_keys))
            previous = loaded[0] if loaded else None
        candidate = build_candidate(
            read_json(args.index), load_config(args.config), args.source_commit,
            args.publication_id, previous,
        )
        validate_with_schema(candidate, CANDIDATE_SCHEMA)
        body = canonical_json(candidate)
        digest = candidate_digest(body)
        atomic_write(args.output, body)
        atomic_write(args.digest_output, (digest + "\n").encode("ascii"))
    except (OSError, PublicationError, KeyError, TypeError) as error:
        print(f"prepare-directory-publication: {error}", file=sys.stderr)
        return 1
    print(digest)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
