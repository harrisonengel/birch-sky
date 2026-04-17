import sys
import unittest
from pathlib import Path

_REPO = Path(__file__).resolve().parents[2]
if str(_REPO) not in sys.path:
    sys.path.insert(0, str(_REPO))

from eval.diff import _config_delta  # noqa: E402


class ConfigDeltaTests(unittest.TestCase):
    def test_no_changes_returns_empty(self):
        a = {"model": "claude-opus-4-7", "note": "baseline"}
        b = {"model": "claude-opus-4-7", "note": "baseline"}
        self.assertEqual(_config_delta(a, b), [])

    def test_changed_values_are_reported(self):
        a = {"model": "claude-opus-4-7", "note": "baseline"}
        b = {"model": "claude-sonnet-4-6", "note": "baseline"}
        out = _config_delta(a, b)
        self.assertEqual(len(out), 1)
        self.assertIn("model", out[0])
        self.assertIn("claude-opus-4-7", out[0])
        self.assertIn("claude-sonnet-4-6", out[0])
        self.assertIn("->", out[0])

    def test_added_key_reported_with_none_on_a_side(self):
        a = {"model": "m1"}
        b = {"model": "m1", "system_prompt_hash": "abc"}
        out = _config_delta(a, b)
        self.assertEqual(len(out), 1)
        self.assertIn("system_prompt_hash", out[0])
        self.assertIn("None", out[0])
        self.assertIn("'abc'", out[0])

    def test_removed_key_reported_with_none_on_b_side(self):
        a = {"model": "m1", "seed_fixture_hash": "deadbeef"}
        b = {"model": "m1"}
        out = _config_delta(a, b)
        self.assertEqual(len(out), 1)
        self.assertIn("seed_fixture_hash", out[0])
        self.assertIn("'deadbeef'", out[0])
        self.assertIn("None", out[0])

    def test_output_is_sorted_by_key(self):
        a = {"zeta": 1, "alpha": 1, "mu": 1}
        b = {"zeta": 2, "alpha": 2, "mu": 2}
        out = _config_delta(a, b)
        keys_in_order = [line.strip().split(":", 1)[0] for line in out]
        self.assertEqual(keys_in_order, sorted(keys_in_order))

    def test_multiple_changes_all_reported(self):
        a = {"model": "m1", "note": "a", "system_prompt_hash": "h1"}
        b = {"model": "m2", "note": "b", "system_prompt_hash": "h2"}
        out = _config_delta(a, b)
        self.assertEqual(len(out), 3)


if __name__ == "__main__":
    unittest.main()
