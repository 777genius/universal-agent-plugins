"""Read original PR #1130 artifacts through gh; never run an installer/client.

The offline auditor remains bound to its historical source. Git object reads
provide immutable source texts, so the caller's working tree is never executed.
Metadata, original ZIPs/logs and receipts are retained in a new private directory.
"""
import argparse
import hashlib
import json
from pathlib import Path
import re
import subprocess

import native_client_collection_evidence as audit

REPO = "777genius/universal-agent-plugins"
RUN = 46139633140
WORKFLOW = ".github/workflows/agentplugins-native-clients.yml"
LANES = {(client, target) for client in ("claude", "codex")
         for target in ("linux-amd64", "darwin-arm64", "windows-amd64")}


def command(args, limit):
    result = subprocess.run(args, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                            check=False, timeout=120)
    audit.require(result.returncode == 0, "read-only command failed")
    audit.require(len(result.stdout) <= limit, "command response exceeds bound")
    return result.stdout


def source_texts(repo):
    def git(*args):
        return command(["git", "-C", str(repo), *args], 4 << 20)

    audit.require(git("rev-parse", audit.COMMIT + "^{tree}").decode().strip() == audit.TREE,
                  "historical source tree mismatch")
    return {path: git("show", audit.COMMIT + ":" + path).decode("utf-8")
            for path in ("scripts/run-native-client-matrix.py", WORKFLOW)}


def validate_run(run):
    audit.require(run.get("id") == RUN and run.get("head_sha") == audit.COMMIT
                  and run.get("path") == WORKFLOW and run.get("event") == "pull_request"
                  and run.get("run_attempt") == 1
                  and run.get("repository", {}).get("full_name") == REPO
                  and run.get("head_repository", {}).get("full_name") == REPO,
                  "historical run identity mismatch")


def validate_job(job, client, target):
    audit.require(type(job.get("id")) is int and job["id"] > 0
                  and job.get("run_id") == RUN and job.get("head_sha") == audit.COMMIT
                  and job.get("run_attempt") == 1
                  and job.get("name") == client + " runtime (" + target + ")"
                  and job.get("status") == "completed" and job.get("conclusion") == "success",
                  "native job identity or result mismatch")
    steps = job.get("steps")
    audit.require(isinstance(steps, list) and steps and all(
        step.get("status") == "completed" and step.get("conclusion") in ("success", "skipped")
        for step in steps), "native job contains failed or incomplete steps")
    names = [step.get("name") for step in steps]
    audit.require(len(names) == len(set(names)), "duplicate native job step")
    mandatory = {
        "Set up job", "Complete job",
        "Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1",
        "Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e",
        "Verify native matrix safety and evidence gates",
        "Build exact source and execute pinned native client in disposable fixture",
        "Upload native evidence including failed stages",
    }
    if (client, target) == ("codex", "linux-amd64"):
        mandatory |= {"Set up Node for browser Skills qualification",
                      "Bundle the real picker before building the public CLI",
                      "Prepare the native Codex Linux sandbox",
                      "Preserve the actual browser to native Skills screenshots"}
    passed = {step["name"] for step in steps if step["conclusion"] == "success"}
    audit.require(mandatory <= passed, "mandatory native step not successful")


def validate_artifact(artifact, client, target):
    audit.require(type(artifact.get("id")) is int and artifact["id"] > 0
                  and artifact.get("name") == "native-client-" + client + "-" + target
                  and artifact.get("expired") is False
                  and type(artifact.get("size_in_bytes")) is int
                  and 0 < artifact["size_in_bytes"] <= 256 << 20
                  and artifact.get("workflow_run", {}).get("id") == RUN
                  and artifact.get("workflow_run", {}).get("head_sha") == audit.COMMIT,
                  "native artifact identity mismatch")
    digest = artifact.get("digest")
    audit.require(isinstance(digest, str) and digest.startswith("sha256:"),
                  "missing authenticated archive digest")
    return audit.digest(digest.removeprefix("sha256:"))


def validate_checkout(log):
    lines = log.decode("utf-8").splitlines()
    commits = [re.sub(r"^\d{4}-\d\d-\d\dT\S+Z ", "", lines[index + 1])
               for index, line in enumerate(lines[:-1]) if "log -1 --format=%H" in line]
    audit.require(commits and set(commits) == {audit.COMMIT}, "native checkout log mismatch")


