package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// 6. CHAT GATEWAY MIT DYNAMISCHEM RAG-PROJEKT-ABRUF & HISTORISCHEM USER-GEDÄCHTNIS
func apiChatHandler(w http.ResponseWriter, r *http.Request) {

	// Erlaube OPTIONS direkt vor der POST-Prüfung
	if r.Method == "OPTIONS" {
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	maskedMessage := maskPII(req.Message)
	userID := hashUserIP(r.RemoteAddr)

	// KORRIGIERT: 'userIP' durch 'userID' ersetzt, damit der Compiler nicht meckert
	_, _ = db.Exec("INSERT INTO chat_logs (user_ip, original_message, masked_message) VALUES (?, ?, ?)", userID, req.Message, maskedMessage)

	firmendatenBytes, err := os.ReadFile("firmendaten.txt")
	firmendaten := string(firmendatenBytes)
	if err != nil {
		firmendaten = "NEXTREKLAM kurumsal tabela imalatı ve iç mimarlık firmasıdır."
	}

	pRows, err := db.Query("SELECT title, desc, date FROM projects")
	var projektKontext string
	if err == nil {
		defer pRows.Close()
		projektKontext = "\n\nŞu an yayında olan güncel örnek projelerimiz:\n"
		for pRows.Next() {
			var pTitle, pDesc, pDate string
			if err := pRows.Scan(&pTitle, &pDesc, &pDate); err == nil {
				projektKontext += fmt.Sprintf("- %s (%s): %s\n", pTitle, pDate, pDesc)
			}
		}
	}

	currentTime := time.Now().Format("02.01.2006")

	var userHistorySummary string
	_ = db.QueryRow("SELECT summary FROM chat_summaries WHERE user_ip = ?", userID).Scan(&userHistorySummary)
	if userHistorySummary != "" {
		userHistorySummary = "\n[Kullanıcının Önceki Konuşma Özeti]: " + userHistorySummary
	}
	systemPrompt := "Sen NEXTREKLAM firmasının yapay zeka asistanısın. Müşterilere kibar, profesyonel ve yardımcı ol. " +
		"Bugünün güncel tarihi kesin olarak şudur: " + currentTime + ".\n" +
		"Soruları şu kurumsal bilgilere ve yaptığımız projelere göre cevapla:\n" +
		firmendaten + projektKontext + userHistorySummary

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	apiURL := "https://openrouter.ai/api/v1/chat/completions"

	// KORRIGIERT: Nur noch exakt verifizierte, aktive Free-Modelle
	models := []string{
		"nvidia/nemotron-3-nano-30b-a3b:free",   // Starkes Hauptmodell (braucht manchmal etwas Zeit)
		"meta-llama/llama-3-8b-instruct:free",   // Extrem stabile Llama 3 Alternative
		"mistralai/mistral-7b-instruct:free",    // Hochverfügbares Mistral-Modell
		"meta-llama/llama-3.2-3b-instruct:free", // Llama 3.2 (Bleibt als schneller Fallback drin)
		"google/gemma-2-9b-it:free",             // Gemma 2 9B Free Pool
	}

	botReply := "Şu an yanıt veremiyorum. Lütfen WhatsApp hattımızı kullanın."
	success := false

	for _, currentModel := range models {
		println(fmt.Sprintf("Versuche Modell: %s...", currentModel))

		bodyMap := map[string]interface{}{
			"model": currentModel,
			"messages": []map[string]string{
				{"role": "system", "content": systemPrompt},
				{"role": "user", "content": maskedMessage},
			},
			"temperature": 0.3,
		}

		jsonBody, _ := json.Marshal(bodyMap)
		proxyReq, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
		if err != nil {
			continue
		}

		proxyReq.Header.Set("Content-Type", "application/json")
		proxyReq.Header.Set("Authorization", "Bearer "+apiKey)
		proxyReq.Header.Set("HTTP-Referer", "https://nextreklam.com.tr")
		proxyReq.Header.Set("X-Title", "NEXTREKLAM")

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(proxyReq)
		if err != nil {
			println(fmt.Sprintf("Netzwerkfehler bei %s: %s", currentModel, err.Error()))
			continue
		}
		defer resp.Body.Close()

		// KORRIGIERT: Flexiblere Inhalts-Prüfung (erlaubt auch 'application/json; charset=utf-8')
		contentType := resp.Header.Get("Content-Type")
		if !strings.Contains(strings.ToLower(contentType), "application/json") {
			println(fmt.Sprintf("Modell %s lieferte HTML statt JSON. Content-Type war: %s", currentModel, contentType))

			// Falls wir die Fehlerseite analysieren wollen, drucken wir die ersten 200 Zeichen ins Terminal:
			buf := new(bytes.Buffer)
			_, _ = io.CopyN(buf, resp.Body, 200)
			println("Server-Antwort (Auszug):", buf.String())
			continue
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			continue
		}

		if _, hasError := result["error"]; hasError {
			println(fmt.Sprintf("API-Fehler bei %s: %v", currentModel, result["error"]))
			continue
		}

		if choices, ok := result["choices"].([]interface{}); ok && len(choices) > 0 {
			// REPARIERT: Index [0] hinzugefügt, um das erste Element der Liste korrekt zu konvertieren
			if firstChoice, ok := choices[0].(map[string]interface{}); ok {
				if msg, ok := firstChoice["message"].(map[string]interface{}); ok {
					if content, ok := msg["content"].(string); ok {
						botReply = content
						success = true
						println(fmt.Sprintf("✅ ERFOLG: Antwort von %s erhalten!", currentModel))
						go updateConversationSummary(userID, maskedMessage, botReply)
						break
					}
				}
			}
		}
	}

	if !success {
		println("❌ KRITISCH: Alle Modelle in der Kette sind fehlgeschlagen.")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ChatResponse{Response: botReply})
}
