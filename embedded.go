// Package cheapskate provides embedded assets for the Cheapskate Finance Tracker.
// This file lives at the project root so it can embed files from any subdirectory.
package cheapskate

import "embed"

// SchemaSQL contains the database schema definition.
//
//go:embed server/db/schema.sql
var SchemaSQL string

// CategoriesJSON contains the default category mappings config.
// This embed may fail if the file doesn't exist; that's handled at runtime.
//
//go:embed categories.json
var CategoriesJSON []byte

// ClientAssets contains all static files served under /assets/.
//
//go:embed client/assets
var ClientAssets embed.FS
