package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
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
			projectFolder := filepath.Join("../frontend", "static", "images", folder)
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

			projectFolder := filepath.Join("../frontend", "static", "images", folder)
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

	tmpl, err := template.ParseFiles("../frontend/templates/admin.html")
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
		mainTargetPath := filepath.Join("../frontend", "static", "images", mainFileName)

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
