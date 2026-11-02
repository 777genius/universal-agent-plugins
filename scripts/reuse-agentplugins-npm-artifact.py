#!/usr/bin/env python3
"""Reuse the exact npm tarball authenticated by a completed release qualification.

The caller independently verifies public binary assets and attestations first.
This helper reads GitHub state and writes only the authenticated npm package.
"""
import argparse
import base64
import datetime
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import re
import stat
import zipfile

HERE = Path(__file__).resolve().parent
SPEC = importlib.util.spec_from_file_location("qualified_draft", HERE / "reuse-agentplugins-verified-draft.py")
reuse = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(reuse)
require = reuse.require
api = reuse.api
output = reuse.output
PACKAGE = "universal-agent-plugins"
MAX_ARCHIVE = 512 << 20
MAX_EXPANDED = 1 << 30
ARTIFACT_KEYS = ("id", "name", "digest", "size_in_bytes", "created_at", "workflow_run")


def identity(repository, tag, commit, run_id, run_attempt, tarball_file):
    require(repository == reuse.REPOSITORY, "unexpected repository")
    require(re.fullmatch(r"agentplugins-v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)", tag),
            "invalid stable tag")
    require(re.fullmatch(r"[0-9a-f]{40}", commit), "invalid frozen commit")
    require(type(run_id) is int and run_id > 0 and type(run_attempt) is int and run_attempt > 0,
            "invalid qualification identity")
    version = tag.removeprefix("agentplugins-v")
    require(tarball_file == f"{PACKAGE}-{version}.tgz", "unexpected npm tarball filename")
    return version


def timestamp(value):
    parsed = datetime.datetime.fromisoformat(value.replace("Z", "+00:00"))
    require(parsed.tzinfo is not None, "artifact timestamp lacks timezone")
    return parsed


def artifact_snapshot(repository, tag, commit, run_id, run_attempt, tarball_file):
    version = identity(repository, tag, commit, run_id, run_attempt, tarball_file)
    prefix = f"repos/{repository}/actions"
    run = api(f"{prefix}/runs/{run_id}")
    require(run.get("id") == run_id and run.get("run_attempt") == run_attempt
            and run.get("repository", {}).get("full_name") == repository
            and run.get("head_repository", {}).get("full_name") == repository
            and run.get("head_sha") == commit and run.get("head_branch") in ("main", tag)
            and run.get("path") == reuse.WORKFLOW and run.get("event") == "workflow_dispatch",
            "npm artifact qualification run identity mismatch")
    if run.get("status") == "completed":
        require(run.get("conclusion") == "success", "npm qualification did not succeed")
        latest = timestamp(run["updated_at"])
    else:
        # Receipt creation runs within verified-draft, before its run completes.
        require(run.get("status") == "in_progress", "npm qualification is not running")
        latest = datetime.datetime.now(datetime.timezone.utc)
    pages = api(f"{prefix}/runs/{run_id}/artifacts?per_page=100", "--paginate", "--slurp")
    name = f"agentplugins-npm-{version}"
    matches = [artifact for page in pages for artifact in page["artifacts"]
               if artifact.get("name") == name]
    require(len(matches) == 1, "missing or duplicate qualified npm artifact")
    artifact = matches[0]
    require(type(artifact.get("id")) is int and artifact["id"] > 0
            and artifact.get("expired") is False
            and type(artifact.get("size_in_bytes")) is int
            and 0 < artifact["size_in_bytes"] <= MAX_ARCHIVE
            and re.fullmatch(r"sha256:[0-9a-f]{64}", artifact.get("digest", ""))
            and artifact.get("workflow_run", {}).get("id") == run_id
            and artifact["workflow_run"].get("head_sha") == commit
            and timestamp(run["run_started_at"]) <= timestamp(artifact["created_at"]) <= latest,
            "npm artifact identity is invalid")
    return {key: artifact[key] for key in ARTIFACT_KEYS}


def download_tarball(repository, artifact, version, tarball_file):
    body = output(["gh", "api", f"repos/{repository}/actions/artifacts/{artifact['id']}/zip"])
    require(len(body) == artifact["size_in_bytes"] and len(body) <= MAX_ARCHIVE
            and hashlib.sha256(body).hexdigest() == artifact["digest"][7:],
            "npm artifact ZIP digest mismatch")
    binaries = {f"agentplugins_{version}_{target.replace('-', '_')}"
                + (".exe" if target.startswith("windows-") else "") for target in reuse.TARGETS}
    allowed = {tarball_file, "verified-release.json", "npm-package.json", "release-assets/"}
    allowed.update("release-assets/" + name for name in
                   binaries | {"checksums.txt", "release-manifest.json", "THIRD_PARTY_NOTICES.txt"})
    with zipfile.ZipFile(io.BytesIO(body)) as archive:
        members = archive.infolist()
        names = [member.filename for member in members]
        require(len(members) <= len(allowed) and len(names) == len(set(names))
                and {tarball_file, "verified-release.json"} <= set(names)
                and set(names) <= allowed, "unexpected npm artifact ZIP contents")
        require(sum(member.file_size for member in members) <= MAX_EXPANDED,
                "npm artifact ZIP expands beyond limit")
        for member in members:
            kind = stat.S_IFMT(member.external_attr >> 16)
            require(member.orig_filename == member.filename and not member.flag_bits & 1
                    and 0 <= member.file_size <= MAX_ARCHIVE
                    and member.compress_type in (zipfile.ZIP_STORED, zipfile.ZIP_DEFLATED)
                    and ((member.filename == "release-assets/" and member.is_dir()
                          and kind in (0, stat.S_IFDIR) and member.file_size == 0)
                         or (not member.is_dir() and kind in (0, stat.S_IFREG))),
                    "unsafe npm artifact ZIP member")
        tarball = archive.read(tarball_file)
    require(0 < len(tarball) <= MAX_ARCHIVE, "empty or oversized npm tarball")
    return tarball


