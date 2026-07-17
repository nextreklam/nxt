// Wartet, bis die gesamte HTML-Seite geladen ist
document.addEventListener("DOMContentLoaded", () => {

  // --- 1. LEAD TRACKING CONVERSION ---
  function trackLeadConversion(source) {
    window.dataLayer = window.dataLayer || [];
    window.dataLayer.push({
      event: "lead_conversion",
      source: source
    });
  }

  document.querySelectorAll("[data-track]").forEach((element) => {
    element.addEventListener("click", () => {
      trackLeadConversion(element.dataset.track);
    });
  });

  // --- 2. LOCAL RAG CHATBOT ACCESS ---
  const chatToggle = document.getElementById('chat-toggle');
  const chatWindow = document.getElementById('chat-window');
  const chatClose = document.getElementById('chat-close');
  const chatInput = document.getElementById('chat-input');
  const chatSend = document.getElementById('chat-send');
  const chatMessages = document.getElementById('chat-messages');

  // KORRIGIERT: Funktioniert nun auf jeder Seite (Index & Galeri), sobald der Link angeklickt wird!
  const navAsistan = document.getElementById("nav-asistan");
  if (navAsistan) {
    navAsistan.addEventListener("click", (e) => {
      e.preventDefault();
      // Falls der Nutzer auf der Galeri-Seite ist und kein Chat-Fenster existiert, leiten wir ihn auf die Startseite zum Chat um
      if (!chatWindow) {
        window.location.href = "/?openchat=true";
        return;
      }
      
      if (chatWindow.style.display === 'none' || chatWindow.style.display === '') {
        chatWindow.style.display = 'flex';
        if (chatInput) chatInput.focus();
      } else {
        chatWindow.style.display = 'none';
      }
    });
  }

  // PRÜFUNG & BEREINIGUNG: Wenn wir über einen Redirect kommen, öffnen wir den Chat automatisch
  const urlParams = new URLSearchParams(window.location.search);
  if (chatWindow && urlParams.get("openchat") === "true") {
    chatWindow.style.display = 'flex';
    if (chatInput) chatInput.focus();
    // Bereinigt die unschöne URL-Zeile im Browser (?openchat=true wird entfernt)
    window.history.replaceState({}, document.title, window.location.pathname);
  }

  // Sicherheitsprüfung für die inneren Bot-Funktionen (Senden, Empfangen, Bubbles)
  if (chatToggle && chatWindow && chatClose) {
    
    // Fenster öffnen/schließen über den runden Button unten rechts
    chatToggle.addEventListener('click', () => {
      chatWindow.style.style.display = (chatWindow.style.display === 'none' || chatWindow.style.display === '') ? 'flex' : 'none';
    });
    
    chatClose.addEventListener('click', () => { 
      chatWindow.style.display = 'none'; 
    });

    // Nachricht im Fenster anzeigen
    function appendMessage(text, sender) {
      if (!chatMessages) return;
      const msgHtml = document.createElement('div');
      
      msgHtml.style.padding = '10px 14px';
      msgHtml.style.borderRadius = '12px';
      msgHtml.style.fontSize = '14px';
      msgHtml.style.lineHeight = '1.4';
      msgHtml.style.wordBreak = 'break-word';
      msgHtml.style.boxShadow = '0 1px 2px rgba(0,0,0,0.05)';
      msgHtml.style.marginBottom = '8px';      
      // die volle Breite (95%) des Fensters nach dem Großziehen nutzt!
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

    // --- VOLLSTÄNDIGER ERSATZ FÜR SENDMESSAGE ---
    async function sendMessage() {
      const text = chatInput.value.trim();
      if (!text) return;

      appendMessage(text, 'user');
      chatInput.value = '';

      appendMessage('Düşünüyorum...', 'bot-loading');

      try {
        const response = await fetch('https://nextreklam.com.tr/api/chat', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ message: text })
        });

        if (!response.ok) {
          throw new Error(`Chat failed with status ${response.status}`);
        }

        const contentType = response.headers.get('content-type') || '';
        if (!contentType.includes('application/json')) {
          throw new Error('Non-JSON response');
        }

        const data = await response.json();
        
        const loadingIndicator = chatMessages.querySelector('.bot-loading');
        if (loadingIndicator) loadingIndicator.remove();
        
        const botResponse = data.response || data.reply || data.textResponse || data.error;
        
        if (botResponse) {
          appendMessage(botResponse, 'bot');
        } else {
          appendMessage('Şu an yanıt veremiyorum. Lütfen daha sonra tekrar deneyin.', 'bot');
        }
      } catch (error) {
        const loadingIndicator = chatMessages.querySelector('.bot-loading');
        if (loadingIndicator) loadingIndicator.remove();
        
        appendMessage('Bağlantı hatası oluştu. Lütfen WhatsApp hattımızı kullanın.', 'bot');
        console.error('Chat Error:', error);
      }
    }

    if (chatSend && chatInput) {
      chatSend.addEventListener('click', sendMessage);
      chatInput.addEventListener('keypress', (e) => { 
        if (e.key === 'Enter') sendMessage(); 
      });
    }
  }
});
