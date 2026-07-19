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

// Hilfsfunktion: Wechselt Schritt für Schritt in Ordner und erstellt sie, falls sie fehlen
func mkdirAllFTP(client *ftp.ServerConn, basePath string, targetFolder string) error {
	// 1. Zuerst ins Hauptverzeichnis wechseln, um eine saubere Ausgangslage zu haben
	_ = client.ChangeDir("/")

	// 2. Den bestehenden Basispfad (public_html/static/images) in Stufen betreten
	baseParts := strings.Split(basePath, "/")
	for _, part := range baseParts {
		if part == "" {
			continue
		}
		err := client.ChangeDir(part)
		if err != nil {
			// Falls der Basisordner wider Erwarten fehlt, erstellen und betreten
			_ = client.MakeDir(part)
			_ = client.ChangeDir(part)
		}
	}

	// 3. Jetzt sind wir im Ordner "public_html/static/images".
	// Hier erstellen wir den spezifischen Projektordner (z.B. "ahade")
	_ = client.MakeDir(targetFolder)

	// 4. In den neu erstellten Projektordner hineinwechseln
	err := client.ChangeDir(targetFolder)
	if err != nil {
		return fmt.Errorf("Konnte nicht in den Zielordner wechseln: %v", err)
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

	// Verbindung mit sicherem Firewall-Timeout aufbauen
	client, err := ftp.Dial(
		ftpHost,
		ftp.DialWithTimeout(15*time.Second),
		ftp.DialWithDisabledEPSV(true), // Zwingt den Datenkanal in den stabilen PASV-Modus
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

	// 🔥 KORREKTUR: Fehlende Konstante entfernt, da die Library standardmäßig Binär nutzt.
	// Setzt die Sitzung vor dem Ordnerbau auf das Root-Verzeichnis zurück.
	_ = client.ChangeDir("/")

	// Ordnerstruktur sicher aufbauen und betreten
	basePath := "public_html/static/images"
	err = mkdirAllFTP(client, basePath, remoteFolder)
	if err != nil {
		return fmt.Errorf("FTP-Ordnerverwaltung fehlgeschlagen: %v", err)
	}

	// Öffnen der lokalen Bilddatei auf Render
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("Lokale Datei konnte nicht geoeffnet werden: %v", err)
	}
	defer localFile.Close()

	// Lädt das Bild direkt mit dem reinen Dateinamen hoch
	err = client.Stor(fileName, localFile)
	if err != nil {
		return fmt.Errorf("FTP Upload Fehler beim Speichern der Datei: %v", err)
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

	_ = client.ChangeDir("/")

	// 1. Pfad radikal von Präfixen befreien (z.B. "static/images/ordner/datei.jpg")
	cleanPath := strings.TrimPrefix(remotePath, "/")
	cleanPath = strings.TrimPrefix(cleanPath, "public_html/")
	cleanPath = strings.TrimPrefix(cleanPath, "/")

	// 2. Datei-Name und Verzeichnis-Pfad trennen
	remoteFilePath := fmt.Sprintf("public_html/%s", cleanPath)

	// Suchen, wo der Dateiname beginnt (letzter Schrägstrich)
	lastSlash := strings.LastIndex(remoteFilePath, "/")

	if lastSlash != -1 {
		dirPath := remoteFilePath[:lastSlash]    // Alles bis zum Ordner
		fileName := remoteFilePath[lastSlash+1:] // Nur der reine Dateiname

		// Erst aktiv in den Ordner wechseln
		err = client.ChangeDir(dirPath)
		if err == nil {
			// Wenn der Wechsel klappt, direkt im Ordner löschen
			_ = client.Delete(fileName)
			return nil
		}
	}

	// Fallback
	_ = client.Delete(remoteFilePath)
	return nil
}
