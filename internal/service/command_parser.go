package service

import (
	"regexp"
	"strings"
)

var stockCommandRegex = regexp.MustCompile(`^/stock=([a-zA-Z0-9.]{1,20})$`)
var helloCommandRegex = regexp.MustCompile(`^/hello$`)

type Command struct {
	Type      string
	StockCode string
}

// ParseCommand parses a message as a command, returning the command and true if valid
func ParseCommand(content string) (*Command, bool) {
	content = strings.TrimSpace(content)

	if matches := stockCommandRegex.FindStringSubmatch(content); matches != nil {
		return &Command{
			Type:      "stock",
			StockCode: strings.ToUpper(matches[1]),
		}, true
	}

	if helloCommandRegex.MatchString(content) {
		return &Command{
			Type: "hello",
		}, true
	}

	return nil, false
}
