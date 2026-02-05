package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/calexandrepcjr/cheapskate-finance-tracker/server/db"
	_ "github.com/mattn/go-sqlite3"
)

func TestHandleStorageStatus(t *testing.T) {
	t.Run("returns zero count for empty database", func(t *testing.T) {
		app := setupTestApp(t)
		defer cleanupTestApp(t, app)

		req := httptest.NewRequest(http.MethodGet, "/api/storage/status", nil)
		rec := httptest.NewRecorder()

		app.HandleStorageStatus(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("HandleStorageStatus() status = %d, want %d", rec.Code, http.StatusOK)
		}

		var resp StorageStatusResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp.TransactionCount != 0 {
			t.Errorf("TransactionCount = %d, want 0", resp.TransactionCount)
		}

		if resp.ServerTime == "" {
			t.Error("ServerTime should not be empty")
		}
	})

	t.Run("returns correct count with transactions", func(t *testing.T) {
		app := setupTestApp(t)
		defer cleanupTestApp(t, app)

		ctx := context.Background()
		_, err := app.Q.CreateTransaction(ctx, db.CreateTransactionParams{
			UserID:      1,
			CategoryID:  1,
			Amount:      -2500,
			Currency:    "USD",
			Description: "Test pizza",
			Date:        time.Now(),
		})
		if err != nil {
			t.Fatalf("Failed to create transaction: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/storage/status", nil)
		rec := httptest.NewRecorder()

		app.HandleStorageStatus(rec, req)

		var resp StorageStatusResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp.TransactionCount != 1 {
			t.Errorf("TransactionCount = %d, want 1", resp.TransactionCount)
		}
	})
}

func TestHandleStorageExport(t *testing.T) {
	t.Run("exports empty data for year with no transactions", func(t *testing.T) {
		app := setupTestApp(t)
		defer cleanupTestApp(t, app)

		req := httptest.NewRequest(http.MethodGet, "/api/storage/export?year=2026", nil)
		rec := httptest.NewRecorder()

		app.HandleStorageExport(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("HandleStorageExport() status = %d, want %d", rec.Code, http.StatusOK)
		}

		var resp StorageExportResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(resp.Transactions) != 0 {
			t.Errorf("Transactions count = %d, want 0", len(resp.Transactions))
		}

		// Should always export categories
		if len(resp.Categories) != 4 {
			t.Errorf("Categories count = %d, want 4", len(resp.Categories))
		}

		if resp.Year != "2026" {
			t.Errorf("Year = %q, want %q", resp.Year, "2026")
		}

		if resp.ExportedAt == "" {
			t.Error("ExportedAt should not be empty")
		}
	})

	t.Run("exports transactions for the correct year", func(t *testing.T) {
		app := setupTestApp(t)
		defer cleanupTestApp(t, app)

		ctx := context.Background()

		// Create transaction in 2025
		_, err := app.Q.CreateTransaction(ctx, db.CreateTransactionParams{
			UserID:      1,
			CategoryID:  1,
			Amount:      -2500,
			Currency:    "USD",
			Description: "2025 pizza",
			Date:        time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC),
		})
		if err != nil {
			t.Fatalf("Failed to create 2025 transaction: %v", err)
		}

		// Create transaction in 2026
		_, err = app.Q.CreateTransaction(ctx, db.CreateTransactionParams{
			UserID:      1,
			CategoryID:  2,
			Amount:      -1500,
			Currency:    "USD",
			Description: "2026 taxi",
			Date:        time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC),
		})
		if err != nil {
			t.Fatalf("Failed to create 2026 transaction: %v", err)
		}

		// Export 2026
		req := httptest.NewRequest(http.MethodGet, "/api/storage/export?year=2026", nil)
		rec := httptest.NewRecorder()

		app.HandleStorageExport(rec, req)

		var resp StorageExportResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(resp.Transactions) != 1 {
			t.Fatalf("Transactions count = %d, want 1", len(resp.Transactions))
		}

		if resp.Transactions[0].Description != "2026 taxi" {
			t.Errorf("Transaction description = %q, want %q", resp.Transactions[0].Description, "2026 taxi")
		}

		if resp.Transactions[0].Amount != -1500 {
			t.Errorf("Transaction amount = %d, want -1500", resp.Transactions[0].Amount)
		}

		if resp.Transactions[0].CategoryName != "Transport" {
			t.Errorf("CategoryName = %q, want %q", resp.Transactions[0].CategoryName, "Transport")
		}
	})

	t.Run("defaults to current year when no year param", func(t *testing.T) {
		app := setupTestApp(t)
		defer cleanupTestApp(t, app)

		req := httptest.NewRequest(http.MethodGet, "/api/storage/export", nil)
		rec := httptest.NewRecorder()

		app.HandleStorageExport(rec, req)

		var resp StorageExportResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		currentYear := time.Now().Format("2006")
		if resp.Year != currentYear {
			t.Errorf("Year = %q, want %q", resp.Year, currentYear)
		}
	})

	t.Run("exports category fields correctly", func(t *testing.T) {
		app := setupTestApp(t)
		defer cleanupTestApp(t, app)

		req := httptest.NewRequest(http.MethodGet, "/api/storage/export?year=2026", nil)
		rec := httptest.NewRecorder()

		app.HandleStorageExport(rec, req)

		var resp StorageExportResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Find Food category
		var food *StorageCategory
		for i := range resp.Categories {
			if resp.Categories[i].Name == "Food" {
				food = &resp.Categories[i]
				break
			}
		}

		if food == nil {
			t.Fatal("Food category not found in export")
		}

		if food.Type != "expense" {
			t.Errorf("Food.Type = %q, want %q", food.Type, "expense")
		}

		if food.Color != "#FF5733" {
			t.Errorf("Food.Color = %q, want %q", food.Color, "#FF5733")
		}
	})
}

