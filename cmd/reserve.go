package cmd

import (
	"log"
	"strconv"
)

var reserveCmd = Cmd{
	Name:        "reserve",
	Description: "Reserve storage",
	Usage:       "reserve <amount>",
	Run:         runReserve,
}

func runReserve(args ...string) {

	if len(args) != 1 {
		log.Fatal("Must give amount")
	}

	amount, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		log.Fatal(err)
	}

	client, err := getRenterClient()
	if err != nil {
		log.Fatal(err)
	}

	contracts, err := client.ReserveStorage(amount)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("SUCCESS")
	log.Println("Summary:")
	for _, c := range contracts {
		log.Printf("\tProvider ID: %s, Bytes Reserved: %d\n", c.ProviderId, c.StorageSpace)
	}
}
