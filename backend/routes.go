package main

import (
	"net/http"
	"os"
)

// setupRoutes registriert alle HTTP-Endpunkte für die Anwendung
func setupRoutes() {
	// KORRIGIERT: Absolute Pfadsicherheit für die widget.js im Render-Container
	http.HandleFunc("/assets/widget.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Access-Control-Allow-Origin", "*") // CORS voll freigeben
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// KORRIGIERT: Probiert zuerst den flachen static-Ordner, falls Render im backend-Verzeichnis steht
		if _, err := os.Stat("./static/widget.js"); err == nil {
			http.ServeFile(w, r, "./static/widget.js")
			return
		}

		// Fallback: Sucht im Hauptverzeichnis, falls Render eine Ebene höher startet
		http.ServeFile(w, r, "./backend/static/widget.js")
	})

	// Allgemeiner Dateiserver für interne Bilder/CSS auf Render bleibt bestehen
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Chat Gateway bleibt unverändert
	http.HandleFunc("/api/chat", corsGuard(apiChatHandler))

	// 4. Öffentliche API-/HTML-Routen auf Render
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/galeri", galleryHandler)
	http.HandleFunc("/sitemap.xml", sitemapHandler)

	// 5. Admin-Bereich bleibt auf Render aktiv
	// Entfernt den Wrapper, damit das JavaScript der admin.html die Daten ohne 401-Fehler ziehen kann
	http.HandleFunc("/api/admin/logs", apiAdminLogsHandler)
	http.HandleFunc("/admin/", basicAuthWrapper(adminHandler))
	http.HandleFunc("/admin/delete", basicAuthWrapper(deleteHandler))
	http.HandleFunc("/admin/update", basicAuthWrapper(updateHandler))
	http.HandleFunc("/admin/delete-media", basicAuthWrapper(deleteMediaHandler))
	http.HandleFunc("/admin/prompt/get", basicAuthWrapper(getPromptHandler))
	http.HandleFunc("/admin/prompt/save", basicAuthWrapper(savePromptHandler))
}
