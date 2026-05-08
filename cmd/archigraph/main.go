package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/cajasmota/archigraph/internal/version"
)

func main() {
	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.BoolVar(&showVersion, "v", false, "alias for --version")
	flag.Parse()

	if showVersion {
		fmt.Println(version.String())
		return
	}

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "archigraph — multi-repo code knowledge graphs for AI agents")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Usage: archigraph <command> [options]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "(commands not yet implemented — this is a scaffolding stub)")
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "archigraph: unknown command: %s\n", args[0])
	os.Exit(1)
}
