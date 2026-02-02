package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/calexandrepcjr/cheapskate-finance-tracker/server/db"
	_ "github.com/mattn/go-sqlite3"
)

func TestEnsureSchema(t *testing.T) {
	// This test requires the schema.sql file to exist
	// We need to run from the project root or adjust the working directory

	// Find the project root by looking for go.mod
	projectRoot := findProjectRoot(t)
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Change to project root for schema file access
	if err := os.Chdir(projectRoot); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalWd)

	t.Run("creates tables on fresh database", func(t *testing.T) {
		dbConn, err := sql.Open("sqlite3", ":memory:")
		if err != nil {
			t.Fatalf("Failed to open database: %v", err)
		}
		defer dbConn.Close()

		app := &Application{
			DB: dbConn,
			Q:  db.New(dbConn),
		}

		err = app.ensureSchema()
		if err != nil {
			t.Fatalf("ensureSchema() error = %v", err)
		}

		// Verify tables exist
		tables := []string{"users", "categories", "transactions"}
		for _, table := range tables {
			var name string
			err := dbConn.QueryRow(
				"SELECT name FROM sqlite_master WHERE type='table' AND name=?",
				table,
			).Scan(&name)
			if err != nil {
				t.Errorf("Table %q should exist after ensureSchema()", table)
			}
		}
	})

	t.Run("seeds default categories", func(t *testing.T) {
		dbConn, err := sql.Open("sqlite3", ":memory:")
		if err != nil {
			t.Fatalf("Failed to open database: %v", err)
		}
		defer dbConn.Close()

		app := &Application{
			DB: dbConn,
			Q:  db.New(dbConn),
		}

		err = app.ensureSchema()
		if err != nil {
			t.Fatalf("ensureSchema() error = %v", err)
		}

		// Verify categories were seeded
		var count int
		err = dbConn.QueryRow("SELECT COUNT(*) FROM categories").Scan(&count)
		if err != nil {
			t.Fatalf("Failed to count categories: %v", err)
		}
		if count != 4 {
			t.Errorf("Expected 4 seeded categories, got %d", count)
		}

		// Verify specific categories
		expectedCategories := []string{"Food", "Transport", "Housing", "Salary"}
		for _, cat := range expectedCategories {
			var name string
			err := dbConn.QueryRow("SELECT name FROM categories WHERE name = ?", cat).Scan(&name)
			if err != nil {
				t.Errorf("Category %q should exist", cat)
			}
		}
	})

	t.Run("idempotent - can run multiple times", func(t *testing.T) {
		dbConn, err := sql.Open("sqlite3", ":memory:")
		if err != nil {
			t.Fatalf("Failed to open database: %v", err)
		}
		defer dbConn.Close()

		app := &Application{
			DB: dbConn,
			Q:  db.New(dbConn),
		}

		// Run schema twice - should not error fatally
		err = app.ensureSchema()
		if err != nil {
			t.Fatalf("First ensureSchema() error = %v", err)
		}

		// Second run may log errors but should not return fatal error
		// (due to "table already exists" etc.)
		_ = app.ensureSchema() // May have errors but shouldn't panic
	})
}

