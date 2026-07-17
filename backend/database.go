package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	// WICHTIG: Stellen Sie sicher, dass dieser Import exakt so auch in der main.go steht!
	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

var db *sql.DB

func initDB() {
	dbUrl := os.Getenv("TURSO_DATABASE_URL")
	dbToken := os.Getenv("TURSO_AUTH_TOKEN")

	if dbUrl != "" {
		// Wenn die URL mit "libsql://" beginnt, ersetzen wir es durch "https://"
		// Das zwingt das SDK, eine reine Online-Netzwerkverbindung aufzubauen.
		if strings.HasPrefix(dbUrl, "libsql://") {
			dbUrl = strings.Replace(dbUrl, "libsql://", "https://", 1)
		}

		// Token sicher an die URL anhängen
		dbUrl = fmt.Sprintf("%s?authToken=%s", dbUrl, dbToken)
	} else {
		// Lokaler Fallback für die Entwicklung auf dem PC
		dbUrl = "file:nxt.db" // KORRIGIERT: "file:" Präfix für lokale SQLite-Kompatibilität im Treiber
	}

	var err error
	// Verbindung mit dem offiziellen "libsql" Treiber öffnen
	db, err = sql.Open("libsql", dbUrl)
	if err != nil {
		log.Fatalf("Fehler beim Öffnen der Turso-DB: %v", err)
	}

	// 🔥 NEU: Connection Pooling einrichten (Verhindert Abstürze bei hoher Last)
	db.SetMaxOpenConns(10)                  // Maximal 10 gleichzeitige offene Verbindungen zu Turso
	db.SetMaxIdleConns(5)                   // Maximal 5 ungenutzte Verbindungen im Pool behalten
	db.SetConnMaxLifetime(30 * time.Minute) // Verbindungen nach 30 Minuten erneuern

	// 3. Verbindung testen
	err = db.Ping()
	if err != nil {
		log.Fatalf("Turso-Datenbank nicht erreichbar: %v", err)
	}

	fmt.Println("Erfolgreich mit Turso-Datenbank verbunden!")

	// 4. Tabellen-Strukturen absichern
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
}
