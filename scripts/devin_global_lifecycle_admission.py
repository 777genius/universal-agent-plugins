"""Real public UAP Global lifecycle after the pinned ordinary Devin gate.

Native JSON observation is separate from production's file-only verifier.
This research consumer never attests authentication, model or MCP execution.
"""
import argparse
import copy
import hashlib
import json
import os
from pathlib import Path
import re
import stat

import devin_ordinary_metadata_admission as ordinary

base = ordinary.base
NAME, SKILL = "uap-devin-global", "uap-global-marker"
SERVERS = ("uap_global_stdio", "uap_global_http")
NS = "io.github.777genius.agentplugins/managed-stdio-v1"
SCANNER = (9989584, "b6723451f155eb49e84acb34d1f06de929bddfcff6238cd4e38cdc6519396fba")
POLICY = {"id": "agent-plugin-install", "version": 2,
          "digest": "sha256:9cf869e299d847d7078aeca01f5a182fcdb0144bbf513c83290459991c79e037"}
PUBLIC = ("uap-add", "uap-doctor-installed", "uap-repair", "uap-doctor-repaired", "uap-remove")
NATIVE = tuple(kind + "-" + phase + "-" + scope for phase in ("installed", "repaired", "removed")
               for scope in ("a", "b") for kind in ("mcp", "skills"))


def sha(body):
    return hashlib.sha256(body).hexdigest()


def security_role(value):
    required = {"schema_version", "subject", "scanner", "policy", "outcome", "counts", "scanned_files", "report_digest"}
    base.require(type(value) is dict and required <= set(value) <= required | {"findings", "evidence_source"}
                 and type(value["schema_version"]) is int and value["schema_version"] == 1
                 and value["scanner"] == {"id": "lintai", "version": "0.1.3"} and value["policy"] == POLICY,
                 "Global actual security assessment")
    subject, counts = value["subject"], value["counts"]
    base.require(type(subject) is dict and set(subject) == {"tree_digest", "manifest_digest"}
                 and all(type(part) is str and re.fullmatch(r"sha256:[0-9a-f]{64}", part) for part in subject.values())
                 and type(counts) is dict and set(counts) == {"blocking", "warnings", "total"}
                 and all(type(count) is int and 0 <= count <= 4096 for count in counts.values())
                 and counts["blocking"] == 0 and counts["warnings"] == counts["total"]
                 and value["outcome"] == ("warnings" if counts["warnings"] else "no_blocking_findings")
                 and type(value["scanned_files"]) is int and 0 <= value["scanned_files"] <= 512
                 and type(value["report_digest"]) is str and re.fullmatch(r"sha256:[0-9a-f]{64}", value["report_digest"]),
                 "Global scan subject/counts")
    key = sha("\0".join((subject["tree_digest"], subject["manifest_digest"], "lintai", "0.1.3",
                          POLICY["id"], "2", POLICY["digest"])).encode())
    return "security/assessments/" + key + ".json"


def installer_pin(path, budget):
    base.require(path.is_absolute() and path.resolve(strict=True) == path, "prebuilt installer alias")
    before = path.lstat()
    base.require(stat.S_ISREG(before.st_mode) and before.st_nlink == 1 and before.st_uid == os.getuid()
                 and 0 < before.st_size <= 128 << 20, "prebuilt installer authority")
    fd = base.shared.artifacts.open_member(path)
    try:
        held, digest, count = os.fstat(fd), hashlib.sha256(), 0
        base.require(base.shared.artifacts.same_opened(before, held), "prebuilt installer changed at open")
        while count <= before.st_size:
            budget.check()
            chunk = os.read(fd, min(65536, before.st_size + 1 - count))
            if not chunk: break
            count += len(chunk)
            base.require(count <= before.st_size, "prebuilt installer grew")
            digest.update(chunk)
        identity = lambda value: base.shared.artifacts.identity(value) + (value.st_uid, value.st_gid)
        base.require(count == before.st_size and identity(os.fstat(fd)) == identity(held)
                     and identity(path.lstat()) == identity(before),
                     "prebuilt installer changed")
        return count, digest.hexdigest()
    finally:
        os.close(fd)


