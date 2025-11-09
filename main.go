package main

import (
	"flag"
	"fmt"
	"os"
)

var (
	version = "dev"
)

func main() {
	// Command line args
	showVersion := flag.Bool("version", false, "Set to dump the version and exit")
	cfgPath := flag.String("cfgpath", "config.yml", "Path to config file for the server")
	flag.Parse()

	if *showVersion {
		fmt.Printf("version: %s", version)
		os.Exit(0)
	}

	// Grab the config
	cfg, err := getConfig(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}

	// Fire up the local web server
	startServer(cfg)
}
