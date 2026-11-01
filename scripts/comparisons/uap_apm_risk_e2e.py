#!/usr/bin/env python3
"""Isolated, evidence-only risky lifecycle comparison; never touches actual profiles.

Three workflow groups execute in parallel. This script reports real PASS/FAIL,
BLOCKED, UNSUPPORTED or NOT_RUN per product and scenario, not a leaderboard.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import secrets
import shutil
import signal
import stat
import subprocess
import tempfile
import time
import tomllib

import uap_apm_skills_e2e as baseline

REPO = baseline.REPOSITORY
SKILL = baseline.SKILL_PATH
SKILL_SECOND = "tests/fixtures/uap_apm/skills/benchmark-secondary"
PARENT = "tests/fixtures/uap_apm/risk_graph/parent"
CHILD = "tests/fixtures/uap_apm/risk_graph/child"
PARENT_V1 = "25bc7b7cc56f97761eaf76b00448e0c4ec4ee244"
PARENT_V2 = "73c417b0c845aebc5452e8d9c57d25d4eb2ae246"
CHILD_V1 = "ab81308e402e3ac64c3e1840088770d98a1b6245"
CHILD_V2 = "fb96f0f4d1ef62c2ea3dfc11c1880380fe3737b2"
SOURCE = "https://github.com/777genius/universal-agent-plugins.git"
GROUPS = {
    "cold-profile": ("R1_missing_receipts", "R2_claude_profiles"),
    "graph-removal": ("R3_transitive_update", "R6_git_pull_removal"),
    "failure-secrets": ("R4_interrupted_operation", "R5_mcp_env_secret"),
}
MARKER = "UAP_APM_SECRET_CANARY"


def h(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def fixture(commit: str, subpath: str) -> bytes:
    return subprocess.check_output(["git", "show", f"{commit}:{subpath}"])


def git(*args: str, cwd: Path, env: dict[str, str]) -> str:
    clean = dict(env)
    clean.update({"GIT_CONFIG_NOSYSTEM": "1", "GIT_TERMINAL_PROMPT": "0"})
    p = subprocess.run(["git", "-c", "user.name=777genius",
                        "-c", "user.email=iliyazelenkog@gmail.com", *args],
                       cwd=cwd, env=clean, stdin=subprocess.DEVNULL,
                       stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                       timeout=30, check=True)
    return p.stdout.decode("utf-8", "replace").strip()


def observed_file(path: Path, boundary: Path, limit: int = 2 * 1024 * 1024) -> bytes | None:
    """A missing, nonregular, oversize or symlinked file never counts as content."""
    try:
        rel = path.relative_to(boundary)
        if not rel.parts:
            return None
        cursor = boundary
        if cursor.is_symlink():
            return None
        for part in rel.parts:
            cursor = cursor / part
            if cursor.is_symlink():
                return None
        if not stat.S_ISREG(path.lstat().st_mode):
            return None
        fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
        with os.fdopen(fd, "rb") as source:
            state = os.fstat(source.fileno())
            if not stat.S_ISREG(state.st_mode) or state.st_size > limit:
                return None
            result = source.read(limit + 1)
        return result if len(result) <= limit else None
    except (ValueError, OSError):
        return None


def status(eligible: bool, command: dict | None, invariant: bool, reason: str,
           *, protected: bool = True, provenance: bool = True) -> tuple[str, str]:
    if not eligible:
        return "BLOCKED", "documented prerequisite or supported route not established: " + reason
    if not protected:
        return "FAIL", "a protected or foreign file changed: " + reason
    if command is None or command.get("timed_out") or command.get("cleanup_error"):
        return "BLOCKED", "process outcome incomplete: " + reason
    if command.get("exit_code") != 0:
        return "BLOCKED" if invariant else "FAIL", "command refused: " + reason
    if not invariant or not provenance:
        return "FAIL", reason
    return "PASS", reason


def receipt(product: str, case: str, label: str, **observed) -> dict:
    return {"product": product, "scenario": case, "status": label,
            "native_model_execution": "NOT_RUN", **observed}


def run(product: str, cli: str, project: Path, root: Path, env: dict[str, str],
        *, replay: bool = False, commit: str, target: str = "codex") -> dict:
    if target == "codex":
        argv = baseline.tool_args(product, cli, project, commit, replay=replay)
    elif product == "uap":
        if replay:
            argv = [cli, "install", "--locked", "--scope", "project",
                    "--project-root", str(project), "--yes", "--format", "json"]
        else:
            argv = [cli, "skills", "add", f"{REPO}@{commit}//{SKILL}",
                    "--target", target, "--scope", "project",
                    "--project-root", str(project), "--no-ui", "--yes", "--format", "json"]
    else:
        argv = [cli, "install", *(["--frozen"] if replay else []), "--target", target]
    return baseline.run_process(argv, project, env, root)


def prepare_project(product: str, project: Path, commit: str) -> None:
    if product == "apm":
        baseline.prepare_apm_manifest(project, commit)


def clean_snapshot(project: Path, files: tuple[str, ...]) -> dict[str, str | None]:
    return {name: (h(content) if (content := observed_file(project / name, project)) is not None else None)
            for name in files}


def case_cold(product: str, cli: str, root: Path, commit: str, expected: dict[str, bytes]) -> dict:
    env = baseline.isolated_env(root)
    project, foreign = baseline.make_project(root)
    prepare_project(product, project, commit)
    initial = run(product, cli, project, root, env, commit=commit)
    delivered = baseline.payloads(project, expected)
    portable, capture_error = baseline.captured_portable(project, product)
    first_good = (initial["exit_code"] == 0 and not initial["timed_out"]
                  and not initial["cleanup_error"] and delivered["skills"]
                  and delivered["references"] and bool(portable) and capture_error is None
                  and baseline.portable_identity(portable, commit, product)
                    ["exact_source_commit_recorded_in_lock"])
    if not first_good:
        return receipt(product, "R1_missing_receipts", "BLOCKED",
                       stage="initial_source_admission", initial=initial,
                       reason="initial complete package and source lock unavailable")
    # Recreate a committed *deployed* Skill with no project-local receipt/cache.
    # This is stronger than PR #617's empty destination cold restore.
    owned = (".agents/skills/benchmark-note/SKILL.md",
             ".agents/skills/benchmark-note/references/fixture.txt")
    fresh_root = root / "cold"
    fresh_env = baseline.isolated_env(fresh_root)
    committed, neighbor = baseline.make_project(fresh_root)
    for name, data in portable.items():
        (committed / name).write_bytes(data)
    for name in owned:
        dest = committed / name
        dest.parent.mkdir(parents=True, exist_ok=True)
        dest.write_bytes(expected[name.removeprefix(".agents/skills/benchmark-note/")])
    git("init", "-q", cwd=committed, env=fresh_env)
    git("add", "-A", cwd=committed, env=fresh_env)
    git("commit", "-qm", "pinned portable inputs and committed Skill", cwd=committed, env=fresh_env)
    teammate = fresh_root / "clean-clone"
    git("clone", "-q", str(committed), str(teammate), cwd=fresh_root, env=fresh_env)
    before = clean_snapshot(teammate, owned)
    portable_before, err = baseline.captured_portable(teammate, product)
    # A fresh HOME makes any old user receipts unavailable.
    actual = run(product, cli, teammate, fresh_root, fresh_env, commit=commit, replay=True)
    after = clean_snapshot(teammate, owned)
    portable_after, after_err = baseline.captured_portable(teammate, product)
    preserved = baseline.safe_same_bytes(
        teammate / ".agents/skills/foreign-neighbor/SKILL.md", neighbor, teammate)
    intact = before == after and all(after.values()) and preserved
    lock_intact = (err is None and after_err is None and portable_before == portable_after)
    outcome, why = status(True, actual, intact and lock_intact, "cold clone must not delete or rewrite committed Skill/lock", protected=intact)
    return receipt(product, "R1_missing_receipts", outcome, reason=why,
                   scenario_type="committed deployed files + clean HOME and receipts",
                   initial_portable_sha256={k:h(v) for k,v in portable.items()},
                   before=before, after=after, lock_identical=lock_intact,
                   command=actual, foreign_unchanged=preserved)


def profile_scan(root: Path, expected: bytes) -> dict[str, str]:
    """Only enumerate a small fixed Skills subtree; no symlink traversal."""
    folder = root / "skills"
    result: dict[str, str] = {}
    if not folder.is_dir() or folder.is_symlink():
        return result
    for subdir in sorted(folder.iterdir()):
        if subdir.is_symlink() or not subdir.is_dir():
            continue
        skill = subdir / "SKILL.md"
        data = observed_file(skill, root)
        if data == expected:
            result[str(skill.relative_to(root))] = h(data)
        if len(result) > 32:
            return {}
    return result


def case_profiles(product: str, cli: str, root: Path, commit: str, expected: dict[str, bytes],
                  *, claude_available: bool) -> dict:
    if not claude_available:
        return receipt(product, "R2_claude_profiles", "BLOCKED", reason="genuine pinned Claude CLI not available")
    env = baseline.isolated_env(root)
    home = Path(env["HOME"])
    a, b = home / ".claude", home / "profiles" / "b"
    for dest in (a, b):
        (dest / "skills" / "foreign-neighbor").mkdir(parents=True, exist_ok=True)
        (dest / "skills" / "foreign-neighbor" / "SKILL.md").write_bytes(b"FOREIGN PROFILE DATA\n")
    project = root / "project"
    project.mkdir()
    apm_source = f"{REPO}/{SKILL}#{commit}"
    outcome: list[dict] = []
    for index, chosen in enumerate((a, b, a)):
        active = dict(env, CLAUDE_CONFIG_DIR=str(chosen))
        if product == "uap":
            argv = [cli, "skills", "add", f"{REPO}@{commit}//{SKILL}",
                    "--target", "claude", "--scope", "user", "--no-ui", "--yes", "--format", "json"]
        elif index == 0:
            argv = [cli, "install", apm_source, "--global", "--target", "claude"]
        else:
            argv = [cli, "install", "--global", "--target", "claude"]
        cmd = baseline.run_process(argv, project, active, root)
        outcome.append({"phase":index, "selected":"A" if chosen==a else "B", "cmd":cmd,
                        "a":profile_scan(a, expected["SKILL.md"]),
                        "b":profile_scan(b, expected["SKILL.md"]),
                        "foreign": all(baseline.safe_same_bytes(
                          p / "skills/foreign-neighbor/SKILL.md", b"FOREIGN PROFILE DATA\n", home)
                          for p in (a,b))})
    first, second, third = outcome
    if not first["a"] or first["cmd"]["exit_code"] != 0:
        verdict="BLOCKED"
        reason="first user-scope Claude installation not admitted (runtime/prerequisite/format)"
    elif not second["foreign"] or not third["foreign"]:
        verdict="FAIL";reason="foreign profile sentinel changed"
    elif not second["a"] or not third["a"]:
        verdict="FAIL";reason="a later install removed or altered profile A's Skill"
    elif any(x["cmd"]["exit_code"] != 0 for x in outcome[1:]):
        verdict="BLOCKED";reason="safe refusal or unsupported multi-profile operation"
    elif not second["b"] or not third["b"]:
        verdict="FAIL";reason="profile B installation was missing after successful command"
    else:
        verdict="PASS";reason="both profiles retained exact Skill and foreign bytes"
    return receipt(product,"R2_claude_profiles",verdict,reason=reason,
                   phases=outcome,claude_version_verified=True,
                   selected_uninstall="NOT_RUN (separate unsafe deletion path requires verified ownership)")


def root_graph(product: str, project: Path, parent_commit: str) -> None:
    if product == "uap":
        (project / "uap.toml").write_text(
            "schema_version = 1\n\n[dependencies.parent]\n"
            f'git = "{SOURCE}"\npath = "{PARENT}"\ncommit = "{parent_commit}"\n'
            '\n[targets]\ncodex = ["parent"]\n')
    else:
        (project / "apm.yml").write_text(
            "name: risk-graph-consumer\nversion: 1.0.0\ntargets: [codex]\n"
            "dependencies:\n  apm:\n"
            f"    - {REPO}/{PARENT}#{parent_commit}\n")


def graph_command(product: str, cli: str, project: Path,
                  *, updated: bool = False, lock: bool = False) -> list[str]:
    if product == "uap":
        command=[cli, "deps", "lock" if lock else "sync",
                 "--scope","project","--project-root",str(project)]
        if not lock:
            command += ["--yes","--format","json"]
        return command
    return [cli, "install", "--target", "codex", *(["--update"] if updated else [])]


def child_lock(project: Path, product: str, commit: str) -> bool:
    try:
        if product == "uap":
            lock = json.loads((project / "uap.lock").read_bytes())
            return any(isinstance(n,dict)
                       and (g:=n.get("source",{}).get("git",{})).get("resolved_skill_path",g.get("path")) == CHILD
                       and g.get("commit")==commit
                       for n in lock.get("packages",[]))
        import yaml
        lock=yaml.safe_load((project / "apm.lock.yaml").read_bytes())
        return any(isinstance(n,dict) and n.get("virtual_path")==CHILD
                   and n.get("resolved_commit")==commit for n in lock.get("dependencies",[]))
    except (OSError,ValueError,TypeError,KeyError,ImportError):
        return False


def case_graph(product: str, cli: str, root: Path) -> dict:
    env=baseline.isolated_env(root)
    project,_=baseline.make_project(root)
    root_graph(product,project,PARENT_V1)
    initial=baseline.run_process(graph_command(product,cli,project),project,env,root)
    old=fixture(CHILD_V1,CHILD+"/skills/risk-child/SKILL.md")
    new=fixture(CHILD_V2,CHILD+"/skills/risk-child/SKILL.md")
    old_skill=project / ".agents/skills/risk-child/SKILL.md"
    first=observed_file(old_skill,project)==old and child_lock(project,product,CHILD_V1)
    if initial["exit_code"]!=0 or not first:
        return receipt(product,"R3_transitive_update","BLOCKED",stage="initial_graph_admission",
                       reason="two-package transitive graph did not reach expected native source state",
                       initial=initial,child_v1_seen=first)
    root_graph(product,project,PARENT_V2)
    prelock=None
    if product=="uap":
        prelock=baseline.run_process(graph_command(product,cli,project,lock=True),project,env,root)
    updated=baseline.run_process(graph_command(product,cli,project,updated=True),project,env,root)
    actual=observed_file(old_skill,project)
    graph_ok=child_lock(project,product,CHILD_V2)
    foreign_ok=baseline.safe_same_bytes(project/".agents/skills/foreign-neighbor/SKILL.md",
                                       b"FOREIGN USER CONTENT: preserve without edits\n",project)
    did=actual==new and graph_ok
    result,reason=status(True,updated,did,"parent v2 must resolve child B once and deploy child B",
                         protected=foreign_ok)
    if prelock is not None and prelock["exit_code"]!=0 and result=="PASS":
        result="BLOCKED";reason="UAP root intent pin update refused before apply"
    return receipt(product,"R3_transitive_update",result,reason=reason,initial=initial,
                   pin_refresh=prelock,command=updated,foreign_unchanged=foreign_ok,
                   original_sha256=h(old),expected_sha256=h(new),
                   actual_sha256=h(actual) if actual is not None else None,
                   child_v2_locked=graph_ok)


def case_interrupted(product: str, cli: str, root: Path, commit: str) -> dict:
    """No probabilistic SIGKILL as alleged post-commit evidence: fail closed."""
    env=baseline.isolated_env(root)
    project,original=baseline.make_project(root)
    prepare_project(product,project,commit)
    argv=baseline.tool_args(product,cli,project,commit)
    # Preflight is a *separate* real negative control, not a forged crash proof.
    # A safe isolated error is introduced via a malformed existing lock.
    file=project / ("uap.lock" if product=="uap" else "apm.lock.yaml")
    file.write_text("{invalid: unresolved: [\n")
    before=clean_snapshot(project,(str(file.relative_to(project)),
                        ".agents/skills/foreign-neighbor/SKILL.md"))
    control=baseline.run_process(argv,project,env,root)
    after=clean_snapshot(project,(str(file.relative_to(project)),
                        ".agents/skills/foreign-neighbor/SKILL.md"))
    foreign_ok=baseline.safe_same_bytes(
        project/".agents/skills/foreign-neighbor/SKILL.md",original,project)
    if not foreign_ok or before!=after:
        label="FAIL";reason="negative preflight unexpectedly touched foreign content/lock"
    elif control["exit_code"]==0:
        label="FAIL";reason="invalid pre-existing lock was silently accepted"
    else:
        label="BLOCKED";reason="preflight negative passed, but no deterministic post-write crash barrier available"
    return receipt(product,"R4_interrupted_operation",label,reason=reason,
                   controlled_preflight=control,preflight_untouched=before==after,
                   deterministic_post_write_barrier=False,crash_recovery="NOT_RUN")


def case_secret(product: str, cli: str, root: Path) -> dict:
    env=baseline.isolated_env(root)
    project,foreign=baseline.make_project(root)
    name="riskcanary"
    sentinel="risk-" + secrets.token_hex(24)
    placeholder="$"+"{"+MARKER+"}"
    env[MARKER]=sentinel
    command=["node","-e","process.exit(0)"]
    if product=="uap":
        package=project / "package"
        package.mkdir()
        (package/"plugin.json").write_text(json.dumps({
            "name":"risk-mcp", "version":"0.1.0", "mcpServers":"./mcp.json"})+"\n")
        (package/"mcp.json").write_text(json.dumps({"mcpServers":{
            name:{"command":command[0],"args":command[1:],
                  "env":{MARKER:placeholder}}}})+"\n")
        argv=[cli,"add",str(package),"--target","codex","--scope","project",
              "--project-root",str(project),"--yes","--format","json"]
    else:
        (project/"apm.yml").write_text(
            "name: secret-fixture\nversion: 1.0.0\ntargets: [codex]\n"
            "dependencies:\n  mcp:\n    - name: riskcanary\n"
            "      registry: false\n      transport: stdio\n      command: node\n"
            '      args: ["-e", "process.exit(0)"]\n'
            f'      env:\n        {MARKER}: "{placeholder}"\n')
        argv=[cli,"install","--target","codex"]
    cmd=baseline.run_process(argv,project,env,root)
    # Ensure neither the synthetic value nor external inherited environment leaks.
    cmd["output_tail"]=cmd.get("output_tail","").replace(sentinel,"<redacted-synthetic-marker>")
    targets=[".codex/config.toml","uap.toml","uap.lock","apm.yml","apm.lock.yaml"]
    found=[]
    for name_path in targets:
        data=observed_file(project/name_path,project)
        if data is not None:
            found.append({"file":name_path,"synthetic_plaintext_present":sentinel.encode() in data,
                          "literal_placeholder_present":placeholder.encode() in data,
                          "sha256":h(data)})
    leak=any(row["synthetic_plaintext_present"] for row in found)
    protected=baseline.safe_same_bytes(project/".agents/skills/foreign-neighbor/SKILL.md",
                                      foreign,project)
    config=next((row for row in found if row["file"]==".codex/config.toml"),None)
    if leak or not protected:
        verdict="FAIL";reason="synthetic canary escaped or foreign content was changed"
    elif cmd["exit_code"] != 0:
        verdict="BLOCKED";reason="MCP project route refused, check safe local prerequisite in receipt"
    elif config is None:
        verdict="FAIL";reason="successful MCP install did not materialize project Codex config"
    else:
        verdict="PASS";reason="synthetic canary not persisted; native env forwarding remains unqualified"
    return receipt(product,"R5_mcp_env_secret",verdict,reason=reason,
                   command=cmd,config_files=found,foreign_unchanged=protected,
                   native_mcp_call="NOT_RUN",synthetic_value_not_in_artifact=True)


def two_skills_manifest(product: str, project: Path, commit: str,
                        *, include_secondary: bool = True) -> None:
    if product=="uap":
        s=("schema_version = 1\n\n[dependencies.benchmark]\nkind = \"skill\"\n"
           f'git = "{SOURCE}"\nskill = "benchmark-note"\npath = "{SKILL}"\ncommit = "{commit}"\n')
        if include_secondary:
            s+=("\n[dependencies.secondary]\nkind = \"skill\"\n"
                f'git = "{SOURCE}"\nskill = "benchmark-secondary"\n'
                f'path = "{SKILL_SECOND}"\ncommit = "{commit}"\n')
        s+="\n[targets]\ncodex = ["+('\"benchmark\", \"secondary\"' if include_secondary else '"benchmark"')+"]\n"
        (project/"uap.toml").write_text(s)
    else:
        s=("name: removal-consumer\nversion: 1.0.0\ntargets: [codex]\n"
           "dependencies:\n  apm:\n"
           f"    - {REPO}/{SKILL}#{commit}\n")
        if include_secondary:
            s+=f"    - {REPO}/{SKILL_SECOND}#{commit}\n"
        (project/"apm.yml").write_text(s)


def removal_command(product: str, cli: str, project: Path, *, lock: bool=False) -> list[str]:
    if product=="uap":
        argv=[cli,"deps","lock" if lock else "sync","--scope","project",
              "--project-root",str(project)]
        if not lock:
            argv+=["--yes","--format","json"]
        return argv
    return [cli,"install","--target","codex"]


def case_removed(product: str, cli: str, root: Path, commit: str,
                 expected: dict[str,bytes]) -> dict:
    env=baseline.isolated_env(root)
    author,_=baseline.make_project(root)
    two_skills_manifest(product,author,commit)
    start=baseline.run_process(removal_command(product,cli,author),author,env,root)
    names=("uap.toml","uap.lock") if product=="uap" else ("apm.yml","apm.lock.yaml")
    secondary=author/".agents/skills/benchmark-secondary/SKILL.md"
    second_body=fixture(commit,SKILL_SECOND+"/SKILL.md")
    if (start["exit_code"]!=0 or observed_file(secondary,author)!=second_body
        or any(not (author/name).is_file() for name in names)):
        return receipt(product,"R6_git_pull_removal","BLOCKED",
                       stage="two_dependencies_initial_admission",initial=start,
                       reason="two pinned Skills did not form a supported source graph")
    git("init","-q",cwd=author,env=env)
    git("add",*names,cwd=author,env=env)
    git("commit","-qm","two deps",cwd=author,env=env)
    teammate=root/"teammate"
    git("clone","-q",str(author),str(teammate),cwd=root,env=env)
    sibling = teammate / ".agents/skills/foreign-neighbor/SKILL.md"
    sibling.parent.mkdir(parents=True, exist_ok=True)
    sibling.write_bytes(b"FOREIGN USER CONTENT: preserve without edits\n")
    teammate_root=root/"teammate-iso"
    mate_env=baseline.isolated_env(teammate_root)
    mate_first=baseline.run_process(removal_command(product,cli,teammate),teammate,mate_env,root)
    mate_path=teammate/".agents/skills/benchmark-secondary/SKILL.md"
    if mate_first["exit_code"]!=0 or observed_file(mate_path,teammate)!=second_body:
        return receipt(product,"R6_git_pull_removal","BLOCKED",
                       stage="teammate_initial_install",initial=mate_first,
                       reason="second user agent installation not admitted")
    two_skills_manifest(product,author,commit,include_secondary=False)
    refresh=None
    if product=="uap":
        refresh=baseline.run_process(removal_command(product,cli,author,lock=True),author,env,root)
    changed=baseline.run_process(removal_command(product,cli,author),author,env,root)
    if changed["exit_code"]!=0 or refresh is not None and refresh["exit_code"]!=0:
        return receipt(product,"R6_git_pull_removal","BLOCKED",stage="author_removal",
                       refresh=refresh,command=changed,
                       reason="author could not produce valid remaining lock")
    git("add",*names,cwd=author,env=env)
    git("commit","-qm","drop secondary",cwd=author,env=env)
    git("pull","-q","--ff-only",cwd=teammate,env=mate_env)
    postpull=observed_file(mate_path,teammate)
    audit=None
    if product=="apm":
        audit=baseline.run_process([cli,"audit","--ci","--no-policy"],
                                   teammate,mate_env,root)
    applied=baseline.run_process(removal_command(product,cli,teammate),
                                 teammate,mate_env,root)
    final=observed_file(mate_path,teammate)
    original=observed_file(teammate/".agents/skills/benchmark-note/SKILL.md",teammate)
    foreign=observed_file(teammate/".agents/skills/foreign-neighbor/SKILL.md",teammate)
    preserved=(original==expected["SKILL.md"] and
               foreign==b"FOREIGN USER CONTENT: preserve without edits\n")
    explicitly_reported=bool(audit and audit.get("exit_code")!=0
                             and "orphan" in audit.get("output_tail","").lower())
    # No silent, still agent-visible orphan is allowed after reported success.
    if applied["exit_code"]==0 and final is not None and not explicitly_reported:
        verdict="FAIL";reason="removed dependency silently remains agent-visible after pull/sync"
    elif not preserved:
        verdict="FAIL";reason="surviving Skill or foreign file unexpectedly modified"
    elif applied["exit_code"]==0 and final is None:
        verdict="PASS";reason="removed dependency cleaned and surviving Skill intact"
    elif explicitly_reported:
        verdict="PASS";reason="orphan explicitly diagnosed (deletion not claimed)"
    else:
        verdict="BLOCKED";reason="cleanup rejected or missing evidence; inspect command"
    return receipt(product,"R6_git_pull_removal",verdict,reason=reason,
                   author_action=changed,teammate_install=mate_first,teammate_sync=applied,
                   audit=audit,postpull_before_install_sha256=h(postpull) if postpull else None,
                   stale_skill_still_present=final is not None,
                   survivor_intact=original==expected["SKILL.md"],
                   foreign_unchanged=preserved)


def main() -> int:
    parser=argparse.ArgumentParser()
    parser.add_argument("--uap",type=Path,required=True)
    parser.add_argument("--apm",type=Path,required=True)
    parser.add_argument("--fixture-commit",required=True)
    parser.add_argument("--uap-source-sha",required=True)
    parser.add_argument("--group",choices=sorted(GROUPS),required=True)
    parser.add_argument("--output",type=Path,required=True)
    parser.add_argument("--claude-verified",action="store_true")
    o=parser.parse_args()
    if not all(baseline.PIN.fullmatch(v) for v in
               [o.fixture_commit,o.uap_source_sha,PARENT_V1,PARENT_V2,CHILD_V1,CHILD_V2]):
        parser.error("source pins must be exact immutable 40-character SHAs")
    for name,exe in [("uap",o.uap),("apm",o.apm)]:
        if not exe.is_file() or exe.is_symlink():
            parser.error(f"missing regular {name} executable")
    cases=[]
    meta={"schema_version":1,"group":o.group,"fixture_commit":o.fixture_commit,
          "uap_release_source_sha":o.uap_source_sha,
          "harness_sha256":h(Path(__file__).read_bytes()),
          "released_versions":{"uap":"0.1.95 (source build)","apm":"0.35.0"},
          "limitations":["No production credentials or model calls",
                         "No native agent session / MCP tools/call",
                         "Risk cases with missing prerequisites are BLOCKED, not product FAIL",
                         "No proof of OS mount namespace filesystem confinement"]}
    o.output.mkdir(parents=True,exist_ok=True)
    def save():
        data=dict(meta,cases=cases)
        (o.output/"report.json").write_text(json.dumps(data,indent=2,sort_keys=True)+"\n")
        rows=[f"# UAP / APM risk matrix — {o.group} (evidence, not a winner)",
              "","| Scenario | UAP | APM |","| --- | --- | --- |"]
        for case in GROUPS[o.group]:
            vals=[next((x["status"] for x in cases if x["product"]==product and
                        x["scenario"]==case),"NOT_RUN") for product in ("uap","apm")]
            rows.append(f"| {case} | {vals[0]} | {vals[1]} |")
        rows+=["","Only actual read-back receipts qualify. No vendor OAuth or model runs.",""]
        (o.output/"report.md").write_text("\n".join(rows))
    save()
    expected={name:fixture(o.fixture_commit,SKILL+"/"+name)
              for name in baseline.EXPECTED_FILES}
    with tempfile.TemporaryDirectory(prefix="uap-apm-risks-",dir=os.environ.get("RUNNER_TEMP")) as temp:
        for product,cli in (("uap",str(o.uap)),("apm",str(o.apm))):
            for scenario in GROUPS[o.group]:
                root=Path(temp)/product/scenario
                root.mkdir(parents=True)
                try:
                    if scenario=="R1_missing_receipts":
                        entry=case_cold(product,cli,root,o.fixture_commit,expected)
                    elif scenario=="R2_claude_profiles":
                        entry=case_profiles(product,cli,root,o.fixture_commit,expected,
                                            claude_available=o.claude_verified)
                    elif scenario=="R3_transitive_update":
                        entry=case_graph(product,cli,root)
                    elif scenario=="R4_interrupted_operation":
                        entry=case_interrupted(product,cli,root,o.fixture_commit)
                    elif scenario=="R5_mcp_env_secret":
                        entry=case_secret(product,cli,root)
                    else:
                        # The second Skill was introduced after the original
                        # #617 fixture SHA. Use a later immutable source commit.
                        entry=case_removed(product,cli,root,PARENT_V2,expected)
                except Exception as exc:
                    entry=receipt(product,scenario,"BLOCKED",
                                  stage="observer_exception",
                                  reason=f"observer failed closed: {type(exc).__name__}")
                cases.append(entry)
                save()
    # Observer exceptions are infrastructure failures; product FAIL/BLOCKED
    # are evidence statuses and must not turn the workflow itself red.
    return 1 if any(row.get("stage")=="observer_exception" for row in cases) else 0


if __name__=="__main__":
    raise SystemExit(main())