def regular(ctx, relative, inventory, *, large=None):
    info = inventory[relative]
    maximum = large[0] if large else 1 << 20
    base.require(len(info) == 9 and stat.S_ISREG(info[2]) and info[3] == 1
                 and info[0] == ctx.original_output_identity[0]
                 and tuple(info[4:6]) == ctx.original_output_identity[3:]
                 and 0 <= info[6] <= maximum, "Global ordinary file authority")
    value = base.read_bootstrap_file(ctx, relative, info, inventory,
                                     maximum=maximum, digest_only=large is not None)
    if large:
        base.require(info[6] == large[0] and value == large[1] and info[2] & 0o111,
                     "Global exact large role differs")
    return value


def capture(ctx, installer_pin, helper=None, *, authorities=None, inventory=None):
    """Hash every fixture file; only two exact large executable roles differ."""
    if inventory is None:
        inventory = base.metadata_inventory(ctx.root, ctx.budget)
    base.require(all(inventory.get(name) == identity for name, identity in (authorities or {}).items()),
                 "Global original directory authority changed")
    result, total = {}, 0
    for name, info in inventory.items():
        row = {"identity": info}
        if len(info) == 9:
            role = ((213734376, base.BINARY) if ctx.root / name == Path(ctx.argv[0]) else
                    installer_pin if helper is not None and name == helper else
                    SCANNER if name == "home/uap-state/security/lintai/0.1.3/linux-amd64/lintai" else None)
            value = regular(ctx, name, inventory, large=role)
            row["sha256"] = value if role else sha(value)
            if not role:
                total += info[6]
                base.require(total <= 16 << 20, "Global ordinary aggregate")
        result[name] = row
    return inventory, result


