"""One bundled inspect experiment; no installer, process authority or native fallback."""
from contextlib import ExitStack
import hashlib
import json
import os
import re
from pathlib import Path
import shutil
import sys
import time
from types import SimpleNamespace

from hermes_passive_history_processes import (
    HeldChild, LedgerHandle, REGISTER, SELECT, SENTINEL, capture_command, decoded,
    digest, document, encoded, remaining, require, validate_command, validate_forged, validate_closed_child, rejoin_commands,
)


HELPERS = ("bounded directory_identity ordinary_bytes stable_file same_opened_capture "
           "open_inventory_directory check_inventory_directory verify_inventory_directory held_census_evidence "
           "public_runtime_pins public_snapshot public_continuity public_stdio_recipe public_stdio_events "
           "public_spawn_read public_spawn_status public_spawn_status_pid public_spawn_launcher public_spawn_entry "
           "public_spawn_install public_spawn_continuity public_windows_process_identity public_windows_log_read "
           "public_windows_log_row public_windows_log_role public_windows_log_continuity public_inspect_lock "
           "public_profile observer_runtime_command write_json history_control_report").split()
COMMANDS = ("git", "add", "bootstrap", "public-stdio-a", "selection", "inspect", "noninspect", "active")
CHALLENGES = ("ready", "after-register", "before-select", "after-select", "before-inspect",
              "after-inspect", "after-noninspect", "after-active")
REFUSALS = {"noninspect": "hermes runtime history is restricted to passive inspection",
            "active": "hermes passive registry purpose or selected install differs"}
LEDGER = "spawn-ledger.json"


def projected(image):
    prefix = "public-history/"
    require("public-history" in image, "private original root missing")
    return {".": image["public-history"], **{key[len(prefix):]: value for key, value in image.items()
                                             if key.startswith(prefix)}}


def original_image(image):
    return {key: value for key, value in image.items()
            if key != "public-history" and not key.startswith("public-history/")}


def directory_facts(info):
    names = ("st_dev", "st_ino", "st_mode", "st_uid", "st_gid")
    if os.name == "nt":
        names += ("st_birthtime_ns", "st_file_attributes", "st_reparse_tag")
    return tuple(getattr(info, name) for name in names)


