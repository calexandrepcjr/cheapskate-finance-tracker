# CLAUDE.md - AI Assistant Guide for Cheapskate Finance Tracker

This document provides comprehensive guidance for AI assistants working on the Cheapskate codebase.

## Project Overview

Cheapskate is a self-hosted, privacy-focused personal finance tracker built with Go. It uses server-side rendering with HTMX for interactivity, providing an SPA-like experience without heavy JavaScript frameworks.

**Current Stage:** MVP/Early Beta (single-user, no authentication)

## Technology Stack

| Layer | Technology | Purpose |
|-------|------------|---------|
| Language | Go 1.25+ | Backend logic |
| Router | chi/v5 | HTTP routing and middleware |
| Templates | Templ | Type-safe HTML templating |
| Database | SQLite | Embedded file-based storage |
| SQL Codegen | SQLC | Type-safe query generation |
| Frontend | HTMX + Tailwind CSS | Interactive UI without JS frameworks |
| Dev Server | Air | Hot-reload development |

## Directory Structure

```
cheapskate-finance-tracker/
├── .claude/
│   └── settings.json        # Claude Code hooks (pre-commit tests)
├── client/
│   └── templates/           # Templ UI components
│       ├── layout.templ     # Master layout wrapper
│       ├── home.templ       # Input form page
│       ├── dashboard.templ  # Transaction list view
│       └── *_templ.go       # Generated Go files (DO NOT EDIT)
├── server/
│   ├── db/                  # Database layer
│   │   ├── schema.sql       # SQLite schema definition
│   │   ├── queries.sql      # SQLC query definitions
│   │   ├── queries_test.go  # Database integration tests
│   │   ├── models.go        # Generated models (DO NOT EDIT)
│   │   ├── db.go            # Generated DB code (DO NOT EDIT)
│   │   ├── querier.go       # Generated interface (DO NOT EDIT)
│   │   └── queries.sql.go   # Generated queries (DO NOT EDIT)
│   ├── main.go              # Application entry point
│   ├── main_test.go         # Integration tests
│   ├── routes.go            # HTTP route definitions
│   ├── handlers_frontend.go # HTTP request handlers
│   ├── handlers_frontend_test.go # Handler tests
│   ├── parser.go            # Transaction input parsing
│   └── parser_test.go       # Parser unit tests
├── scripts/
│   ├── setup-hooks.sh       # Installs git hooks
│   ├── validate-commit-msg.sh # Validates conventional commit format
│   └── hooks/
│       ├── pre-commit       # Pre-commit hook (runs tests)
│       └── commit-msg       # Commit-msg hook (conventional commits)
├── .air.toml                # Hot-reload configuration
├── sqlc.yaml                # SQLC code generator config
├── Makefile                 # Build automation
├── go.mod                   # Go module definition
└── README.md                # User documentation
```

## Key Files Reference

### Server Entry Point (`server/main.go`)
- Parses CLI flags: `--port` (default: 8080), `--db` (default: cheapskate.db)
- Creates `Application` struct with config, DB connection, and queries
- Runs schema migration via `ensureSchema()`
- Seeds default data via `ensureSeed()`
- Sets up chi router with logging and recovery middleware

### Routes (`server/routes.go`)
```
GET  /                  → HandleHome (input form)
GET  /dashboard         → HandleDashboard (transaction list)
POST /api/transaction   → HandleTransactionCreate (HTMX endpoint)
```

### Handlers (`server/handlers_frontend.go`)
- `HandleHome()` - Renders the transaction input form
- `HandleDashboard()` - Fetches recent transactions, renders dashboard
- `HandleTransactionCreate()` - Parses input, creates transaction, returns HTML fragment

### Parser (`server/parser.go`)
- `ParseTransaction(input)` - Parses natural language like "12.50 coffee"
- `inferCategory(desc)` - Simple keyword-based category matching
- **Note:** Contains TODO for LLM integration

### Database Schema (`server/db/schema.sql`)
4 tables:
- `users` - User accounts
- `categories` - Transaction categories (income/expense)
- `transactions` - Financial transactions (amount stored in cents)

### Templ Components (`client/templates/`)
- `Layout(title, content)` - Master wrapper with header/footer
- `Home()` / `InputForm()` - Transaction input page
- `Dashboard()` / `HouseView()` - Transaction list with house visualization
- `TransactionSuccess()` / `TransactionError()` - HTMX response fragments

## Development Commands

