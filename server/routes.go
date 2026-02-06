package main

import (
	"github.com/go-chi/chi/v5"
)

func (app *Application) setupRoutes(r chi.Router) {
	r.Get("/", app.HandleHome)
	r.Get("/dashboard", app.HandleDashboard)
	r.Get("/dashboard/detailed", app.HandleDashboardDetailed)
	r.Get("/settings", app.HandleSettings)
	r.Get("/api/transactions", app.HandleTransactionsPage)
	r.Post("/api/transaction", app.HandleTransactionCreate)
	r.Delete("/api/transaction/{id}", app.HandleTransactionDelete)
	r.Get("/api/export/csv", app.HandleExportCSV)
	r.Delete("/api/data", app.HandleWipeData)

	// Storage endpoints for IndexedDB <-> SQLite synchronization
	r.Get("/api/storage/status", app.HandleStorageStatus)
	r.Get("/api/storage/export", app.HandleStorageExport)
	r.Post("/api/storage/import", app.HandleStorageImport)
}
