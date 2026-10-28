"""Pure failed-capture diagnostics; no client or qualification is synthesized."""
import copy
import unittest

import project_native_runtime as runtime
import project_native_windsurf_components as component
from test_project_native_windsurf_components import fixture, raw, sha


def delta_pages(count=15):
    roots = {role: dict(added=0, removed=0, changed=0) for role in component.DELTA_ROLES}
    details = []
    for index in range(count):
        role = component.DELTA_RANK[index % 6]
        operation = ("changed", "added", "removed")[index % 3]
        key = role + "\0PRIVATE_PATH_" + str(index)
        if index == 0:
            key = "xdg-config\0devin/config.json"
        value = {"kind": "file", "mode": 0o100600, "bytes": 1,
                 "content_sha256": sha(b"a"), "record_sha256": sha(key.encode())}
        if operation == "changed":
            value = dict(kind="directory", mode=0o040700, record_sha256=sha(key.encode()))
        before = {"kind": "absent"} if operation == "added" else copy.deepcopy(value)
        after = {"kind": "absent"} if operation == "removed" else copy.deepcopy(value)
        if operation == "changed":
            after["record_sha256"] = sha((key + "after").encode())
        details.append(dict(root=role, key_sha256=sha(key.encode()), operation=operation,
                            before=before, after=after))
        roots[role][operation] += 1
    details.sort(key=component._delta_detail)
    total = {field: sum(row[field] for row in roots.values()) for field in ("added", "removed", "changed")}
    common = dict(status="captured", before=6 + total["changed"] + total["removed"],
                  after=6 + total["changed"] + total["added"], total=total, roots=roots)
    return [dict(copy.deepcopy(common), omitted_count=count - len(details[offset:offset + 3]),
                 details=copy.deepcopy(details[offset:offset + 3])) for offset in (0, 3, 6, 9, 12)]


def envelope(pages=None, count=4, stage="empty-user-skills"):
    bodies, source = fixture(count, stage)
    summary = runtime.strict_json(bodies["runtime-attempt.json"])
    summary["profile_delta_pages"] = delta_pages() if pages is None else pages
    summary["profile_delta"] = copy.deepcopy(summary["profile_delta_pages"][0])
    return bodies, source, summary


def admit(bodies, source, summary):
    bodies["runtime-attempt.json"] = raw(summary)
    return runtime.cli_attempt_members(bodies, source, "windsurf")


