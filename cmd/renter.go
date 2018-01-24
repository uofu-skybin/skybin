package cmd

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path"
	"skybin/renter"
	"skybin/metaserver"
	"skybin/core"
	"skybin/util"
	"io/ioutil"
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

	if !r.Config.IsRegistered {
		pubKeyBytes, err := ioutil.ReadFile(r.Config.PublicKeyFile)
		if err != nil {
			log.Fatal("Unable to read public key file. Error: ", err)
		}
		info := core.RenterInfo{
			ID: r.Config.RenterId,
			PublicKey: string(pubKeyBytes),
		}
		metaService := metaserver.NewClient(r.Config.MetaAddr, &http.Client{})
		err = metaService.RegisterRenter(&info)
		if err != nil {
			log.Fatal("Unable to register with metaserver. Error: ", err)
		}
		r.Config.IsRegistered = true
		err = util.SaveJson(path.Join(homedir, "renter", "config.json"), r.Config)
		if err != nil {
			log.Fatal("Unable to update config file. Error: ", err)
		}
	}

	addr := r.Config.ApiAddr
	if len(*addrFlag) > 0 {
		addr = *addrFlag
	}

	logfile, err := os.OpenFile(path.Join(homedir, "renter", "renter.log"),
		os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Cannot create log file: %s", err)
	}
	defer logfile.Close()
	logger := log.New(logfile, "", log.LstdFlags)

	server := renter.NewServer(r, logger)

	log.Println("starting renter server at", addr)
	log.Fatal(http.ListenAndServe(addr, server))
}
