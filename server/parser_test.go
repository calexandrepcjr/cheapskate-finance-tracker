package main

import (
	"testing"
)

// testCategoryConfig returns the built-in default config for testing
func testCategoryConfig() *CategoryConfig {
	return defaultCategoryConfig()
}

func TestParseTransaction(t *testing.T) {
	catConfig := testCategoryConfig()

	tests := []struct {
		name       string
		input      string
		wantAmount int64
		wantDesc   string
		wantCat    string
		wantErr    bool
	}{
		{
			name:       "simple integer amount",
			input:      "50 pizza",
			wantAmount: 5000,
			wantDesc:   "pizza",
			wantCat:    "Food",
			wantErr:    false,
		},
		{
			name:       "decimal amount with two places",
			input:      "12.50 taxi ride",
			wantAmount: 1250,
			wantDesc:   "taxi ride",
			wantCat:    "Transport",
			wantErr:    false,
		},
		{
			name:       "decimal amount with one place",
			input:      "9.5 coffee",
			wantAmount: 950,
			wantDesc:   "coffee",
			wantCat:    "Food", // coffee is a food keyword
			wantErr:    false,
		},
		{
			name:       "leading and trailing spaces",
			input:      "  25 groceries  ",
			wantAmount: 2500,
			wantDesc:   "groceries",
			wantCat:    "Food", // groceries is a food keyword
			wantErr:    false,
		},
		{
			name:       "large amount",
			input:      "999999.99 rent payment",
			wantAmount: 99999999,
			wantDesc:   "rent payment",
			wantCat:    "Housing",
			wantErr:    false,
		},
		{
			name:       "zero amount",
			input:      "0 free sample",
			wantAmount: 0,
			wantDesc:   "free sample",
			wantCat:    "Housing",
			wantErr:    false,
		},
		{
			name:       "uber keyword triggers transport",
			input:      "15 uber to airport",
			wantAmount: 1500,
			wantDesc:   "uber to airport",
			wantCat:    "Transport",
			wantErr:    false,
		},
		{
			name:       "bus keyword triggers transport",
			input:      "2.50 bus fare",
			wantAmount: 250,
			wantDesc:   "bus fare",
			wantCat:    "Transport",
			wantErr:    false,
		},
		{
			name:       "food keyword triggers food",
			input:      "30 food delivery",
			wantAmount: 3000,
			wantDesc:   "food delivery",
			wantCat:    "Food",
			wantErr:    false,
		},
		{
			name:       "burger keyword triggers food",
			input:      "8.99 burger king",
			wantAmount: 899,
			wantDesc:   "burger king",
			wantCat:    "Food",
			wantErr:    false,
		},
		// Error cases
		{
			name:    "missing description",
			input:   "50",
			wantErr: true,
		},
		{
			name:    "missing amount",
			input:   "pizza",
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:    "only whitespace",
			input:   "   ",
			wantErr: true,
		},
		{
			name:    "amount with too many decimal places",
			input:   "12.345 something",
			wantErr: true,
		},
		{
			name:    "negative amount format",
			input:   "-50 refund",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTransaction(tt.input, catConfig)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseTransaction(%q) expected error, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseTransaction(%q) unexpected error: %v", tt.input, err)
				return
			}

			if got.Amount != tt.wantAmount {
				t.Errorf("ParseTransaction(%q).Amount = %d, want %d", tt.input, got.Amount, tt.wantAmount)
			}
			if got.Description != tt.wantDesc {
				t.Errorf("ParseTransaction(%q).Description = %q, want %q", tt.input, got.Description, tt.wantDesc)
			}
			if got.Category != tt.wantCat {
				t.Errorf("ParseTransaction(%q).Category = %q, want %q", tt.input, got.Category, tt.wantCat)
			}
		})
	}
}

