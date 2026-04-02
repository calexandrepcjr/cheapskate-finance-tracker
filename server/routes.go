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
	r.Post("/api/transaction/{id}/remove", app.HandleTransactionSoftDelete)
	r.Get("/api/export/csv", app.HandleExportCSV)
	r.Delete("/api/data", app.HandleWipeData)

	// Storage endpoints for IndexedDB <-> SQLite synchronization
	r.Get("/api/storage/status", app.HandleStorageStatus)
	r.Get("/api/storage/export", app.HandleStorageExport)
	r.Post("/api/storage/import", app.HandleStorageImport)

	// Backup endpoints
	r.Get("/api/backup/download", app.HandleBackupDownload)
	r.Post("/api/backup/restore", app.HandleBackupRestore)
	r.Get("/api/backup/status", app.HandleBackupStatus)

	// Google Drive endpoints
	r.Get("/api/gdrive/status", app.HandleGDriveStatus)
	r.Get("/api/gdrive/connect", app.HandleGDriveConnect)
	r.Get("/api/gdrive/disconnect", app.HandleGDriveDisconnect)
	r.Get("/api/gdrive/sync", app.HandleGDriveSync)
	r.Get("/oauth2/callback", app.HandleGDriveCallback)
}
