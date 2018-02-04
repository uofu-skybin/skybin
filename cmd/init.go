package cmd

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/asn1"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path"
	"skybin/core"
	"skybin/provider"
	"skybin/renter"
	"skybin/util"
)

var initCmd = Cmd{
	Name:        "init",
	Description: "Set up a skybin directory",
	Usage:       "init [-home DIR] [-keyfile FILE]",
	Run:         runInit,
}

func runInit(args ...string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	homeFlag := fs.String("home", "", "directory to place skybin files")
	keyFileFlag := fs.String("keyfile", "", "file containing renter's private key")
	fs.Parse(args)

	homedir := *homeFlag
	if len(homedir) == 0 {
		var err error
		homedir, err = defaultHomeDir()
		if err != nil {
			log.Fatal(err)
		}
	}

	if _, err := os.Stat(homedir); err == nil {
		log.Fatalf("error: %s already exists\n", homedir)
	}

	// Create repo
	checkErr(os.MkdirAll(homedir, 0700))
	initRenter(homedir, *keyFileFlag)
	initProvider(homedir)
}

func initRenter(homedir string, keyfile string) {

	// Create home folder
	checkErr(os.MkdirAll(path.Join(homedir, "renter"), 0700))

	// Create keys
	var rsaKey *rsa.PrivateKey
	var err error
	if len(keyfile) != 0 {
		var keybytes []byte
		keybytes, err = ioutil.ReadFile(keyfile)
		checkErr(err)
		rsaKey, err = util.UnmarshalPrivateKey(keybytes)
	} else {
		rsaKey, err = rsa.GenerateKey(rand.Reader, 2048)
	}
	checkErr(err)

	privateKeyPath := path.Join(homedir, "renter", "renterid")
	publicKeyPath := privateKeyPath + ".pub"

	savePrivateKey(rsaKey, privateKeyPath)
	savePublicKey(&rsaKey.PublicKey, publicKeyPath)

	// Create config
	renterConfig := renter.Config{
		RenterId:       createId(rsaKey.PublicKey),
		ApiAddr:        core.DefaultRenterAddr,
		MetaAddr:       core.DefaultMetaAddr,
		PrivateKeyFile: privateKeyPath,
		PublicKeyFile:  publicKeyPath,
	}
	err = util.SaveJson(path.Join(homedir, "renter", "config.json"), &renterConfig)
	checkErr(err)
}

func initProvider(homedir string) {
	checkErr(os.MkdirAll(path.Join(homedir, "provider"), 0700))
	checkErr(os.MkdirAll(path.Join(homedir, "provider/blocks"), 0700))

	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	checkErr(err)

	privateKeyPath := path.Join(homedir, "provider", "providerid")
	publicKeyPath := path.Join(homedir, "provider", "providerid.pub")

	savePrivateKey(rsaKey, privateKeyPath)
	savePublicKey(&rsaKey.PublicKey, publicKeyPath)

	// Create provider config
	checkErr(err)
	providerConfig := provider.Config{
		ProviderID:     createId(rsaKey.PublicKey),
		ApiAddr:        core.DefaultProviderAddr,
		MetaAddr:       core.DefaultMetaAddr,
		PrivateKeyFile: privateKeyPath,
		PublicKeyFile:  publicKeyPath,
	}
	err = util.SaveJson(path.Join(homedir, "provider", "config.json"), &providerConfig)
	checkErr(err)
}

func createId(pubKey crypto.PublicKey) string {
	keyBytes, err := asn1.Marshal(pubKey)
	checkErr(err)
	return util.Hash(keyBytes)
}

func savePrivateKey(key *rsa.PrivateKey, path string) {
	checkErr(ioutil.WriteFile(path, util.MarshalPrivateKey(key), 0666))
}

func savePublicKey(key *rsa.PublicKey, path string) {
	keybytes, err := util.MarshalPublicKey(key)
	checkErr(err)
	checkErr(ioutil.WriteFile(path, keybytes, 0666))
}

func checkErr(err error) {
	if err != nil {
		log.Fatal("init failure: ", err)
	}
}