class WindsurfDeltaPagesTests(unittest.TestCase):
    def test_fixed_windows_preserve_page_zero_counts_overflow_and_failure(self):
        for count in (1, 3, 4, 14, 15, 16):
            with self.subTest(count=count):
                bodies, source, summary = envelope(delta_pages(count))
                result = admit(bodies, source, summary)
                pages = result["profile_delta_pages"]
                self.assertEqual(pages[0], result["profile_delta"])
                selected = [row for page in pages for row in page["details"]]
                self.assertEqual(len(selected), min(count, 15))
                self.assertEqual(count - len(selected), max(0, count - 15))
                for index, page in enumerate(pages):
                    self.assertEqual(len(page["details"]), min(3, max(0, count - 3 * index)))
                    self.assertEqual(page["omitted_count"], count - len(page["details"]))
                    self.assertLessEqual(len(raw(page)), 2048)
                    self.assertNotIn(b"PRIVATE", raw(page))
                self.assertTrue(result["test_failed"])
                self.assertFalse(result["runtime_attempt_complete"])
                self.assertFalse(any(result["qualification"].values()))

    def test_details_bound_retains_global_counts_and_all_omissions(self):
        pages = delta_pages(15)
        for page in (pages[0], pages[2]):
            page.update(status="details-bound", details=[], omitted_count=15)
        bodies, source, summary = envelope(pages)
        result = admit(bodies, source, summary)
        self.assertEqual(sum(len(page["details"]) for page in result["profile_delta_pages"]), 9)
        summary["profile_delta_pages"][2]["details"] = delta_pages(15)[2]["details"]
        with self.assertRaises(ValueError):
            admit(bodies, source, summary)

    def test_absent_field_keeps_legacy_result_and_equal_or_refused_frames_refuse(self):
        bodies, source = fixture(4, "empty-user-skills")
        original = bodies["runtime-attempt.json"]
        result = runtime.cli_attempt_members(bodies, source, "windsurf")
        self.assertNotIn("profile_delta_pages", result)
        self.assertEqual(bodies["runtime-attempt.json"], original)
        for status in ("equal", "record-refused", "inventory-bound"):
            bodies, source, summary = envelope(delta_pages(0))
            for page in summary["profile_delta_pages"]:
                page["status"] = status
            summary["profile_delta"]["status"] = status
            with self.subTest(status=status), self.assertRaises(ValueError):
                admit(bodies, source, summary)

    def test_unknown_fields_types_counts_omissions_and_fixed_pages_refuse(self):
        changes = {
            "four-pages": lambda s: s["profile_delta_pages"].pop(),
            "six-pages": lambda s: s["profile_delta_pages"].append(copy.deepcopy(s["profile_delta_pages"][-1])),
            "unknown-page": lambda s: s["profile_delta_pages"][1].update(path="PRIVATE_PATH"),
            "boolean-count": lambda s: s["profile_delta_pages"][1].update(before=True),
            "negative-count": lambda s: s["profile_delta_pages"][1].update(after=-1),
            "inventory-bound": lambda s: s["profile_delta_pages"][1].update(before=4097),
            "omission": lambda s: s["profile_delta_pages"][1].update(omitted_count=0),
            "root-count": lambda s: s["profile_delta_pages"][1]["roots"]["home"].update(added=8),
            "root-role": lambda s: s["profile_delta_pages"][1]["roots"].update(private=dict(added=0, removed=0, changed=0)),
            "page-zero": lambda s: s["profile_delta_pages"][0]["details"][0].update(key_sha256="0" * 64),
            "detail-missing": lambda s: s["profile_delta_pages"][1]["details"].pop(),
            "duplicate": lambda s: s["profile_delta_pages"].__setitem__(1, copy.deepcopy(s["profile_delta_pages"][0])),
            "page-order": lambda s: s["profile_delta_pages"].__setitem__(slice(1, 3), s["profile_delta_pages"][1:3][::-1]),
            "config-priority": lambda s: s["profile_delta_pages"][0]["details"].reverse(),
            "no-native-stage": lambda s: s.update(attempt_stage="capture-review"),
            "no-original": lambda s: s.pop("profile_delta"),
        }
        for kind, change in changes.items():
            bodies, source, summary = envelope()
            change(summary)
            with self.subTest(kind=kind), self.assertRaises(ValueError):
                admit(bodies, source, summary)
        bodies, source, summary = envelope(count=2, stage="git-b")
        with self.assertRaises(ValueError):
            admit(bodies, source, summary)

    def test_closed_detail_and_ordinary_value_shapes_refuse(self):
        for kind in ("raw-path", "root", "hash", "operation", "present-added", "absent-changed",
                     "same-record", "unknown-value", "mode-kind", "boolean-mode", "bytes", "content-hash"):
            bodies, source, summary = envelope()
            # A nonzero page avoids a page-zero mismatch masking the actual malformed field.
            detail = summary["profile_delta_pages"][1]["details"][0]
            self.assertEqual(detail["operation"], "added")
            value = detail["after"]
            if kind == "raw-path": detail["path"] = "PRIVATE_PATH"
            elif kind == "root": detail["root"] = "private"
            elif kind == "hash": detail["key_sha256"] = "A" * 64
            elif kind == "operation": detail["operation"] = "renamed"
            elif kind == "present-added": detail["before"] = copy.deepcopy(value)
            elif kind == "absent-changed": detail["operation"] = "changed"
            elif kind == "same-record": detail.update(operation="changed", before=copy.deepcopy(value))
            elif kind == "unknown-value": value["uid"] = 1
            elif kind == "mode-kind": value["mode"] = 0o040700
            elif kind == "boolean-mode": value["mode"] = True
            elif kind == "bytes": value["bytes"] = (64 << 20) + 1
            elif kind == "content-hash": value["content_sha256"] = "not-sha256"
            with self.subTest(kind=kind), self.assertRaises(ValueError):
                admit(bodies, source, summary)

    def test_serialized_page_bound_and_duplicate_json_keys_refuse(self):
        pages = delta_pages(3)
        for detail in pages[0]["details"]:
            detail.update(operation="changed", before=dict(kind="file", mode=0o100600, bytes=64 << 20,
                          content_sha256=sha(b"a"), record_sha256=sha(b"before")),
                          after=dict(kind="file", mode=0o100600, bytes=64 << 20,
                          content_sha256=sha(b"b"), record_sha256=sha(b"after")))
        self.assertGreater(len(raw(pages[0])), 2048)
        bodies, source, summary = envelope(pages)
        with self.assertRaises(ValueError):
            admit(bodies, source, summary)
        bodies, source, summary = envelope()
        encoded = raw(summary).replace(b'"profile_delta_pages":', b'"profile_delta_pages":[],"profile_delta_pages":', 1)
        bodies["runtime-attempt.json"] = encoded
        with self.assertRaises(ValueError):
            runtime.cli_attempt_members(bodies, source, "windsurf")


if __name__ == "__main__":
    unittest.main()
