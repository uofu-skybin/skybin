package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"log"
	"os"
	"path"
	"flag"
	"crypto/rand"
	"os/user"
)

var initCmd = Cmd{
	Name: "init",
	Description: "Set up skybin",
	Run: runInit,
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

const (
	DefaultMetaAddr = "127.0.0.1:8002"
	DefaultRenterAddr = ":8001"
)

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

	// Create renter keys
	userkey, err := rsa.GenerateKey(rand.Reader, 2048)
	checkErr(err)
	savePrivateKey(userkey, path.Join(homedir, "renter", "renterid"))
	savePublicKey(userkey.PublicKey, path.Join(homedir, "renter", "renterid.pub"))

	// Create renter config
	renterConfig := RenterConfig{
		Addr: DefaultRenterAddr,
		MetaAddr: DefaultMetaAddr,
	}
	err = saveJSON(path.Join(homedir, "renter", "config.json"), &renterConfig)
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
