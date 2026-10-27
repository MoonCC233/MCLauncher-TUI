package version

import "fmt"

// These values are injected by GoReleaser.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func Full() string {
	if Commit == "none" || Commit == "" {
		return Version
	}
	if len(Commit) > 7 {
		Commit = Commit[:7]
	}
	return fmt.Sprintf("%s (%s)", Version, Commit)
}
