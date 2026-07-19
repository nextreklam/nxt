package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"
)

// ==========================================
// 1. FTP-HILFSFUNKTIONEN FÜR GÜZEL-SERVER
// ==========================================

// Hilfsfunktion: Erstellt Ordnerstrukturen Schritt für Schritt auf dem FTP-Server
func mkdirAllFTP(client *ftp.ServerConn, path string) error {
	// Ersetzt Backslashes für Linux-Kompatibilität auf Güzel
	cleanPath := strings.ReplaceAll(path, "\\", "/")
	parts := strings.Split(cleanPath, "/")

	currentPath := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		if currentPath == "" {
			currentPath = part
		} else {
			currentPath = currentPath + "/" + part
		}
		// Erstellt den Teilordner. Fehler (z.B. Ordner existiert bereits) werden ignoriert.
		_ = client.MakeDir(currentPath)
	}
	return nil
}

// Hilfsfunktion für den FTP-Transfer zu Güzel
func uploadToGuzelViaFTP(localPath, remoteFolder, fileName string) error {
	ftpHost := os.Getenv("FTP_HOST")
	ftpUser := os.Getenv("FTP_USER")
	ftpPass := os.Getenv("FTP_PASS")

	if ftpHost == "" || ftpUser == "" || ftpPass == "" {
		return fmt.Errorf("FTP-Zugangsdaten nicht konfiguriert")
	}

	// KORRIGIERT: Verbindung herstellen mit Timeout UND Deaktivierung von EPSV.
	// Das zwingt die Verbindung in den normalen Passiv-Modus (PASV), den DirectAdmin erwartet.
	client, err := ftp.Dial(
		ftpHost,
		ftp.DialWithTimeout(5*time.Second),
		ftp.DialWithDisabledEPSV(true),
	)
	if err != nil {
		return fmt.Errorf("FTP Dial Fehler: %v", err)
	}
	defer func() {
		_ = client.Quit()
	}()

	err = client.Login(ftpUser, ftpPass)
	if err != nil {
		return fmt.Errorf("FTP Login Fehler: %v", err)
	}

	// In das Hauptverzeichnis wechseln
	_ = client.ChangeDir("/")

	// Rekursive Ordnererstellung
	remoteBasePath := fmt.Sprintf("public_html/static/images/%s", remoteFolder)
	err = mkdirAllFTP(client, remoteBasePath)
	if err != nil {
		return fmt.Errorf("FTP-Ordnererstellung fehlgeschlagen: %v", err)
	}

	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("Lokale Datei konnte nicht geoeffnet werden: %v", err)
	}
	defer localFile.Close()

	remoteFilePath := fmt.Sprintf("%s/%s", remoteBasePath, fileName)
	err = client.Stor(remoteFilePath, localFile)
	if err != nil {
		return fmt.Errorf("FTP Upload Fehler auf Güzel: %v", err)
	}

	return nil
}

// Hilfsfunktion: Löscht eine Datei physisch via FTP auf dem Güzel-Server
func deleteFromGuzelViaFTP(remotePath string) error {
	ftpHost := os.Getenv("FTP_HOST")
	ftpUser := os.Getenv("FTP_USER")
	ftpPass := os.Getenv("FTP_PASS")

	if ftpHost == "" || ftpUser == "" || ftpPass == "" || remotePath == "" {
		return nil
	}

	// Verbindung herstellen mit sicherem Timeout und PASV-Erzwingung
	client, err := ftp.Dial(
		ftpHost,
		ftp.DialWithTimeout(15*time.Second),
		ftp.DialWithDisabledEPSV(true),
	)
	if err != nil {
		return err
	}
	defer func() {
		_ = client.Quit()
	}()

	err = client.Login(ftpUser, ftpPass)
	if err != nil {
		return err
	}

	// 1. Pfad radikal von Präfixen befreien (z.B. "static/images/ordner/datei.jpg")
	cleanPath := strings.TrimPrefix(remotePath, "/")
	cleanPath = strings.TrimPrefix(cleanPath, "public_html/")
	cleanPath = strings.TrimPrefix(cleanPath, "/")

	// 2. Datei-Name und Verzeichnis-Pfad trennen
	// Beispiel: remoteFilePath wird zu "public_html/static/images/ordner/datei.jpg"
	remoteFilePath := fmt.Sprintf("public_html/%s", cleanPath)

	// Suchen, wo der Dateiname beginnt (letzter Schrägstrich)
	lastSlash := strings.LastIndex(remoteFilePath, "/")

	if lastSlash != -1 {
		dirPath := remoteFilePath[:lastSlash]    // Alles bis zum Ordner
		fileName := remoteFilePath[lastSlash+1:] // Nur der reine Dateiname (z.B. "main.jpg")

		// 🔥 DIE RETTUNG: Erst aktiv in den Ordner wechseln
		err = client.ChangeDir(dirPath)
		if err == nil {
			// Wenn der Wechsel klappt, direkt im Ordner löschen
			_ = client.Delete(fileName)
			return nil
		}
	}

	// Fallback: Falls die Pfadtrennung scheitert, alten Löschversuch als Backup nutzen
	_ = client.Delete(remoteFilePath)
	return nil
}
