package cmd

import "log"

var rmCmd = Cmd{
	Name:        "rm",
	Description: "Remove a file from skybin",
	Usage:       "rm <filename> [-r]",
	Run:         runRm,
}

func runRm(args ...string) {
	if len(args) < 1 {
		log.Fatal("Must give filename")
	}

	recursive := false
	for _, arg := range args {
		if arg == "-r" {
			recursive = true
			break
		}
	}

	filename := args[0]

	client, err := getRenterClient()
	if err != nil {
		log.Fatal(err)
	}

	files, err := client.ListFiles()
	if err != nil {
		log.Fatal(err)
	}

	var fileId string
	for _, file := range files {
		if file.Name == filename {
			fileId = file.ID
			break
		}
	}

	err = client.Remove(fileId, &recursive)
	if err != nil {
		log.Fatal(err)
	}

}
