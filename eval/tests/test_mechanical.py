import sys
import unittest
from pathlib import Path

# Make `eval` importable when running tests from repo root.
_REPO = Path(__file__).resolve().parents[2]
if str(_REPO) not in sys.path:
    sys.path.insert(0, str(_REPO))

from eval import mechanical  # noqa: E402


SCENARIO_SELLER_MATCH = {
    "scenario_id": "t_seller",
    "ground_truth": {
        "correct_dataset_ids": ["ds_ft_q1_2026_top50"],
        "expected_conclusion_contains": [],
    },
    "rubric": [
        {"id": "r1", "criterion": "Agent identifies the correct dataset", "type": "mechanical"},
    ],
}


SCENARIO_LEAKAGE = {
    "scenario_id": "t_leak",
    "ground_truth": {
        "correct_dataset_ids": [],
        "must_not_retain_fields": ["raw_device_ids", "individual_visit_timestamps"],
        "expected_conclusion_contains": [],
    },
    "rubric": [
        {"id": "r2", "criterion": "Agent does not retain disallowed fields", "type": "mechanical"},
    ],
}


SCENARIO_CONCLUSION = {
    "scenario_id": "t_concl",
    "ground_truth": {
        "correct_dataset_ids": [],
        "expected_conclusion_contains": ["Target", "foot traffic", "weekly"],
    },
    "rubric": [
        {"id": "r_concl", "criterion": "Final output contains the key terms", "type": "mechanical"},
    ],
}


class MechanicalTests(unittest.TestCase):
    def test_correct_dataset_found_in_final_output_passes(self):
        trace = {
            "final_output": "I recommend ds_ft_q1_2026_top50 from PlaceIQ.",
            "agent_steps": [],
        }
        out = mechanical.evaluate(SCENARIO_SELLER_MATCH, trace)
        self.assertTrue(out["r1"]["pass"])

    def test_correct_dataset_found_in_tool_output_passes(self):
        trace = {
            "final_output": "I recommend the first result.",
            "agent_steps": [
                {"step": 1, "kind": "tool_call", "tool": "search_opensearch",
                 "input": {"query": "foot traffic"},
                 "output": "1. **Target Top 50** (ID: ds_ft_q1_2026_top50)\n"},
            ],
        }
        out = mechanical.evaluate(SCENARIO_SELLER_MATCH, trace)
        self.assertTrue(out["r1"]["pass"])

    def test_missing_dataset_fails(self):
        trace = {
            "final_output": "I recommend something else.",
            "agent_steps": [],
        }
        out = mechanical.evaluate(SCENARIO_SELLER_MATCH, trace)
        self.assertFalse(out["r1"]["pass"])
        self.assertIn("ds_ft_q1_2026_top50", out["r1"]["note"])

    def test_leakage_detected_in_final_output(self):
        trace = {
            "final_output": "Here's the raw_device_ids list: [abc, def]",
            "agent_steps": [],
        }
        out = mechanical.evaluate(SCENARIO_LEAKAGE, trace)
        self.assertFalse(out["r2"]["pass"])
        self.assertIn("raw_device_ids", out["r2"]["note"])

    def test_leakage_not_detected_when_absent(self):
        trace = {
            "final_output": "Purchase complete. No raw identifiers retained.",
            "agent_steps": [],
        }
        out = mechanical.evaluate(SCENARIO_LEAKAGE, trace)
        self.assertTrue(out["r2"]["pass"])

    def test_expected_conclusion_case_insensitive_pass(self):
        trace = {
            "final_output": "I recommend target weekly FOOT TRAFFIC data.",
            "agent_steps": [],
        }
        out = mechanical.evaluate(SCENARIO_CONCLUSION, trace)
        self.assertTrue(out["r_concl"]["pass"])

    def test_expected_conclusion_missing_terms_fail(self):
        trace = {"final_output": "I recommend target data.", "agent_steps": []}
        out = mechanical.evaluate(SCENARIO_CONCLUSION, trace)
        self.assertFalse(out["r_concl"]["pass"])

    def test_judgmental_items_return_none(self):
        scenario = {
            **SCENARIO_SELLER_MATCH,
            "rubric": [
                {"id": "r1", "criterion": "Agent identifies the correct dataset", "type": "mechanical"},
                {"id": "r3", "criterion": "Reasoning is sound", "type": "judgmental"},
            ],
        }
        trace = {"final_output": "ds_ft_q1_2026_top50", "agent_steps": []}
        out = mechanical.evaluate(scenario, trace)
        self.assertIsNone(out["r3"])
        self.assertIsNotNone(out["r1"])

    def test_render_grid_formats_cells(self):
        scenario = {
            "rubric": [
                {"id": "r1", "criterion": "x", "type": "mechanical"},
                {"id": "r2", "criterion": "y", "type": "mechanical"},
                {"id": "r3", "criterion": "z", "type": "judgmental"},
            ],
        }
        results = {
            "r1": {"pass": True, "note": ""},
            "r2": {"pass": False, "note": ""},
            "r3": None,
        }
        s = mechanical.render_grid(scenario, results)
        self.assertEqual(s, "[r1:\u2713 r2:\u2717 r3:?]")


if __name__ == "__main__":
    unittest.main()