class Journey:
    def __init__(self, ctx, probes, expected, nonce, effects, installer, source):
        self.ctx, self.probes, self.expected = ctx, probes, copy.deepcopy(expected)
        self.nonce, self.effects, self.installer, self.source = nonce, effects, installer, source
        self.home = ctx.home / "uap-state"
        self.profile = Path(expected["user"]["root"])
        self.authorities = {path.relative_to(ctx.root).as_posix(): base.root_identity(path)
                            for path in (self.home, self.profile, source)}
        effects.verify(base.metadata_inventory(ctx.root, ctx.budget))
        self.cli_pin = installer_pin(installer, ctx.budget)
        ctx.identities[installer] = self.cli_pin[1]
        self.binding, self.identifier, self.helper = None, None, None
        self.data_receipt, self.package_members = None, set()
        self.security, self.assessment_path = None, None
        self.source_bodies = dict(ctx.global_source_bodies)
        self.state_history, self.native_labels, self.public_labels = [], [], []
        self.log_births = {}
        self.effects.max_logs, self.effects.max_members = 20, 36
        for proof in effects.proofs:
            for name, value in proof.items():
                if base.BOOTSTRAP_LOG.fullmatch(name):
                    self.log_births.setdefault(name, {"command": "ordinary-gate", "sha256": value["sha256"]})

    def frame(self, inventory=None):
        return capture(self.ctx, self.cli_pin, self.helper, authorities=self.authorities, inventory=inventory)

    def read(self, path, inventory):
        return regular(self.ctx, path.relative_to(self.ctx.root).as_posix(), inventory)

    def state(self, inventory, *, removed=False):
        value = base.strict_json(self.read(self.home / "state-v2.json", inventory))
        base.require(type(value) is dict and value.get("schema_version") == 5
                     and type(value.get("installations")) is list, "Global state schema")
        if removed:
            base.require(not value["installations"], "Global removal left installation")
            self.closed_home(inventory, removed=True)
            return value
        base.require(len(value["installations"]) == 1, "Global single installation")
        item = value["installations"][0]
        identifier = item.get("installation_id")
        base.require(type(identifier) is str and re.fullmatch(r"[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}", identifier)
                     and item.get("declared_name") == NAME and len(item.get("clients", {})) == 1,
                     "Global installation identity")
        binding = next(iter(item["clients"].values()))
        physical = NAME + "-" + sha(identifier.encode())[:12]
        active = self.home / "managed/clients/windsurf" / physical
        binding_id = "client_" + sha("\0".join((identifier, "windsurf", "user", str(active))).encode())[:24]
        data_id = "data_" + sha("\0".join((identifier, physical, "user")).encode())[:24]
        base.require(binding.get("client_id") == "windsurf" and binding.get("scope") == "user"
                     and set(item["clients"]) == {binding_id} and binding.get("client_binding_id") == binding_id
                     and binding.get("data_receipt_id") == data_id
                     and binding.get("target_locator") == str(active)
                     and binding.get("physical_artifact_id") == physical
                     and binding.get("native_profile_root") == str(self.profile)
                     and binding.get("activation") == "prepared"
                     and binding.get("materialization") == "materialized" and binding.get("policy") == "allowed"
                     and binding.get("activation_attested", False) is False
                     and binding.get("native_activation_attempt", "") == ""
                     and binding.get("verification") == "installation_verified"
                     and binding.get("authentication") == "not_checked", "Global binding authority")
        base.require(self.identifier in (None, identifier), "Global installation replaced")
        source = item.get("source", {})
        base.require(source.get("canonical_source") == str(self.source)
                     and source.get("requested_source") == str(self.source), "Global source identity")
        base.require(self.security is not None and self.security["subject"] == {
            "tree_digest": source.get("tree_digest"), "manifest_digest": item.get("package", {}).get("manifest_digest")},
            "Global scan does not bind installed source")
        objects = binding.get("native_objects")
        wanted = {"devin_global_skill_directory:" + SKILL,
                  *("devin_global_mcp_server:" + name for name in SERVERS)}
        base.require(type(objects) is list and len(objects) == 3
                     and {obj.get("object_id") for obj in objects} == wanted,
                     "Global exact native receipts")
        for obj in objects:
            name, kind = obj["logical_name"], obj["kind"]
            base.require((kind == "devin_global_skill_directory" and name == SKILL
                          and obj.get("source_relative") == "skills/" + SKILL)
                         or (kind == "devin_global_mcp_server" and name in SERVERS
                             and obj.get("source_relative", "") == ""), "Global receipt kind/source")
            path = self.profile / ("skills/" + SKILL if kind == "devin_global_skill_directory" else "mcp_config.json")
            base.require(obj.get("path") == str(path) and obj.get("protection_class") == "managed"
                         and obj.get("user_modified", False) is False and obj.get("before_digest", "") == ""
                         and type(obj.get("managed_digest")) is str and obj["managed_digest"].startswith("sha256:"),
                         "Global native receipt path/digest")
        self.identifier, self.binding = identifier, binding
        receipts = binding.get("receipts")
        base.require(type(receipts) is list and receipts and all(receipt.get("phase") == "committed"
                     and receipt.get("client_binding_id") == binding.get("client_binding_id") for receipt in receipts),
                     "Global committed public receipts")
        receipt = item.get("data_receipts", {}).get(binding.get("data_receipt_id"))
        data = self.home / "plugin-data" / physical
        base.require(type(receipt) is dict and receipt.get("scope") == "user" and receipt.get("state") == "owned"
                     and receipt.get("physical_backend_id") == physical and receipt.get("locator") == str(data)
                     and receipt.get("data_receipt_id") == data_id and receipt.get("ownership_digest")
                     == "sha256:" + sha(("agentplugins-plugin-data-v1\0" + data_id + "\0" + str(data)).encode()),
                     "Global data ownership")
        base.require(self.data_receipt is None or receipt == self.data_receipt, "Global data ownership changed")
        self.data_receipt = receipt
        self.helper = (active / NS / "agentplugins").relative_to(self.ctx.root).as_posix()
        self.verify_payload(active, inventory)
        self.closed_home(inventory)
        return value

    def verify_payload(self, active, inventory):
        source_files = self.source_bodies
        for name, body in source_files.items():
            base.require(self.read(self.source / name, inventory) == body, "Global authored source changed")
        selected = {name[len(str(active.relative_to(self.ctx.root))) + 1:]: info
                    for name, info in inventory.items() if name.startswith(active.relative_to(self.ctx.root).as_posix() + "/")}
        files = {name for name, info in selected.items() if len(info) == 9}
        base.require(files == set(source_files) | {NS + "/agentplugins", NS + "/metadata.json"},
                     "Global package closed membership")
        self.package_members = {str((active / name).relative_to(self.home)) for name in files}
        for name, body in source_files.items():
            if name != "mcp.json":
                base.require(self.read(active / name, inventory) == body, "Global installed source bytes differ")
        metadata = base.strict_json(self.read(active / NS / "metadata.json", inventory))
        base.require(metadata == {"protocol": 1, "sha256": self.cli_pin[1], "cliVersion": "0.1.0-development"},
                     "Global managed helper metadata")
        regular(self.ctx, self.helper, inventory, large=self.cli_pin)
        document = base.strict_json(self.read(active / "mcp.json", inventory))
        base.require(set(document) <= {"$schema", "mcpServers"}
                     and document.get("$schema", "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json")
                     == "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json", "Global projected document settings")
        projected = document["mcpServers"]
        base.require(set(projected) == set(SERVERS), "Global MCP projection membership")
        stdio, http = projected[SERVERS[0]], projected[SERVERS[1]]
        data = self.data_receipt["locator"]
        base.require(stdio.get("command") == str(active / NS / "agentplugins")
                     and stdio.get("args") == ["--internal-stdio-v1", str(active), data, str(active), "plugin", "--",
                                                "python3", str(active / "inert.py")]
                     and stdio.get("env") == {"PLUGIN_ROOT": str(active), "PLUGIN_DATA": data}
                     and http.get("url") == self.probes["user"].url, "Global projected transport")
        native = base.strict_json(self.read(self.profile / "mcp_config.json", inventory))
        base.require(set(native) == {"mcpServers"}, "Global native unexpected root settings")
        entries = native.get("mcpServers", {})
        base.require(set(entries) == set(self.expected["user"]["mcp"]) | set(SERVERS)
                     and all(entries[name] == entry for name, entry in self.expected["user"]["mcp"].items()),
                     "Global foreign MCP changed")
        for name in SERVERS:
            entry = dict(projected[name]); entry.pop("type", None)
            if name == SERVERS[1]: entry["transport"] = "http"
            base.require(entries[name] == entry, "Global native entry differs from installed projection")
        skill = self.profile / "skills" / SKILL / "SKILL.md"
        base.require(self.read(skill, inventory) == source_files["skills/" + SKILL + "/SKILL.md"],
                     "Global projected Skill differs")

    def closed_home(self, inventory, *, removed=False):
        prefix = self.home.relative_to(self.ctx.root).as_posix() + "/"
        files = {name[len(prefix):] for name, info in inventory.items() if name.startswith(prefix) and len(info) == 9}
        data_marker = "plugin-data/" + Path(self.data_receipt["locator"]).name + "/.agentplugins-data-owner.json"
        allowed = {"state-v2.json", "mutation.lock", data_marker,
                   "security/lintai/0.1.3/linux-amd64/lintai"} | (set() if removed else self.package_members)
        if self.assessment_path is not None:
            allowed.add(self.assessment_path)
        base.require(files <= allowed and {"state-v2.json", "mutation.lock", data_marker} <= files,
                     "Global unknown UAP state member")
        marker = base.strict_json(self.read(self.home / data_marker, inventory))
        base.require(marker == self.data_receipt, "Global data marker differs")
        lock = base.strict_json(self.read(self.home / "mutation.lock", inventory))
        base.require(set(lock) == {"pid", "acquired_at"} and type(lock["pid"]) is int and lock["pid"] > 0
                     and type(lock["acquired_at"]) is str and len(lock["acquired_at"]) <= 40,
                     "Global mutation lock shape")
        if self.assessment_path is not None:
            cached = base.strict_json(self.read(self.home / self.assessment_path, inventory))
            expected = dict(self.security); expected.pop("evidence_source", None)
            base.require(cached == expected, "Global source-bound scan cache differs")
        dirs = {"operations-v2", "plugin-data/.agentplugins-data-operations",
                "managed", "managed/clients", "managed/clients/windsurf"}
        for name in allowed:
            dirs.update(str(parent) for parent in Path(name).parents if str(parent) != ".")
        base.require(all(name[len(prefix):] in dirs for name, info in inventory.items()
                         if name.startswith(prefix) and len(info) == 5), "Global unknown UAP directory")

    def public(self, label, argv, *, removed=False):
        before, old = self.frame()
        base.require(installer_pin(self.installer, self.ctx.budget) == self.cli_pin, "prebuilt installer changed before public command")
        env = {**self.ctx.env, "AGENTPLUGINS_HOME": str(self.home),
               "PATH": str(Path(self.ctx.argv[0]).parent) + os.pathsep + self.ctx.env["PATH"]}
        confirmation = [] if label.startswith("uap-doctor") else ["--yes"]
        record, out, _ = self.ctx.command(label, [str(self.installer), *argv, *confirmation, "--target", "windsurf",
                                                  "--scope", "user", "--format", "json"],
                                           self.ctx.projects["user"], env, 90)
        base.require(record["exit"] == 0, "public Global lifecycle refused")
        base.require(installer_pin(self.installer, self.ctx.budget) == self.cli_pin, "prebuilt installer changed during public command")
        response = base.strict_json(out)
        base.require(set(response) == {"schema_version", "command", "result", "data"}
                     and type(response["schema_version"]) is int and response["schema_version"] == 1
                     and response["command"] == argv[0] and response["result"] == "success",
                     "public Global envelope refused")
        if label == "uap-add":
            security = response["data"].get("security")
            self.assessment_path = security_role(security)
            self.security = security
        after = base.metadata_inventory(self.ctx.root, self.ctx.budget)
        base.require(all(after.get(name) == identity for name, identity in self.authorities.items()),
                     "Global original directory authority changed")
        self.verify_public_metadata(before, after, doctor=label.startswith("uap-doctor"))
        state = self.state(after, removed=removed)
        self.state_history.append(state)
        self.public_labels.append(label)
        if label.startswith("uap-doctor"):
            base.require(self.frame()[1] == old, "Global doctor wrote fixture")
        else:
            self.verify_public_delta(before, after, removed)
        base.zero_effects(self.probes, self.ctx.root / "stdio-started")

    def verify_public_metadata(self, before, after, *, doctor=False):
        allowed = {str(self.home.relative_to(self.ctx.root)),
                   str((self.profile / "mcp_config.json").relative_to(self.ctx.root)),
                   str((self.profile / "skills" / SKILL).relative_to(self.ctx.root))}
        for name in before.keys() | after.keys():
            if before.get(name) == after.get(name): continue
            base.require(not doctor and any(name == prefix or name.startswith(prefix + "/") for prefix in allowed),
                         "public Global changed foreign file or doctor metadata")

    def verify_public_delta(self, before, after, removed):
        self.verify_public_metadata(before, after)
        if removed:
            native = base.strict_json(self.read(self.profile / "mcp_config.json", after))
            base.require(native == {"mcpServers": self.expected["user"]["mcp"]}
                         and not (self.profile / "skills" / SKILL).exists(), "Global exact removal differs")

    def native(self, phase):
        for scope in ("a", "b"):
            for kind in ("mcp", "skills"):
                label = kind + "-" + phase + "-" + scope
                before, old = self.frame()
                record, out, _ = self.ctx.probe(label, [kind, "list", "--json"], project=scope)
                base.require(record["exit"] == 0, "Global native inventory refused")
                after = base.metadata_inventory(self.ctx.root, self.ctx.budget)
                self.verify_native_metadata(before, after)
                after, now = self.frame(after)
                self.ctx.summary.setdefault("global_native_effects", []).append({"label": label,
                    "changes": {name: {"before": old.get(name), "after": now.get(name)}
                                for name in old.keys() | now.keys() if old.get(name) != now.get(name)}})
                self.verify_native_delta(before, after, old, now, label)
                rows = base.strict_json(out)
                if kind == "mcp":
                    owned = [row for row in rows if row.get("name") in SERVERS]
                    base.require(len(owned) == (0 if phase == "removed" else 2), "Global native MCP membership")
                    if owned:
                        inventory = base.metadata_inventory(self.ctx.root, self.ctx.budget)
                        entries = base.strict_json(self.read(self.profile / "mcp_config.json", inventory))["mcpServers"]
                        expected = copy.deepcopy(self.expected)
                        expected["user"]["mcp"].update({name: entries[name] for name in SERVERS})
                    else: expected = self.expected
                    base.verify_metadata_sources(out, self.ctx, expected, scope)
                else:
                    selected = [row for row in rows if row.get("base_dir") == str(self.profile / "skills" / SKILL)]
                    base.require(len(selected) == (0 if phase == "removed" else 1), "Global native Skill membership")
                    for row in selected:
                        base.require(row.get("provider") == "Devin" and row.get("display_name") == SKILL,
                                     "Global native Skill identity")
                    foreign = [row for row in rows if row not in selected]
                    base.verify_skill_metadata_sources(json.dumps(foreign).encode(), self.ctx, self.expected, scope, self.nonce)
                self.native_labels.append(label)
                base.zero_effects(self.probes, self.ctx.root / "stdio-started")

    def verify_native_metadata(self, before, after):
        # Refuse nonauxiliary identity changes before hashing any post-native body.
        auxiliary = lambda name: (name in base.BOOTSTRAP_DIRS or name in base.BOOTSTRAP_FIXED
                                  or name in (base.INSTALLATION_ID, base.MAINTENANCE)
                                  or base.BOOTSTRAP_LOG.fullmatch(name))
        for name in before.keys() | after.keys():
            if before.get(name) != after.get(name):
                base.require(auxiliary(name), "native Global changed nonauxiliary authority")
                if name in before and base.BOOTSTRAP_LOG.fullmatch(name):
                    base.require(False, "existing task-born log changed")
        projected = dict(self.effects.baseline)
        projected.update({name: value for name, value in after.items()
                          if auxiliary(name) or name == self.effects.config_path})
        self.effects.verify(projected)

    def verify_native_delta(self, before, after, old, now, label):
        # Only already admitted startup roles may change during native listings.
        auxiliary = lambda name: (name in base.BOOTSTRAP_DIRS or name in base.BOOTSTRAP_FIXED
                                  or name in (base.INSTALLATION_ID, base.MAINTENANCE)
                                  or base.BOOTSTRAP_LOG.fullmatch(name))
        for name in old.keys() | now.keys():
            if old.get(name) == now.get(name): continue
            base.require(auxiliary(name), "native Global changed nonauxiliary authority")
            if base.BOOTSTRAP_LOG.fullmatch(name) and name in before:
                base.require(old.get(name) == now.get(name), "existing task-born log changed")
        projected = dict(self.effects.baseline)
        projected.update({name: value for name, value in after.items()
                          if auxiliary(name) or name == self.effects.config_path})
        self.effects.verify(projected)
        born = {name for name in after.keys() - before.keys() if base.BOOTSTRAP_LOG.fullmatch(name)}
        base.require(len(born) <= 1, "Global native multiple new logs")
        for name in born:
            base.require(name not in self.log_births, "task-born log name reused")
            self.log_births[name] = {"command": label, "sha256": now[name]["sha256"]}

    def execute(self):
        self.public("uap-add", ["add", str(self.source), "--no-save"])
        self.native("installed")
        self.public("uap-doctor-installed", ["doctor", self.identifier])
        quarantine_skill(self.ctx, self.profile, self.source_bodies["skills/" + SKILL + "/SKILL.md"])
        self.public("uap-repair", ["repair", self.identifier])
        self.native("repaired")
        self.public("uap-doctor-repaired", ["doctor", self.identifier])
        self.public("uap-remove", ["remove", self.identifier], removed=True)
        self.native("removed")
        base.require(self.public_labels == list(PUBLIC) and self.native_labels == list(NATIVE), "Global lifecycle sequence")
        self.ctx.summary.update(global_lifecycle_capture_complete=True, public_global_steps=self.public_labels,
                                native_global_steps=self.native_labels, task_born_logs=self.log_births,
                                global_state_history=self.state_history)


