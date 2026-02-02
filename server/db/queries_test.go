package db_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/calexandrepcjr/cheapskate-finance-tracker/server/db"
	_ "github.com/mattn/go-sqlite3"
)

// setupTestDB creates a test database with schema and returns cleanup function
func setupTestDB(t *testing.T) (*db.Queries, func()) {
	t.Helper()

	dbConn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Enable foreign key enforcement (SQLite doesn't enforce by default)
	_, err = dbConn.Exec("PRAGMA foreign_keys = ON;")
	if err != nil {
		t.Fatalf("Failed to enable foreign keys: %v", err)
	}

	schema := `
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE categories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			type TEXT NOT NULL CHECK(type IN ('income', 'expense')),
			icon TEXT,
			color TEXT
		);

		CREATE TABLE transactions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			category_id INTEGER NOT NULL,
			amount INTEGER NOT NULL,
			currency TEXT NOT NULL DEFAULT 'USD',
			description TEXT NOT NULL,
			date DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id),
			FOREIGN KEY (category_id) REFERENCES categories(id)
		);

		INSERT INTO categories (name, type, icon, color) VALUES
		('Food', 'expense', '🍔', '#FF5733'),
		('Transport', 'expense', '🚕', '#33C1FF'),
		('Housing', 'expense', '🏠', '#8D33FF'),
		('Salary', 'income', '💰', '#2ECC71');

		INSERT INTO users (name, email) VALUES ('TestUser', 'test@example.com');
	`

	_, err = dbConn.Exec(schema)
	if err != nil {
		t.Fatalf("Failed to apply schema: %v", err)
	}

	queries := db.New(dbConn)

	cleanup := func() {
		dbConn.Close()
	}

	return queries, cleanup
}

func TestGetUser(t *testing.T) {
	queries, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("returns existing user", func(t *testing.T) {
		user, err := queries.GetUser(ctx, 1)
		if err != nil {
			t.Fatalf("GetUser() error = %v", err)
		}

		if user.ID != 1 {
			t.Errorf("User.ID = %d, want 1", user.ID)
		}
		if user.Name != "TestUser" {
			t.Errorf("User.Name = %q, want %q", user.Name, "TestUser")
		}
		if user.Email != "test@example.com" {
			t.Errorf("User.Email = %q, want %q", user.Email, "test@example.com")
		}
	})

	t.Run("returns error for non-existent user", func(t *testing.T) {
		_, err := queries.GetUser(ctx, 999)
		if err == nil {
			t.Error("GetUser() should return error for non-existent user")
		}
		if err != sql.ErrNoRows {
			t.Errorf("GetUser() error = %v, want sql.ErrNoRows", err)
		}
	})
}

func TestListUsers(t *testing.T) {
	queries, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	users, err := queries.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}

	if len(users) != 1 {
		t.Errorf("ListUsers() returned %d users, want 1", len(users))
	}

	if users[0].Name != "TestUser" {
		t.Errorf("First user name = %q, want %q", users[0].Name, "TestUser")
	}
}

func TestListCategories(t *testing.T) {
	queries, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	categories, err := queries.ListCategories(ctx)
	if err != nil {
		t.Fatalf("ListCategories() error = %v", err)
	}

	if len(categories) != 4 {
		t.Errorf("ListCategories() returned %d categories, want 4", len(categories))
	}

	// Verify ordering (by type, then name)
	// expense types come before income alphabetically
	expectedOrder := []string{"Food", "Housing", "Transport", "Salary"}
	for i, cat := range categories {
		if cat.Name != expectedOrder[i] {
			t.Errorf("Category[%d].Name = %q, want %q", i, cat.Name, expectedOrder[i])
		}
	}
}

