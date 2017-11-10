package main

import (
	"flag"
	"skybin/core"
	"skybin/metaserver"
	"net/http"
	"log"
)

var metaServerCmd = Cmd{
	Name: "metaserver",
	Description: "Run a metaserver server",
	Run: runMetaServer,
}

func runMetaServer(args ...string) {
	fs := flag.NewFlagSet("metaserver", flag.ExitOnError)
	addrFlag := fs.String("addr", "", "address to run on (host:port)")
	fs.Parse(args)

	addr := core.DefaultMetaAddr
	if len(*addrFlag) > 0 {
		addr = *addrFlag
	}

	server := metaserver.NewServer()

	log.Println("starting metaserver server at", addr)
	log.Fatal(http.ListenAndServe(addr, server))
}
