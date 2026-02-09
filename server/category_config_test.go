package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCategoryConfig_FromFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-categories.json")

	configJSON := `{
		"default_category": "Misc",
		"categories": [
			{"name": "Coffee", "keywords": ["latte", "espresso", "cappuccino"]},
			{"name": "Travel", "keywords": ["hotel", "flight"]}
		]
	}`

	err := os.WriteFile(configPath, []byte(configJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg := LoadCategoryConfig(configPath)

	if cfg.DefaultCategory != "Misc" {
		t.Errorf("DefaultCategory = %q, want %q", cfg.DefaultCategory, "Misc")
	}
	if len(cfg.Categories) != 2 {
		t.Fatalf("len(Categories) = %d, want 2", len(cfg.Categories))
	}
	if cfg.Categories[0].Name != "Coffee" {
		t.Errorf("Categories[0].Name = %q, want %q", cfg.Categories[0].Name, "Coffee")
	}
	if len(cfg.Categories[0].Keywords) != 3 {
		t.Errorf("len(Categories[0].Keywords) = %d, want 3", len(cfg.Categories[0].Keywords))
	}
}

func TestLoadCategoryConfig_FileNotFound(t *testing.T) {
	cfg := LoadCategoryConfig("/nonexistent/path/categories.json")

	// Should return default config
	if cfg.DefaultCategory != "Housing" {
		t.Errorf("DefaultCategory = %q, want %q (built-in default)", cfg.DefaultCategory, "Housing")
	}
	if len(cfg.Categories) != 4 {
		t.Errorf("len(Categories) = %d, want 4 (built-in defaults)", len(cfg.Categories))
	}
}

func TestLoadCategoryConfig_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bad.json")

	err := os.WriteFile(configPath, []byte("not valid json {{{"), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg := LoadCategoryConfig(configPath)

	// Should return default config on parse error
	if cfg.DefaultCategory != "Housing" {
		t.Errorf("DefaultCategory = %q, want %q (built-in default)", cfg.DefaultCategory, "Housing")
	}
}

func TestCategoryConfig_InferCategory(t *testing.T) {
	cfg := &CategoryConfig{
		DefaultCategory: "Unknown",
		Categories: []CategoryEntry{
			{Name: "Income", Keywords: []string{"salary", "bonus"}},
			{Name: "Food", Keywords: []string{"pizza", "burger", "coffee"}},
			{Name: "Transport", Keywords: []string{"taxi", "bus", "uber"}},
		},
	}

	tests := []struct {
		name  string
		desc  string
		want  string
	}{
		{name: "matches income", desc: "monthly salary", want: "Income"},
		{name: "matches food", desc: "pizza delivery", want: "Food"},
		{name: "matches transport", desc: "uber ride home", want: "Transport"},
		{name: "case insensitive", desc: "PIZZA HUT", want: "Food"},
		{name: "mixed case", desc: "My Salary Deposit", want: "Income"},
		{name: "no match uses default", desc: "random purchase", want: "Unknown"},
		{name: "empty string uses default", desc: "", want: "Unknown"},
		{name: "priority order matters", desc: "bonus pizza", want: "Income"}, // bonus is in Income which comes first
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.InferCategory(tt.desc)
			if got != tt.want {
				t.Errorf("InferCategory(%q) = %q, want %q", tt.desc, got, tt.want)
			}
		})
	}
}

func TestDefaultCategoryConfig(t *testing.T) {
	cfg := defaultCategoryConfig()

	if cfg.DefaultCategory != "Housing" {
		t.Errorf("Default config DefaultCategory = %q, want %q", cfg.DefaultCategory, "Housing")
	}

	// Should have the 4 original categories
	expectedNames := []string{"Earned Income", "Food", "Transport", "Housing"}
	if len(cfg.Categories) != len(expectedNames) {
		t.Fatalf("Default config has %d categories, want %d", len(cfg.Categories), len(expectedNames))
	}

	for i, name := range expectedNames {
		if cfg.Categories[i].Name != name {
			t.Errorf("Categories[%d].Name = %q, want %q", i, cfg.Categories[i].Name, name)
		}
		if len(cfg.Categories[i].Keywords) == 0 {
			t.Errorf("Categories[%d] (%s) has no keywords", i, name)
		}
	}
}