func TestHandleStorageImport(t *testing.T) {
	t.Run("imports transactions into empty database", func(t *testing.T) {
		app := setupTestApp(t)
		defer cleanupTestApp(t, app)

		importReq := StorageImportRequest{
			Transactions: []StorageTransaction{
				{
					ID:           100,
					Amount:       -2500,
					Currency:     "USD",
					Description:  "Imported pizza",
					Date:         "2026-01-15T10:00:00Z",
					CategoryName: "Food",
					CategoryType: "expense",
				},
				{
					ID:           101,
					Amount:       -1500,
					Currency:     "USD",
					Description:  "Imported taxi",
					Date:         "2026-01-20T10:00:00Z",
					CategoryName: "Transport",
					CategoryType: "expense",
				},
			},
		}

		body, _ := json.Marshal(importReq)
		req := httptest.NewRequest(http.MethodPost, "/api/storage/import", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		app.HandleStorageImport(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("HandleStorageImport() status = %d, want %d", rec.Code, http.StatusOK)
		}

		var resp StorageImportResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp.Imported != 2 {
			t.Errorf("Imported = %d, want 2", resp.Imported)
		}

		if resp.Errors != 0 {
			t.Errorf("Errors = %d, want 0", resp.Errors)
		}

		// Verify transactions were created
		ctx := context.Background()
		count, err := app.Q.CountAllTransactions(ctx)
		if err != nil {
			t.Fatalf("Failed to count transactions: %v", err)
		}
		if count != 2 {
			t.Errorf("Transaction count = %d, want 2", count)
		}
	})

	t.Run("skips import when database already has transactions", func(t *testing.T) {
		app := setupTestApp(t)
		defer cleanupTestApp(t, app)

		// Pre-populate with a transaction
		ctx := context.Background()
		_, err := app.Q.CreateTransaction(ctx, db.CreateTransactionParams{
			UserID:      1,
			CategoryID:  1,
			Amount:      -1000,
			Currency:    "USD",
			Description: "Existing transaction",
			Date:        time.Now(),
		})
		if err != nil {
			t.Fatalf("Failed to create existing transaction: %v", err)
		}

		importReq := StorageImportRequest{
			Transactions: []StorageTransaction{
				{
					ID:           200,
					Amount:       -5000,
					Currency:     "USD",
					Description:  "Should be skipped",
					Date:         "2026-02-01T10:00:00Z",
					CategoryName: "Food",
					CategoryType: "expense",
				},
			},
		}

		body, _ := json.Marshal(importReq)
		req := httptest.NewRequest(http.MethodPost, "/api/storage/import", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		app.HandleStorageImport(rec, req)

		var resp StorageImportResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp.Imported != 0 {
			t.Errorf("Imported = %d, want 0 (should be skipped)", resp.Imported)
		}

		if resp.Skipped != 1 {
			t.Errorf("Skipped = %d, want 1", resp.Skipped)
		}

		// Verify only the original transaction exists
		count, err := app.Q.CountAllTransactions(ctx)
		if err != nil {
			t.Fatalf("Failed to count transactions: %v", err)
		}
		if count != 1 {
			t.Errorf("Transaction count = %d, want 1 (only original)", count)
		}
	})

	t.Run("handles invalid request body", func(t *testing.T) {
		app := setupTestApp(t)
		defer cleanupTestApp(t, app)

		req := httptest.NewRequest(http.MethodPost, "/api/storage/import", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		app.HandleStorageImport(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("HandleStorageImport() status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("handles empty import gracefully", func(t *testing.T) {
		app := setupTestApp(t)
		defer cleanupTestApp(t, app)

		importReq := StorageImportRequest{
			Transactions: []StorageTransaction{},
		}

		body, _ := json.Marshal(importReq)
		req := httptest.NewRequest(http.MethodPost, "/api/storage/import", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		app.HandleStorageImport(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("HandleStorageImport() status = %d, want %d", rec.Code, http.StatusOK)
		}

		var resp StorageImportResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp.Imported != 0 {
			t.Errorf("Imported = %d, want 0", resp.Imported)
		}
	})

	t.Run("handles unknown category with fallback", func(t *testing.T) {
		app := setupTestApp(t)
		defer cleanupTestApp(t, app)

		importReq := StorageImportRequest{
			Transactions: []StorageTransaction{
				{
					ID:           300,
					Amount:       -3000,
					Currency:     "USD",
					Description:  "Unknown cat tx",
					Date:         "2026-03-01T10:00:00Z",
					CategoryName: "NonExistentCategory",
					CategoryType: "expense",
				},
			},
		}

		body, _ := json.Marshal(importReq)
		req := httptest.NewRequest(http.MethodPost, "/api/storage/import", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		app.HandleStorageImport(rec, req)

		var resp StorageImportResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Should fall back to first category
		if resp.Imported != 1 {
			t.Errorf("Imported = %d, want 1 (should use fallback category)", resp.Imported)
		}
	})

	t.Run("handles invalid date format", func(t *testing.T) {
		app := setupTestApp(t)
		defer cleanupTestApp(t, app)

		importReq := StorageImportRequest{
			Transactions: []StorageTransaction{
				{
					ID:           400,
					Amount:       -1000,
					Currency:     "USD",
					Description:  "Bad date tx",
					Date:         "not-a-date",
					CategoryName: "Food",
					CategoryType: "expense",
				},
			},
		}

		body, _ := json.Marshal(importReq)
		req := httptest.NewRequest(http.MethodPost, "/api/storage/import", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		app.HandleStorageImport(rec, req)

		var resp StorageImportResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp.Errors != 1 {
			t.Errorf("Errors = %d, want 1", resp.Errors)
		}

		if resp.Imported != 0 {
			t.Errorf("Imported = %d, want 0", resp.Imported)
		}
	})

	t.Run("preserves amount sign from IndexedDB data", func(t *testing.T) {
		app := setupTestApp(t)
		defer cleanupTestApp(t, app)

		importReq := StorageImportRequest{
			Transactions: []StorageTransaction{
				{
					ID:           500,
					Amount:       -5000,
					Currency:     "USD",
					Description:  "Negative amount expense",
					Date:         "2026-04-01T10:00:00Z",
					CategoryName: "Food",
					CategoryType: "expense",
				},
				{
					ID:           501,
					Amount:       100000,
					Currency:     "USD",
					Description:  "Positive income",
					Date:         "2026-04-01T10:00:00Z",
					CategoryName: "Earned Income",
					CategoryType: "income",
				},
			},
		}

		body, _ := json.Marshal(importReq)
		req := httptest.NewRequest(http.MethodPost, "/api/storage/import", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		app.HandleStorageImport(rec, req)

		var resp StorageImportResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp.Imported != 2 {
			t.Errorf("Imported = %d, want 2", resp.Imported)
		}

		// Verify amounts are preserved correctly
		ctx := context.Background()
		txs, err := app.Q.ListRecentTransactions(ctx)
		if err != nil {
			t.Fatalf("Failed to list transactions: %v", err)
		}

		if len(txs) != 2 {
			t.Fatalf("Transaction count = %d, want 2", len(txs))
		}

		// Transactions are ordered by date DESC, both same date, check by description
		for _, tx := range txs {
			if tx.Description == "Negative amount expense" && tx.Amount != -5000 {
				t.Errorf("Expense amount = %d, want -5000", tx.Amount)
			}
			if tx.Description == "Positive income" && tx.Amount != 100000 {
				t.Errorf("Income amount = %d, want 100000", tx.Amount)
			}
		}
	})
}

func TestHandleStorageStatus_ContentType(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	req := httptest.NewRequest(http.MethodGet, "/api/storage/status", nil)
	rec := httptest.NewRecorder()

	app.HandleStorageStatus(rec, req)

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}
}

func TestHandleStorageStatus_MultipleTransactions(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	ctx := context.Background()

	// Create multiple transactions
	for i := 0; i < 5; i++ {
		_, err := app.Q.CreateTransaction(ctx, db.CreateTransactionParams{
			UserID:      1,
			CategoryID:  1,
			Amount:      -int64((i + 1) * 1000),
			Currency:    "USD",
			Description: fmt.Sprintf("Transaction %d", i+1),
			Date:        time.Now(),
		})
		if err != nil {
			t.Fatalf("Failed to create transaction %d: %v", i, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/storage/status", nil)
	rec := httptest.NewRecorder()

	app.HandleStorageStatus(rec, req)

	var resp StorageStatusResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.TransactionCount != 5 {
		t.Errorf("TransactionCount = %d, want 5", resp.TransactionCount)
	}
}

func TestHandleStorageStatus_ServerTimeFormat(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	req := httptest.NewRequest(http.MethodGet, "/api/storage/status", nil)
	rec := httptest.NewRecorder()

	app.HandleStorageStatus(rec, req)

	var resp StorageStatusResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify server time is valid RFC3339
	_, err := time.Parse(time.RFC3339, resp.ServerTime)
	if err != nil {
		t.Errorf("ServerTime %q is not valid RFC3339: %v", resp.ServerTime, err)
	}
}

func TestHandleStorageExport_MultipleTransactionsMultipleCategories(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	ctx := context.Background()

	// Create transactions across categories
	txParams := []db.CreateTransactionParams{
		{UserID: 1, CategoryID: 1, Amount: -2500, Currency: "USD", Description: "Pizza", Date: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)},
		{UserID: 1, CategoryID: 1, Amount: -3000, Currency: "USD", Description: "Sushi", Date: time.Date(2026, 2, 10, 10, 0, 0, 0, time.UTC)},
		{UserID: 1, CategoryID: 2, Amount: -1500, Currency: "USD", Description: "Bus", Date: time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)},
		{UserID: 1, CategoryID: 3, Amount: -80000, Currency: "USD", Description: "Rent", Date: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)},
		{UserID: 1, CategoryID: 4, Amount: 200000, Currency: "USD", Description: "Salary", Date: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)},
	}

	for _, tx := range txParams {
		_, err := app.Q.CreateTransaction(ctx, tx)
		if err != nil {
			t.Fatalf("Failed to create transaction %q: %v", tx.Description, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/storage/export?year=2026", nil)
	rec := httptest.NewRecorder()

	app.HandleStorageExport(rec, req)

	var resp StorageExportResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(resp.Transactions) != 5 {
		t.Errorf("Transactions count = %d, want 5", len(resp.Transactions))
	}

	// Verify all descriptions present
	descMap := make(map[string]bool)
	for _, tx := range resp.Transactions {
		descMap[tx.Description] = true
	}
	for _, p := range txParams {
		if !descMap[p.Description] {
			t.Errorf("Missing transaction %q in export", p.Description)
		}
	}

	// Verify categories are exported
	if len(resp.Categories) != 4 {
		t.Errorf("Categories count = %d, want 4", len(resp.Categories))
	}
}

func TestHandleStorageExport_IncomeCategories(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	req := httptest.NewRequest(http.MethodGet, "/api/storage/export?year=2026", nil)
	rec := httptest.NewRecorder()

	app.HandleStorageExport(rec, req)

	var resp StorageExportResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Find income category
	var incomeCategory *StorageCategory
	for i := range resp.Categories {
		if resp.Categories[i].Type == "income" {
			incomeCategory = &resp.Categories[i]
			break
		}
	}

	if incomeCategory == nil {
		t.Fatal("No income category found in export")
	}

	if incomeCategory.Name != "Earned Income" {
		t.Errorf("Income category name = %q, want %q", incomeCategory.Name, "Earned Income")
	}
}

func TestHandleStorageExport_TransactionFields(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	ctx := context.Background()
	date := time.Date(2026, 5, 20, 14, 30, 0, 0, time.UTC)
	_, err := app.Q.CreateTransaction(ctx, db.CreateTransactionParams{
		UserID:      1,
		CategoryID:  1,
		Amount:      -4299,
		Currency:    "USD",
		Description: "Detailed field check",
		Date:        date,
	})
	if err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/storage/export?year=2026", nil)
	rec := httptest.NewRecorder()

	app.HandleStorageExport(rec, req)

	var resp StorageExportResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(resp.Transactions) != 1 {
		t.Fatalf("Expected 1 transaction, got %d", len(resp.Transactions))
	}

	tx := resp.Transactions[0]
	if tx.Amount != -4299 {
		t.Errorf("Amount = %d, want -4299", tx.Amount)
	}
	if tx.Currency != "USD" {
		t.Errorf("Currency = %q, want %q", tx.Currency, "USD")
	}
	if tx.Description != "Detailed field check" {
		t.Errorf("Description = %q, want %q", tx.Description, "Detailed field check")
	}
	if tx.CategoryName != "Food" {
		t.Errorf("CategoryName = %q, want %q", tx.CategoryName, "Food")
	}
	if tx.Date == "" {
		t.Error("Date should not be empty")
	}
	// Verify date is parseable RFC3339
	parsedDate, err := time.Parse(time.RFC3339, tx.Date)
	if err != nil {
		t.Errorf("Date %q is not valid RFC3339: %v", tx.Date, err)
	}
	if parsedDate.Year() != 2026 || parsedDate.Month() != 5 || parsedDate.Day() != 20 {
		t.Errorf("Date = %v, want 2026-05-20", parsedDate)
	}
}

func TestHandleStorageExport_ContentType(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	req := httptest.NewRequest(http.MethodGet, "/api/storage/export?year=2026", nil)
	rec := httptest.NewRecorder()

	app.HandleStorageExport(rec, req)

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}
}

func TestHandleStorageImport_MultipleCategories(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	importReq := StorageImportRequest{
		Transactions: []StorageTransaction{
			{ID: 1, Amount: -2500, Currency: "USD", Description: "Food item", Date: "2026-01-15T10:00:00Z", CategoryName: "Food", CategoryType: "expense"},
			{ID: 2, Amount: -1500, Currency: "USD", Description: "Bus ride", Date: "2026-01-16T10:00:00Z", CategoryName: "Transport", CategoryType: "expense"},
			{ID: 3, Amount: -80000, Currency: "USD", Description: "Monthly rent", Date: "2026-01-01T10:00:00Z", CategoryName: "Housing", CategoryType: "expense"},
			{ID: 4, Amount: 200000, Currency: "USD", Description: "Monthly pay", Date: "2026-01-01T10:00:00Z", CategoryName: "Earned Income", CategoryType: "income"},
		},
	}

	body, _ := json.Marshal(importReq)
	req := httptest.NewRequest(http.MethodPost, "/api/storage/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.HandleStorageImport(rec, req)

	var resp StorageImportResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Imported != 4 {
		t.Errorf("Imported = %d, want 4", resp.Imported)
	}
	if resp.Errors != 0 {
		t.Errorf("Errors = %d, want 0", resp.Errors)
	}

	// Verify all transactions with correct categories
	ctx := context.Background()
	txs, err := app.Q.ListRecentTransactions(ctx)
	if err != nil {
		t.Fatalf("Failed to list transactions: %v", err)
	}
	if len(txs) != 4 {
		t.Fatalf("Transaction count = %d, want 4", len(txs))
	}

	catMap := make(map[string]bool)
	for _, tx := range txs {
		catMap[tx.CategoryName] = true
	}
	expectedCats := []string{"Food", "Transport", "Housing", "Earned Income"}
	for _, cat := range expectedCats {
		if !catMap[cat] {
			t.Errorf("Missing category %q in imported transactions", cat)
		}
	}
}

func TestHandleStorageImport_CurrencyPreservation(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	importReq := StorageImportRequest{
		Transactions: []StorageTransaction{
			{ID: 1, Amount: -5000, Currency: "USD", Description: "USD transaction", Date: "2026-01-15T10:00:00Z", CategoryName: "Food", CategoryType: "expense"},
		},
	}

	body, _ := json.Marshal(importReq)
	req := httptest.NewRequest(http.MethodPost, "/api/storage/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.HandleStorageImport(rec, req)

	var resp StorageImportResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Imported != 1 {
		t.Fatalf("Imported = %d, want 1", resp.Imported)
	}

	// Verify currency is preserved
	ctx := context.Background()
	txs, err := app.Q.ListRecentTransactions(ctx)
	if err != nil {
		t.Fatalf("Failed to list transactions: %v", err)
	}
	if len(txs) != 1 {
		t.Fatalf("Transaction count = %d, want 1", len(txs))
	}
	if txs[0].Currency != "USD" {
		t.Errorf("Currency = %q, want %q", txs[0].Currency, "USD")
	}
}

func TestHandleStorageImport_LargeImport(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	// Import 50 transactions
	transactions := make([]StorageTransaction, 50)
	for i := 0; i < 50; i++ {
		transactions[i] = StorageTransaction{
			ID:           int64(i + 1),
			Amount:       -int64((i + 1) * 100),
			Currency:     "USD",
			Description:  fmt.Sprintf("Bulk import item %d", i+1),
			Date:         fmt.Sprintf("2026-01-%02dT10:00:00Z", (i%28)+1),
			CategoryName: "Food",
			CategoryType: "expense",
		}
	}

	importReq := StorageImportRequest{Transactions: transactions}
	body, _ := json.Marshal(importReq)
	req := httptest.NewRequest(http.MethodPost, "/api/storage/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.HandleStorageImport(rec, req)

	var resp StorageImportResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Imported != 50 {
		t.Errorf("Imported = %d, want 50", resp.Imported)
	}
	if resp.Errors != 0 {
		t.Errorf("Errors = %d, want 0", resp.Errors)
	}

	// Verify count
	ctx := context.Background()
	count, err := app.Q.CountAllTransactions(ctx)
	if err != nil {
		t.Fatalf("Failed to count: %v", err)
	}
	if count != 50 {
		t.Errorf("Transaction count = %d, want 50", count)
	}
}

func TestHandleStorageImport_ContentType(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	importReq := StorageImportRequest{Transactions: []StorageTransaction{}}
	body, _ := json.Marshal(importReq)
	req := httptest.NewRequest(http.MethodPost, "/api/storage/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.HandleStorageImport(rec, req)

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}
}

func TestHandleStorageImport_NilTransactions(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	// Send request with no transactions field
	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/api/storage/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.HandleStorageImport(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp StorageImportResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Imported != 0 {
		t.Errorf("Imported = %d, want 0", resp.Imported)
	}
}

func TestHandleStorageImport_MixedValidInvalid(t *testing.T) {
	app := setupTestApp(t)
	defer cleanupTestApp(t, app)

	importReq := StorageImportRequest{
		Transactions: []StorageTransaction{
			{ID: 1, Amount: -2500, Currency: "USD", Description: "Valid tx", Date: "2026-01-15T10:00:00Z", CategoryName: "Food", CategoryType: "expense"},
			{ID: 2, Amount: -1000, Currency: "USD", Description: "Bad date tx", Date: "invalid-date", CategoryName: "Food", CategoryType: "expense"},
			{ID: 3, Amount: -3000, Currency: "USD", Description: "Another valid tx", Date: "2026-02-20T10:00:00Z", CategoryName: "Transport", CategoryType: "expense"},
		},
	}

	body, _ := json.Marshal(importReq)
	req := httptest.NewRequest(http.MethodPost, "/api/storage/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.HandleStorageImport(rec, req)

	var resp StorageImportResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Imported != 2 {
		t.Errorf("Imported = %d, want 2", resp.Imported)
	}
	if resp.Errors != 1 {
		t.Errorf("Errors = %d, want 1", resp.Errors)
	}
}

func TestStorageRoundTrip(t *testing.T) {
	t.Run("export then import preserves data", func(t *testing.T) {
		// Create app with data
		app1 := setupTestApp(t)
		defer cleanupTestApp(t, app1)

		ctx := context.Background()
		_, err := app1.Q.CreateTransaction(ctx, db.CreateTransactionParams{
			UserID:      1,
			CategoryID:  1,
			Amount:      -4200,
			Currency:    "USD",
			Description: "Roundtrip pizza",
			Date:        time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
		})
		if err != nil {
			t.Fatalf("Failed to create transaction: %v", err)
		}

		_, err = app1.Q.CreateTransaction(ctx, db.CreateTransactionParams{
			UserID:      1,
			CategoryID:  4,
			Amount:      500000,
			Currency:    "USD",
			Description: "Roundtrip salary",
			Date:        time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
		})
		if err != nil {
			t.Fatalf("Failed to create transaction: %v", err)
		}

		// Export from app1
		exportReq := httptest.NewRequest(http.MethodGet, "/api/storage/export?year=2026", nil)
		exportRec := httptest.NewRecorder()
		app1.HandleStorageExport(exportRec, exportReq)

		var exportResp StorageExportResponse
		if err := json.NewDecoder(exportRec.Body).Decode(&exportResp); err != nil {
			t.Fatalf("Failed to decode export response: %v", err)
		}

		if len(exportResp.Transactions) != 2 {
			t.Fatalf("Expected 2 exported transactions, got %d", len(exportResp.Transactions))
		}

		// Import into fresh app2
		app2 := setupTestApp(t)
		defer cleanupTestApp(t, app2)

		importReq := StorageImportRequest{
			Transactions: exportResp.Transactions,
		}
		importBody, _ := json.Marshal(importReq)

		importHTTPReq := httptest.NewRequest(http.MethodPost, "/api/storage/import", bytes.NewReader(importBody))
		importHTTPReq.Header.Set("Content-Type", "application/json")
		importRec := httptest.NewRecorder()

		app2.HandleStorageImport(importRec, importHTTPReq)

		var importResp StorageImportResponse
		if err := json.NewDecoder(importRec.Body).Decode(&importResp); err != nil {
			t.Fatalf("Failed to decode import response: %v", err)
		}

		if importResp.Imported != 2 {
			t.Errorf("Imported = %d, want 2", importResp.Imported)
		}

		// Verify data in app2
		txs, err := app2.Q.ListRecentTransactions(ctx)
		if err != nil {
			t.Fatalf("Failed to list transactions: %v", err)
		}

		if len(txs) != 2 {
			t.Fatalf("Transaction count in app2 = %d, want 2", len(txs))
		}

		// Check descriptions exist (order may differ due to date sorting)
		descriptions := make(map[string]bool)
		for _, tx := range txs {
			descriptions[tx.Description] = true
		}
		if !descriptions["Roundtrip pizza"] {
			t.Error("Missing 'Roundtrip pizza' transaction after import")
		}
		if !descriptions["Roundtrip salary"] {
			t.Error("Missing 'Roundtrip salary' transaction after import")
		}
	})
}
