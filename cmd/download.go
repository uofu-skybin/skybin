package cmd

import (
	"log"
	"path"
	"os"
	"path/filepath"
)

var downloadCmd = Cmd{
	Name: "download",
	Description: "Download a file",
	Usage: "download <filename> [destination]",
	Run: runDownload,
}

func runDownload(args ...string) {
	if len(args) < 1 {
		log.Fatal("must provide filename")
	}

	filename := args[0]
	var destination string
	if len(args) == 2 {
		var err error
		destination, err = filepath.Abs(args[1])
		if err != nil {
			log.Fatal(err)
		}
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			log.Fatal(err)
		}
		destination = path.Join(cwd, path.Base(filename))
	}

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
	if len(fileId) == 0 {
		log.Fatalf("Cannot find file %s", filename)
	}

	err = client.Download(fileId, destination)
	if err != nil {
		log.Fatal(err)
	}
}
