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
ORDER BY t.date DESC
LIMIT 20;

-- name: GetCategoryByName :one
SELECT * FROM categories
WHERE name = ? LIMIT 1;

-- name: ListCategories :many
SELECT * FROM categories
ORDER BY type, name;
