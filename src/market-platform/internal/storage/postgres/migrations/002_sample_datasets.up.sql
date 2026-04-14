-- Sample dataset tables.
--
-- These represent the structured content behind marketplace listings.
-- In production, seller data lives in the object store; for the MVP
-- demo we keep it in Postgres so buyers agents can run SQL queries
-- against it directly.

-- Consumer electronics pricing: per-product pricing across retailers
CREATE TABLE sample_consumer_electronics_pricing (
    id SERIAL PRIMARY KEY,
    product_name TEXT NOT NULL,
    category TEXT NOT NULL,
    brand TEXT NOT NULL,
    retailer TEXT NOT NULL,
    price_usd NUMERIC(10,2) NOT NULL,
    msrp_usd NUMERIC(10,2),
    discount_pct NUMERIC(5,2) NOT NULL DEFAULT 0,
    in_stock BOOLEAN NOT NULL DEFAULT true,
    price_date DATE NOT NULL
);
CREATE INDEX idx_cep_category ON sample_consumer_electronics_pricing(category);
CREATE INDEX idx_cep_retailer ON sample_consumer_electronics_pricing(retailer);

-- Cross-platform price comparison: Amazon vs Walmart vs Target
CREATE TABLE sample_ecommerce_price_comparison (
    id SERIAL PRIMARY KEY,
    product_name TEXT NOT NULL,
    category TEXT NOT NULL,
    amazon_price_usd NUMERIC(10,2),
    walmart_price_usd NUMERIC(10,2),
    target_price_usd NUMERIC(10,2),
    best_price_usd NUMERIC(10,2) NOT NULL,
    best_platform TEXT NOT NULL,
    price_spread_pct NUMERIC(5,2) NOT NULL DEFAULT 0,
    snapshot_date DATE NOT NULL
);
CREATE INDEX idx_epc_category ON sample_ecommerce_price_comparison(category);

-- Shopping ads benchmark: ad performance by category and platform
CREATE TABLE sample_shopping_ads_benchmark (
    id SERIAL PRIMARY KEY,
    category TEXT NOT NULL,
    platform TEXT NOT NULL,
    avg_cpc_usd NUMERIC(6,3) NOT NULL,
    avg_ctr_pct NUMERIC(5,2) NOT NULL,
    avg_conversion_rate_pct NUMERIC(5,2) NOT NULL,
    avg_roas NUMERIC(6,2) NOT NULL,
    monthly_spend_index INTEGER NOT NULL,
    num_advertisers INTEGER NOT NULL,
    report_month TEXT NOT NULL
);
CREATE INDEX idx_sab_category ON sample_shopping_ads_benchmark(category);
CREATE INDEX idx_sab_platform ON sample_shopping_ads_benchmark(platform);
