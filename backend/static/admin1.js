// =========================================================================
// 🔥 REPARIERTE AUTOMATISCHE TIMEOUT & SESSION-SCHUTZ LOGIK
// =========================================================================
const TIMEOUT_LIMIT = 10 * 60 * 1000; // 10 Minuten Inaktivität
let inactivityTimer;

// Prüft, ob das Timeout abgelaufen ist oder ob manipuliert wurde
function checkPastTimeout() {
  const sessionActive = sessionStorage.getItem('admin_session_active');
  const lastActivity = localStorage.getItem('admin_last_activity');
  
  // Wenn der Tab neu geöffnet wurde oder keine Aktivität registriert ist
  if (!sessionActive || !lastActivity) {
    return; // Backend-Validierung über loadChatLogs() abwarten
  }

  const timePassed = Date.now() - parseInt(lastActivity);
  if (timePassed > TIMEOUT_LIMIT) {
    alert("10 dakika boyunca işlem yapılmadığı için oturumunuz güvenlik nedeniyle kapatılmıştır.");
    window.forceSessionLogout(true);
  }
}

// Setzt den Timer bei Benutzerinteraktionen zurück
function resetInactivityTimer() {
  // Aktualisiert den Timer nur, wenn die Session vom Server validiert wurde
  if (sessionStorage.getItem('admin_session_active') === 'true') {
    localStorage.setItem('admin_last_activity', Date.now());
    
    clearTimeout(inactivityTimer);
    inactivityTimer = setTimeout(() => {
      alert("10 dakika boyunca işlem yapılmadığı için oturumunuz güvenlik nedeniyle kapatılmıştır.");
      window.forceSessionLogout(true);
    }, TIMEOUT_LIMIT);
  }
}

// 🔥 REPARIERT: Löscht die Session komplett und erzwingt das Login-Fenster ohne Redirects!
window.forceSessionLogout = function(redirectToHome) {
  localStorage.removeItem('admin_last_activity');
  sessionStorage.removeItem('admin_session_active');

  // Überschreibt den Browser-Cache mit einem ungültigen AJAX-Aufruf
  const xhr = new XMLHttpRequest();
  xhr.open("GET", window.location.protocol + "//logout:logout@" + window.location.host + "/api/admin/logs?clear=" + Date.now(), true);
  xhr.send();
  
  xhr.onreadystatechange = function () {
    if (xhr.readyState === 4) {
      // Keine Weiterleitung: Wir bleiben auf der Admin-Seite und erzwingen die Maske
      window.location.href = window.location.pathname + "?auth_clear=" + Date.now();
    }
  };
};

// Wird aufgerufen, sobald das Backend grünes Licht gibt
function setSessionAsValidated() {
  sessionStorage.setItem('admin_session_active', 'true');
  localStorage.setItem('admin_last_activity', Date.now());
}

// =========================================================================
// 🚀 DYNAMISCHES LADEN DER CHAT-LOGS & SESSION-VALIDIERUNG
// =========================================================================
async function loadChatLogs() {
  try {
    // Wir fügen den X-Admin-Session Header hinzu, damit das Go-Backend uns autorisiert
    const response = await fetch('/api/admin/logs', {
      headers: {
        'X-Admin-Session': sessionStorage.getItem('admin_session_active') === 'true' ? 'active' : 'guest',
        'Cache-Control': 'no-cache'
      }
    });    
    // REPARIERT: Wenn das Backend 401 liefert, wird der Zugriff sofort verweigert
    if (response.status === 401) {
      localStorage.removeItem('admin_last_activity');
      sessionStorage.removeItem('admin_session_active');
      window.forceSessionLogout(true);
      return;
    }

    if (!response.ok) {
      console.error('API-Fehler:', response.status);
      const tbody = document.getElementById('logTableBody');
      if (tbody) tbody.innerHTML = '<tr><td colspan="4" class="empty-list" style="color:var(--danger)">Loglar yüklenemedi (Sunucu Hatası: ' + response.status + ').</td></tr>';
      return;
    }
    
    // Daten erfolgreich geladen -> Login war korrekt, Session starten!
    setSessionAsValidated();
    resetInactivityTimer();

    const logs = await response.json();
    const tbody = document.getElementById('logTableBody');
    if (!tbody) return;
    tbody.innerHTML = '';

    // Absicherung: Falls Logs null, kein Array oder leer sind
    if (!logs || !Array.isArray(logs) || logs.length === 0) {
      tbody.innerHTML = '<tr><td colspan="4" class="empty-list">Henüz kaydedilmiş bir chat logu bulunmuyor.</td></tr>';
      return;
    }

    logs.forEach(log => {
      const tr = document.createElement('tr');
      
      const tdID = document.createElement('td');
      tdID.textContent = log.id || '-';
      
      const tdIP = document.createElement('td');
      tdIP.className = 'ip-cell';
      tdIP.textContent = log.user_ip || '-';
      
      const tdOriginal = document.createElement('td');
      tdOriginal.className = 'original-cell';
      tdOriginal.textContent = log.original_message || '-';
      
      const tdMasked = document.createElement('td');
      tdMasked.className = 'masked-cell';
      tdMasked.textContent = log.masked_message || '-';
      
      tr.appendChild(tdID);
      tr.appendChild(tdIP);
      tr.appendChild(tdOriginal);
      tr.appendChild(tdMasked);
      
      tbody.appendChild(tr);
    });
  } catch (error) {
    console.error('Logs yüklenirken hata oluştu:', error);
    const tbody = document.getElementById('logTableBody');
    if (tbody) {
      tbody.innerHTML = '<tr><td colspan="4" class="empty-list" style="color:var(--danger)">Loglar yüklenemedi (Bağlantı Hatası).</td></tr>';
    }
  }
}

// =========================================================================
// INITIALISIERUNG BEIM SEITENSTART
// =========================================================================
document.addEventListener("DOMContentLoaded", () => {
  checkPastTimeout();
  loadChatLogs(); // Lädt Daten und validiert zeitgleich die Session
  
  // Event-Listener zur Überwachung von Inaktivität
  document.addEventListener("mousemove", resetInactivityTimer);
  document.addEventListener("keypress", resetInactivityTimer);
  document.addEventListener("click", resetInactivityTimer);
  document.addEventListener("scroll", resetInactivityTimer);
});
