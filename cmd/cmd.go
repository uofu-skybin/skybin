package cmd

import (
	"fmt"
	"net/http"
	"os"
	"os/user"
	"path"
	"skybin/renter"
	"skybin/util"
	"text/tabwriter"
)

type Cmd struct {
	Name        string
	Usage       string
	Description string
	Run         func(args ...string)
}

var Commands = []Cmd{
	initCmd,
	reserveCmd,
	uploadCmd,
	listCmd,
	downloadCmd,
	rmCmd,
	mkdirCmd,
	renterCmd,
	providerCmd,
	metaServerCmd,
}

func init() {
	Commands = append(Commands, helpCmd)
}

func PrintUsage() {
	fmt.Printf("usage: %s <command> [option...]\n", os.Args[0])
	fmt.Println()
	fmt.Println("\tInteract with skybin")
	fmt.Println()
	fmt.Println("commands:")

	tw := tabwriter.NewWriter(os.Stdout, 0, 5, 5, ' ', 0)

	for _, cmd := range Commands {
		fmt.Fprintf(tw, "\t%s\t%s\n", cmd.Name, cmd.Description)
	}

	tw.Flush()

	fmt.Println()
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

func getRenterClient() (*renter.Client, error) {
	homedir, err := findHomeDir()
	if err != nil {
		return nil, err
	}

	var config renter.Config
	err = util.LoadJson(path.Join(homedir, "renter", "config.json"), &config)
	if err != nil {
		return nil, fmt.Errorf("Cannot load renter config. Error: %s", err)
	}

	return renter.NewClient(config.ApiAddr, &http.Client{}), nil
}