class HistoryControl:
    def __init__(self, helpers, outer, baseline, probes, env, pins, deadline):
        self.api = SimpleNamespace(**{name: helpers[name] for name in HELPERS}, pins=pins)
        self.outer, self.baseline, self.probes, self.pins = outer, baseline, probes, pins
        self.private, self.project = outer / "public-history", outer / "public-history/public-a"
        self.source, self.evidence = self.private / "public-source-a", self.private / "evidence"
        self.selected = self.project / ".hermes/uap-session"
        require(all(name not in env or type(env[name]) is str and env[name] == ""
                    for name in ("HERMES_HOME_MODE", "HERMES_UID", "HERMES_GID", "HERMES_SKIP_CHMOD")),
                "private inspection permission policy conflicts with inherited selectors")
        self.env = dict(env, AGENTPLUGINS_HOME=str(self.private / "public-uap-state"))
        self.nonce = env["HERMES_CALL_NONCE"]
        self.deadline = min(deadline - 50, time.monotonic() + 120)
        require(remaining(self.deadline) > 40, "insufficient original control/cold reserve")
        self.native, self.python = env["HERMES_BIN"], env["HERMES_PROFILE_PYTHON"]
        self.code = Path(env["HERMES_PROFILE_RUNTIME_ROOT"])
        self.code_identity, self.install = self.api.public_spawn_install(self.code, Path(self.native))
        self.current, self.accepted, self.held = baseline, None, []
        self.records, self.captures, self.phases = [], {}, []
        self.sentinel, self.registrar, self.ledger = None, None, None
        self.http = {key: list(probe.methods()) for key, probe in probes.items()}
        self.spawn, self.log_lock, self.config, self.events = None, None, None, []
        self.hold(outer)

    def hold(self, path):
        named = path.lstat()
        fd = self.api.open_inventory_directory(path)
        try:
            opened = os.fstat(fd)
            self.api.check_inventory_directory(named, opened, "passive original directory changed at open")
            require(directory_facts(path.lstat()) == directory_facts(named), "directory acquisition changed")
        except BaseException:
            os.close(fd)
            raise
        self.held.append((path, fd, directory_facts(named), directory_facts(opened)))

    def join(self):
        remaining(self.deadline)
        self.api.public_runtime_pins(self.pins, rehash=False)
        require(self.api.directory_identity(self.code) == self.code_identity, "original code root differs")
        for path, fd, named, opened in self.held:
            require(path.resolve() == path and directory_facts(path.lstat()) == named
                    and directory_facts(os.fstat(fd)) == opened, "original held directory differs")
        require(self.http == {key: list(probe.methods()) for key, probe in self.probes.items()}, "unexpected HTTP dispatch")
        rejoin_commands(self.api, self.captures, self.evidence, self.deadline)

    def snapshot(self):
        self.join()
        image, proofs = self.api.public_snapshot(self.outer, {"a": self.project}, self.pins)
        require(original_image(image) == self.baseline, "original User/A/B/outer changed")
        if self.accepted is not None:
            require(proofs["a"] is not None, "accepted installation disappeared")
            self.api.public_continuity(self.accepted, proofs["a"])
            require(proofs["a"]["root_receipt"] == self.accepted["root_receipt"], "original root receipt changed")
        self.join()
        return image, proofs

    def private_snapshot(self):
        self.last_full, proofs = self.snapshot()
        return projected(self.last_full), proofs

    def phase(self, label, image):
        require(original_image(image) == self.baseline, "protected original changed")
        self.phases.append(dict(label=label, original_sha256=digest(original_image(image))))
        self.current = image

    def prepare(self):
        require(not self.private.exists() and not self.private.is_symlink(), "fresh private fixture required")
        self.private.mkdir(mode=0o700)
        self.hold(self.private)
        for directory in (self.project, self.selected.parent, self.source, self.evidence, self.private / "public-uap-state"):
            directory.mkdir(mode=0o700)
            self.hold(directory)
        self.api.evidence_identity = self.api.directory_identity(self.evidence)
        self.run_command("git", ["git", "init", "-q", str(self.project)], authored=True)
        name = "hermes-public-a"
        skill = self.source / "skills" / name / "SKILL.md"
        skill.parent.mkdir(parents=True)
        skill.write_bytes(("---\nname: " + name + "\ndescription: Inert passive-history control.\n---\nFIXED\n").encode())
        (self.source / "plugin.json").write_text(json.dumps({"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
            "name": name, "version": "1.0.0", "description": "Owned passive-history control"}))
        servers = {name + "-http": {"type": "streamable-http", "url": self.probes["a"].url},
                   name + "-stdio": self.api.public_stdio_recipe(self.project, self.pins["probe"], self.nonce)}
        (self.source / "mcp.json").write_text(json.dumps({"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json", "mcpServers": servers}))
        self.phase("authored", self.snapshot()[0])
        self.run_command("add", self.uap(["add", str(self.source), "--no-save", "--yes", "--format", "json"]))
        self.accepted = self.snapshot()[1]["a"]
        require(self.accepted is not None, "actual public add has no binding")
        for path in (self.selected, self.accepted["data"]):
            self.hold(path)
        import hermes_yaml as yaml
        self.config = yaml.safe_load(self.api.ordinary_bytes(self.selected / "config.yaml", deadline=self.deadline))["mcp_servers"]
        require(set(self.config) == {"hermes-public-a-http", "hermes-public-a-stdio"}
                and self.config["hermes-public-a-http"] == {"url": self.probes["a"].url}, "owned selected inventory differs")

    def uap(self, args):
        return [self.pins["uap"]["path"], *args, "--target", "hermes", "--scope", "project", "--project-root", str(self.project)]

    def run_command(self, label, argv, *, native=False, authored=False, inspect=False, refused=False):
        before, _ = self.snapshot()
        require((authored and label == "git" and not self.records) or before == self.current, "unmeasured inter-command effect")
        with ExitStack() as handles:
            finish = None
            if inspect:
                finish = handles.enter_context(self.api.public_inspect_lock(
                    self.private, self.project, projected(before), self.private_snapshot))
            record, capture = capture_command(self.api, argv, self.project,
                dict(self.env, HERMES_HOME=str(self.selected)) if native else self.env,
                self.evidence, label, self.deadline, refused=refused, lineage=os.name == "nt" and label == "public-stdio-a")
            if finish is not None:
                image, _ = finish()
                after = self.last_full
                require(projected(after) == image, "inspect captured image differs")
            else:
                after, _ = self.snapshot()
        allowed = ["public-history/evidence/" + label + suffix for suffix in (".stdout", ".stderr", ".status.json")]
        if label == "git": allowed += ["public-history/public-a/.git"]
        if label == "add": allowed += ["public-history/public-a/.uap", "public-history/public-a/.hermes", "public-history/public-uap-state"]
        if label in {"bootstrap", "public-stdio-a"}: allowed += [self.selected.relative_to(self.outer).as_posix()]
        if label == "public-stdio-a": allowed += [(self.accepted["data"] / "hermes-events.jsonl").relative_to(self.outer).as_posix()]
        if inspect: allowed += ["public-history/public-a/.uap/mutation.lock"]
        require(all(any(key == prefix or key.startswith(prefix + "/") for prefix in allowed)
                    for key in set(before) | set(after) if before.get(key) != after.get(key)), "unmeasured command effect")
        if label == "add": require(document(decoded(record["stdout"])).get("result") == "success", "public add failed")
        if label in REFUSALS:
            text = decoded(record["stdout"]) + decoded(record["stderr"])
            require(REFUSALS[label].encode() in text, "dry-run refused at an unrelated boundary")
        self.records.append(record)
        self.captures[label] = capture
        self.phase(label, after)
        return record

    def authorities(self):
        return (self.api.directory_identity(self.private), self.api.directory_identity(self.project),
                self.accepted["parent_identity"], self.accepted["root_identity"])

    def bootstrap(self):
        record = self.run_command("bootstrap", self.uap(["run", "--inspect", "--confirm-client-start"]), inspect=True)
        require(document(decoded(record["stdout"])) == self.config, "initial public inventory differs")
        image = projected(self.current)
        prefix = "public-a/.hermes/uap-session/"
        require(not any(prefix + name in image for name in (LEDGER, LEDGER + ".lock", "logs/.__agent.lock"))
                and not (self.accepted["data"] / "hermes-events.jsonl").exists(), "bootstrap created premature history/events")
        self.api.public_profile(self.selected, image, self.private,
            {image[prefix + "config.yaml"]["sha256"]}, stdio=None, spawn=None, log_lock=None)
        self.hold(self.selected / "logs")
        # Native cold bootstrap secures new directories before history capture.
        self.env["HERMES_SKIP_CHMOD"] = "1"

    def history(self, image):
        return self.api.public_spawn_read(self.selected, self.private, projected(image), self.authorities(), self.deadline)

    def native_producer(self):
        before = self.current
        record = self.run_command("public-stdio-a", [self.native, "mcp", "test", "hermes-public-a-stdio"], native=True)
        status = self.api.public_spawn_status(self.evidence, self.api.evidence_identity, "public-stdio-a",
                                             record["status"]["argv"], decoded(record["stdout"]), self.deadline)
        original = self.captures["public-stdio-a"][1]
        require([(p, self.api.stable_file(i), b) for p, i, b in status["files"]] ==
                [(p, self.api.stable_file(i), b) for p, i, b in original], "native original capture reselected")
        pid = self.api.public_spawn_status_pid(status, self.deadline)
        self.spawn, bodies = self.history(self.current)
        path = self.accepted["data"] / "hermes-events.jsonl"
        self.events = self.api.public_stdio_events(path, self.nonce, captured=self.current[path.relative_to(self.outer).as_posix()])
        launcher = self.api.public_spawn_launcher(self.accepted, self.config["hermes-public-a-stdio"], self.pins["probe"]["path"])
        entry = self.api.public_spawn_entry(bodies[LEDGER], [], self.events, self.nonce, pid, self.install,
            launcher, self.pins["probe"]["path"], sys.platform, **({"native_process": status["process_identity"],
            "native_python": self.python, "native_image": self.native} if os.name == "nt" else {}))
        if os.name == "nt":
            self.api.public_windows_log_role("public-stdio-a", "a", record["status"]["argv"][1:], native=True,
                                            inspect=False, expected_failure=False, test="stdio")
            require(self.api.public_windows_log_row(self.selected, self.private, projected(before)) is None,
                    "agent log lock lacks original absence")
            if self.api.public_windows_log_row(self.selected, self.private, projected(self.current)) is not None:
                logs = projected(before).get("public-a/.hermes/uap-session/logs")
                self.log_lock = self.api.public_windows_log_read(self.selected, self.private, projected(self.current),
                    self.authorities(), original_logs=logs, deadline=self.deadline)
        self.profile(self.current)
        self.producer = dict(entry=entry, argv=launcher, events=self.events, receipt=self.spawn,
                             body=encoded(bodies[LEDGER]), log_lock=self.log_lock)

    def profile(self, image):
        config_hash = image[(self.selected / "config.yaml").relative_to(self.outer).as_posix()]["sha256"]
        self.api.public_profile(self.selected, projected(image), self.private, {config_hash},
                                stdio=("hermes-public-a-stdio", 1), spawn=self.spawn, log_lock=self.log_lock)

    def observer(self, code, args):
        from hermes_cli._launchers import runtime_command
        require(Path(runtime_command.__code__.co_filename).resolve() == self.code / "hermes_cli/_launchers.py",
                "observer bootstrap source differs")
        return self.api.observer_runtime_command(runtime_command, self.code, Path(self.python), code=code, args=args)

    def forge(self):
        self.sentinel = HeldChild([self.python, "-I", "-B", "-c", SENTINEL, self.nonce], self.project, self.env, self.deadline)
        self.sentinel.challenge(self.nonce, "ready")
        require(self.snapshot()[0] == self.current, "sentinel changed original inputs")
        argv = self.observer(REGISTER, [str(self.code), str(self.sentinel.child.pid), str(os.getpid()), self.nonce])
        self.registrar = HeldChild(argv, self.project, dict(self.env, HERMES_HOME=str(self.selected)), self.deadline)
        self.registration = self.registrar.response()
        require(self.registrar.wait_dead(), "registrar did not exit")
        image, _ = self.snapshot()
        key = (self.selected / LEDGER).relative_to(self.outer).as_posix()
        require({key} == {name for name in set(image) | set(self.current) if image.get(name) != self.current.get(name)},
                "registrar changed more than its declared ledger")
        receipt, bodies = self.history(image)
        self.api.public_spawn_continuity(self.spawn, receipt, stdio=True)
        self.spawn = receipt
        self.forged = document(bodies[LEDGER])
        validate_forged(self.forged, self.registration, self.sentinel.argv, self.nonce, self.install,
                        self.sentinel.child.pid, self.registrar.child.pid, os.getpid())
        self.ledger = LedgerHandle(self.api, self.selected / LEDGER, bodies[LEDGER], self.deadline)
        self.forged_body, self.forged_digest, self.forged_receipt = bodies[LEDGER], digest(receipt), receipt
        self.profile(image)
        self.phase("forged", image)
        self.sentinel.challenge(self.nonce, "after-register")

    def preserved(self, *, processes=True):
        self.ledger.verify()
        if processes:
            require(self.registrar.dead(), "reserved registrar became live")
        image, _ = self.snapshot()
        require(image == self.current, "unmeasured passive history effect")
        receipt, bodies = self.history(image)
        self.api.public_spawn_continuity(self.spawn, receipt)
        require(bodies[LEDGER] == self.ledger.body, "held ledger body differs from capture")
        if self.log_lock is not None:
            current = self.api.public_windows_log_read(self.selected, self.private, projected(image), self.authorities(),
                original_logs=self.log_lock["logs_row"], deadline=self.deadline)
            self.api.public_windows_log_continuity(self.log_lock, current)
        self.profile(image)

    def exercise(self):
        self.sentinel.challenge(self.nonce, "before-select")
        self.preserved()
        record = self.run_command("selection", self.observer(SELECT, [str(self.code), self.nonce]), native=True)
        self.selection = document(decoded(record["stdout"]))
        require(self.selection == dict(nonce=self.nonce, pid=record["status"]["pid"],
                recorded=[self.sentinel.child.pid], selected=[self.sentinel.child.pid]), "pinned selector is not sensitive")
        self.preserved()
        self.sentinel.challenge(self.nonce, "after-select")
        self.sentinel.challenge(self.nonce, "before-inspect")
        record = self.run_command("inspect", self.uap(["run", "--inspect", "--confirm-client-start"]), inspect=True)
        require(document(decoded(record["stdout"])) == self.config, "actual public inventory differs")
        self.preserved()
        self.retained_receipt = self.history(self.current)[0]
        self.retained_digest = digest(self.retained_receipt)
        require(self.retained_digest == self.forged_digest, "actual inspect changed original forged receipt")
        self.sentinel.challenge(self.nonce, "after-inspect")
        self.run_command("noninspect", self.uap(["run", "--dry-run", "--format", "json"]), refused=True)
        self.preserved()
        self.sentinel.challenge(self.nonce, "after-noninspect")
        self.active_control()

    def active_control(self):
        before = self.current
        self.active_body = self.ledger.purpose("cli")
        after, _ = self.snapshot()
        key = (self.selected / LEDGER).relative_to(self.outer).as_posix()
        require({key} == {name for name in set(before) | set(after) if before.get(name) != after.get(name)},
                "active-purpose authoring changed another member")
        self.spawn = self.history(after)[0]
        self.phase("active-authored", after)
        self.run_command("active", self.uap(["run", "--inspect", "--dry-run", "--format", "json"]), refused=True)
        self.preserved()
        self.sentinel.challenge(self.nonce, "after-active")

    def close_processes(self):
        result, errors = {}, []
        for name in ("registrar", "sentinel"):
            child = getattr(self, name)
            if child is not None and not child.closed:
                try:
                    result[name] = child.close()
                except BaseException as error:
                    errors.append(error)
        if errors:
            raise RuntimeError("passive history original process closure failed") from errors[0]
        return result

    def close_handles(self, *, keep_outer=False):
        errors = []
        if self.ledger is not None:
            try:
                self.ledger.close()
            except BaseException as error:
                errors.append(error)
            self.ledger = None
        while len(self.held) > int(keep_outer):
            try:
                os.close(self.held.pop()[1])
            except BaseException as error:
                errors.append(error)
        require(not errors, "original directory/file handle closure incomplete")

    def complete(self, closed):
        self.preserved(processes=False)
        require(set(closed) == {"sentinel", "registrar"} and closed["registrar"]["exit"] == 0
                and closed["sentinel"]["exit"] != 0, "intentional death/registrar closure differs")
        report = self.report(closed)
        self.join()
        private_identity = next(named for path, _, named, _ in self.held if path == self.private)
        require(directory_facts(self.private.lstat()) == private_identity, "original owned cleanup root differs")
        self.close_handles(keep_outer=True)
        require(directory_facts(self.private.lstat()) == private_identity, "original owned cleanup root differs")
        path, fd, named, opened = self.held[0]
        require(directory_facts(path.lstat()) == named and directory_facts(os.fstat(fd)) == opened,
                "original outer changed before cleanup")
        remaining(self.deadline)
        shutil.rmtree(self.private)
        image, _ = self.api.public_snapshot(self.outer, {}, self.pins)
        require(image == self.baseline, "owned fixture cleanup did not restore original image")
        remaining(self.deadline)
        path, fd, named, opened = self.held[0]
        require(directory_facts(path.lstat()) == named and directory_facts(os.fstat(fd)) == opened,
                "original outer changed during cleanup")
        report["cleanup"] = dict(processes=True, pipes=True, fixture=True, original_sha256=digest(image))
        self.api.history_control_report(report, source={"commit": self.env["HERMES_PROFILE_SOURCE_HEAD"],
                                       "tree": self.env["HERMES_PROFILE_SOURCE_TREE"]}, nonce=self.nonce,
                        pins=self.pins, native=self.native, python=self.python, outer=self.outer,
                        original_sha256=digest(self.baseline),
                        authorities=(self.code_identity, self.held[0][2][:5]))
        return report

    def report(self, closed):
        return dict(schema=1, kind="hermes-held-sibling-inspect-control", source={
            "commit": self.env["HERMES_PROFILE_SOURCE_HEAD"], "tree": self.env["HERMES_PROFILE_SOURCE_TREE"]},
            nonce=self.nonce, platform=sys.platform, runtime=dict(native=self.native, python=self.python,
            code=str(self.code), install=self.install, code_identity=list(self.code_identity), pins=self.pins),
            fixture=dict(outer=str(self.outer), project=str(self.project), data=str(self.accepted["data"]),
            installation_sha256=digest(self.accepted["installed"]),
            binding_sha256=digest(self.accepted["binding"]), roots=[dict(path=str(p), identity=list(n)) for p, _, n, _ in self.held]),
            commands=self.records, servers=self.config, producer=self.producer, registration=self.registration, selection=self.selection,
            forged=dict(body=encoded(self.forged_body), active_body=encoded(self.active_body), receipt_sha256=self.forged_digest,
                        retained_sha256=self.retained_digest, receipt=self.forged_receipt, retained_receipt=self.retained_receipt),
            processes=dict(sentinel=dict(argv=self.sentinel.argv, checks=self.sentinel.checks, closed=closed["sentinel"]),
                           registrar=dict(argv=self.registrar.argv, closed=closed["registrar"])),
            protected=dict(original_sha256=digest(self.baseline), phases=self.phases), cleanup=None,
            claim="no-ledger-directed-termination-of-original-held-sentinel", universal_signal_audit=False,
            admission_qualified=False, production_project_mcp=False)


