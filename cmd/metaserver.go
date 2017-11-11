package cmd

import (
	"flag"
	"log"
	"net/http"
	"skybin/core"
	"skybin/metaserver"
)

var metaServerCmd = Cmd{
	Name:        "metaserver",
	Description: "Start a metadata server",
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

	server := metaserver.NewServer(".")

	log.Println("starting metaserver server at", addr)
	log.Fatal(http.ListenAndServe(addr, server))
}
