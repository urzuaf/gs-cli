package repl

import (
	"github.com/c-bata/go-prompt"
	"gs-cli/internal/session"
)

type Completer struct {
	State *session.State
}

func (c *Completer) Complete(d prompt.Document) []prompt.Suggest {
	s := []prompt.Suggest{
		{Text: "\\connect", Description: "Connect to a local path or remote port"},
		{Text: "\\l", Description: "List databases"},
		{Text: "\\c", Description: "Select active database"},
		{Text: "\\i", Description: "Ingest nodes and edges (Local only)"},
		{Text: "\\create", Description: "Create a new database (Local only)"},
		{Text: "\\drop", Description: "Delete a database (Local only)"},
		{Text: "\\s", Description: "Show disk statistics (Local only)"},
		{Text: "\\d", Description: "Describe active database"},
		{Text: "\\results", Description: "Toggle detailed query results (ON/OFF)"},
		{Text: "\\q", Description: "Exit"},
	}
	return prompt.FilterHasPrefix(s, d.GetWordBeforeCursor(), true)
}
