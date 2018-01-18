package cmd

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path"
	"skybin/renter"
)

var renterCmd = Cmd{
	Name:        "renter",
	Description: "Start a renter server",
	Usage:       "renter [-addr]",
	Run:         runRenter,
}

func runRenter(args ...string) {
	fs := flag.NewFlagSet("renter", flag.ExitOnError)
	addrFlag := fs.String("addr", "", "address to listen at (host:port)")
	fs.Parse(args)

	homedir, err := findHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	r, err := renter.LoadFromDisk(path.Join(homedir, "renter"))
	if err != nil {
		log.Fatal(err)
	}

	addr := r.Config.ApiAddr
	if len(*addrFlag) > 0 {
		addr = *addrFlag
	}

	logfile, err := os.OpenFile(path.Join(homedir, "renter", "renter.log"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Cannot create log file: %s", err)
	}
	defer logfile.Close()
	logger := log.New(logfile, "", log.LstdFlags)

	server := renter.NewServer(r, logger)

	log.Println("starting renter server at", addr)
	log.Fatal(http.ListenAndServe(addr, server))
}
