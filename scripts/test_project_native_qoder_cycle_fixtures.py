"""Authored finite parser inputs, never genuine hosted qualification evidence."""
import base64
import copy
from datetime import datetime, timedelta, timezone
import json

import project_native_qoder_cycle as cycle
import project_native_qoder_cycle_logs as logs
import project_native_qoder_cycle_proof as proof

SOURCE = dict(source_commit="a" * 40, source_tree="b" * 40)
ROOT, NONCE = "/inert/qoder-cycle", "1" * 32
LAYOUT = dict(root=ROOT, home=ROOT + "/home", a=ROOT + "/project", b=ROOT + "/project-b", outside=ROOT + "/outside", installer="/held/installer", client="/held/qodercli", git="/held/git")


def raw(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":")).encode()


def stream(body):
    return dict(bytes=len(body), sha256=cycle.sha(body), body=base64.b64encode(body).decode())


def mutation(bound, group, sequence, kind, before="", after=""):
    operation = group + "-001"
    suffix, parent = cycle.sha(operation.encode())[:16], str(cycle.PurePosixPath(bound["target_locator"]).parent)
    result = dict(operation_id=operation, operation_group_id=group, sequence=sequence, mutation_type=kind, client_binding_id=bound["client_binding_id"], active_path=bound["target_locator"], backup_path=parent + "/.agentplugins-backup-" + suffix, phase="committed")
    if kind == "directory_swap": result["staging_path"] = parent + "/.agentplugins-staging-" + suffix
    if before: result["before_digest"] = before
    if after: result["after_digest"] = after
    return result


def state_rows():
    result, current = {}, {}
    for stage in proof.STATE_STAGES:
        key = "b" if stage.endswith("-b") else "a"
        if stage in ("repeat-add-a", "unchanged-update-a", "before-source-revision-a", "before-missing-a", "repeat-remove-a"):
            result[stage] = copy.deepcopy(current[key])
            continue
        revision = "b-one" if key == "b" else "a-two" if stage in ("update-a", "missing-repair-a", "remove-a") else "a-one"
        files = proof.authored(NONCE, key, revision)
        iid = key * 8 + "-1111-4111-8111-" + key * 12
        physical = cycle.NAME + "-" + cycle.sha(iid.encode())[:12]
        native_base = LAYOUT[key] + "/.uap/managed/clients/qoder"
        active = native_base + "/" + physical
        bid = "client_" + cycle.sha("\0".join((iid, "qoder", "project", active)).encode())[:24]
        group, stamp = "op-" + cycle.sha(stage.encode())[:32], "2026-10-28T00:00:00Z"
        source = ROOT + "/cycle-source-" + key
        source_digest, manifest_digest = proof.tree_digest(files, True), "sha256:" + cycle.sha(files["plugin.json"])
        binding = dict(client_binding_id=bid, client_id="qoder", scope="project", target_locator=active, physical_artifact_id=physical, materialization="materialized", activation="prepared", authentication="not_checked", policy="allowed", verification="installation_verified", affected_surfaces=["qoder"], native_profile_root=native_base,
                       package_revision=dict(version="1.0.0", tree_digest=source_digest, manifest_digest=manifest_digest), updated_at=stamp,
                       native_objects=[dict(object_id="package:qoder:" + physical, kind="managed_package_directory", logical_name=cycle.NAME, path=active, managed_digest=proof.tree_digest(proof.projected(NONCE, key, revision)), protection_class="managed")])
        removed = stage in ("remove-a", "remove-b")
        prior = current.get(key)
        if removed:
            binding = copy.deepcopy(prior["binding"])
        else:
            binding["receipts"] = copy.deepcopy(prior["binding"]["receipts"]) if prior else []
            before = prior["binding"]["native_objects"][0]["managed_digest"] if prior and stage != "missing-repair-a" else ""
            binding["receipts"].append(mutation(binding, group, len(binding["receipts"]) + 1, "directory_swap", before, binding["native_objects"][0]["managed_digest"]))
        installation = dict(installation_id=iid, declared_name=cycle.NAME, origin_mode="direct", operation_group_id=group, created_at=stamp, updated_at=stamp,
                            source=dict(source_binding_id="src_" + cycle.sha((source + "\0\0").encode())[:24], requested_source=source, canonical_source=source, resolved_revision="", tree_digest=source_digest),
                            package=dict(loader_kind="agent_plugins", format_id="agent-plugins/1.0.0", schema_uri=cycle.SCHEMA, declared_name=cycle.NAME, version="1.0.0", manifest_digest=manifest_digest, inventory=dict(mcp_present=False, mcp_enabled=False, skills=[cycle.NAME])), clients={} if removed else {bid: binding})
        payload = proof.projected(NONCE, key, revision)
        delivery = dict(active=active, view=active + "/.agentplugins-qoder-native", files=proof.tree(payload, "644", "755"), manifest=stream(payload[".agentplugins-qoder-native/.qoder-plugin/plugin.json"]), skill=stream(files["skills/" + cycle.NAME + "/SKILL.md"]))
        data_path = LAYOUT[key] + "/.uap/plugin-data/" + physical
        data = dict(allocated=False, path=data_path, receipt=None, owner=None, files=None)
        document = dict(schema_version=5, installations=[installation])
        if removed:
            did = "data_" + cycle.sha("\0".join((iid, physical, "project")).encode())[:24]
            receipt = dict(data_receipt_id=did, physical_backend_id=physical, scope="project", locator=data_path, state="owned", ownership_digest="sha256:" + cycle.sha(("agentplugins-plugin-data-v1\0" + did + "\0" + data_path).encode()), created_at=stamp, updated_at=stamp)
            owner = raw(receipt) + b"\n"
            installation.update(data_retained=True, data_receipts={did: receipt})
            data.update(allocated=True, receipt=receipt, owner=stream(owner), files=proof.tree({".agentplugins-data-owner.json": owner}))
            delivery.update(files=None, manifest=None, skill=None)
            document["transaction_receipts"] = [mutation(binding, group, len(binding["receipts"]) + 1, "directory_remove", binding["native_objects"][0]["managed_digest"])]
        result[stage] = dict(scope=key, revision=revision, removed=removed, source_digest=source_digest, state=stream(raw(document)), binding=binding, delivery=delivery, data=data)
        current[key] = result[stage]
    return result


