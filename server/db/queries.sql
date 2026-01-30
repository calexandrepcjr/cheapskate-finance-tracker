-- name: GetUser :one
SELECT id, name, email, created_at FROM users
WHERE id = ? LIMIT 1;

-- name: ListUsers :many
SELECT id, name, email, created_at FROM users
ORDER BY name;

-- name: CreateTransaction :one
INSERT INTO transactions (
  user_id, category_id, amount, currency, description, date
) VALUES (
  ?, ?, ?, ?, ?, ?
)
RETURNING id, user_id, category_id, amount, currency, description, date, created_at;

-- name: ListRecentTransactions :many
SELECT t.id, t.user_id, t.category_id, t.amount, t.currency, t.description, t.date, t.created_at, c.name as category_name, c.icon as category_icon, u.name as user_name
FROM transactions t
JOIN categories c ON t.category_id = c.id
JOIN users u ON t.user_id = u.id
ORDER BY t.date DESC
LIMIT 20;

-- name: GetCategoryByName :one
SELECT id, name, type, icon, color FROM categories
WHERE name = ? LIMIT 1;

-- name: ListCategories :many
SELECT id, name, type, icon, color FROM categories
ORDER BY type, name;

-- name: GetCategoryStats :many
SELECT 
    c.name, 
    c.icon,
    c.color,
    c.type,
    COALESCE(SUM(t.amount), 0) as total_amount
FROM categories c
LEFT JOIN transactions t ON c.id = t.category_id 
    AND t.date >= ? -- Filter by start date (e.g. beginning of month)
GROUP BY c.id;

-- name: DeleteTransaction :exec
DELETE FROM transactions
WHERE id = ? AND user_id = ?;
