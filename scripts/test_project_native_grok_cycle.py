"""Authored parser inputs and hostile joins; no native execution/qualification."""
import base64
import copy
import json
import unittest

import project_native_grok_cycle as cycle
import project_native_runtime as runtime

SOURCE = {"source_commit": "a" * 40, "source_tree": "b" * 40}
ROOT, NONCE = "/inert/grok-cycle", "1" * 32


def raw(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":")).encode()


def stream(body):
    return {"bytes": len(body), "sha256": cycle.sha(body), "body": base64.b64encode(body).decode()}


def snap(files, mode=0o600, directories=0o700):
    paths = {"."}
    for path in files:
        paths.add(path)
        paths.update(str(parent) for parent in cycle.PurePosixPath(path).parents)
    result = {}
    for index, path in enumerate(sorted(paths)):
        file = path in files
        entry = {"kind": "file" if file else "directory", "mode": mode if file else directories,
                 "uid": 1000, "gid": 1000, "device": 1, "inode": index + 1, "modified_ns": 100 if file else 0, "changed_ns": 100 if file else 0}
        if file:
            entry["content"] = stream(files[path])
        result[path] = entry
    return result


def mutation(bound, group, sequence, kind, before="", after=""):
    operation = group + "-001"
    suffix = cycle.sha(operation.encode())[:16]
    row = {"operation_id": operation, "operation_group_id": group, "sequence": sequence,
           "mutation_type": kind, "client_binding_id": bound["client_binding_id"], "phase": "committed",
           "active_path": bound["target_locator"], "backup_path": str(cycle.PurePosixPath(bound["target_locator"]).parent / (".agentplugins-backup-" + suffix))}
    if kind == "directory_swap": row["staging_path"] = bound["native_profile_root"] + "/.agentplugins-staging-" + suffix
    if before: row["before_digest"] = before
    if after: row["after_digest"] = after
    return row


