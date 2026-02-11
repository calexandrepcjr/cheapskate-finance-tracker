package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/calexandrepcjr/cheapskate-finance-tracker/server/db"
	_ "github.com/mattn/go-sqlite3"
)

// setupTestAppWithFile creates a test app using a file-based SQLite database.
// This is needed for backup tests since the backup API copies between databases.
func setupTestAppWithFile(t *testing.T, dbPath string) *Application {
	t.Helper()

	dbConn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	schema := `
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS categories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			type TEXT NOT NULL CHECK(type IN ('income', 'expense')),
			icon TEXT,
			color TEXT
		);

		CREATE TABLE IF NOT EXISTS transactions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			category_id INTEGER NOT NULL,
			amount INTEGER NOT NULL,
			currency TEXT NOT NULL DEFAULT 'USD',
			description TEXT NOT NULL,
			date DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			deleted_at DATETIME DEFAULT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id),
			FOREIGN KEY (category_id) REFERENCES categories(id)
		);
	`

	_, err = dbConn.Exec(schema)
	if err != nil {
		t.Fatalf("Failed to apply test schema: %v", err)
	}

	// Seed data
	_, err = dbConn.Exec(`
		INSERT OR IGNORE INTO categories (name, type, icon, color) VALUES
		('Food', 'expense', '🍔', '#FF5733'),
		('Transport', 'expense', '🚕', '#33C1FF');
		INSERT OR IGNORE INTO users (name, email) VALUES ('TestUser', 'test@example.com');
	`)
	if err != nil {
		t.Fatalf("Failed to seed test data: %v", err)
	}

	queries := db.New(dbConn)

	return &Application{
		Config: Config{Port: 8080, DBPath: dbPath},
		DB:     dbConn,
		Q:      queries,
	}
}

func TestPerformBackup(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.db")
	app := setupTestAppWithFile(t, srcPath)
	defer app.DB.Close()

	// Insert test transaction
	_, err := app.Q.CreateTransaction(context.Background(), db.CreateTransactionParams{
		UserID:      1,
		CategoryID:  1,
		Amount:      -1250,
		Currency:    "USD",
		Description: "test pizza",
		Date:        time.Now(),
	})
	if err != nil {
		t.Fatalf("Failed to create test transaction: %v", err)
	}

	// Set backup path and perform backup
	app.Config.BackupPath = filepath.Join(tmpDir, "backups")
	err = app.performBackup()
	if err != nil {
		t.Fatalf("performBackup failed: %v", err)
	}

	// Verify backup file exists
	backupPath := filepath.Join(app.Config.BackupPath, "cheapskate.db")
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Fatal("Backup file was not created")
	}

	// Verify backup contains the data
	backupDB, err := sql.Open("sqlite3", backupPath)
	if err != nil {
		t.Fatalf("Failed to open backup database: %v", err)
	}
	defer backupDB.Close()

	var count int
	err = backupDB.QueryRow("SELECT COUNT(*) FROM transactions").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query backup database: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 transaction in backup, got %d", count)
	}

	var desc string
	err = backupDB.QueryRow("SELECT description FROM transactions LIMIT 1").Scan(&desc)
	if err != nil {
		t.Fatalf("Failed to read transaction from backup: %v", err)
	}
	if desc != "test pizza" {
		t.Errorf("Expected description 'test pizza', got %q", desc)
	}
}

