package tui

import "strings"

// Command represents a parsed slash command.
type Command struct {
	Name string
	Args []string
}

// ParseCommand parses a slash command string into a Command.
// Returns nil if the input is not a valid command.
func ParseCommand(input string) *Command {
	input = strings.TrimSpace(input)
	if input == "" || input[0] != '/' {
		return nil
	}

	parts := strings.Fields(input)
	return &Command{
		Name: parts[0],
		Args: parts[1:],
	}
}
