import sys
import unittest
from pathlib import Path

_REPO = Path(__file__).resolve().parents[2]
if str(_REPO) not in sys.path:
    sys.path.insert(0, str(_REPO))

from eval import paths  # noqa: E402


class FilenameSortTests(unittest.TestCase):
    def test_iso_timestamps_sort_chronologically(self):
        names = [
            "s001_alt_data_basic__2026-04-16T10-00-00Z__run_ccc.json",
            "s001_alt_data_basic__2026-04-15T09-00-00Z__run_aaa.json",
            "s001_alt_data_basic__2026-04-15T23-59-59Z__run_bbb.json",
        ]
        self.assertEqual(sorted(names), [
            "s001_alt_data_basic__2026-04-15T09-00-00Z__run_aaa.json",
            "s001_alt_data_basic__2026-04-15T23-59-59Z__run_bbb.json",
            "s001_alt_data_basic__2026-04-16T10-00-00Z__run_ccc.json",
        ])

    def test_runs_for_scenario_returns_sorted(self):
        import tempfile
        with tempfile.TemporaryDirectory() as tmp:
            runs = Path(tmp) / "runs"
            runs.mkdir()
            (runs / "s001__2026-04-16T10-00-00Z__run_c.json").write_text("{}")
            (runs / "s001__2026-04-15T09-00-00Z__run_a.json").write_text("{}")
            (runs / "s001__2026-04-15T23-59-59Z__run_b.json").write_text("{}")
            (runs / "s002__2026-04-15T12-00-00Z__run_x.json").write_text("{}")

            orig = paths.RUNS_DIR
            paths.RUNS_DIR = runs
            try:
                found = paths.runs_for_scenario("s001")
                self.assertEqual([p.name for p in found], [
                    "s001__2026-04-15T09-00-00Z__run_a.json",
                    "s001__2026-04-15T23-59-59Z__run_b.json",
                    "s001__2026-04-16T10-00-00Z__run_c.json",
                ])
            finally:
                paths.RUNS_DIR = orig

    def test_latest_run_returns_newest_timestamp_within_scenario(self):
        import tempfile
        with tempfile.TemporaryDirectory() as tmp:
            runs = Path(tmp) / "runs"
            runs.mkdir()
            (runs / "s001__2026-04-15T09-00-00Z__run_a.json").write_text("{}")
            (runs / "s001__2026-04-16T10-00-00Z__run_c.json").write_text("{}")
            (runs / "s001__2026-04-15T23-59-59Z__run_b.json").write_text("{}")

            orig = paths.RUNS_DIR
            paths.RUNS_DIR = runs
            try:
                self.assertEqual(
                    paths.latest_run().name,
                    "s001__2026-04-16T10-00-00Z__run_c.json",
                )
            finally:
                paths.RUNS_DIR = orig

    def test_parse_run_filename_splits_cleanly(self):
        name = "s001_alt_data_basic__2026-04-16T10-00-00Z__run_xyz.json"
        self.assertEqual(
            paths.parse_run_filename(name),
            ("s001_alt_data_basic", "2026-04-16T10-00-00Z", "run_xyz"),
        )

    def test_parse_run_filename_rejects_malformed(self):
        self.assertIsNone(paths.parse_run_filename("not_a_run.json"))
        self.assertIsNone(paths.parse_run_filename("a__b.json"))

    def test_run_filename_roundtrips_through_parse(self):
        built = paths.run_filename("s003_ambig", "2026-04-16T12-00-00Z", "abcd1234")
        parsed = paths.parse_run_filename(built)
        self.assertEqual(parsed, ("s003_ambig", "2026-04-16T12-00-00Z", "abcd1234"))


if __name__ == "__main__":
    unittest.main()
