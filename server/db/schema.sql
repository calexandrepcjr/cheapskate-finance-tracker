CREATE TABLE users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  email TEXT NOT NULL UNIQUE,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE categories (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  type TEXT NOT NULL CHECK(type IN ('income', 'expense')),
  icon TEXT, -- Emoji or icon class
  color TEXT -- Hex code for UI
);

CREATE TABLE transactions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  category_id INTEGER NOT NULL,
  amount INTEGER NOT NULL, -- Stored in cents (e.g. 100 = $1.00)
  currency TEXT NOT NULL DEFAULT 'USD',
  description TEXT NOT NULL,
  date DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id),
  FOREIGN KEY (category_id) REFERENCES categories(id)
);

-- Seed some default categories
INSERT INTO categories (name, type, icon, color) VALUES
('Food', 'expense', '🍔', '#FF5733'),
('Transport', 'expense', '🚕', '#33C1FF'),
('Housing', 'expense', '🏠', '#8D33FF'),
('Earned Income', 'income', '💰', '#2ECC71');
