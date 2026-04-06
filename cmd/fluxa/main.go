// Package main is the entrypoint for the Fluxa AI gateway binary.
package main

import (
	"fmt"
	"os"
)

// Version is the gateway release version. It is overridden at build time
// via -ldflags "-X main.Version=...".
var Version = "0.0.1-dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("fluxa %s\n", Version)
		return
	}
	fmt.Printf("fluxa %s — AI gateway\n", Version)
}