def proof_rows():
    rows = state_rows()
    rows["fixture"] = dict(nonce=NONCE, layout=LAYOUT, owner_uid=1000, owner_gid=1000)
    for key, revision in (("a", "a-one"), ("b", "b-one"), ("a", "a-two")):
        files = proof.authored(NONCE, key, revision)
        rows["source-" + key + "-" + revision] = dict(source=ROOT + "/cycle-source-" + key, digest=proof.tree_digest(files, True), files=proof.tree(files))
    for key in ("a", "b"):
        body = ("CALLER_" + key + "_" + NONCE + "\n").encode()
        rows["caller-" + key] = dict(path=LAYOUT[key] + "/.uap/plugin-data/caller-owned/keep.txt", body=stream(body), files=proof.tree({"keep.txt": body}))
    registry, enabled, profile = {}, {}, {".": "directory:20000000700", ".auth": "directory:20000000700"}
    for key in ("user", "a", "b"):
        name, path, _, manifest, body, entry = proof.control(LAYOUT, NONCE, key)
        files = {".qoder-plugin/plugin.json": manifest, "skills/" + name + "/SKILL.md": body}
        rows["original-" + key] = dict(key=key, path=path, manifest=stream(manifest), skill=stream(body), files=proof.tree(files), entry=entry)
        registry[name + "@local"], enabled[name + "@local"] = [entry], False
        if key == "user":
            for relative, value in proof.tree(files).items():
                profile["plugins/" + name + ("/" + relative if relative != "." else "")] = ("file:600:" + str(len(files[relative])) + ":" + cycle.sha(files[relative])) if relative in files else value
    reg = raw(dict(version=2, plugins=registry))
    settings = raw(dict(general=dict(enableAutoUpdate=False), enabledPlugins=enabled))
    root = LAYOUT["home"] + "/.qoder"
    rows["original-registry"] = dict(path=root + "/plugins/installed_plugins_v2.json", registry=stream(reg), settings_path=root + "/settings.json", settings=stream(settings))
    profile.update({"settings.json": "file:600:" + str(len(settings)) + ":" + cycle.sha(settings), "plugins": "directory:20000000700", "plugins/installed_plugins_v2.json": "file:600:" + str(len(reg)) + ":" + cycle.sha(reg), "installation_id": "file:644:36:" + cycle.sha(b"anonymous installation UUID fixture"), ".auth/machine_id": "file:600:36:" + cycle.sha(b"anonymous machine UUID fixture"), ".auth/.credential-transaction": "file:600:0:" + cycle.sha(b"")})
    for path in ("tmp", "tmp/telemetry", "tmp/telemetry/active-runs"):
        profile[path] = "directory:20000000700"
    rows["original-profile"] = dict(logs_absent=True, inventory=profile)
    before = rows["before-missing-a"]
    rows["missing-a"] = dict(path=before["delivery"]["active"], source_digest=before["source_digest"], object=before["binding"]["native_objects"][0], before=before["delivery"]["files"], absent=True)
    return rows


