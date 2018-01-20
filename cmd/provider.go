package cmd

import (
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"skybin/core"
	"skybin/metaserver"
	"skybin/provider"
	"skybin/util"
)

var providerCmd = Cmd{
	Name:        "provider",
	Description: "Start a provider server",
	Usage:       "provider [-addr]",
	Run:         runProvider,
}

func runProvider(args ...string) {
	fs := flag.NewFlagSet("provider", flag.ExitOnError)
	addrFlag := fs.String("addr", "", "address to listen at (host:port)")
	fs.Parse(args)

	homedir, err := findHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	p, err := provider.LoadFromDisk(path.Join(homedir, "provider"))
	if err != nil {
		log.Fatal(err)
	}

	addr := *addrFlag
	if len(addr) == 0 {
		addr = p.Config.ApiAddr
	}

	// Register with metadata service.
	if !p.Config.IsRegistered {
		pubKeyBytes, err := ioutil.ReadFile(p.Config.PublicKeyFile)
		if err != nil {
			log.Fatal("Could not read public key file. Error: ", err)
		}
		info := core.ProviderInfo{
			ID:         p.Config.ProviderID,
			PublicKey:  string(pubKeyBytes),
			Addr:       addr,
			SpaceAvail: 1 << 32,
		}
		metaService := metaserver.NewClient(p.Config.MetaAddr, &http.Client{})
		err = metaService.RegisterProvider(&info)
		if err != nil {
			log.Fatalf("Unable to register with metaservice. Error: %s", err)
		}
		p.Config.IsRegistered = true
		err = util.SaveJson(path.Join(homedir, "provider", "config.json"), p.Config)
		if err != nil {
			log.Fatalf("Unable to update config after registering with metaserver. Error: %s", err)
		}
	}

	logfile, err := os.OpenFile(path.Join(homedir, "provider", "provider.log"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Cannot create log file: %s", err)
	}
	defer logfile.Close()
	logger := log.New(logfile, "", log.LstdFlags)

	server := provider.NewServer(p, logger)

	log.Println("starting provider server at", addr)
	log.Fatal(http.ListenAndServe(addr, server))
}
