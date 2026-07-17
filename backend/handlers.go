package main

import (
	"net/http"
	"path/filepath"
	"strings"
	"text/template"
)

// 1. LANDINGPAGE HANDLER
func homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	tmpl, err := template.ParseFiles("../frontend/templates/index.html")
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

	tmpl, err := template.New("galeri.html").Funcs(funcMap).ParseFiles("../frontend/templates/galeri.html")
	if err != nil {
		http.Error(w, "Galeri şablonu bulunamadı.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.Execute(w, projects)
}
