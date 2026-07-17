// --- AUTOMATISCHES TIMEOUT & SESSION-SCHUTZ LOGIK ---
const TIMEOUT_LIMIT = 10 * 60 * 1000; // 10 Minuten
let inactivityTimer;

// KORRIGIERT: Loop-Schutz beim ersten Session-Check
if (!sessionStorage.getItem('admin_session_active')) {
  sessionStorage.setItem('admin_session_active', 'true');
  localStorage.setItem('admin_last_activity', Date.now());
}

function resetInactivityTimer() {
  localStorage.setItem('admin_last_activity', Date.now());
  
  clearTimeout(inactivityTimer);
  inactivityTimer = setTimeout(() => {
    alert("10 dakika boyunca işlem yapılmadığı için oturumunuz güvenlik nedeniyle kapatılmıştır.");
    window.forceSessionLogout(true);
  }, TIMEOUT_LIMIT);
}

function checkPastTimeout() {
  const lastActivity = localStorage.getItem('admin_last_activity');
  if (lastActivity) {
    const timePassed = Date.now() - parseInt(lastActivity);
    if (timePassed > TIMEOUT_LIMIT) {
      window.forceSessionLogout(true);
    }
  }
}

// KORRIGIERT: Explizit an window gebunden, damit Inline-HTML-Events (onclick) darauf zugreifen können
window.forceSessionLogout = function(redirectToHome) {
  localStorage.removeItem('admin_last_activity');
  sessionStorage.removeItem('admin_session_active');

  const xhr = new XMLHttpRequest();
  xhr.open("GET", "/api/admin/logs", true, "invalid_user_logout", "clear_auth_cache_123");
  xhr.send();
  
  xhr.onreadystatechange = function () {
    if (xhr.readyState === 4) {
      if (redirectToHome) {
        window.location.href = "about:blank";
      } else {
        window.location.reload();
      }
    }
  };
};

async function loadChatLogs() {
  try {
    const response = await fetch('/api/admin/logs');
    if (!response.ok) {
      console.error('API-Fehler:', response.status);
      const tbody = document.getElementById('logTableBody');
      if (tbody) tbody.innerHTML = '<tr><td colspan="4" class="empty-list" style="color:var(--danger)">Loglar yüklenemedi (Sunucu Hatası: ' + response.status + ').</td></tr>';
      return;
    }
    
    const logs = await response.json();
    const tbody = document.getElementById('logTableBody');
    if (!tbody) return;
    tbody.innerHTML = '';

    // Absicherung: Falls logs null, kein Array oder leer ist
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

document.addEventListener("DOMContentLoaded", () => {
  checkPastTimeout();
  resetInactivityTimer();
  loadChatLogs();
  
  document.addEventListener("mousemove", resetInactivityTimer);
  document.addEventListener("keypress", resetInactivityTimer);
  document.addEventListener("click", resetInactivityTimer);
  document.addEventListener("scroll", resetInactivityTimer);
});
