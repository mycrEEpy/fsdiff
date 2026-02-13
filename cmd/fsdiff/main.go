package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/mycreepy/fsdiff/internal/fsdiff"
)

var (
	shouldPrintVersion bool

	version = "develop"
	commit  = "HEAD"
	date    = "just now"
)

func main() {
	flag.BoolVar(&shouldPrintVersion, "version", false, "Show version")

	flag.Parse()

	if shouldPrintVersion {
		printVersion()
		return
	}

	walker, err := fsdiff.New()
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}

	err = walker.Run()
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}
}

func printVersion() {
	fmt.Printf("fsdiff version %s (commit %s) built at %s\n", version, commit, date)
}
