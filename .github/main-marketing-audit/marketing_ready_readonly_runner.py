"""Read-only selected Marketing artifact batch; no six-client matrix claim."""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request

REPO = "777genius/universal-agent-plugins"
API = "https://api.github.com"
HEAD = "ee6d714f9f684a5baf4b6740e4e8cb0481918be5"
CHECKOUT = "ee6d714f9f684a5baf4b6740e4e8cb0481918be5"
TREE = "fa9f279164dc3060ebe300db608963060a9d4d19"
MAIN = "aeb3c5e3580ed19db9c0f78b3ddd2ec7f8a086c8"
CORE = "ce7b1a5f48548360b567b47dc430496e1878b55a"
WORKFLOW = ".github/workflows/marketing-kit-project-release-e2e.yml"
RUN = 46152361079
CLIENTS = {"codex", "cursor", "claude", "gemini", "opencode", "copilot"}
NATIVE_PACKAGES = {
    "codex": "@openai/codex@0.153.4",
    "claude": "@anthropic-ai/claude-code@2.1.263",
    "gemini": "@google/gemini-cli@0.66.0",
    "opencode": "opencode-ai@1.18.34",
    "copilot": "@github/copilot@1.0.105",
}
API_LIMIT, ZIP_LIMIT, LOG_LIMIT = 4 << 20, 256 << 20, 32 << 20
sys.dont_write_bytecode = True

SOURCE_FILES = {
    ".github/workflows/marketing-kit-project-release-e2e.yml": "7d1c957e25d528fddd36734db2b95ee55ccfc994",
    "scripts/claude_cache_retirement_audit.py": "ecb56574b3e892be42cb6c79c3df346268c522c6",
    "scripts/codex_runtime_audit.py": "759b35886a434a7f17e2c36d8fd93d6a12bbe45d",
    "scripts/live-install-audit.py": "ceab34554b6851be09fae9dbc5345fdd753bd0f8",
    "scripts/marketing-kit-project-release-e2e.py": "af2ac520fbd9adee43b411864df73b0ea3b936de",
    "scripts/marketing_kit_claude_discovery.py": "7f1e14c453d7e9f8bcabcfed7bb4343343bf5743",
    "scripts/marketing_kit_claude_retirement.py": "c96774600899745327892d6e24e9a47c68c0ee2a",
    "scripts/marketing_kit_codex_profile.py": "7214510d013d7077fa0cc8adf949ec82778c9574",
    "scripts/native_client_project_artifacts.py": "bfcb69f1393b9b5671062da6e40dce54b3ea7951",
    "scripts/native_client_research.py": "c28e5f88dc26fa58efd45492b41ad899e6335605",
    "scripts/test_live_install_audit_env.py": "1d4c36f2b7e6b64233a037634a644f52013ccf7c",
    "scripts/test_marketing_kit_claude_discovery.py": "1c1d1fa5cde86130ff24dfaf102bcfb8ad8eab58",
    "scripts/test_marketing_kit_claude_retirement.py": "0ad598ed18ba88f3e0589b191b380461c866c7b2",
    "scripts/test_marketing_kit_codex_profile.py": "37159810eb32778cee838d1ec540012c3b2da04a",
    "scripts/test_marketing_kit_codex_sqlite.py": "0f01d6fab0a0fdda7205d418797bafc6a605e947",
    "scripts/test_marketing_kit_copilot_refusal.py": "6f7516aa1e8ec435b572d93190a61ffc57743d55",
    "scripts/test_marketing_kit_peer_initialization.py": "7faacd179627922f0bc48d4a4ea317bafac9d356",
    "scripts/test_marketing_kit_policy_boundary.py": "c17da88d634165cc7de087ae275933ac997f8002"
}

def require(ok, reason):
    if not ok:
        raise ValueError(reason)


def sha(body):
    return hashlib.sha256(body).hexdigest()


def pairs(values):
    result = {}
    for key, value in values:
        require(key not in result, "duplicate_json_key")
        result[key] = value
    return result


def decode(body):
    return json.loads(body.decode("utf-8"), object_pairs_hook=pairs,
                      parse_constant=lambda _: require(False, "nonfinite_json"))


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, fp, code, msg, headers, newurl):
        return None


