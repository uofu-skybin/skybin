package main

import (
	"log"
	"os"
	"skybin/cmd"
)

func main() {
	log.SetFlags(0)

	if len(os.Args) < 2 {
		cmd.PrintUsage()
		os.Exit(1)
	}

	for _, command := range cmd.Commands {
		if command.Name == os.Args[1] {
			command.Run(os.Args[2:]...)
			return
		}
	}

	cmd.PrintUsage()
}
