package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/calexandrepcjr/cheapskate-finance-tracker/client/templates"
)

// BackupStatusResponse is the JSON response for backup status.
type BackupStatusResponse struct {
	Enabled      bool   `json:"enabled"`
	BackupPath   string `json:"backup_path"`
	LastBackupAt string `json:"last_backup_at"`
}

// HandleBackupStatus returns the current backup configuration and last backup time.
func (app *Application) HandleBackupStatus(w http.ResponseWriter, r *http.Request) {
	lastBackup := getLastBackupTime()
	lastBackupStr := ""
	if !lastBackup.IsZero() {
		lastBackupStr = lastBackup.UTC().Format(time.RFC3339)
	}

	resp := BackupStatusResponse{
		Enabled:      app.Config.BackupPath != "",
		BackupPath:   app.Config.BackupPath,
		LastBackupAt: lastBackupStr,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleBackupDownload creates a consistent SQLite backup and serves it as a download.
func (app *Application) HandleBackupDownload(w http.ResponseWriter, r *http.Request) {
	// Create temp file for the backup
	tmpFile, err := os.CreateTemp("", "cheapskate-backup-*.db")
	if err != nil {
		http.Error(w, "Failed to create backup", http.StatusInternalServerError)
		return
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Perform backup to temp file
	if err := sqliteBackup(app.DB, tmpPath); err != nil {
		log.Printf("Backup download failed: %v", err)
		http.Error(w, "Failed to create backup", http.StatusInternalServerError)
		return
	}

	// Serve the file
	filename := fmt.Sprintf("cheapskate-backup-%s.db", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/x-sqlite3")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	http.ServeFile(w, r, tmpPath)
}

// HandleBackupRestore accepts a .db file upload and restores it into the live database.
func (app *Application) HandleBackupRestore(w http.ResponseWriter, r *http.Request) {
	// Limit upload size to 100MB
	r.Body = http.MaxBytesReader(w, r.Body, 100<<20)

	file, _, err := r.FormFile("backup")
	if err != nil {
		templates.BackupRestoreError("No file provided").Render(r.Context(), w)
		return
	}
	defer file.Close()

	// Save to temp file
	tmpFile, err := os.CreateTemp("", "cheapskate-restore-*.db")
	if err != nil {
		templates.BackupRestoreError("Failed to process upload").Render(r.Context(), w)
		return
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpFile, file); err != nil {
		tmpFile.Close()
		templates.BackupRestoreError("Failed to save upload").Render(r.Context(), w)
		return
	}
	tmpFile.Close()

	// Validate SQLite magic bytes
	f, err := os.Open(tmpPath)
	if err != nil {
		templates.BackupRestoreError("Failed to read uploaded file").Render(r.Context(), w)
		return
	}
	magic := make([]byte, 16)
	_, err = io.ReadFull(f, magic)
	f.Close()
	if err != nil || string(magic) != "SQLite format 3\000" {
		templates.BackupRestoreError("Invalid file: not a SQLite database").Render(r.Context(), w)
		return
	}

	// Restore: copy uploaded DB into live database
	if err := sqliteRestore(app.DB, tmpPath); err != nil {
		log.Printf("Backup restore failed: %v", err)
		templates.BackupRestoreError("Failed to restore backup: " + err.Error()).Render(r.Context(), w)
		return
	}

	log.Println("Database restored from uploaded backup")
	templates.BackupRestoreSuccess().Render(r.Context(), w)
}
