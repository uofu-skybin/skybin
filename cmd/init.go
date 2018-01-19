package cmd

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"flag"
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
	_ = fs.String("keyfile", "", "file containing renter's private key")
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

	// Create repo
	checkErr(os.MkdirAll(homedir, 0700))
	initRenter(homedir)
	initProvider(homedir)
}

func initRenter(homedir string) {

	// Create home folder
	checkErr(os.MkdirAll(path.Join(homedir, "renter"), 0700))

	// Create keys
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	checkErr(err)

	privateKeyPath := path.Join(homedir, "renter", "renterid")
	publicKeyPath := privateKeyPath + ".pub"

	savePrivateKey(rsaKey, privateKeyPath)
	savePublicKey(rsaKey.PublicKey, publicKeyPath)

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
	checkErr(os.MkdirAll(path.Join(homedir, "provider/blocks"), 0700)) // add blocks folder

	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	checkErr(err)

	privateKeyPath := path.Join(homedir, "provider", "providerid")
	publicKeyPath := path.Join(homedir, "provider", "providerid.pub")

	savePrivateKey(rsaKey, privateKeyPath)
	savePublicKey(rsaKey.PublicKey, publicKeyPath)

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
	keyBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	savePemBlock(keyBlock, path)
}

func savePublicKey(key rsa.PublicKey, path string) {
	bytes, err := x509.MarshalPKIXPublicKey(&key)
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
