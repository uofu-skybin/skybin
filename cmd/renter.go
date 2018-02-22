package cmd

import (
	"crypto/rand"
	"crypto/rsa"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"skybin/core"
	"skybin/metaserver"
	"skybin/renter"
	"skybin/util"
)

var renterUsage = `renter <command>

     Renter command tree

commands:

    init   Set up a skybin renter
    daemon Start the renter daemon
    info   Print daemon status information
`

var renterCommands = []*Cmd{
	&renterInitCmd,
	&renterDaemonCmd,
	&renterInfoCmd,
}

var renterCmd = Cmd{
	Name:        "renter",
	Description: "Renter command tree",
	Usage:       renterUsage,
	Run:         runRenter,
	Subcommands: renterCommands,
}

func runRenter(args ...string) {
	if len(args) == 0 {
		log.Fatal("usage: ", os.Args[0], " ", renterUsage)
	}
	for _, cmd := range renterCommands {
		if args[0] == cmd.Name {
			cmd.Run(args[1:]...)
			return
		}
	}
	log.Fatal("usage: ", os.Args[0], " ", renterUsage)
}

var renterInitUsage = `renter init [options...]
options:
    --alias     Renter alias to register (required)
    --homedir   Home directory to create (default ~/.skybin/renter)
    --key-file  The key file to import your identity from
    --meta-addr Address of the metaserver to register with
    --api-addr  Daemon API address in 'host:port' form
`

var renterInitCmd = Cmd{
	Name:        "init",
	Description: "Set up a skybin renter",
	Usage:       renterInitUsage,
	Run:         runRenterInit,
}

func runRenterInit(args ...string) {
	fs := flag.NewFlagSet("", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Println(renterInitUsage)
	}
	homeDirFlag := fs.String("homedir", "", "")
	aliasFlag := fs.String("alias", "", "")
	keyFileFlag := fs.String("key-file", "", "")
	metaAddrFlag := fs.String("meta-addr", "", "")
	apiAddrFlag := fs.String("api-addr", "", "")
	fs.Parse(args)

	if len(*aliasFlag) == 0 && len(*keyFileFlag) == 0 {
		log.Fatal("You must give an alias")
	}

	if len(*apiAddrFlag) > 0 {
		err := util.ValidateNetAddr(*apiAddrFlag)
		if err != nil {
			log.Fatal("Invalid API address.")
		}
	}

	if len(*metaAddrFlag) > 0 {
		err := util.ValidateNetAddr(*metaAddrFlag)
		if err != nil {
			log.Fatal("Invalid metaserver address.")
		}
	}

	homeDir := *homeDirFlag
	if len(homeDir) == 0 {
		defaultHome, err := defaultSkybinHome()
		if err != nil {
			log.Fatal("Unable to find user home directory. Error: ", err)
		}
		homeDir = path.Join(defaultHome, "renter")
	}

	if _, err := os.Stat(homeDir); err == nil {
		log.Fatal("Unable to create home directory. ", homeDir, " already exists")
	}

	err := os.MkdirAll(homeDir, 0700)
	if err != nil {
		log.Fatal("Unable to create home directory. Error: ", err)
	}

	// Create keys
	var rsaKey *rsa.PrivateKey
	if len(*keyFileFlag) > 0 {
		var keybytes []byte
		keybytes, err = ioutil.ReadFile(*keyFileFlag)
		if err != nil {
			os.RemoveAll(homeDir)
			log.Fatal("Unable to read key file. Error: ", err)
		}
		rsaKey, err = util.UnmarshalPrivateKey(keybytes)
		if err != nil {
			os.RemoveAll(homeDir)
			log.Fatal("Unable to decode private key. Are you sure ", *keyFileFlag, " is a valid key file?")
		}
	} else {
		rsaKey, err = rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			os.RemoveAll(homeDir)
			log.Fatal("Unable to generate user RSA key. Error: ", err)
		}
	}

	// Save keys
	privateKeyPath := path.Join(homeDir, "renterid")
	err = ioutil.WriteFile(privateKeyPath, util.MarshalPrivateKey(rsaKey), 0666)
	if err != nil {
		os.RemoveAll(homeDir)
		log.Fatal("Unable to save generated private key. Error: ", err)
	}
	publicKeyPath := privateKeyPath + ".pub"
	publicKeyBytes, err := util.MarshalPublicKey(&rsaKey.PublicKey)
	if err != nil {
		os.RemoveAll(homeDir)
		log.Fatal("Unable to save public key. Error: ", err)
	}
	err = ioutil.WriteFile(publicKeyPath, publicKeyBytes, 0666)
	if err != nil {
		os.RemoveAll(homeDir)
		log.Fatal("Unable to save public key. Error: ", err)
	}

	// Create renter config
	config := renter.DefaultConfig()
	config.PrivateKeyFile = privateKeyPath
	config.PublicKeyFile = publicKeyPath
	config.ApiAddr = core.DefaultRenterAddr
	config.MetaAddr = core.DefaultMetaAddr
	if len(*apiAddrFlag) > 0 {
		config.ApiAddr = *apiAddrFlag
	}
	if len(*metaAddrFlag) > 0 {
		config.MetaAddr = *metaAddrFlag
	}

	// Fetch user information from or register with metaserver
	metaService := metaserver.NewClient(config.MetaAddr, &http.Client{})
	if len(*keyFileFlag) > 0 {
		renterId := util.FingerprintKey(publicKeyBytes)
		err := metaService.AuthorizeRenter(rsaKey, renterId)
		if err != nil {
			os.RemoveAll(homeDir)
			log.Fatal("Unable to retrieve renter information. Are you sure this is the correct key?")
		}
		renterInfo, err := metaService.GetRenter(renterId)
		if err != nil {
			os.RemoveAll(homeDir)
			log.Fatal("Unable to retrieve renter information. Error: ", err)
		}

		// Pull out the user's renter alias
		config.RenterId = renterId
		config.Alias = renterInfo.Alias
	} else {

		// Register new user
		info := core.RenterInfo{
			Alias:     *aliasFlag,
			PublicKey: string(publicKeyBytes),
		}
		updatedInfo, err := metaService.RegisterRenter(&info)
		if err != nil {
			os.RemoveAll(homeDir)
			log.Fatal("Unable to register with metaserver. Error: ", err)
		}

		// Pull renter ID out of updated info
		config.RenterId = updatedInfo.ID
		config.Alias = *aliasFlag
	}

	err = util.SaveJson(path.Join(homeDir, "config.json"), &config)
	if err != nil {
		os.RemoveAll(homeDir)
		log.Fatal("Unable to save renter configuration file. Error: ", err)
	}
}