```bash
# Install required tools (sqlc, templ, air)
make tools

# Run code generators (SQLC + Templ)
make generate

# Build binary to ./bin/server
make build

# Run server directly (includes code generation)
make run

# Start dev server with hot-reload
make dev

# Clean build artifacts
make clean
```

## Code Generation Pipeline

**Important:** This project uses code generation. Never manually edit generated files.

1. **SQLC** generates Go code from SQL:
   - Input: `server/db/schema.sql` + `server/db/queries.sql`
   - Output: `server/db/models.go`, `db.go`, `querier.go`, `queries.sql.go`
   - Config: `sqlc.yaml`

2. **Templ** generates Go code from templates:
   - Input: `client/templates/*.templ`
   - Output: `client/templates/*_templ.go`

**Workflow for changes:**
- To add/modify database queries: Edit `queries.sql`, run `make generate`
- To change schema: Edit `schema.sql`, run `make generate`
- To modify UI: Edit `.templ` files, run `make generate`
- With `make dev`, regeneration happens automatically on save

## Code Conventions

### Go Patterns
- HTTP handlers named `Handle<Action>` (e.g., `HandleDashboard`)
- Database queries named verb-first (e.g., `GetUser`, `ListCategories`)
- Context as first parameter in all DB functions
- Explicit `if err != nil` error handling
- Amounts stored as int64 cents, formatted for display

### Templ Patterns
```go
// Components are functions returning templ.Component
templ ComponentName(params) {
    <div>{ params }</div>
}

// Embed components with @
@Layout("Title") {
    @ContentComponent()
}

// Loop with for
for _, item := range items {
    @ItemComponent(item)
}
```

### HTMX Patterns
- Forms use `hx-post`, `hx-target`, `hx-swap` attributes
- Responses return HTML fragments, not full pages
- Target elements have unique IDs for swapping

### Database Patterns
- All queries defined in `queries.sql` with SQLC annotations
- Use `sql.NullString`, `sql.NullTime` for nullable fields
- Prepared statements via SQLC's `emit_prepared_queries`

## Adding New Features

### Adding a New Database Query
1. Add query to `server/db/queries.sql`:
   ```sql
   -- name: GetTransactionByID :one
   SELECT * FROM transactions WHERE id = ?;
   ```
2. Run `make generate`
3. Use in handler: `app.Q.GetTransactionByID(ctx, id)`

### Adding a New Route
1. Add route in `server/routes.go`:
   ```go
   r.Get("/new-route", app.HandleNewRoute)
   ```
2. Add handler in `server/handlers_frontend.go`:
   ```go
   func (app *Application) HandleNewRoute(w http.ResponseWriter, r *http.Request) {
       // Handler logic
   }
   ```

### Adding a New Template
1. Create `client/templates/newpage.templ`:
   ```go
   package templates

   templ NewPage() {
       @Layout("Page Title") {
           <div>Content here</div>
       }
   }
   ```
2. Run `make generate`
3. Use in handler: `templates.NewPage().Render(r.Context(), w)`

## Current Limitations

- **Single-user:** User ID is hardcoded to 1 in handlers
- **No authentication:** Anyone with access can view/create transactions
- **Category inference:** Basic keyword matching, marked for LLM enhancement

## Architecture Notes

### Data Flow
```
User Input → HTMX POST → Handler → ParseTransaction()
    → Resolve Category → SQLC Insert → Templ Response → HTMX DOM Update
```

### Application Struct
```go
type Application struct {
    Config Config
    DB     *sql.DB
    Q      *db.Queries  // SQLC generated queries
}
```

### Money Handling
- All amounts stored as `int64` cents in database
- Converted to display format: `formatMoney(1250)` → "$12.50"
- Currency field exists but defaults to "USD"

## Testing Guidelines

The project has comprehensive test coverage. Run tests with:

```bash
go test ./... -v
```

### Test Files

| File | Tests |
|------|-------|
| `server/parser_test.go` | Unit tests for `ParseTransaction`, `parseAmount`, `inferCategory`, `formatMoney` |
| `server/handlers_frontend_test.go` | HTTP handler tests using `httptest` with in-memory SQLite |
| `server/main_test.go` | Integration tests for `ensureSchema`, `ensureSeed` |
| `server/db/queries_test.go` | Database layer tests for all SQLC queries |

### Git Pre-commit Hook

A git pre-commit hook runs `go test ./...` before every commit. Install it with:

```bash
./scripts/setup-hooks.sh
```

