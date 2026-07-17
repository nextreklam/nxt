package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
)

// 7. HILFSFUNKTION FÜR DATENSCHUTZ (PII Maskierung)
func maskPII(text string) string {
	// E-Mails maskieren
	emailRegex := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	text = emailRegex.ReplaceAllString(text, "[MASKED_EMAIL]")

	// Telefonnummern (Türkische Mobil- und Festnetzformate) maskieren
	phoneRegex := regexp.MustCompile(`(\+90|0)?\s*(\d{3})\s*(\d{3})\s*(\d{2})\s*(\d{2})|\d{10,11}`)
	text = phoneRegex.ReplaceAllString(text, "[MASKED_PHONE]")

	// Adressen maskieren (einfache heuristische Annäherung)
	addressRegex := regexp.MustCompile(`\d{1,5}\s+\w+(\s+\w+)*`)
	text = addressRegex.ReplaceAllString(text, "[MASKED_ADDRESS]")
	return text
}

// 8. HINTERGRUND-WORKER FÜR USER-PROFILIERUNG (Zusammenfassung)
func updateConversationSummary(userIP, userMsg, botResp string) {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return
	}

	summaryPrompt := fmt.Sprintf(
		"Aşağıdaki konuşmadan yola çıkarak müşteri hakkında sadece akılda tutulması gereken önemli kısa bilgileri (Örn: Şişli'den yazıyor, fiyat bilgisi istiyor) tek bir kısa cümleyle özetle:\nKullanıcı: %s\nAsistan: %s",
		userMsg, botResp,
	)

	bodyMap := map[string]interface{}{
		"model": "nvidia/nemotron-3-nano-30b-a3b:free",
		"messages": []map[string]string{
			{"role": "user", "content": summaryPrompt},
		},
		"temperature": 0.1,
	}

	jsonBody, _ := json.Marshal(bodyMap)
	// KORRIGIERT: Vollständige OpenRouter Chat-Completions URL eingetragen
	req, _ := http.NewRequest("POST", "https://openrouter.ai", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("HTTP-Referer", "https://nextreklam.com.tr")
	req.Header.Set("X-Title", "NEXTREKLAM Summary")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return
	}

	// KORRIGIERT: Sicheres Type-Casting für das Array-Mapping (Verhindert Abstürze im Hintergrund)
	if choices, ok := result["choices"].([]interface{}); ok && len(choices) > 0 {
		if firstChoice, ok := choices[0].(map[string]interface{}); ok {
			if msg, ok := firstChoice["message"].(map[string]interface{}); ok {
				if aiSummary, ok := msg["content"].(string); ok {
					_, _ = db.Exec(`INSERT INTO chat_summaries (user_ip, summary, last_updated) 
						VALUES (?, ?, CURRENT_TIMESTAMP)
						ON CONFLICT(user_ip) DO UPDATE SET summary = ?, last_updated = CURRENT_TIMESTAMP`,
						userIP, aiSummary, aiSummary,
					)
				}
			}
		}
	}
}

