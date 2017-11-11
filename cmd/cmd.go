package cmd

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
	renterCmd,
	providerCmd,
	metaServerCmd,
}
