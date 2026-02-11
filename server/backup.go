package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mattn/go-sqlite3"
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
func (app *Application) startBackupLoop() {
	interval := time.Duration(app.Config.BackupInterval) * time.Minute
	log.Printf("Backup enabled: path=%s interval=%s", app.Config.BackupPath, interval)

	// Run once immediately on startup
	app.runBackup()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		app.runBackup()
	}
}

func (app *Application) runBackup() {
	if err := app.performBackup(); err != nil {
		log.Printf("Backup failed (db): %v", err)
	}
	if err := app.performJSONExport(); err != nil {
		log.Printf("Backup failed (json): %v", err)
	}
	setLastBackupTime(time.Now())
	log.Printf("Backup completed to %s", app.Config.BackupPath)
}

// performBackup creates a consistent SQLite backup using the backup API.
func (app *Application) performBackup() error {
	destPath := filepath.Join(app.Config.BackupPath, "cheapskate.db")

	// Ensure backup directory exists
	if err := os.MkdirAll(app.Config.BackupPath, 0755); err != nil {
		return err
	}

	return sqliteBackup(app.DB, destPath)
}

// sqliteBackup copies a live SQLite database to destPath using the backup API.
func sqliteBackup(srcDB *sql.DB, destPath string) error {
	srcConn, err := srcDB.Conn(context.Background())
	if err != nil {
		return err
	}
	defer srcConn.Close()

	return srcConn.Raw(func(driverConn interface{}) error {
		src := driverConn.(*sqlite3.SQLiteConn)

		destDB, err := sql.Open("sqlite3", destPath)
		if err != nil {
			return err
		}
		defer destDB.Close()

		destConn, err := destDB.Conn(context.Background())
		if err != nil {
			return err
		}
		defer destConn.Close()

		return destConn.Raw(func(dc interface{}) error {
			dest := dc.(*sqlite3.SQLiteConn)
			backup, err := dest.Backup("main", src, "main")
			if err != nil {
				return err
			}
			_, err = backup.Step(-1)
			if err != nil {
				backup.Finish()
				return err
			}
			return backup.Finish()
		})
	})
}

// sqliteRestore copies a SQLite file into the live database using the backup API.
func sqliteRestore(destDB *sql.DB, srcPath string) error {
	destConn, err := destDB.Conn(context.Background())
	if err != nil {
		return err
	}
	defer destConn.Close()

	return destConn.Raw(func(driverConn interface{}) error {
		dest := driverConn.(*sqlite3.SQLiteConn)

		srcDB, err := sql.Open("sqlite3", srcPath+"?mode=ro")
		if err != nil {
			return err
		}
		defer srcDB.Close()

		srcConn, err := srcDB.Conn(context.Background())
		if err != nil {
			return err
		}
		defer srcConn.Close()

		return srcConn.Raw(func(sc interface{}) error {
			src := sc.(*sqlite3.SQLiteConn)
			backup, err := dest.Backup("main", src, "main")
			if err != nil {
				return err
			}
			_, err = backup.Step(-1)
			if err != nil {
				backup.Finish()
				return err
			}
			return backup.Finish()
		})
	})
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
