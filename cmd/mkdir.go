package cmd

import (
	"log"
)

var mkdirCmd = Cmd{
	Name:        "mkdir",
	Description: "Create a folder",
	Usage:       "mkdir <name>",
	Run:         runMkdir,
}

func runMkdir(args ...string) {
	if len(args) < 1 {
		log.Fatal("Must provide foldername")
	}

	foldername := args[0]

	client, err := getRenterClient()
	if err != nil {
		log.Fatal(err)
	}

	_, err = client.CreateFolder(foldername)
	if err != nil {
		log.Fatal(err)
	}
}
