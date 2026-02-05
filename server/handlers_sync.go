package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/calexandrepcjr/cheapskate-finance-tracker/server/db"
)

// SyncTransaction represents a transaction in the sync JSON format
type SyncTransaction struct {
	ID           int64  `json:"id"`
	Amount       int64  `json:"amount"`
	Currency     string `json:"currency"`
	Description  string `json:"description"`
	Date         string `json:"date"`
	CategoryName string `json:"category_name"`
	CategoryType string `json:"category_type"`
	CreatedAt    string `json:"created_at"`
}

// SyncCategory represents a category in the sync JSON format
type SyncCategory struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	Icon  string `json:"icon"`
	Color string `json:"color"`
}

// SyncStatusResponse is the response for the sync status endpoint
type SyncStatusResponse struct {
	TransactionCount int64  `json:"transaction_count"`
	ServerTime       string `json:"server_time"`
}

// SyncExportResponse is the response for the sync export endpoint
type SyncExportResponse struct {
	Transactions []SyncTransaction `json:"transactions"`
	Categories   []SyncCategory    `json:"categories"`
	Year         string            `json:"year"`
	ExportedAt   string            `json:"exported_at"`
}

// SyncImportRequest is the request body for the sync import endpoint
type SyncImportRequest struct {
	Transactions []SyncTransaction `json:"transactions"`
}

// SyncImportResponse is the response for the sync import endpoint
type SyncImportResponse struct {
	Imported int `json:"imported"`
	Skipped  int `json:"skipped"`
	Errors   int `json:"errors"`
}

// HandleSyncStatus returns the current transaction count so the client
// can determine whether the database needs reconstruction from IndexedDB.
func (app *Application) HandleSyncStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	count, err := app.Q.CountAllTransactions(ctx)
	if err != nil {
		http.Error(w, "Failed to count transactions", http.StatusInternalServerError)
		return
	}

	resp := SyncStatusResponse{
		TransactionCount: count,
		ServerTime:       time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleSyncExport returns all transactions and categories for a given year
// as JSON, for the client to store in IndexedDB.
func (app *Application) HandleSyncExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	yearParam := r.URL.Query().Get("year")
	if yearParam == "" {
		yearParam = fmt.Sprintf("%d", time.Now().Year())
	}

	// Fetch transactions for the year
	txRows, err := app.Q.ListTransactionsByYear(ctx, yearParam)
	if err != nil {
		http.Error(w, "Failed to load transactions", http.StatusInternalServerError)
		return
	}

	transactions := make([]SyncTransaction, 0, len(txRows))
	for _, tx := range txRows {
		createdAt := ""
		if tx.CreatedAt.Valid {
			createdAt = tx.CreatedAt.Time.UTC().Format(time.RFC3339)
		}
		transactions = append(transactions, SyncTransaction{
			ID:           tx.ID,
			Amount:       tx.Amount,
			Currency:     tx.Currency,
			Description:  tx.Description,
			Date:         tx.Date.UTC().Format(time.RFC3339),
			CategoryName: tx.CategoryName,
			CategoryType: "",
			CreatedAt:    createdAt,
		})
	}

	// Fetch categories
	catRows, err := app.Q.ListCategories(ctx)
	if err != nil {
		http.Error(w, "Failed to load categories", http.StatusInternalServerError)
		return
	}

	categories := make([]SyncCategory, 0, len(catRows))
	for _, cat := range catRows {
		icon := ""
		if cat.Icon.Valid {
			icon = cat.Icon.String
		}
		color := ""
		if cat.Color.Valid {
			color = cat.Color.String
		}
		categories = append(categories, SyncCategory{
			ID:    cat.ID,
			Name:  cat.Name,
			Type:  cat.Type,
			Icon:  icon,
			Color: color,
		})
	}

	resp := SyncExportResponse{
		Transactions: transactions,
		Categories:   categories,
		Year:         yearParam,
		ExportedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleSyncImport accepts transactions from IndexedDB and imports them
// into the SQLite database. Used to reconstruct data after DB deletion.
func (app *Application) HandleSyncImport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req SyncImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Check if DB already has transactions - avoid duplicate imports
	count, err := app.Q.CountAllTransactions(ctx)
	if err != nil {
		http.Error(w, "Failed to check transaction count", http.StatusInternalServerError)
		return
	}
	if count > 0 {
		resp := SyncImportResponse{Imported: 0, Skipped: len(req.Transactions), Errors: 0}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}

	userID := int64(1)
	imported := 0
	skipped := 0
	errors := 0

	for _, syncTx := range req.Transactions {
		// Resolve category by name
		cat, err := app.Q.GetCategoryByName(ctx, syncTx.CategoryName)
		if err != nil {
			// Try to find a fallback category
			cats, catErr := app.Q.ListCategories(ctx)
			if catErr != nil || len(cats) == 0 {
				log.Printf("Sync import: could not resolve category %q: %v", syncTx.CategoryName, err)
				errors++
				continue
			}
			cat = cats[0]
		}

		// Parse date
		txDate, err := time.Parse(time.RFC3339, syncTx.Date)
		if err != nil {
			log.Printf("Sync import: could not parse date %q: %v", syncTx.Date, err)
			errors++
			continue
		}

		_, err = app.Q.CreateTransaction(ctx, db.CreateTransactionParams{
			UserID:      userID,
			CategoryID:  cat.ID,
			Amount:      syncTx.Amount,
			Currency:    syncTx.Currency,
			Description: syncTx.Description,
			Date:        txDate,
		})
		if err != nil {
			log.Printf("Sync import: failed to create transaction: %v", err)
			errors++
			continue
		}

		imported++
	}

	skipped = len(req.Transactions) - imported - errors

	resp := SyncImportResponse{
		Imported: imported,
		Skipped:  skipped,
		Errors:   errors,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
