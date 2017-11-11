package cmd

import "log"

var listCmd = Cmd{
	Name: "list",
	Description: "List uploaded files",
	Usage: "list",
	Run: runList,
}

func runList(args ...string) {

	client, err := getRenterClient()
	if err != nil {
		log.Fatal(err)
	}

	files, err := client.ListFiles()
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		log.Println(file.Name)
	}
}