def validate_report(value, *, source, nonce, pins, native, python, outer):
    value = document(json.dumps(value, allow_nan=False).encode())
    fields = set("schema kind source nonce platform runtime fixture commands servers producer registration selection forged processes protected cleanup claim universal_signal_audit admission_qualified production_project_mcp".split())
    require(type(value) is dict and set(value) == fields and type(value["schema"]) is int and value["schema"] == 1
            and value["kind"] == "hermes-held-sibling-inspect-control" and value["source"] == source
            and type(source) is dict and set(source) == {"commit", "tree"}
            and all(type(x) is str and re.fullmatch(r"[a-f0-9]{40}", x) for x in source.values())
            and value["nonce"] == nonce and type(nonce) is str and re.fullmatch(r"[a-f0-9]{64}", nonce)
            and value["platform"] == sys.platform, "source-bound closed report differs")
    require(value["claim"] == "no-ledger-directed-termination-of-original-held-sentinel"
            and all(value[key] is False for key in ("universal_signal_audit", "admission_qualified", "production_project_mcp")),
            "report qualification differs")
    runtime = value["runtime"]
    require(type(runtime) is dict and set(runtime) == {"native", "python", "code", "install", "code_identity", "pins"}
            and runtime["native"] == native and runtime["python"] == python and runtime["pins"] == pins
            and Path(runtime["code"]) == Path(native).parent.parent / "hermes-agent"
            and type(runtime["code_identity"]) is list and len(runtime["code_identity"]) == 5
            and all(type(x) is int for x in runtime["code_identity"]), "report original runtime differs")
    fixture = value["fixture"]
    require(type(fixture) is dict and set(fixture) == {"outer", "project", "data", "installation_sha256", "binding_sha256", "roots"}
            and fixture["outer"] == str(outer) and fixture["project"] == str(outer / "public-history/public-a")
            and all(type(fixture[key]) is str and re.fullmatch(r"[a-f0-9]{64}", fixture[key])
                    for key in ("installation_sha256", "binding_sha256")), "closed fixture authority differs")
    commands = value["commands"]
    require(type(commands) is list and len(commands) == len(COMMANDS)
            and [item.get("label") for item in commands] == list(COMMANDS), "command membership/order differs")
    total = 0
    for command in commands:
        label = command["label"]
        validate_command(command, refused=label in REFUSALS, lineage=sys.platform == "win32" and label == "public-stdio-a")
        total += sum(len(decoded(command[key])) for key in ("stdout", "stderr", "status_raw"))
        if label in REFUSALS:
            require(REFUSALS[label].encode() in decoded(command["stdout"]) + decoded(command["stderr"]),
                    "negative did not reach its intended guard")
    total += validate_process_report(value)
    require(total <= 256 << 10, "aggregate retained command bound")
    validate_phase_report(value)
    return value


