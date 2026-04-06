#!/usr/bin/env python3
"""
IE Marketplace Financial Model Runner

Loads a YAML config with assumptions, projects monthly metrics,
checks targets, and identifies load-bearing assumptions.

Usage:
    python fin/run_model.py                              # default config
    python fin/run_model.py --scenario fin/my_scenario.yaml
    python fin/run_model.py --output markdown             # output as markdown
    python fin/run_model.py --output csv                  # output as csv
    python fin/run_model.py --sensitivity                 # run sensitivity analysis
"""

import argparse
import math
import sys
from pathlib import Path

import yaml
import pandas as pd
from dateutil.relativedelta import relativedelta
from datetime import datetime


def load_config(path: str) -> dict:
    with open(path) as f:
        return yaml.safe_load(f)


def project_monthly(config: dict) -> pd.DataFrame:
    """Project monthly metrics from assumptions over the time horizon."""
    a = config["assumptions"]
    supply = a["supply"]
    demand = a["demand"]
    txn = a["transactions"]
    liq = a["liquidity"]
    trust = a["trust"]
    thick = a.get("thickness", {})
    time = a["time"]

    months = time["months"]
    start = datetime.strptime(time["start_month"], "%Y-%m")

    rows = []
    # Running state
    cumulative_verified_outcomes = trust["verified_outcome_count"]

    for m in range(months):
        month_date = start + relativedelta(months=m)
        month_label = month_date.strftime("%Y-%m")
        growth_factor = (1 + supply["supply_growth_rate_mom"]) ** m
        txn_growth = (1 + txn["transaction_growth_rate_mom"]) ** m

        # --- Phase 1: Supply ---
        sellers = supply["sellers_onboarded"] * growth_factor
        total_listings = sellers * supply["listings_per_seller"]
        categories = min(supply["listing_category_count"] * (1 + 0.05 * m),
                         supply["listing_category_count"] * 3)  # cap at 3x

        # --- Phase 1b: Market Thickness ---
        # match_rate = how likely a buyer query finds a relevant listing
        # Saturating exponential: thin markets convert poorly, thick ones plateau
        if thick:
            supply_density = total_listings / max(categories, 1)
            density_hl = thick["density_halflife"]
            max_mr = thick["max_match_rate"]
            match_rate = max_mr * (1 - math.exp(-supply_density / density_hl))
        else:
            # Legacy: no thickness model, match_rate = 1 (no gating)
            supply_density = total_listings / max(categories, 1)
            match_rate = 1.0

        # --- Phase 2: Demand ---
        # Buyer growth loosely tracks supply growth (network effects)
        buyer_growth = (1 + supply["supply_growth_rate_mom"] * 0.8) ** m
        buyer_signups = demand["buyer_signups"] * buyer_growth
        active_queriers = buyer_signups * demand["buyer_to_query_conversion"]
        total_queries = active_queriers * demand["queries_per_active_buyer"]
        bounties = demand["bounties_posted"] * buyer_growth

        # Fulfillment driven by supply density, not calendar time
        base_fulfill = demand["query_fulfillment_rate"]
        query_fulfillment = min(0.95, base_fulfill + (1 - base_fulfill) * match_rate)
        base_bounty = demand["bounty_fulfillment_rate"]
        bounty_fulfillment = min(0.80, base_bounty + (1 - base_bounty) * match_rate)

        # --- Phase 3: Transactions ---
        # Activation gated by match_rate: thin market → fewer buyers convert
        effective_activation = txn["activation_rate"] * match_rate
        active_buyers = buyer_signups * effective_activation
        # Churn increases when market is thin
        churn_penalty = thick.get("churn_thickness_penalty", 0) * (1 - match_rate)
        effective_churn = liq["buyer_churn_monthly"] + churn_penalty
        effective_churn = min(effective_churn, 0.50)  # cap at 50%
        retention = (1 - effective_churn) ** m
        retained_buyers = active_buyers * retention
        # Add repeat buyers
        repeat_buyers = retained_buyers * liq["repeat_purchase_rate"]
        effective_buyers = retained_buyers + repeat_buyers * 0.5  # repeat buyers buy more

        monthly_transactions = effective_buyers * txn["purchases_per_buyer_per_month"]
        atv = txn["average_transaction_value"]
        gmv = monthly_transactions * atv
        take_rate = txn["take_rate"]
        net_revenue = gmv * take_rate
        arr = net_revenue * 12  # annualized run rate

        # --- Phase 4: Liquidity ---
        listing_liq = monthly_transactions / max(total_listings, 1)
        b2s_ratio = buyer_signups / max(sellers, 1)

        # --- Phase 5: Trust ---
        monthly_disputes = monthly_transactions * trust["dispute_rate"]
        resolved = monthly_disputes * trust["dispute_resolution_rate"]
        cumulative_verified_outcomes += resolved
        reliability_score = min(1.0, 0.5 + trust["reliability_score_improvement_mom"] * m)

        rows.append({
            "month": month_label,
            # Supply
            "sellers": round(sellers),
            "total_listings": round(total_listings),
            "categories": round(categories, 1),
            # Thickness
            "supply_density": round(supply_density, 1),
            "match_rate": round(match_rate, 3),
            # Demand
            "buyer_signups": round(buyer_signups),
            "active_queriers": round(active_queriers),
            "total_queries": round(total_queries),
            "bounties": round(bounties),
            "query_fulfillment_rate": round(query_fulfillment, 3),
            "bounty_fulfillment_rate": round(bounty_fulfillment, 3),
            # Transactions
            "active_buyers": round(retained_buyers),
            "monthly_transactions": round(monthly_transactions),
            "atv": round(atv, 2),
            "gmv": round(gmv, 2),
            "net_revenue": round(net_revenue, 2),
            "arr": round(arr, 2),
            "take_rate": take_rate,
            # Liquidity
            "listing_liquidity": round(listing_liq, 3),
            "buyer_seller_ratio": round(b2s_ratio, 2),
            "repeat_purchase_rate": liq["repeat_purchase_rate"],
            # Trust
            "dispute_rate": trust["dispute_rate"],
            "monthly_disputes": round(monthly_disputes),
            "verified_outcomes_cumulative": round(cumulative_verified_outcomes),
            "reliability_score": round(reliability_score, 3),
        })

    return pd.DataFrame(rows)


