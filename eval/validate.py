"""Minimal shape validator for scenarios, traces, and verdicts.

Hand-rolled so the eval harness has zero runtime dependencies beyond the
agent it's running. If validation grows complex, swap in jsonschema.
"""

from __future__ import annotations

from typing import Any


class SchemaError(ValueError):
    pass


def _require(obj: dict, key: str, kind: type, where: str) -> Any:
    if key not in obj:
        raise SchemaError(f"{where}: missing required field '{key}'")
    val = obj[key]
    if not isinstance(val, kind):
        raise SchemaError(
            f"{where}: '{key}' must be {kind.__name__}, got {type(val).__name__}"
        )
    return val


def validate_scenario(obj: dict) -> None:
    where = "scenario"
    sid = _require(obj, "scenario_id", str, where)
    where = f"scenario[{sid}]"
    _require(obj, "description", str, where)

    buyer = _require(obj, "buyer_context", dict, where)
    _require(buyer, "task", str, f"{where}.buyer_context")
    # budget_usd and constraints are optional

    _require(obj, "market_fixture", str, where)

    truth = _require(obj, "ground_truth", dict, where)
    for key in ("correct_seller_ids", "correct_dataset_ids",
                "must_not_retain_fields", "expected_conclusion_contains"):
        if key in truth and not isinstance(truth[key], list):
            raise SchemaError(f"{where}.ground_truth.{key} must be list")

    rubric = _require(obj, "rubric", list, where)
    if not rubric:
        raise SchemaError(f"{where}.rubric must not be empty")
    ids = set()
    for i, item in enumerate(rubric):
        loc = f"{where}.rubric[{i}]"
        if not isinstance(item, dict):
            raise SchemaError(f"{loc} must be object")
        rid = _require(item, "id", str, loc)
        if rid in ids:
            raise SchemaError(f"{loc}: duplicate rubric id '{rid}'")
        ids.add(rid)
        _require(item, "criterion", str, loc)
        rtype = _require(item, "type", str, loc)
        if rtype not in ("mechanical", "judgmental"):
            raise SchemaError(f"{loc}: type must be 'mechanical' or 'judgmental'")


def validate_fixture(obj: dict) -> None:
    where = "fixture"
    fid = _require(obj, "fixture_id", str, where)
    where = f"fixture[{fid}]"

    sellers = _require(obj, "sellers", list, where)
    seller_ext_ids: set[str] = set()
    for i, s in enumerate(sellers):
        loc = f"{where}.sellers[{i}]"
        if not isinstance(s, dict):
            raise SchemaError(f"{loc} must be object")
        ext = _require(s, "external_id", str, loc)
        if ext in seller_ext_ids:
            raise SchemaError(f"{loc}: duplicate external_id '{ext}'")
        seller_ext_ids.add(ext)
        _require(s, "name", str, loc)
        _require(s, "email", str, loc)

    listings = _require(obj, "listings", list, where)
    listing_ext_ids: set[str] = set()
    for i, l in enumerate(listings):
        loc = f"{where}.listings[{i}]"
        if not isinstance(l, dict):
            raise SchemaError(f"{loc} must be object")
        ext = _require(l, "external_id", str, loc)
        if ext in listing_ext_ids:
            raise SchemaError(f"{loc}: duplicate external_id '{ext}'")
        listing_ext_ids.add(ext)
        ses = _require(l, "seller_external_id", str, loc)
        if ses not in seller_ext_ids:
            raise SchemaError(f"{loc}: seller_external_id '{ses}' not in fixture sellers")
        _require(l, "title", str, loc)
        _require(l, "description", str, loc)
        _require(l, "category", str, loc)
        _require(l, "price_cents", int, loc)
        if "tags" in l and not isinstance(l["tags"], list):
            raise SchemaError(f"{loc}: tags must be list")


def validate_verdict(obj: dict) -> None:
    where = "verdict"
    _require(obj, "run_id", str, where)
    _require(obj, "scenario_id", str, where)
    _require(obj, "judge", str, where)
    _require(obj, "judged_at", str, where)
    results = _require(obj, "rubric_results", list, where)
    for i, r in enumerate(results):
        loc = f"{where}.rubric_results[{i}]"
        if not isinstance(r, dict):
            raise SchemaError(f"{loc} must be object")
        _require(r, "id", str, loc)
        # `pass` may be true/false/None (null) when judgmental is unrated.
        if "pass" not in r:
            raise SchemaError(f"{loc}: missing 'pass'")
    overall = _require(obj, "overall", str, where)
    if overall not in ("pass", "fail", "mixed", "pending"):
        raise SchemaError(f"{where}.overall must be one of pass/fail/mixed/pending")