func TestPerformJSONExport(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.db")
	app := setupTestAppWithFile(t, srcPath)
	defer app.DB.Close()

	// Insert test transaction
	_, err := app.Q.CreateTransaction(context.Background(), db.CreateTransactionParams{
		UserID:      1,
		CategoryID:  1,
		Amount:      -500,
		Currency:    "USD",
		Description: "coffee",
		Date:        time.Now(),
	})
	if err != nil {
		t.Fatalf("Failed to create test transaction: %v", err)
	}

	// Perform JSON export
	app.Config.BackupPath = filepath.Join(tmpDir, "backups")
	os.MkdirAll(app.Config.BackupPath, 0755)
	err = app.performJSONExport()
	if err != nil {
		t.Fatalf("performJSONExport failed: %v", err)
	}

	// Read and parse the JSON file
	jsonPath := filepath.Join(app.Config.BackupPath, "cheapskate.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("Failed to read JSON export: %v", err)
	}

	var export StorageExportResponse
	if err := json.Unmarshal(data, &export); err != nil {
		t.Fatalf("Failed to parse JSON export: %v", err)
	}

	if len(export.Transactions) != 1 {
		t.Errorf("Expected 1 transaction in JSON export, got %d", len(export.Transactions))
	}
	if export.Transactions[0].Description != "coffee" {
		t.Errorf("Expected description 'coffee', got %q", export.Transactions[0].Description)
	}
	if len(export.Categories) != 2 {
		t.Errorf("Expected 2 categories in JSON export, got %d", len(export.Categories))
	}
	if export.Year != "all" {
		t.Errorf("Expected year 'all', got %q", export.Year)
	}
	if export.ExportedAt == "" {
		t.Error("Expected ExportedAt to be set")
	}
}

func TestHandleBackupDownload(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.db")
	app := setupTestAppWithFile(t, srcPath)
	defer app.DB.Close()

	// Insert test data
	_, err := app.Q.CreateTransaction(context.Background(), db.CreateTransactionParams{
		UserID:      1,
		CategoryID:  1,
		Amount:      -750,
		Currency:    "USD",
		Description: "lunch",
		Date:        time.Now(),
	})
	if err != nil {
		t.Fatalf("Failed to create test transaction: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/backup/download", nil)
	rec := httptest.NewRecorder()

	app.HandleBackupDownload(rec, req)

	resp := rec.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Check headers
	contentDisp := resp.Header.Get("Content-Disposition")
	if contentDisp == "" {
		t.Error("Expected Content-Disposition header to be set")
	}

	// Verify the response body starts with SQLite magic bytes
	body, _ := io.ReadAll(resp.Body)
	if len(body) < 16 {
		t.Fatal("Response body too small to be a SQLite database")
	}
	if string(body[:16]) != "SQLite format 3\000" {
		t.Error("Response does not contain SQLite magic bytes")
	}
}

func TestHandleBackupRestore(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source database with data to restore from
	srcPath := filepath.Join(tmpDir, "restore-source.db")
	srcDB, err := sql.Open("sqlite3", srcPath)
	if err != nil {
		t.Fatalf("Failed to create source database: %v", err)
	}
	_, err = srcDB.Exec(`
		CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL, email TEXT NOT NULL UNIQUE, created_at DATETIME DEFAULT CURRENT_TIMESTAMP);
		CREATE TABLE categories (id INTEGER PRIMARY KEY, name TEXT NOT NULL, type TEXT NOT NULL CHECK(type IN ('income', 'expense')), icon TEXT, color TEXT);
		CREATE TABLE transactions (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL, category_id INTEGER NOT NULL, amount INTEGER NOT NULL, currency TEXT NOT NULL DEFAULT 'USD', description TEXT NOT NULL, date DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, created_at DATETIME DEFAULT CURRENT_TIMESTAMP, deleted_at DATETIME DEFAULT NULL);
		INSERT INTO users (name, email) VALUES ('RestoredUser', 'restored@example.com');
		INSERT INTO categories (name, type) VALUES ('Restored Cat', 'expense');
		INSERT INTO transactions (user_id, category_id, amount, currency, description, date) VALUES (1, 1, -9999, 'USD', 'restored transaction', CURRENT_TIMESTAMP);
	`)
	if err != nil {
		t.Fatalf("Failed to set up source database: %v", err)
	}
	srcDB.Close()

	// Create the target app (empty database to restore into)
	destPath := filepath.Join(tmpDir, "target.db")
	app := setupTestAppWithFile(t, destPath)
	defer app.DB.Close()

	// Read the source .db file and create a multipart upload
	fileData, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("Failed to read source database file: %v", err)
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("backup", "restore-source.db")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}
	part.Write(fileData)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/backup/restore", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	app.HandleBackupRestore(rec, req)

	resp := rec.Result()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	// Verify the data was restored
	var desc string
	err = app.DB.QueryRow("SELECT description FROM transactions LIMIT 1").Scan(&desc)
	if err != nil {
		t.Fatalf("Failed to query restored database: %v", err)
	}
	if desc != "restored transaction" {
		t.Errorf("Expected 'restored transaction', got %q", desc)
	}
}

