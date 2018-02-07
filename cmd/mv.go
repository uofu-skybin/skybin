package cmd

import (
	"log"
	"skybin/core"
)

var mvCmd = Cmd{
	Name:        "mv",
	Description: "Rename a file or folder",
	Usage:       "mv <old-name> <new-name>",
	Run:         runMv,
}

func runMv(args ...string) {
	if len(args) != 2 {
		log.Fatal("Must provide <old-name> and <new-name>")
	}
	oldName := args[0]
	newName := args[1]

	client, err := getRenterClient()
	if err != nil {
		log.Fatal(err)
	}

	files, err := client.ListFiles()
	if err != nil {
		log.Fatal(err)
	}

	var file *core.File
	for _, f := range files {
		if f.Name == oldName {
			file = f
			break
		}
	}
	if file == nil {
		log.Fatal("Cannot find file ", oldName)
	}
	err = client.RenameFile(file.ID, newName)
	if err != nil {
		log.Fatal(err)
	}
}