def inputs():
    """A complete independent authored container, never genuine hosted proof."""
    paths = {"home": ROOT + "/home", "project_a": ROOT + "/project", "project_b": ROOT + "/project-b", "outside": ROOT + "/outside"}
    config = b'[marketplace]\ndefault_skills_installs_purged = true\n[plugins]\nenabled = ["uap-grok-cycle-a", "uap-grok-cycle-b", "uap-grok-original-user"]\ndisabled = ["uap-grok-foreign-a", "uap-grok-foreign-b"]\n[features]\nturn_summary = false\n'
    trust = "".join('[folders."' + paths[key] + '"]\ntrusted = true\n' for key in ("project_a", "project_b")).encode()
    fixed = {paths["home"] + "/.grok/config.toml": snap({".": config}), paths["home"] + "/.grok/trusted_folders.toml": snap({".": trust})}
    for who in ("a", "b", "user"):
        name = "uap-grok-original-user" if who == "user" else "uap-grok-foreign-" + who
        native = paths["home"] if who == "user" else paths["project_" + who]
        desc = "Original user " + NONCE if who == "user" else "Foreign inert marker"
        marker = "USER_" + NONCE if who == "user" else "FOREIGN_" + who + "_" + NONCE
        fixed[native + "/.grok/plugins/" + name] = snap({".grok-plugin/plugin.json": raw({"name": name, "version": "0.9.0", "skills": "./skills/"}), "skills/" + name + "/SKILL.md": f"---\nname: {name}\ndescription: {desc}\n---\n{marker}\n".encode()})
        if who != "user":
            fixed[native + "/.uap/plugin-data/caller-owned"] = snap({"keep.txt": f"CALLER_{who}_{NONCE}\n".encode()})
    proof, current = {}, {}
    for stage in cycle.PROOF:
        if stage == "fixture":
            proof[stage] = {"root": ROOT, **paths, "nonce": NONCE, "fixed": fixed}
            continue
        if stage.startswith("source-"):
            _, who, revision = stage.split("-", 2)
            files = cycle.authored(who, revision, NONCE)
            proof[stage] = {"scope": who, "source": ROOT + "/cycle-source-" + who, "revision": revision, "digest": cycle.tree_digest(files, True), "files": snap(files)}
            continue
        if stage == "final":
            proof[stage] = {"preserved": fixed, **{"retained_" + who: current[who]["payload"] for who in ("a", "b")}, **{"absent_" + who: current[who]["binding"]["target_locator"] for who in ("a", "b")}}
            continue
        if stage == "damaged-a":
            proof[stage] = {"scope": "a", "removed_path": current["a"]["binding"]["target_locator"], "before": current["a"]["payload"], "absent": True, "preserved": fixed}
            continue
        who = "b" if stage.endswith("-b") else "a"
        if stage in ("repeat-add-a", "unchanged-update-a", "before-source-revision-a", "before-missing-a", "repeat-remove-a"):
            proof[stage] = copy.deepcopy(current[who])
            continue
        revision = "b-one" if who == "b" else "a-two" if cycle.PROOF.index(stage) >= cycle.PROOF.index("update-a") else "a-one"
        name = "uap-grok-cycle-" + who
        iid = who * 8 + "-1111-4111-8111-" + who * 12
        physical = name + "-" + cycle.sha(iid.encode())[:12]
        active = paths["project_" + who] + "/.grok/plugins/" + physical
        bid = "client_" + cycle.sha("\0".join((iid, "grok", "project", active)).encode())[:24]
        source = proof["source-" + who + "-" + revision]
        manifest = "sha256:" + cycle.sha(cycle.authored(who, revision, NONCE)["plugin.json"])
        stamp = "2026-10-28T00:00:00Z"
        b = {"client_binding_id": bid, "client_id": "grok", "scope": "project", "target_locator": active, "physical_artifact_id": physical,
             "materialization": "materialized", "activation": "prepared", "authentication": "not_checked", "policy": "allowed", "verification": "package_validated",
             "affected_surfaces": ["grok"], "native_profile_root": paths["project_" + who] + "/.grok", "updated_at": stamp,
             "package_revision": {"version": "1.0.0", "tree_digest": source["digest"], "manifest_digest": manifest},
             "native_objects": [{"object_id": "package:grok:" + physical, "kind": "managed_package_directory", "logical_name": name, "path": active, "protection_class": "managed", "managed_digest": cycle.tree_digest(cycle.projected(who, revision, NONCE), False)}]}
        group = "op-" + cycle.sha(stage.encode())[:32]
        removed = stage.startswith("remove-")
        previous = current.get(who)
        if removed:
            b = copy.deepcopy(previous["binding"])
        else:
            b["receipts"] = copy.deepcopy(previous["binding"]["receipts"]) if previous else []
            before = previous["binding"]["native_objects"][0]["managed_digest"] if previous and stage != "missing-repair-a" else ""
            b["receipts"].append(mutation(b, group, len(b["receipts"]) + 1, "directory_swap", before, b["native_objects"][0]["managed_digest"]))
        ins = {"installation_id": iid, "declared_name": name, "origin_mode": "direct", "operation_group_id": group, "created_at": stamp, "updated_at": stamp,
               "source": {"source_binding_id": "src_" + cycle.sha((source["source"] + "\0\0").encode())[:24], "requested_source": source["source"], "canonical_source": source["source"], "resolved_revision": "", "tree_digest": source["digest"]},
               "package": {"loader_kind": "agent_plugins", "format_id": "agent-plugins/1.0.0", "schema_uri": cycle.SCHEMA, "declared_name": name, "version": "1.0.0", "manifest_digest": manifest, "inventory": {"mcp_present": False, "mcp_enabled": False, "skills": [name]}}, "clients": {} if removed else {bid: b}}
        payload = snap(cycle.projected(who, revision, NONCE), 0o644, 0o755)
        if removed:
            did = "data_" + cycle.sha("\0".join((iid, physical, "project")).encode())[:24]
            path = paths["project_" + who] + "/.uap/plugin-data/" + physical
            receipt = {"data_receipt_id": did, "physical_backend_id": physical, "scope": "project", "locator": path, "ownership_digest": "sha256:" + cycle.sha(("agentplugins-plugin-data-v1\0" + did + "\0" + path).encode()), "state": "owned", "created_at": stamp, "updated_at": stamp}
            ins.update(data_retained=True, data_receipts={did: receipt})
            payload = snap({".agentplugins-data-owner.json": raw(receipt) + b"\n"})
        state = {"schema_version": 5, "installations": [ins]}
        if removed:
            state["transaction_receipts"] = [mutation(b, group, len(b["receipts"]) + 1, "directory_remove", b["native_objects"][0]["managed_digest"])]
        value = {"scope": who, "revision": revision, "removed": removed, "state": stream(raw(state)), "binding": b, "source_digest": source["digest"], "payload": payload, "preserved": fixed}
        proof[stage], current[who] = value, value
    return paths, proof


