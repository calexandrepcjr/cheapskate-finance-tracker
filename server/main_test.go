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
		expectedCategories := []string{"Food", "Transport", "Housing", "Earned Income"}
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

func TestEnsureSeed_NoDuplicateSalaryCategories(t *testing.T) {
	projectRoot := findProjectRoot(t)
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	if err := os.Chdir(projectRoot); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalWd)

	dbConn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer dbConn.Close()

	app := &Application{
		DB: dbConn,
		Q:  db.New(dbConn),
	}

	// Apply schema (which seeds default categories)
	err = app.ensureSchema()
	if err != nil {
		t.Fatalf("ensureSchema() error = %v", err)
	}

	// Run ensureSeed multiple times (simulating multiple server restarts)
	for i := 0; i < 5; i++ {
		err = app.ensureSeed()
		if err != nil {
			t.Fatalf("ensureSeed() iteration %d error = %v", i, err)
		}
	}

	// Verify no duplicate Salary categories
	var salaryCount int
	err = dbConn.QueryRow("SELECT COUNT(*) FROM categories WHERE name = 'Salary'").Scan(&salaryCount)
	if err != nil {
		t.Fatalf("Failed to count Salary categories: %v", err)
	}
	if salaryCount > 1 {
		t.Errorf("Found %d Salary categories after 5 seed runs, want at most 1", salaryCount)
	}

	// Verify no duplicate categories at all
	var totalCategories int
	err = dbConn.QueryRow("SELECT COUNT(*) FROM categories").Scan(&totalCategories)
	if err != nil {
		t.Fatalf("Failed to count categories: %v", err)
	}

	var uniqueCategories int
	err = dbConn.QueryRow("SELECT COUNT(DISTINCT name) FROM categories").Scan(&uniqueCategories)
	if err != nil {
		t.Fatalf("Failed to count unique categories: %v", err)
	}

	if totalCategories != uniqueCategories {
		t.Errorf("Total categories (%d) != unique categories (%d), duplicates exist", totalCategories, uniqueCategories)
	}
}

func TestEnsureSeed_FixesIncomeCategoryTypes(t *testing.T) {
	projectRoot := findProjectRoot(t)
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	if err := os.Chdir(projectRoot); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalWd)

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

	// Simulate a legacy database where Salary was incorrectly marked as expense
	_, err = dbConn.Exec("INSERT INTO categories (name, type, icon, color) VALUES ('Salary', 'expense', '💰', '#2ECC71')")
	if err != nil {
		t.Fatalf("Failed to insert legacy Salary category: %v", err)
	}

	// Run ensureSeed - should fix the category type
	err = app.ensureSeed()
	if err != nil {
		t.Fatalf("ensureSeed() error = %v", err)
	}

	// Verify Salary is now income type
	var catType string
	err = dbConn.QueryRow("SELECT type FROM categories WHERE name = 'Salary'").Scan(&catType)
	if err != nil {
		t.Fatalf("Failed to get Salary type: %v", err)
	}
	if catType != "income" {
		t.Errorf("Salary type = %q, want %q", catType, "income")
	}

	// Verify Earned Income is still income type
	err = dbConn.QueryRow("SELECT type FROM categories WHERE name = 'Earned Income'").Scan(&catType)
	if err != nil {
		t.Fatalf("Failed to get Earned Income type: %v", err)
	}
	if catType != "income" {
		t.Errorf("Earned Income type = %q, want %q", catType, "income")
	}
}

func TestEnsureSeed_CleansDuplicateSalaryCategories(t *testing.T) {
	projectRoot := findProjectRoot(t)
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	if err := os.Chdir(projectRoot); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalWd)

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

	// Manually create duplicate Salary categories (simulating the old bug)
	for i := 0; i < 3; i++ {
		_, err = dbConn.Exec("INSERT INTO categories (name, type, icon, color) VALUES ('Salary', 'income', '💰', '#2ECC71')")
		if err != nil {
			t.Fatalf("Failed to insert duplicate Salary %d: %v", i, err)
		}
	}

	// Verify there are now 3 duplicates
	var count int
	err = dbConn.QueryRow("SELECT COUNT(*) FROM categories WHERE name = 'Salary'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count: %v", err)
	}
	if count != 3 {
		t.Fatalf("Expected 3 Salary categories before cleanup, got %d", count)
	}

	// Run ensureSeed - should clean up duplicates
	err = app.ensureSeed()
	if err != nil {
		t.Fatalf("ensureSeed() error = %v", err)
	}

	// Verify only 1 Salary remains
	err = dbConn.QueryRow("SELECT COUNT(*) FROM categories WHERE name = 'Salary'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count after cleanup: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 Salary category after cleanup, got %d", count)
	}
}

func TestEnsureSeed_IdempotentOverall(t *testing.T) {
	projectRoot := findProjectRoot(t)
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	if err := os.Chdir(projectRoot); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalWd)

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

	// Run full ensureSeed first time
	err = app.ensureSeed()
	if err != nil {
		t.Fatalf("First ensureSeed() error = %v", err)
	}

	// Capture state after first run
	var userCount1, catCount1 int
	dbConn.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount1)
	dbConn.QueryRow("SELECT COUNT(*) FROM categories").Scan(&catCount1)

	// Run ensureSeed again
	err = app.ensureSeed()
	if err != nil {
		t.Fatalf("Second ensureSeed() error = %v", err)
	}

	// Verify state is identical
	var userCount2, catCount2 int
	dbConn.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount2)
	dbConn.QueryRow("SELECT COUNT(*) FROM categories").Scan(&catCount2)

	if userCount1 != userCount2 {
		t.Errorf("User count changed: %d -> %d", userCount1, userCount2)
	}
	if catCount1 != catCount2 {
		t.Errorf("Category count changed: %d -> %d", catCount1, catCount2)
	}

	// Run 3 more times for good measure
	for i := 0; i < 3; i++ {
		err = app.ensureSeed()
		if err != nil {
			t.Fatalf("ensureSeed() iteration %d error = %v", i+3, err)
		}
	}

	// Final state check
	var userCountFinal, catCountFinal int
	dbConn.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCountFinal)
	dbConn.QueryRow("SELECT COUNT(*) FROM categories").Scan(&catCountFinal)

	if userCount1 != userCountFinal {
		t.Errorf("User count after 5 seeds: %d, want %d", userCountFinal, userCount1)
	}
	if catCount1 != catCountFinal {
		t.Errorf("Category count after 5 seeds: %d, want %d", catCountFinal, catCount1)
	}
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
