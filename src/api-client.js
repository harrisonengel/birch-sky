// Thin API client. The frontend's only path into the agent layer is the
// harness `/enter` endpoint — direct calls to the market platform's search
// API are not allowed. Other market-platform endpoints (listings, purchases,
// buy-orders) are still called directly for now since they are transactional,
// not agent-mediated. In dev, Vite proxies /api/v1 -> market-platform :8080
// and /agent -> harness :8000.

const API_BASE = '/api/v1';
const AGENT_BASE = '/agent';

async function post(url, body) {
  const resp = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!resp.ok) {
    const err = await resp.json().catch(() => ({ error: resp.statusText }));
    throw new Error(err.error || `HTTP ${resp.status}`);
  }
  return resp.json();
}

async function get(url) {
  const resp = await fetch(url);
  if (!resp.ok) {
    const err = await resp.json().catch(() => ({ error: resp.statusText }));
    throw new Error(err.error || `HTTP ${resp.status}`);
  }
  return resp.json();
}

/**
 * Enter the marketplace via the agent harness. The harness is the only
 * service the frontend talks to for catalog access; it forwards to the
 * market platform's /api/v1/search endpoint internally.
 * @param {string} query - Natural language query
 * @returns {Promise<{results: Array, total: number, mode: string}>}
 */
export async function enterMarketplace(query) {
  return post(`${AGENT_BASE}/enter`, {
    query,
    mode: 'text',
    per_page: 10,
  });
}

/**
 * Get a single listing by ID.
 * @param {string} id
 * @returns {Promise<object>}
 */
export async function getListing(id) {
  return get(`${API_BASE}/listings/${id}`);
}

/**
 * Initiate a purchase for a listing.
 * @param {string} buyerID
 * @param {string} listingID
 * @returns {Promise<{transaction_id: string, client_secret?: string, already_owned: boolean}>}
 */
export async function initiatePurchase(buyerID, listingID) {
  return post(`${API_BASE}/purchases`, {
    buyer_id: buyerID,
    listing_id: listingID,
  });
}

/**
 * Confirm a purchase (completes transaction, creates ownership).
 * @param {string} transactionID
 * @returns {Promise<object>}
 */
export async function confirmPurchase(transactionID) {
  return post(`${API_BASE}/purchases/${transactionID}/confirm`, {});
}

/**
 * Post a buy order (request for data).
 * @param {object} params
 * @returns {Promise<object>}
 */
export async function createBuyOrder({ buyerID, query, description, maxPriceCents, category }) {
  return post(`${API_BASE}/buy-orders`, {
    buyer_id: buyerID,
    query: query,
    max_price_cents: maxPriceCents,
    currency: 'usd',
    category: category || '',
    criteria: {},
  });
}

/**
 * Start a new prepper clarification session.
 * @param {string} buyerID
 * @param {string} initialQuery
 * @returns {Promise<{session_id: string, status: string, turn: number, question?: string, briefing?: object}>}
 */
export async function startPrepper(buyerID, initialQuery) {
  return post(`/api/prepper/start`, {
    buyer_id: buyerID,
    initial_query: initialQuery,
  });
}

/**
 * Submit an answer to the prepper's latest clarifying question.
 * @param {string} sessionID
 * @param {string} answer
 * @returns {Promise<{session_id: string, status: string, turn: number, question?: string, briefing?: object}>}
 */
export async function respondPrepper(sessionID, answer) {
  return post(`/api/prepper/respond`, {
    session_id: sessionID,
    answer: answer,
  });
}

/**
 * Run the buyer agent via the harness service.
 * @param {string} userInput
 * @param {object} context - {background, goal, constraints}
 * @returns {Promise<{response: string}>}
 */
export async function runAgent(userInput, context) {
  return post(`${AGENT_BASE}/run`, {
    starting_context: context || {
      background: 'You are helping a buyer find data on the Information Exchange.',
      goal: 'Find relevant data listings.',
      constraints: '',
    },
    user_input: userInput,
    max_turns: 10,
  });
}

/**
 * Check if the backend is reachable.
 * @returns {Promise<boolean>}
 */
export async function checkHealth() {
  try {
    const resp = await fetch(`${API_BASE}/../health`);
    return resp.ok;
  } catch {
    return false;
  }
}