def quarantine_skill(ctx, profile, expected):
    parent, target = profile / "skills", ctx.private / "deliberately-quarantined-skill"
    base.require(not target.exists() and not target.is_symlink(), "Skill quarantine preexists")
    original = base.root_identity(parent / SKILL)
    held_parents = {path: base.root_identity(path) for path in (parent, ctx.private)}
    descriptors = []
    try:
        for path in (parent, ctx.private):
            fd = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_NONBLOCK)
            descriptors.append(fd)
            base.require(base.KnownFileDiagnostics.identity(os.fstat(fd)) == held_parents[path], "quarantine parent changed")
        inventory = base.metadata_inventory(ctx.root, ctx.budget)
        member = (parent / SKILL / "SKILL.md").relative_to(ctx.root).as_posix()
        base.require(regular(ctx, member, inventory) == expected, "quarantined Skill source differs")
        ctx.budget.check()
        base.require(base.root_identity(parent / SKILL) == original, "quarantine Skill changed")
        os.rename(SKILL, target.name, src_dir_fd=descriptors[0], dst_dir_fd=descriptors[1])
        base.require(base.root_identity(target) == original and not (parent / SKILL).exists(), "Skill quarantine inode differs")
        base.require(all(base.root_identity(path) == identity for path, identity in held_parents.items()), "quarantine parent replaced")
        inventory = base.metadata_inventory(ctx.root, ctx.budget)
        base.require(regular(ctx, (target / "SKILL.md").relative_to(ctx.root).as_posix(), inventory) == expected,
                     "quarantine source bytes changed")
        ctx.summary["deliberate_skill_quarantine"] = {"identity": original, "sha256": sha(expected)}
    finally:
        for fd in reversed(descriptors): os.close(fd)