def validate_process_report(value):
    processes = value["processes"]
    require(type(processes) is dict and set(processes) == {"sentinel", "registrar"}, "process role membership differs")
    sentinel, registrar = processes["sentinel"], processes["registrar"]
    require(type(sentinel) is dict and set(sentinel) == {"argv", "checks", "closed"}
            and type(registrar) is dict and set(registrar) == {"argv", "closed"}
            and sentinel["argv"] == [value["runtime"]["python"], "-I", "-B", "-c", SENTINEL, value["nonce"]], "fixed original sentinel differs")
    out = validate_closed_child(sentinel["closed"], sentinel=True)
    registered = validate_closed_child(registrar["closed"], sentinel=False)
    require([document(line) for line in out.splitlines()] == sentinel["checks"]
            and len(registered.splitlines()) == 1 and document(registered.strip()) == value["registration"],
            "original child stdout join differs")
    checks = sentinel["checks"]
    require(type(checks) is list and len(checks) == len(CHALLENGES), "challenge membership differs")
    for phase, check in zip(CHALLENGES, checks):
        require(check == dict(nonce=value["nonce"], phase=phase, pid=sentinel["closed"]["pid"], parent=value["registration"]["parent"])
                and type(check["pid"]) is int and type(check["parent"]) is int, "original challenge order/identity differs")
    forged = value["forged"]
    require(type(forged) is dict and set(forged) == {"body", "active_body", "receipt_sha256", "retained_sha256", "receipt", "retained_receipt"}
            and type(forged["receipt_sha256"]) is str and re.fullmatch(r"[a-f0-9]{64}", forged["receipt_sha256"])
            and forged["retained_sha256"] == forged["receipt_sha256"] == digest(forged["receipt"])
            and forged["receipt"] == forged["retained_receipt"], "immutable history commitment differs")
    rows, active = document(decoded(forged["body"])), document(decoded(forged["active_body"]))
    require(len(decoded(forged["body"])) <= 4096 and len(decoded(forged["active_body"])) <= 4096, "ledger body bound")
    validate_forged(rows, value["registration"], sentinel["argv"], value["nonce"], value["runtime"]["install"],
                    sentinel["closed"]["pid"], registrar["closed"]["pid"], value["registration"]["parent"])
    require(active == [dict(rows[0], purpose="cli")], "active control changed another ledger field")
    selected = value["commands"][4]
    selection = value["selection"]
    require(type(selection) is dict and set(selection) == {"nonce", "pid", "recorded", "selected"}
            and type(selection["pid"]) is int and all(type(selection[key]) is list
                and all(type(pid) is int for pid in selection[key]) for key in ("recorded", "selected")),
            "selector result types differ")
    require(value["selection"] == document(decoded(selected["stdout"])) == dict(nonce=value["nonce"],
            pid=selected["status"]["pid"], recorded=[sentinel["closed"]["pid"]], selected=[sentinel["closed"]["pid"]]),
            "actual selector result differs")
    return len(out) + len(registered)


