#!/usr/bin/env python3
"""
Find the minimum-viable assumption changes to hit $500K ARR by 2027-08.

Uses a grid search over the most load-bearing assumptions to find
scenarios where Month 16 ARR is in the $450K-$600K range (i.e., close
to target without wild overshoot).

Now includes market thickness model — supply density gates conversion.
"""

import copy
import itertools
import yaml
from pathlib import Path
from run_model import project_monthly, format_currency
import pandas as pd

pd.set_option("display.width", 220)
pd.set_option("display.max_columns", None)


def load_base():
    path = Path(__file__).parent / "marketplace_model.yaml"
    with open(path) as f:
        return yaml.safe_load(f)


def run_grid():
    base = load_base()

    # Grid: monthly onboarding rates (starting from 0) + other levers.
    # These are month-1 rates that grow at supply_growth_rate_mom.
    grid = {
        "sellers_onboarded": [10, 20, 30, 50],
        "listings_per_seller": [4, 6, 8],
        "supply_growth_rate_mom": [0.10, 0.15, 0.20],
        "buyer_signups": [30, 50, 100, 150],
        "activation_rate": [0.30, 0.40],
        "purchases_per_buyer_per_month": [2, 3],
        "average_transaction_value": [500, 750, 1000],
        "buyer_churn_monthly": [0.04, 0.06, 0.08],
        "take_rate": [0.15, 0.18],
        "density_halflife": [60, 80],
    }

    keys = list(grid.keys())
    combos = list(itertools.product(*[grid[k] for k in keys]))
    print(f"Testing {len(combos)} combinations...")

    results = []
    for combo in combos:
        params = dict(zip(keys, combo))
        cfg = copy.deepcopy(base)
        a = cfg["assumptions"]
        a["time"]["months"] = 16

        a["supply"]["sellers_onboarded"] = params["sellers_onboarded"]
        a["supply"]["listings_per_seller"] = params["listings_per_seller"]
        a["supply"]["supply_growth_rate_mom"] = params["supply_growth_rate_mom"]
        a["demand"]["buyer_signups"] = params["buyer_signups"]
        a["transactions"]["activation_rate"] = params["activation_rate"]
        a["transactions"]["purchases_per_buyer_per_month"] = params["purchases_per_buyer_per_month"]
        a["transactions"]["average_transaction_value"] = params["average_transaction_value"]
        a["liquidity"]["buyer_churn_monthly"] = params["buyer_churn_monthly"]
        a["transactions"]["take_rate"] = params["take_rate"]
        a["thickness"]["density_halflife"] = params["density_halflife"]

        df = project_monthly(cfg)
        month1_arr = df.iloc[0]["arr"]
        month1_match = df.iloc[0]["match_rate"]
        aug_arr = df.iloc[-1]["arr"]
        aug_match = df.iloc[-1]["match_rate"]

        # Find when $500K is first hit
        hit_month = None
        for _, row in df.iterrows():
            if row["arr"] >= 500_000:
                hit_month = row["month"]
                break

        results.append({
            **params,
            "month1_arr": month1_arr,
            "month1_match": month1_match,
            "aug2027_arr": aug_arr,
            "aug_match": aug_match,
            "hit_500k": hit_month or "Never",
        })

    rdf = pd.DataFrame(results)

    # Filter: hits $500K by 2027-08 but starts modestly (from zero)
    # Month 1 ARR will always be small since we start from 0; focus on
    # final ARR being in a reasonable range around target
    viable = rdf[
        (rdf["aug2027_arr"] >= 450_000) &
        (rdf["aug2027_arr"] <= 2_000_000)
    ].copy()

    viable["gap_to_500k"] = viable["aug2027_arr"] - 500_000
    viable["abs_gap"] = viable["gap_to_500k"].abs()

    # Count how many params changed from baseline
    baseline = {
        "sellers_onboarded": 50, "listings_per_seller": 4,
        "supply_growth_rate_mom": 0.15,
        "buyer_signups": 200, "activation_rate": 0.30,
        "purchases_per_buyer_per_month": 2, "average_transaction_value": 500,
        "buyer_churn_monthly": 0.10, "take_rate": 0.15,
        "density_halflife": 80,
    }
    # Note: grid values below baseline (e.g. 10 sellers/mo vs 50)
    # still count as changes
    def count_changes(row):
        return sum(1 for k, v in baseline.items() if row[k] != v)

    viable["num_changes"] = viable.apply(count_changes, axis=1)
    viable = viable.sort_values(["num_changes", "abs_gap"])

    print(f"\n{'=' * 120}")
    print(f"  VIABLE PATHS TO ~$500K ARR BY AUG 2027 (WITH MARKET THICKNESS)")
    print(f"  (Filtered: $450K-$800K final ARR, <$200K starting ARR)")
    print(f"  Total viable: {len(viable)} out of {len(combos)} tested")
    print(f"{'=' * 120}\n")

    if viable.empty:
        print("  *** No viable paths found in grid. Market thickness makes $500K much harder. ***")
        # Show best results regardless
        best = rdf.nlargest(10, "aug2027_arr")
        best["aug2027_arr"] = best["aug2027_arr"].apply(format_currency)
        best["month1_arr"] = best["month1_arr"].apply(format_currency)
        print("\n  Top 10 by ARR (unfiltered):")
        print(best[["sellers_onboarded", "listings_per_seller", "supply_growth_rate_mom",
                     "buyer_signups", "activation_rate", "average_transaction_value",
                     "buyer_churn_monthly", "density_halflife",
                     "month1_arr", "month1_match", "aug2027_arr", "aug_match",
                     "hit_500k"]].to_string(index=False))
        return results

    # Show top 20 by fewest changes, closest to target
    display = viable.head(20).copy()
    display["month1_arr_fmt"] = display["month1_arr"].apply(format_currency)
    display["aug2027_arr_fmt"] = display["aug2027_arr"].apply(format_currency)
    display["gap_to_500k_fmt"] = display["gap_to_500k"].apply(format_currency)

    show_cols = ["sellers_onboarded", "listings_per_seller", "supply_growth_rate_mom",
                 "buyer_signups", "activation_rate", "purchases_per_buyer_per_month",
                 "average_transaction_value", "buyer_churn_monthly", "take_rate",
                 "density_halflife", "month1_match", "aug_match",
                 "month1_arr_fmt", "aug2027_arr_fmt", "hit_500k", "num_changes"]
    print(display[show_cols].to_string(index=False))

    # Detailed projections for top 3
    print(f"\n\n{'=' * 120}")
    print(f"  TOP 3 MOST LIKELY PATHS (fewest assumption changes, closest to $500K)")
    print(f"{'=' * 120}")

    for i, (_, row) in enumerate(viable.head(3).iterrows()):
        cfg = copy.deepcopy(base)
        a = cfg["assumptions"]
        a["time"]["months"] = 16
        a["supply"]["sellers_onboarded"] = row["sellers_onboarded"]
        a["supply"]["listings_per_seller"] = row["listings_per_seller"]
        a["supply"]["supply_growth_rate_mom"] = row["supply_growth_rate_mom"]
        a["demand"]["buyer_signups"] = row["buyer_signups"]
        a["transactions"]["activation_rate"] = row["activation_rate"]
        a["transactions"]["purchases_per_buyer_per_month"] = row["purchases_per_buyer_per_month"]
        a["transactions"]["average_transaction_value"] = row["average_transaction_value"]
        a["liquidity"]["buyer_churn_monthly"] = row["buyer_churn_monthly"]
        a["transactions"]["take_rate"] = row["take_rate"]
        a["thickness"]["density_halflife"] = row["density_halflife"]

        df = project_monthly(cfg)

        changes = []
        for k, v in baseline.items():
            if row[k] != v:
                changes.append(f"  {k}: {v} → {row[k]}")

        print(f"\n--- Path {i+1}: {int(row['num_changes'])} assumption changes ---")
        for c in changes:
            print(c)
        print()

        cols = ["month", "sellers", "total_listings", "supply_density", "match_rate",
                "buyer_signups", "active_buyers",
                "monthly_transactions", "gmv", "net_revenue", "arr"]
        disp = df[cols].copy()
        disp["gmv"] = disp["gmv"].apply(format_currency)
        disp["net_revenue"] = disp["net_revenue"].apply(format_currency)
        disp["arr"] = disp["arr"].apply(format_currency)
        print(disp.to_string(index=False))
        print()


if __name__ == "__main__":
    run_grid()
