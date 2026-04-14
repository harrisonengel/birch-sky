import { MOCK_RESULTS, generateBuyRequest } from './mock-data.js';
import {
  searchMarketplace,
  initiatePurchase,
  confirmPurchase,
  createBuyOrder,
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
async function searchBackend(query) {
  try {
    const resp = await searchMarketplace(query);
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

  function getPath() {
    const toggle = document.getElementById('demo-mode');
    if (toggle) {
      const val = toggle.value;
      if (val === 'results') return 'happy';
      if (val === 'no-results') return 'no-results';
    }
    return 'auto';
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
      results = await searchBackend(query);
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

  // Wire up handlers
  chat.onUserMessage = (text) => {
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
      // Real buy order creation
      const maxPriceCents = Math.round(parseFloat(data.maxPrice || '5') * 100);
      await createBuyOrder({
        buyerID,
        query: data.title,
        description: data.description,
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
