package cmd

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"flag"
	"log"
	"os"
	"os/user"
	"path"
	"skybin/core"
	"skybin/provider"
	"skybin/renter"
	"skybin/util"
)

var initCmd = Cmd{
	Name:        "init",
	Description: "Set up a skybin directory",
	Run:         runInit,
}

func defaultHomeDir() (string, error) {
	user, err := user.Current()
	if err != nil {
		return "", err
	}
	return path.Join(user.HomeDir, ".skybin"), nil
}

func findHomeDir() (string, error) {
	homedir := os.Getenv("SKYBIN_HOME")
	if len(homedir) == 0 {
		var err error
		homedir, err = defaultHomeDir()
		if err != nil {
			return "", err
		}
	}
	return homedir, nil
}

func runInit(args ...string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	homeFlag := fs.String("home", "", "directory to place skybin files")
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
		log.Fatalf("error: %s already exists", homedir)
	}

	// Create repo directories
	checkErr(os.MkdirAll(homedir, 0700))
	checkErr(os.MkdirAll(path.Join(homedir, "renter"), 0700))
	checkErr(os.MkdirAll(path.Join(homedir, "provider"), 0700))
	checkErr(os.MkdirAll(path.Join(homedir, "provider/blocks"), 0700)) // add blocks folder

	// Create renter keys
	renterKey, err := rsa.GenerateKey(rand.Reader, 2048)
	checkErr(err)
	savePrivateKey(renterKey, path.Join(homedir, "renter", "renterid"))
	savePublicKey(renterKey.PublicKey, path.Join(homedir, "renter", "renterid.pub"))

	// Create renter config
	keyBytes, err := asn1.Marshal(renterKey.PublicKey)
	checkErr(err)
	renterConfig := renter.Config{
		RenterId:     util.Hash(keyBytes),
		Addr:         core.DefaultRenterAddr,
		MetaAddr:     core.DefaultMetaAddr,
		IdentityFile: path.Join(homedir, "renter", "renterid"),
	}
	err = util.SaveJson(path.Join(homedir, "renter", "config.json"), &renterConfig)
	checkErr(err)

	// Create provider keys
	providerKey, err := rsa.GenerateKey(rand.Reader, 2048)
	checkErr(err)
	savePrivateKey(providerKey, path.Join(homedir, "provider", "providerid"))
	savePublicKey(providerKey.PublicKey, path.Join(homedir, "provider", "providerid.pub"))

	// Create provider config
	keyBytes, err = asn1.Marshal(providerKey.PublicKey)
	checkErr(err)
	providerConfig := provider.Config{
		ProviderID:   util.Hash(keyBytes),
		Addr:         core.DefaultProviderAddr,
		MetaAddr:     core.DefaultMetaAddr,
		IdentityFile: path.Join(homedir, "provider", "providerid"),
		BlockDir:     path.Join(homedir, "provider", "blocks"),
	}
	err = util.SaveJson(path.Join(homedir, "provider", "config.json"), &providerConfig)
	checkErr(err)
}

func savePrivateKey(key *rsa.PrivateKey, path string) {
	keyBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	savePemBlock(keyBlock, path)
}

func savePublicKey(key rsa.PublicKey, path string) {
	bytes, err := asn1.Marshal(key)
	checkErr(err)
	keyBlock := &pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: bytes,
	}
	savePemBlock(keyBlock, path)
}

func savePemBlock(block *pem.Block, path string) {
	f, err := os.Create(path)
	checkErr(err)
	defer f.Close()
	checkErr(pem.Encode(f, block))
}

func checkErr(err error) {
	if err != nil {
		log.Fatal("init failure: ", err)
	}
}
