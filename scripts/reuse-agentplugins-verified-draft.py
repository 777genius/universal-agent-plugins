#!/usr/bin/env python3
"""Authenticate one completed draft qualification and reverify its live assets.

The receipt is an exact-run GitHub Actions artifact, not a caller-supplied file.
This command only reads GitHub state; publication stays in the gated workflow.
"""
import argparse
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import re
import subprocess
import tempfile
import zipfile

HERE = Path(__file__).resolve().parent
SPEC = importlib.util.spec_from_file_location("draft_verifier", HERE / "verify-agentplugins-draft.py")
draft = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(draft)
REPOSITORY = draft.REPOSITORY
WORKFLOW = draft.WORKFLOW
TARGETS = ("darwin-amd64", "darwin-arm64", "linux-amd64", "linux-arm64",
           "windows-amd64", "windows-arm64")


def require(ok, message):
    if not ok:
        raise ValueError(message)


def output(command):
    return subprocess.check_output(command, timeout=300)


def api(endpoint, *options):
    return json.loads(output(["gh", "api", endpoint, *options]))


def one_job(jobs, name, run_id, conclusion):
    matches = [job for job in jobs if job.get("name") == name]
    require(len(matches) == 1 and matches[0].get("run_id") == run_id
            and matches[0].get("status") == "completed"
            and matches[0].get("conclusion") == conclusion,
            "missing or wrong qualification job: " + name)
    if conclusion == "success":
        steps = matches[0].get("steps", [])
        require(steps and all(step.get("conclusion") == "success" for step in steps),
                "incomplete qualification job: " + name)


def authenticate(args):
    require(args.repository == REPOSITORY, "unexpected repository")
    require(re.fullmatch(r"agentplugins-v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)", args.tag),
            "invalid stable tag")
    require(re.fullmatch(r"[0-9a-f]{40}", args.commit), "invalid frozen commit")
    require(args.run_id > 0 and args.run_attempt > 0, "missing qualification identity")
    prefix = f"repos/{REPOSITORY}/actions"
    run = api(f"{prefix}/runs/{args.run_id}")
    require(run.get("id") == args.run_id and run.get("run_attempt") == args.run_attempt
            and run.get("repository", {}).get("full_name") == REPOSITORY
            and run.get("head_repository", {}).get("full_name") == REPOSITORY
            and run.get("head_sha") == args.commit and run.get("head_branch") == "main"
            and run.get("path") == WORKFLOW and run.get("event") == "workflow_dispatch"
            and run.get("status") == "completed" and run.get("conclusion") == "success",
            "qualification run is not successful on the exact tagged main commit")
    workflow = api(f"{prefix}/workflows/{run['workflow_id']}")
    require(workflow.get("id") == run["workflow_id"] and workflow.get("path") == WORKFLOW,
            "qualification workflow identity changed")
    pages = api(f"{prefix}/runs/{args.run_id}/attempts/{args.run_attempt}/jobs?per_page=100",
                "--paginate", "--slurp")
    jobs = [job for page in pages for job in page["jobs"]]
    for name in ("validate",
                 "build (darwin, amd64)", "build (darwin, arm64)",
                 "build (linux, amd64)", "build (linux, arm64)",
                 "build (windows, amd64, .exe)", "build (windows, arm64, .exe)",
                 "stage-draft",
                 "platform-proof / Verify release and pack exact npm tarball",
                 *(f"platform-proof / native runtime E2E ({target})" for target in TARGETS),
                 "platform-proof / Aggregate all six native platform proofs", "verified-draft"):
        one_job(jobs, name, args.run_id, "success")
    one_job(jobs, "promote-release", args.run_id, "skipped")
    pages = api(f"{prefix}/runs/{args.run_id}/artifacts?per_page=100", "--paginate", "--slurp")
    name = f"agentplugins-verified-draft-{args.run_id}-{args.run_attempt}"
    matches = [artifact for page in pages for artifact in page["artifacts"]
               if artifact.get("name") == name]
    require(len(matches) == 1, "missing or duplicate verified draft receipt")
    artifact = matches[0]
    require(type(artifact.get("id")) is int and artifact["id"] > 0
            and artifact.get("expired") is False
            and type(artifact.get("size_in_bytes")) is int
            and 0 < artifact["size_in_bytes"] <= 2 << 20
            and re.fullmatch(r"sha256:[0-9a-f]{64}", artifact.get("digest", ""))
            and artifact.get("workflow_run", {}).get("id") == args.run_id
            and artifact["workflow_run"].get("head_sha") == args.commit
            and run["run_started_at"] <= artifact["created_at"] <= run["updated_at"],
            "receipt artifact identity is invalid")
    body = output(["gh", "api", f"{prefix}/artifacts/{artifact['id']}/zip"])
    require(len(body) == artifact["size_in_bytes"]
            and hashlib.sha256(body).hexdigest() == artifact["digest"][7:],
            "receipt artifact ZIP digest mismatch")
    with zipfile.ZipFile(io.BytesIO(body)) as archive:
        require(archive.namelist() == ["verified-draft.json"],
                "unexpected receipt artifact contents")
        member = archive.infolist()[0]
        require(member.file_size <= 1 << 20 and (member.external_attr >> 16) & 0o170000 in (0, 0o100000),
                "invalid receipt member")
        receipt = json.loads(archive.read(member))
    require(receipt.get("schema_version") == 1
            and receipt.get("release_state") == "verified-draft"
            and receipt.get("repository") == REPOSITORY
            and receipt.get("producer_commit") == args.commit
            and receipt.get("producer_run_id") == args.run_id
            and receipt.get("producer_run_attempt") == args.run_attempt
            and receipt.get("signer_workflow") == f"github.com/{REPOSITORY}/{WORKFLOW}"
            and receipt.get("release", {}).get("tag") == args.tag
            and receipt.get("release", {}).get("commit") == args.commit
            and receipt.get("release", {}).get("draft") is True
            and receipt.get("release", {}).get("prerelease") is False
            and type(receipt.get("release", {}).get("id")) is int
            and re.fullmatch(r"[0-9a-f]{64}", receipt.get("asset_set_digest", "")),
            "verified draft receipt identity mismatch")
    return receipt


