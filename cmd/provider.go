package cmd

import (
	"crypto/rand"
	"crypto/rsa"
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
	"strings"
)

const providerUsage = `provider <command>

     Provider command tree

commands:

    init   Set up a skybin provider
    daemon Start a provider daemon
    info   Print daemon status information
`

var providerCmds = []*Cmd{
	&providerInitCmd,
	&providerDaemonCmd,
	&providerInfoCmd,
}

var providerCmd = Cmd{
	Name:        "provider",
	Description: "Provider command tree",
	Usage:       providerUsage,
	Run:         runProvider,
	Subcommands: providerCmds,
}

func runProvider(args ...string) {
	if len(args) == 0 {
		log.Fatal("usage: ", os.Args[0], " ", providerUsage)
	}
	for _, cmd := range providerCmds {
		if args[0] == cmd.Name {
			cmd.Run(args[1:]...)
			return
		}
	}
	log.Fatal("usage: ", os.Args[0], " ", providerUsage)
}

var providerInitUsage = `provider init [options...]
options:
    --homedir          Home directory to place files in (default ~/.skybin/provider)
    --public-api-addr  Public network address for renter traffic in 'host:port' form
    --local-api-addr   Local API network address in 'host:port' form
    --meta-addr        Address of the metaserver to register with
    --storage-space    Storage space to make available to renters (default 10GB)
    --pricing-policy   Policy to determine storage rates (fixed, passive, or aggressive) default: passive
    --min-storage-rate Minimum storage rate to charge, in tenths of cents/1e9 bytes/30 days
    --max-storage-rate Maximum storage rate to charge, in tenths of cents/1e9 bytes/30 days
`

var providerInitCmd = Cmd{
	Name:        "init",
	Description: "Set up a skybin provider",
	Usage:       providerInitUsage,
	Run:         runProviderInit,
}

func runProviderInit(args ...string) {
	fs := flag.NewFlagSet("", flag.ExitOnError)
	fs.Usage = func() {
		log.Println(providerInitUsage)
	}
	homeDirFlag := fs.String("homedir", "", "")
	metaAddrFlag := fs.String("meta-addr", "", "")
	localApiAddrFlag := fs.String("local-api-addr", "", "")
	publicApiAddrFlag := fs.String("public-api-addr", "", "")
	storageSpaceFlag := fs.String("storage-space", "", "")
	pricingPolicyFlag := fs.String("pricing-policy", "", "")
	minStorageRateFlag := fs.Int64("min-storage-rate", -1, "")
	maxStorageRateFlag := fs.Int64("max-storage-rate", -1, "")
	fs.Parse(args)

	homeDir := *homeDirFlag
	if len(homeDir) == 0 {
		skybinHome, err := defaultSkybinHome()
		if err != nil {
			log.Fatal("Unable to find user home directory. Error: ", err)
		}
		homeDir = path.Join(skybinHome, "provider")
	}

	if _, err := os.Stat(homeDir); err == nil {
		log.Fatal("Unable to create home directory. ", homeDir, " already exists")
	}

	err := os.MkdirAll(homeDir, 0700)
	if err != nil {
		log.Fatal("Unable to create provider directory. Error: ", err)
	}
	err = os.MkdirAll(path.Join(homeDir, "blocks"), 0700)
	if err != nil {
		os.RemoveAll(homeDir)
		log.Fatal("Unable to create blocks directory. Error: ", err)
	}

	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		os.RemoveAll(homeDir)
		log.Fatal("Unable to generate provider RSA key. Error: ", err)
	}

	privateKeyPath := path.Join(homeDir, "providerid")
	err = ioutil.WriteFile(privateKeyPath, util.MarshalPrivateKey(rsaKey), 0666)
	if err != nil {
		os.RemoveAll(homeDir)
		log.Fatal("Unable to save private key. Error: ", err)
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

	config := provider.Config{
		PublicApiAddr:  core.DefaultPublicProviderAddr,
		LocalApiAddr:   core.DefaultLocalProviderAddr,
		MetaAddr:       core.DefaultMetaAddr,
		PrivateKeyFile: privateKeyPath,
		PublicKeyFile:  publicKeyPath,
		SpaceAvail:     provider.DefaultStorageSpace,
		PricingPolicy:  provider.DefaultPricingPolicy,
		MinStorageRate: provider.DefaultMinStorageRate,
		StorageRate:    provider.DefaultMinStorageRate,
		MaxStorageRate: provider.DefaultMaxStorageRate,
	}
	if len(*metaAddrFlag) > 0 {
		err = util.ValidateNetAddr(*metaAddrFlag)
		if err != nil {
			os.RemoveAll(homeDir)
			log.Fatal("Invalid meta server address")
		}
		config.MetaAddr = *metaAddrFlag
	}
	if len(*publicApiAddrFlag) > 0 {
		err = util.ValidateNetAddr(*publicApiAddrFlag)
		if err != nil {
			os.RemoveAll(homeDir)
			log.Fatal("Invalid public API address")
		}
		config.PublicApiAddr = *publicApiAddrFlag
	}
	if len(*localApiAddrFlag) > 0 {
		err = util.ValidateNetAddr(*localApiAddrFlag)
		if err != nil {
			os.RemoveAll(homeDir)
			log.Fatal("Invalid local API address")
		}
		config.LocalApiAddr = *localApiAddrFlag
	}
	if len(*storageSpaceFlag) > 0 {
		amt, err := util.ParseByteAmount(*storageSpaceFlag)
		if err != nil {
			os.RemoveAll(homeDir)
			log.Fatal("Invalid storage amount.")
		}
		if amt < provider.MinStorageSpace {
			os.RemoveAll(homeDir)
			log.Fatal("Storage space must be at least ", util.FormatByteAmount(provider.MinStorageSpace))
		}
		config.SpaceAvail = amt
	}
	if len(*pricingPolicyFlag) > 0 {
		policy := provider.PricingPolicy(*pricingPolicyFlag)
		switch policy {
		case provider.FixedPricingPolicy:
			fallthrough
		case provider.PassivePricingPolicy:
			fallthrough
		case provider.AggressivePricingPolicy:
			config.PricingPolicy = policy
		default:
			os.RemoveAll(homeDir)
			log.Fatal("Invalid pricing policy")
		}
	}
	if *minStorageRateFlag != -1 {
		config.MinStorageRate = *minStorageRateFlag
		if config.StorageRate < *minStorageRateFlag {
			config.StorageRate = *minStorageRateFlag
		}
	}
	if *maxStorageRateFlag != -1 {
		config.MaxStorageRate = *maxStorageRateFlag
		if config.StorageRate > config.MaxStorageRate {
			config.StorageRate = config.MaxStorageRate
		}
	}

	// Register with metaserver
	info := core.ProviderInfo{
		PublicKey:   string(publicKeyBytes),
		Addr:        config.PublicApiAddr,
		SpaceAvail:  config.SpaceAvail,
		StorageRate: config.StorageRate,
	}
	metaClient := metaserver.NewClient(config.MetaAddr, &http.Client{})
	updatedInfo, err := metaClient.RegisterProvider(&info)
	if err != nil {
		os.RemoveAll(homeDir)
		log.Fatal("Unable to register with metaserver. Error: ", err)
	}

	// Pull generated provider ID from info
	config.ProviderID = updatedInfo.ID

	err = util.SaveJson(path.Join(homeDir, "config.json"), &config)
	if err != nil {
		os.RemoveAll(homeDir)
		log.Fatal("Unable to save provider configuration file. Error: ", err)
	}
}

