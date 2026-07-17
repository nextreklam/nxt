package main

import (
	"encoding/json"
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

// 3. ADMIN DASHBOARD HANDLER (Uploads & Verwaltung mit FTP-Sync zu Güzel)
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

		// Ermittelt den dynamischen lokalen Zielordner auf Render
		baseDir := "static"
		if _, err := os.Stat("backend"); err == nil {
			baseDir = "backend/static"
		}
		projectFolder := filepath.Join(baseDir, "images", folder)
		_ = os.MkdirAll(projectFolder, os.ModePerm)

		// Hauptbild verarbeiten
		var mainImgPath string
		file, header, err := r.FormFile("mainImage")
		if err == nil {
			defer file.Close()

			ext := filepath.Ext(header.Filename)
			fileName := fmt.Sprintf("%s_main_%d%s", folder, time.Now().UnixNano()%1000, ext)
			targetPath := filepath.Join(projectFolder, fileName)

			out, err := os.Create(targetPath)
			if err == nil {
				defer out.Close()
				_, _ = io.Copy(out, file)
				mainImgPath = "/static/images/" + folder + "/" + fileName

				// Live-FTP-Upload zu Güzel im Hintergrund
				go func() {
					errUpload := uploadToGuzelViaFTP(targetPath, folder, fileName)
					if errUpload != nil {
						log.Println("Kritischer FTP-Fehler für Hauptbild:", errUpload)
					} else {
						log.Println("Erfolg: Hauptbild via FTP auf Güzel gespeichert!")
					}
				}()
			}
		}

		// Mehrfach-Galerie verarbeiten
		var galleryPaths []string
		files := r.MultipartForm.File["galleryMedia"]
		for i, fHeader := range files {
			f, err := fHeader.Open()
			if err != nil {
				continue
			}
			defer f.Close()

			ext := filepath.Ext(fHeader.Filename)
			fileNameClean := fmt.Sprintf("%s_gal_%d_%d%s", folder, i, time.Now().UnixNano()%1000, ext)
			targetPath := filepath.Join(projectFolder, fileNameClean)

			out, err := os.Create(targetPath)
			if err == nil {
				defer out.Close()
				_, _ = io.Copy(out, f)
				galleryPaths = append(galleryPaths, "/static/images/"+folder+"/"+fileNameClean)

				// Live-FTP-Upload zu Güzel im Hintergrund
				go func(p, fn string) {
					errUpload := uploadToGuzelViaFTP(p, folder, fn)
					if errUpload != nil {
						log.Println("Kritischer FTP-Fehler für Galeriebild:", errUpload)
					}
				}(targetPath, fileNameClean)
			}
		}
		gallerySerialized := strings.Join(galleryPaths, ",")

		// In DB speichern
		_, err = db.Exec("INSERT INTO projects (folder, title, date, desc, main_img, gallery) VALUES (?, ?, ?, ?, ?, ?)",
			folder, title, date, desc, mainImgPath, gallerySerialized)
		if err != nil {
			http.Error(w, "Veritabanı kayıt hatası", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/admin/", http.StatusSeeOther)
		return
	}

	// GET: Liste für das Dashboard laden
	rows, err := db.Query("SELECT id, folder, title, date, desc, main_img, gallery FROM projects ORDER BY id DESC")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		err := rows.Scan(&p.ID, &p.Folder, &p.Title, &p.Date, &p.Desc, &p.MainImg, &p.GalleryStr)
		if err != nil {
			log.Println("Dashboard Scan Hatası:", err.Error())
			continue
		}
		projects = append(projects, p)
	}

	// Flexible Pfad-Weiche für das HTML-Template
	var tmpl *template.Template
	if _, errStat := os.Stat("templates/admin.html"); errStat == nil {
		tmpl, err = template.ParseFiles("templates/admin.html")
	} else {
		tmpl, err = template.ParseFiles("backend/templates/admin.html")
	}

	if err != nil {
		http.Error(w, "Admin şablonu bulunamadı. Hata: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.Execute(w, projects)
}

// 4. DELETE PROJECT HANDLER (Löscht Projekt aus DB und bereinigt Medien via FTP)
func deleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Geçersiz işlem", http.StatusMethodNotAllowed)
		return
	}
	id := r.FormValue("id")

	// Medienpfade vor dem Löschen aus der DB holen
	var mainImg, galleryStr string
	_ = db.QueryRow("SELECT main_img, gallery FROM projects WHERE id = ?", id).Scan(&mainImg, &galleryStr)

	_, err := db.Exec("DELETE FROM projects WHERE id = ?", id)
	if err != nil {
		http.Error(w, "Silme işlemi başarısız", http.StatusInternalServerError)
		return
	}

	// Medien asynchron vom Güzel-Server via FTP löschen
	go func(main string, gal string) {
		if main != "" {
			_ = deleteFromGuzelViaFTP(main)
		}
		if gal != "" {
			paths := strings.Split(gal, ",")
			for _, p := range paths {
				if p != "" {
					_ = deleteFromGuzelViaFTP(p)
				}
			}
		}
	}(mainImg, galleryStr)

	http.Redirect(w, r, "/admin/", http.StatusSeeOther)
}

