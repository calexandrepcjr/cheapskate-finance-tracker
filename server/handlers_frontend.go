package main

import (
	"net/http"
	"strconv"
	"time"

	"github.com/calexandrepcjr/cheapskate-finance-tracker/client/templates"
	"github.com/calexandrepcjr/cheapskate-finance-tracker/server/db"
	"github.com/go-chi/chi/v5"
)

func (app *Application) HandleHome(w http.ResponseWriter, r *http.Request) {
	templates.Home().Render(r.Context(), w)
}

func (app *Application) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Fetch recent transactions
	txs, err := app.Q.ListRecentTransactions(ctx)
	if err != nil {
		http.Error(w, "Failed to load dashboard: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 2. Fetch category stats for current month
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	rawStats, err := app.Q.GetCategoryStats(ctx, startOfMonth)
	if err != nil {
		// Log error but continue with empty stats
		rawStats = []db.GetCategoryStatsRow{}
	}

	// 3. Transform to map for easier lookup in template
	// Map Category Name -> Total Amount (in cents)
	statsMap := make(map[string]int64)
	for _, s := range rawStats {
		// Store absolute value for display in the grid if it's an expense
		// The query returns sum(amount), which might be negative for expenses
		// The query returns sum(amount), which might be negative for expenses
		var val int64
		if s.TotalAmount != nil {
			switch v := s.TotalAmount.(type) {
			case int64:
				val = v
			case float64:
				val = int64(v)
			default:
				val = 0
			}
		}

		// NOTE: We keep expenses negative so the UI knows they are expenses
		statsMap[s.Name] = val
	}

	templates.Dashboard(txs, statsMap).Render(ctx, w)
}

func (app *Application) HandleTransactionCreate(w http.ResponseWriter, r *http.Request) {
	input := r.FormValue("input")

	// 1. Parse
	parsed, err := ParseTransaction(input)
	if err != nil {
		templates.TransactionError("Could not understand that. Try '50 pizza'").Render(r.Context(), w)
		return
	}

	// 2. Resolve Category
	// For now, we query by name. If not found, use a default ID (e.g. 1) or create one.
	var catID int64
	var catType string

	cat, err := app.Q.GetCategoryByName(r.Context(), parsed.Category)
	if err != nil {
		// Fallback to first category
		cats, _ := app.Q.ListCategories(r.Context())
		if len(cats) > 0 {
			catID = cats[0].ID
			catType = cats[0].Type
		} else {
			catID = 1 // Risky if empty
			catType = "expense"
		}
	} else {
		catID = cat.ID
		catType = cat.Type
	}

	// 2.5 Negate amount if generic expense
	// If the user typed "100 pizza", we parse 10000.
	// If category is expense, store -10000.
	// If category is income, keep 10000.
	finalAmount := parsed.Amount
	if catType == "expense" && finalAmount > 0 {
		finalAmount = -finalAmount
	}

	// 3. User ID (Hardcoded for single user MVP/Monolith)
	userID := int64(1)

	// 4. Insert
	_, err = app.Q.CreateTransaction(r.Context(), db.CreateTransactionParams{
		UserID:      userID,
		CategoryID:  catID,
		Amount:      finalAmount,
		Currency:    "USD",
		Description: parsed.Description,
		Date:        time.Now(),
	})
	if err != nil {
		templates.TransactionError("Failed to save: "+err.Error()).Render(r.Context(), w)
		return
	}

	// 5. Render Success
	displayAmt := formatMoney(finalAmount)
	templates.TransactionSuccess(displayAmt, parsed.Description, finalAmount < 0).Render(r.Context(), w)
}

func (app *Application) HandleTransactionDelete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	// Hardcoded user ID for now
	userID := int64(1)

	err = app.Q.DeleteTransaction(r.Context(), db.DeleteTransactionParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		http.Error(w, "Failed to delete", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func formatMoney(cents int64) string {
	return "$" + formatFloat(float64(cents)/100.0, 2)
}

func formatFloat(f float64, prec int) string {
	return strconv.FormatFloat(f, 'f', prec, 64)
}
