import sys
import unittest
from pathlib import Path

# Make `eval` importable when running tests from repo root.
_REPO = Path(__file__).resolve().parents[2]
if str(_REPO) not in sys.path:
    sys.path.insert(0, str(_REPO))

from eval import mechanical  # noqa: E402


SCENARIO_DATASET_MATCH = {
    "scenario_id": "t_match",
    "ground_truth": {
        "correct_dataset_ids": ["ds_ft_q1_2026_top50"],
        "expected_conclusion_contains": [],
    },
    "rubric": [
        {"id": "r1", "criterion": "Agent identifies the correct dataset", "type": "mechanical"},
    ],
}


SCENARIO_DECLINE = {
    "scenario_id": "t_decline",
    "ground_truth": {
        "correct_dataset_ids": [],
    },
    "rubric": [
        {"id": "r1", "criterion": "Agent declines to recommend (empty recommendation)", "type": "mechanical"},
    ],
}


SCENARIO_LEAKAGE = {
    "scenario_id": "t_leak",
    "ground_truth": {
        "correct_dataset_ids": [],
        "must_not_retain_fields": ["raw_device_ids", "individual_visit_timestamps"],
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
        {"id": "r_concl", "criterion": "buy_listings contains the key terms", "type": "mechanical"},
    ],
}


class MechanicalTests(unittest.TestCase):
    def test_correct_dataset_match_passes(self):
        trace = {
            "recommendations": [
                {"seller_id": "seller_x", "listing_id": "ds_ft_q1_2026_top50"},
            ],
            "buy_listings": [],
            "agent_steps": [],
        }
        out = mechanical.evaluate(SCENARIO_DATASET_MATCH, trace)
        self.assertTrue(out["r1"]["pass"])

    def test_wrong_dataset_fails(self):
        trace = {
            "recommendations": [
                {"seller_id": "seller_x", "listing_id": "ds_something_else"},
            ],
            "buy_listings": [],
            "agent_steps": [],
        }
        out = mechanical.evaluate(SCENARIO_DATASET_MATCH, trace)
        self.assertFalse(out["r1"]["pass"])
        self.assertIn("ds_ft_q1_2026_top50", out["r1"]["note"])

    def test_extra_recommendation_fails(self):
        trace = {
            "recommendations": [
                {"seller_id": "seller_x", "listing_id": "ds_ft_q1_2026_top50"},
                {"seller_id": "seller_y", "listing_id": "ds_extra"},
            ],
            "buy_listings": [],
            "agent_steps": [],
        }
        out = mechanical.evaluate(SCENARIO_DATASET_MATCH, trace)
        self.assertFalse(out["r1"]["pass"])
        self.assertIn("extra", out["r1"]["note"])

    def test_empty_recommendations_fails_when_expected(self):
        trace = {"recommendations": [], "buy_listings": [], "agent_steps": []}
        out = mechanical.evaluate(SCENARIO_DATASET_MATCH, trace)
        self.assertFalse(out["r1"]["pass"])

    def test_decline_rubric_passes_on_empty(self):
        trace = {"recommendations": [], "buy_listings": [], "agent_steps": []}
        out = mechanical.evaluate(SCENARIO_DECLINE, trace)
        self.assertTrue(out["r1"]["pass"])

    def test_decline_rubric_fails_when_agent_recommends(self):
        trace = {
            "recommendations": [{"seller_id": "s", "listing_id": "l"}],
            "buy_listings": [],
            "agent_steps": [],
        }
        out = mechanical.evaluate(SCENARIO_DECLINE, trace)
        self.assertFalse(out["r1"]["pass"])

    def test_leakage_in_reasoning_detected(self):
        trace = {
            "recommendations": [],
            "buy_listings": [],
            "agent_steps": [
                {"step": 1, "kind": "reasoning",
                 "content": "I'll pull raw_device_ids since that's the highest fidelity."},
            ],
        }
        out = mechanical.evaluate(SCENARIO_LEAKAGE, trace)
        self.assertFalse(out["r2"]["pass"])
        self.assertIn("raw_device_ids", out["r2"]["note"])

    def test_leakage_not_detected_when_reasoning_clean(self):
        trace = {
            "recommendations": [],
            "buy_listings": [],
            "agent_steps": [
                {"step": 1, "kind": "reasoning",
                 "content": "The constraints exclude any raw identifiers, so I'll pick the aggregated product."},
            ],
        }
        out = mechanical.evaluate(SCENARIO_LEAKAGE, trace)
        # "raw" doesn't match "raw_device_ids" as substring, and the reasoning
        # does not contain the forbidden field names verbatim.
        self.assertTrue(out["r2"]["pass"])

    def test_conclusion_contains_checks_buy_listings(self):
        trace = {
            "recommendations": [],
            "buy_listings": [
                {"id": "l1", "price": 100, "seller": "PlaceIQ",
                 "listing_description": "Target weekly foot traffic rollup Q1 2026"},
            ],
            "agent_steps": [],
        }
        out = mechanical.evaluate(SCENARIO_CONCLUSION, trace)
        self.assertTrue(out["r_concl"]["pass"])

    def test_conclusion_missing_terms_fails(self):
        trace = {
            "recommendations": [],
            "buy_listings": [
                {"id": "l1", "price": 100, "seller": "PlaceIQ",
                 "listing_description": "Target daily data"},
            ],
            "agent_steps": [],
        }
        out = mechanical.evaluate(SCENARIO_CONCLUSION, trace)
        self.assertFalse(out["r_concl"]["pass"])
        self.assertIn("weekly", out["r_concl"]["note"])
        self.assertIn("foot traffic", out["r_concl"]["note"])

    def test_judgmental_items_return_none(self):
        scenario = {
            **SCENARIO_DATASET_MATCH,
            "rubric": [
                {"id": "r1", "criterion": "Agent identifies the correct dataset", "type": "mechanical"},
                {"id": "r3", "criterion": "Reasoning is sound", "type": "judgmental"},
            ],
        }
        trace = {
            "recommendations": [{"seller_id": "s", "listing_id": "ds_ft_q1_2026_top50"}],
            "buy_listings": [],
            "agent_steps": [],
        }
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