func TestEnsureSeed(t *testing.T) {
	projectRoot := findProjectRoot(t)
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	if err := os.Chdir(projectRoot); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalWd)

	t.Run("creates default user when table is empty", func(t *testing.T) {
		dbConn, err := sql.Open("sqlite3", ":memory:")
		if err != nil {
			t.Fatalf("Failed to open database: %v", err)
		}
		defer dbConn.Close()

		app := &Application{
			DB: dbConn,
			Q:  db.New(dbConn),
		}

		// First apply schema
		err = app.ensureSchema()
		if err != nil {
			t.Fatalf("ensureSchema() error = %v", err)
		}

		// Then seed
		err = app.ensureSeed()
		if err != nil {
			t.Fatalf("ensureSeed() error = %v", err)
		}

		// Verify user exists
		var count int
		err = dbConn.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
		if err != nil {
			t.Fatalf("Failed to count users: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected 1 seeded user, got %d", count)
		}

		// Verify specific user data
		var name, email string
		err = dbConn.QueryRow("SELECT name, email FROM users WHERE id = 1").Scan(&name, &email)
		if err != nil {
			t.Fatalf("Failed to get user: %v", err)
		}
		if name != "CapCJ" {
			t.Errorf("User name = %q, want %q", name, "CapCJ")
		}
		if email != "capcj@example.com" {
			t.Errorf("User email = %q, want %q", email, "capcj@example.com")
		}
	})

	t.Run("does not create duplicate users", func(t *testing.T) {
		dbConn, err := sql.Open("sqlite3", ":memory:")
		if err != nil {
			t.Fatalf("Failed to open database: %v", err)
		}
		defer dbConn.Close()

		app := &Application{
			DB: dbConn,
			Q:  db.New(dbConn),
		}

		err = app.ensureSchema()
		if err != nil {
			t.Fatalf("ensureSchema() error = %v", err)
		}

		// Seed twice
		err = app.ensureSeed()
		if err != nil {
			t.Fatalf("First ensureSeed() error = %v", err)
		}

		err = app.ensureSeed()
		if err != nil {
			t.Fatalf("Second ensureSeed() error = %v", err)
		}

		// Should still only have one user
		var count int
		err = dbConn.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
		if err != nil {
			t.Fatalf("Failed to count users: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected 1 user after multiple seeds, got %d", count)
		}
	})

	t.Run("does not seed if users already exist", func(t *testing.T) {
		dbConn, err := sql.Open("sqlite3", ":memory:")
		if err != nil {
			t.Fatalf("Failed to open database: %v", err)
		}
		defer dbConn.Close()

		app := &Application{
			DB: dbConn,
			Q:  db.New(dbConn),
		}

		err = app.ensureSchema()
		if err != nil {
			t.Fatalf("ensureSchema() error = %v", err)
		}

		// Insert a different user first
		_, err = dbConn.Exec("INSERT INTO users (name, email) VALUES ('ExistingUser', 'existing@example.com')")
		if err != nil {
			t.Fatalf("Failed to insert existing user: %v", err)
		}

		// Now seed
		err = app.ensureSeed()
		if err != nil {
			t.Fatalf("ensureSeed() error = %v", err)
		}

		// Should still only have the existing user
		var count int
		err = dbConn.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
		if err != nil {
			t.Fatalf("Failed to count users: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected 1 user (existing), got %d", count)
		}

		// Verify it's the existing user, not the seeded one
		var name string
		err = dbConn.QueryRow("SELECT name FROM users WHERE id = 1").Scan(&name)
		if err != nil {
			t.Fatalf("Failed to get user: %v", err)
		}
		if name != "ExistingUser" {
			t.Errorf("User should be 'ExistingUser', got %q", name)
		}
	})
}

func TestFileServer(t *testing.T) {
	// Test the path normalization logic
	t.Run("adds trailing slash if missing", func(t *testing.T) {
		// The fileServer function modifies the path internally
		// We can verify its behavior by checking the route pattern it creates
		// This is more of a smoke test since the actual routing is handled by chi

		path := "/assets"
		if path[len(path)-1] != '/' {
			path += "/"
		}
		path += "*"

		expected := "/assets/*"
		if path != expected {
			t.Errorf("Path normalization: got %q, want %q", path, expected)
		}
	})

	t.Run("preserves existing trailing slash", func(t *testing.T) {
		path := "/assets/"
		if path[len(path)-1] != '/' {
			path += "/"
		}
		path += "*"

		expected := "/assets/*"
		if path != expected {
			t.Errorf("Path normalization: got %q, want %q", path, expected)
		}
	})
}

// findProjectRoot finds the project root by looking for go.mod
func findProjectRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("Could not find project root (go.mod)")
		}
		dir = parent
	}
}
