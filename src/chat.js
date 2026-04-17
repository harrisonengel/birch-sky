import { generateBuyRequest } from './mock-data.js';

export function initChat(panel) {
  const messagesEl = panel.querySelector('#chat-messages') || document.getElementById('chat-messages');
  const inputEl = panel.querySelector('#chat-input') || document.getElementById('chat-input');
  const sendBtn = panel.querySelector('#chat-send') || document.getElementById('chat-send');

  let onUserMessage = null;
  let onBuyClick = null;
  let onBuyRequestSubmit = null;

  function scrollToBottom() {
    messagesEl.scrollTop = messagesEl.scrollHeight;
  }

  function addMessage(sender, text) {
    const bubble = document.createElement('div');
    bubble.className = `chat-bubble ${sender}`;
    bubble.textContent = text;
    messagesEl.appendChild(bubble);
    scrollToBottom();
    return bubble;
  }

  function addTypingIndicator() {
    const bubble = document.createElement('div');
    bubble.className = 'chat-bubble agent typing-indicator';
    bubble.innerHTML = '<span>.</span><span>.</span><span>.</span>';
    bubble.dataset.typing = 'true';
    messagesEl.appendChild(bubble);
    scrollToBottom();
    return bubble;
  }

  function removeTypingIndicator() {
    const el = messagesEl.querySelector('[data-typing="true"]');
    if (el) el.remove();
  }

  function addResults(results) {
    const container = document.createElement('div');
    container.className = 'chat-bubble agent';
    container.style.padding = '8px';
    container.style.background = 'transparent';
    container.style.border = 'none';

    results.forEach((r, i) => {
      const card = document.createElement('div');
      card.className = 'result-card';
      card.innerHTML = `
        <div class="result-title">${r.title}</div>
        <div class="result-seller">${r.seller}</div>
        <div class="result-desc">${r.description}</div>
        <div class="result-footer">
          <span class="trust-score">Trust: ${r.trustScore}%
            <span class="trust-bar"><span class="trust-bar-fill" style="width:${r.trustScore}%"></span></span>
          </span>
          <button class="buy-btn" data-result-id="${r.id}">Buy - $${r.price.toFixed(2)}</button>
        </div>
      `;
      container.appendChild(card);

      // Staggered fade-in
      setTimeout(() => card.classList.add('visible'), 100 * (i + 1));

      // Buy button handler
      const btn = card.querySelector('.buy-btn');
      btn.addEventListener('click', () => {
        if (onBuyClick) onBuyClick(r, btn);
      });
    });

    messagesEl.appendChild(container);
    scrollToBottom();
  }

  function addBuyRequestForm(query) {
    const prefill = generateBuyRequest(query);
    const container = document.createElement('div');
    container.className = 'chat-bubble agent';
    container.style.padding = '8px';
    container.style.background = 'transparent';
    container.style.border = 'none';

    const form = document.createElement('div');
    form.className = 'buy-request-form';
    form.innerHTML = `
      <button type="button" class="br-close" aria-label="Dismiss buy request">×</button>
      <label>Request Title</label>
      <input type="text" class="br-title" value="${prefill.title}" />
      <label>Description</label>
      <textarea class="br-desc">${prefill.description}</textarea>
      <label>Max Price ($)</label>
      <input type="text" class="br-price" value="${prefill.maxPrice}" />
      <label>Expiration</label>
      <input type="text" class="br-expiry" value="${prefill.expiration}" />
      <button class="submit-btn">Post Buy Request</button>
    `;

    form.querySelector('.submit-btn').addEventListener('click', () => {
      const data = {
        title: form.querySelector('.br-title').value,
        description: form.querySelector('.br-desc').value,
        maxPrice: form.querySelector('.br-price').value,
        expiration: form.querySelector('.br-expiry').value,
      };
      if (onBuyRequestSubmit) onBuyRequestSubmit(data);
    });

    form.querySelector('.br-close').addEventListener('click', () => {
      container.remove();
    });

    container.appendChild(form);
    messagesEl.appendChild(container);
    scrollToBottom();
  }

  function setInputEnabled(enabled) {
    inputEl.disabled = !enabled;
    sendBtn.disabled = !enabled;
  }

  function handleSend() {
    const text = inputEl.value.trim();
    if (!text) return;
    inputEl.value = '';
    addMessage('user', text);
    if (onUserMessage) onUserMessage(text);
  }

  inputEl.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') handleSend();
  });
  sendBtn.addEventListener('click', handleSend);

  return {
    addMessage,
    addTypingIndicator,
    removeTypingIndicator,
    addResults,
    addBuyRequestForm,
    setInputEnabled,
    set onUserMessage(fn) { onUserMessage = fn; },
    set onBuyClick(fn) { onBuyClick = fn; },
    set onBuyRequestSubmit(fn) { onBuyRequestSubmit = fn; },
  };
}
