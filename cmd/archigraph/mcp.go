package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/cajasmota/archigraph/internal/mcp"
)

// runMCP parses flags for the `mcp` subcommand. Currently the only verb is
// `serve`, which runs the MCP server on stdio until the connection closes.
//
//	archigraph mcp serve [--registry <path>]
func runMCP(argv []string) error {
	if len(argv) == 0 {
		return fmt.Errorf("missing verb (try `archigraph mcp serve`)")
	}
	switch argv[0] {
	case "serve":
		return runMCPServe(argv[1:])
	default:
		return fmt.Errorf("unknown verb: %s", argv[0])
	}
}

func runMCPServe(argv []string) error {
	fs := flag.NewFlagSet("mcp serve", flag.ContinueOnError)
	registry := fs.String("registry", "", "path to registry.json (default: ~/.archigraph/registry.json)")
	if err := fs.Parse(argv); err != nil {
		return err
	}
	debugLevel := 0
	if v := os.Getenv("ARCHIGRAPH_MCP_DEBUG"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			debugLevel = n
		}
	}
	cwd, _ := os.Getwd()
	srv, err := mcp.NewServer(mcp.Config{
		RegistryPath: *registry,
		DebugLevel:   debugLevel,
		CWD:          cwd,
	})
	if err != nil {
		return err
	}
	return srv.ServeStdio()
}
