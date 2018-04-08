package constants

type BuildModeType int

const (
	BuildModeTest BuildModeType = iota
	BuildModeProd
)

const (
	BuildMode = BuildModeTest
)

const (
	// Tenths of cents
	DefaultTestRenterBalance = 10000 * 100 * 10
)
