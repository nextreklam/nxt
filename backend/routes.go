package main

import (
	"net/http"
)

// setupRoutes registriert alle HTTP-Endpunkte für die Anwendung
func setupRoutes() {
	// 1. Spezifische Route für widget.js ZUERST registrieren (mit CORS-Schutz gewrapped)
	http.HandleFunc("/static/widget.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Access-Control-Allow-Origin", "*") // Erlaubt Güzel das Laden der Datei
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		http.ServeFile(w, r, "./static/widget.js")
	})

	// 2. Allgemeiner Dateiserver für alle anderen statischen Assets (Bilder, CSS)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// 3. KRITISCH: CORS-geschütztes Chat Gateway (Völlig korrekt so, stört das Widget nicht!)
	// Das Widget lädt sich selbst über die obige Route und sendet seine Daten dann hierhin.
	http.HandleFunc("/api/chat", corsGuard(apiChatHandler))

	// 4. Öffentliche API-/HTML-Routen auf Render
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/galeri", galleryHandler)
	http.HandleFunc("/sitemap.xml", sitemapHandler)

	// 5. Admin-Bereich bleibt auf Render aktiv
	http.HandleFunc("/admin/", basicAuthWrapper(adminHandler))
	http.HandleFunc("/admin/delete", basicAuthWrapper(deleteHandler))
	http.HandleFunc("/admin/update", basicAuthWrapper(updateHandler))
	http.HandleFunc("/admin/delete-media", basicAuthWrapper(deleteMediaHandler))
	http.HandleFunc("/api/admin/logs", basicAuthWrapper(apiAdminLogsHandler))
	http.HandleFunc("/admin/prompt/get", basicAuthWrapper(getPromptHandler))
	http.HandleFunc("/admin/prompt/save", basicAuthWrapper(savePromptHandler))
}