def public_output(stage, selected):
    verb = "repair" if "repair" in stage else stage.removeprefix("repeat-").removeprefix("unchanged-").split("-")[0]
    ins = cycle.load(cycle.stream(selected["state"]))["installations"][0]
    group = "op-" + cycle.sha(stage.encode())[:32]
    output = dict(operation_id=group, plugin=cycle.NAME, version="1.0.0", source="local" if verb in ("repair", "remove") else "direct local source", tree_digest=selected["source_digest"], manifest_digest=ins["package"]["manifest_digest"], dry_run=False,
                  result=dict(installation_id=ins["installation_id"], requires_confirmation=False, mutated=stage not in ("repeat-add-a", "unchanged-update-a")))
    if stage == "repeat-add-a":
        del output["operation_id"]
        output["result"]["no_change"] = True
        return raw(dict(schema_version=1, command="add", result="success", data=output))
    target = dict(target="qoder", status="external_completed", output=output)
    if verb == "update": target["selected"] = True
    data = dict(batch=True, status="data_retained" if verb == "remove" else "completed", succeeded=1, failed=0, plugin=cycle.NAME, dry_run=False, targets=[target], operation_id=group)
    if verb == "remove":
        receipt = selected["data"]["receipt"]
        data.update(data_retained=True, plugin_data_preserved=True, retained_data=[{key: receipt[key] for key in ("data_receipt_id", "physical_backend_id", "scope", "state")}])
    return raw(dict(schema_version=1, command=verb, result="success", data=data))


def native_catalog(stage, rows):
    key = cycle.role(stage)
    expected = []
    if cycle.has_flag(stage) and stage in ("installed-a", "installed-b", "stable-a", "updated-a", "updated-b", "repaired-a", "remaining-b"):
        revision = "a-two" if stage in ("updated-a", "repaired-a") else key + "-one"
        expected.append(dict(id=cycle.NAME + "@flag", name=cycle.NAME, source=cycle.NAME + "@inline", version="1.0.0", scope="flag", enabled=True, canDisable=False, installPath=rows["add-" + key]["delivery"]["view"], description=cycle.DESCRIPTION,
                             resources=dict(skills=[dict(name=cycle.NAME, description=f"Disposable Qoder {key} {revision} {proof.nonce(NONCE, key)}")], agents=[], mcpServers=[], commands=[], hooks=[])))
    for who in ("a", "b", "user"):
        name, _, description, _, _, entry = proof.control(LAYOUT, NONCE, who)
        expected.append(dict(id=name + "@local", name=name, source=name + "@local", enabled=False, canDisable=True, description=description,
                             resources=dict(skills=[dict(name=name, description=description)], agents=[], mcpServers=[], commands=[], hooks=[]), **entry))
    return raw(expected)


def log_record(expected, identifiers, when):
    index = len(identifiers)
    pid, text = 1000 + index, when.isoformat(timespec="milliseconds").replace("+00:00", "Z")
    identity = when.strftime("%Y-%m-%dT%H-%M-%S-") + f"{when.microsecond // 1000:03d}Z-{index:x}-p{pid}"
    target = "runs/" + identity
    seconds = int(when.timestamp())
    def metadata(inode, mode, links, size):
        return f"1:{inode}:{mode:o}:{links}:1000:1000:{size}:{seconds}:0:{seconds}:0"
    latest = "symlink:" + metadata(2000 + index, 0o120777, 1, len(target)) + ":target:" + metadata(3000 + index, 0o40755, 2, 4096) + ":" + str(len(target)) + ":" + cycle.sha(target.encode())
    form = "direct" if index % 2 else "compiled-embedded"
    run = dict(run_id=identity, latest_target=target, manifest_bytes=200 + index, manifest_sha256=cycle.sha(("manifest" + str(index)).encode()), pid=pid, ppid=999, started_at=text,
               cli_version="1.1.78", platform="linux", arch="x64", process_role="main", pid_provenance="manifest-only", run_id_matches_manifest=True, cwd_matches=True,
               argv_matches=form == "direct", exact_added_run_closure=True, preexisting_records_stable=True)
    after = dict(expected)
    if not identifiers:
        after.update({"logs": "directory:20000000755", "logs/runs": "directory:20000000755"})
    path = "logs/runs/" + identity
    logbytes, loghash = 123 + index, cycle.sha(("log" + str(index)).encode())
    after.update({path: "directory:20000000755", path + "/manifest.json": "file:644:" + str(run["manifest_bytes"]) + ":" + run["manifest_sha256"],
                  path + "/qodercli.log": "file:644:" + str(logbytes) + ":" + loghash, "logs/latest": latest})
    transition = dict(kind="one-run", before_sha256=cycle.sha(raw(expected)), after_sha256=cycle.sha(raw(after)), argv_form=form, latest_record=latest, run=run, delta=logs.delta(expected, after), log_bytes=logbytes, log_sha256=loghash)
    identifiers.append(identity)
    return transition, after


