# SQLite Sync Plan: File Backup + UI Controls

## Summary

Implement a "dumb and simple" sync strategy that backs up the SQLite database to a
configurable path on a timer, with manual download/upload controls in the Settings UI.
Users point `--backup-path` at a cloud-synced folder (Google Drive desktop, Dropbox,
Syncthing, etc.) and get automatic cloud sync with zero API credentials or external
services.

## Architecture

```
Cheapskate Server
  |
  |-- backup goroutine (every N minutes)
  |     |
  |     |-- SQLite backup API → {backup-path}/cheapskate.db
  |     |-- JSON export       → {backup-path}/cheapskate.json
  |
  |-- GET  /api/backup/download  → browser downloads .db file
  |-- POST /api/backup/restore   → browser uploads .db file to replace current DB
  |
  |-- Settings UI
        |-- "Download Backup" button
        |-- "Restore from Backup" file upload
        |-- Backup status indicator (last backup time, path)
```

## Implementation Steps

### Step 1: Add backup configuration to `server/main.go`

Add two new CLI flags to the `Config` struct:

```go
type Config struct {
    Port           int
    DBPath         string
    CategoriesPath string
    BackupPath     string  // NEW: directory for backup files (empty = disabled)
    BackupInterval int     // NEW: backup interval in minutes (default: 30)
}
```

Parse the new flags:
```go
flag.StringVar(&cfg.BackupPath, "backup-path", "", "Directory for automatic backups (disabled if empty)")
flag.IntVar(&cfg.BackupInterval, "backup-interval", 30, "Backup interval in minutes")
```

Start the backup goroutine in `main()` if `BackupPath` is set:
```go
if cfg.BackupPath != "" {
    go app.startBackupLoop()
}
```

### Step 2: Create `server/backup.go` (~120 lines)

This file contains:

1. **`performBackup()`** - Uses the `mattn/go-sqlite3` backup API via raw connection
   to create a consistent `.db` snapshot. This is safe to call while the DB is being
   written to.

2. **`performJSONExport()`** - Reuses the existing `StorageExportResponse` format
   from `handlers_sync.go` to write a human-readable JSON sidecar. Exports ALL years
   (not just current year like the IndexedDB sync does).

3. **`startBackupLoop()`** - Runs on a ticker, calls both backup functions. Logs
   success/failure. Also runs once immediately on startup.

4. **`lastBackupTime`** - A mutex-protected timestamp tracking when the last
   successful backup completed (for the UI status indicator).

Key implementation detail for the SQLite backup API:
```go
func (app *Application) performBackup() error {
    destPath := filepath.Join(app.Config.BackupPath, "cheapskate.db")

    // Use raw connection to access sqlite3 backup API
    conn, err := app.DB.Conn(context.Background())
    if err != nil {
        return err
    }
    defer conn.Close()

    return conn.Raw(func(driverConn interface{}) error {
        srcConn := driverConn.(*sqlite3.SQLiteConn)

        // Open destination
        destDB, err := sql.Open("sqlite3", destPath)
        if err != nil {
            return err
        }
        defer destDB.Close()

        destConnCtx, err := destDB.Conn(context.Background())
        if err != nil {
            return err
        }
        defer destConnCtx.Close()

        return destConnCtx.Raw(func(dc interface{}) error {
            destConn := dc.(*sqlite3.SQLiteConn)
            backup, err := destConn.Backup("main", srcConn, "main")
            if err != nil {
                return err
            }
            _, err = backup.Step(-1) // copy all pages
            if err != nil {
                return err
            }
            return backup.Finish()
        })
    })
}
```

### Step 3: Add backup download/restore handlers (~100 lines)

Add to `server/handlers_backup.go`:

1. **`HandleBackupDownload`** (`GET /api/backup/download`)
   - Creates an in-memory SQLite backup using the backup API
   - Writes to a temp file, then serves it with `Content-Disposition: attachment`
   - Filename: `cheapskate-backup-{date}.db`

2. **`HandleBackupRestore`** (`POST /api/backup/restore`)
   - Accepts a multipart file upload of a `.db` file
   - Validates it's a valid SQLite database (check magic bytes: first 16 bytes)
   - Saves to a temp location
   - Uses SQLite backup API in reverse to copy uploaded DB into the live database
   - Returns success/error HTML fragment for HTMX swap

