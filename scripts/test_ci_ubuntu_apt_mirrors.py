import re
import unittest

from ci_ubuntu_apt_mirrors import MirrorError, prioritize

ARCHIVE = "https://archive.ubuntu.com/ubuntu"
SECURITY = "https://security.ubuntu.com/ubuntu"
AZURE = "http://azure.archive.ubuntu.com/ubuntu/"


class UbuntuMirrorTests(unittest.TestCase):
    def test_general_and_security_lists_prefer_their_existing_https_endpoint(self):
        for preferred, plain in ((ARCHIVE, "http://archive.ubuntu.com/ubuntu/"),
                                 (SECURITY, "http://security.ubuntu.com/ubuntu/")):
            source = AZURE + "\tpriority:1\n" + plain + "\tpriority:2\n" + preferred + "/\tpriority:3\n"
            expected = AZURE + "\tpriority:1\n" + plain + "\tpriority:2\n" + preferred + "/\tpriority:0\n"
            self.assertEqual(prioritize(source, preferred), expected)
            self.assertEqual(prioritize(expected, preferred), expected)

    def test_comments_whitespace_metadata_and_line_endings_are_preserved(self):
        source = "# keep this comment\r\n\r\n" + AZURE + "\tpriority:19  arch:amd64\tlang:en \r\n" + ARCHIVE + "/\tarch:amd64 priority:20 type:index\t\r\n"
        expected = "# keep this comment\r\n\r\n" + AZURE + "\tpriority:1  arch:amd64\tlang:en \r\n" + ARCHIVE + "/\tarch:amd64 priority:0 type:index\t\r\n"
        self.assertEqual(prioritize(source, ARCHIVE), expected)
        self.assertEqual(prioritize(expected, ARCHIVE), expected)

    def test_nonpreferred_priority_ties_and_order_are_preserved(self):
        source = AZURE + "\tpriority:0\n" + SECURITY + "/\tpriority:0\n" + ARCHIVE + "/\tpriority:0\n"
        result = prioritize(source, ARCHIVE)
        self.assertEqual(result, AZURE + "\tpriority:1\n" + SECURITY + "/\tpriority:1\n" + ARCHIVE + "/\tpriority:0\n")
        self.assertEqual(prioritize(result, ARCHIVE), result)

    def test_omitted_priorities_stay_omitted_except_for_the_preferred_endpoint(self):
        source = AZURE + "\n" + SECURITY + "/\tarch:amd64\n" + ARCHIVE + "/\n"
        expected = AZURE + "\n" + SECURITY + "/\tarch:amd64\n" + ARCHIVE + "/\tpriority:0\n"
        self.assertEqual(prioritize(source, ARCHIVE), expected)
        self.assertEqual(prioritize(expected, ARCHIVE), expected)

    def test_omitted_preferred_priority_preserves_its_other_metadata(self):
        source = AZURE + "\tpriority:8\n" + ARCHIVE + "/\tarch:amd64 type:index\n"
        expected = AZURE + "\tpriority:1\n" + ARCHIVE + "/\tarch:amd64 type:index priority:0\n"
        self.assertEqual(prioritize(source, ARCHIVE), expected)

    def test_explicit_group_order_remains_ahead_of_unprioritized_rows(self):
        source = SECURITY + "/\n" + AZURE + "\tpriority:7\n" + "http://archive.ubuntu.com/ubuntu/\tpriority:2\n" + ARCHIVE + "/\tpriority:9\n"
        result = prioritize(source, ARCHIVE)
        self.assertEqual(result, SECURITY + "/\n" + AZURE + "\tpriority:2\n" + "http://archive.ubuntu.com/ubuntu/\tpriority:1\n" + ARCHIVE + "/\tpriority:0\n")
        self.assertEqual(prioritize(result, ARCHIVE), result)

    def test_unknown_uri_is_refused_without_disclosing_configuration(self):
        secret = "https://private-user:TEST-secret@unexpected.example/ubuntu"
        with self.assertRaises(MirrorError) as caught:
            prioritize(secret + "\tpriority:1\n" + ARCHIVE + "/\n", ARCHIVE)
        self.assertNotIn(secret, str(caught.exception))
        self.assertNotIn("TEST-secret", str(caught.exception))

    def test_duplicate_uri_with_or_without_slash_is_refused(self):
        with self.assertRaisesRegex(MirrorError, "duplicate mirror URI"):
            prioritize(ARCHIVE + "/\tpriority:1\n" + ARCHIVE + "\tpriority:2\n", ARCHIVE)

    def test_repeated_or_malformed_priority_keys_are_refused(self):
        for metadata in ("priority:1 priority:2", "priority:-1", "priority:no", "priority:"):
            with self.subTest(metadata=metadata):
                with self.assertRaises(MirrorError):
                    prioritize(ARCHIVE + "/\t" + metadata + "\n", ARCHIVE)

    def test_preferred_https_must_already_exist(self):
        for source in ("", "# comment only\n", AZURE + "\tpriority:1\n",
                       "http://archive.ubuntu.com/ubuntu/\tpriority:1\n"):
            with self.subTest(source=source):
                with self.assertRaisesRegex(MirrorError, "canonical HTTPS mirror is missing"):
                    prioritize(source, ARCHIVE)

    def test_only_priority_metadata_changes_and_no_endpoint_is_added(self):
        source = AZURE + "\tpriority:5 arch:amd64\n" + ARCHIVE + "/\tpriority:8 lang:en\n"
        result = prioritize(source, ARCHIVE)
        self.assertEqual(re.sub(r"priority:[0-9]+", "priority:N", source),
                         re.sub(r"priority:[0-9]+", "priority:N", result))


if __name__ == "__main__":
    unittest.main()