def container():
    """Full authored parser container, not native observation or release proof."""
    rows = proof_rows()
    selected = {stage: {**row, "view": row["delivery"]["view"], "installation": json.loads(base64.b64decode(row["state"]["body"]))["installations"][0]} for stage, row in rows.items() if stage in proof.STATE_STAGES}
    expected = dict(rows["original-profile"]["inventory"])
    original_hash, identifiers, public, catalogs = cycle.sha(raw(expected)), [], [], []
    origin = datetime(2026, 10, 28, 0, 0, 0, tzinfo=timezone.utc)
    for index, (kind, stage) in enumerate(cycle.ORDER):
        started = origin + timedelta(milliseconds=100 * index)
        ended = started + timedelta(milliseconds=1)
        if kind == "public":
            cwd, argv = cycle.public_args(stage, LAYOUT, selected)
            code, out, err = 0, b"", b""
            if stage == "version": out = b"1.1.78\n"
            elif stage == "repeat-remove-a": code, err = 1, b'agentplugins: plugin is not installed for target "qoder" in project scope; no target was changed\n'
            elif not stage.startswith("git-"): out, err = public_output(stage, rows[stage]), cycle.progress(stage)
        else:
            key = cycle.role(stage)
            cwd, argv = LAYOUT[key], [LAYOUT["client"], "plugins", "list", "--json"]
            if cycle.has_flag(stage): argv += ["--plugin-dir", rows["add-" + key]["delivery"]["view"]]
            code, out, err = 0, native_catalog(stage, rows), b""
        if kind == "catalog" or stage == "version":
            transition, expected = log_record(expected, identifiers, started)
        else:
            transition = dict(kind="unchanged", before_sha256=cycle.sha(raw(expected)), after_sha256=cycle.sha(raw(expected)))
        value = dict(command=dict(argv=argv, cwd=cwd, exit=code, error=False, deadline=False, elapsed_ms=1, cleanup=cycle.CLEANUP),
                     started_at=started.isoformat(timespec="microseconds").replace("+00:00", "Z"), ended_at=ended.isoformat(timespec="microseconds").replace("+00:00", "Z"), stdout=stream(out), stderr=stream(err), log=transition)
        (public if kind == "public" else catalogs).append(dict(stage=stage, value=value))
    rows["log-ledger"] = dict(original_sha256=original_hash, expected_sha256=cycle.sha(raw(expected)), runs=identifiers)
    summary = dict(client="qoder", version="1.1.78", capture_mode=cycle.MODE, activation="prepared", vendor_authentication_qualified=False, real_model_qualified=False,
                   nonce=NONCE, layout=LAYOUT, fixture_authored_updater_config=LAYOUT["home"] + "/.qoder/settings.json", cycle_stage="complete-cycle", cycle_public_stages=list(cycle.PUBLIC), cycle_catalog_stages=list(cycle.CATALOG), cycle_log_runs=21,
                   cycle_observed_complete=True, runtime_attempt_complete=True, test_failed=False, elapsed_ms=5000, qualification={key: key in cycle.FLAGS[:2] for key in cycle.FLAGS},
                   source_identity=dict(installer_base_commit=SOURCE["source_commit"], installer_tree=SOURCE["source_tree"], installer_source_state="committed", installer_patch_sha256=[]),
                   acquisition={"client": "qoder", "acquisition": {"sha256": "5e3213d2d4256c86c5ad2c8bf73fd392f92ba80d0d38f822806542805a48a8ea", "pin_verified": True,
                        "members": [{"Name": "qodercli", "Kind": "file", "Bytes": 182982112, "sha256": "0da0976e13fe00bc28f725d6fe7c2f0fe9fa9a8c37eeb2ee4e80ad4c0fc58953"}]}, "node": None,
                        "runtime_files": [{"path": LAYOUT["client"], "sha256": "0da0976e13fe00bc28f725d6fe7c2f0fe9fa9a8c37eeb2ee4e80ad4c0fc58953"}]},
                   scanner={"archive_sha256": "2b3d176db752433b904a4b42375543ff398f4841d22e48f7d4f23ded925b72da", "binary_sha256": "b6723451f155eb49e84acb34d1f06de929bddfcff6238cd4e38cdc6519396fba", "platform": "linux-amd64", "route": "prepopulated supported scanner cache; ordinary CLI security assessment", "source_tag": "v0.1.3", "version": "0.1.3"})
    aggregates = public, catalogs, [dict(stage=stage, value=rows[stage]) for stage in cycle.PROOF]
    result = {name: b"".join(raw(row) + b"\n" for row in captured) for name, captured in zip(cycle.MEMBERS, aggregates)}
    result["runtime-attempt.json"] = raw(summary)
    return result