def container():
    paths, proof = inputs()
    public, inspections = [], []
    def command(argv, cwd, out=b"", err=b"", exit_code=0):
        return {"argv": argv, "cwd": cwd, "exit": exit_code, "error": False, "deadline": False, "elapsed_ms": 10,
                "cleanup": "full descendant supervision; forced cleanup fails", "stdout": stream(out), "stderr": stream(err)}
    for stage in cycle.PUBLIC:
        who = "b" if stage.endswith("-b") else "a"
        cwd = paths["outside"] if stage in ("git-outside", "version") else paths["project_" + who]
        out, err, exit_code = b"", b"", 0
        if stage.startswith("git-"):
            argv = ["/held/git", "-c", "core.hooksPath=/dev/null", "init", "--quiet"]
        elif stage == "version":
            argv, out = ["/held/grok", "--version"], b"grok 1.0.70 (f50904eaee6c)\n"
        else:
            verb = "repair" if "repair" in stage else stage.removeprefix("repeat-").removeprefix("unchanged-").split("-")[0]
            p = proof[stage]
            ins = cycle.load(cycle.stream(p["state"]))["installations"][0]
            name = "uap-grok-cycle-" + who
            selector = ROOT + "/cycle-source-" + who if verb == "add" or stage == "update-a" else ins["installation_id"] if verb == "repair" else name
            argv = ["/held/installer", verb, selector] + (["--no-save"] if verb == "add" else []) + ["--scope", "project", "--project-root", cwd, "--target", "grok", "--format", "json"]
            if stage == "repeat-remove-a":
                exit_code, err = 1, b'agentplugins: plugin is not installed for target "grok" in project scope; no target was changed\n'
            else:
                group = "op-" + cycle.sha(stage.encode())[:32]
                output = {"operation_id": group, "plugin": name, "version": "1.0.0", "source": "direct local source", "tree_digest": p["source_digest"], "manifest_digest": ins["package"]["manifest_digest"], "dry_run": False,
                          "result": {"installation_id": ins["installation_id"], "requires_confirmation": False, "mutated": stage not in ("repeat-add-a", "unchanged-update-a")}}
                target = {"target": "grok", "status": "external_completed", "output": output}
                if verb == "update": target["selected"] = True
                data = {"operation_id": group, "batch": True, "status": "data_retained" if verb == "remove" else "completed", "succeeded": 1, "failed": 0, "plugin": name, "dry_run": False, "targets": [target]}
                if verb == "remove":
                    receipt = next(iter(ins["data_receipts"].values()))
                    data.update(plugin_data_preserved=True, data_retained=True, retained_data=[{key: receipt[key] for key in ("data_receipt_id", "physical_backend_id", "scope", "state")}])
                out = raw({"schema_version": 1, "command": verb, "result": "success", "data": data})
                if stage.startswith("add-"): err = f"Resolving and validating one Agent Plugin package for every selected target...\nInstalling {name} for 1 selected clients...\n".encode()
                elif stage == "repeat-add-a": err = b"Resolving and validating one Agent Plugin package for every selected target...\nApplying transactional client package...\n"
                elif verb == "update": err = b"Resolving and validating one updated Agent Plugin package for every selected target...\n"
                elif verb == "repair": err = b"Resolving and validating each unique exact installed package revision once...\n"
        public.append({"stage": stage, "value": command(argv, cwd, out, err, exit_code)})
    for stage in cycle.INSPECT:
        who = "outside" if stage.endswith("outside") else stage[-1]
        cwd = paths["outside"] if who == "outside" else paths["project_" + who]
        selected = [("uap-grok-original-user", "user", paths["home"] + "/.grok/plugins/uap-grok-original-user", "Original user " + NONCE, True)]
        if who != "outside":
            name = "uap-grok-foreign-" + who
            selected.append((name, "project", cwd + "/.grok/plugins/" + name, "Foreign inert marker", False))
            if stage in ("installed-a", "installed-b", "stable-a", "updated-a", "updated-b", "repaired-a", "remaining-b"):
                key = "add-b" if who == "b" else "update-a" if stage in ("updated-a", "repaired-a") else "add-a"
                p = proof[key]
                selected.append(("uap-grok-cycle-" + who, "project", p["binding"]["target_locator"], p["revision"] + " " + NONCE, True))
        plugins = [{"name": name, "scope": scope, "path": path, "enabled": True, "provides": {"skills": 1, "agents": 0, "hooks": False, "mcpServers": 0}} for name, scope, path, _, _ in selected]
        skills = [{"name": name, "description": desc, "source": {"type": "plugin", "plugin_name": name, "path": path + "/skills/" + name + "/SKILL.md"}, "userInvocable": True} for name, _, path, desc, enabled in selected if enabled]
        out = raw({"grokVersion": "1.0.70", "cwd": cwd, "projectRoot": cwd + "/", "projectTrusted": who != "outside", "mcpServers": [], "plugins": plugins, "skills": skills})
        inspections.append({"stage": stage, "value": command(["/held/grok", "inspect", "--json"], cwd, out)})
    result = {"client": "grok", "version": "1.0.70", "capture_mode": cycle.MODE, "attempt_stage": "complete-cycle", "runtime_attempt_complete": True,
              "activation": "prepared", "vendor_authentication_qualified": False, "real_model_qualified": False, "test_failed": False, "cycle_observed_complete": True, "elapsed_ms": 1000,
              "qualification": {key: key in cycle.FLAGS[:2] for key in cycle.FLAGS}, "source_identity": {"installer_base_commit": SOURCE["source_commit"], "installer_tree": SOURCE["source_tree"], "installer_source_state": "committed", "installer_patch_sha256": []},
              "cycle_public_stages": list(cycle.PUBLIC), "cycle_inspect_stages": list(cycle.INSPECT), "cycle_proof_stages": list(cycle.PROOF),
              "acquisition": {"client": "grok", "node": None, "acquisition": {"sha256": "11d27a30adfb75c4b9b180072ab27412a1916f054ff5497ab70fb9a77d304bcc", "pin_verified": True,
                  "members": [{"Name": "grok", "Kind": "file", "Bytes": 195700192, "sha256": "22d281c5964c5444147328096ba2a221f3f0874c6858e27c825656a80f6e58e9"}]}, "runtime_files": [{"path": "/held/grok", "sha256": "22d281c5964c5444147328096ba2a221f3f0874c6858e27c825656a80f6e58e9"}]},
              "scanner": {"archive_sha256": "2b3d176db752433b904a4b42375543ff398f4841d22e48f7d4f23ded925b72da", "binary_sha256": "b6723451f155eb49e84acb34d1f06de929bddfcff6238cd4e38cdc6519396fba", "platform": "linux-amd64", "route": "prepopulated supported scanner cache; ordinary CLI security assessment", "source_tag": "v0.1.3", "version": "0.1.3"}}
    rows = public, inspections, [{"stage": stage, "value": proof[stage]} for stage in cycle.PROOF]
    bodies = {name: b"".join(raw(row) + b"\n" for row in captured) for name, captured in zip(cycle.MEMBERS, rows)}
    bodies["runtime-attempt.json"] = raw(result)
    return bodies