// 4a. UPDATE PROJECT HANDLER (Texte aktualisieren + ALTES BILD LÖSCHEN + FTP SYNC)
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

	var existingGallery, existingMainImg string
	_ = db.QueryRow("SELECT main_img, gallery FROM projects WHERE id = ?", id).Scan(&existingMainImg, &existingGallery)

	_ = r.ParseMultipartForm(100 << 20)

	baseDir := "static"
	if _, err := os.Stat("backend"); err == nil {
		baseDir = "backend/static"
	}
	projectFolder := filepath.Join(baseDir, "images", folder)
	_ = os.MkdirAll(projectFolder, os.ModePerm)

	finalMainImg := existingMainImg
	mainFile, mainHeader, err := r.FormFile("mainImage")
	if err == nil {
		defer mainFile.Close()
		mainExt := filepath.Ext(mainHeader.Filename)
		mainFileName := fmt.Sprintf("%s_main_%d%s", folder, time.Now().UnixNano()%1000, mainExt)
		mainTargetPath := filepath.Join(projectFolder, mainFileName)

		mainOut, err := os.Create(mainTargetPath)
		if err == nil {
			defer mainOut.Close()
			_, _ = io.Copy(mainOut, mainFile)
			finalMainImg = "/static/images/" + folder + "/" + mainFileName

			// Live-FTP-Upload für das neue Hauptbild zu Güzel
			go func() {
				_ = uploadToGuzelViaFTP(mainTargetPath, folder, mainFileName)
			}()

			// Altes Hauptbild von Güzel und lokal löschen
			if existingMainImg != "" {
				go func(oldPath string) {
					_ = deleteFromGuzelViaFTP(oldPath)
				}(existingMainImg)
				oldLocalPath := filepath.Join(baseDir, "images", strings.TrimPrefix(existingMainImg, "/static/images/"))
				_ = os.Remove(oldLocalPath)
			}
		}
	}

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
		targetPath := filepath.Join(projectFolder, fileNameClean)

		out, err := os.Create(targetPath)
		if err == nil {
			defer out.Close()
			_, _ = io.Copy(out, f)
			newGalleryPaths = append(newGalleryPaths, "/static/images/"+folder+"/"+fileNameClean)

			// Live-FTP-Upload für neue Galeriebilder zu Güzel
			go func(p, fn string) {
				_ = uploadToGuzelViaFTP(p, folder, fn)
			}(targetPath, fileNameClean)
		}
	}

	finalGallery := existingGallery
	if len(newGalleryPaths) > 0 {
		newSerialized := strings.Join(newGalleryPaths, ",")
		if finalGallery != "" {
			finalGallery += "," + newSerialized
		} else {
			finalGallery = newSerialized
		}
	}

	_, err = db.Exec("UPDATE projects SET folder = ?, title = ?, date = ?, desc = ?, main_img = ?, gallery = ? WHERE id = ?",
		folder, title, date, desc, finalMainImg, finalGallery, id)
	if err != nil {
		http.Error(w, "Güncelleme hatası", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/", http.StatusSeeOther)
}

// 4d. EINZELNES BILD AUS DB LÖSCHEN (AJAX + FTP SYNC)
func deleteMediaHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Geçersiz işlem", http.StatusMethodNotAllowed)
		return
	}

	id := r.FormValue("id")
	mediaPath := r.FormValue("path")
	targetType := r.FormValue("type")

	if targetType == "main" {
		_, err := db.Exec("UPDATE projects SET main_img = '' WHERE id = ?", id)
		if err != nil {
			http.Error(w, "Ana görsel silme hatası", http.StatusInternalServerError)
			return
		}
	} else {
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

	// Datei asynchron vom Güzel-Server via FTP löschen
	if mediaPath != "" {
		go func(path string) {
			_ = deleteFromGuzelViaFTP(path)
		}(mediaPath)

		baseDir := "static"
		if _, err := os.Stat("backend"); err == nil {
			baseDir = "backend/static"
		}
		oldLocalPath := filepath.Join(baseDir, "images", strings.TrimPrefix(mediaPath, "/static/images/"))
		_ = os.Remove(oldLocalPath)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// 5. API HANDLER FÜR CHAT-LOGS (Die letzten 50 Einträge)
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

// 6. API HANDLER: FIRMENDATEN.TXT LESEN (GET)
func getPromptHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	content, err := os.ReadFile("firmendaten.txt")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"content": ""})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"content": string(content)})
}