var renterDaemonCmd = Cmd{
	Name:        "daemon",
	Description: "Start a renter daemon",
	Usage:       "renter daemon [-addr]",
	Run:         runRenterDaemon,
}

func runRenterDaemon(args ...string) {
	fs := flag.NewFlagSet("renter daemon", flag.ExitOnError)
	addrFlag := fs.String("addr", "", "address to listen at (host:port)")
	fs.Parse(args)

	homedir, err := findRenterHomedir()
	if err != nil {
		log.Fatal(err)
	}

	r, err := renter.LoadFromDisk(homedir)
	if err != nil {
		log.Fatal(err)
	}

	addr := r.Config.ApiAddr
	if len(*addrFlag) > 0 {
		addr = *addrFlag
	}

	logfile, err := os.OpenFile(path.Join(homedir, "renter.log"),
		os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal("Unable to open log file. Error:", err)
	}
	defer logfile.Close()
	logger := log.New(logfile, "", log.LstdFlags)

	r.SetLogger(logger)
	server := renter.NewServer(r, logger)

	log.Println("starting renter server at", addr)
	log.Fatal(http.ListenAndServe(addr, server))
}

var renterInfoCmd = Cmd{
	Name:        "info",
	Description: "Print renter daemon status information",
	Usage:       "renter info",
	Run:         runRenterInfo,
}

func runRenterInfo(args ...string) {
	client, err := getRenterClient()
	if err != nil {
		log.Fatal(err)
	}
	info, err := client.GetInfo()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Renter ID:", info.ID)
	fmt.Println("Renter Alias:", info.Alias)
	fmt.Println("Renter home folder:", info.HomeDir)
	fmt.Println("Daemon API Address:", info.ApiAddr)
	fmt.Println("Total Files:", info.TotalFiles)
	fmt.Println("Total Storage Contracts:", info.TotalContracts)
	fmt.Println("Reserved Storage:", util.FormatByteAmount(info.ReservedStorage))
	fmt.Println("Used Storage:", util.FormatByteAmount(info.UsedStorage))
	fmt.Println("Free Storage:", util.FormatByteAmount(info.FreeStorage))
}

// Finds the renter config file and returns a renter client
// pointed to the appropriate address.
func getRenterClient() (*renter.Client, error) {
	homedir, err := findRenterHomedir()
	if err != nil {
		return nil, err
	}

	var config renter.Config
	err = util.LoadJson(path.Join(homedir, "config.json"), &config)
	if err != nil {
		return nil, fmt.Errorf("Cannot load renter config. Error: %s", err)
	}

	return renter.NewClient(config.ApiAddr, &http.Client{}), nil
}