3. **`HandleBackupStatus`** (`GET /api/backup/status`)
   - Returns JSON with: `last_backup_at`, `backup_path`, `backup_enabled`
   - Used by the Settings UI to show current backup status

### Step 4: Add routes in `server/routes.go`

```go
// Backup endpoints
r.Get("/api/backup/download", app.HandleBackupDownload)
r.Post("/api/backup/restore", app.HandleBackupRestore)
r.Get("/api/backup/status", app.HandleBackupStatus)
```

### Step 5: Update Settings UI in `client/templates/settings.templ`

Add a "Backup & Restore" section between "Export Data" and "Danger Zone":

```
┌─────────────────────────────────────────────────┐
│ Backup & Restore                                │
│                                                 │
│ Automatic backups: Enabled / Disabled           │
│ Backup path: /app/backups                       │
│ Last backup: 2 minutes ago                      │
│                                                 │
│ [Download Backup]   [Restore from Backup...]    │
│                                                 │
│ ℹ️  Tip: Point --backup-path at a cloud-synced  │
│    folder (Dropbox, Google Drive, Syncthing)    │
│    for automatic offsite backups.               │
└─────────────────────────────────────────────────┘
```

The template needs:
- A new `BackupStatus` struct parameter
- Download button: simple `<a href="/api/backup/download">`
- Restore: file input + HTMX POST to `/api/backup/restore`
- Status display showing last backup time and path
- Result div for restore feedback

### Step 6: Update Docker configuration

**`docker-compose.yml`** - Add backup volume:
```yaml
volumes:
  - cheapskate-data:/app/data
  - cheapskate-backups:/app/backups  # NEW
```

**`Dockerfile`** - Create backup directory and add `categories.json`:
```dockerfile
RUN mkdir -p /app/data /app/backups
CMD ["/app/server", "--port", "8080", "--db", "/app/data/cheapskate.db", "--backup-path", "/app/backups"]
```

### Step 7: Add tests

**`server/backup_test.go`**:
- Test `performBackup()` creates a valid .db file
- Test `performJSONExport()` creates valid JSON matching `StorageExportResponse` format
- Test `HandleBackupDownload` returns a valid SQLite file
- Test `HandleBackupRestore` with a valid .db file replaces data
- Test `HandleBackupRestore` rejects non-SQLite files
- Test backup loop respects interval timing

### Step 8: Documentation

Update `README.md` with a "Backup & Sync" section:
- How to use `--backup-path` for automatic backups
- How to point it at a cloud-synced folder
- How to use the Settings UI for manual backup/restore
- Advanced: Litestream setup for continuous replication to S3

## Files to Create/Modify

| File | Action | Lines (est.) |
|------|--------|-------------|
| `server/backup.go` | CREATE | ~120 |
| `server/handlers_backup.go` | CREATE | ~100 |
| `server/backup_test.go` | CREATE | ~150 |
| `server/main.go` | MODIFY | +10 |
| `server/routes.go` | MODIFY | +5 |
| `client/templates/settings.templ` | MODIFY | +50 |
| `docker-compose.yml` | MODIFY | +2 |
| `Dockerfile` | MODIFY | +2 |

**Total new code: ~440 lines** (including tests)
**New dependencies: 0** (uses existing `mattn/go-sqlite3` backup API)

## Why This Approach

1. **Zero new dependencies** - The SQLite backup API is already available through `mattn/go-sqlite3`
2. **Works with ANY cloud storage** - Dropbox, GDrive desktop, Syncthing, OneDrive, rsync, NFS mount, USB drive
3. **No API keys or OAuth** - No GCP project, no credentials, no token refresh logic
4. **Human-readable sidecar** - The JSON export lets users inspect/debug their data
5. **Existing code reuse** - JSON export reuses `StorageExportResponse` format from `handlers_sync.go`
6. **Manual fallback** - Settings UI provides download/restore for users without Docker or cloud folders
7. **Cheapskate score: 10/10** - Maximum capability for minimum complexity
