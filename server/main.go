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
	Port   int
	DBPath string
}

type Application struct {
	Config Config
	DB     *sql.DB
	Q      *db.Queries
}

func main() {
	var cfg Config
	flag.IntVar(&cfg.Port, "port", 8080, "HTTP server port")
	flag.StringVar(&cfg.DBPath, "db", "cheapskate.db", "Path to SQLite database")
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

	app := &Application{
		Config: cfg,
		DB:     dbConn,
		Q:      queries,
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

	// Ensure Salary category exists for backwards compatibility
	_, err = app.DB.Exec(`INSERT OR IGNORE INTO categories (name, type, icon, color) VALUES ('Salary', 'income', '💰', '#2ECC71')`)
	if err != nil {
		log.Printf("Warning: Could not ensure Salary category: %v", err)
	}

	return nil
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
