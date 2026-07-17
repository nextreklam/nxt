package main

import (
	"net/http"
	"os"
)

// setupRoutes registriert alle HTTP-Endpunkte für die Anwendung
func setupRoutes() {
	// 1. ABSOLUTE PFADSICHERHEIT FÜR DIE WIDGET.JS
	http.HandleFunc("/assets/widget.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if _, err := os.Stat("./static/widget.js"); err == nil {
			http.ServeFile(w, r, "./static/widget.js")
			return
		}
		http.ServeFile(w, r, "./backend/static/widget.js")
	})
	// 2. ABSOLUTE PFADSICHERHEIT FÜR CSS, IMAGES & JS AUF RENDER
	staticDir := "static"
	if _, err := os.Stat("./backend/static"); err == nil {
		staticDir = "./backend/static"
	} else if _, err := os.Stat("./static"); err != nil {
		// Falls im Container ein anderer Pfad genutzt wird, suchen wir dynamisch
		wd, _ := os.Getwd()
		println("Aktuelles Arbeitsverzeichnis auf Render:", wd)
	}

	// Schaltet den statischen Ordner weltweit frei (ohne BasicAuth-Blockade!)
	fs := http.FileServer(http.Dir(staticDir))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// 3. CHAT GATEWAY
	http.HandleFunc("/api/chat", corsGuard(apiChatHandler))

	// 4. ÖFFENTLICHE API- / HTML-ROUTEN
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/galeri", galleryHandler)
	http.HandleFunc("/sitemap.xml", sitemapHandler)

	// 5. ADMIN-BEREICH (Geschützt via BasicAuth)
	http.HandleFunc("/api/admin/logs", apiAdminLogsHandler)
	http.HandleFunc("/admin/", basicAuthWrapper(adminHandler))
	http.HandleFunc("/admin/delete", basicAuthWrapper(deleteHandler))
	http.HandleFunc("/admin/update", basicAuthWrapper(updateHandler))
	http.HandleFunc("/admin/delete-media", basicAuthWrapper(deleteMediaHandler))
	http.HandleFunc("/admin/prompt/get", basicAuthWrapper(getPromptHandler))
	http.HandleFunc("/admin/prompt/save", basicAuthWrapper(savePromptHandler))
}
