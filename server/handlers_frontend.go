package main

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/calexandrepcjr/cheapskate-finance-tracker/client/templates"
	"github.com/calexandrepcjr/cheapskate-finance-tracker/server/db"
	"github.com/go-chi/chi/v5"
)

func (app *Application) HandleHome(w http.ResponseWriter, r *http.Request) {
	templates.Home().Render(r.Context(), w)
}

const transactionsPageSize = 20

func (app *Application) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get year from query param, default to current year
	yearParam := r.URL.Query().Get("year")
	if yearParam == "" {
		yearParam = fmt.Sprintf("%d", time.Now().Year())
	}

	// Check if we should show deleted transactions
	showDeleted := r.URL.Query().Get("show_deleted") == "true"

	// Get available years for navigation
	years, err := app.Q.GetDistinctTransactionYearsWrapped(ctx)
	if err != nil {
		http.Error(w, "Failed to load years: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// If no transactions exist yet, add current year to list
	currentYear := int64(time.Now().Year())
	hasCurrentYear := false
	for _, y := range years {
		if y.Year == currentYear {
			hasCurrentYear = true
			break
		}
	}
	if !hasCurrentYear {
		years = append([]db.GetDistinctTransactionYearsRow{{Year: currentYear}}, years...)
	}

	var totalCount int64

	if showDeleted {
		// Fetch with deleted transactions included
		txsWithDeleted, err := app.Q.ListTransactionsByYearPaginatedWithDeleted(ctx, db.ListTransactionsByYearPaginatedWithDeletedParams{
			Year:   yearParam,
			Limit:  transactionsPageSize,
			Offset: 0,
		})
		if err != nil {
			http.Error(w, "Failed to load transactions: "+err.Error(), http.StatusInternalServerError)
			return
		}

		totalCount, err = app.Q.CountTransactionsByYearWithDeleted(ctx, yearParam)
		if err != nil {
			http.Error(w, "Failed to count transactions: "+err.Error(), http.StatusInternalServerError)
			return
		}

		categoryTotals, err := app.Q.GetCategoryTotalsByYear(ctx, yearParam)
		if err != nil {
			http.Error(w, "Failed to load category totals: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Convert WithDeleted rows to standard paginated rows for template reuse
		txs := make([]db.ListTransactionsByYearPaginatedRow, len(txsWithDeleted))
		for i, t := range txsWithDeleted {
			txs[i] = db.ListTransactionsByYearPaginatedRow{
				ID: t.ID, UserID: t.UserID, CategoryID: t.CategoryID,
				Amount: t.Amount, Currency: t.Currency, Description: t.Description,
				Date: t.Date, CreatedAt: t.CreatedAt, DeletedAt: t.DeletedAt,
				CategoryName: t.CategoryName, CategoryIcon: t.CategoryIcon,
				CategoryType: t.CategoryType, UserName: t.UserName,
			}
		}

		hasMore := int64(len(txs)) < totalCount
		templates.Dashboard(txs, categoryTotals, years, yearParam, totalCount, hasMore, showDeleted).Render(ctx, w)
		return
	}

	// Fetch first page of transactions (active only)
	txs, err := app.Q.ListTransactionsByYearPaginated(ctx, db.ListTransactionsByYearPaginatedParams{
		Year:   yearParam,
		Limit:  transactionsPageSize,
		Offset: 0,
	})
	if err != nil {
		http.Error(w, "Failed to load transactions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get total count for pagination
	totalCount, err = app.Q.CountTransactionsByYear(ctx, yearParam)
	if err != nil {
		http.Error(w, "Failed to count transactions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch category totals for the mosaic
	categoryTotals, err := app.Q.GetCategoryTotalsByYear(ctx, yearParam)
	if err != nil {
		http.Error(w, "Failed to load category totals: "+err.Error(), http.StatusInternalServerError)
		return
	}

	hasMore := int64(len(txs)) < totalCount

	templates.Dashboard(txs, categoryTotals, years, yearParam, totalCount, hasMore, showDeleted).Render(ctx, w)
}

func (app *Application) HandleTransactionsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	yearParam := r.URL.Query().Get("year")
	if yearParam == "" {
		yearParam = fmt.Sprintf("%d", time.Now().Year())
	}

	offsetParam := r.URL.Query().Get("offset")
	offset, _ := strconv.ParseInt(offsetParam, 10, 64)

	// Fetch page of transactions
	txs, err := app.Q.ListTransactionsByYearPaginated(ctx, db.ListTransactionsByYearPaginatedParams{
		Year:   yearParam,
		Limit:  transactionsPageSize,
		Offset: offset,
	})
	if err != nil {
		http.Error(w, "Failed to load transactions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get total count for pagination
	totalCount, err := app.Q.CountTransactionsByYear(ctx, yearParam)
	if err != nil {
		http.Error(w, "Failed to count transactions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	hasMore := offset+int64(len(txs)) < totalCount
	nextOffset := offset + int64(len(txs))

	templates.TransactionsList(txs, yearParam, nextOffset, hasMore).Render(ctx, w)
}

func (app *Application) HandleDashboardDetailed(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get year from query param, default to current year
	yearParam := r.URL.Query().Get("year")
	if yearParam == "" {
		yearParam = fmt.Sprintf("%d", time.Now().Year())
	}

	// Get available years for navigation
	years, err := app.Q.GetDistinctTransactionYearsWrapped(ctx)
	if err != nil {
		http.Error(w, "Failed to load years: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// If no transactions exist yet, add current year to list
	currentYear := int64(time.Now().Year())
	hasCurrentYear := false
	for _, y := range years {
		if y.Year == currentYear {
			hasCurrentYear = true
			break
		}
	}
	if !hasCurrentYear {
		years = append([]db.GetDistinctTransactionYearsRow{{Year: currentYear}}, years...)
	}

	// Fetch category totals for pie chart
	categoryTotals, err := app.Q.GetCategoryTotalsByYear(ctx, yearParam)
	if err != nil {
		http.Error(w, "Failed to load category totals: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch monthly totals for bar chart
	monthlyTotals, err := app.Q.GetMonthlyTotalsByYear(ctx, yearParam)
	if err != nil {
		http.Error(w, "Failed to load monthly totals: "+err.Error(), http.StatusInternalServerError)
		return
	}

	templates.DashboardDetailed(categoryTotals, monthlyTotals, years, yearParam).Render(ctx, w)
}

func (app *Application) HandleTransactionCreate(w http.ResponseWriter, r *http.Request) {
	input := r.FormValue("input")

	// Check if this is a remove command
	if IsRemoveCommand(input) {
		app.handleRemoveSearch(w, r, input)
		return
	}

	// 1. Parse
	parsed, err := ParseTransaction(input, app.CatConfig)
	if err != nil {
		templates.TransactionError("Could not understand that. Try '50 pizza'").Render(r.Context(), w)
		return
	}

	// 2. Resolve Category
	// For now, we query by name. If not found, try alternative names or use default.
	var catID int64
	var catName string
	var catType string
	cat, err := app.Q.GetCategoryByName(r.Context(), parsed.Category)
	if err != nil {
		// Try alternative name for backwards compatibility
		altName := parsed.Category
		if parsed.Category == "Earned Income" {
			altName = "Salary"
		}
		cat, err = app.Q.GetCategoryByName(r.Context(), altName)
		if err != nil {
			// Fallback to first category
			cats, _ := app.Q.ListCategories(r.Context())
			if len(cats) > 0 {
				catID = cats[0].ID
				catName = cats[0].Name
				catType = cats[0].Type
			} else {
				catID = 1
				catName = "Unknown"
				catType = "expense"
			}
		} else {
			catID = cat.ID
			catName = cat.Name
			catType = cat.Type
		}
	} else {
		catID = cat.ID
		catName = cat.Name
		catType = cat.Type
	}

	// 3. User ID (Hardcoded for single user MVP/Monolith)
	userID := int64(1)

	// 4. Determine amount sign (expenses are negative, income is positive)
	amount := parsed.Amount
	if catType == "expense" {
		amount = -amount
	}

	// 5. Insert
	_, err = app.Q.CreateTransaction(r.Context(), db.CreateTransactionParams{
		UserID:      userID,
		CategoryID:  catID,
		Amount:      amount,
		Currency:    "USD",
		Description: parsed.Description,
		Date:        time.Now(),
	})
	if err != nil {
		templates.TransactionError("Failed to save: "+err.Error()).Render(r.Context(), w)
		return
	}

	// 6. Render Success (display positive amount)
	displayAmt := formatMoney(parsed.Amount)
	templates.TransactionSuccess(displayAmt, parsed.Description, catName).Render(r.Context(), w)
}

func (app *Application) HandleTransactionDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get transaction ID from URL
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid transaction ID", http.StatusBadRequest)
		return
	}

	// User ID (hardcoded for single user MVP)
	userID := int64(1)

	// Soft delete transaction
	err = app.Q.SoftDeleteTransaction(ctx, db.SoftDeleteTransactionParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		http.Error(w, "Failed to delete transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return empty response for HTMX to remove the element
	w.WriteHeader(http.StatusOK)
}

func (app *Application) handleRemoveSearch(w http.ResponseWriter, r *http.Request, input string) {
	ctx := r.Context()

	parsed, err := ParseRemoveCommand(input)
	if err != nil {
		templates.TransactionError("Could not understand that. Try 'remove 50' or 'remove 50 pizza'").Render(ctx, w)
		return
	}

	userID := int64(1)

	// Search for matching transactions by amount
	txs, err := app.Q.SearchTransactionsForRemoval(ctx, db.SearchTransactionsForRemovalParams{
		Amount: parsed.Amount,
		UserID: userID,
	})
	if err != nil {
		templates.TransactionError("Failed to search transactions: "+err.Error()).Render(ctx, w)
		return
	}

	// Filter by description if provided
	if parsed.Description != "" {
		var filtered []db.SearchTransactionsForRemovalRow
		descLower := strings.ToLower(parsed.Description)
		for _, tx := range txs {
			if strings.Contains(strings.ToLower(tx.Description), descLower) ||
				strings.Contains(strings.ToLower(tx.CategoryName), descLower) {
				filtered = append(filtered, tx)
			}
		}
		txs = filtered
	}

	if len(txs) == 0 {
		templates.TransactionError("No matching transactions found for "+formatMoney(parsed.Amount)).Render(ctx, w)
		return
	}

	templates.RemoveCandidates(txs, formatMoney(parsed.Amount)).Render(ctx, w)
}

func (app *Application) HandleTransactionSoftDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid transaction ID", http.StatusBadRequest)
		return
	}

	userID := int64(1)

	err = app.Q.SoftDeleteTransaction(ctx, db.SoftDeleteTransactionParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		http.Error(w, "Failed to remove transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}

	templates.TransactionRemoved().Render(ctx, w)
}

func formatMoney(cents int64) string {
	return "$" + formatFloat(float64(cents)/100.0, 2)
}

func formatFloat(f float64, prec int) string {
	return strconv.FormatFloat(f, 'f', prec, 64)
}

func (app *Application) HandleSettings(w http.ResponseWriter, r *http.Request) {
	var mappings []templates.CategoryMapping
	if app.CatConfig != nil {
		for _, cat := range app.CatConfig.Categories {
			mappings = append(mappings, templates.CategoryMapping{
				Name:     cat.Name,
				Keywords: cat.Keywords,
			})
		}
	}
	templates.Settings(mappings).Render(r.Context(), w)
}

func (app *Application) HandleExportCSV(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	txs, err := app.Q.ListAllTransactionsForExport(ctx)
	if err != nil {
		http.Error(w, "Failed to load transactions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=cheapskate-export.csv")

	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Header row
	writer.Write([]string{"ID", "Date", "Description", "Category", "Type", "Amount", "Currency"})

	for _, t := range txs {
		amount := float64(t.Amount) / 100.0
		if amount < 0 {
			amount = -amount
		}
		writer.Write([]string{
			strconv.FormatInt(t.ID, 10),
			t.Date.Format("2006-01-02"),
			t.Description,
			t.CategoryName,
			t.CategoryType,
			strconv.FormatFloat(amount, 'f', 2, 64),
			t.Currency,
		})
	}
}

func (app *Application) HandleWipeData(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	err := app.Q.DeleteAllTransactions(ctx)
	if err != nil {
		templates.WipeError(err.Error()).Render(ctx, w)
		return
	}

	templates.WipeSuccess().Render(ctx, w)
}
