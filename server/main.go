package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/calexandrepcjr/cheapskate-finance-tracker/server/db"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/mattn/go-sqlite3"
)

type Config struct {
	Port           int
	DBPath         string
	CategoriesPath string
}

type Application struct {
	Config    Config
	DB        *sql.DB
	Q         *db.Queries
	CatConfig *CategoryConfig
}

func main() {
	var cfg Config
	flag.IntVar(&cfg.Port, "port", 8080, "HTTP server port")
	flag.StringVar(&cfg.DBPath, "db", "cheapskate.db", "Path to SQLite database")
	flag.StringVar(&cfg.CategoriesPath, "categories", "categories.json", "Path to category mappings config file")
	flag.Parse()

	// Initialize Database
	dbConn, err := sql.Open("sqlite3", cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer dbConn.Close()

	if err := dbConn.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// Initialize SQLC queries
	queries := db.New(dbConn)

	// Load category mappings
	catConfig := LoadCategoryConfig(cfg.CategoriesPath)

	app := &Application{
		Config:    cfg,
		DB:        dbConn,
		Q:         queries,
		CatConfig: catConfig,
	}

	// Apply migrations
	if err := app.ensureSchema(); err != nil {
		log.Printf("Warning: Failed to ensure schema: %v", err)
	}

	// Seed Data
	if err := app.ensureSeed(); err != nil {
		log.Printf("Warning: Failed to seed data: %v", err)
	}

	// Setup Router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Static Files
	fileServer(r, "/assets", http.Dir("client/assets"))

	// Routes
	app.setupRoutes(r)

	// Start Server
	log.Printf("Starting server on port %d...", cfg.Port)
	addr := fmt.Sprintf(":%d", cfg.Port)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func (app *Application) ensureSchema() error {
	schema, err := os.ReadFile("server/db/schema.sql")
	if err != nil {
		return fmt.Errorf("could not read schema: %w", err)
	}
	_, err = app.DB.Exec(string(schema))
	if err != nil {
		// Just log, as it fails if table exists
		log.Printf("Schema exec: %v", err)
	}
	return nil
}

func (app *Application) ensureSeed() error {
	var count int
	err := app.DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return err // Table might not exist yet if schema failed completely
	}
	if count == 0 {
		log.Println("Seeding default user...")
		_, err := app.DB.Exec("INSERT INTO users (name, email) VALUES ('CapCJ', 'capcj@example.com')")
		if err != nil {
			return err
		}
	}

	// Ensure income categories have correct type (fixes old databases with Salary as expense)
	_, err = app.DB.Exec(`UPDATE categories SET type = 'income' WHERE name IN ('Salary', 'Earned Income') AND type != 'income'`)
	if err != nil {
		log.Printf("Warning: Could not fix category types: %v", err)
	}

	// Ensure Salary category exists for backwards compatibility (only if not already present)
	_, err = app.DB.Exec(`INSERT INTO categories (name, type, icon, color) SELECT 'Salary', 'income', '💰', '#2ECC71' WHERE NOT EXISTS (SELECT 1 FROM categories WHERE name = 'Salary')`)
	if err != nil {
		log.Printf("Warning: Could not ensure Salary category: %v", err)
	}

	// Clean up duplicate Salary categories created by previous bug (keep only the lowest ID)
	_, err = app.DB.Exec(`DELETE FROM categories WHERE name = 'Salary' AND id != (SELECT MIN(id) FROM categories WHERE name = 'Salary')`)
	if err != nil {
		log.Printf("Warning: Could not clean up duplicate Salary categories: %v", err)
	}

	// Ensure all categories referenced by the category config exist in the database
	if app.CatConfig != nil {
		app.ensureCategoriesFromConfig()
	}

	return nil
}

// ensureCategoriesFromConfig creates any missing categories referenced in the config file.
func (app *Application) ensureCategoriesFromConfig() {
	type catDef struct {
		catType string
		icon    string
		color   string
	}

	knownCategories := map[string]catDef{
		"Earned Income":     {catType: "income", icon: "💰", color: "#2ECC71"},
		"Investment Income": {catType: "income", icon: "📈", color: "#27AE60"},
		"Other Income":      {catType: "income", icon: "💵", color: "#16A085"},
		"Food":              {catType: "expense", icon: "🍔", color: "#FF5733"},
		"Transport":         {catType: "expense", icon: "🚕", color: "#33C1FF"},
		"Housing":           {catType: "expense", icon: "🏠", color: "#8D33FF"},
		"Entertainment":     {catType: "expense", icon: "🎬", color: "#E74C3C"},
		"Shopping":          {catType: "expense", icon: "🛍️", color: "#9B59B6"},
		"Health":            {catType: "expense", icon: "💊", color: "#1ABC9C"},
		"Education":         {catType: "expense", icon: "📚", color: "#3498DB"},
		"Personal Care":     {catType: "expense", icon: "💇", color: "#E67E22"},
		"Subscriptions":     {catType: "expense", icon: "📱", color: "#2980B9"},
		"Gifts & Donations": {catType: "expense", icon: "🎁", color: "#E91E63"},
		"Travel":            {catType: "expense", icon: "✈️", color: "#00BCD4"},
		"Pets":              {catType: "expense", icon: "🐾", color: "#795548"},
	}

	for _, cat := range app.CatConfig.Categories {
		def, ok := knownCategories[cat.Name]
		if !ok {
			// Unknown category from config - default to expense
			def = catDef{catType: "expense", icon: "📌", color: "#95A5A6"}
		}

		_, err := app.DB.Exec(
			`INSERT INTO categories (name, type, icon, color) SELECT ?, ?, ?, ? WHERE NOT EXISTS (SELECT 1 FROM categories WHERE name = ?)`,
			cat.Name, def.catType, def.icon, def.color, cat.Name,
		)
		if err != nil {
			log.Printf("Warning: Could not ensure category %q: %v", cat.Name, err)
		}
	}

	// Also ensure the default category exists
	if app.CatConfig.DefaultCategory != "" {
		def, ok := knownCategories[app.CatConfig.DefaultCategory]
		if !ok {
			def = catDef{catType: "expense", icon: "📌", color: "#95A5A6"}
		}
		_, err := app.DB.Exec(
			`INSERT INTO categories (name, type, icon, color) SELECT ?, ?, ?, ? WHERE NOT EXISTS (SELECT 1 FROM categories WHERE name = ?)`,
			app.CatConfig.DefaultCategory, def.catType, def.icon, def.color, app.CatConfig.DefaultCategory,
		)
		if err != nil {
			log.Printf("Warning: Could not ensure default category %q: %v", app.CatConfig.DefaultCategory, err)
		}
	}
}

func fileServer(r chi.Router, path string, root http.FileSystem) {
	if code := path[len(path)-1]; code != '/' {
		path += "/"
	}
	path += "*"

	r.Get(path, func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.RouteContext(r.Context())
		pathPrefix := strings.TrimSuffix(rctx.RoutePattern(), "/*")
		fs := http.StripPrefix(pathPrefix, http.FileServer(root))
		fs.ServeHTTP(w, r)
	})
}
