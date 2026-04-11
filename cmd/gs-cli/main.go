package main

import (
	"fmt"
	"github.com/c-bata/go-prompt"
	"gs-cli/internal/repl"
	"gs-cli/internal/session"
	"log"
)

func main() {
	log.SetFlags(0)

	state := &session.State{
		CurrentMode: session.ModeNone,
	}

	executor := &repl.Executor{State: state}
	completer := &repl.Completer{State: state}

	fmt.Println("--------------------------------------------------")
	fmt.Println("   PathDB Interactive Shell (v0.1-alpha)         ")
	fmt.Println("   Type \\connect <path|port> to start           ")
	fmt.Println("   Use \\q to exit                              ")
	fmt.Println("--------------------------------------------------")

	p := prompt.New(
		executor.Execute,
		completer.Complete,
		prompt.OptionTitle("PathDB Shell"),
		prompt.OptionPrefix("pathdb(none)> "),
		prompt.OptionLivePrefix(func() (string, bool) {
			return state.GetPrompt()
		}),
	)

	p.Run()
}
