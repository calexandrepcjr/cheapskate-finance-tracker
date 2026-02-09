package main

import (
	"encoding/json"
	"log"
	"os"
	"strings"
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
// If the file doesn't exist, returns the built-in default config.
func LoadCategoryConfig(path string) *CategoryConfig {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("Category config file not found at %q, using built-in defaults", path)
		return defaultCategoryConfig()
	}

	var cfg CategoryConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("Failed to parse category config %q: %v, using built-in defaults", path, err)
		return defaultCategoryConfig()
	}

	log.Printf("Loaded %d category mappings from %s", len(cfg.Categories), path)
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
