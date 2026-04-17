import { describe, it, expect } from 'vitest';
import { generateBuyRequest } from './mock-data.js';

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
