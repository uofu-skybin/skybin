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

	addr := r.Config.Addr
	if len(*addrFlag) > 0 {
		addr = *addrFlag
	}

	server := renter.NewServer(r, log.New(os.Stdout, "", log.LstdFlags))

	log.Println("starting renter server at", addr)
	log.Fatal(http.ListenAndServe(addr, server))
}
