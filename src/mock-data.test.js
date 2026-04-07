import { describe, it, expect } from 'vitest';
import { MOCK_RESULTS, generateBuyRequest } from './mock-data.js';

describe('MOCK_RESULTS', () => {
  it('contains entries with the expected shape', () => {
    expect(MOCK_RESULTS.length).toBeGreaterThan(0);
    for (const r of MOCK_RESULTS) {
      expect(r).toMatchObject({
        id: expect.any(Number),
        title: expect.any(String),
        seller: expect.any(String),
        description: expect.any(String),
        price: expect.any(Number),
        trustScore: expect.any(Number),
      });
      expect(r.trustScore).toBeGreaterThanOrEqual(0);
      expect(r.trustScore).toBeLessThanOrEqual(100);
      expect(r.price).toBeGreaterThan(0);
    }
  });

  it('has unique ids', () => {
    const ids = MOCK_RESULTS.map((r) => r.id);
    expect(new Set(ids).size).toBe(ids.length);
  });
});

describe('generateBuyRequest', () => {
  it('uses the full query as title when short', () => {
    const req = generateBuyRequest('pricing data for laptops');
    expect(req.title).toBe('pricing data for laptops');
    expect(req.description).toContain('pricing data for laptops');
    expect(req.maxPrice).toBe('5.00');
    expect(req.expiration).toBe('7 days');
  });

  it('truncates long queries with an ellipsis', () => {
    const longQuery = 'a'.repeat(100);
    const req = generateBuyRequest(longQuery);
    expect(req.title.length).toBeLessThanOrEqual(63); // 60 + '...'
    expect(req.title.endsWith('...')).toBe(true);
  });

  it('does not truncate at exactly 60 characters', () => {
    const query = 'a'.repeat(60);
    const req = generateBuyRequest(query);
    expect(req.title).toBe(query);
    expect(req.title.endsWith('...')).toBe(false);
  });
});
