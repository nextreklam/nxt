package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// Hilfsfunktion zur Ermittlung des korrekten Frontend-Pfads auf Render
func getFrontendTemplatePath(fileName string) string {
	// Weiche 1: Wenn wir uns im 'backend' Ordner befinden und 'frontend' parallel liegt
	path := filepath.Join("../frontend/templates", fileName)
	if _, err := os.Stat(path); err == nil {
		return path
	}
	// Weiche 2: Wenn Render auf der Hauptebene des Monorepos steht
	path = filepath.Join("frontend/templates", fileName)
	if _, err := os.Stat(path); err == nil {
		return path
	}
	// Fallback: Direkt im aktuellen Arbeitsverzeichnis suchen
	return filepath.Join("templates", fileName)
}

// 1. LANDINGPAGE HANDLER
func homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// KORRIGIERT: Dynamischer Pfad statt hartcodiertem "../frontend/..."
	tmplPath := getFrontendTemplatePath("index.html")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		http.Error(w, "Ana sayfa şablonu bulunamadı. Hata: "+err.Error(), http.StatusInternalServerError)
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
		// KORRIGIERT: Fehlerprüfung für rows.Scan hinzugefügt, um Abstürze bei korrupten DB-Zeilen zu verhindern
		err := rows.Scan(&p.ID, &p.Folder, &p.Title, &p.Date, &p.Desc, &p.MainImg, &p.GalleryStr)
		if err != nil {
			log.Println("Galeri Veri Tarama Hatası:", err.Error())
			continue
		}
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

	// KORRIGIERT: Dynamischer Pfad für das Galerie-Template
	tmplPath := getFrontendTemplatePath("galeri.html")
	tmpl, err := template.New("galeri.html").Funcs(funcMap).ParseFiles(tmplPath)
	if err != nil {
		http.Error(w, "Galeri şablonu bulunamadı. Hata: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.Execute(w, projects)
}
