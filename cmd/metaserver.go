package cmd

import (
	"flag"
	"log"
	"net/http"
	"skybin/core"
	"skybin/metaserver"
	"os"
	"path"
)

var metaServerCmd = Cmd{
	Name:        "metaserver",
	Description: "Start a metadata server",
	Usage:       "metaserver [-addr]",
	Run:         runMetaServer,
}

func runMetaServer(args ...string) {
	fs := flag.NewFlagSet("metaserver", flag.ExitOnError)
	addrFlag := fs.String("addr", "", "address to run on (host:port)")
	fs.Parse(args)

	addr := core.DefaultMetaAddr
	if len(*addrFlag) > 0 {
		addr = *addrFlag
	}

	logfile, err := os.OpenFile(path.Join(".", "metaserver.log"),
		os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		log.Fatalf("Cannot create log file: %s\n", err)
	}
	defer logfile.Close()
	logger := log.New(logfile, "", log.LstdFlags)

	server := metaserver.NewServer(".", logger)

	log.Println("starting metaserver server at", addr)
	log.Fatal(http.ListenAndServe(addr, server))
}
