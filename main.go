package main

import (
	"fmt"
	"log"
	"os"
	"text/tabwriter"
)

type Cmd struct {
	Name        string
	Usage       string
	Description string
	Run         func(args ...string)
}

var commands = []Cmd{
	initCmd,
	renterCmd,
}

func usage() {
	fmt.Printf("usage: %s <command> [option...]\n", os.Args[0])
	fmt.Println()
	fmt.Println("\tInteract with skybin")
	fmt.Println()
	fmt.Println("commands:")

	tw := tabwriter.NewWriter(os.Stdout, 0, 5, 5, ' ', 0)

	for _, cmd := range commands {
		fmt.Fprintf(tw, "\t%s\t%s\n", cmd.Name, cmd.Description)
	}

	tw.Flush()

	fmt.Println()
	os.Exit(1)
}

func main() {
	log.SetFlags(0)

	if len(os.Args) < 2 {
		usage()
	}

	for _, command := range commands {
		if command.Name == os.Args[1] {
			command.Run(os.Args[2:]...)
			return
		}
	}

	usage()
}
