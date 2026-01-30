package main

import (
	"net/http"
	"strconv"
	"time"

	"github.com/calexandrepcjr/cheapskate-finance-tracker/client/templates"
	"github.com/calexandrepcjr/cheapskate-finance-tracker/server/db"
)

func (app *Application) HandleHome(w http.ResponseWriter, r *http.Request) {
	templates.Home().Render(r.Context(), w)
}

func (app *Application) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	// Fetch recent transactions
	txs, err := app.Q.ListRecentTransactions(r.Context())
	if err != nil {
		http.Error(w, "Failed to load dashboard: "+err.Error(), http.StatusInternalServerError)
		return
	}
	templates.Dashboard(txs).Render(r.Context(), w)
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
	cat, err := app.Q.GetCategoryByName(r.Context(), parsed.Category)
	if err != nil {
		// Fallback to first category
		cats, _ := app.Q.ListCategories(r.Context())
		if len(cats) > 0 {
			catID = cats[0].ID
		} else {
			catID = 1 // Risky if empty
		}
	} else {
		catID = cat.ID
	}

	// 3. User ID (Hardcoded for single user MVP/Monolith)
	userID := int64(1)

	// 4. Insert
	_, err = app.Q.CreateTransaction(r.Context(), db.CreateTransactionParams{
		UserID:      userID,
		CategoryID:  catID,
		Amount:      parsed.Amount,
		Currency:    "USD",
		Description: parsed.Description,
		Date:        time.Now(),
	})
	if err != nil {
		templates.TransactionError("Failed to save: "+err.Error()).Render(r.Context(), w)
		return
	}

	// 5. Render Success
	displayAmt := formatMoney(parsed.Amount)
	templates.TransactionSuccess(displayAmt, parsed.Description).Render(r.Context(), w)
}

func formatMoney(cents int64) string {
	return "$" + formatFloat(float64(cents)/100.0, 2)
}

func formatFloat(f float64, prec int) string {
	return strconv.FormatFloat(f, 'f', prec, 64)
}