var providerDaemonCmd = Cmd{
	Name:        "daemon",
	Description: "Start the provider daemon",
	Usage:       "provider daemon [--disable-local-api]",
	Run:         runProviderDaemon,
}

func runProviderDaemon(args ...string) {
	fs := flag.NewFlagSet("", flag.ExitOnError)
	disableLocalApiFlag := fs.Bool("disable-local-api", false, "Don't run the local API server")
	fs.Parse(args)

	homedir, err := findProviderHomedir()
	if err != nil {
		log.Fatal(err)
	}

	pvdr, err := provider.LoadFromDisk(homedir)
	if err != nil {
		log.Fatal(err)
	}

	logfile, err := os.OpenFile(path.Join(homedir, "provider.log"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Cannot create log file: %s", err)
	}
	defer logfile.Close()
	logger := log.New(logfile, "", log.LstdFlags)

	pvdr.SetLogger(logger)
	pvdr.StartBackgroundThreads()

	// Run local API
	if !*disableLocalApiFlag {
		log.Println("starting local provider server at", pvdr.Config.LocalApiAddr)
		go func() {
			localServer := provider.NewLocalServer(pvdr, logger)
			log.Fatal(http.ListenAndServe(pvdr.Config.LocalApiAddr, localServer))
		}()
	}

	server := provider.NewServer(pvdr, logger)
	port := pvdr.Config.PublicApiAddr[strings.LastIndex(pvdr.Config.PublicApiAddr, ":"):]
	log.Println("starting public provider server at", pvdr.Config.PublicApiAddr)
	log.Fatal(http.ListenAndServe(port, server))
}

var providerInfoCmd = Cmd{
	Name:        "info",
	Description: "Print provider daemon status information",
	Usage:       "provider info",
	Run:         runProviderInfo,
}

func runProviderInfo(args ...string) {
	log.Fatal("not implemented")
}
