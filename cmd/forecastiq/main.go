// Command forecastiq is the single ForecastIQ binary (ADR-013). It is the
// composition root — the only place infrastructure adapters are wired to
// module ports.
//
// Subcommands:
//
//	forecastiq serve --mode=api|worker|all   run the API and/or worker
//	forecastiq migrate up|down|force         apply / reverse migrations
//	forecastiq seed                          seed reference data (idempotent)
//
// @title                      ForecastIQ API
// @version                   1.0.0
// @description               Weather forecast accuracy comparison platform (first vertical slice).
// @BasePath                  /api/v1
// @securityDefinitions.apikey bearerAuth
// @in                        header
// @name                      Authorization
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "serve":
		err = cmdServe(os.Args[2:])
	case "migrate":
		err = cmdMigrate(os.Args[2:])
	case "seed":
		err = cmdSeed(os.Args[2:])
	case "help", "-h", "--help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `ForecastIQ — weather forecast accuracy platform

Usage:
  forecastiq serve --mode=api|worker|all   run the API and/or worker
  forecastiq migrate up                    apply all pending migrations
  forecastiq migrate down <n>              roll back <n> migrations
  forecastiq migrate force                 clear a dirty migration state
  forecastiq seed                          seed reference data (idempotent)

Configuration is via FIQ_-prefixed environment variables (see .env.example).
`)
}