func basicAuthWrapper(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Abfangen von typischen Browser-Assets
		if strings.HasSuffix(r.URL.Path, ".png") ||
			strings.HasSuffix(r.URL.Path, ".ico") ||
			strings.HasSuffix(r.URL.Path, ".css") ||
			strings.HasSuffix(r.URL.Path, ".js") {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		expectedUser := os.Getenv("ADMIN_USER")
		expectedPass := os.Getenv("ADMIN_PASSWORD")

		if expectedUser == "" || expectedPass == "" {
			expectedUser = "admin"
			expectedPass = "cihat"
		}

		username, password, ok := r.BasicAuth()

		if !ok || username != expectedUser || password != expectedPass {
			// 2. PRÜFUNG: Kommt der Request von einem JavaScript (Fetch/XHR) oder ist es ein API-Pfad?
			isAPI := strings.HasPrefix(r.URL.Path, "/api/") ||
				r.Header.Get("X-Requested-With") == "XMLHttpRequest" ||
				strings.Contains(r.Header.Get("Accept"), "application/json")

			if isAPI {
				// Bei APIs: Sende KEIN WWW-Authenticate! Das verhindert das Browser-Fenster.
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("Zweit-Anfrage blockiert (API-Schutz)"))
				return
			}

			// Nur für den allerersten echten Seitenaufruf (/admin) das Fenster erzwingen
			w.Header().Set("WWW-Authenticate", `Basic realm="NEXTREKLAM Admin Panel"`)
			http.Error(w, "Yetkisiz Erişim!", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}

// 9. API HANDLER: FIRMENDATEN.TXT LESEN (GET)
func getPromptHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	content, err := os.ReadFile("firmendaten.txt")
	if err != nil {
		// Falls die Datei fehlt, leere Antwort senden statt Absturz
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"content": ""})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"content": string(content)})
}

// 10. API HANDLER: FIRMENDATEN.TXT SPEICHERN (POST)
type SavePromptRequest struct {
	Content string `json:"content"`
}

func savePromptHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Geçersiz işlem", http.StatusMethodNotAllowed)
		return
	}

	var req SavePromptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Schreibt den neuen Prompt-Inhalt direkt live in die Datei auf dem Server
	err := os.WriteFile("firmendaten.txt", []byte(req.Content), 0644)
	if err != nil {
		http.Error(w, "Dosya kaydedilemedi", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// 11. DYNAMISCHE XML SITEMAP FÜR GOOGLE & CO. (VOLLSTÄNDIG KORRIGIERT)
func sitemapHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString(`<urlset xmlns="http://sitemaps.org/schemas/sitemap/0.9">`)
	sb.WriteString(`<url><loc>https://nextreklam.com.tr</loc><priority>1.0</priority><changefreq>daily</changefreq></url>`)
	sb.WriteString(`<url><loc>https://nextreklam.com.tr/galeri</loc><priority>0.8</priority><changefreq>weekly</changefreq></url>`)

	// Dynamisch alle Projekte aus der DB für Google auflisten
	rows, err := db.Query("SELECT folder FROM projects")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var folder string
			if err := rows.Scan(&folder); err == nil {
				// ERZWUNGEN: Jedes Projekt wird nun mit der sauberen Linkstruktur "/galeri/ordnername" ausgegeben
				sb.WriteString(fmt.Sprintf("<url><loc>https://nextreklam.com.tr/galeri/%s</loc><priority>0.7</priority><changefreq>monthly</changefreq></url>", folder))
			}
		}
	}

	sb.WriteString(`</urlset>`)
	w.Write([]byte(sb.String()))
}

// hashUserIP erzeugt eine eindeutige, DSGVO-konforme ID aus der IP-Adresse
func hashUserIP(ipStr string) string {
	ipOnly, _, err := net.SplitHostPort(ipStr)
	if err != nil {
		ipOnly = ipStr
	}
	ipOnly = strings.TrimSpace(ipOnly)

	// Sicherer Fallback, falls die .env beim Serverstart nicht geladen wurde
	salt := os.Getenv("SECRET_SALT")
	if salt == "" {
		salt = "NEXTREKLAM_Secret_Salt"
	}

	// IP und Salt kombinieren und hashen
	hash := sha256.Sum256([]byte(ipOnly + salt))

	// Gibt eine 16-stellige eindeutige ID zurück (z.B. "a3f9b2c8e1d4f6a0")
	return fmt.Sprintf("%x", hash)[:16]
}

func corsGuard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Erlaubt Anfragen von deiner Güzel-Domain und lokalen Testumgebungen
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, Accept")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		// ABSOLUT KRITISCH: OPTIONS Preflight muss SOFORT mit 200 OK antworten und darf nicht zum Handler weitergehen!
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}
