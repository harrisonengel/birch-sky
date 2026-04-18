import { generateBuyRequest } from './mock-data.js';
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

function getBuyerID() {
  let id = sessionStorage.getItem('ie_buyer_id');
  if (!id) {
    id = 'buyer-' + Math.random().toString(36).slice(2, 10);
    sessionStorage.setItem('ie_buyer_id', id);
  }
  return id;
}

async function enterBackend(query) {
  const resp = await enterMarketplace(query);
  const listings = (resp && resp.buy_listings) || [];
  return listings.map((r) => ({
    id: r.id,
    title: r.listing_description,
    seller: r.seller || 'Marketplace',
    description: r.listing_description,
    price: (r.price || 0) / 100,
    trustScore: 85,
  }));
}

export function initDemoFlow(scene, chat) {
  let running = false;

  // Prepper state machine. While prepperSession is set, user messages are
  // routed to /api/prepper/respond instead of the marketplace search. Once
  // a Briefing is produced (or the prepper service is unavailable), we
  // fall through to the existing runFlow.
  let prepperSession = null;
  let prepperBriefing = null;
  let prepperDisabled = false;

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

  async function tryStartPrepper(query) {
    if (prepperDisabled) return null;
    try {
      const buyerID = getBuyerID();
      const turn = await startPrepper(buyerID, query);
      return turn;
    } catch {
      prepperDisabled = true;
      return null;
    }
  }

  async function tryRespondPrepper(answer) {
    try {
      return await respondPrepper(prepperSession, answer);
    } catch {
      prepperSession = null;
      prepperDisabled = true;
      return null;
    }
  }

  // Build the search string passed into the harness /enter endpoint. If a
  // prepper Briefing exists, fold its goal_summary + selection_criteria into
  // the query so the marketplace search reflects everything the buyer
  // clarified — the briefing is the only thing that crosses into the walled
  // system.
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

    chat.addMessage('agent', 'Let me check the Exchange for you...');
    chat.addTypingIndicator();
    await delay(800);

    await scene.agentRunTo();
    await scene.buildingWork(1500);
    await scene.agentRunBack();
    chat.removeTypingIndicator();

    let results;
    try {
      results = await enterBackend(buildEnterQuery(query));
    } catch {
      chat.addMessage('agent', 'Something went wrong. Please try again in a moment.');
      running = false;
      chat.setInputEnabled(true);
      return;
    }

    if (results.length === 0) {
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

  chat.onUserMessage = async (text) => {
    if (running) return;

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
        runFlow(text);
        return;
      }
      runFlow(text);
      return;
    }

    const prepperEligible = !prepperBriefing && !prepperDisabled;
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
      }
    }

    runFlow(text);
  };

  chat.onBuyClick = async (result, btn) => {
    btn.disabled = true;
    btn.textContent = 'Processing...';

    const buyerID = getBuyerID();

    try {
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
    } catch {
      btn.disabled = false;
      btn.textContent = `Buy - $${result.price.toFixed(2)}`;
      await showOverlay('Something went wrong', [
        'We couldn\'t complete the purchase. Please try again.',
      ]);
    }
  };

  chat.onBuyRequestSubmit = async (data) => {
    const buyerID = getBuyerID();

    try {
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
      await showOverlay('Something went wrong', [
        'We couldn\'t post your buy request. Please try again.',
      ]);
    }
  };

  chat.addMessage('agent', 'Welcome to The Information Exchange. What information are you looking for today?');
}
