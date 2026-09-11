#!/usr/bin/env python3
"""Read-only, point-in-time verification of a frozen Agentplugins draft.

GitHub reads and attestation verification use gh; asset policy stays in the
existing release-assets.js verifier. No release mutation or installer execution.
"""
import argparse
import datetime
import hashlib
import json
from pathlib import Path
import re
import subprocess
import tempfile

# GitHub API and attestation identity after the repository rename.
REPOSITORY = "777genius/universal-agent-plugins"
# release-assets.js preserves this producer identity for the asset contract.
# It is not an alternate API repository or an accepted attestation signer.
ASSET_PRODUCER_REPOSITORY = "777genius/plugin-kit-ai"
WORKFLOW = ".github/workflows/agentplugins-release.yml"


def run(args, **kwargs):
    return subprocess.run(args, check=True, stdout=subprocess.PIPE, **kwargs).stdout


def api(endpoint, *args):
    return json.loads(run(["gh", "api", endpoint, *args]))


def require(condition, message):
    if not condition:
        raise ValueError(message)


def snapshot(repository, tag, commit, release_id):
    owner, name = repository.split("/")
    query = ("query($owner:String!,$name:String!,$tag:String!){"
             "repository(owner:$owner,name:$name){release(tagName:$tag){databaseId}}}")
    lookup = api("graphql", "-f", f"query={query}", "-F", f"owner={owner}",
                 "-F", f"name={name}", "-F", f"tag={tag}")
    require(not lookup.get("errors"), "release lookup failed")
    release = lookup["data"]["repository"]["release"]
    require(release is not None and release["databaseId"] == release_id,
            "release missing or recreated")
    prefix = f"repos/{repository}"
    metadata = api(f"{prefix}/releases/{release_id}")
    require(metadata["id"] == release_id and metadata["tag_name"] == tag
            and metadata["draft"] is True and metadata["prerelease"] is False,
            "release is not the exact non-prerelease draft")
    require(api(f"{prefix}/commits/{tag}")["sha"] == commit, "tag moved")
    pages = api(f"{prefix}/releases/{release_id}/assets?per_page=100",
                "--paginate", "--slurp")
    assets = [asset for page in pages for asset in page]
    names, ids = set(), set()
    for asset in assets:
        require(isinstance(asset["name"], str)
                and re.fullmatch(r"[A-Za-z0-9_.-]+", asset["name"])
                and asset["name"] not in {".", ".."}
                and asset["name"] not in names, "invalid or duplicate asset name")
        require(type(asset["id"]) is int and asset["id"] > 0
                and asset["id"] not in ids and asset["state"] == "uploaded"
                and type(asset["size"]) is int and asset["size"] > 0,
                "invalid or duplicate asset identity")
        names.add(asset["name"])
        ids.add(asset["id"])
    # Include replacement-sensitive identities, timestamps and any server digest.
    assets = sorted(({key: asset.get(key) for key in
                      ("id", "name", "size", "state", "created_at", "updated_at", "digest")}
                     for asset in assets), key=lambda asset: asset["name"])
    return {"id": release_id, "tag": tag, "commit": commit, "draft": True,
            "prerelease": False, "updated_at": metadata["updated_at"], "assets": assets}


def sha256(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def verify(args):
    require(args.repository == REPOSITORY, "unexpected producer repository")
    require(re.fullmatch(r"agentplugins-v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)",
                         args.tag), "invalid stable tag")
    require(re.fullmatch(r"[0-9a-f]{40}", args.commit), "invalid frozen commit")
    require(re.fullmatch(r"[0-9a-f]{64}", args.asset_set_digest), "invalid frozen digest")
    require(args.release_id > 0 and args.run_id > 0 and args.run_attempt > 0,
            "invalid release or run identity")
    receipt = Path(args.receipt)
    require(not receipt.exists(), "receipt path already exists")
    source = Path(args.source).resolve()
    policy = Path(getattr(args, 'policy_source', None) or source).resolve()
    if getattr(args, 'policy_source', None):
        require(run(['git', '-C', str(source), 'rev-parse', 'HEAD']).decode().strip() == args.commit,
                'wrong producer checkout')
        require(not run(['git', '-C', str(source), 'status', '--porcelain', '--untracked-files=normal']),
                'dirty producer checkout')
    before = snapshot(args.repository, args.tag, args.commit, args.release_id)
    with tempfile.TemporaryDirectory(prefix="agentplugins-draft-") as directory:
        root = Path(directory)
        for asset in before["assets"]:
            data = run(["gh", "api", f"repos/{args.repository}/releases/assets/{asset['id']}",
                        "-H", "Accept: application/octet-stream"])
            require(len(data) == asset["size"], "download size mismatch")
            (root / asset["name"]).write_bytes(data)
        verified = json.loads(run([
            "node", str(policy / "npm/agentplugins/scripts/release-assets.js"),
            "verify", str(root), args.tag, args.commit]))
        require(verified["gate_eligible"] is True
                and verified["repository"] == ASSET_PRODUCER_REPOSITORY,
                "ineligible release assets")
        require(sha256(root / "checksums.txt") == args.asset_set_digest,
                "frozen checksums digest mismatch")
        require((root / "THIRD_PARTY_NOTICES.txt").read_bytes() ==
                (source / "npm/agentplugins/THIRD_PARTY_NOTICES.txt").read_bytes(),
                "source notices mismatch")
        subjects = []
        signer = f"github.com/{args.repository}/{WORKFLOW}"
        for asset in before["assets"]:
            path = root / asset["name"]
            digest = sha256(path)
            require(asset["digest"] in (None, f"sha256:{digest}"), "server digest mismatch")
            run(["gh", "attestation", "verify", str(path), "--repo", args.repository,
                 "--signer-workflow", signer, "--source-digest", args.commit])
            subjects.append({"name": asset["name"], "id": asset["id"],
                             "size": asset["size"], "sha256": digest})
        after = snapshot(args.repository, args.tag, args.commit, args.release_id)
        require(before == after, "release metadata changed during verification")
    result = {"schema_version": 1, "release_state": "verified-draft",
              "verified_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
              "repository": args.repository, "release": after,
              "asset_set_digest": args.asset_set_digest, "subjects": subjects,
              "signer_workflow": signer, "producer_commit": args.commit,
              "producer_run_id": args.run_id, "producer_run_attempt": args.run_attempt}
    # Exclusive creation: failure never leaves a success receipt from this invocation.
    with receipt.open("x", encoding="utf-8") as output:
        output.write(json.dumps(result, indent=2) + "\n")
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("repository", "tag", "commit", "asset-set-digest", "source", "receipt"):
        parser.add_argument(f"--{name}", required=True)
    for name in ("release-id", "run-id", "run-attempt"):
        parser.add_argument(f"--{name}", required=True, type=int)
    parser.add_argument("--policy-source", help="trusted verifier checkout; source remains producer data")
    verify(parser.parse_args())


if __name__ == "__main__":
    main()
