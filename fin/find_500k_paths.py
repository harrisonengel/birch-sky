#!/usr/bin/env python3
"""
Find the minimum-viable assumption changes to hit $500K ARR by 2027-08.

Uses a grid search over the most load-bearing assumptions to find
scenarios where Month 16 ARR is in the $450K-$600K range (i.e., close
to target without wild overshoot).
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

    # Load-bearing params from sensitivity analysis:
    # 1. buyer_churn_monthly (biggest lever)
    # 2. supply_growth_rate_mom (drives buyer growth)
    # 3. activation_rate, purchases/buyer, ATV, take_rate, buyer_signups (all ~20% linear)

    # Grid over realistic ranges
    grid = {
        "buyer_signups": [200, 300, 400],
        "activation_rate": [0.30, 0.35, 0.40],
        "purchases_per_buyer_per_month": [2, 2.5, 3],
        "average_transaction_value": [500, 650, 800],
        "buyer_churn_monthly": [0.04, 0.06, 0.08],
        "supply_growth_rate_mom": [0.15, 0.18, 0.22],
        "repeat_purchase_rate": [0.40, 0.50],
        "take_rate": [0.15, 0.18],
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

        a["demand"]["buyer_signups"] = params["buyer_signups"]
        a["transactions"]["activation_rate"] = params["activation_rate"]
        a["transactions"]["purchases_per_buyer_per_month"] = params["purchases_per_buyer_per_month"]
        a["transactions"]["average_transaction_value"] = params["average_transaction_value"]
        a["liquidity"]["buyer_churn_monthly"] = params["buyer_churn_monthly"]
        a["supply"]["supply_growth_rate_mom"] = params["supply_growth_rate_mom"]
        a["liquidity"]["repeat_purchase_rate"] = params["repeat_purchase_rate"]
        a["transactions"]["take_rate"] = params["take_rate"]

        df = project_monthly(cfg)
        month1_arr = df.iloc[0]["arr"]
        aug_arr = df.iloc[-1]["arr"]

        # Find when $500K is first hit
        hit_month = None
        for _, row in df.iterrows():
            if row["arr"] >= 500_000:
                hit_month = row["month"]
                break

        results.append({
            **params,
            "month1_arr": month1_arr,
            "aug2027_arr": aug_arr,
            "hit_500k": hit_month or "Never",
        })

    rdf = pd.DataFrame(results)

    # Filter: hits $500K by 2027-08, but doesn't start above $500K in month 1
    # and ideally hits close to target (not wildly over)
    viable = rdf[
        (rdf["aug2027_arr"] >= 480_000) &
        (rdf["aug2027_arr"] <= 700_000) &
        (rdf["month1_arr"] < 300_000)
    ].copy()

    viable["gap_to_500k"] = viable["aug2027_arr"] - 500_000
    viable["abs_gap"] = viable["gap_to_500k"].abs()
    viable = viable.sort_values("abs_gap")

    # Count how many params changed from baseline
    baseline = {
        "buyer_signups": 200, "activation_rate": 0.30,
        "purchases_per_buyer_per_month": 2, "average_transaction_value": 500,
        "buyer_churn_monthly": 0.10, "supply_growth_rate_mom": 0.15,
        "repeat_purchase_rate": 0.40, "take_rate": 0.15,
    }
    def count_changes(row):
        return sum(1 for k, v in baseline.items() if row[k] != v)

    viable["num_changes"] = viable.apply(count_changes, axis=1)
    viable = viable.sort_values(["num_changes", "abs_gap"])

    print(f"\n{'=' * 100}")
    print(f"  VIABLE PATHS TO ~$500K ARR BY AUG 2027")
    print(f"  (Filtered: $480K-$700K final ARR, <$300K starting ARR)")
    print(f"  Total viable: {len(viable)} out of {len(combos)} tested")
    print(f"{'=' * 100}\n")

    # Show top 20 by fewest changes, closest to target
    display = viable.head(20).copy()
    display["month1_arr"] = display["month1_arr"].apply(format_currency)
    display["aug2027_arr"] = display["aug2027_arr"].apply(format_currency)
    display["gap_to_500k"] = display["gap_to_500k"].apply(format_currency)

    show_cols = ["buyer_signups", "activation_rate", "purchases_per_buyer_per_month",
                 "average_transaction_value", "buyer_churn_monthly", "supply_growth_rate_mom",
                 "repeat_purchase_rate", "take_rate", "month1_arr", "aug2027_arr",
                 "hit_500k", "gap_to_500k", "num_changes"]
    print(display[show_cols].to_string(index=False))

    # Now pick the top 3 most plausible and show detailed projections
    print(f"\n\n{'=' * 100}")
    print(f"  TOP 3 MOST LIKELY PATHS (fewest assumption changes, closest to $500K)")
    print(f"{'=' * 100}")

    for i, (_, row) in enumerate(viable.head(3).iterrows()):
        cfg = copy.deepcopy(base)
        a = cfg["assumptions"]
        a["time"]["months"] = 16
        a["demand"]["buyer_signups"] = row["buyer_signups"]
        a["transactions"]["activation_rate"] = row["activation_rate"]
        a["transactions"]["purchases_per_buyer_per_month"] = row["purchases_per_buyer_per_month"]
        a["transactions"]["average_transaction_value"] = row["average_transaction_value"]
        a["liquidity"]["buyer_churn_monthly"] = row["buyer_churn_monthly"]
        a["supply"]["supply_growth_rate_mom"] = row["supply_growth_rate_mom"]
        a["liquidity"]["repeat_purchase_rate"] = row["repeat_purchase_rate"]
        a["transactions"]["take_rate"] = row["take_rate"]

        df = project_monthly(cfg)

        changes = []
        bl = {"buyer_signups": 200, "activation_rate": 0.30,
              "purchases_per_buyer_per_month": 2, "average_transaction_value": 500,
              "buyer_churn_monthly": 0.10, "supply_growth_rate_mom": 0.15,
              "repeat_purchase_rate": 0.40, "take_rate": 0.15}
        for k, v in bl.items():
            if row[k] != v:
                changes.append(f"  {k}: {v} → {row[k]}")

        print(f"\n--- Path {i+1}: {int(row['num_changes'])} assumption changes ---")
        for c in changes:
            print(c)
        print()

        cols = ["month", "sellers", "buyer_signups", "active_buyers",
                "monthly_transactions", "gmv", "net_revenue", "arr"]
        disp = df[cols].copy()
        disp["gmv"] = disp["gmv"].apply(format_currency)
        disp["net_revenue"] = disp["net_revenue"].apply(format_currency)
        disp["arr_raw"] = df["arr"]
        disp["arr"] = disp["arr"].apply(format_currency)
        print(disp.drop(columns=["arr_raw"]).to_string(index=False))
        print()


if __name__ == "__main__":
    run_grid()