def author_source(ctx, probes):
    source = ctx.root / "component-source"
    skill = source / "skills" / SKILL / "SKILL.md"
    skill.parent.mkdir(parents=True)
    (source / "plugin.json").write_text(json.dumps({"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
                                                  "name": NAME, "version": "1.0.0"}))
    skill.write_text("---\nname: " + SKILL + "\ndescription: Owned Global source.\n---\nInert marker.\n")
    (source / "inert.py").write_text("from pathlib import Path\nPath(" + repr(str(ctx.root / "stdio-started")) + ").write_text('unexpected')\n")
    (source / "mcp.json").write_text(json.dumps({"mcpServers": {
        SERVERS[0]: {"type": "stdio", "command": "python3", "args": ["${PLUGIN_ROOT}/inert.py"]},
        SERVERS[1]: {"type": "streamable-http", "url": probes["user"].url}}}))
    (ctx.home / "uap-state").mkdir(mode=0o700)
    profile = Path(ctx.env["XDG_CONFIG_HOME"]) / "devin"
    profile.mkdir(mode=0o700, parents=True, exist_ok=True)
    lock = profile / "mcp_config.json.agentplugins.lock"
    with lock.open("xb"):
        pass
    lock.chmod(0o600)  # Exact UAP writer-lock leaf exists before the sole baseline.
    ctx.global_source_bodies = {path.relative_to(source).as_posix(): path.read_bytes()
                               for path in source.rglob("*") if path.is_file()}
    return source


