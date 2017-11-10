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

	var config renter.Config
	err = loadJSON(path.Join(homedir, "renter", "config.json"), &config)
	if err != nil {
		log.Fatalf("error: cannot read config. error: %s", err)
	}

	addr := config.Addr
	if len(*addrFlag) > 0 {
		addr = *addrFlag
	}

	r := renter.Renter{
		Config:  &config,
		Homedir: path.Join(homedir, "renter"),
	}

	server := renter.NewServer(&r, log.New(os.Stdout, "", log.LstdFlags))

	log.Println("starting renter service at", addr)
	log.Fatal(http.ListenAndServe(addr, server))
}