def hashes(tarball):
    return {"integrity": "sha512-" + base64.b64encode(hashlib.sha512(tarball).digest()).decode("ascii"),
            "shasum": hashlib.sha1(tarball).hexdigest()}


def capture_npm_artifact(repository, tag, commit, run_id, run_attempt, tarball_file):
    """Capture immutable artifact identity and exact npm hashes for the receipt."""
    artifact = artifact_snapshot(repository, tag, commit, run_id, run_attempt, tarball_file)
    version = tag.removeprefix("agentplugins-v")
    tarball = download_tarball(repository, artifact, version, tarball_file)
    return {**artifact, "tarball_file": tarball_file, **hashes(tarball)}


def require_public_release(args, receipt):
    frozen = receipt["release"]
    prefix = f"repos/{args.repository}"
    live = api(f"{prefix}/releases/tags/{args.tag}")
    require(live.get("id") == frozen["id"] and live.get("tag_name") == args.tag
            and live.get("draft") is False and live.get("prerelease") is False
            and frozen.get("tag") == args.tag and frozen.get("commit") == args.commit
            and api(f"{prefix}/commits/{args.tag}").get("sha") == args.commit,
            "public release differs from qualified release")
    pages = api(f"{prefix}/releases/{frozen['id']}/assets?per_page=100", "--paginate", "--slurp")
    assets = [asset for page in pages for asset in page]
    keys = ("id", "name", "size", "state", "digest")
    def normalized(items):
        require(len({item["name"] for item in items}) == len(items)
                and len({item["id"] for item in items}) == len(items), "duplicate release asset")
        return sorted(({key: item.get(key) for key in keys} for item in items), key=lambda item: item["name"])
    require(normalized(assets) == normalized(frozen["assets"]),
            "public release assets changed since qualification")
    subjects = {subject["name"]: subject for subject in receipt["subjects"]}
    require(len(subjects) == len(receipt["subjects"]) == len(assets), "invalid qualified subjects")
    for asset in assets:
        subject = subjects.get(asset["name"], {})
        digest = subject.get("sha256", "")
        require(subject.get("id") == asset["id"] and subject.get("size") == asset["size"]
                and asset.get("state") == "uploaded" and re.fullmatch(r"[0-9a-f]{64}", digest)
                and asset.get("digest") in (None, "sha256:" + digest),
                "public release subject changed since qualification")


def verify(args):
    receipt = reuse.authenticate(args)
    require(receipt.get("schema_version") == 2 and isinstance(receipt.get("npm_artifact"), dict),
            "qualification receipt lacks exact npm artifact binding")
    frozen = receipt["npm_artifact"]
    version = identity(args.repository, args.tag, args.commit, args.run_id, args.run_attempt,
                       frozen.get("tarball_file"))
    source = Path(args.source).resolve()
    require(output(["git", "-C", str(source), "rev-parse", "HEAD"]).decode().strip() == args.commit
            and not output(["git", "-C", str(source), "status", "--porcelain", "--untracked-files=normal"])
            and output(["git", "-C", str(source), "rev-list", "-n", "1", f"refs/tags/{args.tag}"]).decode().strip() == args.commit,
            "npm source checkout differs from frozen commit")
    destination = Path(args.output_dir)
    require(not destination.is_symlink() and destination.is_dir(), "output directory must exist")
    mode = destination.stat()
    require(mode.st_uid == os.getuid() and not mode.st_mode & 0o077 and not any(destination.iterdir()),
            "output directory must be private and empty")
    artifact = artifact_snapshot(args.repository, args.tag, args.commit, args.run_id,
                                 args.run_attempt, frozen["tarball_file"])
    require(all(artifact[key] == frozen.get(key) for key in ARTIFACT_KEYS),
            "qualified npm artifact was replaced")
    require_public_release(args, receipt)
    tarball = download_tarball(args.repository, artifact, version, frozen["tarball_file"])
    digest = hashes(tarball)
    require(all(digest[key] == frozen.get(key) for key in digest), "qualified npm tarball hashes changed")
    # Recheck replacement-sensitive public identities after downloading the package.
    require_public_release(args, receipt)
    metadata = {"version": version, "commit": args.commit, "package_name": PACKAGE,
                "tarball_file": frozen["tarball_file"], **digest}
    paths = [destination / frozen["tarball_file"], destination / "npm-package.json"]
    created = []
    try:
        for path, data in zip(paths, (tarball, (json.dumps(metadata, indent=2) + "\n").encode())):
            with path.open("xb") as stream:
                created.append((path, os.fstat(stream.fileno())))
                stream.write(data)
    except Exception:
        for path, owned in created:
            try:
                current = path.lstat()
                if (current.st_dev, current.st_ino) == (owned.st_dev, owned.st_ino):
                    path.unlink()
            except FileNotFoundError:
                pass
        raise
    return metadata


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("repository", "tag", "commit", "source", "output-dir"):
        parser.add_argument(f"--{name}", required=True)
    for name in ("run-id", "run-attempt"):
        parser.add_argument(f"--{name}", required=True, type=int)
    print(json.dumps(verify(parser.parse_args())))


if __name__ == "__main__":
    main()
