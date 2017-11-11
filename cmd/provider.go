package cmd

import (
	"flag"
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
	Run:         runProvider,
}

func runProvider(args ...string) {
	fs := flag.NewFlagSet("provider", flag.ExitOnError)
	addrFlag := flag.String("addr", "", "address to listen at (host:port)")
	fs.Parse(args)

	homedir, err := findHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	var config provider.Config
	err = util.LoadJson(path.Join(homedir, "provider", "config.json"), &config)
	if err != nil {
		log.Fatalf("error: cannot read config. error: %s", err)
	}

	addr := *addrFlag
	if len(addr) == 0 {
		addr = config.Addr
	}

	// Register with metadata service.
	info := core.Provider{
		ID:         "",
		PublicKey:  "",
		Addr:       addr,
		SpaceAvail: 1 << 32,
	}
	metaService := metaserver.NewClient(config.MetaAddr, &http.Client{})
	err = metaService.RegisterProvider(&info)
	if err != nil {
		log.Fatalf("error: unable to register with metaservice. error: %s", err)
	}

	logfile, err := os.OpenFile(path.Join(homedir, "provider", "provider.log"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Cannot create log file: %s", err)
	}
	defer logfile.Close()

	logger := log.New(logfile, "", log.LstdFlags)

	server := provider.NewServer(&config, logger)

	log.Println("starting provider server at", addr)
	log.Fatal(http.ListenAndServe(addr, server))
}
