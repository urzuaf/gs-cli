package main

import (
	"flag"
	"fmt"
	"github.com/c-bata/go-prompt"
	"gs-cli/internal/repl"
	"gs-cli/internal/session"
	"log"
	"os"
	"strings"
)

func main() {
	log.SetFlags(0)

	execCmd := flag.String("e", "", "Execute commands separated by semicolon and exit")
	flag.Parse()

	state := &session.State{
		CurrentMode: session.ModeNone,
	}

	executor := &repl.Executor{State: state}
	completer := &repl.Completer{State: state}

	if *execCmd != "" {
		commands := strings.Split(*execCmd, ";")
		for _, cmd := range commands {
			cmd = strings.TrimSpace(cmd)
			if cmd != "" {
				executor.Execute(cmd)
			}
		}
		os.Exit(0)
	}

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
