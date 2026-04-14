import { MOCK_RESULTS, generateBuyRequest } from './mock-data.js';
import {
  enterMarketplace,
  initiatePurchase,
  confirmPurchase,
  createBuyOrder,
  startPrepper,
  respondPrepper,
} from './api-client.js';

function delay(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function showOverlay(title, lines) {
  return new Promise((resolve) => {
    const overlay = document.getElementById('overlay');
    overlay.innerHTML = '';
    overlay.classList.remove('hidden');

    const card = document.createElement('div');
    card.className = 'overlay-card';
    card.innerHTML = `<h2>${title}</h2>`;
    lines.forEach((line) => {
      const p = document.createElement('p');
      p.textContent = line;
      card.appendChild(p);
    });
    const btn = document.createElement('button');
    btn.textContent = 'Done';
    btn.addEventListener('click', () => {
      overlay.classList.add('hidden');
      resolve();
    });
    card.appendChild(btn);
    overlay.appendChild(card);
  });
}

// Generate a simple buyer ID for this browser session.
function getBuyerID() {
  let id = sessionStorage.getItem('ie_buyer_id');
  if (!id) {
    id = 'buyer-' + Math.random().toString(36).slice(2, 10);
    sessionStorage.setItem('ie_buyer_id', id);
  }
  return id;
}

// Try the real backend; return null on failure so we can fall back.
async function enterBackend(query) {
  try {
    const resp = await enterMarketplace(query);
    if (resp && resp.results && resp.results.length > 0) {
      return resp.results.map((r) => ({
        id: r.listing_id,
        title: r.title,
        seller: r.seller_name || (r.category ? r.category.charAt(0).toUpperCase() + r.category.slice(1) + ' data' : 'Marketplace'),
        description: r.description,
        price: r.price_cents / 100,
        trustScore: Math.min(99, 70 + Math.floor(r.score * 30)),
      }));
    }
    return []; // no results from backend
  } catch {
    return null; // backend unreachable
  }
}

export function initDemoFlow(scene, chat) {
  let running = false;
  let backendAvailable = null; // null = unknown, true/false after first check

  // Prepper state machine. While prepperSession is set, user messages are
  // routed to /api/prepper/respond instead of the marketplace search. Once
  // a Briefing is produced (or the prepper service is unavailable), we
  // fall through to the existing runFlow.
  let prepperSession = null;
  let prepperBriefing = null;
  let prepperDisabled = false; // sticky once we determine prepper is unreachable

  function getPath() {
    const toggle = document.getElementById('demo-mode');
    if (toggle) {
      const val = toggle.value;
      if (val === 'results') return 'happy';
      if (val === 'no-results') return 'no-results';
    }
    return 'auto';
  }

  // Render any prepper response — either a clarifying question (status="asking")
  // or a finalized briefing (status="ready"). Returns true if the loop is
  // still asking and the next user message should go to /respond.
  function renderPrepperTurn(turn) {
    if (turn.status === 'asking' && turn.question) {
      chat.addMessage('agent', turn.question);
      return true;
    }
    if (turn.status === 'ready' && turn.briefing) {
      prepperBriefing = turn.briefing;
      prepperSession = null;
      const summary = turn.briefing.goal_summary || '';
      chat.addMessage(
        'agent',
        `Got it. Searching the Exchange for: ${summary}`
      );
      return false;
    }
    return false;
  }

  // First user message of a session: try to start a prepper conversation.
  // On any failure (network, 503, etc.), mark prepper disabled for this
  // page session and let the caller fall through to the direct flow.
  async function tryStartPrepper(query) {
    if (prepperDisabled) return null;
    // Demo toggles short-circuit prepper so e2e tests stay deterministic.
    if (getPath() !== 'auto') {
      prepperDisabled = true;
      return null;
    }
    try {
      const buyerID = getBuyerID();
      const turn = await startPrepper(buyerID, query);
      return turn;
    } catch {
      prepperDisabled = true;
      backendAvailable = false;
      return null;
    }
  }

  async function tryRespondPrepper(answer) {
    try {
      return await respondPrepper(prepperSession, answer);
    } catch {
      // Mid-conversation failure: drop prepper, treat the latest answer as
      // the buyer's final query, and fall through to the direct flow.
      prepperSession = null;
      prepperDisabled = true;
      return null;
    }
  }

  // Build the search string passed into /api/v1/enter. If a prepper Briefing
  // exists, fold its goal_summary + selection_criteria into the query so the
  // marketplace search reflects everything the buyer clarified — the
  // briefing is the only thing that crosses into the walled system.
  function buildEnterQuery(query) {
    if (!prepperBriefing) return query;
    const parts = [prepperBriefing.goal_summary || query];
    const criteria = prepperBriefing.selection_criteria || [];
    if (criteria.length > 0) {
      parts.push(criteria.join(' '));
    }
    return parts.filter(Boolean).join(' ').trim() || query;
  }

  async function runFlow(query) {
    if (running) return;
    running = true;
    chat.setInputEnabled(false);

    const forcedPath = getPath();

    // Agent acknowledges
    chat.addMessage('agent', 'Let me check the Exchange for you...');
    chat.addTypingIndicator();
    await delay(800);

    // Run to building
    await scene.agentRunTo();

    // Building working
    await scene.buildingWork(1500);

    // Run back
    await scene.agentRunBack();
    chat.removeTypingIndicator();

    // Try real backend first (unless forced to no-results)
    let results = null;
    if (forcedPath !== 'no-results') {
      results = await enterBackend(buildEnterQuery(query));
      if (results === null) {
        // Backend unreachable — use mock data
        if (backendAvailable === null) {
          chat.addMessage('agent', '(Using demo data — backend not connected)');
        }
        backendAvailable = false;
        results = MOCK_RESULTS;
      } else {
        backendAvailable = true;
      }
    }

    if (forcedPath === 'no-results' || (results !== null && results.length === 0)) {
      await runNoResultsPath(query);
    } else {
      await runHappyPath(query, results);
    }

    running = false;
    chat.setInputEnabled(true);
  }

  async function runHappyPath(_query, results) {
    chat.addMessage('agent', 'I found several relevant sources on the Exchange:');
    await delay(300);
    chat.addResults(results);
  }

  async function runNoResultsPath(query) {
    chat.addMessage(
      'agent',
      "I searched the Exchange but couldn't find sources that match your needs well enough. The available data didn't meet the quality threshold."
    );
    await delay(1000);
    chat.addMessage(
      'agent',
      'I recommend posting a buy request so sellers can compete to provide what you need. Here\'s a draft based on your query:'
    );
    await delay(300);
    chat.addBuyRequestForm(query);
  }

  // Wire up handlers. The user-message router has three branches:
  //   1. If a prepper conversation is in progress, route to /respond.
  //   2. Otherwise, try to START a prepper conversation. If we get back a
  //      clarifying question, render it and stop here (await next message).
  //   3. If prepper finalized immediately, or is unavailable / disabled,
  //      fall through to runFlow with the briefing as context (when present).
  chat.onUserMessage = async (text) => {
    if (running) return;

    // Branch 1: in the middle of a clarification loop.
    if (prepperSession) {
      chat.setInputEnabled(false);
      chat.addTypingIndicator();
      const turn = await tryRespondPrepper(text);
      chat.removeTypingIndicator();
      chat.setInputEnabled(true);
      if (turn) {
        const stillAsking = renderPrepperTurn(turn);
        if (stillAsking) {
          prepperSession = turn.session_id;
          return;
        }
        // status === "ready": Briefing captured; fall through to runFlow
        // using the original query that kicked off the conversation. We
        // don't have it on hand so use the buyer's most recent answer as
        // the query — it's the freshest framing they gave us.
        runFlow(text);
        return;
      }
      // Failure mid-conversation: fall through to direct flow.
      runFlow(text);
      return;
    }

    // Branch 2: try to open a clarification conversation.
    // Skip the spinner if we already know prepper will short-circuit (forced
    // demo mode, or previously disabled) — keeps deterministic e2e tests
    // free of incidental UI flicker.
    const prepperEligible =
      !prepperBriefing && !prepperDisabled && getPath() === 'auto';
    if (prepperEligible) {
      chat.setInputEnabled(false);
      chat.addTypingIndicator();
      const turn = await tryStartPrepper(text);
      chat.removeTypingIndicator();
      chat.setInputEnabled(true);
      if (turn) {
        const stillAsking = renderPrepperTurn(turn);
        if (stillAsking) {
          prepperSession = turn.session_id;
          return;
        }
        // Finalized on the first turn — fall through with the briefing.
      }
    }

    // Branch 3: direct flow.
    runFlow(text);
  };

  chat.onBuyClick = async (result, btn) => {
    btn.disabled = true;
    btn.textContent = 'Processing...';

    const buyerID = getBuyerID();

    try {
      // Real purchase flow: initiate then confirm
      const purchase = await initiatePurchase(buyerID, result.id);

      if (purchase.already_owned) {
        btn.textContent = 'Already Owned';
        await showOverlay('Already Purchased', [
          result.title,
          'You already own this dataset.',
        ]);
        return;
      }

      const ownership = await confirmPurchase(purchase.transaction_id);
      btn.textContent = 'Purchased';
      await showOverlay('Purchase Confirmed', [
        result.title,
        `Amount: $${result.price.toFixed(2)}`,
        `Transaction: ${ownership.transaction_id.slice(0, 8)}...`,
      ]);
    } catch (err) {
      // Fallback to demo overlay if backend is down
      btn.textContent = 'Purchased';
      await showOverlay('Purchase Confirmed', [
        result.title,
        `Amount: $${result.price.toFixed(2)}`,
      ]);
    }
  };

  chat.onBuyRequestSubmit = async (data) => {
    const buyerID = getBuyerID();

    try {
      // Real buy order creation — combine title and description into query
      const maxPriceCents = Math.round(parseFloat(data.maxPrice || '5') * 100);
      const query = data.description
        ? `${data.title}: ${data.description}`
        : data.title;
      await createBuyOrder({
        buyerID,
        query,
        maxPriceCents,
        category: '',
      });
      await showOverlay('Buy Request Posted', [
        data.title,
        `Max price: $${data.maxPrice}`,
        `Expires: ${data.expiration}`,
        'Sellers will be notified. You\'ll be alerted when a match is found.',
      ]);
    } catch {
      // Fallback
      await showOverlay('Buy Request Posted', [
        data.title,
        `Max price: $${data.maxPrice}`,
        `Expires: ${data.expiration}`,
        'Sellers will be notified. You\'ll be alerted when a match is found.',
      ]);
    }
  };

  // Add demo toggle to page
  const toggle = document.createElement('div');
  toggle.id = 'demo-toggle';
  toggle.innerHTML = `
    <span>Demo:</span>
    <select id="demo-mode">
      <option value="auto">Auto</option>
      <option value="results">Always Results</option>
      <option value="no-results">Always No Results</option>
    </select>
  `;
  document.body.appendChild(toggle);

  // Welcome message
  chat.addMessage('agent', 'Welcome to The Information Exchange. What information are you looking for today?');
}