def check_targets(df: pd.DataFrame, config: dict) -> pd.DataFrame:
    """Check each target against projected metrics."""
    targets = config.get("targets", [])
    results = []

    metric_map = {
        "total_listings": "total_listings",
        "monthly_transactions": "monthly_transactions",
        "listing_liquidity_pct": "listing_liquidity",
        "arr": "arr",
    }

    for t in targets:
        col = metric_map.get(t["metric"])
        if col is None:
            results.append({**t, "projected": "N/A", "status": "UNKNOWN METRIC"})
            continue

        target_month = t["by"]
        matching = df[df["month"] == target_month]
        if matching.empty:
            # Find closest month
            results.append({**t, "projected": "N/A", "status": "OUTSIDE RANGE"})
            continue

        projected = matching.iloc[0][col]
        hit = projected >= t["value"]
        results.append({
            "name": t["name"],
            "metric": t["metric"],
            "target": t["value"],
            "by": t["by"],
            "projected": round(projected, 2),
            "status": "HIT" if hit else "MISS",
            "gap": round(projected - t["value"], 2),
        })

    return pd.DataFrame(results)


def sensitivity_analysis(config: dict) -> pd.DataFrame:
    """
    Identify load-bearing assumptions by varying each ±20%
    and measuring impact on Month 12 ARR.
    """
    base_df = project_monthly(config)
    base_arr = base_df.iloc[-1]["arr"]

    # Assumptions to test: (section, key, label)
    test_params = [
        ("supply", "sellers_onboarded", "Sellers onboarded"),
        ("supply", "listings_per_seller", "Listings/seller"),
        ("supply", "supply_growth_rate_mom", "Supply growth MoM"),
        ("demand", "buyer_signups", "Buyer signups"),
        ("demand", "buyer_to_query_conversion", "Buyer query conversion"),
        ("demand", "query_fulfillment_rate", "Query fulfillment"),
        ("transactions", "activation_rate", "Activation rate"),
        ("transactions", "purchases_per_buyer_per_month", "Purchases/buyer/mo"),
        ("transactions", "average_transaction_value", "Avg transaction value"),
        ("transactions", "take_rate", "Take rate"),
        ("transactions", "transaction_growth_rate_mom", "Txn growth MoM"),
        ("liquidity", "repeat_purchase_rate", "Repeat purchase rate"),
        ("liquidity", "buyer_churn_monthly", "Buyer churn monthly"),
        ("liquidity", "seller_churn_monthly", "Seller churn monthly"),
    ]

    # Add thickness params if present
    if "thickness" in config["assumptions"]:
        test_params.extend([
            ("thickness", "max_match_rate", "Max match rate"),
            ("thickness", "density_halflife", "Density halflife"),
            ("thickness", "churn_thickness_penalty", "Churn thickness penalty"),
            ("thickness", "price_acceptance_rate", "Price acceptance rate"),
        ])

    results = []
    for section, key, label in test_params:
        import copy
        base_val = config["assumptions"][section][key]

        for delta_pct in [-0.20, 0.20]:
            mod_config = copy.deepcopy(config)
            new_val = base_val * (1 + delta_pct)
            mod_config["assumptions"][section][key] = new_val
            mod_df = project_monthly(mod_config)
            mod_arr = mod_df.iloc[-1]["arr"]
            arr_change = mod_arr - base_arr
            arr_change_pct = arr_change / base_arr if base_arr else 0

            results.append({
                "assumption": label,
                "base_value": round(base_val, 4),
                "delta": f"{delta_pct:+.0%}",
                "new_value": round(new_val, 4),
                "arr_impact": round(arr_change, 2),
                "arr_impact_pct": f"{arr_change_pct:+.1%}",
            })

    df = pd.DataFrame(results)
    # Sort by absolute impact descending
    df["_abs_impact"] = df["arr_impact"].abs()
    df = df.sort_values("_abs_impact", ascending=False).drop(columns=["_abs_impact"])
    return df


