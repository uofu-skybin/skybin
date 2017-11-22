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
)

var providerCmd = Cmd{
	Name:        "provider",
	Description: "Start a provider server",
	Usage:       "provider [-addr]",
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

	p, err := provider.LoadFromDisk(path.Join(homedir, "provider"))
	if err != nil {
		log.Fatal(err)
	}

	addr := *addrFlag
	if len(addr) == 0 {
		addr = p.Config.Addr
	}

	// Register with metadata service.
	info := core.Provider{
		ID:         p.Config.ProviderID,
		PublicKey:  "read public key in here",
		Addr:       addr,
		SpaceAvail: 1 << 32,
	}
	metaService := metaserver.NewClient(p.Config.MetaAddr, &http.Client{})
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

	server := provider.NewServer(p, logger)

	log.Println("starting provider server at", addr)
	log.Fatal(http.ListenAndServe(addr, server))
}
