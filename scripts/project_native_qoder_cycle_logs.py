"""Independent closed Qoder original-profile and one-run log overlay joins."""
import re

from project_native_qoder_cycle import compact, digest, instant, natural, obj, require, same, sha

RUN = re.compile(r"(\d{4}-\d\d-\d\dT)(\d\d)-(\d\d)-(\d\d)-(\d{3})(Z|[+-]\d\d-\d\d)-([0-9a-z]{0,6})-p([1-9][0-9]{0,9})\Z")
RUN_FIELDS = "run_id latest_target manifest_bytes manifest_sha256 pid ppid started_at cli_version platform arch process_role pid_provenance run_id_matches_manifest cwd_matches argv_matches exact_added_run_closure preexisting_records_stable"


def integer(text, base=10, signed=False):
    require(type(text) is str and re.fullmatch(r"-?(?:0|[1-9][0-9]*)" if base == 10 else r"[1-7][0-7]*", text))
    value = int(text, base)
    require((str(value) if base == 10 else format(value, "o")) == text)
    require(-(1 << 63) <= value < 1 << 63 if signed else 0 <= value < 1 << 64)
    return value


def run(value, form, start, end):
    obj(value, RUN_FIELDS)
    text = value["run_id"]
    require(type(text) is str and len(text) <= 64)
    match = RUN.fullmatch(text)
    require(match is not None)
    day, hour, minute, second, millis, zone, _, process = match.groups()
    if zone != "Z":
        require(int(zone[1:3]) < 24 and int(zone[4:]) < 60 and (int(zone[1:3]) != 0 or int(zone[4:]) != 0))
    timestamp = day + hour + ":" + minute + ":" + second + "." + millis + (zone if zone == "Z" else zone[:3] + ":" + zone[4:])
    require(value["started_at"] == timestamp and start // 10**6 * 10**6 <= instant(timestamp) <= end)
    require(type(value["pid"]) is int and value["pid"] == int(process) and 0 < value["pid"] < 1 << 31)
    natural(value["ppid"], (1 << 31) - 1)
    require(value["ppid"] > 0)
    natural(value["manifest_bytes"], 4096)
    require(value["manifest_bytes"] > 0)
    digest(value["manifest_sha256"])
    expected = {"latest_target": "runs/" + text, "cli_version": "1.1.78", "platform": "linux", "arch": "x64", "process_role": "main", "pid_provenance": "manifest-only",
                "run_id_matches_manifest": True, "cwd_matches": True, "argv_matches": form == "direct", "exact_added_run_closure": True, "preexisting_records_stable": True}
    require(all(same(value[key], wanted) for key, wanted in expected.items()))
    return text


def latest(record, identity, uid, gid):
    require(type(record) is str and len(record) <= 1024)
    parts = record.split(":")
    require(len(parts) == 26 and parts[0] == "symlink" and parts[12] == "target")
    for first, mode in ((1, 0o120777), (13, 0o40755)):
        fields = [integer(parts[first + i], 8 if i == 2 else 10, i in (7, 9)) for i in range(11)]
        require(fields[1] > 0 and fields[2] == mode and fields[3] > 0 and fields[4] == uid and fields[5] == gid)
        require(fields[6] < 1 << 63 and fields[8] < 10**9 and fields[10] < 10**9)
        if first == 1:
            require(fields[3] == 1)
    target = ("runs/" + identity).encode()
    require(integer(parts[7]) == integer(parts[24]) == len(target) and digest(parts[25]) == sha(target))
    return record


def projected_value(record):
    if record is None:
        return {"kind": "absent"}
    if record.startswith("symlink:"):
        return {"kind": "symlink", "record_sha256": sha(record.encode())}
    parts = record.split(":")
    if len(parts) == 2:
        return {"kind": "directory", "mode": parts[1]}
    require(len(parts) == 4 and parts[0] == "file")
    return {"kind": "file", "mode": parts[1], "bytes": integer(parts[2]), "content_sha256": digest(parts[3])}


def delta(before, after):
    result = dict(status="captured", before_entries=len(before), after_entries=len(after), added=0, removed=0, changed=0,
                  areas={key: 0 for key in ("log-latest", "logs", "settings", "plugin-cache", "plugins", "cache", "other")}, omitted_count=0, entries=[])
    for key in set(before) | set(after):
        if before.get(key) == after.get(key):
            continue
        require(key == "logs/latest" or key == "logs" or key.startswith("logs/"))
        operation = "added" if key not in before else "removed" if key not in after else "changed"
        area = "log-latest" if key == "logs/latest" else "logs"
        result[operation] += 1
        result["areas"][area] += 1
        result["entries"].append(dict(key_sha256=sha(key.encode()), area=area, operation=operation,
                                      before=projected_value(before.get(key)), after=projected_value(after.get(key))))
    result["entries"].sort(key=lambda entry: (entry["area"] != "log-latest", entry["key_sha256"]))
    if len(result["entries"]) > 4:
        result["status"], result["omitted_count"] = "entry-limit", len(result["entries"]) - 4
        result["entries"] = result["entries"][:4]
    require(len(compact(result)) <= 2048)
    return result


def framing(value):
    obj(value, "kind before_sha256 after_sha256", "argv_form latest_record run delta log_bytes log_sha256")
    digest(value["before_sha256"])
    require(value["kind"] in ("unchanged", "one-run", "refused"))
    if "run" in value:
        packet = obj(value["run"], RUN_FIELDS)
        require(type(packet["run_id"]) is str and len(packet["run_id"]) <= 64 and RUN.fullmatch(packet["run_id"]))
        require(packet["latest_target"] == "runs/" + packet["run_id"])
        natural(packet["manifest_bytes"], 4096)
        require(packet["manifest_bytes"] > 0)
        digest(packet["manifest_sha256"])
        for field in ("pid", "ppid"):
            natural(packet[field], (1 << 31) - 1)
            require(packet[field] > 0)
        instant(packet["started_at"])
        require(all(type(packet[field]) is bool for field in ("run_id_matches_manifest", "cwd_matches", "argv_matches", "exact_added_run_closure", "preexisting_records_stable")))
        require(all(packet[field] == wanted for field, wanted in dict(cli_version="1.1.78", platform="linux", arch="x64", process_role="main", pid_provenance="manifest-only").items()))
    if value["kind"] == "refused":
        require(value["after_sha256"] == "" and set(value) <= set("kind before_sha256 after_sha256 argv_form run".split()))
        if "argv_form" in value:
            require(value["argv_form"] in ("direct", "compiled-embedded", "refused"))
        return
    digest(value["after_sha256"])
    if value["kind"] == "unchanged":
        require(set(value) == {"kind", "before_sha256", "after_sha256"} and value["before_sha256"] == value["after_sha256"])
    else:
        require(set(value) == set("kind before_sha256 after_sha256 argv_form latest_record run delta log_bytes log_sha256".split()))
        require(value["argv_form"] in ("direct", "compiled-embedded"))
        natural(value["log_bytes"], 64 << 20)
        require(value["log_bytes"] > 0)
        digest(value["log_sha256"])
        require(type(value["latest_record"]) is str and len(value["latest_record"]) <= 1024 and type(value["delta"]) is dict and len(compact(value["delta"])) <= 2048)
        parts = value["latest_record"].split(":")
        require(len(parts) == 26)
        uid, gid = integer(parts[5]), integer(parts[6])
        natural(uid, (1 << 32) - 1)
        natural(gid, (1 << 32) - 1)
        latest(value["latest_record"], value["run"]["run_id"], uid, gid)
        packet = obj(value["delta"], "status before_entries after_entries added removed changed areas omitted_count entries")
        require(packet["status"] in ("captured", "entry-limit"))
        for field in ("before_entries", "after_entries", "added", "removed", "changed", "omitted_count"):
            natural(packet[field], 4096)
        obj(packet["areas"], "log-latest logs settings plugin-cache plugins cache other")
        for count in packet["areas"].values():
            natural(count, 4096)
        require(type(packet["entries"]) is list and len(packet["entries"]) <= 4)
        for entry in packet["entries"]:
            obj(entry, "key_sha256 area operation before after")
            digest(entry["key_sha256"])
            require(entry["area"] in packet["areas"] and entry["operation"] in ("added", "removed", "changed"))
            for record in (entry["before"], entry["after"]):
                require(type(record) is dict and record.get("kind") in ("absent", "symlink", "directory", "file"))
                fields = {"absent": "kind", "symlink": "kind record_sha256", "directory": "kind mode", "file": "kind mode bytes content_sha256"}
                obj(record, fields[record["kind"]])
                if record["kind"] == "symlink": digest(record["record_sha256"])
                if record["kind"] in ("directory", "file"): integer(record["mode"], 8)
                if record["kind"] == "file":
                    natural(record["bytes"], 64 << 20)
                    digest(record["content_sha256"])


def transition(value, expected, runs, uid, gid, start, end, native):
    framing(value)
    before = sha(compact(expected))
    require(value["before_sha256"] == before)
    if not native:
        require(value["kind"] == "unchanged" and value["after_sha256"] == before)
        return expected
    require(value["kind"] == "one-run")
    identity = run(value["run"], value["argv_form"], start, end)
    require(identity not in runs)
    path = "logs/runs/" + identity
    require(not any(key == path or key.startswith(path + "/") for key in expected))
    after = dict(expected)
    if not runs:
        require(not any(key == "logs" or key.startswith("logs/") for key in expected))
        after.update({"logs": "directory:20000000755", "logs/runs": "directory:20000000755"})
    after[path] = "directory:20000000755"
    after[path + "/manifest.json"] = "file:644:" + str(value["run"]["manifest_bytes"]) + ":" + value["run"]["manifest_sha256"]
    after[path + "/qodercli.log"] = "file:644:" + str(value["log_bytes"]) + ":" + value["log_sha256"]
    after["logs/latest"] = latest(value["latest_record"], identity, uid, gid)
    require(value["after_sha256"] == sha(compact(after)) and same(value["delta"], delta(expected, after)))
    runs.append(identity)
    return after
