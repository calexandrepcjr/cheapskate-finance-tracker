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
	if strings.Contains(desc, "pizza") || strings.Contains(desc, "food") || strings.Contains(desc, "burger") {
		return "Food"
	}
	if strings.Contains(desc, "taxi") || strings.Contains(desc, "uber") || strings.Contains(desc, "bus") {
		return "Transport"
	}
	return "Housing" // Default fallback
}