def require_current_source(source, tag, commit):
    output(["git", "-C", str(source), "fetch", "--force", "origin",
            "main:refs/remotes/origin/main", f"refs/tags/{tag}:refs/tags/{tag}"])
    require(output(["git", "-C", str(source), "rev-parse",
                    "refs/remotes/origin/main"]).decode().strip() == commit
            and output(["git", "-C", str(source), "rev-list", "-n", "1",
                        f"refs/tags/{tag}"]).decode().strip() == commit,
            "tag or current main moved since qualification")


def verify(args):
    receipt = authenticate(args)
    source = Path(args.source).resolve()
    require(output(["git", "-C", str(source), "rev-parse", "HEAD"]).decode().strip() == args.commit
            and not output(["git", "-C", str(source), "status", "--porcelain", "--untracked-files=normal"]),
            "promotion checkout differs from frozen commit")
    require_current_source(source, args.tag, args.commit)
    with tempfile.TemporaryDirectory(prefix="agentplugins-promotion-") as directory:
        live_args = argparse.Namespace(repository=REPOSITORY, tag=args.tag, commit=args.commit,
                                       release_id=receipt["release"]["id"],
                                       asset_set_digest=receipt["asset_set_digest"],
                                       run_id=args.run_id, run_attempt=args.run_attempt,
                                       source=str(source), receipt=str(Path(directory) / "live.json"))
        live = draft.verify(live_args)
    require_current_source(source, args.tag, args.commit)
    require(live["release"] == receipt["release"]
            and live["subjects"] == receipt["subjects"],
            "draft changed since qualification")
    return live


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("repository", "tag", "commit", "source"):
        parser.add_argument(f"--{name}", required=True)
    for name in ("run-id", "run-attempt"):
        parser.add_argument(f"--{name}", required=True, type=int)
    verify(parser.parse_args())


if __name__ == "__main__":
    main()
