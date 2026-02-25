package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	lastBackupMu   sync.RWMutex
	lastBackupTime time.Time
)

// getLastBackupTime returns the time of the last successful backup.
func getLastBackupTime() time.Time {
	lastBackupMu.RLock()
	defer lastBackupMu.RUnlock()
	return lastBackupTime
}

func setLastBackupTime(t time.Time) {
	lastBackupMu.Lock()
	defer lastBackupMu.Unlock()
	lastBackupTime = t
}

// startBackupLoop runs periodic backups at the configured interval.
func (app *Application) startBackupLoop(ctx context.Context) {
	interval := time.Duration(app.Config.BackupInterval) * time.Minute
	log.Printf("Backup enabled: path=%s interval=%s", app.Config.BackupPath, interval)

	// Run once immediately on startup
	app.runBackup()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Backup loop stopping")
			return
		case <-ticker.C:
			app.runBackup()
		}
	}
}

func (app *Application) runBackup() {
	dbErr := app.performBackup()
	if dbErr != nil {
		log.Printf("Backup failed (db): %v", dbErr)
	}
	if err := app.performJSONExport(); err != nil {
		log.Printf("Backup failed (json): %v", err)
	}
	if dbErr == nil {
		setLastBackupTime(time.Now())
		log.Printf("Backup completed to %s", app.Config.BackupPath)
	}
}

// performBackup creates a consistent SQLite backup using VACUUM INTO.
func (app *Application) performBackup() error {
	destPath := filepath.Join(app.Config.BackupPath, "cheapskate.db")

	// Ensure backup directory exists
	if err := os.MkdirAll(app.Config.BackupPath, 0755); err != nil {
		return err
	}

	return sqliteBackup(app.DB, destPath)
}

// sqliteBackup copies a live SQLite database to destPath using VACUUM INTO.
// This is a portable approach that works with any SQLite driver (no CGO required).
func sqliteBackup(srcDB *sql.DB, destPath string) error {
	// VACUUM INTO fails if the destination file already exists, so remove it first.
	os.Remove(destPath)

	// Escape single quotes in the path for the SQL literal.
	escaped := strings.ReplaceAll(destPath, "'", "''")
	_, err := srcDB.Exec("VACUUM INTO '" + escaped + "'")
	return err
}

// sqliteRestore copies a SQLite file into the live database using ATTACH + table copy.
// This is a portable approach that works with any SQLite driver (no CGO required).
func sqliteRestore(destDB *sql.DB, srcPath string) error {
	escaped := strings.ReplaceAll(srcPath, "'", "''")

	// Attach the source backup database
	_, err := destDB.Exec("ATTACH DATABASE '" + escaped + "' AS restore_src")
	if err != nil {
		return fmt.Errorf("attach source: %w", err)
	}
	defer destDB.Exec("DETACH restore_src")

	// Get list of tables from the backup
	rows, err := destDB.Query("SELECT name FROM restore_src.sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return fmt.Errorf("list tables: %w", err)
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return fmt.Errorf("scan table name: %w", err)
		}
		tables = append(tables, name)
	}
	rows.Close()

	// Disable foreign keys during restore to avoid constraint violations during copy
	destDB.Exec("PRAGMA foreign_keys = OFF")
	defer destDB.Exec("PRAGMA foreign_keys = ON")

	// For each table: clear existing data, then copy from backup
	for _, table := range tables {
		_, err := destDB.Exec("DELETE FROM main." + table)
		if err != nil {
			return fmt.Errorf("delete from %s: %w", table, err)
		}
		_, err = destDB.Exec("INSERT INTO main." + table + " SELECT * FROM restore_src." + table)
		if err != nil {
			return fmt.Errorf("copy %s: %w", table, err)
		}
	}

	return nil
}

// performJSONExport writes a human-readable JSON export alongside the DB backup.
func (app *Application) performJSONExport() error {
	ctx := context.Background()

	txRows, err := app.Q.ListAllTransactionsForExport(ctx)
	if err != nil {
		return err
	}

	transactions := make([]StorageTransaction, 0, len(txRows))
	for _, tx := range txRows {
		transactions = append(transactions, StorageTransaction{
			ID:           tx.ID,
			Amount:       tx.Amount,
			Currency:     tx.Currency,
			Description:  tx.Description,
			Date:         tx.Date.UTC().Format(time.RFC3339),
			CategoryName: tx.CategoryName,
			CategoryType: tx.CategoryType,
		})
	}

	catRows, err := app.Q.ListCategories(ctx)
	if err != nil {
		return err
	}

	categories := make([]StorageCategory, 0, len(catRows))
	for _, cat := range catRows {
		icon := ""
		if cat.Icon.Valid {
			icon = cat.Icon.String
		}
		color := ""
		if cat.Color.Valid {
			color = cat.Color.String
		}
		categories = append(categories, StorageCategory{
			ID:    cat.ID,
			Name:  cat.Name,
			Type:  cat.Type,
			Icon:  icon,
			Color: color,
		})
	}

	resp := StorageExportResponse{
		Transactions: transactions,
		Categories:   categories,
		Year:         "all",
		ExportedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	destPath := filepath.Join(app.Config.BackupPath, "cheapskate.json")
	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}
