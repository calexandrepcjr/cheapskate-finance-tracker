package main

import (
	"github.com/go-chi/chi/v5"
)

func (app *Application) setupRoutes(r chi.Router) {
	r.Get("/", app.HandleHome)
	r.Get("/dashboard", app.HandleDashboard)
	r.Get("/dashboard/detailed", app.HandleDashboardDetailed)
	r.Get("/api/transactions", app.HandleTransactionsPage)
	r.Post("/api/transaction", app.HandleTransactionCreate)
	r.Delete("/api/transaction/{id}", app.HandleTransactionDelete)
}