class GrokCycleContracts(unittest.TestCase):
    def test_complete_four_member_parser_input_closes_only_two_flags(self):
        bodies = container()
        result = runtime.cli_attempt_members(bodies, SOURCE, "grok")
        self.assertTrue(result["runtime_attempt_complete"])
        self.assertEqual(sum(result["qualification"].values()), 2)
        self.assertEqual(len(bodies), 4)
        self.assertFalse(result["real_model_qualified"])
        self.assertFalse(result["vendor_authentication_qualified"])

    def test_missing_stage_failed_command_forged_qualification_and_foreign_scope_refuse(self):
        for fault in ("stage", "command", "cwd", "elapsed", "flag", "source", "extra", "missing-proof", "native-absent", "public-id"):
            bodies = container()
            result = cycle.load(bodies["runtime-attempt.json"])
            if fault == "flag": result["qualification"]["mcp-tools-call"] = True
            elif fault == "source": result["source_identity"]["installer_tree"] = "0" * 40
            elif fault == "extra": bodies["unrecorded.log"] = b"extra"
            elif fault == "missing-proof": bodies[cycle.MEMBERS[2]] = b""
            else:
                name = cycle.MEMBERS[1] if fault == "native-absent" else cycle.MEMBERS[0]
                rows = [cycle.load(line) for line in bodies[name].splitlines()]
                if fault == "stage": rows.pop()
                elif fault == "command": rows[0]["value"]["error"] = True
                elif fault == "cwd": rows[0]["value"]["cwd"] = "/foreign"
                elif fault == "elapsed": rows[0]["value"]["elapsed_ms"] = 35001
                elif fault == "native-absent":
                    native = cycle.load(cycle.stream(rows[-1]["value"]["stdout"]))
                    native["skills"].append(copy.deepcopy(native["skills"][0]))
                    rows[-1]["value"]["stdout"] = stream(raw(native))
                else:
                    output = cycle.load(cycle.stream(rows[4]["value"]["stdout"]))
                    output["data"]["targets"][0]["output"]["result"]["installation_id"] = "foreign"
                    rows[4]["value"]["stdout"] = stream(raw(output))
                bodies[name] = b"".join(raw(row) + b"\n" for row in rows)
            bodies["runtime-attempt.json"] = raw(result)
            with self.subTest(fault=fault), self.assertRaises(ValueError): runtime.cli_attempt_members(bodies, SOURCE, "grok")
        with self.assertRaises(ValueError): runtime.cli_attempt_members(container(), SOURCE, "qoder")

    def test_authored_complete_source_state_projection_and_real_retained_shape(self):
        paths, proof = inputs()
        parsed_paths, nonce, completed = cycle.proofs([{"stage": stage, "value": proof[stage]} for stage in cycle.PROOF])
        self.assertEqual(paths, parsed_paths)
        self.assertEqual(nonce, NONCE)
        self.assertTrue(completed["remove-a"]["removed"])
        self.assertEqual(completed["missing-repair-a"]["source_digest"], completed["update-a"]["source_digest"])
        self.assertNotEqual(completed["add-a"]["source_digest"], completed["update-a"]["source_digest"])

    def test_source_state_native_owner_and_original_authority_mutations_refuse(self):
        for fault in ("source-digest", "foreign-inode", "foreign-bytes", "binding-path", "scope", "native-digest", "retained-data", "missing-partial", "final-stale", "changed-noop"):
            _, p = inputs()
            if fault == "source-digest": p["source-a-a-two"]["digest"] = "sha256:" + "0" * 64
            elif fault == "foreign-inode":
                key = ROOT + "/home/.grok/plugins/uap-grok-original-user"
                p["update-a"]["preserved"] = copy.deepcopy(p["update-a"]["preserved"])
                p["update-a"]["preserved"][key]["."]["inode"] += 1
            elif fault == "foreign-bytes":
                key = ROOT + "/project/.uap/plugin-data/caller-owned"
                p["fixture"]["fixed"][key]["keep.txt"]["content"] = stream(b"foreign")
            elif fault == "binding-path": p["update-a"]["binding"]["target_locator"] = "/foreign"
            elif fault == "scope": p["update-a"]["scope"] = "b"
            elif fault == "native-digest": p["add-a"]["binding"]["native_objects"][0]["managed_digest"] = "sha256:" + "0" * 64
            elif fault == "retained-data": p["remove-a"]["payload"][".agentplugins-data-owner.json"]["content"] = stream(b"{}")
            elif fault == "missing-partial": p["damaged-a"]["removed_path"] += "/skills/uap-grok-cycle-a/SKILL.md"
            elif fault == "final-stale": p["final"]["absent_a"] = "/foreign"
            else: p["repeat-add-a"]["state"] = stream(cycle.stream(p["repeat-add-a"]["state"]) + b" ")
            with self.subTest(fault=fault), self.assertRaises(ValueError):
                cycle.proofs([{"stage": stage, "value": p[stage]} for stage in cycle.PROOF])

    def test_transaction_receipt_types_paths_sequences_history_and_public_groups_refuse(self):
        faults = ("type", "staging", "backup", "active", "operation", "group", "sequence-bool", "sequence", "phase", "binding", "before", "after", "extra", "history", "count", "state-receipt", "remove-staging", "remove-after", "remove-backup", "remove-sequence", "latest-group")
        for fault in faults:
            _, proof = inputs()
            stage = "remove-a" if fault.startswith("remove-") or fault == "state-receipt" else "update-a"
            value = proof[stage]
            state = cycle.load(cycle.stream(value["state"]))
            bound = value["binding"]
            target = state["transaction_receipts"][0] if stage == "remove-a" else bound["receipts"][-1]
            changes = {"type": ("mutation_type", "foreign_write"), "staging": ("staging_path", "/foreign"), "backup": ("backup_path", "/foreign"), "active": ("active_path", "/foreign"),
                       "operation": ("operation_id", target["operation_group_id"] + "-002"), "group": ("operation_group_id", "op-" + "0" * 32), "sequence-bool": ("sequence", True), "sequence": ("sequence", 1),
                       "phase": ("phase", "state_committed"), "binding": ("client_binding_id", "foreign"), "before": ("before_digest", "sha256:" + "0" * 64), "after": ("after_digest", "sha256:" + "0" * 64), "extra": ("foreign", True),
                       "remove-staging": ("staging_path", "/foreign"), "remove-after": ("after_digest", target.get("before_digest")), "remove-backup": ("backup_path", "/foreign"), "remove-sequence": ("sequence", 1)}
            if fault in changes:
                key, change = changes[fault]
                target[key] = change
            elif fault == "history":
                old = bound["receipts"][0]
                bound["receipts"][0] = mutation(bound, "op-" + "0" * 32, 1, "directory_swap", "", old["after_digest"])
            elif fault == "count": bound["receipts"].pop(0)
            elif fault == "state-receipt": state["transaction_receipts"] = []
            else: state["installations"][0]["operation_group_id"] = "op-" + "0" * 32
            if stage != "remove-a": state["installations"][0]["clients"][bound["client_binding_id"]] = copy.deepcopy(bound)
            value["state"] = stream(raw(state))
            with self.subTest(fault=fault), self.assertRaises(ValueError):
                cycle.proofs([{"stage": label, "value": proof[label]} for label in cycle.PROOF])
        for fault in ("missing", "foreign", "output", "reuse"):
            bodies = container()
            rows = [cycle.load(line) for line in bodies[cycle.MEMBERS[0]].splitlines()]
            row = rows[6 if fault == "reuse" else 4]["value"]
            document = cycle.load(cycle.stream(row["stdout"]))
            if fault == "missing": del document["data"]["operation_id"]
            elif fault == "foreign": document["data"]["operation_id"] = "op-" + "0" * 32
            elif fault == "output": document["data"]["targets"][0]["output"]["operation_id"] = "op-" + "0" * 32
            else:
                previous = cycle.load(cycle.stream(rows[4]["value"]["stdout"]))["data"]["operation_id"]
                document["data"]["operation_id"] = previous
                document["data"]["targets"][0]["output"]["operation_id"] = previous
            row["stdout"] = stream(raw(document))
            bodies[cycle.MEMBERS[0]] = b"".join(raw(item) + b"\n" for item in rows)
            with self.subTest(fault=fault), self.assertRaises(ValueError): cycle.members(bodies, SOURCE)

    def test_failed_prefix_retains_unknown_native_output_without_qualification(self):
        bodies = container()
        summary = cycle.load(bodies["runtime-attempt.json"])
        summary.update(runtime_attempt_complete=False, cycle_observed_complete=False, test_failed=True, qualification={key: False for key in cycle.FLAGS})
        public = [cycle.load(line) for line in bodies[cycle.MEMBERS[0]].splitlines()][:4]
        native = [cycle.load(bodies[cycle.MEMBERS[1]].splitlines()[0])]
        native[0]["value"]["stdout"] = stream(b'{"unknownShippingShape":true}')
        for name, rows, field in zip(cycle.MEMBERS, (public, native, []), ("cycle_public_stages", "cycle_inspect_stages", "cycle_proof_stages")):
            bodies[name] = b"".join(raw(row) + b"\n" for row in rows)
            summary[field] = [row["stage"] for row in rows]
        bodies["runtime-attempt.json"] = raw(summary)
        self.assertEqual(cycle.members(bodies, SOURCE), summary)
        self.assertFalse(any(summary["qualification"].values()))
        self.assertEqual(cycle.stream(cycle.load(bodies[cycle.MEMBERS[1]].splitlines()[0])["value"]["stdout"]), b'{"unknownShippingShape":true}')

    def test_raw_and_encoded_private_controls_before_base64_and_after_decode(self):
        for body in (b"uap-grok-private-inert", b'{"key":"\\u0075ap-grok-private-inert"}', b'{}\n{"key":"\\u0075ap-grok-private-inert"}\n'):
            self.assertTrue(runtime.grok_private_credential(body))
            with self.assertRaises(ValueError): cycle.stream(stream(body))
        self.assertEqual(cycle.stream(stream(b"ordinary\n")), b"ordinary\n")
        for key, value in (("bytes", True), ("sha256", "0" * 64), ("body", "Zh=="), ("extra", False)):
            altered = {**stream(b"f"), key: value}
            with self.assertRaises(ValueError): cycle.stream(altered)

    def test_whole_native_catalog_joins_and_disabled_foreign_not_trust_bit(self):
        expected = [("user", "user", "/owned/home/plugin", "Original marker", 1, True), ("foreign", "project", "/owned/A/foreign", "Foreign inert marker", 1, False)]
        row = {"grokVersion": "1.0.70", "cwd": "/owned/A", "projectRoot": "/owned/A/", "projectTrusted": True, "mcpServers": [],
               "plugins": [{"name": name, "scope": scope, "path": path, "enabled": True, "provides": {"skills": count, "agents": 0, "hooks": False, "mcpServers": 0}} for name, scope, path, _, count, _ in expected],
               "skills": [{"name": "user", "description": "Original marker", "userInvocable": True, "source": {"type": "plugin", "plugin_name": "user", "path": "/owned/home/plugin/skills/user/SKILL.md"}}]}
        cycle.inspect(raw(row), "/owned/A", True, expected)
        for fault in ("foreign-loaded", "source", "alias", "duplicate", "count", "trusted", "disabled"):
            bad = copy.deepcopy(row)
            if fault == "foreign-loaded": bad["skills"].append({**bad["skills"][0], "name": "foreign"})
            elif fault == "source": bad["skills"][0]["source"]["path"] = "/foreign"
            elif fault == "alias": bad["ProjectTrusted"] = False
            elif fault == "duplicate": bad["plugins"][1] = bad["plugins"][0]
            elif fault == "count": bad["plugins"][0]["provides"]["skills"] = True
            elif fault == "trusted": bad["projectTrusted"] = False
            else: bad["skills"][0]["disabled"] = True
            with self.subTest(fault=fault), self.assertRaises(ValueError): cycle.inspect(raw(bad), "/owned/A", True, expected)


if __name__ == "__main__":
    unittest.main()