OPENER = urllib.request.build_opener(NoRedirect())


def response(request, limit):
    try:
        reply = OPENER.open(request, timeout=45)
    except urllib.error.HTTPError as error:
        reply = error
    try:
        status, headers = reply.code, reply.headers
        body = reply.read(limit + 1) if status == 200 else b""
        require(len(body) <= limit, "response_bound")
        return status, headers, body
    finally:
        reply.close()


def api_request(path, token):
    require(path.startswith("/repos/" + REPO + "/")
            and "?" not in path and "#" not in path, "api_path")
    return urllib.request.Request(API + path, headers={
        "Authorization": "Bearer " + token, "Accept": "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28",
        "User-Agent": "uap-native-retained-byte-audit",
    })


def api(path, token):
    status, _, body = response(api_request(path, token), API_LIMIT)
    require(status == 200, "api_status")
    return decode(body)


def signed_url(url):
    parsed = urllib.parse.urlsplit(url)
    host = (parsed.hostname or "").lower()
    require(parsed.scheme == "https" and parsed.port in (None, 443)
            and parsed.username is None and parsed.password is None
            and not parsed.fragment and not any(ord(c) < 32 for c in url),
            "signed_url")
    require(any(host.endswith(suffix) for suffix in (
        ".blob.core.windows.net", ".githubusercontent.com",
        ".actions.githubusercontent.com", ".amazonaws.com")), "signed_host")


def redirected(path, token, limit):
    status, headers, _ = response(api_request(path, token), 0)
    require(status == 302, "api_download_redirect")
    location = headers.get("Location", "")
    for _ in range(4):
        signed_url(location)
        # Fresh tokenless requests: no API Authorization or cookies forwarded.
        request = urllib.request.Request(location, headers={
            "User-Agent": "uap-native-retained-byte-audit"})
        status, headers, body = response(request, limit)
        if status == 200:
            return body
        require(status in (301, 302, 303, 307, 308), "download_status")
        location = urllib.parse.urljoin(location, headers.get("Location", ""))
    raise ValueError("redirect_limit")


def git(source, *args):
    completed = subprocess.run(["git", "-C", str(source), *args], check=False,
                               stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
                               timeout=20)
    require(completed.returncode == 0 and len(completed.stdout) <= API_LIMIT,
            "source_git")
    return completed.stdout.decode("utf-8").strip()


def blob_hash(body):
    return hashlib.sha1(b"blob " + str(len(body)).encode() + b"\0" + body).hexdigest()


def positive(value):
    return type(value) is int and value > 0






def manifest_checks(manifest):
    require(manifest["schema_version"] == 1 and manifest["repo"] == REPO
            and manifest["head"] == HEAD and manifest["checkout"] == CHECKOUT
            and manifest["tree"] == TREE and manifest["core_blob"] == CORE
            and manifest["event"] == "push"
            and manifest["run_id"] == RUN and positive(manifest["run_attempt"]),
            "manifest_identity")
    selected = manifest["selected_clients"]
    require(isinstance(selected, list) and 0 < len(selected) <= len(CLIENTS)
            and all(isinstance(client, str) and client in CLIENTS for client in selected)
            and selected == sorted(set(selected)), "manifest_selected_clients")
    require(isinstance(manifest["lanes"], list)
            and [lane["client"] for lane in manifest["lanes"]] == selected,
            "manifest_selected_lane_order")
    clients, jobs, artifacts = set(), set(), set()
    for lane in manifest["lanes"]:
        client = lane["client"]
        require(client in CLIENTS and client not in clients, "manifest_client")
        clients.add(client)
        require(positive(lane["job_id"]) and positive(lane["artifact_id"])
                and lane["job_id"] not in jobs and lane["artifact_id"] not in artifacts,
                "manifest_lane_ids")
        jobs.add(lane["job_id"])
        artifacts.add(lane["artifact_id"])
        require(type(lane["archive_size"]) is int
                and 0 < lane["archive_size"] <= ZIP_LIMIT
                and isinstance(lane["archive_sha256"], str)
                and re.fullmatch("[0-9a-f]{64}", lane["archive_sha256"]),
                "manifest_archive")
    require(clients == set(selected), "manifest_client_coverage")


