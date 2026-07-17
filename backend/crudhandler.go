package main

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// 4. DELETE PROJECT HANDLER (Löscht Projekt aus DB und bereinigt Medien via FTP)
func deleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Geçersiz işlem", http.StatusMethodNotAllowed)
		return
	}
	id := r.FormValue("id")

	var mainImg, galleryStr string
	_ = db.QueryRow("SELECT main_img, gallery FROM projects WHERE id = ?", id).Scan(&mainImg, &galleryStr)

	_, err := db.Exec("DELETE FROM projects WHERE id = ?", id)
	if err != nil {
		http.Error(w, "Silme işlemi başarısız", http.StatusInternalServerError)
		return
	}

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

			go func() {
				_ = uploadToGuzelViaFTP(mainTargetPath, folder, mainFileName)
			}()

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
		// KORRIGIERT: Typ von *filepath.Header auf *multipart.FileHeader geändert
		func(index int, fh *multipart.FileHeader) {
			f, err := fh.Open()
			if err != nil {
				return
			}
			defer f.Close()

			ext := filepath.Ext(fh.Filename)
			fileNameClean := fmt.Sprintf("%s_gal_%d_%d%s", folder, index, time.Now().UnixNano()%1000, ext)
			targetPath := filepath.Join(projectFolder, fileNameClean)

			out, err := os.Create(targetPath)
			if err == nil {
				defer out.Close()
				_, _ = io.Copy(out, f)
				newGalleryPaths = append(newGalleryPaths, "/static/images/"+folder+"/"+fileNameClean)

				go func(p, fn string) {
					_ = uploadToGuzelViaFTP(p, folder, fn)
				}(targetPath, fileNameClean)
			}
		}(i, fHeader)
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

// 🔥 4d. NEU: EINZELNES MEDIENELEMENT LÖSCHEN (Wird von admin2.js aufgerufen)
func deleteMediaHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.FormValue("id")
	pathToRemove := r.FormValue("path")
	mediaType := r.FormValue("type") // "main" oder "gallery"

	var existingMainImg, existingGallery string
	err := db.QueryRow("SELECT main_img, gallery FROM projects WHERE id = ?", id).Scan(&existingMainImg, &existingGallery)
	if err != nil {
		http.Error(w, "Proje bulunamadı", http.StatusNotFound)
		return
	}

	baseDir := "static"
	if _, err := os.Stat("backend"); err == nil {
		baseDir = "backend/static"
	}

	if mediaType == "main" {
		_, err = db.Exec("UPDATE projects SET main_img = '' WHERE id = ?", id)
	} else {
		paths := strings.Split(existingGallery, ",")
		var updatedPaths []string
		for _, p := range paths {
			if p != pathToRemove && p != "" {
				updatedPaths = append(updatedPaths, p)
			}
		}
		newGalleryStr := strings.Join(updatedPaths, ",")
		_, err = db.Exec("UPDATE projects SET gallery = ? WHERE id = ?", newGalleryStr, id)
	}

	if err != nil {
		http.Error(w, "Veritabanı güncelleme hatası", http.StatusInternalServerError)
		return
	}

	go func() {
		_ = deleteFromGuzelViaFTP(pathToRemove)
		localPath := filepath.Join(baseDir, "images", strings.TrimPrefix(pathToRemove, "/static/images/"))
		_ = os.Remove(localPath)
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"success": true}`))
}