func TestGetCategoryByName(t *testing.T) {
	queries, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("returns existing category", func(t *testing.T) {
		cat, err := queries.GetCategoryByName(ctx, "Food")
		if err != nil {
			t.Fatalf("GetCategoryByName() error = %v", err)
		}

		if cat.Name != "Food" {
			t.Errorf("Category.Name = %q, want %q", cat.Name, "Food")
		}
		if cat.Type != "expense" {
			t.Errorf("Category.Type = %q, want %q", cat.Type, "expense")
		}
		if !cat.Icon.Valid || cat.Icon.String != "🍔" {
			t.Errorf("Category.Icon = %v, want '🍔'", cat.Icon)
		}
	})

	t.Run("returns error for non-existent category", func(t *testing.T) {
		_, err := queries.GetCategoryByName(ctx, "NonExistent")
		if err == nil {
			t.Error("GetCategoryByName() should return error for non-existent category")
		}
		if err != sql.ErrNoRows {
			t.Errorf("GetCategoryByName() error = %v, want sql.ErrNoRows", err)
		}
	})

	t.Run("is case sensitive", func(t *testing.T) {
		_, err := queries.GetCategoryByName(ctx, "food") // lowercase
		if err == nil {
			t.Error("GetCategoryByName() should be case-sensitive")
		}
	})
}

func TestCreateTransaction(t *testing.T) {
	queries, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("creates transaction successfully", func(t *testing.T) {
		now := time.Now()
		params := db.CreateTransactionParams{
			UserID:      1,
			CategoryID:  1, // Food
			Amount:      2500,
			Currency:    "USD",
			Description: "Test pizza order",
			Date:        now,
		}

		tx, err := queries.CreateTransaction(ctx, params)
		if err != nil {
			t.Fatalf("CreateTransaction() error = %v", err)
		}

		if tx.ID == 0 {
			t.Error("Transaction.ID should be assigned")
		}
		if tx.Amount != 2500 {
			t.Errorf("Transaction.Amount = %d, want 2500", tx.Amount)
		}
		if tx.Description != "Test pizza order" {
			t.Errorf("Transaction.Description = %q, want %q", tx.Description, "Test pizza order")
		}
		if tx.Currency != "USD" {
			t.Errorf("Transaction.Currency = %q, want %q", tx.Currency, "USD")
		}
	})

	t.Run("fails with invalid user_id", func(t *testing.T) {
		params := db.CreateTransactionParams{
			UserID:      999, // Non-existent user
			CategoryID:  1,
			Amount:      1000,
			Currency:    "USD",
			Description: "Should fail",
			Date:        time.Now(),
		}

		_, err := queries.CreateTransaction(ctx, params)
		if err == nil {
			t.Error("CreateTransaction() should fail with invalid user_id (foreign key)")
		}
	})

	t.Run("fails with invalid category_id", func(t *testing.T) {
		params := db.CreateTransactionParams{
			UserID:      1,
			CategoryID:  999, // Non-existent category
			Amount:      1000,
			Currency:    "USD",
			Description: "Should fail",
			Date:        time.Now(),
		}

		_, err := queries.CreateTransaction(ctx, params)
		if err == nil {
			t.Error("CreateTransaction() should fail with invalid category_id (foreign key)")
		}
	})

	t.Run("stores amount in cents", func(t *testing.T) {
		params := db.CreateTransactionParams{
			UserID:      1,
			CategoryID:  1,
			Amount:      12345, // $123.45
			Currency:    "USD",
			Description: "Cent test",
			Date:        time.Now(),
		}

		tx, err := queries.CreateTransaction(ctx, params)
		if err != nil {
			t.Fatalf("CreateTransaction() error = %v", err)
		}

		if tx.Amount != 12345 {
			t.Errorf("Transaction.Amount = %d, want 12345", tx.Amount)
		}
	})
}