def completion(summary):
    expected = ("source-commit", "source-tree", "source-clean", "git-init-a", "git-init-b", "git-init-c") + base.METADATA_PROBES
    tail = ("uap-add", *NATIVE[:4], "uap-doctor-installed", "uap-repair", *NATIVE[4:8],
            "uap-doctor-repaired", "uap-remove", *NATIVE[8:], "source-final-clean")
    base.require([row["probe"] for row in summary["commands"]] == list(expected + tail)
                 and all(row.get("exit") == 0 and row.get("error") is None for row in summary["commands"])
                 and summary.get("global_lifecycle_capture_complete") is True
                 and summary.get("metadata_capture_complete") is True and summary.get("admission_qualified") is False
                 and set(summary["qualification"]) == set(base.shared.QUALIFICATION_CASES)
                 and all(value is False for value in summary["qualification"].values())
                 and summary.get("native_global_steps") == list(NATIVE)
                 and summary.get("public_global_steps") == list(PUBLIC), "Global completion refused")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--installer", type=Path, required=True)
    args = parser.parse_args()
    def collect(ctx, probes, readout_state):
        source = author_source(ctx, probes)
        installer = args.installer.resolve(strict=True)
        base.require(installer.is_file() and installer == args.installer, "prebuilt installer path")
        def continue_global(ctx, probes, expected, nonce, effects):
            Journey(ctx, probes, expected, nonce, effects, installer, source).execute()
        base.collect(ctx, probes, readout_state, effects_type=ordinary.OrdinaryEffects,
                     continuation=continue_global)
    return base.run(args.output, collector=collect, recipe="ordinary-global", completion=completion, seconds=900)


if __name__ == "__main__":
    raise SystemExit(main())
