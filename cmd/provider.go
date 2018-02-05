package cmd

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path"
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
	addrFlag := fs.String("addr", "", "address to listen at (host:port)")
	localFlag := fs.String("local", "", "address to listen at for local provider api (host:port)")
	fs.Parse(args)

	homedir, err := findHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	logfile, err := os.OpenFile(path.Join(homedir, "provider", "provider.log"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Cannot create log file: %s", err)
	}
	defer logfile.Close()
	logger := log.New(logfile, "", log.LstdFlags)

	p, err := provider.InitProvider(homedir)
	if err != nil {
		log.Fatalf("Failed to init provider: %s", err)
	}

	addr := *addrFlag
	localAddr := *localFlag
	// only run local facing server (statistics for ui) when localAddr is set
	if len(localAddr) != 0 {
		go func() {
			// localAddr = "localhost:29876"
			localServer := provider.NewLocalServer(p, logger)
			log.Println("starting local provider server at", localAddr)
			log.Fatal(http.ListenAndServe(localAddr, localServer))
		}()
	}

	if len(addr) == 0 {
		addr = p.Config.ApiAddr
	} else {
		// change provider configuration to use new public addr and update w/meta
		p.Config.ApiAddr = addr
		p.UpdateMeta()
	}
	server := provider.NewServer(p, logger)
	log.Println("starting public provider server at", addr)
	log.Fatal(http.ListenAndServe(addr, server))
}