def job_steps(job, client):
    steps = job["steps"]
    require(isinstance(steps, list) and steps
            and all(step.get("status") == "completed"
                    and step.get("conclusion") in ("success", "skipped") for step in steps),
            "job_failed_or_incomplete_step")
    names = [step["name"] for step in steps]
    require(len(names) == len(set(names)), "duplicate_step")
    mandatory = {
        "Set up job",
        "Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1",
        "Run actions/setup-node@820762786026740c76f36085b0efc47a31fe5020",
        "Install real released UAP and native client",
        "Exercise actual signed Marketing Kit and three mandatory dependencies",
        "Preserve factual outcomes including failures",
        "Complete job",
    }
    if client == "codex":
        mandatory |= {
            "Install native Codex sandbox prerequisite",
            "Verify historical npm 0.1.115 launcher abort cleanup",
            "Verify published ChatGPT target safety fences",
        }
    observed = {step["name"]: step["conclusion"] for step in steps}
    require(all(observed.get(name) == "success" for name in mandatory),
            "mandatory_Marketing_step_not_success")


def checkout_log(log, client):
    text = log.decode("utf-8")
    lines, commits = text.splitlines(), []
    for index, line in enumerate(lines[:-1]):
        if "log -1 --format=%H" in line:
            value = re.sub(r"^\d{4}-\d\d-\d\dT\S+Z ", "", lines[index + 1])
            require(re.fullmatch("[0-9a-f]{40}", value), "checkout_log_shape")
            commits.append(value)
    require(commits and set(commits) == {CHECKOUT}, "actual_checkout_log_identity")
    require("npm install --global universal-agent-plugins@0.1.116" in text
            and "TARGET: " + client in text, "released_install_lane")
    if client in NATIVE_PACKAGES:
        require("NATIVE_PACKAGE: " + NATIVE_PACKAGES[client] in text,
                "native_matrix_provisioning_pin")
    if client == "cursor":
        for value in (
            "85296944e4600a83c4b764f578d1502bf9c416db200d33a593e8f3ecabfdc294",
            "8b9198f7dde79f7e27cc83dabf3c66a93ca5303b6919719100c82f29d9498a65",
        ):
            require(value in text, "Cursor_provisioning_pin")
    if client == "opencode":
        require('opencode_version="$(timeout --kill-after=5s 25s opencode --version)"' in text
                and '[[ "$opencode_version" == 1.18.34 ]]' in text,
                "OpenCode_source_version_guard")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--manifest", required=True)
    parser.add_argument("--manifest-sha256", required=True)
    parser.add_argument("--source", required=True)
    parser.add_argument("--core", required=True)
    args = parser.parse_args()
    body = Path(args.manifest).read_bytes()
    require(len(body) <= 32768 and sha(body) == args.manifest_sha256,
            "manifest_custody")
    manifest = decode(body)
    manifest_checks(manifest)
    source = Path(args.source).resolve()
    require(git(source, "rev-parse", "HEAD") == HEAD
            and git(source, "rev-parse", "HEAD^{tree}") == TREE
            and git(source, "status", "--porcelain=v1", "--untracked-files=all") == "",
            "source_checkout")
    texts = {}
    for path, expected in SOURCE_FILES.items():
        product_body = (source / path).read_bytes()
        require(len(product_body) <= API_LIMIT and blob_hash(product_body) == expected
                and git(source, "rev-parse", "HEAD:" + path) == expected,
                "canonical_source_blob")
        texts[path] = product_body.decode("utf-8")
    core = Path(args.core).resolve()
    core_body = core.read_bytes()
    require(len(core_body) <= 65536 and blob_hash(core_body) == CORE, "core_custody")
    # Import only the independently reviewed core. Product modules stay inert.
    spec = importlib.util.spec_from_file_location("Marketing_retained_core", core)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    token = os.environ.get("GH_TOKEN", "")
    require(bool(token), "read_token")
    prefix = "/repos/" + REPO
    head = api(prefix + "/git/commits/" + HEAD, token)
    synthetic = api(prefix + "/git/commits/" + CHECKOUT, token)
    require(head["sha"] == HEAD and head["tree"]["sha"] == TREE
            and synthetic["sha"] == CHECKOUT and synthetic["tree"]["sha"] == TREE
            and [parent["sha"] for parent in synthetic["parents"]] == [MAIN],
            "authenticated_head_and_synthetic_tree")
    run = api(prefix + "/actions/runs/" + str(RUN), token)
    require(run["id"] == RUN and run["head_sha"] == HEAD
            and run["repository"]["full_name"] == REPO
            and run["head_repository"]["full_name"] == REPO
            and run["path"] == WORKFLOW
            and run["run_attempt"] == manifest["run_attempt"]
            and run["event"] == manifest["event"]
            and run["head_branch"] == "main", "run_identity")
    receipts = []
    for lane in manifest["lanes"]:
        client = lane["client"]
        job = api(prefix + "/actions/jobs/" + str(lane["job_id"]), token)
        require(job["id"] == lane["job_id"] and job["run_id"] == RUN
                and job["head_sha"] == HEAD and job["run_attempt"] == run["run_attempt"]
                and job["name"] == "npm 0.1.116 / " + client + " / real Project lifecycle"
                and job["status"] == "completed" and job["conclusion"] == "success",
                "Marketing_job_identity_or_result")
        job_steps(job, client)
        log = redirected(prefix + "/actions/jobs/" + str(lane["job_id"]) + "/logs",
                         token, LOG_LIMIT)
        checkout_log(log, client)
        artifact = api(prefix + "/actions/artifacts/" + str(lane["artifact_id"]), token)
        require(artifact["id"] == lane["artifact_id"] and artifact["expired"] is False
                and artifact["name"] == "marketing-kit-0.1.116-" + client
                and artifact["size_in_bytes"] == lane["archive_size"]
                and artifact["digest"] == "sha256:" + lane["archive_sha256"]
                and artifact["workflow_run"]["id"] == RUN
                and artifact["workflow_run"]["head_sha"] == HEAD, "artifact_identity")
        archive = redirected(prefix + "/actions/artifacts/" + str(lane["artifact_id"])
                             + "/zip", token, lane["archive_size"])
        core_manifest = dict(lane, run_id=RUN, job_id=job["id"], sources=texts)
        receipt = module.audit_marketing(
            archive, {"head": HEAD, "checkout": CHECKOUT, "tree": TREE}, RUN, job["id"],
            core_manifest)
        receipt["artifact_id"] = lane["artifact_id"]
        receipt["decoded_job_log_sha256"] = sha(log)
        receipt["decoded_job_log_bytes"] = len(log)
        receipt["job_steps"] = [{"name": step["name"], "conclusion": step["conclusion"]}
                                for step in job["steps"]]
        receipt["installer_authority"] = {
            "kind": "published npm UAP control", "version": "0.1.116",
            "source_CLI_build_used": False,
            "historical_0_1_115_launcher_cleanup": "separate mock-transport test" if client == "codex" else "not_applicable"}
        receipts.append(receipt)
        del archive, log
    require(git(source, "status", "--porcelain=v1", "--untracked-files=all") == "",
            "source_changed_during_audit")
    result = json.dumps({"audit_kind": "selected published Marketing retained-evidence audit batch",
                         "status": "passed", "head": HEAD, "checkout": CHECKOUT,
                         "tree": TREE, "core_blob": CORE, "manifest_sha256": sha(body),
                         "selected_clients": manifest["selected_clients"],
                         "remaining_clients": sorted(CLIENTS - set(manifest["selected_clients"])),
                         "six_client_matrix_qualified": False,
                         "coverage": "selected lanes only; final six-client union checked separately",
                         "lane_count": len(receipts), "receipts": receipts}, sort_keys=True)
    require(len(result.encode("utf-8")) <= 98304, "receipt_bound")
    print(result, flush=True)


if __name__ == "__main__":
    try:
        main()
    except Exception:
        print('{"status":"failed","audit_kind":"selected published Marketing retained-evidence audit batch"}',
              flush=True)
        raise SystemExit(1)
