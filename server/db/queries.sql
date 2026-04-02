-- name: GetUser :one
SELECT * FROM users
WHERE id = ? LIMIT 1;

-- name: ListUsers :many
SELECT * FROM users
ORDER BY name;

-- name: CreateTransaction :one
INSERT INTO transactions (
  user_id, category_id, amount, currency, description, date
) VALUES (
  ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: ListRecentTransactions :many
SELECT t.*, c.name as category_name, c.icon as category_icon, u.name as user_name
FROM transactions t
JOIN categories c ON t.category_id = c.id
JOIN users u ON t.user_id = u.id
WHERE t.deleted_at IS NULL
ORDER BY t.date DESC
LIMIT 20;

-- name: GetCategoryByName :one
SELECT * FROM categories
WHERE name = ? LIMIT 1;

-- name: ListCategories :many
SELECT * FROM categories
ORDER BY type, name;

-- name: GetDistinctTransactionYears :many
SELECT DISTINCT CAST(strftime('%Y', date) AS INTEGER) as year
FROM transactions
WHERE deleted_at IS NULL
ORDER BY year DESC;

-- name: ListTransactionsByYear :many
SELECT t.*, c.name as category_name, c.icon as category_icon, c.type as category_type, u.name as user_name
FROM transactions t
JOIN categories c ON t.category_id = c.id
JOIN users u ON t.user_id = u.id
WHERE strftime('%Y', t.date) = CAST(? AS TEXT)
AND t.deleted_at IS NULL
ORDER BY t.date DESC;

-- name: ListTransactionsByYearPaginated :many
SELECT t.*, c.name as category_name, c.icon as category_icon, c.type as category_type, u.name as user_name
FROM transactions t
JOIN categories c ON t.category_id = c.id
JOIN users u ON t.user_id = u.id
WHERE strftime('%Y', t.date) = CAST(sqlc.arg(year) AS TEXT)
AND t.deleted_at IS NULL
ORDER BY t.date DESC
LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset);

-- name: CountTransactionsByYear :one
SELECT COUNT(*) as count
FROM transactions t
WHERE strftime('%Y', t.date) = CAST(? AS TEXT)
AND t.deleted_at IS NULL;

-- name: GetCategoryTotalsByYear :many
SELECT
    c.id as category_id,
    c.name as category_name,
    c.icon as category_icon,
    c.type as category_type,
    c.color as category_color,
    CAST(COALESCE(SUM(ABS(t.amount)), 0) AS INTEGER) as total_amount,
    COUNT(t.id) as transaction_count
FROM categories c
LEFT JOIN transactions t ON t.category_id = c.id AND strftime('%Y', t.date) = CAST(? AS TEXT) AND t.deleted_at IS NULL
GROUP BY c.id, c.name, c.icon, c.type, c.color
ORDER BY c.type, total_amount DESC;

-- name: GetMonthlyTotalsByYear :many
SELECT
    CAST(strftime('%m', date) AS INTEGER) as month,
    c.type as category_type,
    CAST(COALESCE(SUM(ABS(amount)), 0) AS INTEGER) as total_amount
FROM transactions t
JOIN categories c ON t.category_id = c.id
WHERE strftime('%Y', t.date) = CAST(? AS TEXT)
AND t.deleted_at IS NULL
GROUP BY month, c.type
ORDER BY month;

-- name: DeleteTransaction :exec
DELETE FROM transactions
WHERE id = ? AND user_id = ?;

-- name: SoftDeleteTransaction :exec
UPDATE transactions
SET deleted_at = CURRENT_TIMESTAMP
WHERE id = ? AND user_id = ? AND deleted_at IS NULL;

-- name: RestoreTransaction :exec
UPDATE transactions
SET deleted_at = NULL
WHERE id = ? AND user_id = ?;

-- name: CountAllTransactions :one
SELECT COUNT(*) as count FROM transactions WHERE deleted_at IS NULL;

-- name: ListAllTransactionsForExport :many
SELECT t.id, t.amount, t.currency, t.description, t.date, c.name as category_name, c.type as category_type
FROM transactions t
JOIN categories c ON t.category_id = c.id
WHERE t.deleted_at IS NULL
ORDER BY t.date DESC;

-- name: DeleteAllTransactions :exec
DELETE FROM transactions;

-- name: SearchTransactionsForRemoval :many
SELECT t.*, c.name as category_name, c.icon as category_icon, c.type as category_type, u.name as user_name
FROM transactions t
JOIN categories c ON t.category_id = c.id
JOIN users u ON t.user_id = u.id
WHERE ABS(t.amount) = ?
AND t.deleted_at IS NULL
AND t.user_id = ?
ORDER BY t.date DESC
LIMIT 10;

-- name: ListTransactionsByYearPaginatedWithDeleted :many
SELECT t.*, c.name as category_name, c.icon as category_icon, c.type as category_type, u.name as user_name
FROM transactions t
JOIN categories c ON t.category_id = c.id
JOIN users u ON t.user_id = u.id
WHERE strftime('%Y', t.date) = CAST(sqlc.arg(year) AS TEXT)
ORDER BY t.date DESC
LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset);

-- name: CountTransactionsByYearWithDeleted :one
SELECT COUNT(*) as count
FROM transactions t
WHERE strftime('%Y', t.date) = CAST(? AS TEXT);

-- name: GetTopUsedCategories :many
SELECT c.id, c.name, c.type, c.icon, c.color, COUNT(t.id) as usage_count
FROM categories c
LEFT JOIN transactions t ON t.category_id = c.id AND t.deleted_at IS NULL AND t.user_id = ?
GROUP BY c.id, c.name, c.type, c.icon, c.color
ORDER BY usage_count DESC, c.name ASC
LIMIT ?;

-- name: GetGdriveConfig :one
SELECT * FROM gdrive_config LIMIT 1;

-- name: UpsertGdriveConfig :exec
INSERT INTO gdrive_config (id, enabled, folder_id, folder_name, auto_backup, updated_at)
VALUES (1, ?, ?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(id) DO UPDATE SET
  enabled = excluded.enabled,
  folder_id = excluded.folder_id,
  folder_name = excluded.folder_name,
  auto_backup = excluded.auto_backup,
  updated_at = CURRENT_TIMESTAMP;

-- name: UpdateGdriveLastSync :exec
UPDATE gdrive_config
SET last_sync_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = 1;

-- name: DeleteGdriveConfig :exec
UPDATE gdrive_config
SET enabled = 0,
    folder_id = NULL,
    folder_name = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE id = 1;

-- name: GetBackupMetadata :one
SELECT * FROM backup_metadata ORDER BY created_at DESC LIMIT 1;

-- name: InsertBackupMetadata :exec
INSERT INTO backup_metadata (version, schema_version, checksum)
VALUES (?, ?, ?);
