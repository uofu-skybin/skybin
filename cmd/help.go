package cmd

import (
	"log"
	"os"
	"strings"
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

	var command *Cmd
	commands := Commands
	for _, arg := range args {
		command = nil
		for _, cmd := range commands {
			if cmd.Name == arg {
				command = cmd
				commands = cmd.Subcommands
				break
			}
		}
		if command == nil {
			log.Fatalf("Unrecognized command '%s'\n", strings.Join(args, " "))
		}
	}

	fullName := strings.Join(args, " ")

	log.Println()
	log.Printf("%s - %s\n", fullName, command.Description)
	log.Println()
	log.Printf("usage: %s %s\n", os.Args[0], command.Usage)
}
