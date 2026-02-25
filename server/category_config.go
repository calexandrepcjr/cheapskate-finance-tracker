package main

import (
	"encoding/json"
	"log"
	"os"
	"strings"

	cheapskate "github.com/calexandrepcjr/cheapskate-finance-tracker"
)

type CategoryEntry struct {
	Name     string   `json:"name"`
	Keywords []string `json:"keywords"`
}

type CategoryConfig struct {
	DefaultCategory string          `json:"default_category"`
	Categories      []CategoryEntry `json:"categories"`
}

// LoadCategoryConfig loads category mappings from a JSON file.
// It first tries the filesystem path, then falls back to the embedded config,
// and finally to built-in defaults.
func LoadCategoryConfig(path string) *CategoryConfig {
	// Try filesystem first (allows runtime override)
	data, err := os.ReadFile(path)
	if err != nil {
		// Fall back to embedded categories.json
		data = cheapskate.CategoriesJSON
		if len(data) == 0 {
			log.Printf("Category config not found at %q and no embedded config, using built-in defaults", path)
			return defaultCategoryConfig()
		}
		log.Printf("Using embedded category config")
	}

	var cfg CategoryConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("Failed to parse category config: %v, using built-in defaults", err)
		return defaultCategoryConfig()
	}

	log.Printf("Loaded %d category mappings", len(cfg.Categories))
	return &cfg
}

// InferCategory finds the best matching category for a description.
// Categories are checked in order, so earlier entries take priority.
func (cc *CategoryConfig) InferCategory(desc string) string {
	lower := strings.ToLower(desc)

	for _, cat := range cc.Categories {
		for _, kw := range cat.Keywords {
			if strings.Contains(lower, kw) {
				return cat.Name
			}
		}
	}

	return cc.DefaultCategory
}

// defaultCategoryConfig returns a minimal built-in config matching the original
// hardcoded behavior, used when no config file is found.
func defaultCategoryConfig() *CategoryConfig {
	return &CategoryConfig{
		DefaultCategory: "Housing",
		Categories: []CategoryEntry{
			{
				Name:     "Earned Income",
				Keywords: []string{"salary", "paycheck", "income", "wage", "bonus", "freelance", "dividend", "interest", "refund"},
			},
			{
				Name:     "Food",
				Keywords: []string{"pizza", "food", "burger", "grocery", "groceries", "restaurant", "lunch", "dinner", "breakfast", "coffee", "cafe", "snack", "meal", "takeout", "delivery", "doordash", "ubereats", "grubhub"},
			},
			{
				Name:     "Transport",
				Keywords: []string{"taxi", "uber", "bus", "gas", "fuel", "lyft", "metro", "subway", "train", "parking", "toll", "car", "auto", "vehicle", "flight", "airline", "ticket"},
			},
			{
				Name:     "Housing",
				Keywords: []string{"rent", "mortgage", "electricity", "electric", "water", "internet", "wifi", "cable", "phone", "utility", "utilities", "insurance", "maintenance", "repair", "furniture", "appliance"},
			},
		},
	}
}
