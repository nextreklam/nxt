package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
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

		baseDir := "static"
		if _, err := os.Stat("backend"); err == nil {
			baseDir = "backend/static"
		}
		projectFolder := filepath.Join(baseDir, "images", folder)
		_ = os.MkdirAll(projectFolder, os.ModePerm)

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
				mainImgPath = "static/images/" + folder + "/" + fileName

				errUpload := uploadToGuzelViaFTP(targetPath, folder, fileName)
				if errUpload != nil {
					log.Println("Kritischer FTP-Fehler für Hauptbild:", errUpload)
				} else {
					log.Println("Erfolg: Hauptbild via FTP auf Güzel gespeichert!")
				}

			}
		}

		var galleryPaths []string
		files := r.MultipartForm.File["galleryMedia"]
		for i, fHeader := range files {
			// KORRIGIERT: Typ von *filepath.Header auf *multipart.FileHeader geändert
			func(index int, fh *multipart.FileHeader) {
				f, err := fh.Open()
				if err != nil {
					return
				}
				defer f.Close() // Schließt das Handle sofort nach diesem Durchlauf!

				ext := filepath.Ext(fh.Filename)
				fileNameClean := fmt.Sprintf("%s_gal_%d_%d%s", folder, index, time.Now().UnixNano()%1000, ext)
				targetPath := filepath.Join(projectFolder, fileNameClean)

				out, err := os.Create(targetPath)
				if err == nil {
					defer out.Close()
					_, _ = io.Copy(out, f)
					galleryPaths = append(galleryPaths, "static/images/"+folder+"/"+fileNameClean)

					errUpload := uploadToGuzelViaFTP(targetPath, folder, fileNameClean)
					if errUpload != nil {
						log.Println("Kritischer FTP-Fehler für Galeriebild:", errUpload)
					} else {
						log.Println("Erfolg: Galeriebild via FTP auf Güzel gespeichert!")
					}
				}
			}(i, fHeader)
		}
		gallerySerialized := strings.Join(galleryPaths, ",")

		_, err = db.Exec("INSERT INTO projects (folder, title, date, desc, main_img, gallery) VALUES (?, ?, ?, ?, ?, ?)",
			folder, title, date, desc, mainImgPath, gallerySerialized)
		if err != nil {
			http.Error(w, "Veritabanı kayıt hatası", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/admin/", http.StatusSeeOther)
		return
	}

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

// 5. API HANDLER FÜR CHAT-LOGS (Die letzten 50 Einträge mit CORS-Freigabe)
func apiAdminLogsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

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

	logs := make([]ChatLog, 0)

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
