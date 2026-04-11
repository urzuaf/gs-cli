package session

import (
	"fmt"
	"gs-cli/internal/server"
	"path/filepath"
)

type Mode int

const (
	ModeNone Mode = iota
	ModeLocal
	ModeRemote
)

type State struct {
	CurrentMode Mode
	ActiveDB    string
	ShowResults bool // Toggle to display full query data

	// Local data
	LocalRoot string

	// Remote data
	RemotePort int
	Client     *server.Client
}

func (s *State) GetPrompt() (string, bool) {
	if s.CurrentMode == ModeNone {
		return "pathdb(none)> ", false
	}

	dbName := s.ActiveDB
	if dbName == "" {
		dbName = "no-db"
	}

	if s.CurrentMode == ModeLocal {
		return fmt.Sprintf("pathdb(local:%s:%s)> ", filepath.Base(s.LocalRoot), dbName), true
	}

	return fmt.Sprintf("pathdb(server:%d:%s)> ", s.RemotePort, dbName), true
}
