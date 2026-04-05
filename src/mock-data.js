export const MOCK_RESULTS = [
  {
    id: 1,
    title: 'Consumer Electronics Pricing Index Q1 2026',
    seller: 'RetailMetrics Inc.',
    description: 'Aggregated pricing data across 12 major retailers, 50k+ SKUs',
    price: 2.50,
    trustScore: 94,
  },
  {
    id: 2,
    title: 'Amazon Category Pricing - Electronics Under $100',
    seller: 'DataHarvest',
    description: 'Daily pricing snapshots, 30-day history, 8,200 products',
    price: 1.75,
    trustScore: 87,
  },
  {
    id: 3,
    title: 'Walmart vs Amazon Price Comparison Dataset',
    seller: 'ShopIntel',
    description: 'Side-by-side pricing on overlapping catalog, updated weekly',
    price: 3.00,
    trustScore: 91,
  },
  {
    id: 4,
    title: 'Google Shopping Ads Benchmark - Electronics',
    seller: 'AdDataCo',
    description: 'CPC, impression share, and pricing signals from Shopping ads',
    price: 4.25,
    trustScore: 78,
  },
];

export function generateBuyRequest(query) {
  return {
    title: query.length > 60 ? query.slice(0, 60) + '...' : query,
    description: `Seeking data to answer: "${query}". Require current data with verifiable sourcing.`,
    maxPrice: '5.00',
    expiration: '7 days',
  };
}