func TestListRecentTransactions(t *testing.T) {
	queries, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("returns empty list when no transactions", func(t *testing.T) {
		txs, err := queries.ListRecentTransactions(ctx)
		if err != nil {
			t.Fatalf("ListRecentTransactions() error = %v", err)
		}

		if len(txs) != 0 {
			t.Errorf("ListRecentTransactions() returned %d transactions, want 0", len(txs))
		}
	})

	t.Run("returns transactions with joined data", func(t *testing.T) {
		// Create a transaction first
		_, err := queries.CreateTransaction(ctx, db.CreateTransactionParams{
			UserID:      1,
			CategoryID:  1, // Food
			Amount:      1500,
			Currency:    "USD",
			Description: "Test meal",
			Date:        time.Now(),
		})
		if err != nil {
			t.Fatalf("Failed to create transaction: %v", err)
		}

		txs, err := queries.ListRecentTransactions(ctx)
		if err != nil {
			t.Fatalf("ListRecentTransactions() error = %v", err)
		}

		if len(txs) != 1 {
			t.Fatalf("ListRecentTransactions() returned %d transactions, want 1", len(txs))
		}

		tx := txs[0]
		// Verify joined fields
		if tx.CategoryName != "Food" {
			t.Errorf("Transaction.CategoryName = %q, want %q", tx.CategoryName, "Food")
		}
		if tx.UserName != "TestUser" {
			t.Errorf("Transaction.UserName = %q, want %q", tx.UserName, "TestUser")
		}
		if !tx.CategoryIcon.Valid || tx.CategoryIcon.String != "🍔" {
			t.Errorf("Transaction.CategoryIcon = %v, want '🍔'", tx.CategoryIcon)
		}
	})

	t.Run("orders by date descending", func(t *testing.T) {
		// Create transactions with different dates
		dates := []time.Time{
			time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
			time.Date(2024, 1, 3, 10, 0, 0, 0, time.UTC), // Most recent
			time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC),
		}

		for i, date := range dates {
			_, err := queries.CreateTransaction(ctx, db.CreateTransactionParams{
				UserID:      1,
				CategoryID:  1,
				Amount:      int64((i + 1) * 1000),
				Currency:    "USD",
				Description: date.Format("2006-01-02"),
				Date:        date,
			})
			if err != nil {
				t.Fatalf("Failed to create transaction: %v", err)
			}
		}

		txs, err := queries.ListRecentTransactions(ctx)
		if err != nil {
			t.Fatalf("ListRecentTransactions() error = %v", err)
		}

		// First transaction should be from the previous test, then Jan 3, Jan 2, Jan 1
		if len(txs) < 4 {
			t.Fatalf("Expected at least 4 transactions, got %d", len(txs))
		}

		// Check the new transactions are ordered correctly (skip first from previous test)
		if txs[1].Description != "2024-01-03" {
			t.Errorf("Second transaction should be from Jan 3, got %q", txs[1].Description)
		}
		if txs[2].Description != "2024-01-02" {
			t.Errorf("Third transaction should be from Jan 2, got %q", txs[2].Description)
		}
		if txs[3].Description != "2024-01-01" {
			t.Errorf("Fourth transaction should be from Jan 1, got %q", txs[3].Description)
		}
	})

	t.Run("limits to 20 results", func(t *testing.T) {
		// We need a fresh database for this test
		queries2, cleanup2 := setupTestDB(t)
		defer cleanup2()

		// Create 25 transactions
		for i := 0; i < 25; i++ {
			_, err := queries2.CreateTransaction(ctx, db.CreateTransactionParams{
				UserID:      1,
				CategoryID:  1,
				Amount:      int64(i * 100),
				Currency:    "USD",
				Description: "Transaction",
				Date:        time.Now(),
			})
			if err != nil {
				t.Fatalf("Failed to create transaction %d: %v", i, err)
			}
		}

		txs, err := queries2.ListRecentTransactions(ctx)
		if err != nil {
			t.Fatalf("ListRecentTransactions() error = %v", err)
		}

		if len(txs) != 20 {
			t.Errorf("ListRecentTransactions() returned %d transactions, want 20 (limit)", len(txs))
		}
	})
}

func TestCategoryTypes(t *testing.T) {
	queries, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	categories, err := queries.ListCategories(ctx)
	if err != nil {
		t.Fatalf("ListCategories() error = %v", err)
	}

	// Count expense vs income categories
	expenseCount := 0
	incomeCount := 0
	for _, cat := range categories {
		switch cat.Type {
		case "expense":
			expenseCount++
		case "income":
			incomeCount++
		default:
			t.Errorf("Unexpected category type: %q", cat.Type)
		}
	}

	if expenseCount != 3 {
		t.Errorf("Expected 3 expense categories, got %d", expenseCount)
	}
	if incomeCount != 1 {
		t.Errorf("Expected 1 income category, got %d", incomeCount)
	}
}