def validate_phase_report(value):
    protected, cleanup = value["protected"], value["cleanup"]
    require(type(protected) is dict and set(protected) == {"original_sha256", "phases"}
            and type(protected["original_sha256"]) is str and re.fullmatch(r"[a-f0-9]{64}", protected["original_sha256"]),
            "original protected commitment differs")
    phases = ("git", "authored", "add", "bootstrap", "public-stdio-a", "forged", "selection", "inspect", "noninspect", "active-authored", "active")
    require(protected["phases"] == [dict(label=label, original_sha256=protected["original_sha256"]) for label in phases]
            and cleanup == dict(processes=True, pipes=True, fixture=True, original_sha256=protected["original_sha256"])
            and all(cleanup[key] is True for key in ("processes", "pipes", "fixture")), "phase order or cleanup differs")
    fixture, private = value["fixture"], Path(value["fixture"]["outer"]) / "public-history"
    data = Path(fixture["data"])
    require(data.parent == private / "public-a/.uap/plugin-data"
            and re.fullmatch(r"hermes-public-a-[a-f0-9]{12}", data.name), "original data role differs")
    expected = {private.parent, private, private / "public-a", private / "public-source-a", private / "evidence",
                private / "public-uap-state", private / "public-a/.hermes", private / "public-a/.hermes/uap-session",
                private / "public-a/.hermes/uap-session/logs", data}
    roots = fixture["roots"]
    require(type(roots) is list and len(roots) == len(expected)
            and all(type(item) is dict and set(item) == {"path", "identity"} and type(item["path"]) is str
                    and type(item["identity"]) is list and len(item["identity"]) == (8 if os.name == "nt" else 5)
                    and all(type(number) is int for number in item["identity"]) for item in roots)
            and {Path(item["path"]) for item in roots} == expected, "original held root membership differs")


def run_control(helpers, *, outer, baseline, probes, env, pins, deadline):
    control = HistoryControl(helpers, outer, baseline, probes, env, pins, deadline)
    complete = False
    try:
        control.prepare()
        control.bootstrap()
        control.native_producer()
        control.forge()
        control.exercise()
        closed = control.close_processes()
        complete = True
        return control.complete(closed)
    finally:
        try:
            if not complete:
                control.close_processes()
        finally:
            control.close_handles()
