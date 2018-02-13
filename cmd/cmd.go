package cmd

import (
	"fmt"
	"os"
	"os/user"
	"path"
	"text/tabwriter"
)

type Cmd struct {
	Name        string
	Usage       string
	Description string
	Run         func(args ...string)
	Subcommands []*Cmd
}

var Commands = []*Cmd{
	&reserveCmd,
	&uploadCmd,
	&listCmd,
	&downloadCmd,
	&rmCmd,
	&mkdirCmd,
	&renterCmd,
	&providerCmd,
	&metaServerCmd,
}

func init() {
	Commands = append(Commands, &helpCmd)
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

func defaultSkybinHome() (string, error) {
	user, err := user.Current()
	if err != nil {
		return "", err
	}
	return path.Join(user.HomeDir, ".skybin"), nil
}

func findSkybinHomedir() (string, error) {
	homedir := os.Getenv("SKYBIN_HOME")
	if len(homedir) == 0 {
		var err error
		homedir, err = defaultSkybinHome()
		if err != nil {
			return "", err
		}
	}
	return homedir, nil
}

func findRenterHomedir() (string, error) {
	homedir := os.Getenv("SKYBIN_RENTER_HOME")
	if len(homedir) == 0 {
		var err error
		homedir, err = findSkybinHomedir()
		if err != nil {
			return "", err
		}
		homedir = path.Join(homedir, "renter")
	}
	return homedir, nil
}

func findProviderHomedir() (string, error) {
	homedir := os.Getenv("SKYBIN_PROVIDER_HOME")
	if len(homedir) == 0 {
		var err error
		homedir, err = findSkybinHomedir()
		if err != nil {
			return "", err
		}
		homedir = path.Join(homedir, "provider")
	}
	return homedir, nil
}
