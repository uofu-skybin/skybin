package main

import (
	"flag"
	"log"
	"path"
	"skybin/core"
	"skybin/metaserver"
	"net/http"
	"github.com/gorilla/mux"
)

var providerCmd = Cmd{
	Name: "provider",
	Description: "Run a provider server",
	Run: runProvider,
}

type ProviderConfig struct {
	Addr string `json:"address"`
	MetaAddr string `json:"metadataServiceAddress"`
}

type providerServer struct {

}

func runProvider(args ...string) {
	fs := flag.NewFlagSet("provider", flag.ExitOnError)
	addrFlag := flag.String("addr", "", "address to listen at (host:port)")
	fs.Parse(args)

	homedir, err := findHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	var config ProviderConfig
	err = loadJSON(path.Join(homedir, "provider", "config.json"), &config)
	if err != nil {
		log.Fatalf("error: cannot read config. error: %s", err)
	}

	addr := *addrFlag
	if len(addr) == 0 {
		addr = config.Addr
	}

	info := core.Provider{
		ID: "",
		PublicKey: "",
		Addr: addr,
		SpaceAvail: 1 << 32,
	}

	// Register with metaserver service.
	metaService := metaserver.NewClient(config.MetaAddr, &http.Client{})
	err = metaService.RegisterProvider(&info)
	if err != nil {
		log.Fatalf("error: unable to register with metaservice. error: %s", err)
	}

	router := mux.NewRouter()

	log.Println("starting provider server at", addr)
	log.Fatal(http.ListenAndServe(addr, router))
}