This ensures all tests pass before any code can be committed, regardless of whether commits are made via CLI, IDE, or AI assistant.

### Claude Code Pre-commit Hook

A Claude Code hook is also configured in `.claude/settings.json` to run tests and install git hooks before commits made through Claude Code. This provides an additional layer of enforcement for AI-assisted development.

**Important:** If tests fail during a commit attempt, fix the failing tests before retrying the commit.

## Conventional Commits (REQUIRED)

**All commits to this repository MUST follow the Conventional Commits specification.**

This project enforces conventional commits through a `commit-msg` git hook. Invalid commit messages will be rejected.

### Format

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

### Allowed Types

| Type | Description | Example |
|------|-------------|---------|
| `feat` | A new feature | `feat: add transaction export to CSV` |
| `fix` | A bug fix | `fix: correct amount parsing for decimals` |
| `docs` | Documentation only | `docs: update API documentation` |
| `style` | Formatting, no code change | `style: fix indentation in handlers` |
| `refactor` | Code change (no feature/fix) | `refactor: extract validation logic` |
| `perf` | Performance improvement | `perf: optimize database queries` |
| `test` | Adding or correcting tests | `test: add parser edge case tests` |
| `build` | Build system or dependencies | `build: upgrade Go to 1.25` |
| `ci` | CI configuration changes | `ci: add GitHub Actions workflow` |
| `chore` | Other maintenance tasks | `chore: update .gitignore` |
| `revert` | Reverts a previous commit | `revert: revert "feat: add export"` |

### Optional Scope

Add a scope in parentheses to specify the affected component:

```
feat(parser): add support for currency symbols
fix(db): handle null values in transactions
test(handlers): add dashboard endpoint coverage
```

### Examples

```bash
# Good commit messages
feat: add user authentication system
fix(parser): handle negative transaction amounts
docs: add API endpoint documentation
test(db): add queries integration tests
refactor(handlers): extract common validation logic
build: add Docker support

# Bad commit messages (will be REJECTED)
added new feature          # Missing type
fix - corrected bug        # Wrong separator (use colon)
FEAT: uppercase type       # Types must be lowercase
feat:no space after colon  # Missing space after colon
update code                # Missing type
```

### Making Commits

When committing changes, always use this format:

```bash
git commit -m "type(scope): description"
```

Or for multi-line messages:

```bash
git commit -m "$(cat <<'EOF'
feat(parser): add currency symbol support

- Support for $, EUR, GBP symbols
- Auto-detect currency from symbol
- Default to USD when no symbol present

Closes #123
EOF
)"
```

### Validation Scripts

The project includes validation scripts:

- `scripts/hooks/commit-msg` - Git hook that validates commit messages
- `scripts/validate-commit-msg.sh` - Standalone validation script

To manually validate a commit message:

```bash
./scripts/validate-commit-msg.sh "feat: my commit message"
```

### Writing New Tests

- Use table-driven tests (Go idiom) for comprehensive coverage
- Use `setupTestApp()` helper for handler tests (provides in-memory SQLite)
- Use `setupTestDB()` helper for database tests
- Test both success and error paths
- Verify database state changes for mutation operations

## Common Issues

### "cannot find package" errors
Run `make generate` to ensure all generated code exists.

### Changes not reflecting in browser
- Check if `make dev` is running
- Air watches `.go`, `.templ`, `.sql` files only
- Generated files are excluded from watch

### Database schema changes
After modifying `schema.sql`:
1. Delete `cheapskate.db` file
2. Run `make generate`
3. Restart server (schema applied on startup)

## File Quick Reference

| Task | File(s) to Edit |
|------|-----------------|
| Add DB table/column | `server/db/schema.sql` |
| Add DB query | `server/db/queries.sql` |
| Add HTTP route | `server/routes.go` |
| Add HTTP handler | `server/handlers_frontend.go` |
| Modify input parsing | `server/parser.go` |
| Modify UI layout | `client/templates/layout.templ` |
| Modify home page | `client/templates/home.templ` |
| Modify dashboard | `client/templates/dashboard.templ` |
| Change dev server config | `.air.toml` |
| Change SQLC settings | `sqlc.yaml` |
| Add parser tests | `server/parser_test.go` |
| Add handler tests | `server/handlers_frontend_test.go` |
| Add DB tests | `server/db/queries_test.go` |
| Configure Claude hooks | `.claude/settings.json` |
| Add/modify git hooks | `scripts/hooks/` |
| Validate commit message | `scripts/validate-commit-msg.sh` |
