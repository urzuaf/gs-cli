package main

import (
	"log"
	"os"
)

func main() {
	log.SetFlags(0)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
