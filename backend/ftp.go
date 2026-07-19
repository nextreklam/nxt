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

	// KORRIGIERT: Erhöhtes Verbindungs-Timeout (15 Sekunden) für langsame Firewalls
	client, err := ftp.Dial(
		ftpHost,
		ftp.DialWithTimeout(15*time.Second),
		ftp.DialWithDisabledEPSV(true), // Verhindert Einfrieren des Datenkanals
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

	// Pfad sauber bereinigen
	cleanPath := strings.TrimPrefix(remotePath, "/")
	cleanPath = strings.TrimPrefix(cleanPath, "public_html/")
	cleanPath = strings.TrimPrefix(cleanPath, "/")

	remoteFilePath := fmt.Sprintf("public_html/%s", cleanPath)

	// Lösche die Datei auf Güzel (Fehler werden für flüssigen Ablauf ignoriert)
	_ = client.Delete(remoteFilePath)
	return nil
}
