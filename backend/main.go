package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

func main() {
	// Lädt die .env Datei beim Serverstart
	if err := godotenv.Load(); err != nil {
		log.Println("Hinweis: Keine .env Datei gefunden, System-Umgebungsvariablen werden genutzt.")
	}

	// 1. Turso-Datenbank mit Token korrekt öffnen
	initDB()

	// 2. Routen aus routes.go registrieren (Hier wird /static/ bereits registriert!)
	setupRoutes()

	// 3. Systemordner lokal im Render-Container garantieren
	_ = os.MkdirAll("./static/images", os.ModePerm)
	_ = os.MkdirAll("./static/css", os.ModePerm)
	_ = os.MkdirAll("./templates", os.ModePerm)

	// 4. Pfad zu den Firmendaten (main.go liegt im Ordner 'backend')
	if _, err := os.Stat("firmendaten.txt"); os.IsNotExist(err) {
		defaultData := "NEXTREKLAM kurumsal tabela imalatı ve iç mimarlık firmasıdır.\nAdres: Akıncılar, Çizmeci Sokak No.1, Güngören, İstanbul."
		_ = os.WriteFile("firmendaten.txt", []byte(defaultData), 0644)
	}

	// 5. Port von Render auslesen (Fallback auf 10000, falls lokal)
	port := os.Getenv("PORT")
	if port == "" {
		port = "10000"
	}

	fmt.Printf("Server startet auf Port %s...\n", port)

	// Startet den Server
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, nil))
}
