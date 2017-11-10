package main

import (
	"fmt"
	"log"
	"os"
	"skybin/cmd"
	"text/tabwriter"
)

func usage() {
	fmt.Printf("usage: %s <command> [option...]\n", os.Args[0])
	fmt.Println()
	fmt.Println("\tInteract with skybin")
	fmt.Println()
	fmt.Println("commands:")

	tw := tabwriter.NewWriter(os.Stdout, 0, 5, 5, ' ', 0)

	for _, cmd := range cmd.Commands {
		fmt.Fprintf(tw, "\t%s\t%s\n", cmd.Name, cmd.Description)
	}

	tw.Flush()

	fmt.Println()
}

func main() {
	log.SetFlags(0)

	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	for _, command := range cmd.Commands {
		if command.Name == os.Args[1] {
			command.Run(os.Args[2:]...)
			return
		}
	}

	usage()
}
