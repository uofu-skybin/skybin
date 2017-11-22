package cmd

import (
	"log"
	"os"
)

var helpCmd = Cmd{
	Name:        "help",
	Usage:       "help [command]",
	Description: "Print command usage",
	Run:         runHelp,
}

func runHelp(args ...string) {
	if len(args) == 0 {
		PrintUsage()
		return
	}

	name := args[0]

	for _, cmd := range Commands {
		if cmd.Name == name {
			log.Println()
			log.Printf("%s - %s\n", cmd.Name, cmd.Description)
			log.Println()
			log.Printf("usage: %s %s\n", os.Args[0], cmd.Usage)
			log.Println()
			return
		}
	}

	log.Printf("Unrecognized command '%s'\n", name)
}
