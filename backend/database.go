package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
)

var db *sql.DB

func initDB() {
	dbUrl := os.Getenv("TURSO_DATABASE_URL")
	dbToken := os.Getenv("TURSO_AUTH_TOKEN")

	if dbUrl != "" {
		// WICHTIG: Wenn die URL mit "libsql://" beginnt, ersetzen wir es durch "https://"
		// Das zwingt das neue SDK, eine reine Online-Netzwerkverbindung aufzubauen.
		if strings.HasPrefix(dbUrl, "libsql://") {
			dbUrl = strings.Replace(dbUrl, "libsql://", "https://", 1)
		}

		// Token wie gewohnt anhängen
		dbUrl = fmt.Sprintf("%s?authToken=%s", dbUrl, dbToken)
	} else {
		// Lokaler Fallback für deinen PC
		dbUrl = "nxt.db"
	}

	// 2. Verbindung mit dem "turso" Treiber öffnen
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
}
