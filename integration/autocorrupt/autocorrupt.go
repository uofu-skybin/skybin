package main

// This is a script to automatically corrupt a certain number
// of blocks for a given file. It assumes that the block files
// are located in a subdirectory reposDir, as they should be
// if they are stored by providers running under the test net.

import (
	cryptrand "crypto/rand"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"skybin/core"
	"time"

	"github.com/globalsign/mgo"
)

const (
	dbAddress        = "127.0.0.1"
	dbName           = "skybin"
	defaultNumBlocks = -1
	reposDir         = "./repos"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	log.SetFlags(0)
	flag.Usage = func() {
		fmt.Printf("usage: %s [options] renter file\n", os.Args[0])
		fmt.Println("options:")
		flag.PrintDefaults()
	}
	numBlocksFlag := flag.Int("num-blocks", defaultNumBlocks, "number of blocks to corrupt")
	flag.Parse()
	if flag.NArg() < 2 {
		flag.Usage()
		os.Exit(1)
	}

	renter := flag.Arg(0)
	filename := flag.Arg(1)

	// Find the metadata for the file to corrupt
	session, err := mgo.Dial(dbAddress)
	if err != nil {
		log.Fatal("Unable to connect to DB: ", err)
	}
	defer session.Close()
	files := session.DB(dbName).C("files")
	selector := struct {
		OwnerAlias string
		Name       string
	}{
		renter,
		filename,
	}
	var file core.File
	err = files.Find(selector).One(&file)
	if err != nil {
		log.Fatal("Unable to find file: ", err)
	}

	// Select blocks to corrupt
	version := &file.Versions[len(file.Versions)-1]
	numBlocks := *numBlocksFlag
	if *numBlocksFlag == defaultNumBlocks {
		numBlocks = 1 + rand.Intn(version.NumParityBlocks-1)
	}
	if numBlocks > version.NumDataBlocks {
		numBlocks = version.NumDataBlocks
	}

	// Maps block IDs to blocks
	blocks := map[string]*core.Block{}
	for len(blocks) < numBlocks {
		idx := rand.Intn(version.NumDataBlocks)
		block := &version.Blocks[idx]
		blocks[block.ID] = block
	}

	// Corrupt the blocks on the filesystem
	err = filepath.Walk(reposDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		base := filepath.Base(path)
		block, exists := blocks[base]
		if !exists {
			return nil
		}
		fmt.Printf("corrupting file block %s\n", block.ID)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return err
		}
		defer f.Close()
		offset := rand.Int63n(info.Size())
		_, err = f.Seek(offset, os.SEEK_SET)
		if err != nil {
			return err
		}
		buf := make([]byte, 64)
		cryptrand.Read(buf)
		_, err = f.Write(buf)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		log.Fatal("error corrupting blocks:", err)
	}
	fmt.Println("done")
}
