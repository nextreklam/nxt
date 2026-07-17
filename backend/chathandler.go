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
	// KORRIGIERT: CORS Header für preflight requests und direkte Rückkehr
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.WriteHeader(http.StatusOK)
		return
	}

	// KORRIGIERT: Doppelte Abfrage entfernt
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

	_, _ = db.Exec("INSERT INTO chat_logs (user_ip, original_message, masked_message) VALUES (?, ?, ?)", userID, req.Message, maskedMessage)

	firmendatenBytes, err := os.ReadFile("firmendaten.txt")
	firmendaten := string(firmendatenBytes)
	if err != nil {
		firmendaten = "NEXTREKLAM kurumsal tabela imalatı ve iç mimarlık firmasıdır."
	}

	pRows, err := db.Query("SELECT title, desc, date FROM projects")
	var projektKontext string
	if err == nil {
		// KORRIGIERT: defer direkt nach der Fehlerprüfung platzieren, um Verbindung sicher zu schließen
		defer pRows.Close()
		projektKontext = "\n\nŞu an yayında olan güncel örnek projelerimiz:\n"
		for pRows.Next() {
			var pTitle, pDesc, pDate string
			if err := pRows.Scan(&pTitle, &pDesc, &pDate); err == nil {
				projektKontext += fmt.Sprintf("- %s (%s): %s\n", pTitle, pDate, pDesc)
			}
		}
		// Absicherung für unvorhergesehene Schleifenfehler während des Iterierens
		if err := pRows.Err(); err != nil {
			println("Fehler beim Iterieren der Projekte:", err.Error())
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

	// Aktualisierte Free-Modellkette
	models := []string{
		"nvidia/nemotron-3-nano-30b-a3b:free",
		"meta-llama/llama-3-8b-instruct:free",
		"mistralai/mistral-7b-instruct:free",
		"meta-llama/llama-3.2-3b-instruct:free",
		"google/gemma-2-9b-it:free",
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

		// KORRIGIERT: defer resp.Body.Close() liest nun den Body im Fehlerfall aus, ohne offene Sockets zu hinterlassen
		contentType := resp.Header.Get("Content-Type")
		if !strings.Contains(strings.ToLower(contentType), "application/json") {
			println(fmt.Sprintf("Modell %s lieferte HTML statt JSON. Content-Type war: %s", currentModel, contentType))

			buf := new(bytes.Buffer)
			_, _ = io.CopyN(buf, resp.Body, 200)
			println("Server-Antwort (Auszug):", buf.String())
			resp.Body.Close()
			continue
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		if _, hasError := result["error"]; hasError {
			println(fmt.Sprintf("API-Fehler bei %s: %v", currentModel, result["error"]))
			continue
		}

		if choices, ok := result["choices"].([]interface{}); ok && len(choices) > 0 {
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
	w.Header().Set("Access-Control-Allow-Origin", "*") // CORS-Absicherung für die Antwort
	json.NewEncoder(w).Encode(ChatResponse{Response: botReply})
}
