package version

import "runtime/debug"

var (
	Version = "0.0.0-dev"
	Commit  = "unknown"
	Date    = "unknown"
)

func String() string {
	v := Version
	if info, ok := debug.ReadBuildInfo(); ok && Version == "0.0.0-dev" {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" {
				Commit = s.Value
			}
			if s.Key == "vcs.time" {
				Date = s.Value
			}
		}
	}
	return v + " (commit " + Commit + ", built " + Date + ")"
}
