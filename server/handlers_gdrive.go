package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/calexandrepcjr/cheapskate-finance-tracker/client/templates"
)

func (app *Application) HandleGDriveStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if app.GDrive == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled":   false,
			"connected": false,
		})
		return
	}

	status := app.GDrive.GetStatus()
	json.NewEncoder(w).Encode(status)
}

func (app *Application) HandleGDriveConnect(w http.ResponseWriter, r *http.Request) {
	if app.GDrive == nil {
		http.Error(w, "Google Drive not configured", http.StatusBadRequest)
		return
	}

	authURL := app.GDrive.GetAuthURL()
	if authURL == "" {
		http.Error(w, "Google Drive not configured", http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

func (app *Application) HandleGDriveCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		return
	}

	if app.GDrive == nil {
		http.Error(w, "Google Drive not configured", http.StatusBadRequest)
		return
	}

	if err := app.GDrive.HandleCallback(code); err != nil {
		log.Printf("Google Drive callback error: %v", err)
		http.Error(w, "Failed to complete authorization: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if app.GDrive.GetConfig() != nil {
		app.GDrive.GetConfig().Connected = true
	}

	http.Redirect(w, r, "/settings", http.StatusTemporaryRedirect)
}

func (app *Application) HandleGDriveDisconnect(w http.ResponseWriter, r *http.Request) {
	if app.GDrive == nil {
		http.Error(w, "Google Drive not configured", http.StatusBadRequest)
		return
	}

	if err := app.GDrive.Disconnect(); err != nil {
		log.Printf("Google Drive disconnect error: %v", err)
		http.Error(w, "Failed to disconnect: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

func (app *Application) HandleGDriveSync(w http.ResponseWriter, r *http.Request) {
	if app.GDrive == nil {
		http.Error(w, "Google Drive not configured", http.StatusBadRequest)
		return
	}

	if !app.GDrive.IsConnected() {
		http.Error(w, "Not connected to Google Drive", http.StatusBadRequest)
		return
	}

	tmpDB, err := os.CreateTemp("", "cheapskate-sync-*.db")
	if err != nil {
		http.Error(w, "Failed to create temp file", http.StatusInternalServerError)
		return
	}
	tmpDBPath := tmpDB.Name()
	tmpDB.Close()
	defer os.Remove(tmpDBPath)

	if err := sqliteBackup(app.DB, tmpDBPath); err != nil {
		http.Error(w, "Failed to create backup", http.StatusInternalServerError)
		return
	}

	tmpJSON := filepath.Join(filepath.Dir(tmpDBPath), "cheapskate-sync.json")
	if err := exportTransactionsJSON(app.DB, tmpJSON); err != nil {
		http.Error(w, "Failed to create JSON export", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmpJSON)

	if err := app.GDrive.UploadBackup(tmpDBPath, tmpJSON); err != nil {
		log.Printf("Google Drive sync error: %v", err)
		http.Error(w, "Failed to upload backup: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"last_sync":  time.Now().Format(time.RFC3339),
		"folder_name": app.GDrive.GetConfig().FolderName,
	})
}

func exportTransactionsJSON(dbPath, outputPath string) error {
	return fmt.Errorf("JSON export not implemented")
}