def collect(repo, destination):
    texts = source_texts(repo)
    # Reject an altered source before the first network read.
    audit.authorities({"runner_source": texts["scripts/run-native-client-matrix.py"],
                       "workflow_source": texts[WORKFLOW]})
    destination.mkdir(mode=0o700, parents=False, exist_ok=False)

    def gh(path, filename, limit=4 << 20):
        body = command(["gh", "api", "repos/" + REPO + "/" + path], limit)
        with (destination / filename).open("xb") as output:
            output.write(body)
        return body

    run = audit.document(gh(f"actions/runs/{RUN}", "run.json"))
    validate_run(run)
    jobs = audit.document(gh(f"actions/runs/{RUN}/attempts/1/jobs?per_page=100", "jobs.json"))
    artifacts = audit.document(gh(f"actions/runs/{RUN}/artifacts?per_page=100", "artifacts.json"))
    audit.require(jobs.get("total_count") == len(jobs.get("jobs", []))
                  and artifacts.get("total_count") == len(artifacts.get("artifacts", [])),
                  "incomplete native metadata page")
    receipts = []
    for client, target in sorted(LANES):
        name = client + " runtime (" + target + ")"
        selected = [job for job in jobs["jobs"] if job.get("name") == name]
        audit.require(len(selected) == 1, "missing or ambiguous native job")
        job = selected[0]
        validate_job(job, client, target)
        selected = [artifact for artifact in artifacts["artifacts"]
                    if artifact.get("name") == "native-client-" + client + "-" + target]
        audit.require(len(selected) == 1, "missing or ambiguous native archive")
        artifact = selected[0]
        digest = validate_artifact(artifact, client, target)
        stem = client + "-" + target
        log = gh(f"actions/jobs/{job['id']}/logs", stem + ".log", 32 << 20)
        validate_checkout(log)
        blob = gh(f"actions/artifacts/{artifact['id']}/zip", stem + ".zip", 256 << 20)
        manifest = {"client": client, "target": target, "archive_size": artifact["size_in_bytes"],
                    "archive_sha256": digest, "run_id": RUN, "job_id": job["id"],
                    "runner_source": texts["scripts/run-native-client-matrix.py"],
                    "workflow_source": texts[WORKFLOW]}
        receipt = audit.audit_native(blob, {"commit": audit.COMMIT, "tree": audit.TREE},
                                     RUN, job["id"], manifest)
        receipt = dict(receipt, artifact_id=artifact["id"],
                       decoded_job_log_sha256=hashlib.sha256(log).hexdigest())
        (destination / (stem + ".receipt.json")).write_text(
            json.dumps(receipt, indent=2) + "\n", encoding="utf-8")
        receipts.append(receipt)
    receipt = {"status": "passed", "lane_count": len(receipts), "run_id": RUN,
               "auditor_sha256": audit.sha(Path(audit.__file__).read_bytes()),
               "acquisition_sha256": audit.sha(Path(__file__).read_bytes()),
               "source": {"commit": audit.COMMIT, "tree": audit.TREE}, "receipts": receipts}
    (destination / "receipt.json").write_text(json.dumps(receipt, indent=2) + "\n", encoding="utf-8")
    return receipt


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source-repo", type=Path, required=True)
    parser.add_argument("--evidence-dir", type=Path, required=True,
                        help="new private directory; existing evidence is never overwritten")
    args = parser.parse_args()
    receipt = collect(args.source_repo, args.evidence_dir)
    print(json.dumps({"status": receipt["status"], "lane_count": receipt["lane_count"],
                      "source": receipt["source"]}))


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        # Never echo subprocess stderr, RPC payloads, signed URLs or credentials.
        # Our exact ValueErrors carry static checks, while decoder/subprocess
        # exception messages may contain input and must remain private.
        reason = error.args[0] if type(error) is ValueError and error.args else ""
        safe = isinstance(reason, str) and re.fullmatch(r"[A-Za-z][A-Za-z0-9 _:/.-]{0,120}", reason)
        print(json.dumps({"status": "failed", "error_type": type(error).__name__,
                          "reason": reason if safe else "details retained privately"}))
        raise SystemExit(1)
