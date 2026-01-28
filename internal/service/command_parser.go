package service

import (
	"regexp"
	"strings"
)

var stockCommandRegex = regexp.MustCompile(`^/stock=([a-zA-Z0-9.]{1,20})$`)

// Command represents a parsed command
type Command struct {
	Type      string
	StockCode string
}

// ParseCommand attempts to parse a message as a command
// Returns the command and true if it's a command, nil and false otherwise
func ParseCommand(content string) (*Command, bool) {
	content = strings.TrimSpace(content)

	// Check for stock command
	if matches := stockCommandRegex.FindStringSubmatch(content); matches != nil {
		return &Command{
			Type:      "stock",
			StockCode: strings.ToUpper(matches[1]),
		}, true
	}

	return nil, false
}