func TestHandleBackupRestoreRejectsInvalidFile(t *testing.T) {
	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "target.db")
	app := setupTestAppWithFile(t, destPath)
	defer app.DB.Close()

	// Create a multipart upload with invalid data
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("backup", "not-a-database.db")
	part.Write([]byte("this is not a sqlite database at all"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/backup/restore", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	app.HandleBackupRestore(rec, req)

	resp := rec.Result()
	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 (HTMX error fragment), got %d", resp.StatusCode)
	}
	if !bytes.Contains(body, []byte("not a SQLite database")) {
		t.Errorf("Expected error about invalid SQLite file, got: %s", bodyStr)
	}
}

func TestHandleBackupStatus(t *testing.T) {
	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "target.db")
	app := setupTestAppWithFile(t, destPath)
	defer app.DB.Close()

	t.Run("backup disabled", func(t *testing.T) {
		app.Config.BackupPath = ""
		req := httptest.NewRequest(http.MethodGet, "/api/backup/status", nil)
		rec := httptest.NewRecorder()

		app.HandleBackupStatus(rec, req)

		var status BackupStatusResponse
		json.NewDecoder(rec.Body).Decode(&status)

		if status.Enabled {
			t.Error("Expected backup to be disabled")
		}
	})

	t.Run("backup enabled", func(t *testing.T) {
		app.Config.BackupPath = "/some/path"
		setLastBackupTime(time.Now())

		req := httptest.NewRequest(http.MethodGet, "/api/backup/status", nil)
		rec := httptest.NewRecorder()

		app.HandleBackupStatus(rec, req)

		var status BackupStatusResponse
		json.NewDecoder(rec.Body).Decode(&status)

		if !status.Enabled {
			t.Error("Expected backup to be enabled")
		}
		if status.BackupPath != "/some/path" {
			t.Errorf("Expected backup path '/some/path', got %q", status.BackupPath)
		}
		if status.LastBackupAt == "" {
			t.Error("Expected LastBackupAt to be set")
		}
	})
}

func TestLastBackupTime(t *testing.T) {
	// Reset state
	setLastBackupTime(time.Time{})

	// Should be zero initially
	if !getLastBackupTime().IsZero() {
		t.Error("Expected initial last backup time to be zero")
	}

	// Set and read back
	now := time.Now()
	setLastBackupTime(now)
	got := getLastBackupTime()
	if !got.Equal(now) {
		t.Errorf("Expected %v, got %v", now, got)
	}
}

func TestRunBackupDoesNotUpdateTimeOnFailure(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.db")
	app := setupTestAppWithFile(t, srcPath)
	defer app.DB.Close()

	// Reset backup time
	setLastBackupTime(time.Time{})

	// Set backup path to an impossible location
	app.Config.BackupPath = "/dev/null/impossible/path"

	app.runBackup()

	// lastBackupTime should still be zero because performBackup failed
	if !getLastBackupTime().IsZero() {
		t.Error("Expected lastBackupTime to remain zero after failed backup")
	}
}

func TestRunBackupUpdatesTimeOnSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.db")
	app := setupTestAppWithFile(t, srcPath)
	defer app.DB.Close()

	// Reset backup time
	setLastBackupTime(time.Time{})

	// Set backup path to a valid location
	app.Config.BackupPath = filepath.Join(tmpDir, "backups")

	before := time.Now()
	app.runBackup()
	after := time.Now()

	got := getLastBackupTime()
	if got.IsZero() {
		t.Error("Expected lastBackupTime to be set after successful backup")
	}
	if got.Before(before) || got.After(after) {
		t.Errorf("lastBackupTime %v not in expected range [%v, %v]", got, before, after)
	}
}
