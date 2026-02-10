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

// ParsedRemoveCommand represents a parsed "remove" command from user input
type ParsedRemoveCommand struct {
	Amount      int64  // Cents
	Description string // Optional description filter
}

var (
	// Matches "50 pizza" or "50.50 taxi"
	reSimple = regexp.MustCompile(`^(\d+(?:\.\d{1,2})?)\s+(.+)$`)
	// Matches "remove 50" or "remove 50.50" or "remove 50 pizza"
	reRemove = regexp.MustCompile(`(?i)^remove\s+(\d+(?:\.\d{1,2})?)(?:\s+(.+))?$`)
)

// IsRemoveCommand checks if the input is a remove command
func IsRemoveCommand(input string) bool {
	return reRemove.MatchString(strings.TrimSpace(input))
}

// ParseRemoveCommand parses a "remove <amount> [description]" command
func ParseRemoveCommand(input string) (ParsedRemoveCommand, error) {
	input = strings.TrimSpace(input)

	matches := reRemove.FindStringSubmatch(input)
	if matches == nil {
		return ParsedRemoveCommand{}, errors.New("not a valid remove command")
	}

	amount, err := parseAmount(matches[1])
	if err != nil {
		return ParsedRemoveCommand{}, err
	}

	desc := ""
	if len(matches) > 2 {
		desc = strings.TrimSpace(matches[2])
	}

	return ParsedRemoveCommand{
		Amount:      amount,
		Description: desc,
	}, nil
}

func ParseTransaction(input string, catConfig *CategoryConfig) (ParsedTransaction, error) {
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
			Category:    catConfig.InferCategory(desc),
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

