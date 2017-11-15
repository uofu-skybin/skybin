package cmd

import (
	"log"
	"path"
	"path/filepath"
)

var uploadCmd = Cmd{
	Name:        "upload",
	Description: "Upload a file",
	Usage:       "upload <filename> [destination]",
	Run:         runUpload,
}

func runUpload(args ...string) {

	if len(args) < 1 {
		log.Fatal("Must provide filename")
	}

	filename, err := filepath.Abs(args[0])
	if err != nil {
		log.Fatal(err)
	}

	destpath := path.Base(filename)
	if len(args) == 2 {
		destpath = args[1]
	}

	client, err := getRenterClient()
	if err != nil {
		log.Fatal(err)
	}

	_, err = client.Upload(filename, destpath)
	if err != nil {
		log.Fatal(err)
	}

}
