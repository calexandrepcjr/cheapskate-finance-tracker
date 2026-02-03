package main

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

type ParsedTransaction struct {
	Amount      int64 // Cents
	Description string
	Category    string // Inferred or empty
}

var (
	// Matches "50 pizza" or "50.50 taxi"
	reSimple = regexp.MustCompile(`^(\d+(?:\.\d{1,2})?)\s+(.+)$`)
)

func ParseTransaction(input string) (ParsedTransaction, error) {
	input = strings.TrimSpace(input)

	// Try Regex First
	if matches := reSimple.FindStringSubmatch(input); matches != nil {
		amountStr := matches[1]
		desc := matches[2]

		amount, err := parseAmount(amountStr)
		if err != nil {
			return ParsedTransaction{}, err
		}

		return ParsedTransaction{
			Amount:      amount,
			Description: strings.TrimSpace(desc),
			Category:    inferCategory(desc), // Simple keyword matching for now
		}, nil
	}

	// TODO: Fallback to LLM here
	return ParsedTransaction{}, errors.New("could not parse input")
}

func parseAmount(s string) (int64, error) {
	// Simple float parsing to cents
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return int64(f * 100), nil
}

func inferCategory(desc string) string {
	desc = strings.ToLower(desc)

	// Income keywords - check first
	incomeKeywords := []string{"salary", "paycheck", "income", "wage", "bonus", "freelance", "dividend", "interest", "refund"}
	for _, kw := range incomeKeywords {
		if strings.Contains(desc, kw) {
			return "Earned Income"
		}
	}

	// Food keywords
	foodKeywords := []string{"pizza", "food", "burger", "grocery", "groceries", "restaurant", "lunch", "dinner", "breakfast", "coffee", "cafe", "snack", "meal", "takeout", "delivery", "doordash", "ubereats", "grubhub"}
	for _, kw := range foodKeywords {
		if strings.Contains(desc, kw) {
			return "Food"
		}
	}

	// Transport keywords
	transportKeywords := []string{"taxi", "uber", "bus", "gas", "fuel", "lyft", "metro", "subway", "train", "parking", "toll", "car", "auto", "vehicle", "flight", "airline", "ticket"}
	for _, kw := range transportKeywords {
		if strings.Contains(desc, kw) {
			return "Transport"
		}
	}

	// Housing keywords (explicit match before defaulting)
	housingKeywords := []string{"rent", "mortgage", "electricity", "electric", "water", "internet", "wifi", "cable", "phone", "utility", "utilities", "insurance", "maintenance", "repair", "furniture", "appliance"}
	for _, kw := range housingKeywords {
		if strings.Contains(desc, kw) {
			return "Housing"
		}
	}

	return "Housing" // Default fallback for unrecognized expenses
}
