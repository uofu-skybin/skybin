package cmd

import (
	"log"
	"strconv"
	"strings"
)

var reserveCmd = Cmd{
	Name:        "reserve",
	Description: "Reserve storage",
	Usage:       "reserve <amount>",
	Run:         runReserve,
}

func parseBytes(s string) (int64, error) {
	var mul int64 = 1
	l := strings.ToLower(s)
	if strings.HasSuffix(l, "gb") {
		mul = 1e9
		l = s[:len(l)-len("gb")]
	} else if strings.HasSuffix(l, "mb") {
		mul = 1e6
		l = l[:len(l)-len("gb")]
	} else if strings.HasSuffix(l, "kb") {
		mul = 1e3
		l = l[:len(l)-len("kb")]
	} else if strings.HasSuffix(l, "b") {
		l = l[:len(l)-len("b")]
	}
	n, err := strconv.ParseInt(l, 10, 64)
	if err != nil {
		return 0, err
	}
	return n * mul, nil
}

func runReserve(args ...string) {

	if len(args) != 1 {
		log.Fatal("Must give amount")
	}

	amount, err := parseBytes(args[0])
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
