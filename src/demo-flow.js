import { MOCK_RESULTS } from './mock-data.js';

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

export function initDemoFlow(scene, chat) {
  let queryCount = 0;
  let running = false;

  function getPath() {
    const toggle = document.getElementById('demo-mode');
    if (toggle) {
      const val = toggle.value;
      if (val === 'results') return 'happy';
      if (val === 'no-results') return 'no-results';
    }
    // Auto: alternate
    return queryCount % 2 === 0 ? 'happy' : 'no-results';
  }

  async function runFlow(query) {
    if (running) return;
    running = true;
    queryCount++;
    chat.setInputEnabled(false);

    const path = getPath();

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

    if (path === 'happy') {
      await runHappyPath(query);
    } else {
      await runNoResultsPath(query);
    }

    running = false;
    chat.setInputEnabled(true);
  }

  async function runHappyPath(_query) {
    chat.addMessage('agent', 'I found several relevant sources on the Exchange:');
    await delay(300);
    chat.addResults(MOCK_RESULTS);
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
    btn.textContent = 'Purchased';
    await showOverlay('Purchase Confirmed', [
      result.title,
      `Seller: ${result.seller}`,
      `Amount: $${result.price.toFixed(2)}`,
    ]);
  };

  chat.onBuyRequestSubmit = async (data) => {
    await showOverlay('Buy Request Posted', [
      data.title,
      `Max price: $${data.maxPrice}`,
      `Expires: ${data.expiration}`,
      'Sellers will be notified. You\'ll be alerted when a match is found.',
    ]);
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