def format_currency(val):
    """Format large numbers as $X.XK or $X.XM."""
    if abs(val) >= 1_000_000:
        return f"${val/1_000_000:.1f}M"
    elif abs(val) >= 1_000:
        return f"${val/1_000:.1f}K"
    return f"${val:.0f}"


def print_summary(df: pd.DataFrame):
    """Print a high-level summary of the projection."""
    first = df.iloc[0]
    last = df.iloc[-1]
    print("\n" + "=" * 60)
    print(f"  IE MARKETPLACE MODEL — {first['month']} to {last['month']}")
    print("=" * 60)
    print(f"  Month 1  ARR:  {format_currency(first['arr'])}")
    print(f"  Month {len(df):>2} ARR:  {format_currency(last['arr'])}")
    print(f"  Month {len(df):>2} GMV:  {format_currency(last['gmv'])}/mo")
    print(f"  Match rate:    {first['match_rate']:>6.1%} → {last['match_rate']:>6.1%}")
    print(f"  Sellers:       {first['sellers']:>6} → {last['sellers']:>6}")
    print(f"  Listings:      {first['total_listings']:>6} → {last['total_listings']:>6}")
    print(f"  Active buyers: {first['active_buyers']:>6} → {last['active_buyers']:>6}")
    print(f"  Txns/mo:       {first['monthly_transactions']:>6} → {last['monthly_transactions']:>6}")
    print(f"  Take rate:     {last['take_rate']:.0%}")
    print("=" * 60)


def main():
    parser = argparse.ArgumentParser(description="IE Marketplace Financial Model")
    parser.add_argument("--scenario", default=None,
                        help="Path to YAML scenario file")
    parser.add_argument("--output", choices=["table", "markdown", "csv"], default="table",
                        help="Output format")
    parser.add_argument("--sensitivity", action="store_true",
                        help="Run sensitivity analysis on assumptions")
    parser.add_argument("--save", default=None,
                        help="Save output to file")
    args = parser.parse_args()

    # Find config
    if args.scenario:
        config_path = args.scenario
    else:
        # Default: look relative to this script
        config_path = Path(__file__).parent / "marketplace_model.yaml"

    config = load_config(config_path)

    # Project monthly metrics
    df = project_monthly(config)

    # Summary
    print_summary(df)

    # Key columns for display
    display_cols = [
        "month", "sellers", "total_listings", "supply_density", "match_rate",
        "buyer_signups", "active_buyers", "monthly_transactions",
        "gmv", "net_revenue", "arr",
        "listing_liquidity", "reliability_score",
    ]

    print("\n── Monthly Projection ──")
    display_df = df[display_cols].copy()
    display_df["gmv"] = display_df["gmv"].apply(format_currency)
    display_df["net_revenue"] = display_df["net_revenue"].apply(format_currency)
    display_df["arr"] = display_df["arr"].apply(format_currency)

    if args.output == "markdown":
        print(display_df.to_markdown(index=False))
    elif args.output == "csv":
        print(display_df.to_csv(index=False))
    else:
        pd.set_option("display.max_columns", None)
        pd.set_option("display.width", 200)
        print(display_df.to_string(index=False))

    # Targets
    targets_df = check_targets(df, config)
    if not targets_df.empty:
        print("\n── Target Assessment ──")
        if args.output == "markdown":
            print(targets_df.to_markdown(index=False))
        else:
            print(targets_df.to_string(index=False))

    # Sensitivity
    if args.sensitivity:
        sens_df = sensitivity_analysis(config)
        print("\n── Sensitivity Analysis (±20% each assumption → ARR impact) ──")
        if args.output == "markdown":
            print(sens_df.to_markdown(index=False))
        else:
            pd.set_option("display.max_rows", None)
            print(sens_df.to_string(index=False))

    # Save
    if args.save:
        save_path = Path(args.save)
        if save_path.suffix == ".csv":
            df.to_csv(save_path, index=False)
        elif save_path.suffix == ".md":
            with open(save_path, "w") as f:
                f.write(f"# {config['model_name']}\n\n")
                f.write(f"Generated: {config.get('date', 'N/A')}\n\n")
                f.write("## Monthly Projection\n\n")
                f.write(df[display_cols].to_markdown(index=False))
                f.write("\n\n## Targets\n\n")
                f.write(targets_df.to_markdown(index=False))
                if args.sensitivity:
                    f.write("\n\n## Sensitivity Analysis\n\n")
                    sens_df = sensitivity_analysis(config)
                    f.write(sens_df.to_markdown(index=False))
        else:
            df.to_csv(save_path, index=False)
        print(f"\nSaved to {save_path}")


if __name__ == "__main__":
    main()
