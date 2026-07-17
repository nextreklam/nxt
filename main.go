package main

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/tursodatabase/turso-go"
)

var db *sql.DB

// Datenstrukturen für Templates und APIs
type Project struct {
	ID         int64
	Folder     string
	Title      string
	Date       string
	Desc       string
	MainImg    string
	GalleryStr string
	GalleryArr []string
}

type ChatRequest struct {
	Message string `json:"message"`
}

type ChatResponse struct {
	Response string `json:"response"`
}

type ChatLog struct {
	ID              int    `json:"id"`
	UserIP          string `json:"user_ip"`
	OriginalMessage string `json:"original_message"`
	MaskedMessage   string `json:"masked_message"`
}

func initDB() {
	// 1. Umgebungsvariablen auslesen
	dbUrl := os.Getenv("TURSO_DATABASE_URL")
	dbToken := os.Getenv("TURSO_AUTH_TOKEN")

	// Lokaler Fallback für die Entwicklung auf deinem PC
	if dbUrl == "" {
		dbUrl = "file:local_chat.db"
	} else {
		// Für Turso muss die URL das korrekte Format mit dem Token haben
		dbUrl = fmt.Sprintf("%s?authToken=%s", dbUrl, dbToken)
	}

	// 2. Verbindung mit dem neuen "libsql" Treiber öffnen
	var err error
	db, err = sql.Open("libsql", dbUrl)
	if err != nil {
		log.Fatalf("Fehler beim Öffnen der Turso-DB: %v", err)
	}

	// 3. Verbindung testen
	err = db.Ping()
	if err != nil {
		log.Fatalf("Turso-Datenbank nicht erreichbar: %v", err)
	}

	fmt.Println("Erfolgreich mit Turso-Datenbank verbunden!")
}

