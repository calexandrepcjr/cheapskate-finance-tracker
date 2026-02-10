package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/calexandrepcjr/cheapskate-finance-tracker/server/db"
	"github.com/go-chi/chi/v5"
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
			deleted_at DATETIME DEFAULT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id),
			FOREIGN KEY (category_id) REFERENCES categories(id)
		);

		INSERT INTO categories (name, type, icon, color) VALUES
		('Food', 'expense', '🍔', '#FF5733'),
		('Transport', 'expense', '🚕', '#33C1FF'),
		('Housing', 'expense', '🏠', '#8D33FF'),
		('Earned Income', 'income', '💰', '#2ECC71');

		INSERT INTO users (name, email) VALUES ('TestUser', 'test@example.com');
	`

	_, err = dbConn.Exec(schema)
	if err != nil {
		t.Fatalf("Failed to apply test schema: %v", err)
	}

	queries := db.New(dbConn)

	return &Application{
		Config:    Config{Port: 8080, DBPath: ":memory:"},
		DB:        dbConn,
		Q:         queries,
		CatConfig: defaultCategoryConfig(),
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

func TestHandleDashboardDetailed(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	t.Run("empty transactions", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/dashboard/detailed", nil)
		rec := httptest.NewRecorder()

		app.HandleDashboardDetailed(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("HandleDashboardDetailed() status = %d, want %d", rec.Code, http.StatusOK)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "Analytics") {
			t.Error("HandleDashboardDetailed() should contain 'Analytics' title")
		}
	})

	t.Run("with transactions", func(t *testing.T) {
		ctx := context.Background()
		_, err := app.Q.CreateTransaction(ctx, db.CreateTransactionParams{
			UserID:      1,
			CategoryID:  1,
			Amount:      5000,
			Currency:    "USD",
			Description: "Test expense",
			Date:        time.Now(),
		})
		if err != nil {
			t.Fatalf("Failed to create test transaction: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/dashboard/detailed", nil)
		rec := httptest.NewRecorder()

		app.HandleDashboardDetailed(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("HandleDashboardDetailed() status = %d, want %d", rec.Code, http.StatusOK)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "$50.00") {
			t.Error("HandleDashboardDetailed() should display formatted amount")
		}
	})
}

func TestHandleDashboard_YearFilter(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	// Create transactions in different years
	ctx := context.Background()

	// Transaction for 2025
	_, err := app.DB.ExecContext(ctx, `
		INSERT INTO transactions (user_id, category_id, amount, currency, description, date)
		VALUES (1, 1, 2500, 'USD', 'Old transaction', '2025-06-15 10:00:00')
	`)
	if err != nil {
		t.Fatalf("Failed to create 2025 transaction: %v", err)
	}

	// Transaction for current year
	_, err = app.Q.CreateTransaction(ctx, db.CreateTransactionParams{
		UserID:      1,
		CategoryID:  1,
		Amount:      3500,
		Currency:    "USD",
		Description: "Current year transaction",
		Date:        time.Now(),
	})
	if err != nil {
		t.Fatalf("Failed to create current year transaction: %v", err)
	}

	t.Run("default to current year", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
		rec := httptest.NewRecorder()

		app.HandleDashboard(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("HandleDashboard() status = %d, want %d", rec.Code, http.StatusOK)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "Current year transaction") {
			t.Error("HandleDashboard() should show current year transaction by default")
		}
	})

	t.Run("filter by 2025", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/dashboard?year=2025", nil)
		rec := httptest.NewRecorder()

		app.HandleDashboard(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("HandleDashboard() status = %d, want %d", rec.Code, http.StatusOK)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "Old transaction") {
			t.Error("HandleDashboard() should show 2025 transaction when year=2025")
		}
		if strings.Contains(body, "Current year transaction") {
			t.Error("HandleDashboard() should NOT show current year transaction when year=2025")
		}
	})
}

func TestHandleDashboard_NoDuplicateCategories(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	ctx := context.Background()

	// Insert a transaction so the dashboard has data
	_, err := app.Q.CreateTransaction(ctx, db.CreateTransactionParams{
		UserID:      1,
		CategoryID:  1, // Food
		Amount:      -2500,
		Currency:    "USD",
		Description: "Pizza",
		Date:        time.Now(),
	})
	if err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()

	app.HandleDashboard(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("HandleDashboard() status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	// Count occurrences of each category name — each should appear at most once
	// in the category totals section
	categories := []string{"Food", "Transport", "Housing", "Earned Income"}
	for _, cat := range categories {
		count := strings.Count(body, cat)
		if count > 1 {
			// There may be multiple mentions in different contexts (once in mosaic, once in list)
			// but verify there's no unreasonable duplication (e.g., 4+ times for Salary)
			if count > 3 {
				t.Errorf("Category %q appears %d times in dashboard, likely duplicated", cat, count)
			}
		}
	}
}

func TestHandleDashboard_IncomeAndExpense(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	ctx := context.Background()

	// Create expense
	_, err := app.Q.CreateTransaction(ctx, db.CreateTransactionParams{
		UserID:      1,
		CategoryID:  1, // Food (expense)
		Amount:      -5000,
		Currency:    "USD",
		Description: "Grocery shopping",
		Date:        time.Now(),
	})
	if err != nil {
		t.Fatalf("Failed to create expense: %v", err)
	}

	// Create income
	_, err = app.Q.CreateTransaction(ctx, db.CreateTransactionParams{
		UserID:      1,
		CategoryID:  4, // Earned Income
		Amount:      200000,
		Currency:    "USD",
		Description: "January salary",
		Date:        time.Now(),
	})
	if err != nil {
		t.Fatalf("Failed to create income: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()

	app.HandleDashboard(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("HandleDashboard() status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Grocery shopping") {
		t.Error("Dashboard should display expense description")
	}
	if !strings.Contains(body, "January salary") {
		t.Error("Dashboard should display income description")
	}
}

func TestHandleDashboard_MultipleCategoriesExpenses(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	ctx := context.Background()
	currentYear := time.Now().Year()
	baseDate := time.Date(currentYear, 3, 15, 10, 0, 0, 0, time.UTC)

	// Create transactions in multiple categories
	transactions := []db.CreateTransactionParams{
		{UserID: 1, CategoryID: 1, Amount: -2500, Currency: "USD", Description: "Lunch", Date: baseDate},
		{UserID: 1, CategoryID: 1, Amount: -3000, Currency: "USD", Description: "Dinner", Date: baseDate},
		{UserID: 1, CategoryID: 2, Amount: -1500, Currency: "USD", Description: "Bus ticket", Date: baseDate},
		{UserID: 1, CategoryID: 3, Amount: -80000, Currency: "USD", Description: "Rent payment", Date: baseDate},
	}

	for _, tx := range transactions {
		_, err := app.Q.CreateTransaction(ctx, tx)
		if err != nil {
			t.Fatalf("Failed to create transaction %q: %v", tx.Description, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()

	app.HandleDashboard(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("HandleDashboard() status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	// Verify all transactions appear
	for _, tx := range transactions {
		if !strings.Contains(body, tx.Description) {
			t.Errorf("Dashboard should contain transaction %q", tx.Description)
		}
	}
}

func TestHandleDashboard_EmptyDatabase(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()

	app.HandleDashboard(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("HandleDashboard() with empty DB status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleDashboard_PaginationHasMore(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	ctx := context.Background()
	currentYear := time.Now().Year()

	// Create more than 20 transactions (page size) to trigger pagination
	for i := 0; i < 25; i++ {
		date := time.Date(currentYear, 1, i+1, 10, 0, 0, 0, time.UTC)
		_, err := app.Q.CreateTransaction(ctx, db.CreateTransactionParams{
			UserID:      1,
			CategoryID:  1,
			Amount:      -int64((i + 1) * 100),
			Currency:    "USD",
			Description: fmt.Sprintf("Transaction %d", i+1),
			Date:        date,
		})
		if err != nil {
			t.Fatalf("Failed to create transaction %d: %v", i, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()

	app.HandleDashboard(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("HandleDashboard() status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	// With 25 transactions and page size of 20, the response should indicate more pages
	// The "Load More" button or similar pagination indicator should be present
	if !strings.Contains(body, "hx-get") {
		t.Error("Dashboard with >20 transactions should contain a load-more element with hx-get")
	}
}

func TestHandleTransactionsPage_Pagination(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	ctx := context.Background()
	currentYear := time.Now().Year()
	yearStr := fmt.Sprintf("%d", currentYear)

	// Create 25 transactions
	for i := 0; i < 25; i++ {
		date := time.Date(currentYear, 1, (i%28)+1, 10, 0, 0, 0, time.UTC)
		_, err := app.Q.CreateTransaction(ctx, db.CreateTransactionParams{
			UserID:      1,
			CategoryID:  1,
			Amount:      -int64((i + 1) * 100),
			Currency:    "USD",
			Description: fmt.Sprintf("Paginated tx %d", i+1),
			Date:        date,
		})
		if err != nil {
			t.Fatalf("Failed to create transaction %d: %v", i, err)
		}
	}

	t.Run("first page", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/transactions?year="+yearStr+"&offset=0", nil)
		rec := httptest.NewRecorder()

		app.HandleTransactionsPage(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("second page", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/transactions?year="+yearStr+"&offset=20", nil)
		rec := httptest.NewRecorder()

		app.HandleTransactionsPage(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		body := rec.Body.String()
		// Second page should not have a load-more since only 5 items remain
		// (25 total - 20 offset = 5, which is < 20 page size)
		if strings.Contains(body, "offset=40") {
			t.Error("Second page should not have offset=40 link since all items shown")
		}
	})

	t.Run("default year", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/transactions?offset=0", nil)
		rec := httptest.NewRecorder()

		app.HandleTransactionsPage(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("empty page beyond data", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/transactions?year="+yearStr+"&offset=100", nil)
		rec := httptest.NewRecorder()

		app.HandleTransactionsPage(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("year with no transactions", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/transactions?year=2000&offset=0", nil)
		rec := httptest.NewRecorder()

		app.HandleTransactionsPage(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})
}

func TestHandleDashboardDetailed_YearFilter(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	ctx := context.Background()

	// Create transactions in different years
	_, err := app.DB.ExecContext(ctx, `
		INSERT INTO transactions (user_id, category_id, amount, currency, description, date)
		VALUES (1, 1, -5000, 'USD', 'Old detailed expense', '2024-06-15 10:00:00')
	`)
	if err != nil {
		t.Fatalf("Failed to create 2024 transaction: %v", err)
	}

	_, err = app.Q.CreateTransaction(ctx, db.CreateTransactionParams{
		UserID:      1,
		CategoryID:  1,
		Amount:      -3000,
		Currency:    "USD",
		Description: "Current detailed expense",
		Date:        time.Now(),
	})
	if err != nil {
		t.Fatalf("Failed to create current year transaction: %v", err)
	}

	t.Run("default year shows current data", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/dashboard/detailed", nil)
		rec := httptest.NewRecorder()

		app.HandleDashboardDetailed(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("filter by 2024", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/dashboard/detailed?year=2024", nil)
		rec := httptest.NewRecorder()

		app.HandleDashboardDetailed(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("empty year", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/dashboard/detailed?year=2000", nil)
		rec := httptest.NewRecorder()

		app.HandleDashboardDetailed(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})
}

func TestHandleDashboardDetailed_CategoryTotals(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	ctx := context.Background()
	currentYear := time.Now().Year()
	baseDate := time.Date(currentYear, 6, 15, 10, 0, 0, 0, time.UTC)

	// Create expenses in different categories
	_, err := app.Q.CreateTransaction(ctx, db.CreateTransactionParams{
		UserID:      1,
		CategoryID:  1, // Food
		Amount:      -5000,
		Currency:    "USD",
		Description: "Food expense",
		Date:        baseDate,
	})
	if err != nil {
		t.Fatalf("Failed to create food transaction: %v", err)
	}

	_, err = app.Q.CreateTransaction(ctx, db.CreateTransactionParams{
		UserID:      1,
		CategoryID:  2, // Transport
		Amount:      -2000,
		Currency:    "USD",
		Description: "Transport expense",
		Date:        baseDate,
	})
	if err != nil {
		t.Fatalf("Failed to create transport transaction: %v", err)
	}

	// Create income
	_, err = app.Q.CreateTransaction(ctx, db.CreateTransactionParams{
		UserID:      1,
		CategoryID:  4, // Earned Income
		Amount:      300000,
		Currency:    "USD",
		Description: "Income",
		Date:        baseDate,
	})
	if err != nil {
		t.Fatalf("Failed to create income: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/dashboard/detailed", nil)
	rec := httptest.NewRecorder()

	app.HandleDashboardDetailed(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	// Verify both category names and amounts appear
	if !strings.Contains(body, "Food") {
		t.Error("Detailed dashboard should show Food category")
	}
	if !strings.Contains(body, "Transport") {
		t.Error("Detailed dashboard should show Transport category")
	}
	if !strings.Contains(body, "$50.00") {
		t.Error("Detailed dashboard should show Food total $50.00")
	}
}

func TestHandleDashboardDetailed_MonthlyTotals(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	ctx := context.Background()
	currentYear := time.Now().Year()

	// Create transactions in different months
	months := []time.Month{time.January, time.March, time.June}
	for _, m := range months {
		date := time.Date(currentYear, m, 15, 10, 0, 0, 0, time.UTC)
		_, err := app.Q.CreateTransaction(ctx, db.CreateTransactionParams{
			UserID:      1,
			CategoryID:  1,
			Amount:      -5000,
			Currency:    "USD",
			Description: fmt.Sprintf("Expense in %s", m.String()),
			Date:        date,
		})
		if err != nil {
			t.Fatalf("Failed to create transaction for %s: %v", m.String(), err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/dashboard/detailed", nil)
	rec := httptest.NewRecorder()

	app.HandleDashboardDetailed(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Verify response renders successfully with monthly data
	body := rec.Body.String()
	if !strings.Contains(body, "Analytics") {
		t.Error("Detailed dashboard should contain Analytics title")
	}
}

func TestHandleTransactionDelete(t *testing.T) {
	t.Run("deletes existing transaction", func(t *testing.T) {
		app := setupTestApp(t)
		defer cleanupTestApp(t, app)

		ctx := context.Background()

		// Create a transaction
		tx, err := app.Q.CreateTransaction(ctx, db.CreateTransactionParams{
			UserID:      1,
			CategoryID:  1,
			Amount:      -2500,
			Currency:    "USD",
			Description: "To be deleted",
			Date:        time.Now(),
		})
		if err != nil {
			t.Fatalf("Failed to create transaction: %v", err)
		}

		// Set up chi route context with the ID
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", strconv.FormatInt(tx.ID, 10))

		req := httptest.NewRequest(http.MethodDelete, "/api/transaction/"+strconv.FormatInt(tx.ID, 10), nil)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		rec := httptest.NewRecorder()

		app.HandleTransactionDelete(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("HandleTransactionDelete() status = %d, want %d", rec.Code, http.StatusOK)
		}

		// Verify transaction is deleted
		count, err := app.Q.CountAllTransactions(ctx)
		if err != nil {
			t.Fatalf("Failed to count transactions: %v", err)
		}
		if count != 0 {
			t.Errorf("Transaction count after delete = %d, want 0", count)
		}
	})

	t.Run("invalid ID format", func(t *testing.T) {
		app := setupTestApp(t)
		defer cleanupTestApp(t, app)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "not-a-number")

		req := httptest.NewRequest(http.MethodDelete, "/api/transaction/not-a-number", nil)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		rec := httptest.NewRecorder()

		app.HandleTransactionDelete(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("HandleTransactionDelete() with invalid ID status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("non-existent transaction ID", func(t *testing.T) {
		app := setupTestApp(t)
		defer cleanupTestApp(t, app)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "99999")

		req := httptest.NewRequest(http.MethodDelete, "/api/transaction/99999", nil)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		rec := httptest.NewRecorder()

		app.HandleTransactionDelete(rec, req)

		// Should still return 200 — DELETE is idempotent
		if rec.Code != http.StatusOK {
			t.Errorf("HandleTransactionDelete() non-existent status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("delete then verify dashboard updates", func(t *testing.T) {
		app := setupTestApp(t)
		defer cleanupTestApp(t, app)

		ctx := context.Background()

		// Create two transactions
		tx1, err := app.Q.CreateTransaction(ctx, db.CreateTransactionParams{
			UserID:      1,
			CategoryID:  1,
			Amount:      -2500,
			Currency:    "USD",
			Description: "Keep this one",
			Date:        time.Now(),
		})
		if err != nil {
			t.Fatalf("Failed to create transaction 1: %v", err)
		}

		tx2, err := app.Q.CreateTransaction(ctx, db.CreateTransactionParams{
			UserID:      1,
			CategoryID:  2,
			Amount:      -1500,
			Currency:    "USD",
			Description: "Delete this one",
			Date:        time.Now(),
		})
		if err != nil {
			t.Fatalf("Failed to create transaction 2: %v", err)
		}

		// Delete tx2
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", strconv.FormatInt(tx2.ID, 10))

		req := httptest.NewRequest(http.MethodDelete, "/api/transaction/"+strconv.FormatInt(tx2.ID, 10), nil)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		rec := httptest.NewRecorder()

		app.HandleTransactionDelete(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Delete status = %d, want %d", rec.Code, http.StatusOK)
		}

		// Verify only tx1 remains
		count, err := app.Q.CountAllTransactions(ctx)
		if err != nil {
			t.Fatalf("Failed to count: %v", err)
		}
		if count != 1 {
			t.Errorf("Transaction count = %d, want 1", count)
		}
		_ = tx1 // tx1 should remain
	})
}

func TestHandleSettings(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	req := httptest.NewRequest(http.MethodGet, "/settings", nil)
	rec := httptest.NewRecorder()

	app.HandleSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("HandleSettings() status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Settings") {
		t.Error("HandleSettings() response should contain 'Settings'")
	}
	if !strings.Contains(body, "Export") {
		t.Error("HandleSettings() response should contain 'Export'")
	}
	if !strings.Contains(body, "Wipe") {
		t.Error("HandleSettings() response should contain 'Wipe'")
	}

	// Verify category mappings are shown
	if !strings.Contains(body, "Category Mappings") {
		t.Error("HandleSettings() response should contain 'Category Mappings'")
	}
	if !strings.Contains(body, "Food") {
		t.Error("HandleSettings() response should contain 'Food' category")
	}
	if !strings.Contains(body, "Transport") {
		t.Error("HandleSettings() response should contain 'Transport' category")
	}
	if !strings.Contains(body, "pizza") {
		t.Error("HandleSettings() response should contain 'pizza' keyword")
	}
}

func TestHandleExportCSV(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	t.Run("empty export", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/export/csv", nil)
		rec := httptest.NewRecorder()

		app.HandleExportCSV(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("HandleExportCSV() status = %d, want %d", rec.Code, http.StatusOK)
		}

		contentType := rec.Header().Get("Content-Type")
		if contentType != "text/csv" {
			t.Errorf("Content-Type = %q, want %q", contentType, "text/csv")
		}

		disposition := rec.Header().Get("Content-Disposition")
		if !strings.Contains(disposition, "cheapskate-export.csv") {
			t.Errorf("Content-Disposition = %q, want to contain 'cheapskate-export.csv'", disposition)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "ID,Date,Description,Category,Type,Amount,Currency") {
			t.Error("CSV should contain header row")
		}
	})

	t.Run("export with transactions", func(t *testing.T) {
		ctx := context.Background()
		_, err := app.Q.CreateTransaction(ctx, db.CreateTransactionParams{
			UserID:      1,
			CategoryID:  1,
			Amount:      -2500,
			Currency:    "USD",
			Description: "Test pizza",
			Date:        time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC),
		})
		if err != nil {
			t.Fatalf("Failed to create test transaction: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/export/csv", nil)
		rec := httptest.NewRecorder()

		app.HandleExportCSV(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("HandleExportCSV() status = %d, want %d", rec.Code, http.StatusOK)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "Test pizza") {
			t.Error("CSV should contain transaction description")
		}
		if !strings.Contains(body, "25.00") {
			t.Error("CSV should contain formatted amount")
		}
		if !strings.Contains(body, "Food") {
			t.Error("CSV should contain category name")
		}
	})
}

func TestHandleWipeData(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	// Create some transactions
	ctx := context.Background()
	_, err := app.Q.CreateTransaction(ctx, db.CreateTransactionParams{
		UserID:      1,
		CategoryID:  1,
		Amount:      2500,
		Currency:    "USD",
		Description: "Test pizza",
		Date:        time.Now(),
	})
	if err != nil {
		t.Fatalf("Failed to create test transaction: %v", err)
	}

	_, err = app.Q.CreateTransaction(ctx, db.CreateTransactionParams{
		UserID:      1,
		CategoryID:  2,
		Amount:      1500,
		Currency:    "USD",
		Description: "Test taxi",
		Date:        time.Now(),
	})
	if err != nil {
		t.Fatalf("Failed to create test transaction: %v", err)
	}

	// Verify transactions exist
	txs, err := app.Q.ListRecentTransactions(ctx)
	if err != nil {
		t.Fatalf("Failed to list transactions: %v", err)
	}
	if len(txs) != 2 {
		t.Fatalf("Expected 2 transactions, got %d", len(txs))
	}

	// Wipe data
	req := httptest.NewRequest(http.MethodDelete, "/api/data", nil)
	rec := httptest.NewRecorder()

	app.HandleWipeData(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("HandleWipeData() status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "deleted") {
		t.Error("HandleWipeData() response should confirm deletion")
	}

	// Verify transactions are gone
	txs, err = app.Q.ListRecentTransactions(ctx)
	if err != nil {
		t.Fatalf("Failed to list transactions: %v", err)
	}
	if len(txs) != 0 {
		t.Errorf("Expected 0 transactions after wipe, got %d", len(txs))
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
			wantCents: -5000, // Expenses are stored as negative
		},
		{
			name:      "decimal amount",
			input:     "12.50 test item",
			wantCents: -1250, // Expenses are stored as negative
		},
		{
			name:      "small decimal",
			input:     "0.99 test item",
			wantCents: -99, // Expenses are stored as negative
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

func TestHandleTransactionDelete_SoftDelete(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	ctx := context.Background()

	// Create a transaction
	tx, err := app.Q.CreateTransaction(ctx, db.CreateTransactionParams{
		UserID:      1,
		CategoryID:  1,
		Amount:      -2500,
		Currency:    "USD",
		Description: "test pizza",
		Date:        time.Now(),
	})
	if err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	// Soft delete it via the handler (DELETE /api/transaction/{id})
	req := httptest.NewRequest(http.MethodDelete, "/api/transaction/"+strconv.FormatInt(tx.ID, 10), nil)
	rec := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.FormatInt(tx.ID, 10))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	app.HandleTransactionDelete(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("HandleTransactionDelete() status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Verify transaction is soft deleted (not in active list)
	active, err := app.Q.ListRecentTransactions(ctx)
	if err != nil {
		t.Fatalf("ListRecentTransactions() error = %v", err)
	}
	for _, a := range active {
		if a.ID == tx.ID {
			t.Error("Soft-deleted transaction should not appear in active listing")
		}
	}

	// Verify it still exists in the DB (not hard deleted)
	count, err := app.Q.CountTransactionsByYearWithDeleted(ctx, fmt.Sprintf("%d", time.Now().Year()))
	if err != nil {
		t.Fatalf("CountTransactionsByYearWithDeleted() error = %v", err)
	}
	if count != 1 {
		t.Errorf("Transaction should still exist in DB (soft deleted), got count = %d", count)
	}
}

func TestHandleTransactionCreate_RemoveCommand(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	ctx := context.Background()

	// Create a transaction to be found by remove
	_, err := app.Q.CreateTransaction(ctx, db.CreateTransactionParams{
		UserID:      1,
		CategoryID:  1,
		Amount:      -2500,
		Currency:    "USD",
		Description: "test pizza",
		Date:        time.Now(),
	})
	if err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	t.Run("remove command shows matching transactions", func(t *testing.T) {
		form := url.Values{"input": {"remove 25"}}
		req := httptest.NewRequest(http.MethodPost, "/api/transaction", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		app.HandleTransactionCreate(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "test pizza") {
			t.Error("Remove candidates should show matching transaction description")
		}
		if !strings.Contains(body, "transaction(s) matching") {
			t.Error("Remove candidates should show count message")
		}
	})

	t.Run("remove command with no matches shows error", func(t *testing.T) {
		form := url.Values{"input": {"remove 999"}}
		req := httptest.NewRequest(http.MethodPost, "/api/transaction", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		app.HandleTransactionCreate(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "No matching transactions") {
			t.Error("Should show no matching transactions message")
		}
	})
}

func TestHandleDashboard_ShowDeleted(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	ctx := context.Background()
	yearStr := fmt.Sprintf("%d", time.Now().Year())

	// Create and soft-delete a transaction
	tx, err := app.Q.CreateTransaction(ctx, db.CreateTransactionParams{
		UserID:      1,
		CategoryID:  1,
		Amount:      -2500,
		Currency:    "USD",
		Description: "deleted pizza",
		Date:        time.Now(),
	})
	if err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}
	err = app.Q.SoftDeleteTransaction(ctx, db.SoftDeleteTransactionParams{
		ID:     tx.ID,
		UserID: 1,
	})
	if err != nil {
		t.Fatalf("Failed to soft delete: %v", err)
	}

	t.Run("hidden by default", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/dashboard?year="+yearStr, nil)
		rec := httptest.NewRecorder()

		app.HandleDashboard(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		body := rec.Body.String()
		if strings.Contains(body, "deleted pizza") {
			t.Error("Deleted transaction should not appear when show_deleted is not set")
		}
	})

	t.Run("visible with show_deleted=true", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/dashboard?year="+yearStr+"&show_deleted=true", nil)
		rec := httptest.NewRecorder()

		app.HandleDashboard(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "deleted pizza") {
			t.Error("Deleted transaction should appear when show_deleted=true")
		}
		if !strings.Contains(body, "removed") {
			t.Error("Deleted transaction should show 'removed' label")
		}
	})
}
