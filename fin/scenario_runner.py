#!/usr/bin/env python3
"""
Scenario runner: find assumptions that hit $500K ARR by August 2027.

Tests multiple named scenarios with different assumption tweaks,
projects 16 months (May 2026 → Aug 2027), and ranks by plausibility.
"""

import copy
import yaml
from pathlib import Path
from run_model import project_monthly, format_currency
import pandas as pd


def load_base():
    path = Path(__file__).parent / "marketplace_model.yaml"
    with open(path) as f:
        return yaml.safe_load(f)


def make_scenario(base, name, overrides, notes):
    """Create a scenario by applying overrides dict to base config."""
    cfg = copy.deepcopy(base)
    cfg["assumptions"]["time"]["months"] = 16  # extend to Aug 2027
    for section, key, val in overrides:
        cfg["assumptions"][section][key] = val
    return {"name": name, "config": cfg, "overrides": overrides, "notes": notes}


def run_scenarios():
    base = load_base()

    scenarios = []

    # --- Scenario 1: Aggressive demand + lower churn ---
    scenarios.append(make_scenario(base,
        "High Demand, Low Churn",
        [
            ("demand", "buyer_signups", 500),
            ("transactions", "activation_rate", 0.40),
            ("liquidity", "buyer_churn_monthly", 0.05),
            ("transactions", "purchases_per_buyer_per_month", 3),
        ],
        "2.5x initial buyers, better activation (40%), halve churn to 5%, 3 purchases/mo"
    ))

    # --- Scenario 2: Premium pricing + higher ATV ---
    scenarios.append(make_scenario(base,
        "Premium Pricing",
        [
            ("demand", "buyer_signups", 350),
            ("transactions", "average_transaction_value", 1200),
            ("transactions", "activation_rate", 0.35),
            ("liquidity", "buyer_churn_monthly", 0.06),
            ("transactions", "take_rate", 0.18),
        ],
        "Enterprise focus: $1200 ATV, 18% take rate, 350 initial buyers, 6% churn"
    ))

    # --- Scenario 3: Supply-led growth (more sellers = more buyers via network effects) ---
    scenarios.append(make_scenario(base,
        "Supply-Led Growth",
        [
            ("supply", "sellers_onboarded", 100),
            ("supply", "supply_growth_rate_mom", 0.22),
            ("demand", "buyer_signups", 400),
            ("transactions", "activation_rate", 0.35),
            ("liquidity", "buyer_churn_monthly", 0.06),
            ("transactions", "purchases_per_buyer_per_month", 3),
        ],
        "2x starting sellers, 22% supply growth, 400 buyers, 3 purchases/mo, 6% churn"
    ))

    # --- Scenario 4: Balanced growth ---
    scenarios.append(make_scenario(base,
        "Balanced Growth",
        [
            ("demand", "buyer_signups", 400),
            ("transactions", "activation_rate", 0.35),
            ("transactions", "purchases_per_buyer_per_month", 2.5),
            ("transactions", "average_transaction_value", 750),
            ("liquidity", "buyer_churn_monthly", 0.06),
            ("liquidity", "repeat_purchase_rate", 0.50),
            ("supply", "supply_growth_rate_mom", 0.18),
        ],
        "Moderate improvements across the board: $750 ATV, 2.5 purchases/mo, 6% churn, 50% repeat"
    ))

    # --- Scenario 5: Volume play ---
    scenarios.append(make_scenario(base,
        "Volume Play",
        [
            ("demand", "buyer_signups", 600),
            ("transactions", "activation_rate", 0.30),
            ("transactions", "purchases_per_buyer_per_month", 4),
            ("transactions", "average_transaction_value", 300),
            ("liquidity", "buyer_churn_monthly", 0.07),
            ("liquidity", "repeat_purchase_rate", 0.55),
        ],
        "High volume, lower price: 600 buyers, 4 purchases/mo, $300 ATV, 55% repeat"
    ))

    # --- Scenario 6: Enterprise whale strategy ---
    scenarios.append(make_scenario(base,
        "Enterprise Whales",
        [
            ("demand", "buyer_signups", 150),
            ("transactions", "activation_rate", 0.50),
            ("transactions", "purchases_per_buyer_per_month", 5),
            ("transactions", "average_transaction_value", 2000),
            ("transactions", "take_rate", 0.20),
            ("liquidity", "buyer_churn_monthly", 0.04),
            ("liquidity", "repeat_purchase_rate", 0.60),
            ("supply", "supply_growth_rate_mom", 0.18),
        ],
        "Fewer but larger enterprise buyers: $2K ATV, 20% take, 5 purchases/mo, 4% churn"
    ))

    # --- Scenario 7: Moderate realistic ---
    scenarios.append(make_scenario(base,
        "Moderate Realistic",
        [
            ("demand", "buyer_signups", 350),
            ("transactions", "activation_rate", 0.35),
            ("transactions", "purchases_per_buyer_per_month", 3),
            ("transactions", "average_transaction_value", 650),
            ("liquidity", "buyer_churn_monthly", 0.06),
            ("liquidity", "repeat_purchase_rate", 0.45),
            ("supply", "supply_growth_rate_mom", 0.18),
        ],
        "Conservative stretch: $650 ATV, 3 purchases/mo, 350 buyers, 18% supply growth"
    ))

    # --- Scenario 8: Churn-focused ---
    scenarios.append(make_scenario(base,
        "Retention Focus",
        [
            ("demand", "buyer_signups", 300),
            ("transactions", "activation_rate", 0.40),
            ("transactions", "purchases_per_buyer_per_month", 3),
            ("transactions", "average_transaction_value", 600),
            ("liquidity", "buyer_churn_monthly", 0.03),
            ("liquidity", "repeat_purchase_rate", 0.60),
        ],
        "Best-in-class retention: 3% churn, 60% repeat, $600 ATV, 300 buyers"
    ))

    # Run all scenarios
    print("=" * 90)
    print(f"  SCENARIO ANALYSIS: $500K ARR by 2027-08")
    print("=" * 90)

    results = []
    for s in scenarios:
        df = project_monthly(s["config"])
        aug_row = df[df["month"] == "2027-08"]
        if aug_row.empty:
            aug_arr = df.iloc[-1]["arr"]
            aug_month = df.iloc[-1]["month"]
        else:
            aug_arr = aug_row.iloc[0]["arr"]
            aug_month = "2027-08"

        # Find first month hitting $500K
        hit_month = None
        for _, row in df.iterrows():
            if row["arr"] >= 500_000:
                hit_month = row["month"]
                break

        results.append({
            "name": s["name"],
            "aug_arr": aug_arr,
            "hit_500k": hit_month or "Never",
            "notes": s["notes"],
            "df": df,
        })

    # Sort by ARR descending
    results.sort(key=lambda x: x["aug_arr"], reverse=True)

    # Print summary table
    print(f"\n{'Scenario':<25} {'Aug 2027 ARR':>14} {'Hits $500K':>12}  Notes")
    print("-" * 110)
    for r in results:
        marker = " <<< TARGET" if r["aug_arr"] >= 500_000 else ""
        print(f"{r['name']:<25} {format_currency(r['aug_arr']):>14} {r['hit_500k']:>12}  {r['notes']}{marker}")

    # Print detailed monthly for top scenarios that hit the target
    hits = [r for r in results if r["aug_arr"] >= 500_000]
    if hits:
        print(f"\n\n{'=' * 90}")
        print(f"  DETAILED PROJECTIONS FOR SCENARIOS HITTING $500K ARR")
        print(f"{'=' * 90}")
        for r in hits:
            df = r["df"]
            print(f"\n--- {r['name']} ---")
            print(f"    {r['notes']}")
            cols = ["month", "sellers", "total_listings", "match_rate",
                    "buyer_signups", "active_buyers",
                    "monthly_transactions", "gmv", "net_revenue", "arr"]
            display = df[cols].copy()
            display["gmv"] = display["gmv"].apply(format_currency)
            display["net_revenue"] = display["net_revenue"].apply(format_currency)
            display["arr"] = display["arr"].apply(format_currency)
            pd.set_option("display.width", 200)
            print(display.to_string(index=False))
    else:
        print("\n  *** No scenarios hit $500K ARR by Aug 2027 ***")
        print("  Closest scenarios listed above. Consider more aggressive assumptions.")

    return results


if __name__ == "__main__":
    run_scenarios()