func main() {
	// Lädt die .env Datei beim Serverstart
	if err := godotenv.Load(); err != nil {
		log.Println("Hinweis: Keine .env Datei gefunden, System-Umgebungsvariablen werden genutzt.")
	}

	// 1. SQLite Datenbank öffnen
	var err error
	db, err = sql.Open("sqlite3", "./nxt.db")
	if err != nil {
		log.Fatal("Veritabanı bağlantı hatası:", err)
	}

	// 2. Tabellen für Projekte, Chat-Logs und Zusammenfassungen einzeln anlegen
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS projects (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		folder TEXT,
		title TEXT,
		date TEXT,
		desc TEXT,
		main_img TEXT,
		gallery TEXT
	);`)
	if err != nil {
		log.Fatal("Projects tablosu oluşturma hatası:", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS chat_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_ip TEXT,
		original_message TEXT,
		masked_message TEXT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);`)
	if err != nil {
		log.Fatal("Chat_logs tablosu oluşturma hatası:", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS chat_summaries (
		user_ip TEXT PRIMARY KEY,
		summary TEXT,
		last_updated DATETIME DEFAULT CURRENT_TIMESTAMP
	);`)
	if err != nil {
		log.Fatal("Chat_summaries tablosu oluşturma hatası:", err)
	}

	// 3. Systemordner für Uploads und Styles garantieren
	_ = os.MkdirAll("./static/images", os.ModePerm)
	_ = os.MkdirAll("./static/css", os.ModePerm)
	_ = os.MkdirAll("./templates", os.ModePerm)
	// 1. Ordner außerhalb des öffentlichen Bereichs garantieren
	_ = os.MkdirAll("./backups", os.ModePerm)

	// 2. Alten Backup-Inhalt restlos löschen, um Platz zu sparen
	if files, err := os.ReadDir("./backups"); err == nil {
		for _, file := range files {
			// Verhindert das versehentliche Löschen von Unterordnern
			if !file.IsDir() {
				_ = os.Remove("./backups/" + file.Name())
			}
		}
		// log.Println("🗑️ Alte Backups erfolgreich bereinigt.")
	}

	// 3. Automatisches neues Backup beim Serverstart erstellen
	if _, err := os.Stat("nxt.db"); err == nil {
		input, _ := os.ReadFile("nxt.db")
		backupName := fmt.Sprintf("./backups/nxt_backup_%s.db", time.Now().Format("2006-01-02_15-04"))
		_ = os.WriteFile(backupName, input, 0644)
		// log.Println("💾 Neues sicheres Datenbank-Backup erstellt:", backupName)
	}
	// Default-Firmendaten anlegen, falls Datei nicht existiert
	if _, err := os.Stat("firmendaten.txt"); os.IsNotExist(err) {
		defaultData := "NEXTREKLAM kurumsal tabela imalatı ve iç mimarlık firmasıdır.\nAdres: Akıncılar, Çizmeci Sokak No.1, Güngören, İstanbul."
		_ = os.WriteFile("firmendaten.txt", []byte(defaultData), 0644)
	}

	// 4. HTTP Routen registrieren (KORRIGIERT FÜR BASIC AUTH)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/galeri", galleryHandler)
	http.HandleFunc("/sitemap.xml", sitemapHandler)
	// Jetzt wieder sicher und sauber eingewickelt!
	http.HandleFunc("/api/chat", corsGuard(apiChatHandler))

	// Admin-Hauptroute MIT Schrägstrich am Ende registrieren
	http.HandleFunc("/admin/", basicAuthWrapper(adminHandler))

	// Unterrouten (Die Middleware bleibt zur Sicherheit überall aktiv)
	http.HandleFunc("/admin/delete", basicAuthWrapper(deleteHandler))
	http.HandleFunc("/admin/update", basicAuthWrapper(updateHandler))
	http.HandleFunc("/admin/delete-media", basicAuthWrapper(deleteMediaHandler)) // KORRIGIERT: Middleware hinzugefügt!

	// API & Prompt Routen
	http.HandleFunc("/api/admin/logs", basicAuthWrapper(apiAdminLogsHandler))
	http.HandleFunc("/admin/prompt/get", basicAuthWrapper(getPromptHandler))
	http.HandleFunc("/admin/prompt/save", basicAuthWrapper(savePromptHandler))

	log.Println("🚀 NEXTREKLAM Server gestartet unter: http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// 1. LANDINGPAGE HANDLER
func homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	tmpl, err := template.ParseFiles("templates/index.html")
	if err != nil {
		http.Error(w, "Ana sayfa şablonu bulunamadı.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.Execute(w, nil)
}

// 2. GALERIE HANDLER (Dynamischer DB-Abruf & Modals mit Video-Weiche)
func galleryHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, folder, title, date, desc, main_img, gallery FROM projects ORDER BY id DESC")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		rows.Scan(&p.ID, &p.Folder, &p.Title, &p.Date, &p.Desc, &p.MainImg, &p.GalleryStr)
		if p.GalleryStr != "" {
			p.GalleryArr = strings.Split(p.GalleryStr, ",")
		}
		projects = append(projects, p)
	}

	funcMap := template.FuncMap{
		"jsonHasVideoExtension": func(path string) bool {
			ext := strings.ToLower(filepath.Ext(path))
			return ext == ".mp4" || ext == ".webm" || ext == ".ogg" || ext == ".mov"
		},
	}

	tmpl, err := template.New("galeri.html").Funcs(funcMap).ParseFiles("templates/galeri.html")
	if err != nil {
		http.Error(w, "Galeri şablonu bulunamadı.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.Execute(w, projects)
}

// 3. ADMIN DASHBOARD HANDLER (Uploads & Verwaltung)
func adminHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if err := r.ParseMultipartForm(100 << 20); err != nil { // 100MB Max für Videos
			http.Error(w, "Dosya boyutu çok büyük!", http.StatusBadRequest)
			return
		}

		folder := r.FormValue("folder")
		title := r.FormValue("title")
		date := r.FormValue("date")
		desc := r.FormValue("desc")

		// Hauptbild verarbeiten und im Unterordner ablegen
		var mainImgPath string
		file, header, err := r.FormFile("mainImage")
		if err == nil {
			defer file.Close()

			// ZIELORDNER ERSTELLEN: Z.B. static/images/villa
			projectFolder := filepath.Join("static", "images", folder)
			_ = os.MkdirAll(projectFolder, os.ModePerm) // Erstellt den Unterordner, falls er fehlt

			ext := filepath.Ext(header.Filename)
			fileName := fmt.Sprintf("%s_main_%d%s", folder, time.Now().UnixNano()%1000, ext)
			targetPath := filepath.Join(projectFolder, fileName) // In den Unterordner schreiben

			out, err := os.Create(targetPath)
			if err == nil {
				defer out.Close()
				_, _ = io.Copy(out, file)
				mainImgPath = "/static/images/" + folder + "/" + fileName
			}
		}

		// Mehrfach-Galerie verarbeiten und im Unterordner ablegen
		var galleryPaths []string
		files := r.MultipartForm.File["galleryMedia"]
		for i, fHeader := range files {
			f, err := fHeader.Open()
			if err != nil {
				continue
			}
			defer f.Close()

			projectFolder := filepath.Join("static", "images", folder)
			_ = os.MkdirAll(projectFolder, os.ModePerm)

			ext := filepath.Ext(fHeader.Filename)
			fileNameClean := fmt.Sprintf("%s_gal_%d_%d%s", folder, i, time.Now().UnixNano()%1000, ext)
			targetPath := filepath.Join(projectFolder, fileNameClean) // In den Unterordner schreiben

			out, err := os.Create(targetPath)
			if err == nil {
				defer out.Close()
				_, _ = io.Copy(out, f)
				galleryPaths = append(galleryPaths, "/static/images/"+folder+"/"+fileNameClean)
			}
		}
		gallerySerialized := strings.Join(galleryPaths, ",")

		// In DB speichern (Sicherstellen, dass alle 6 Werte eingetragen werden)
		_, err = db.Exec("INSERT INTO projects (folder, title, date, desc, main_img, gallery) VALUES (?, ?, ?, ?, ?, ?)",
			folder, title, date, desc, mainImgPath, gallerySerialized)
		if err != nil {
			http.Error(w, "Veritabanı kayıt hatası", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}

	// GET: Liste für das Dashboard laden
	// KORRIGIERT: Reihenfolge der Spalten exakt auf die Struktur abgestimmt
	rows, err := db.Query("SELECT id, folder, title, date, desc, main_img, gallery FROM projects ORDER BY id DESC")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		// KORRIGIERT: 7 Zeiger für 7 Spalten – verhindert leere Pfade oder Abstürze
		err := rows.Scan(&p.ID, &p.Folder, &p.Title, &p.Date, &p.Desc, &p.MainImg, &p.GalleryStr)
		if err != nil {
			log.Println("Dashboard Scan Hatası:", err.Error())
			continue
		}
		projects = append(projects, p)
	}

	tmpl, err := template.ParseFiles("templates/admin.html")
	if err != nil {
		http.Error(w, "Admin şablonu bulunamadı.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.Execute(w, projects)
}

// 4. DELETE PROJECT HANDLER
func deleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Geçersiz işlem", http.StatusMethodNotAllowed)
		return
	}
	id := r.FormValue("id")
	_, err := db.Exec("DELETE FROM projects WHERE id = ?", id)
	if err != nil {
		http.Error(w, "Silme işlemi başarısız", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// 4a. UPDATE PROJECT HANDLER (Texte aktualisieren + ALTES HAUPTBILD LÖSCHEN BEI NEUEM UPLOAD)
func updateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Geçersiz işlem", http.StatusMethodNotAllowed)
		return
	}

	id := r.FormValue("id")
	folder := r.FormValue("folder")
	title := r.FormValue("title")
	date := r.FormValue("date")
	desc := r.FormValue("desc")

	// 1. Bestehende Galerie- und Hauptbild-Pfade aus der DB holen
	var existingGallery, existingMainImg string
	_ = db.QueryRow("SELECT main_img, gallery FROM projects WHERE id = ?", id).Scan(&existingMainImg, &existingGallery)

	_ = r.ParseMultipartForm(100 << 20)

	// 2. KORRIGIERT: Altes Bild löschen und durch neues ersetzen
	finalMainImg := existingMainImg
	mainFile, mainHeader, err := r.FormFile("mainImage")
	if err == nil {
		defer mainFile.Close()
		mainExt := filepath.Ext(mainHeader.Filename)
		mainFileName := fmt.Sprintf("%s_main_%d%s", folder, time.Now().UnixNano()%1000, mainExt)
		mainTargetPath := filepath.Join("static", "images", mainFileName)

		mainOut, err := os.Create(mainTargetPath)
		if err == nil {
			defer mainOut.Close()
			_, _ = io.Copy(mainOut, mainFile)
			finalMainImg = "/static/images/" + mainFileName

			// KORRIGIERT: Löscht das alte Hauptbild physisch von der Festplatte
			if existingMainImg != "" {
				oldLocalPath := strings.TrimPrefix(existingMainImg, "/")
				_ = os.Remove(oldLocalPath)
			}
		}
	}

	// 3. Neue Galerie-Bilder verarbeiten, falls hochgeladen
	var newGalleryPaths []string
	files := r.MultipartForm.File["galleryMedia"]
	for i, fHeader := range files {
		f, err := fHeader.Open()
		if err != nil {
			continue
		}
		defer f.Close()

		ext := filepath.Ext(fHeader.Filename)
		fileNameClean := fmt.Sprintf("%s_gal_%d_%d%s", folder, i, time.Now().UnixNano()%1000, ext)
		targetPath := filepath.Join("static", "images", fileNameClean)

		out, err := os.Create(targetPath)
		if err == nil {
			defer out.Close()
			_, _ = io.Copy(out, f)
			newGalleryPaths = append(newGalleryPaths, "/static/images/"+fileNameClean)
		}
	}

	// 4. Galeriepfade mergen
	finalGallery := existingGallery
	if len(newGalleryPaths) > 0 {
		newSerialized := strings.Join(newGalleryPaths, ",")
		if finalGallery != "" {
			finalGallery += "," + newSerialized
		} else {
			finalGallery = newSerialized
		}
	}

	// 5. In DB speichern
	_, err = db.Exec("UPDATE projects SET folder = ?, title = ?, date = ?, desc = ?, main_img = ?, gallery = ? WHERE id = ?",
		folder, title, date, desc, finalMainImg, finalGallery, id)
	if err != nil {
		http.Error(w, "Güncelleme hatası", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// 4d. ENZELNES BILD (GALERIE ODER HAUPTBILD) AUS DB LÖSCHEN (AJAX)
func deleteMediaHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Geçersiz işlem", http.StatusMethodNotAllowed)
		return
	}

	id := r.FormValue("id")
	mediaPath := r.FormValue("path")
	targetType := r.FormValue("type") // Nimmt entweder "main" oder "gallery" entgegen

	if targetType == "main" {
		// Hauptbild in der DB leeren
		_, err := db.Exec("UPDATE projects SET main_img = '' WHERE id = ?", id)
		if err != nil {
			http.Error(w, "Ana görsel silme hatası", http.StatusInternalServerError)
			return
		}
	} else {
		// Galerie-Bild aus dem kommagetrennten String filtern
		var galleryStr string
		err := db.QueryRow("SELECT gallery FROM projects WHERE id = ?", id).Scan(&galleryStr)
		if err != nil {
			http.Error(w, "Proje bulunamadı", http.StatusNotFound)
			return
		}

		paths := strings.Split(galleryStr, ",")
		var updatedPaths []string
		for _, p := range paths {
			if p != mediaPath && p != "" {
				updatedPaths = append(updatedPaths, p)
			}
		}
		newGalleryStr := strings.Join(updatedPaths, ",")

		_, err = db.Exec("UPDATE projects SET gallery = ? WHERE id = ?", newGalleryStr, id)
		if err != nil {
			http.Error(w, "Galeri güncelleme hatası", http.StatusInternalServerError)
			return
		}
	}

	// Datei physisch von der Festplatte löschen (Verhindert Datenmüll im static-Ordner)
	if mediaPath != "" {
		localPath := strings.TrimPrefix(mediaPath, "/")
		_ = os.Remove(localPath)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// 5. API HANDLER FÜR CHAT-LOGS (Reduziert auf die letzten 50 Einträge)
func apiAdminLogsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rows, err := db.Query("SELECT id, user_ip, original_message, masked_message FROM chat_logs ORDER BY id DESC LIMIT 50")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var logs []ChatLog
	for rows.Next() {
		var log ChatLog
		if err := rows.Scan(&log.ID, &log.UserIP, &log.OriginalMessage, &log.MaskedMessage); err != nil {
			continue
		}
		logs = append(logs, log)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

// 6. CHAT GATEWAY MIT DYNAMISCHEM RAG-PROJEKT-ABRUF & HISTORISCHEM USER-GEDÄCHTNIS
func apiChatHandler(w http.ResponseWriter, r *http.Request) {
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

	var userHistorySummary string
	_ = db.QueryRow("SELECT summary FROM chat_summaries WHERE user_ip = ?", userID).Scan(&userHistorySummary)
	if userHistorySummary != "" {
		userHistorySummary = "\n[Kullanıcının Önceki Konuşma Özeti]: " + userHistorySummary
	}
	systemPrompt := "Sen NEXTREKLAM firmasının yapay zeka asistanısın. Müşterilere kibar, profesyonel ve yardımcı ol. Soruları şu kurumsal bilgilere ve yaptığımız projelere göre cevapla:\n" + firmendaten + projektKontext + userHistorySummary

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

		// 1. Erlaubte Domains definieren
		allowedOrigins := map[string]bool{
			"http://localhost:8080":     true,
			"https://nextreklam.com.tr": true,
			"http://127.0.0.1:8080":     true, // Fallback für lokales Testen
		}

		// Falls die Anfrage von deiner Render-Domain kommt, erlauben wir sie dynamisch
		// (Ersetzt das spätere manuelle Eintragen der exakten Render-URL)
		if origin != "" && (allowedOrigins[origin] || strings.HasSuffix(origin, ".onrender.com")) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Credentials", "true") // Wichtig für Basic Auth Sessions
		}

		// 2. Den OPTIONS Preflight-Check des Browsers sofort abfangen
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}
