package main

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/calexandrepcjr/cheapskate-finance-tracker/server/db"
	_ "github.com/mattn/go-sqlite3"
)

// setupTestApp creates a test Application with an in-memory SQLite database
func setupTestApp(t *testing.T) *Application {
	t.Helper()

	// Create in-memory SQLite database
	dbConn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Apply schema
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
		t.Fatalf("Failed to apply test schema: %v", err)
	}

	queries := db.New(dbConn)

	return &Application{
		Config: Config{Port: 8080, DBPath: ":memory:"},
		DB:     dbConn,
		Q:      queries,
	}
}

// cleanupTestApp closes the test database connection
func cleanupTestApp(t *testing.T, app *Application) {
	t.Helper()
	if app.DB != nil {
		app.DB.Close()
	}
}

func TestHandleHome(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	app.HandleHome(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("HandleHome() status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	// Check for expected content from the home template
	if !strings.Contains(body, "Cheapskate") {
		t.Error("HandleHome() response should contain 'Cheapskate'")
	}
}

func TestHandleDashboard(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	t.Run("empty transactions", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
		rec := httptest.NewRecorder()

		app.HandleDashboard(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("HandleDashboard() status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("with transactions", func(t *testing.T) {
		// Insert a test transaction
		ctx := context.Background()
		_, err := app.Q.CreateTransaction(ctx, db.CreateTransactionParams{
			UserID:      1,
			CategoryID:  1, // Food
			Amount:      2500,
			Currency:    "USD",
			Description: "Test pizza",
			Date:        time.Now(),
		})
		if err != nil {
			t.Fatalf("Failed to create test transaction: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
		rec := httptest.NewRecorder()

		app.HandleDashboard(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("HandleDashboard() status = %d, want %d", rec.Code, http.StatusOK)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "Test pizza") {
			t.Error("HandleDashboard() should display transaction description")
		}
	})
}

func TestHandleTransactionCreate(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantStatusCode int
		wantContains   string
		wantError      bool
	}{
		{
			name:           "valid transaction - food",
			input:          "25 pizza delivery",
			wantStatusCode: http.StatusOK,
			wantContains:   "$25.00",
			wantError:      false,
		},
		{
			name:           "valid transaction - transport",
			input:          "15.50 uber ride",
			wantStatusCode: http.StatusOK,
			wantContains:   "$15.50",
			wantError:      false,
		},
		{
			name:           "valid transaction - default category",
			input:          "100 electricity bill",
			wantStatusCode: http.StatusOK,
			wantContains:   "$100.00",
			wantError:      false,
		},
		{
			name:           "invalid input - missing description",
			input:          "50",
			wantStatusCode: http.StatusOK, // Returns 200 with error HTML fragment
			wantContains:   "Could not understand",
			wantError:      true,
		},
		{
			name:           "invalid input - empty",
			input:          "",
			wantStatusCode: http.StatusOK,
			wantContains:   "Could not understand",
			wantError:      true,
		},
		{
			name:           "invalid input - no amount",
			input:          "pizza",
			wantStatusCode: http.StatusOK,
			wantContains:   "Could not understand",
			wantError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupTestApp(t)
			defer cleanupTestApp(t, app)

			// Create form data
			form := url.Values{}
			form.Add("input", tt.input)

			req := httptest.NewRequest(http.MethodPost, "/api/transaction", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()

			app.HandleTransactionCreate(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("HandleTransactionCreate() status = %d, want %d", rec.Code, tt.wantStatusCode)
			}

			body := rec.Body.String()
			if !strings.Contains(body, tt.wantContains) {
				t.Errorf("HandleTransactionCreate() body should contain %q, got: %s", tt.wantContains, body)
			}

			// Verify transaction was created in database (for valid inputs)
			if !tt.wantError {
				txs, err := app.Q.ListRecentTransactions(context.Background())
				if err != nil {
					t.Fatalf("Failed to list transactions: %v", err)
				}
				if len(txs) == 0 {
					t.Error("HandleTransactionCreate() should have created a transaction")
				}
			}
		})
	}
}

func TestHandleTransactionCreate_CategoryResolution(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	tests := []struct {
		name         string
		input        string
		wantCategory string
	}{
		{
			name:         "food category",
			input:        "10 pizza",
			wantCategory: "Food",
		},
		{
			name:         "transport category",
			input:        "20 taxi",
			wantCategory: "Transport",
		},
		{
			name:         "default to housing",
			input:        "30 random stuff",
			wantCategory: "Housing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form := url.Values{}
			form.Add("input", tt.input)

			req := httptest.NewRequest(http.MethodPost, "/api/transaction", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()

			app.HandleTransactionCreate(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("HandleTransactionCreate() status = %d", rec.Code)
			}

			// Get the latest transaction
			txs, err := app.Q.ListRecentTransactions(context.Background())
			if err != nil {
				t.Fatalf("Failed to list transactions: %v", err)
			}
			if len(txs) == 0 {
				t.Fatal("No transactions found")
			}

			// Find the most recent one (first in the list due to ORDER BY date DESC)
			latestTx := txs[0]
			if latestTx.CategoryName != tt.wantCategory {
				t.Errorf("Transaction category = %q, want %q", latestTx.CategoryName, tt.wantCategory)
			}
		})
	}
}

func TestHandleTransactionCreate_AmountConversion(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	tests := []struct {
		name       string
		input      string
		wantCents  int64
	}{
		{
			name:      "integer amount",
			input:     "50 test item",
			wantCents: 5000,
		},
		{
			name:      "decimal amount",
			input:     "12.50 test item",
			wantCents: 1250,
		},
		{
			name:      "small decimal",
			input:     "0.99 test item",
			wantCents: 99,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear previous transactions by creating fresh app
			app := setupTestApp(t)
			defer cleanupTestApp(t, app)

			form := url.Values{}
			form.Add("input", tt.input)

			req := httptest.NewRequest(http.MethodPost, "/api/transaction", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()

			app.HandleTransactionCreate(rec, req)

			txs, err := app.Q.ListRecentTransactions(context.Background())
			if err != nil {
				t.Fatalf("Failed to list transactions: %v", err)
			}
			if len(txs) == 0 {
				t.Fatal("No transactions found")
			}

			if txs[0].Amount != tt.wantCents {
				t.Errorf("Transaction amount = %d cents, want %d cents", txs[0].Amount, tt.wantCents)
			}
		})
	}
}
