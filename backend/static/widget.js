// Wartet, bis die gesamte HTML-Seite geladen ist
document.addEventListener("DOMContentLoaded", () => {

  // --- CONFIGURATION ---
  const BACKEND_URL = "https://nxt-4llp.onrender.com";
  const FRONTEND_URL = "https://nextreklam.com.tr";

  // --- 1. LEAD TRACKING CONVERSION ---
  function trackLeadConversion(source) {
    window.dataLayer = window.dataLayer || [];
    window.dataLayer.push({
      event: "lead_conversion",
      source: source
    });
  }

  // Event-Delegation fuer das Tracking
  document.addEventListener("click", (e) => {
    const trackTarget = e.target.closest("[data-track]");
    if (trackTarget) {
      trackLeadConversion(trackTarget.dataset.track);
    }
  });

  // --- 2. GLOBALER CLICK-LISTENER (Event-Delegation) ---
  document.addEventListener("click", (e) => {
    const chatWindow = document.getElementById('chat-window');
    const chatInput = document.getElementById('chat-input');

    // A: Klick auf den Kapsel-Button (chat-toggle)
    if (e.target.closest('#chat-toggle')) {
      if (!chatWindow) {
        // Falls wir auf der Galerie-Seite sind und das HTML fehlt
        window.location.href = FRONTEND_URL + "/?openchat=true";
        return;
      }
      chatWindow.style.display = (chatWindow.style.display === 'none' || chatWindow.style.display === '') ? 'flex' : 'none';
      if (chatWindow.style.display === 'flex' && chatInput) {
        chatInput.focus();
      }
      return;
    }

    // B: Klick auf das Schliessen-Kreuz (chat-close)
    if (e.target.closest('#chat-close')) {
      if (chatWindow) chatWindow.style.display = 'none';
      return;
    }

    // C: Klick auf den Navigations-Link (nav-asistan)
    if (e.target.closest('#nav-asistan')) {
      e.preventDefault();
      if (!chatWindow) {
        window.location.href = FRONTEND_URL + "/?openchat=true";
        return;
      }
      chatWindow.style.display = (chatWindow.style.display === 'none' || chatWindow.style.display === '') ? 'flex' : 'none';
      if (chatWindow.style.display === 'flex' && chatInput) {
        chatInput.focus();
      }
      return;
    }

    // D: Klick auf den Senden-Button (chat-send)
    if (e.target.closest('#chat-send')) {
      sendMessage();
    }
  });

  // --- 3. INPUT-FIELD LISTENER (Enter-Taste) ---
  document.addEventListener("keypress", (e) => {
    if (e.key === 'Enter' && e.target.id === 'chat-input') {
      sendMessage();
    }
  });

  // --- 4. REDIRECT CHECK ---
  const urlParams = new URLSearchParams(window.location.search);
  const initialChatWindow = document.getElementById('chat-window');
  const initialChatInput = document.getElementById('chat-input');
  
  if (initialChatWindow && urlParams.get("openchat") === "true") {
    initialChatWindow.style.display = 'flex';
    if (initialChatInput) initialChatInput.focus();
    window.history.replaceState({}, document.title, window.location.pathname);
  }

  // --- 5. FUNCTIONS (Message Handling) ---
  function appendMessage(text, sender) {
    const chatMessages = document.getElementById('chat-messages');
    if (!chatMessages) return;
    
    const msgHtml = document.createElement('div');
    msgHtml.style.padding = '10px 14px';
    msgHtml.style.borderRadius = '12px';
    msgHtml.style.fontSize = '14px';
    msgHtml.style.lineHeight = '1.4';
    msgHtml.style.wordBreak = 'break-word';
    msgHtml.style.boxShadow = '0 1px 2px rgba(0,0,0,0.05)';
    msgHtml.style.marginBottom = '8px';      
    msgHtml.style.maxWidth = '95%';
    msgHtml.style.width = 'fit-content';
    
    if (sender === 'user') {
      msgHtml.style.background = '#ff6a00';
      msgHtml.style.color = 'white';
      msgHtml.style.marginLeft = 'auto';
      msgHtml.style.borderBottomRightRadius = '2px';
    } else {
      if (sender === 'bot-loading') {
        msgHtml.classList.add('bot-loading');
      }
      msgHtml.style.background = '#eef2f6';
      msgHtml.style.color = '#13192d';
      msgHtml.style.marginRight = 'auto';
      msgHtml.style.borderBottomLeftRadius = '2px';
    }
    
    msgHtml.textContent = text;
    chatMessages.appendChild(msgHtml);
    chatMessages.scrollTop = chatMessages.scrollHeight;
  }

  async function sendMessage() {
    const chatInput = document.getElementById('chat-input');
    const chatMessages = document.getElementById('chat-messages');
    if (!chatInput) return;

    const text = chatInput.value.trim();
    if (!text) return;

    appendMessage(text, 'user');
    chatInput.value = '';

    appendMessage('Düşünüyorum...', 'bot-loading');

    try {
      const response = await fetch(`${BACKEND_URL}/api/chat`, {
        method: 'POST',
        headers: { 
          'Content-Type': 'application/json',
          'X-Requested-With': 'XMLHttpRequest'
        },
        body: JSON.stringify({ message: text })
      });

      if (!response.ok) {
        throw new Error(`Chat failed with status ${response.status}`);
      }

      const data = await response.json();
      
      if (chatMessages) {
        const loadingIndicator = chatMessages.querySelector('.bot-loading');
        if (loadingIndicator) loadingIndicator.remove();
      }
      
      const botResponse = data.response || data.reply || data.textResponse || data.error;
      
      if (botResponse) {
        appendMessage(botResponse, 'bot');
      } else {
        appendMessage('Şu an yanıt veremiyorum. Lütfen daha sonra tekrar deneyin.', 'bot');
      }
    } catch (error) {
      if (chatMessages) {
        const loadingIndicator = chatMessages.querySelector('.bot-loading');
        if (loadingIndicator) loadingIndicator.remove();
      }
      appendMessage('Bağlantı hatası oluştu. Lütfen WhatsApp hattımızı kullanın.', 'bot');
      console.error('Chat Error:', error);
    }
  }
});