func TestParseAmount(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{
			name:  "integer",
			input: "50",
			want:  5000,
		},
		{
			name:  "decimal two places",
			input: "12.50",
			want:  1250,
		},
		{
			name:  "decimal one place",
			input: "9.5",
			want:  950,
		},
		{
			name:  "zero",
			input: "0",
			want:  0,
		},
		{
			name:  "large number",
			input: "999999.99",
			want:  99999999,
		},
		{
			name:  "small decimal",
			input: "0.01",
			want:  1,
		},
		{
			name:    "invalid string",
			input:   "abc",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "special characters",
			input:   "$50",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAmount(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("parseAmount(%q) expected error, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("parseAmount(%q) unexpected error: %v", tt.input, err)
				return
			}

			if got != tt.want {
				t.Errorf("parseAmount(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestInferCategory(t *testing.T) {
	catConfig := testCategoryConfig()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		// Food keywords
		{name: "pizza lowercase", input: "pizza delivery", want: "Food"},
		{name: "pizza uppercase", input: "PIZZA HUT", want: "Food"},
		{name: "pizza mixed case", input: "Pizza Party", want: "Food"},
		{name: "food keyword", input: "food court", want: "Food"},
		{name: "burger keyword", input: "burger and fries", want: "Food"},

		// Transport keywords
		{name: "taxi keyword", input: "taxi to work", want: "Transport"},
		{name: "uber keyword", input: "uber ride", want: "Transport"},
		{name: "uber uppercase", input: "UBER EATS", want: "Transport"}, // Note: uber eats is transport due to keyword order
		{name: "bus keyword", input: "bus ticket", want: "Transport"},

		// Default fallback
		{name: "no matching keyword", input: "random purchase", want: "Housing"},
		{name: "utilities", input: "electricity bill", want: "Housing"},
		{name: "empty description", input: "", want: "Housing"},
		{name: "numbers only", input: "12345", want: "Housing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := catConfig.InferCategory(tt.input)
			if got != tt.want {
				t.Errorf("InferCategory(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestInferCategoryWithCustomConfig(t *testing.T) {
	customConfig := &CategoryConfig{
		DefaultCategory: "Other",
		Categories: []CategoryEntry{
			{Name: "Drinks", Keywords: []string{"coffee", "tea", "soda"}},
			{Name: "Work", Keywords: []string{"office", "meeting"}},
		},
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "matches drinks", input: "morning coffee", want: "Drinks"},
		{name: "matches work", input: "office supplies", want: "Work"},
		{name: "falls back to default", input: "random thing", want: "Other"},
		{name: "case insensitive", input: "COFFEE SHOP", want: "Drinks"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := customConfig.InferCategory(tt.input)
			if got != tt.want {
				t.Errorf("InferCategory(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatMoney(t *testing.T) {
	tests := []struct {
		name  string
		cents int64
		want  string
	}{
		{name: "standard amount", cents: 1250, want: "$12.50"},
		{name: "zero", cents: 0, want: "$0.00"},
		{name: "one cent", cents: 1, want: "$0.01"},
		{name: "one dollar", cents: 100, want: "$1.00"},
		{name: "large amount", cents: 100000, want: "$1000.00"},
		{name: "very large amount", cents: 99999999, want: "$999999.99"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatMoney(tt.cents)
			if got != tt.want {
				t.Errorf("formatMoney(%d) = %q, want %q", tt.cents, got, tt.want)
			}
		})
	}
}

func TestIsRemoveCommand(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "simple remove", input: "remove 50", want: true},
		{name: "remove with decimal", input: "remove 50.50", want: true},
		{name: "remove with description", input: "remove 50 pizza", want: true},
		{name: "remove case insensitive", input: "Remove 100", want: true},
		{name: "REMOVE uppercase", input: "REMOVE 25 taxi", want: true},
		{name: "not a remove command", input: "50 pizza", want: false},
		{name: "remove without amount", input: "remove pizza", want: false},
		{name: "empty string", input: "", want: false},
		{name: "just remove", input: "remove", want: false},
		{name: "remove with spaces", input: "  remove 50  ", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRemoveCommand(tt.input)
			if got != tt.want {
				t.Errorf("IsRemoveCommand(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseRemoveCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantAmt  int64
		wantDesc string
		wantErr  bool
	}{
		{name: "remove with integer amount", input: "remove 50", wantAmt: 5000, wantDesc: ""},
		{name: "remove with decimal amount", input: "remove 12.50", wantAmt: 1250, wantDesc: ""},
		{name: "remove with description", input: "remove 50 pizza", wantAmt: 5000, wantDesc: "pizza"},
		{name: "remove with multi-word description", input: "remove 25 taxi to work", wantAmt: 2500, wantDesc: "taxi to work"},
		{name: "case insensitive", input: "REMOVE 100 groceries", wantAmt: 10000, wantDesc: "groceries"},
		{name: "leading/trailing spaces", input: "  remove 30 coffee  ", wantAmt: 3000, wantDesc: "coffee"},
		{name: "not a remove command", input: "50 pizza", wantErr: true},
		{name: "empty string", input: "", wantErr: true},
		{name: "remove without amount", input: "remove pizza", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRemoveCommand(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseRemoveCommand(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseRemoveCommand(%q) unexpected error: %v", tt.input, err)
				return
			}
			if got.Amount != tt.wantAmt {
				t.Errorf("ParseRemoveCommand(%q).Amount = %d, want %d", tt.input, got.Amount, tt.wantAmt)
			}
			if got.Description != tt.wantDesc {
				t.Errorf("ParseRemoveCommand(%q).Description = %q, want %q", tt.input, got.Description, tt.wantDesc)
			}
		})
	}
}

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		name  string
		value float64
		prec  int
		want  string
	}{
		{name: "two decimal places", value: 12.5, prec: 2, want: "12.50"},
		{name: "zero", value: 0, prec: 2, want: "0.00"},
		{name: "no decimal places", value: 100, prec: 0, want: "100"},
		{name: "three decimal places", value: 3.14159, prec: 3, want: "3.142"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatFloat(tt.value, tt.prec)
			if got != tt.want {
				t.Errorf("formatFloat(%f, %d) = %q, want %q", tt.value, tt.prec, got, tt.want)
			}
		})
	}
}
